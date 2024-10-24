// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

var MockJobInfoScraper = &MockScraper{fixture: "fixtures/squeue_out.json"}

func CollectCounterValue(counter prometheus.Counter) float64 {
	metricChan := make(chan prometheus.Metric, 1)
	counter.Collect(metricChan)
	dtoMetric := new(dto.Metric)
	(<-metricChan).Write(dtoMetric)
	return dtoMetric.GetCounter().GetValue()
}

func TestNewJobsController(t *testing.T) {
	assert := assert.New(t)
	config := &Config{
		PollLimit: 10,
		TraceConf: &TraceConfig{
			sharedFetcher: &JobCliFallbackFetcher{
				scraper:    &MockScraper{fixture: "fixtures/squeue_fallback.txt"},
				cache:      NewAtomicThrottledCache[JobMetric](1),
				errCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
			},
		},
		cliOpts: &CliOpts{
			fallback: true,
		},
	}
	jc := NewJobsController(config)
	assert.NotNil(jc)
}

func TestParseJobMetrics(t *testing.T) {
	assert := assert.New(t)
	scraper := &MockScraper{fixture: "fixtures/squeue_out.json"}
	fetcher := &JobJsonFetcher{
		scraper:    scraper,
		cache:      NewAtomicThrottledCache[JobMetric](100),
		errCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
	}
	jms, err := fetcher.fetch()
	assert.NoError(err)
	// test parse of single job
	var job *JobMetric
	for _, m := range jms {
		if m.JobId == 26515966 {
			job = &m
			break
		}
	}
	assert.NotNil(job)
	assert.Equal(6.4e13, totalAllocMem(&job.JobResources))
}

func TestParseCliFallback(t *testing.T) {
	assert := assert.New(t)
	cliFallbackFetcher := &JobCliFallbackFetcher{
		scraper:    &MockScraper{fixture: "fixtures/squeue_fallback.txt"},
		cache:      NewAtomicThrottledCache[JobMetric](100),
		errCounter: prometheus.NewCounter(prometheus.CounterOpts{Name: "errors"}),
	}
	metrics, err := cliFallbackFetcher.fetch()
	assert.Nil(err)
	assert.NotEmpty(metrics)
	assert.Equal(2., CollectCounterValue(cliFallbackFetcher.errCounter))
}

func TestUserJobMetric(t *testing.T) {
	// setup
	assert := assert.New(t)
	scraper := &MockScraper{fixture: "fixtures/squeue_out.json"}
	fetcher := &JobJsonFetcher{
		scraper:    scraper,
		cache:      NewAtomicThrottledCache[JobMetric](100),
		errCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
	}
	jms, err := fetcher.fetch()
	assert.Nil(err)

	//test
	state := "RUNNING"
	expectedUser := "bkd"

	for user, metric := range parseUserJobMetrics(jms) {
		if user == expectedUser {
			assert.Equal(2., metric.totalJobCount)
			assert.Equal(1., metric.allocCpu[state])
			assert.Equal(1., metric.stateJobCount[state])
			assert.Equal(6.4e+13, metric.allocMemory[state])
		} else {
			t.Fatal("unexpected user in reseult")
		}
	}
}

func TestJobCollect(t *testing.T) {
	assert := assert.New(t)
	config := &Config{
		PollLimit: 10,
		TraceConf: &TraceConfig{
			sharedFetcher: &JobJsonFetcher{
				scraper:    &MockScraper{fixture: "fixtures/squeue_out.json"},
				cache:      NewAtomicThrottledCache[JobMetric](1),
				errCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
			},
			rate: 10,
		},
		cliOpts: &CliOpts{},
	}
	jc := NewJobsController(config)
	jobChan := make(chan prometheus.Metric)
	go func() {
		jc.Collect(jobChan)
		close(jobChan)
	}()
	jobMetrics := make([]prometheus.Metric, 0)
	for metric, ok := <-jobChan; ok; metric, ok = <-jobChan {
		t.Log(metric.Desc().String())
		jobMetrics = append(jobMetrics, metric)
	}
	assert.NotEmpty(jobMetrics)
}

