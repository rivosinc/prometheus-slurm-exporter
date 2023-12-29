// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/exp/slog"
)

type CliOpts struct {
	sinfo        []string
	squeue       []string
	lic          []string
	sdiag        []string
	licEnabled   bool
	diagsEnabled bool
	fallback     bool
}

type TraceConfig struct {
	enabled       bool
	path          string
	rate          uint64
	sharedFetcher SlurmMetricFetcher[JobMetric]
}

type Config struct {
	TraceConf     *TraceConfig
	PollLimit     float64
	LogLevel      slog.Level
	ListenAddress string
	MetricsPath   string
	cliOpts       *CliOpts
}

type CliFlags struct {
	SlurmLicEnabled      bool
	SlurmDiagEnabled     bool
	SlurmCliFallback     bool
	TraceEnabled         bool
	SlurmPollLimit       float64
	LogLevel             string
	ListenAddress        string
	MetricsPath          string
	SlurmSqueueOverride  string
	SlurmSinfoOverride   string
	SlurmDiagOverride    string
	TraceRate            uint64
	TracePath            string
	SlurmLicenseOverride string
}

var logLevelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func NewConfig(cliFlags *CliFlags) (*Config, error) {
	// defaults
	cliOpts := CliOpts{
		squeue:       []string{"squeue", "--json"},
		sinfo:        []string{"sinfo", "--json"},
		lic:          []string{"scontrol", "show", "lic", "--json"},
		sdiag:        []string{"sdiag", "--json"},
		licEnabled:   cliFlags.SlurmLicEnabled,
		diagsEnabled: cliFlags.SlurmDiagEnabled,
		fallback:     cliFlags.SlurmCliFallback,
	}
	traceConf := TraceConfig{
		enabled: cliFlags.TraceEnabled,
		path:    "/trace",
		rate:    10,
	}
	config := &Config{
		PollLimit:     10,
		LogLevel:      slog.LevelInfo,
		ListenAddress: ":9092",
		MetricsPath:   "/metrics",
		TraceConf:     &traceConf,
		cliOpts:       &cliOpts,
	}
	if lm, ok := os.LookupEnv("POLL_LIMIT"); ok {
		if limit, err := strconv.ParseFloat(lm, 64); err != nil {
			return nil, err
		} else {
			config.PollLimit = limit
		}
	}
	if cliFlags.SlurmPollLimit > 0 {
		config.PollLimit = cliFlags.SlurmPollLimit
	}
	if lvl, ok := os.LookupEnv("LOGLEVEL"); ok {
		config.LogLevel = logLevelMap[lvl]
	}
	if cliFlags.LogLevel != "" {
		config.LogLevel = logLevelMap[cliFlags.LogLevel]
	}
	if cliFlags.ListenAddress != "" {
		config.ListenAddress = cliFlags.ListenAddress
	}
	if cliFlags.MetricsPath != "" {
		config.MetricsPath = cliFlags.MetricsPath
	}
	if cliFlags.SlurmSqueueOverride != "" {
		cliOpts.squeue = strings.Split(cliFlags.SlurmSqueueOverride, " ")
	}
	if cliFlags.SlurmSinfoOverride != "" {
		cliOpts.sinfo = strings.Split(cliFlags.SlurmSinfoOverride, " ")
	}
	if cliFlags.SlurmSinfoOverride != "" {
		cliOpts.sdiag = strings.Split(cliFlags.SlurmSinfoOverride, " ")
	}
	if cliFlags.TraceRate != 0 {
		traceConf.rate = cliFlags.TraceRate
	}
	if cliFlags.TracePath != "" {
		traceConf.path = cliFlags.TracePath
	}
	if cliFlags.SlurmLicenseOverride != "" {
		cliOpts.lic = strings.Split(cliFlags.SlurmLicenseOverride, " ")
	}
	traceConf.sharedFetcher = &JobJsonFetcher{
		scraper: NewCliScraper(cliOpts.squeue...),
		cache:   NewAtomicThrottledCache[JobMetric](config.PollLimit),
		errCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "job_scrape_errors",
			Help: "job scrape errors",
		}),
	}
	if cliOpts.fallback {
		// we define a custom json format that we convert back into the openapi format
		cliOpts.squeue = []string{"squeue", "--states=all", "-h", "-o", `{"a": "%a", "id": %A, "end_time": "%e", "u": "%u", "state": "%T", "p": "%P", "cpu": %C, "mem": "%m"}`}
		cliOpts.sinfo = []string{"sinfo", "-h", "-o", `{"s": "%T", "mem": %m, "n": "%n", "l": "%O", "p": "%R", "fmem": "%e", "cstate": "%C", "w": %w}`}
		traceConf.sharedFetcher = &JobCliFallbackFetcher{
			scraper: NewCliScraper(cliOpts.squeue...),
			cache:   NewAtomicThrottledCache[JobMetric](config.PollLimit),
			errCounter: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "job_scrape_errors",
				Help: "job scrape errors",
			}),
		}
	}
	return config, nil
}

func InitPromServer(config *Config) http.Handler {
	textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.LogLevel,
	})
	slog.SetDefault(slog.New(textHandler))
	prometheus.MustRegister(NewNodeCollecter(config), NewJobsController(config))
	if traceconf := config.TraceConf; traceconf.enabled {
		slog.Info("trace path enabled at path: " + config.ListenAddress + traceconf.path)
		traceController := NewTraceCollector(config)
		http.HandleFunc(traceconf.path, traceController.uploadTrace)
		prometheus.MustRegister(traceController)
	}
	cliOpts := config.cliOpts
	if cliOpts.licEnabled {
		slog.Info("licence collection enabled")
		prometheus.MustRegister(NewLicCollector(config))
	}
	if cliOpts.diagsEnabled {
		slog.Info("daemon diagnostic collection enabled")
		prometheus.MustRegister(NewDiagsCollector(config))
	}
	return promhttp.Handler()
}
