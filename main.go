package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/exp/slog"
)

var (
	listenAddress = flag.String("web.listen-address", "",
		"Address to listen on for telemetry (default: 9092)")
	metricsPath = flag.String("web.telemetry-path", "",
		"Path under which to expose metrics (default: /metrics)")
	logLevel       = flag.String("web.log-level", "", "Log level: info, debug, error, warning")
	traceEnabled   = flag.Bool("trace.enabled", false, "Set up Post endpoint for collecting traces")
	tracePath      = flag.String("trace.path", "/trace", "POST path to upload job proc info")
	traceRate      = flag.Uint64("trace.rate", 0, "number of seconds proc info should stay in memory before being marked as stale")
	slurmPollLimit = flag.Float64("slurm.poll-limit", 0, "throttle for slurmctld (default: 10s)")
	logLevelMap    = map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
)

type TraceConfig struct {
	enabled       bool
	path          string
	rate          uint64
	sharedFetcher SlurmFetcher
}

type Config struct {
	traceConf     *TraceConfig
	pollLimit     float64
	logLevel      slog.Level
	listenAddress string
	metricsPath   string
}

func NewConfig() (*Config, error) {
	// defaults
	sharedFetcher := NewCliFetcher("squeue", "--json")
	config := &Config{
		pollLimit:     10,
		logLevel:      slog.LevelInfo,
		listenAddress: ":9092",
		metricsPath:   "/metrics",
		traceConf: &TraceConfig{
			enabled:       *traceEnabled,
			path:          *tracePath,
			rate:          *traceRate,
			sharedFetcher: sharedFetcher,
		},
	}
	if lm, ok := os.LookupEnv("POLL_LIMIT"); ok {
		if limit, err := strconv.ParseFloat(lm, 64); err != nil {
			return nil, err
		} else {
			config.pollLimit = limit
		}
	}
	if *slurmPollLimit > 0 {
		config.pollLimit = *slurmPollLimit
	}
	if lvl, ok := os.LookupEnv("LOGLEVEL"); ok {
		config.logLevel = logLevelMap[strings.ToLower(lvl)]
	}
	if *logLevel != "" {
		config.logLevel = logLevelMap[*logLevel]
	}
	if *listenAddress != "" {
		config.listenAddress = *listenAddress
	}
	if *metricsPath != "" {
		config.metricsPath = *metricsPath
	}
	sharedFetcher.cache = NewAtomicThrottledCache(config.pollLimit)
	return config, nil
}

func (c *Config) SetFetcher(fetcher SlurmFetcher) {
	c.traceConf.sharedFetcher = fetcher
}

func main() {
	config, err := NewConfig()
	if err != nil {
		slog.Error(fmt.Sprintf("invalid configuration: %q", err))
		return
	}
	textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.logLevel,
	})
	slog.SetDefault(slog.New(textHandler))
	flag.Parse()
	prometheus.MustRegister(NewNodeCollecter(config), NewJobsController(config))
	if *traceEnabled {
		slog.Info("trace path enabled at path: " + *listenAddress + *tracePath)
		traceController := NewTraceController(config)
		http.HandleFunc(*tracePath, traceController.uploadTrace)
		prometheus.MustRegister(traceController)
	}
	http.Handle(*metricsPath, promhttp.Handler())
	slog.Info("serving metrics at " + *listenAddress + *metricsPath)
	slog.Error(fmt.Sprintf("server exited with %q", http.ListenAndServe(*listenAddress, nil)))
}
