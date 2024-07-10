// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
package exporter

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/exp/slog"
)

type AccountLimitMetric struct {
	Account string
	// limit to the amount of resources for a particular account in the RUNNING state
	AllocatedMem  float64
	AllocatedCPU  float64
	AllocatedJobs float64
	// limit to the amount of resources that can be either PENDING or RUNNING
	TotalJobs float64
}

type AccountCsvFetcher struct {
	scraper      SlurmByteScraper
	errorCounter prometheus.Counter
	cache        *AtomicThrottledCache[AccountLimitMetric]
}

func (acf *AccountCsvFetcher) fetchFromCli() ([]AccountLimitMetric, error) {
	cliCsv, err := acf.scraper.FetchRawBytes()
	if err != nil {
		acf.errorCounter.Inc()
		slog.Error(fmt.Sprintf("failed to scrape account metrics with %q", err))
		return nil, err
	}

	reader := csv.NewReader(bytes.NewBuffer(cliCsv))
	reader.Comma = '|'
	accountMetrics := make([]AccountLimitMetric, 0)
	for records, err := reader.Read(); err != io.EOF; records, err = reader.Read() {
		if err != nil {
			acf.errorCounter.Inc()
			slog.Error("failed to scrape account metric row %v", records)
			continue
		}
		if len(records) != 6 {
			acf.errorCounter.Inc()
			slog.Error("failed to scrape account metric row %v", records)
			continue
		}
		user, account, cpu, mem, runningJobs, totalJobs := records[0], records[1], records[2], records[3], records[4], records[5]

		if user != "" {
			// sacctmgr will display account limits by setting the user to ""
			// otherwise the user -> account association is shown
			// i.e user Bob can allocate x cpu within account Blah
			continue
		}
		metric := AccountLimitMetric{Account: account}
		if mem != "" {
			if memMb, err := strconv.ParseFloat(mem, 64); err != nil {
				slog.Error("failed to scrape account metric mem string %s", mem)
				acf.errorCounter.Inc()
			} else {
				metric.AllocatedMem = memMb * 1e6
			}
		}
		if cpu != "" {
			if cpuCount, err := strconv.ParseFloat(cpu, 64); err != nil {
				slog.Error("failed to scrape account metric cpu string %s", cpu)
				acf.errorCounter.Inc()
			} else {
				metric.AllocatedCPU = cpuCount
			}
		}
		if runningJobs != "" {
			if runnableJobs, err := strconv.ParseFloat(runningJobs, 64); err != nil {
				slog.Error(fmt.Sprintf("failed to scrape account metric AllocatableJobs (jobs in RUNNING state) with err: %q", err))
				acf.errorCounter.Inc()
			} else {
				metric.AllocatedJobs = runnableJobs
			}
		}
		if totalJobs != "" {
			if allJobs, err := strconv.ParseFloat(totalJobs, 64); err != nil {
				slog.Error(fmt.Sprintf("failed to scrape account metric TotalJobs (jobs in RUNNING or PENDING state) with err: %q", err))
				acf.errorCounter.Inc()
			} else {
				metric.TotalJobs = allJobs
			}
		}
		accountMetrics = append(accountMetrics, metric)
	}
	return accountMetrics, nil
}

func (acf *AccountCsvFetcher) FetchMetrics() ([]AccountLimitMetric, error) {
	return acf.cache.FetchOrThrottle(acf.fetchFromCli)
}

func (acf *AccountCsvFetcher) ScrapeError() prometheus.Counter {
	return acf.errorCounter
}

func (acf *AccountCsvFetcher) ScrapeDuration() time.Duration {
	return acf.scraper.Duration()
}

type LimitCollector struct {
	fetcher             SlurmMetricFetcher[AccountLimitMetric]
	accountCpuLimit     *prometheus.Desc
	accountMemLimit     *prometheus.Desc
	limitScrapeDuration *prometheus.Desc
	limitScrapeError    prometheus.Counter
}

func NewLimitCollector(config *Config) *LimitCollector {
	cliOpts := config.cliOpts
	if !cliOpts.sacctEnabled {
		log.Fatal("tried to invoke limit collector while cli disabled")
	}
	return &LimitCollector{
		fetcher: &AccountCsvFetcher{
			scraper: NewCliScraper(cliOpts.sacctmgr...),
			cache:   NewAtomicThrottledCache[AccountLimitMetric](config.PollLimit),
			errorCounter: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "slurm_account_scrape_error",
				Help: "Slurm sacct scrape error",
			}),
		},
		accountCpuLimit:     prometheus.NewDesc("slurm_account_cpu_limit", "slurm account cpu limit", []string{"account"}, nil),
		accountMemLimit:     prometheus.NewDesc("slurm_account_mem_limit", "slurm account mem limit (in bytes)", []string{"account"}, nil),
		limitScrapeDuration: prometheus.NewDesc("slurm_limit_scrape_duration", "slurm sacctmgr scrape duration", nil, nil),
		limitScrapeError: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "slurm_account_collect_error",
			Help: "Slurm sacct collect error",
		}),
	}
}

func (lc *LimitCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- lc.accountCpuLimit
	ch <- lc.accountMemLimit
	ch <- lc.limitScrapeDuration
	ch <- lc.limitScrapeError.Desc()
}

func (lc *LimitCollector) Collect(ch chan<- prometheus.Metric) {
	defer func() {
		ch <- lc.limitScrapeError
	}()
	limitMetrics, err := lc.fetcher.FetchMetrics()
	if err != nil {
		lc.limitScrapeError.Inc()
		slog.Error(fmt.Sprintf("lic parse error %q", err))
		return
	}
	ch <- prometheus.MustNewConstMetric(lc.limitScrapeDuration, prometheus.GaugeValue, float64(lc.fetcher.ScrapeDuration().Milliseconds()))
	for _, account := range limitMetrics {
		if account.AllocatedMem > 0 {
			ch <- prometheus.MustNewConstMetric(lc.accountMemLimit, prometheus.GaugeValue, account.AllocatedMem, account.Account)
		}
		if account.AllocatedCPU > 0 {
			ch <- prometheus.MustNewConstMetric(lc.accountCpuLimit, prometheus.GaugeValue, account.AllocatedCPU, account.Account)
		}
	}
}
