// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
package cext

import (
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rivosinc/prometheus-slurm-exporter/exporter"
	"golang.org/x/exp/slog"
)

func InitPromServer(config *exporter.Config) (http.Handler, []Destructor) {
	textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.LogLevel,
	})
	slog.SetDefault(slog.New(textHandler))
	nodeCollector := exporter.NewNodeCollecter(config)
	cNodeFetcher := NewNodeFetcher(config.PollLimit)
	nodeCollector.SetFetcher(cNodeFetcher)
	prometheus.MustRegister(nodeCollector)
	CJobFetcher := NewJobFetcher(config.PollLimit)
	jobCollector := exporter.NewJobsController(config)
	jobCollector.SetFetcher(CJobFetcher)
	prometheus.MustRegister(jobCollector)
	return promhttp.Handler(), []Destructor{cNodeFetcher, CJobFetcher}
}
