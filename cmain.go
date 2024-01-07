//go:build cenabled

// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/rivosinc/prometheus-slurm-exporter/cext"
	"github.com/rivosinc/prometheus-slurm-exporter/exporter"
	"golang.org/x/exp/slog"
)

var (
	listenAddress = flag.String("web.listen-address", "",
		`Address to listen on for telemetry "(default: :9092)"`)
	metricsPath = flag.String("web.telemetry-path", "",
		"Path under which to expose metrics (default: /metrics)")
	logLevel = flag.String("web.log-level", "", "Log level: info, debug, error, warning")
)

func main() {
	flag.Parse()
	cliArgs := exporter.CliFlags{
		ListenAddress: *listenAddress,
		MetricsPath:   *metricsPath,
		LogLevel:      *logLevel,
	}
	config, err := exporter.NewConfig(&cliArgs)
	if err != nil {
		log.Fatalf("failed to init config with %q", err)
	}
	handler, fetchersToFree := cext.InitPromServer(config)
	defer func() {
		for _, fetcher := range fetchersToFree {
			fetcher.Deinit()
		}
	}()
	http.Handle(config.MetricsPath, handler)
	slog.Info("serving metrics at " + config.ListenAddress + config.MetricsPath)
	log.Fatalf("server exited with %q", http.ListenAndServe(config.ListenAddress, nil))
}
