// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
package slurmcprom

import (
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	psm "github.com/rivosinc/prometheus-slurm-exporter/exporter"
	"golang.org/x/exp/slog"
)

func initPromServer(config *psm.Config) http.Handler {
	textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.LogLevel,
	})
	slog.SetDefault(slog.New(textHandler))
	nodeCollector := psm.NewNodeCollecter(config)
	cNodeFetcher := NewNodeFetcher(config.PollLimit)
	nodeCollector.SetFetcher(cNodeFetcher)
	prometheus.MustRegister()
	return promhttp.Handler()
}
