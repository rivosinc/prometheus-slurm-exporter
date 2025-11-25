// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type FairShareMetric struct {
	Account   string
	FairShare float64
}

type FairShareFetcher struct {
	scraper      SlurmByteScraper
	errorCounter prometheus.Counter
	cache        *AtomicThrottledCache[FairShareMetric]
}

func (fsf *FairShareFetcher) fetchFromCli() ([]FairShareMetric, error) {
	cliCsv, err := fsf.scraper.FetchRawBytes()
	if err != nil {
		fsf.errorCounter.Inc()
		slog.Error(fmt.Sprintf("failed to scrape fairshare metrics with %q", err))
		return nil, err
	}

	reader := csv.NewReader(bytes.NewBuffer(cliCsv))
	reader.Comma = '|'
	reader.TrimLeadingSpace = true

	// Use map to deduplicate accounts (keep last seen value)
	accountMap := make(map[string]float64)
	for records, err := reader.Read(); err != io.EOF; records, err = reader.Read() {
		if err != nil {
			fsf.errorCounter.Inc()
			slog.Error(fmt.Sprintf("failed to scrape fairshare metric row %v", records))
			continue
		}
		if len(records) != 2 {
			fsf.errorCounter.Inc()
			slog.Error(fmt.Sprintf("expected 2 fields, got %d in row %v", len(records), records))
			continue
		}

		account, fairshareStr := records[0], records[1]

		// Skip empty values and "inf" values
		if fairshareStr == "" || fairshareStr == "inf" {
			continue
		}

		fairshare, err := strconv.ParseFloat(fairshareStr, 64)
		if err != nil {
			slog.Warn("Failed to parse fairshare value", "account", account, "value", fairshareStr, "err", err)
			fsf.errorCounter.Inc()
			continue
		}

		// Store in map, overwriting any previous value for this account
		accountMap[account] = fairshare
	}

	// Convert map to slice
	fairshareMetrics := make([]FairShareMetric, 0, len(accountMap))
	for account, fairshare := range accountMap {
		fairshareMetrics = append(fairshareMetrics, FairShareMetric{
			Account:   account,
			FairShare: fairshare,
		})
	}

	return fairshareMetrics, nil
}

func (fsf *FairShareFetcher) FetchMetrics() ([]FairShareMetric, error) {
	return fsf.cache.FetchOrThrottle(fsf.fetchFromCli)
}

func (fsf *FairShareFetcher) ScrapeError() prometheus.Counter {
	return fsf.errorCounter
}

func (fsf *FairShareFetcher) ScrapeDuration() time.Duration {
	return fsf.scraper.Duration()
}

type FairShareCollector struct {
	fetcher                 SlurmMetricFetcher[FairShareMetric]
	accountFairShare        *prometheus.Desc
	fairshareScrapeDuration *prometheus.Desc
	fairshareScrapeError    prometheus.Counter
}

func NewFairShareCollector(config *Config) *FairShareCollector {
	cliOpts := config.cliOpts
	return &FairShareCollector{
		fetcher: &FairShareFetcher{
			scraper: NewCliScraper(cliOpts.fairshare...),
			cache:   NewAtomicThrottledCache[FairShareMetric](config.PollLimit),
			errorCounter: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "slurm_fairshare_scrape_error",
				Help: "Slurm sshare scrape error",
			}),
		},
		accountFairShare: prometheus.NewDesc(
			"slurm_account_fairshare",
			"FairShare value for account",
			[]string{"account"},
			nil,
		),
		fairshareScrapeDuration: prometheus.NewDesc(
			"slurm_fairshare_scrape_duration",
			"slurm sshare scrape duration",
			nil,
			nil,
		),
		fairshareScrapeError: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "slurm_fairshare_collect_error",
			Help: "Slurm sshare collect error",
		}),
	}
}

func (fsc *FairShareCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- fsc.accountFairShare
	ch <- fsc.fairshareScrapeDuration
	ch <- fsc.fairshareScrapeError.Desc()
}

func (fsc *FairShareCollector) Collect(ch chan<- prometheus.Metric) {
	defer func() {
		ch <- fsc.fairshareScrapeError
	}()

	fairshareMetrics, err := fsc.fetcher.FetchMetrics()
	if err != nil {
		fsc.fairshareScrapeError.Inc()
		slog.Error(fmt.Sprintf("fairshare parse error %q", err))
		return
	}

	ch <- prometheus.MustNewConstMetric(
		fsc.fairshareScrapeDuration,
		prometheus.GaugeValue,
		float64(fsc.fetcher.ScrapeDuration().Milliseconds()),
	)

	for _, metric := range fairshareMetrics {
		ch <- prometheus.MustNewConstMetric(
			fsc.accountFairShare,
			prometheus.GaugeValue,
			metric.FairShare,
			metric.Account,
		)
	}
}