func TestJobCollect_Fallback(t *testing.T) {
	assert := assert.New(t)
	config := &Config{
		PollLimit: 10,
		TraceConf: &TraceConfig{
			sharedFetcher: &JobCliFallbackFetcher{
				scraper:    &MockScraper{fixture: "fixtures/squeue_fallback.txt"},
				cache:      NewAtomicThrottledCache[JobMetric](1),
				errCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
			},
			rate: 10,
		},
		cliOpts: &CliOpts{
			fallback: true,
		},
	}
	jc := NewJobsController(config)
	jobChan := make(chan prometheus.Metric)
	go func() {
		jc.Collect(jobChan)
		close(jobChan)
	}()
	jobMetrics := make([]prometheus.Metric, 0)
	for metric, ok := <-jobChan; ok; metric, ok = <-jobChan {
		t.Log(metric.Desc().String())
		jobMetrics = append(jobMetrics, metric)
	}
	assert.NotEmpty(jobMetrics)

}

func TestParsePartitionJobMetrics(t *testing.T) {
	assert := assert.New(t)
	scraper := &MockScraper{fixture: "fixtures/squeue_out.json"}
	fetcher := &JobJsonFetcher{
		scraper:    scraper,
		cache:      NewAtomicThrottledCache[JobMetric](100),
		errCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
	}
	jms, err := fetcher.fetch()
	assert.Nil(err)

	partitionJobMetrics := parsePartitionJobMetrics(jms)
	assert.Equal(float64(1), partitionJobMetrics["hw-l"].partitionState["RUNNING"])
}

func TestParsePartMetrics(t *testing.T) {
	assert := assert.New(t)
	scraper := &MockScraper{fixture: "fixtures/squeue_out.json"}
	fetcher := &JobJsonFetcher{
		scraper:    scraper,
		cache:      NewAtomicThrottledCache[JobMetric](100),
		errCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
	}
	jms, err := fetcher.fetch()
	assert.Nil(err)

	featureMetrics := parseFeatureMetric(jms)
	assert.Equal(1., featureMetrics["a100-80gb"].total)
	assert.Equal(1., featureMetrics["preemptible"].allocCpu)
}

func TestJobDescribe(t *testing.T) {
	assert := assert.New(t)
	ch := make(chan *prometheus.Desc)
	config, err := NewConfig(new(CliFlags))
	assert.Nil(err)
	config.TraceConf.sharedFetcher = &JobJsonFetcher{
		scraper:    MockJobInfoScraper,
		cache:      NewAtomicThrottledCache[JobMetric](1),
		errCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
	}
	config.TraceConf.rate = 10
	jc := NewJobsController(config)
	go func() {
		jc.Describe(ch)
		close(ch)
	}()
	descs := make([]*prometheus.Desc, 0)
	for desc, ok := <-ch; ok; desc, ok = <-ch {
		descs = append(descs, desc)
	}
	assert.NotEmpty(descs)
}

func TestNAbleTimeJson(t *testing.T) {
	assert := assert.New(t)
	data := `"2023-09-21T14:31:11"`
	var nat NAbleTime
	err := nat.UnmarshalJSON([]byte(data))
	assert.Nil(err)
	assert.True(nat.Equal(time.Date(2023, 9, 21, 14, 31, 11, 0, time.UTC)))
}

func TestNAbleTimeJson_NA(t *testing.T) {
	assert := assert.New(t)
	data := `"N/A"`
	var nat NAbleTime
	err := nat.UnmarshalJSON([]byte(data))
	assert.Nil(err)
	assert.True(nat.Equal(time.Time{}))
}

func TestParseCliFallbackEmpty(t *testing.T) {
	assert := assert.New(t)
	scraper := &StringByteScraper{msg: ""}
	cliFallbackFetcher := &JobCliFallbackFetcher{
		scraper:    scraper,
		cache:      NewAtomicThrottledCache[JobMetric](100),
		errCounter: prometheus.NewCounter(prometheus.CounterOpts{Name: "errors"}),
	}
	metrics, err := cliFallbackFetcher.fetch()
	assert.NoError(err)
	assert.Empty(metrics)
	assert.Zero(CollectCounterValue(cliFallbackFetcher.errCounter))
	assert.Equal(1, scraper.Callcount)
	scraper.msg = "\n"
	metrics, err = cliFallbackFetcher.fetch()
	assert.NoError(err)
	assert.Empty(metrics)
	assert.Zero(CollectCounterValue(cliFallbackFetcher.errCounter))
	assert.Equal(2, scraper.Callcount)
}

func TestCliJobFetcherCacheHit(t *testing.T) {
	assert := assert.New(t)
	scraper := &MockScraper{fixture: "fixtures/squeue_fallback.txt"}
	cliFallbackFetcher := &JobCliFallbackFetcher{
		scraper:    scraper,
		cache:      NewAtomicThrottledCache[JobMetric](100),
		errCounter: prometheus.NewCounter(prometheus.CounterOpts{Name: "errors"}),
	}
	metrics, err := cliFallbackFetcher.FetchMetrics()
	assert.NotEmpty(metrics)
	assert.NoError(err)
	assert.Equal(1, scraper.CallCount)
	metrics, err = cliFallbackFetcher.FetchMetrics()
	assert.NotEmpty(metrics)
	assert.NoError(err)
	// assert cache hit
	assert.Equal(1, scraper.CallCount)
}

