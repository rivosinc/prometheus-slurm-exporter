package cext

// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

import "C"

import (
	"fmt"
	"strings"
	"time"

	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rivosinc/prometheus-slurm-exporter/exporter"
)

type Destructor interface {
	Deinit()
}

type CNodeFetcher struct {
	cache        *exporter.AtomicThrottledCache[exporter.NodeMetric]
	scraper      NodeMetricScraper
	duration     time.Duration
	errorCounter prometheus.Counter
}

// should be defer'd immediately after new cmd to prevent mem leaks
func (cni *CNodeFetcher) Deinit() {
	DeleteNodeMetricScraper(cni.scraper)
}

func (cni *CNodeFetcher) CToGoMetricConvert() ([]exporter.NodeMetric, error) {
	if errno := cni.scraper.CollectNodeInfo(); errno != 0 {
		cni.errorCounter.Inc()
		return nil, fmt.Errorf("Node Info CPP errno: %d", errno)
	}
	cni.scraper.IterReset()
	nodeMetrics := make([]exporter.NodeMetric, 0)
	metric := NewPromNodeMetric()
	defer DeletePromNodeMetric(metric)
	nodeStates := map[uint64]string{
		0: "UNKNOWN",
		1: "DOWN",
		2: "IDLE",
		3: "ALLOCATED",
		4: "ERROR",
		5: "MIXED",
		6: "FUTURE",
		// used by the C api to detect end of enum. Shouldn't ever be emitted
		7: "END",
	}

	now := time.Now()
	for cni.scraper.IterNext(metric) == 0 {
		nodeMetrics = append(nodeMetrics, exporter.NodeMetric{
			Hostname:    metric.GetHostname(),
			Cpus:        float64(metric.GetCpus()),
			RealMemory:  float64(metric.GetRealMemory()),
			FreeMemory:  float64(metric.GetFreeMem()),
			Partitions:  strings.Split(metric.GetPartitions(), ","),
			State:       nodeStates[metric.GetNodeState()],
			AllocMemory: float64(metric.GetAllocMem()),
			AllocCpus:   float64(metric.GetAllocCpus()),
			IdleCpus:    float64(metric.GetCpus()) - float64(metric.GetAllocCpus()),
			Weight:      float64(metric.GetWeight()),
			CpuLoad:     float64(metric.GetCpuLoad()),
		})
	}
	cni.duration = time.Since(now)
	return nodeMetrics, nil
}

func (cni *CNodeFetcher) FetchMetrics() ([]exporter.NodeMetric, error) {
	return cni.cache.FetchOrThrottle(cni.CToGoMetricConvert)
}

func (cni *CNodeFetcher) ScrapeDuration() time.Duration {
	return cni.duration
}

func (cni *CNodeFetcher) ScrapeError() prometheus.Counter {
	return cni.errorCounter
}

func NewNodeFetcher(pollLimit float64) *CNodeFetcher {
	return &CNodeFetcher{
		cache:   exporter.NewAtomicThrottledCache[exporter.NodeMetric](pollLimit),
		scraper: NewNodeMetricScraper(""),
		errorCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "slurm_cplugin_node_fetch_error",
			Help: "slurm cplugin fetch error",
		}),
	}
}

type CJobFetcher struct {
	cache        *exporter.AtomicThrottledCache[exporter.JobMetric]
	scraper      JobMetricScraper
	duration     time.Duration
	errorCounter prometheus.Counter
}

func (cjf *CJobFetcher) CToGoMetricConvert() ([]exporter.JobMetric, error) {
	if errno := cjf.scraper.CollectJobInfo(); errno != 0 {
		cjf.errorCounter.Inc()
		return nil, fmt.Errorf("Job Info CPP errno: %d", errno)
	}
	jobStates := map[int]string{
		0:  "PENDING",
		1:  "RUNNING",
		2:  "SUSPENDED",
		3:  "COMPLETE",
		4:  "CANCELLED",
		5:  "FAILED",
		6:  "TIMEOUT",
		7:  "NODE_FAIL",
		8:  "PREEMPTED",
		9:  "BOOT_FAIL",
		10: "DEADLINE",
		11: "OOM",
		// should never happen
		12: "END",
	}
	metrics := make([]exporter.JobMetric, 0)
	cmetric := NewPromJobMetric()
	defer DeletePromJobMetric(cmetric)
	cjf.scraper.IterReset()
	for cjf.scraper.IterNext(cmetric) == 0 {
		metric := exporter.JobMetric{
			Account:   cmetric.GetAccount(),
			JobId:     float64(cmetric.GetJobId()),
			EndTime:   cmetric.GetEndTime(),
			JobState:  jobStates[cmetric.GetJobState()],
			UserName:  cmetric.GetUserName(),
			Partition: cmetric.GetPartitions(),
			JobResources: exporter.JobResource{
				AllocCpus:  cmetric.GetAllocCpus(),
				AllocNodes: map[string]*exporter.NodeResource{"0": {Mem: cmetric.GetAllocMem()}},
			},
		}
		metrics = append(metrics, metric)
		slog.Error(fmt.Sprintf("metrics %v, alloc mem %f", metric, metric.JobResources.AllocNodes["0"].Mem))
	}

	return metrics, nil
}

func (cjf *CJobFetcher) FetchMetrics() ([]exporter.JobMetric, error) {
	return cjf.cache.FetchOrThrottle(cjf.CToGoMetricConvert)
}

func (cjf *CJobFetcher) ScrapeDuration() time.Duration {
	return cjf.duration
}

func (cjf *CJobFetcher) ScrapeError() prometheus.Counter {
	return cjf.errorCounter
}

func (cjf *CJobFetcher) Deinit() {
	DeleteJobMetricScraper(cjf.scraper)
}

func NewJobFetcher(pollLimit float64) *CJobFetcher {
	return &CJobFetcher{
		cache:   exporter.NewAtomicThrottledCache[exporter.JobMetric](pollLimit),
		scraper: NewJobMetricScraper(""),
		errorCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "slurm_cplugin_job_fetch_error",
			Help: "slurm cplugin job fetch error",
		}),
	}
}
