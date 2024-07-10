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
	assert.Len(accountLimits, 9)
	account1Limit := accountLimits[0]
	assert.Equal(account1Limit.Account, "account1")
	assert.Equal(account1Limit.CPU, 964.)
	assert.Equal(account1Limit.Mem, 15468557.*1e6)
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
	// 9 accounts, 2 metrics each + error&duration counter
	assert.Len(limitMetrics, 20)
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
