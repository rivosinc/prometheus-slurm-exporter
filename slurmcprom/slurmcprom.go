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
	"github.com/rivosinc/prometheus-slurm-exporter/exporter"
)

type CMetricFetcher[M exporter.SlurmPrimitiveMetric] interface {
	exporter.SlurmMetricFetcher[M]
	Deinit()
}

type CNodeFetcher struct {
	cache        *exporter.AtomicThrottledCache[exporter.NodeMetric]
	scraper      NodeMetricScraper
	duration     time.Duration
	errorCounter prometheus.Counter
}

// should be defer'd immediately after new cmd to prevent mem leaks
func (cni *CNodeFetcher) Deinit() {
	DeleteNodeMetricScraper(cni.scraper)
}

func (cni *CNodeFetcher) CToGoMetricConvert() ([]exporter.NodeMetric, error) {
	if errno := cni.scraper.CollectNodeInfo(); errno != 0 {
		cni.errorCounter.Inc()
		return nil, fmt.Errorf("Node Info CPP errno: %d", errno)
	}
	cni.scraper.IterReset()
	nodeMetrics := make([]exporter.NodeMetric, 0)
	metric := NewPromNodeMetric()
	defer DeletePromNodeMetric(metric)
	nodeStates := map[uint64]string{
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

	now := time.Now()
	for cni.scraper.IterNext(metric) == 0 {
		nodeMetric := exporter.NodeMetric{
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
	cni.duration = time.Since(now)
	return nodeMetrics, nil
}

func (cni *CNodeFetcher) FetchMetrics() ([]exporter.NodeMetric, error) {
	return cni.cache.FetchOrThrottle(cni.CToGoMetricConvert)
}

func (cni *CNodeFetcher) ScrapeDuration() time.Duration {
	return cni.duration
}

func (cni *CNodeFetcher) ScrapeError() prometheus.Counter {
	return cni.errorCounter
}

func NewNodeFetcher(pollLimit float64) *CNodeFetcher {
	return &CNodeFetcher{
		cache:   exporter.NewAtomicThrottledCache[exporter.NodeMetric](pollLimit),
		scraper: NewNodeMetricScraper(""),
		errorCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "slurm_cplugin_node_fetch_error",
			Help: "slurm cplugin fetch error",
		}),
	}
}
