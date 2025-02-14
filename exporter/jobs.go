// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"log/slog"
)

// the pending reason for a job denoting that a requested node is unavailable
const reqNodeNotAvailReason string = "ReqNodeNotAvail, UnavailableNodes"

type NodeResource struct {
	Mem float64 `json:"memory"`
}

type JobResource struct {
	AllocCpus  float64                  `json:"allocated_cpus"`
	AllocNodes map[string]*NodeResource `json:"allocated_nodes"`
}
type JobMetric struct {
	Account      string      `json:"account"`
	JobId        float64     `json:"job_id"`
	EndTime      float64     `json:"end_time"`
	JobState     string      `json:"job_state"`
	Partition    string      `json:"partition"`
	UserName     string      `json:"user_name"`
	Features     string      `json:"features"`
	JobResources JobResource `json:"job_resources"`
	StateReason  string      `json:"state_reason"`
}

type squeueResponse struct {
	Meta struct {
		SlurmVersion struct {
			Version struct {
				Major int `json:"major"`
				Micro int `json:"micro"`
				Minor int `json:"minor"`
			} `json:"version"`
			Release string `json:"release"`
		} `json:"Slurm"`
	} `json:"meta"`
	Errors []string    `json:"errors"`
	Jobs   []JobMetric `json:"jobs"`
}

type JobJsonFetcher struct {
	scraper    SlurmByteScraper
	cache      *AtomicThrottledCache[JobMetric]
	errCounter prometheus.Counter
}

func (jjf *JobJsonFetcher) fetch() ([]JobMetric, error) {
	data, err := jjf.scraper.FetchRawBytes()
	if err != nil {
		jjf.errCounter.Inc()
		return nil, err
	}
	var squeue squeueResponse
	err = json.Unmarshal(data, &squeue)
	if err != nil {
		slog.Error(fmt.Sprintf("Unmarshaling node metrics %q", err))
		return nil, err
	}
	for _, j := range squeue.Jobs {
		for _, resource := range j.JobResources.AllocNodes {
			resource.Mem *= 1e9
		}
	}
	return squeue.Jobs, nil
}

func (jjf *JobJsonFetcher) FetchMetrics() ([]JobMetric, error) {
	return jjf.cache.FetchOrThrottle(jjf.fetch)
}

func (jjf *JobJsonFetcher) ScrapeDuration() time.Duration {
	return jjf.scraper.Duration()
}

func (jjf *JobJsonFetcher) ScrapeError() prometheus.Counter {
	return jjf.errCounter
}

type JobCliFallbackFetcher struct {
	scraper    SlurmByteScraper
	cache      *AtomicThrottledCache[JobMetric]
	errCounter prometheus.Counter
}

func (jcf *JobCliFallbackFetcher) fetch() ([]JobMetric, error) {
	squeue, err := jcf.scraper.FetchRawBytes()
	if err != nil {
		return nil, err
	}
	jobMetrics := make([]JobMetric, 0)
	// clean input
	squeue = bytes.TrimSpace(squeue)
	squeue = bytes.Trim(squeue, "\n")
	if len(squeue) == 0 {
		// handle no jobs returned
		return nil, nil
	}

	for i, line := range bytes.Split(squeue, []byte("\n")) {
		var metric struct {
			Account     string    `json:"a"`
			JobId       float64   `json:"id"`
			EndTime     NAbleTime `json:"end_time"`
			JobState    string    `json:"state"`
			Partition   string    `json:"p"`
			UserName    string    `json:"u"`
			Cpu         int64     `json:"cpu"`
			Mem         string    `json:"mem"`
			StateReason string    `json:"r"`
		}
		if err := json.Unmarshal(line, &metric); err != nil {
			slog.Error(fmt.Sprintf("squeue fallback parse error: failed on line %d `%s`", i, line))
			jcf.errCounter.Inc()
			continue
		}
		mem, err := MemToFloat(metric.Mem)
		if err != nil {
			slog.Error(fmt.Sprintf("squeue fallback parse error: failed on line %d `%s` with err `%q`", i, line, err))
			jcf.errCounter.Inc()
			continue
		}
		re := regexp.MustCompile(`^\((?P<reason>(.+))\)$`)
		if metric.JobState == "PENDING" {
			if matches := re.FindStringSubmatch(metric.StateReason); matches != nil {
				metric.StateReason = matches[re.SubexpIndex("reason")]
			} else {
				slog.Error(fmt.Sprintf("squeue failed to pull pending state reason. Got state reason: %s", metric.StateReason))
				jcf.errCounter.Inc()
			}
		}

		openapiJobMetric := JobMetric{
			Account:     metric.Account,
			JobId:       metric.JobId,
			JobState:    metric.JobState,
			Partition:   metric.Partition,
			UserName:    metric.UserName,
			EndTime:     float64(metric.EndTime.Unix()),
			StateReason: metric.StateReason,
			JobResources: JobResource{
				AllocCpus:  float64(metric.Cpu),
				AllocNodes: map[string]*NodeResource{"0": {Mem: mem}},
			},
		}
		jobMetrics = append(jobMetrics, openapiJobMetric)
	}
	return jobMetrics, nil
}

