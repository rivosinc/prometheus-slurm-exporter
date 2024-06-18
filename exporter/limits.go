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
	// mem in bytes
	Mem float64
	CPU float64
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
		if len(records) != 3 {
			acf.errorCounter.Inc()
			slog.Error("failed to scrape account metric row %v", records)
			continue
		}
		account, cpu, mem := records[0], records[1], records[2]
		if mem == "" && cpu == "" {
			continue
		}
		metric := AccountLimitMetric{Account: account}
		if mem != "" {
			if memMb, err := strconv.ParseFloat(mem, 64); err != nil {
				slog.Error("failed to scrape account metric mem string %s", mem)
				acf.errorCounter.Inc()
			} else {
				metric.Mem = memMb * 1000
			}
		}
		if cpu != "" {
			if cpuCount, err := strconv.ParseFloat(cpu, 64); err != nil {
				slog.Error("failed to scrape account metric cpu string %s", cpu)
				acf.errorCounter.Inc()
			} else {
				metric.CPU = cpuCount
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
	fetcher          SlurmMetricFetcher[AccountLimitMetric]
	accountCpuLimit  *prometheus.Desc
	accountMemLimit  *prometheus.Desc
	limitScrapeError prometheus.Counter
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
		accountCpuLimit: prometheus.NewDesc("slurm_account_cpu_limit", "slurm account cpu limit", []string{"account"}, nil),
		accountMemLimit: prometheus.NewDesc("slurm_account_mem_limit", "slurm account mem limit (in bytes)", []string{"account"}, nil),
		limitScrapeError: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "slurm_account_collect_error",
			Help: "Slurm sacct collect error",
		}),
	}
}

func (lc *LimitCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- lc.accountCpuLimit
	ch <- lc.accountMemLimit
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
	for _, account := range limitMetrics {
		if account.Mem > 0 {
			ch <- prometheus.MustNewConstMetric(lc.accountMemLimit, prometheus.GaugeValue, account.Mem, account.Account)
		}
		if account.CPU > 0 {
			ch <- prometheus.MustNewConstMetric(lc.accountCpuLimit, prometheus.GaugeValue, account.CPU, account.Account)
		}
	}
}
