// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"golang.org/x/exp/slog"

	"github.com/rivosinc/prometheus-slurm-exporter/exporter"
)

var (
	listenAddress = flag.String("web.listen-address", "",
		`Address to listen on for telemetry "(default: :9092)"`)
	metricsPath = flag.String("web.telemetry-path", "",
		"Path under which to expose metrics (default: /metrics)")
	logLevel             = flag.String("web.log-level", "", "Log level: info, debug, error, warning")
	traceEnabled         = flag.Bool("trace.enabled", false, "Set up Post endpoint for collecting traces")
	tracePath            = flag.String("trace.path", "", "POST path to upload job proc info")
	traceRate            = flag.Uint64("trace.rate", 0, "number of seconds proc info should stay in memory before being marked as stale (default 10)")
	slurmPollLimit       = flag.Float64("slurm.poll-limit", 0, "throttle for slurmctld (default: 10s)")
	slurmSinfoOverride   = flag.String("slurm.sinfo-cli", "", "sinfo cli override")
	slurmSqueueOverride  = flag.String("slurm.squeue-cli", "", "squeue cli override")
	slurmLicenseOverride = flag.String("slurm.lic-cli", "", "squeue cli override")
	slurmDiagOverride    = flag.String("slurm.diag-cli", "", "sdiag cli override")
	slurmLicEnabled      = flag.Bool("slurm.collect-licenses", false, "Collect license info from slurm")
	slurmDiagEnabled     = flag.Bool("slurm.collect-diags", false, "Collect daemon diagnostics stats from slurm")
	slurmCliFallback     = flag.Bool("slurm.cli-fallback", false, "drop the --json arg and revert back to standard squeue for performance reasons")
	logLevelMap          = map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
)

func main() {
	cliFlags := exporter.CliFlags{
		ListenAddress:        *listenAddress,
		MetricsPath:          *metricsPath,
		LogLevel:             *logLevel,
		TraceEnabled:         *traceEnabled,
		TracePath:            *tracePath,
		SlurmPollLimit:       *slurmPollLimit,
		SlurmSinfoOverride:   *slurmSinfoOverride,
		SlurmSqueueOverride:  *slurmSqueueOverride,
		SlurmLicenseOverride: *slurmLicenseOverride,
		SlurmDiagOverride:    *slurmDiagOverride,
		SlurmLicEnabled:      *slurmLicEnabled,
		SlurmDiagEnabled:     *slurmDiagEnabled,
		SlurmCliFallback:     *slurmCliFallback,
	}
	config, err := exporter.NewConfig(&cliFlags)
	if err != nil {
		log.Fatalf("failed to init config with %q", err)
	}
	handler := exporter.InitPromServer(config)
	fmt.Println("Hello World")
	http.Handle(config.MetricsPath, handler)
	slog.Info("serving metrics at " + config.ListenAddress + config.MetricsPath)
	log.Fatalf("server exited with %q", http.ListenAndServe(config.ListenAddress, nil))

}