func (jcf *JobCliFallbackFetcher) FetchMetrics() ([]JobMetric, error) {
	return jcf.cache.FetchOrThrottle(jcf.fetch)
}

func (jcf *JobCliFallbackFetcher) ScrapeDuration() time.Duration {
	return jcf.scraper.Duration()
}

func (jcf *JobCliFallbackFetcher) ScrapeError() prometheus.Counter {
	return jcf.errCounter
}

func totalAllocMem(resource *JobResource) float64 {
	var allocMem float64
	for _, node := range resource.AllocNodes {
		allocMem += node.Mem
	}
	return allocMem
}

type NAbleTime struct{ time.Time }

// report beginning of time in the case of N/A
func (nat *NAbleTime) UnmarshalJSON(data []byte) error {
	var tString string
	if err := json.Unmarshal(data, &tString); err != nil {
		return err
	}
	nullSet := map[string]struct{}{"N/A": {}, "NONE": {}}
	if _, ok := nullSet[tString]; ok {
		nat.Time = time.Time{}
		return nil
	}
	t, err := time.Parse("2006-01-02T15:04:05", tString)
	nat.Time = t
	return err
}

type UserJobMetric struct {
	stateJobCount map[string]float64
	totalJobCount float64
	allocMemory   map[string]float64
	allocCpu      map[string]float64
}

func parseUserJobMetrics(jobMetrics []JobMetric) map[string]*UserJobMetric {
	userMetricMap := make(map[string]*UserJobMetric)
	for _, jobMetric := range jobMetrics {
		metric, ok := userMetricMap[jobMetric.UserName]
		if !ok {
			metric = &UserJobMetric{
				stateJobCount: make(map[string]float64),
				allocMemory:   make(map[string]float64),
				allocCpu:      make(map[string]float64),
			}
		}
		metric.stateJobCount[jobMetric.JobState]++
		metric.totalJobCount++
		metric.allocMemory[jobMetric.JobState] += totalAllocMem(&jobMetric.JobResources)
		metric.allocCpu[jobMetric.JobState] += jobMetric.JobResources.AllocCpus
		userMetricMap[jobMetric.UserName] = metric
	}
	return userMetricMap
}

type AccountMetric struct {
	stateAllocMem map[string]float64
	stateAllocCpu map[string]float64
	stateJobCount map[string]float64
}

func parseAccountMetrics(jobs []JobMetric) map[string]*AccountMetric {
	accountMap := make(map[string]*AccountMetric)
	for _, job := range jobs {
		metric, ok := accountMap[job.Account]
		if !ok {
			metric = &AccountMetric{
				stateJobCount: make(map[string]float64),
				stateAllocMem: make(map[string]float64),
				stateAllocCpu: make(map[string]float64),
			}
			accountMap[job.Account] = metric
		}
		metric.stateAllocCpu[job.JobState] += job.JobResources.AllocCpus
		metric.stateAllocMem[job.JobState] += totalAllocMem(&job.JobResources)
		metric.stateJobCount[job.JobState]++
	}
	return accountMap
}

type PartitionJobMetric struct {
	partitionState map[string]float64
}

func parsePartitionJobMetrics(jobs []JobMetric) map[string]*PartitionJobMetric {
	partitionMetric := make(map[string]*PartitionJobMetric)
	for _, job := range jobs {
		metric, ok := partitionMetric[job.Partition]
		if !ok {
			metric = &PartitionJobMetric{
				partitionState: make(map[string]float64),
			}
			partitionMetric[job.Partition] = metric
		}
		metric.partitionState[job.JobState]++
	}
	return partitionMetric
}

type StateReasonMetric struct {
	pendingStateCount map[string]float64
}

