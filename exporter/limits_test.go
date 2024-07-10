// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
package exporter

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

var MockSacctFetcher = &MockScraper{fixture: "fixtures/sacctmgr.txt"}

func TestAccountLimitFetch(t *testing.T) {
	assert := assert.New(t)
	fetcher := AccountCsvFetcher{
		scraper:      MockSacctFetcher,
		errorCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
		cache:        NewAtomicThrottledCache[AccountLimitMetric](10),
	}
	accountLimits, err := fetcher.fetchFromCli()
	assert.NoError(err)
	assert.Len(accountLimits, 6)
	var account5Limits AccountLimitMetric
	for _, metric := range accountLimits {
		if metric.Account == "account5" {
			account5Limits = metric
		}
	}
	assert.Equal(account5Limits.Account, "account5")
	assert.Equal(account5Limits.AllocatedCPU, 3974.)
	assert.Equal(account5Limits.AllocatedMem, 47752500.*1e6)
	assert.Equal(account5Limits.AllocatedJobs, 4.e3)
	assert.Equal(account5Limits.TotalJobs, 3.e4)
}

func TestNewLimitCollector(t *testing.T) {
	assert := assert.New(t)
	config := Config{
		PollLimit: 10,
		cliOpts: &CliOpts{
			sacctEnabled: true,
		},
	}
	collector := NewLicCollector(&config)
	assert.NotNil(collector)
}

func TestLimitCollector(t *testing.T) {
	assert := assert.New(t)
	config := Config{
		PollLimit: 10,
		cliOpts: &CliOpts{
			sacctEnabled: true,
		},
	}
	lc := NewLimitCollector(&config)
	lc.fetcher = &AccountCsvFetcher{
		scraper:      MockSacctFetcher,
		errorCounter: lc.fetcher.ScrapeError(),
		cache:        NewAtomicThrottledCache[AccountLimitMetric](10),
	}
	lcChan := make(chan prometheus.Metric)
	go func() {
		lc.Collect(lcChan)
		close(lcChan)
	}()
	limitMetrics := make([]prometheus.Metric, 0)
	for metric, ok := <-lcChan; ok; metric, ok = <-lcChan {
		t.Log(metric.Desc().String())
		limitMetrics = append(limitMetrics, metric)
	}
	assert.NotEmpty(limitMetrics)
}
func TestLimitDescribe(t *testing.T) {
	assert := assert.New(t)
	config := Config{
		PollLimit: 10,
		cliOpts: &CliOpts{
			sacctEnabled: true,
		},
	}
	lc := NewLimitCollector(&config)
	lc.fetcher = &AccountCsvFetcher{
		scraper:      MockSacctFetcher,
		errorCounter: lc.fetcher.ScrapeError(),
		cache:        NewAtomicThrottledCache[AccountLimitMetric](10),
	}
	lcChan := make(chan *prometheus.Desc)
	go func() {
		lc.Describe(lcChan)
		close(lcChan)
	}()
	limitMetrics := make([]*prometheus.Desc, 0)
	for desc, ok := <-lcChan; ok; desc, ok = <-lcChan {
		t.Log(desc.String())
		limitMetrics = append(limitMetrics, desc)
	}
	assert.Len(limitMetrics, 4)
}
