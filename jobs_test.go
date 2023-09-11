// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

var MockJobInfoFetcher = &MockFetcher{fixture: "fixtures/squeue_out.json"}

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
	var job *JobMetrics
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
	metrics, err := parseCliFallback(data)
	assert.Nil(err)
	assert.NotEmpty(metrics)
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