func parseStateReasonMetric(jobs []JobMetric) *StateReasonMetric {
	metric := StateReasonMetric{
		pendingStateCount: make(map[string]float64),
	}

	for _, job := range jobs {
		if job.JobState != "PENDING" {
			continue
		}
		reason := job.StateReason
		if strings.Contains(reason, reqNodeNotAvailReason) {
			// consolidate pending node not avail reason to be node agnostic. i.e
			// from (ReqNodeNotAvail, UnavailableNodes:cs[100,...])
			// to (ReqNodeNotAvail, UnavailableNodes)
			reason = fmt.Sprintf("(%s)", reqNodeNotAvailReason)
		}
		metric.pendingStateCount[reason]++
	}
	return &metric
}

type FeatureJobMetric struct {
	allocMem float64
	allocCpu float64
	total    float64
}

func parseFeatureMetric(jobs []JobMetric) map[string]*FeatureJobMetric {
	featureMap := make(map[string]*FeatureJobMetric)
	for _, job := range jobs {
		for _, feature := range strings.Split(job.Features, "&") {
			metric, ok := featureMap[feature]
			if !ok {
				metric = new(FeatureJobMetric)
				featureMap[feature] = metric
			}
			metric.allocCpu += job.JobResources.AllocCpus
			metric.allocMem += totalAllocMem(&job.JobResources)
			metric.total++
		}
	}
	return featureMap
}

type JobsCollector struct {
	// collector state
	fetcher      SlurmMetricFetcher[JobMetric]
	fallback     bool
	jobAllocCpus *prometheus.Desc
	jobAllocMem  *prometheus.Desc
	// user metrics
	userJobStateTotal *prometheus.Desc
	userJobMemAlloc   *prometheus.Desc
	userJobCpuAlloc   *prometheus.Desc
	// partition
	partitionJobStateTotal *prometheus.Desc
	// account metrics
	accountJobStateMemAlloc *prometheus.Desc
	accountJobStateCpuAlloc *prometheus.Desc
	accountJobStateTotal    *prometheus.Desc
	// feature metrics
	featureJobMemAlloc *prometheus.Desc
	featureJobCpuAlloc *prometheus.Desc
	featureJobTotal    *prometheus.Desc
	// reason metrics
	pendingReasonTotal *prometheus.Desc
	// exporter metrics
	jobScrapeDuration *prometheus.Desc
	jobScrapeError    prometheus.Counter
}

func (jc *JobsCollector) SetFetcher(fetcher SlurmMetricFetcher[JobMetric]) {
	jc.fetcher = fetcher
}

func NewJobsController(config *Config) *JobsCollector {
	cliOpts := config.cliOpts
	fetcher := config.TraceConf.sharedFetcher
	return &JobsCollector{
		fetcher:  fetcher,
		fallback: cliOpts.fallback,
		// individual job metrics
		jobAllocCpus:            prometheus.NewDesc("slurm_job_alloc_cpus", "amount of cpus allocated per job", []string{"jobid"}, nil),
		jobAllocMem:             prometheus.NewDesc("slurm_job_alloc_mem", "amount of mem allocated per job", []string{"jobid"}, nil),
		userJobStateTotal:       prometheus.NewDesc("slurm_user_state_total", "total jobs per state per user", []string{"username", "state"}, nil),
		userJobMemAlloc:         prometheus.NewDesc("slurm_user_mem_alloc", "total mem alloc per user", []string{"username", "state"}, nil),
		userJobCpuAlloc:         prometheus.NewDesc("slurm_user_cpu_alloc", "total cpu alloc per user", []string{"username", "state"}, nil),
		partitionJobStateTotal:  prometheus.NewDesc("slurm_partition_job_state_total", "total jobs per partition per state", []string{"partition", "state"}, nil),
		accountJobStateMemAlloc: prometheus.NewDesc("slurm_account_job_state_mem_alloc", "alloc mem consumed per account per job state", []string{"account", "state"}, nil),
		accountJobStateCpuAlloc: prometheus.NewDesc("slurm_account_job_state_cpu_alloc", "alloc cpu consumed per account per job state", []string{"account", "state"}, nil),
		accountJobStateTotal:    prometheus.NewDesc("slurm_account_job_state_total", "total jobs per account per job state", []string{"account", "state"}, nil),
		featureJobMemAlloc:      prometheus.NewDesc("slurm_feature_mem_alloc", "alloc mem consumed per feature", []string{"feature"}, nil),
		featureJobCpuAlloc:      prometheus.NewDesc("slurm_feature_cpu_alloc", "alloc cpu consumed per feature", []string{"feature"}, nil),
		featureJobTotal:         prometheus.NewDesc("slurm_feature_total", "alloc cpu consumed per feature", []string{"feature"}, nil),
		pendingReasonTotal:      prometheus.NewDesc("slurm_pending_reason_total", "count of the reason jobs are pending", []string{"reason"}, nil),
		jobScrapeDuration:       prometheus.NewDesc("slurm_job_scrape_duration", fmt.Sprintf("how long the cmd %v took (ms)", cliOpts.squeue), nil, nil),
		jobScrapeError: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "slurm_job_scrape_error",
			Help: "slurm job scrape error",
		}),
	}
}

