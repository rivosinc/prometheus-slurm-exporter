package exporter

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// AccountCredits represents credits information for a Slurm account
type AccountCredits struct {
	Account    string
	Allocation float64
	Remaining  float64
	Used       float64
	UsedPct    float64
}

// CliTextCreditsFetcher fetches and parses scredits text output
type CliTextCreditsFetcher struct {
	scraper      SlurmByteScraper
	cache        *AtomicThrottledCache[AccountCredits]
	errorCounter prometheus.Counter
}

func (ctf *CliTextCreditsFetcher) fetch() ([]AccountCredits, error) {
	data, err := ctf.scraper.FetchRawBytes()
	if err != nil {
		slog.Error(fmt.Sprintf("fetch error %q", err))
		ctf.errorCounter.Inc()
		return nil, err
	}
	
	credits, err := parseCredits(data)
	if err != nil {
		slog.Error(fmt.Sprintf("parse error %q", err))
		ctf.errorCounter.Inc()
		return nil, err
	}
	
	return credits, nil
}

func (ctf *CliTextCreditsFetcher) FetchMetrics() ([]AccountCredits, error) {
	return ctf.cache.FetchOrThrottle(ctf.fetch)
}

func (ctf *CliTextCreditsFetcher) ScrapeDuration() time.Duration {
	return ctf.cache.duration
}

func (ctf *CliTextCreditsFetcher) ScrapeError() prometheus.Counter {
	return ctf.errorCounter
}

// CreditsCollector collects Slurm account credits/billing metrics
type CreditsCollector struct {
	fetcher              SlurmMetricFetcher[AccountCredits]
	accountAllocation    *prometheus.Desc
	accountRemaining     *prometheus.Desc
	accountUsed          *prometheus.Desc
	accountUsedPercent   *prometheus.Desc
	scrapeDuration       *prometheus.Desc
	scrapeError          prometheus.Counter
}

// NewCreditsCollector creates a new CreditsCollector
func NewCreditsCollector(config *Config) *CreditsCollector {
	cliOpts := config.cliOpts
	fetcher := &CliTextCreditsFetcher{
		scraper: NewCliScraper(cliOpts.scredits...),
		cache:   NewAtomicThrottledCache[AccountCredits](config.PollLimit),
		errorCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "slurm_credits_scrape_error",
			Help: "slurm credits scrape error",
		}),
	}
	
	return &CreditsCollector{
		fetcher: fetcher,
		accountAllocation: prometheus.NewDesc(
			"slurm_scredits_allocation_total",
			"Total allocation in SU per account",
			[]string{"account"},
			nil,
		),
		accountRemaining: prometheus.NewDesc(
			"slurm_scredits_remaining",
			"Remaining credits in SU per account",
			[]string{"account"},
			nil,
		),
		accountUsed: prometheus.NewDesc(
			"slurm_scredits_used",
			"Used credits in SU per account",
			[]string{"account"},
			nil,
		),
		accountUsedPercent: prometheus.NewDesc(
			"slurm_scredits_used_percent",
			"Percentage of credits used per account",
			[]string{"account"},
			nil,
		),
		scrapeDuration: prometheus.NewDesc(
			"slurm_scredits_scrape_duration",
			"Time taken to scrape credits data (ms)",
			nil,
			nil,
		),
		scrapeError: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "slurm_credits_scrape_error",
			Help: "Slurm credits scrape errors",
		}),
	}
}

// Describe implements prometheus.Collector
func (cc *CreditsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- cc.accountAllocation
	ch <- cc.accountRemaining
	ch <- cc.accountUsed
	ch <- cc.accountUsedPercent
	ch <- cc.scrapeDuration
	cc.scrapeError.Describe(ch)
}

// Collect implements prometheus.Collector
func (cc *CreditsCollector) Collect(ch chan<- prometheus.Metric) {
	defer func() {
		ch <- cc.scrapeError
	}()
	
	credits, err := cc.fetcher.FetchMetrics()
	if err != nil {
		cc.scrapeError.Inc()
		slog.Error(fmt.Sprintf("Failed to fetch credits: %v", err))
		return
	}

	duration := cc.fetcher.ScrapeDuration().Milliseconds()
	ch <- prometheus.MustNewConstMetric(cc.scrapeDuration, prometheus.GaugeValue, float64(duration))

	for _, ac := range credits {
		ch <- prometheus.MustNewConstMetric(cc.accountAllocation, prometheus.GaugeValue, ac.Allocation, ac.Account)
		ch <- prometheus.MustNewConstMetric(cc.accountRemaining, prometheus.GaugeValue, ac.Remaining, ac.Account)
		ch <- prometheus.MustNewConstMetric(cc.accountUsed, prometheus.GaugeValue, ac.Used, ac.Account)
		ch <- prometheus.MustNewConstMetric(cc.accountUsedPercent, prometheus.GaugeValue, ac.UsedPct, ac.Account)
	}
}

// parseCredits parses the text output from scredits command
func parseCredits(data []byte) ([]AccountCredits, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var credits []AccountCredits
	inDataSection := false

	for scanner.Scan() {
		line := scanner.Text()

		// Skip header lines until we find the separator
		if strings.HasPrefix(line, "---") {
			inDataSection = true
			continue
		}

		if !inDataSection {
			continue
		}

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse data line: "Account | Allocation(SU) | Remaining(SU) | Used(SU) | Used(%) |"
		fields := strings.Split(line, "|")
		if len(fields) < 5 {
			continue
		}

		account := strings.TrimSpace(fields[0])
		if account == "" || account == "Account" {
			continue
		}

		allocation, err := parseFloat(fields[1])
		if err != nil {
			slog.Warn(fmt.Sprintf("Failed to parse allocation for account %s: %v", account, err))
			continue
		}

		remaining, err := parseFloat(fields[2])
		if err != nil {
			slog.Warn(fmt.Sprintf("Failed to parse remaining for account %s: %v", account, err))
			continue
		}

		used, err := parseFloat(fields[3])
		if err != nil {
			slog.Warn(fmt.Sprintf("Failed to parse used for account %s: %v", account, err))
			continue
		}

		usedPct, err := parseFloat(fields[4])
		if err != nil {
			slog.Warn(fmt.Sprintf("Failed to parse used percent for account %s: %v", account, err))
			continue
		}

		credits = append(credits, AccountCredits{
			Account:    account,
			Allocation: allocation,
			Remaining:  remaining,
			Used:       used,
			UsedPct:    usedPct,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading credits data: %w", err)
	}

	return credits, nil
}

// parseFloat parses a float from a string, handling empty strings
func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}
