package slurmcprom

// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

import "C"

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	psm "github.com/rivosinc/prometheus-slurm-exporter"
)

type CNodeFetcher struct {
	cache        *psm.AtomicThrottledCache[psm.NodeMetric]
	scraper      NodeMetricScraper
	duration     time.Duration
	errorCounter prometheus.Counter
}

func (cni *CNodeFetcher) NodeMetricConvert() ([]psm.NodeMetric, error) {
	if errno := cni.scraper.CollectNodeInfo(); errno != 0 {
		cni.errorCounter.Inc()
		return nil, fmt.Errorf("Node Info CPP errno: %d", errno)
	}
	nodeMetrics := make([]psm.NodeMetric, 0)
	return nodeMetrics, nil
}

func (cni *CNodeFetcher) FetchMetrics() ([]psm.NodeMetric, error) {
	return nil, nil
}

func NewNodeFetcher() *CNodeFetcher {
	return &CNodeFetcher{
		cache:   psm.NewAtomicThrottledCache[psm.NodeMetric](1),
		scraper: NewNodeMetricScraper(""),
		errorCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "slurm_cplugin_node_fetch_error",
			Help: "slurm cplugin fetch error",
		}),
	}
}