func (jc *JobsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- jc.jobAllocCpus
	ch <- jc.jobAllocMem
	ch <- jc.userJobStateTotal
	ch <- jc.userJobMemAlloc
	ch <- jc.userJobCpuAlloc
	ch <- jc.partitionJobStateTotal
	ch <- jc.accountJobStateMemAlloc
	ch <- jc.accountJobStateCpuAlloc
	ch <- jc.accountJobStateTotal
	ch <- jc.featureJobMemAlloc
	ch <- jc.featureJobCpuAlloc
	ch <- jc.featureJobTotal
	ch <- jc.pendingReasonTotal
	ch <- jc.jobScrapeDuration
	ch <- jc.jobScrapeError.Desc()
}

func (jc *JobsCollector) Collect(ch chan<- prometheus.Metric) {
	defer func() {
		ch <- jc.fetcher.ScrapeError()
	}()
	jobMetrics, err := jc.fetcher.FetchMetrics()
	ch <- prometheus.MustNewConstMetric(jc.jobScrapeDuration, prometheus.GaugeValue, float64(jc.fetcher.ScrapeDuration().Milliseconds()))
	if err != nil {
		slog.Error(fmt.Sprintf("fetcher failure %q", err))
		return
	}
	userMetrics := parseUserJobMetrics(jobMetrics)
	for user, metric := range userMetrics {
		for state, allocCpu := range metric.allocCpu {
			if allocCpu > 0 {
				ch <- prometheus.MustNewConstMetric(jc.userJobCpuAlloc, prometheus.GaugeValue, allocCpu, user, state)
			}
		}
		for state, allocMem := range metric.allocMemory {
			if allocMem > 0 {
				ch <- prometheus.MustNewConstMetric(jc.userJobMemAlloc, prometheus.GaugeValue, allocMem, user, state)
			}
		}
		for state, count := range metric.stateJobCount {
			if count > 0 {
				ch <- prometheus.MustNewConstMetric(jc.userJobStateTotal, prometheus.GaugeValue, count, user, state)
			}
		}
	}

	emitNonZeroStateConstGuage := func(desc *prometheus.Desc, metricMap map[string]float64, account string) {
		for state, val := range metricMap {
			if val > 0 {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, val, account, state)
			}
		}
	}

	accountMetrics := parseAccountMetrics(jobMetrics)
	for account, metric := range accountMetrics {
		emitNonZeroStateConstGuage(jc.accountJobStateCpuAlloc, metric.stateAllocCpu, account)
		emitNonZeroStateConstGuage(jc.accountJobStateMemAlloc, metric.stateAllocMem, account)
		emitNonZeroStateConstGuage(jc.accountJobStateTotal, metric.stateJobCount, account)
	}

	partitionJobMetrics := parsePartitionJobMetrics(jobMetrics)
	for partition, stateTotals := range partitionJobMetrics {
		for state, totalJobs := range stateTotals.partitionState {
			ch <- prometheus.MustNewConstMetric(jc.partitionJobStateTotal, prometheus.GaugeValue, totalJobs, partition, state)
		}
	}

	featureJobMetric := parseFeatureMetric(jobMetrics)
	for feature, metric := range featureJobMetric {
		if metric.allocCpu > 0 {
			ch <- prometheus.MustNewConstMetric(jc.featureJobCpuAlloc, prometheus.GaugeValue, metric.allocCpu, feature)
		}
		if metric.allocMem > 0 {
			ch <- prometheus.MustNewConstMetric(jc.featureJobMemAlloc, prometheus.GaugeValue, metric.allocMem, feature)
		}
		if metric.total > 0 {
			ch <- prometheus.MustNewConstMetric(jc.featureJobTotal, prometheus.GaugeValue, metric.total, feature)
		}
	}

	stateReasonMetric := parseStateReasonMetric(jobMetrics)
	for pendingReason, pendingCount := range stateReasonMetric.pendingStateCount {
		ch <- prometheus.MustNewConstMetric(jc.pendingReasonTotal, prometheus.GaugeValue, pendingCount, pendingReason)
	}
}
