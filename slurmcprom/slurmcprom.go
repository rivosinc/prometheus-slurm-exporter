package slurmcprom

// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

import "C"

import (
	"fmt"
	"strings"
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

// should be defer'd immediately after new cmd to prevent mem leaks
func (cni *CNodeFetcher) Deinit() {
	DeleteNodeMetricScraper(cni.scraper)
}

func (cni *CNodeFetcher) NodeMetricConvert() ([]psm.NodeMetric, error) {
	if errno := cni.scraper.CollectNodeInfo(); errno != 0 {
		cni.errorCounter.Inc()
		return nil, fmt.Errorf("Node Info CPP errno: %d", errno)
	}
	cni.scraper.IterReset()
	nodeMetrics := make([]psm.NodeMetric, 0)
	metric := NewPromNodeMetric()
	defer DeletePromNodeMetric(metric)
	nodeStates := map[uint]string{
		0: "UNKNOWN",
		1: "DOWN",
		2: "IDLE",
		3: "ALLOCATED",
		4: "ERROR",
		5: "MIXED",
		6: "FUTURE",
		// used by the C api to detect end of enum. Shouldn't ever be emitted
		7: "END",
	}
	for cni.scraper.IterNext(metric) == 0 {
		nodeMetric := psm.NodeMetric{
			Hostname:    metric.GetHostname(),
			Cpus:        float64(metric.GetCpus()),
			RealMemory:  float64(metric.GetRealMemory()),
			FreeMemory:  float64(metric.GetFreeMem()),
			Partitions:  strings.Split(metric.GetPartitions(), ","),
			State:       nodeStates[metric.GetNodeState()],
			AllocMemory: float64(metric.GetAllocMem()),
			AllocCpus:   float64(metric.GetAllocCpus()),
			IdleCpus:    float64(metric.GetCpus()) - float64(metric.GetAllocCpus()),
			Weight:      float64(metric.GetWeight()),
			CpuLoad:     float64(metric.GetCpuLoad()),
		}
		nodeMetrics = append(nodeMetrics, nodeMetric)
	}

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
