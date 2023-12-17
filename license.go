// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/exp/slog"
)

type LicenseMetric struct {
	LicenseName string `json:"LicenseName"`
	Total       int    `json:"Total"`
	Used        int    `json:"Used"`
	Free        int    `json:"Free"`
	Remote      bool   `json:"Remote"`
	Reserved    int    `json:"Reserved"`
}

type scontrolLicResponse struct {
	Meta struct {
		SlurmVersion struct {
			Version struct {
				Major int `json:"major"`
				Micro int `json:"micro"`
				Minor int `json:"minor"`
			} `json:"version"`
			Release string `json:"release"`
		} `json:"Slurm"`
	} `json:"meta"`
	Licenses []LicenseMetric `json:"licenses"`
}

type CliJsonLicMetricFetcher struct {
	scraper      SlurmByteScraper
	cache        *AtomicThrottledCache[LicenseMetric]
	errorCounter prometheus.Counter
}

func (cjl *CliJsonLicMetricFetcher) fetch() ([]LicenseMetric, error) {
	licBytes, err := cjl.scraper.FetchRawBytes()
	if err != nil {
		slog.Error(fmt.Sprintf("fetch error %q", err))
		cjl.errorCounter.Inc()
		return nil, err
	}
	lic := new(scontrolLicResponse)
	if err := json.Unmarshal(licBytes, lic); err != nil {
		slog.Error(fmt.Sprintf("Unmarshaling license metrics %q", err))
		return nil, err
	}
	return lic.Licenses, nil
}

func (cjl *CliJsonLicMetricFetcher) FetchMetrics() ([]LicenseMetric, error) {
	return cjl.cache.FetchOrThrottle(cjl.fetch)
}

func (cjl *CliJsonLicMetricFetcher) ScrapeDuration() time.Duration {
	return cjl.cache.duration
}

func (cjl *CliJsonLicMetricFetcher) ScrapeError() prometheus.Counter {
	return cjl.errorCounter
}

type LicCollector struct {
	fetcher        SlurmMetricFetcher[LicenseMetric]
	licTotal       *prometheus.Desc
	licUsed        *prometheus.Desc
	licFree        *prometheus.Desc
	licReserved    *prometheus.Desc
	licScrapeError prometheus.Counter
}

func NewLicCollector(config *Config) *LicCollector {
	cliOpts := config.cliOpts
	fetcher := &CliJsonLicMetricFetcher{
		scraper: NewCliFetcher(cliOpts.lic...),
		cache:   NewAtomicThrottledCache[LicenseMetric](config.pollLimit),
		errorCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "slurm_lic_scrape_error",
			Help: "slurm license scrape error",
		}),
	}
	return &LicCollector{
		fetcher:     fetcher,
		licTotal:    prometheus.NewDesc("slurm_lic_total", "slurm license total", []string{"name"}, nil),
		licUsed:     prometheus.NewDesc("slurm_lic_used", "slurm license used", []string{"name"}, nil),
		licFree:     prometheus.NewDesc("slurm_lic_free", "slurm license free", []string{"name"}, nil),
		licReserved: prometheus.NewDesc("slurm_lic_reserved", "slurm license reserved", []string{"name"}, nil),
		licScrapeError: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "slurm_lic_scrape_error",
			Help: "slurm license scrape error",
		}),
	}
}

func (lc *LicCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- lc.licTotal
	ch <- lc.licUsed
	ch <- lc.licFree
	ch <- lc.licReserved
	ch <- lc.licScrapeError.Desc()
}

func (lc *LicCollector) Collect(ch chan<- prometheus.Metric) {
	defer func() {
		ch <- lc.licScrapeError
	}()
	licMetrics, err := lc.fetcher.FetchMetrics()
	if err != nil {
		lc.licScrapeError.Inc()
		slog.Error(fmt.Sprintf("lic parse error %q", err))
		return
	}
	for _, lic := range licMetrics {
		if lic.Total > 0 {
			ch <- prometheus.MustNewConstMetric(lc.licTotal, prometheus.GaugeValue, float64(lic.Total), lic.LicenseName)
		}
		if lic.Free > 0 {
			ch <- prometheus.MustNewConstMetric(lc.licFree, prometheus.GaugeValue, float64(lic.Free), lic.LicenseName)
		}
		if lic.Used > 0 {
			ch <- prometheus.MustNewConstMetric(lc.licUsed, prometheus.GaugeValue, float64(lic.Used), lic.LicenseName)
		}
		if lic.Reserved > 0 {
			ch <- prometheus.MustNewConstMetric(lc.licReserved, prometheus.GaugeValue, float64(lic.Reserved), lic.LicenseName)
		}
	}
}
