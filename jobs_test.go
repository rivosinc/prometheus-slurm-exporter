// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

var MockJobInfoFetcher = &MockFetcher{fixture: "fixtures/squeue_out.json"}

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
		pollLimit: 10,
		traceConf: &TraceConfig{
			sharedFetcher: MockJobInfoFetcher,
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
	fixture, err := MockJobInfoFetcher.Fetch()
	assert.Nil(err)
	jms, err := parseJobMetrics(fixture)
	assert.Nil(err)
	assert.NotEmpty(jms)
	// test parse of single job
	var job *JobMetric
	for _, m := range jms {
		if m.JobId == 26515966 {
			job = &m
			break
		}
	}
	assert.NotNil(job)
	assert.Equal(float64(64000), totalAllocMem(&job.JobResources))
}

func TestParseCliFallback(t *testing.T) {
	assert := assert.New(t)
	fetcher := MockFetcher{fixture: "fixtures/squeue_fallback.txt"}
	data, err := fetcher.Fetch()
	assert.Nil(err)
	counter := prometheus.NewCounter(prometheus.CounterOpts{Name: "errors"})
	metrics, err := parseCliFallback(data, counter)
	assert.Nil(err)
	assert.NotEmpty(metrics)
	assert.Equal(2., CollectCounterValue(counter))
}

func TestUserJobMetric(t *testing.T) {
	// setup
	assert := assert.New(t)
	fixture, err := MockJobInfoFetcher.Fetch()
	assert.Nil(err)
	jms, err := parseJobMetrics(fixture)
	assert.Nil(err)

	//test
	for _, metric := range parseUserJobMetrics(jms) {
		assert.Positive(metric.allocCpu)
		assert.Positive(metric.allocMemory)
	}
}

func TestJobCollect(t *testing.T) {
	assert := assert.New(t)
	config := &Config{
		pollLimit: 10,
		traceConf: &TraceConfig{
			sharedFetcher: MockJobInfoFetcher,
			rate:          10,
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
		pollLimit: 10,
		traceConf: &TraceConfig{
			sharedFetcher: &MockFetcher{fixture: "fixtures/squeue_fallback.txt"},
			rate:          10,
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
	fixture, err := MockJobInfoFetcher.Fetch()
	assert.Nil(err)
	jms, err := parseJobMetrics(fixture)
	assert.Nil(err)

	partitionJobMetrics := parsePartitionJobMetrics(jms)
	assert.Equal(float64(1), partitionJobMetrics["hw-l"].partitionState["RUNNING"])
}

func TestJobDescribe(t *testing.T) {
	assert := assert.New(t)
	ch := make(chan *prometheus.Desc)
	config, err := NewConfig()
	assert.Nil(err)
	config.SetFetcher(MockJobInfoFetcher)
	jc := NewJobsController(config)
	jc.fetcher = MockJobInfoFetcher
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
