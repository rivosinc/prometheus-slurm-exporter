// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

var MockLicFetcher = &MockScraper{fixture: "fixtures/license_out.json"}

func TestNewLicController(t *testing.T) {
	assert := assert.New(t)
	config := Config{
		pollLimit: 10,
		cliOpts: &CliOpts{
			licEnabled: true,
		},
	}
	lc := NewLicCollector(&config)
	assert.NotNil(lc)
}

func TestLicCollect(t *testing.T) {
	assert := assert.New(t)
	config := Config{
		pollLimit: 10,
		cliOpts: &CliOpts{
			licEnabled: true,
		},
	}
	lc := NewLicCollector(&config)
	lc.fetcher = &CliJsonLicMetricFetcher{
		scraper:      MockLicFetcher,
		cache:        NewAtomicThrottledCache[LicenseMetric](1),
		errorCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
	}
	lcChan := make(chan prometheus.Metric)
	go func() {
		lc.Collect(lcChan)
		close(lcChan)
	}()
	licMetrics := make([]prometheus.Metric, 0)
	for metric, ok := <-lcChan; ok; metric, ok = <-lcChan {
		t.Log(metric.Desc().String())
		licMetrics = append(licMetrics, metric)
	}
	assert.NotEmpty(licMetrics)
}

func TestLicCollect_ColectionE(t *testing.T) {
	assert := assert.New(t)
	config := Config{
		pollLimit: 10,
		cliOpts: &CliOpts{
			licEnabled: true,
		},
	}
	lc := NewLicCollector(&config)
	lc.fetcher = &CliJsonLicMetricFetcher{
		scraper:      MockLicFetcher,
		cache:        NewAtomicThrottledCache[LicenseMetric](1),
		errorCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
	}
	lcChan := make(chan prometheus.Metric)
	go func() {
		lc.Collect(lcChan)
		close(lcChan)
	}()
	licMetrics := make([]prometheus.Metric, 0)
	for metric, ok := <-lcChan; ok; metric, ok = <-lcChan {
		t.Log(metric.Desc().String())
		licMetrics = append(licMetrics, metric)
	}

	assert.Equal(3, len(licMetrics))
}

func TestLicDescribe(t *testing.T) {
	assert := assert.New(t)
	config := Config{
		pollLimit: 10,
		cliOpts: &CliOpts{
			licEnabled: true,
		},
	}
	lc := NewLicCollector(&config)
	lc.fetcher = &CliJsonLicMetricFetcher{
		scraper:      MockLicFetcher,
		cache:        NewAtomicThrottledCache[LicenseMetric](1),
		errorCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
	}
	lcChan := make(chan *prometheus.Desc)
	go func() {
		lc.Describe(lcChan)
		close(lcChan)
	}()
	licMetrics := make([]*prometheus.Desc, 0)
	for desc, ok := <-lcChan; ok; desc, ok = <-lcChan {
		t.Log(desc.String())
		licMetrics = append(licMetrics, desc)
	}
	assert.NotEmpty(licMetrics)
}
