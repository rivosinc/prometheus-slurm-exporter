package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/exp/slog"
)

var (
	listenAddress = flag.String("web.listen-address", ":8092",
		"Address to listen on for telemetry")
	metricsPath = flag.String("web.telemetry-path", "/metrics",
		"Path under which to expose metrics")
	traceEnabled = flag.Bool("trace.enabled", false, "Set up Post endpoint for collecting traces")
	tracePath    = flag.String("trace.path", "/trace", "POST path to upload job proc info")
	traceRate    = flag.Uint64("trace.rate", 10, "number of seconds proc info should stay in memory before being marked as stale")
)

func main() {
	logLevel := "info"
	if lvl, ok := os.LookupEnv("LOGLEVEL"); ok {
		logLevel = strings.ToLower(lvl)
	}
	logLevelMap := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	opts := slog.HandlerOptions{
		Level: logLevelMap[logLevel],
	}
	textHandler := slog.NewTextHandler(os.Stdout, &opts)
	slog.SetDefault(slog.New(textHandler))
	flag.Parse()
	jobsCollector := NewJobsController()
	prometheus.MustRegister(NewNodeCollecter(), jobsCollector)
	if *traceEnabled {
		slog.Info("trace path enabled at path: " + *listenAddress + *tracePath)
		traceController := NewTraceController(*traceRate, jobsCollector)
		http.HandleFunc(*tracePath, traceController.uploadTrace)
		prometheus.MustRegister(traceController)
	}
	http.Handle(*metricsPath, promhttp.Handler())
	slog.Info("serving metrics at " + *listenAddress + *metricsPath)
	slog.Error(fmt.Sprintf("server exited with %q", http.ListenAndServe(*listenAddress, nil)))
}
