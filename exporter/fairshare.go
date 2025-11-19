package exporter

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
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

func (fsf *FairShareFetcher) parseFairShare(output string) ([]FairShareMetric, error) {
	var metrics []FairShareMetric
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		// Check if line contains data
		if !strings.Contains(line, "|") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}

		account := strings.TrimSpace(parts[0])
		if account == "" {
			continue
		}

		fairshareStr := strings.TrimSpace(parts[1])
		// Skip empty values and "inf" values
		if fairshareStr == "" || fairshareStr == "inf" {
			continue
		}

		fairshare, err := strconv.ParseFloat(fairshareStr, 64)
		if err != nil {
			slog.Warn("Failed to parse fairshare value", "account", account, "value", fairshareStr, "err", err)
			continue
		}

		metrics = append(metrics, FairShareMetric{
			Account:   account,
			FairShare: fairshare,
		})
	}

	return metrics, nil
}

func (fsf *FairShareFetcher) fetchFromCli() ([]FairShareMetric, error) {
	output, err := fsf.scraper.FetchRawBytes()
	if err != nil {
		fsf.errorCounter.Inc()
		slog.Error(fmt.Sprintf("failed to scrape fairshare metrics with %q", err))
		return nil, err
	}

	return fsf.parseFairShare(string(output))
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
	return &FairShareCollector{
		fetcher: &FairShareFetcher{
			scraper: NewCliScraper("sshare", "-n", "-P", "-o", "account,levelfs"),
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