func TestCliJobFetcherCacheMiss(t *testing.T) {
	assert := assert.New(t)
	scraper := &MockScraper{fixture: "fixtures/squeue_fallback.txt"}
	cliFallbackFetcher := &JobCliFallbackFetcher{
		scraper:    scraper,
		cache:      NewAtomicThrottledCache[JobMetric](0),
		errCounter: prometheus.NewCounter(prometheus.CounterOpts{Name: "errors"}),
	}
	metrics, err := cliFallbackFetcher.FetchMetrics()
	assert.NotEmpty(metrics)
	assert.NoError(err)
	assert.Equal(1, scraper.CallCount)
	metrics, err = cliFallbackFetcher.FetchMetrics()
	assert.NotEmpty(metrics)
	assert.NoError(err)
	// assert cache hit
	assert.Equal(2, scraper.CallCount)
}

func TestJsonJobFetcherCacheHit(t *testing.T) {
	assert := assert.New(t)
	scraper := &MockScraper{fixture: "fixtures/squeue_out.json"}
	cliFallbackFetcher := &JobJsonFetcher{
		scraper:    scraper,
		cache:      NewAtomicThrottledCache[JobMetric](100),
		errCounter: prometheus.NewCounter(prometheus.CounterOpts{Name: "errors"}),
	}
	metrics, err := cliFallbackFetcher.FetchMetrics()
	assert.NotEmpty(metrics)
	assert.NoError(err)
	assert.Equal(1, scraper.CallCount)
	metrics, err = cliFallbackFetcher.FetchMetrics()
	assert.NotEmpty(metrics)
	assert.NoError(err)
	// assert cache hit
	assert.Equal(1, scraper.CallCount)
}

func TestJsonJobFetcherCacheMiss(t *testing.T) {
	assert := assert.New(t)
	scraper := &MockScraper{fixture: "fixtures/squeue_out.json"}
	cliFallbackFetcher := &JobJsonFetcher{
		scraper:    scraper,
		cache:      NewAtomicThrottledCache[JobMetric](0),
		errCounter: prometheus.NewCounter(prometheus.CounterOpts{Name: "errors"}),
	}
	metrics, err := cliFallbackFetcher.FetchMetrics()
	assert.NotEmpty(metrics)
	assert.NoError(err)
	assert.Equal(1, scraper.CallCount)
	metrics, err = cliFallbackFetcher.FetchMetrics()
	assert.NotEmpty(metrics)
	assert.NoError(err)
	// assert cache hit
	assert.Equal(2, scraper.CallCount)
}

func TestParseStateReasonMetric_Fallback(t *testing.T) {
	assert := assert.New(t)
	scraper := &MockScraper{fixture: "fixtures/squeue_fallback.txt"}
	cliFallbackFetcher := &JobCliFallbackFetcher{
		scraper:    scraper,
		cache:      NewAtomicThrottledCache[JobMetric](0),
		errCounter: prometheus.NewCounter(prometheus.CounterOpts{Name: "errors"}),
	}
	jobMetrics, err := cliFallbackFetcher.FetchMetrics()
	assert.NotEmpty(jobMetrics)
	assert.NoError(err)
	m := parseStateReasonMetric(jobMetrics)
	assert.NotEmpty(m.pendingStateCount)
	assert.Equal(m.pendingStateCount["Dependency"], 1.)
	assert.Equal(m.pendingStateCount["Priority"], 1.)
}
func TestParseStateReasonMetric_Json(t *testing.T) {
	assert := assert.New(t)
	scraper := &MockScraper{fixture: "fixtures/squeue_out.json"}
	JsonFetcher := &JobJsonFetcher{
		scraper:    scraper,
		cache:      NewAtomicThrottledCache[JobMetric](0),
		errCounter: prometheus.NewCounter(prometheus.CounterOpts{Name: "errors"}),
	}
	jobMetrics, err := JsonFetcher.FetchMetrics()
	assert.NotEmpty(jobMetrics)
	assert.NoError(err)
	m := parseStateReasonMetric(jobMetrics)
	assert.NotEmpty(m.pendingStateCount)
	assert.Equal(m.pendingStateCount["Dependency"], 1.)
}
