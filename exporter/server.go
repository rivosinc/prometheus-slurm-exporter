// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
)

type CliOpts struct {
	sinfo         []string
	squeue        []string
	sacctmgr      []string
	lic           []string
	sdiag         []string
	licEnabled    bool
	diagsEnabled  bool
	fallback      bool
	sacctEnabled  bool
	excludeFilter *regexp.Regexp
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
	SlurmLicEnabled           bool
	SlurmDiagEnabled          bool
	SlurmCliFallback          bool
	TraceEnabled              bool
	SacctEnabled              bool
	SlurmPollLimit            float64
	LogLevel                  string
	ListenAddress             string
	MetricsPath               string
	SlurmSqueueOverride       string
	SlurmSinfoOverride        string
	SlurmDiagOverride         string
	SlurmAcctOverride         string
	TraceRate                 uint64
	TracePath                 string
	SlurmLicenseOverride      string
	MetricsExcludeFilterRegex string
}

var logLevelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func NewConfig(cliFlags *CliFlags) (*Config, error) {
	// defaults
	compiledExcludeRegex, err := regexp.Compile(cliFlags.MetricsExcludeFilterRegex)
	if err != nil {
		return nil, err
	}
	cliOpts := CliOpts{
		squeue:        []string{"squeue", "--json"},
		sinfo:         []string{"sinfo", "--json"},
		lic:           []string{"scontrol", "show", "lic", "--json"},
		sdiag:         []string{"sdiag", "--json"},
		sacctmgr:      []string{"sacctmgr", "show", "assoc", "format=User,Account,GrpCPU,GrpMem,GrpJobs,GrpSubmit", "--noheader", "--parsable2"},
		licEnabled:    cliFlags.SlurmLicEnabled,
		diagsEnabled:  cliFlags.SlurmDiagEnabled,
		fallback:      cliFlags.SlurmCliFallback,
		sacctEnabled:  cliFlags.SacctEnabled,
		excludeFilter: compiledExcludeRegex,
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
	if cliFlags.SlurmDiagOverride != "" {
		cliOpts.sdiag = strings.Split(cliFlags.SlurmDiagOverride, " ")
	}
	if cliFlags.SlurmAcctOverride != "" {
		cliOpts.sacctmgr = strings.Split(cliFlags.SlurmAcctOverride, " ")
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
	if cliOpts.fallback {
		// we define a custom json format that we convert back into the openapi format
		if cliFlags.SlurmSqueueOverride == "" {
			cliOpts.squeue = []string{"squeue", "--states=all", "-h", "-r", "-o", `{"a": "%a", "id": %A, "end_time": "%e", "u": "%u", "state": "%T", "p": "%P", "cpu": %C, "mem": "%m", "array_id": "%K", "r": "%R"}`}
		}
		if cliFlags.SlurmSinfoOverride == "" {
			// set field lengths wide enough to avoid truncation
			cliOpts.sinfo = []string{"sinfo", "-h", "-O", "StateCompact:12|,Memory:15|,NodeHost:30|,CPUsLoad:12|,Partition:15|,FreeMem:15|,CPUsState:15|,Weight:10|,AllocMem:15"}
		}
		// must instantiate the job fetcher here since it is shared between 2 collectors
		traceConf.sharedFetcher = &JobCliFallbackFetcher{
			scraper: NewCliScraper(cliOpts.squeue...),
			cache:   NewAtomicThrottledCache[JobMetric](config.PollLimit),
			errCounter: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "job_scrape_errors",
				Help: "job scrape errors",
			}),
		}
	} else {
		traceConf.sharedFetcher = &JobJsonFetcher{
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

func NewPromHTTPServer(metricsExcludeFilter *regexp.Regexp) http.Handler {
	// Create a handler that filters metrics based on the exclude regex pattern
	if metricsExcludeFilter == nil || metricsExcludeFilter.String() == "" {
		return promhttp.Handler()
	}
	slog.Info("filtering metrics based on regex: " + metricsExcludeFilter.String())
	filteredGatherer := prometheus.GathererFunc(func() ([]*dto.MetricFamily, error) {
		allMetrics, err := prometheus.DefaultGatherer.Gather()
		if err != nil {
			return nil, err
		}
		var filteredMetrics []*dto.MetricFamily
		for _, mf := range allMetrics {
			if !metricsExcludeFilter.MatchString(mf.GetName()) {
				filteredMetrics = append(filteredMetrics, mf)
			}
		}
		return filteredMetrics, nil
	})
	return promhttp.HandlerFor(filteredGatherer, promhttp.HandlerOpts{})
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
	if cliOpts.sacctEnabled {
		slog.Info("account limit collection enabled")
		prometheus.MustRegister(NewLimitCollector(config))
	}

	return NewPromHTTPServer(cliOpts.excludeFilter)
}
