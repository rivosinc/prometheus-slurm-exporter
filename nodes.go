// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog"
)

type NodeMetric struct {
	Hostname    string   `json:"hostname"`
	Cpus        float64  `json:"cpus"`
	RealMemory  float64  `json:"real_memory"`
	FreeMemory  float64  `json:"free_memory"`
	Partitions  []string `json:"partitions"`
	State       string   `json:"state"`
	AllocMemory float64  `json:"alloc_memory"`
	AllocCpus   float64  `json:"alloc_cpus"`
	IdleCpus    float64  `json:"idle_cpus"`
	Weight      float64  `json:"weight"`
	CpuLoad     float64  `json:"cpu_load"`
}

type sinfoResponse struct {
	Meta struct {
		SlurmVersion struct {
			Version struct {
				Major int `json:"major"`
				Micro int `json:"micro"`
				Minor int `json:"minor"`
			} `json:"version"`
			Release string `json:"release"`
		} `json:"Slurm"`
	} `json:"meta"`
	Errors []string     `json:"errors"`
	Nodes  []NodeMetric `json:"nodes"`
}

func parseNodeMetrics(jsonNodeList []byte) ([]NodeMetric, error) {
	squeue := sinfoResponse{}
	err := json.Unmarshal(jsonNodeList, &squeue)
	if err != nil {
		slog.Error("Unmarshaling node metrics %q", err)
		return nil, err
	}
	if len(squeue.Errors) > 0 {
		for _, e := range squeue.Errors {
			slog.Error("Api error response %q", e)
		}
		return nil, errors.New(squeue.Errors[0])
	}
	return squeue.Nodes, nil
}

type NAbleFloat float64

func (naf *NAbleFloat) UnmarshalJSON(data []byte) error {
	var fString string
	if err := json.Unmarshal(data, &fString); err != nil {
		return err
	}
	if fString == "N/A" {
		*naf = 0
		return nil
	}
	var f float64
	if err := json.Unmarshal([]byte(fString), &f); err != nil {
		return err
	}
	*naf = NAbleFloat(f)
	return nil
}

func parseNodeCliFallback(sinfo []byte) ([]NodeMetric, error) {
	nodeMetrics := make(map[string]*NodeMetric, 0)
	for i, line := range bytes.Split(bytes.Trim(sinfo, "\n"), []byte("\n")) {
		var metric struct {
			Hostname   string     `json:"n"`
			RealMemory float64    `json:"mem"`
			FreeMemory NAbleFloat `json:"fmem"`
			CpuState   string     `json:"cstate"`
			Partition  string     `json:"p"`
			CpuLoad    NAbleFloat `json:"l"`
			State      string     `json:"s"`
			Weight     float64    `json:"w"`
		}
		if err := json.Unmarshal(line, &metric); err != nil {
			return nil, fmt.Errorf("sinfo failed to parse line %d: %s, got %q", i, line, err)
		}
		// convert mem units from MB to Bytes
		metric.RealMemory *= 1e6
		metric.FreeMemory *= 1e6
		cpuStates := strings.Split(metric.CpuState, "/")
		if len(cpuStates) != 4 {
			return nil, fmt.Errorf("unexpected cpu state format. Got %s", metric.CpuState)
		}
		allocated, err := strconv.ParseFloat(cpuStates[0], 64)
		if err != nil {
			return nil, err
		}
		idle, err := strconv.ParseFloat(cpuStates[1], 64)
		if err != nil {
			return nil, err
		}
		other, err := strconv.ParseFloat(cpuStates[2], 64)
		if err != nil {
			return nil, err
		}
		total, err := strconv.ParseFloat(cpuStates[3], 64)
		if err != nil {
			return nil, err
		}
		_ = other
		if nodeMetric, ok := nodeMetrics[metric.Hostname]; ok {
			nodeMetric.Partitions = append(nodeMetric.Partitions, metric.Partition)
			states := strings.Split(nodeMetric.State, "&")
			if !slices.Contains(states, metric.State) {
				// nodes can have multiple states. Our query puts them on separate lines
				nodeMetric.State += "&" + metric.State
			}
		} else {
			nodeMetrics[metric.Hostname] = &NodeMetric{
				Hostname:    metric.Hostname,
				Cpus:        total,
				RealMemory:  metric.RealMemory,
				FreeMemory:  float64(metric.FreeMemory),
				Partitions:  []string{metric.Partition},
				State:       metric.State,
				AllocMemory: metric.RealMemory - float64(metric.FreeMemory),
				AllocCpus:   allocated,
				IdleCpus:    idle,
				Weight:      metric.Weight,
				CpuLoad:     float64(metric.CpuLoad),
			}
		}
	}
	values := make([]NodeMetric, 0)
	for _, val := range nodeMetrics {
		values = append(values, *val)
	}
	return values, nil
}

type PartitionMetric struct {
	Cpus        float64
	RealMemory  float64
	FreeMemory  float64
	AllocMemory float64
	AllocCpus   float64
	CpuLoad     float64
	IdleCpus    float64
	Weight      float64
}

func fetchNodePartitionMetrics(nodes []NodeMetric) map[string]*PartitionMetric {
	partitions := make(map[string]*PartitionMetric)
	for _, node := range nodes {
		for _, p := range node.Partitions {
			partition, ok := partitions[p]
			if !ok {
				partition = new(PartitionMetric)
				partitions[p] = partition
			}
			partition.Cpus += node.Cpus
			partition.RealMemory += node.RealMemory
			partition.FreeMemory += node.FreeMemory
			partition.AllocMemory += node.AllocMemory
			partition.AllocCpus += node.AllocCpus
			partition.IdleCpus += node.IdleCpus
			partition.Weight += node.Weight
			partition.CpuLoad += node.CpuLoad
		}
	}
	return partitions
}

type PerStateMetric struct {
	Cpus  float64
	Count float64
}

type CpuSummaryMetric struct {
	Total    float64
	Idle     float64
	Load     float64
	PerState map[string]*PerStateMetric
}

func fetchNodeTotalCpuMetrics(nodes []NodeMetric) *CpuSummaryMetric {
	cpuSummaryMetrics := &CpuSummaryMetric{
		PerState: make(map[string]*PerStateMetric),
	}
	for _, node := range nodes {
		cpuSummaryMetrics.Total += node.Cpus
		cpuSummaryMetrics.Idle += node.IdleCpus
		cpuSummaryMetrics.Load += node.CpuLoad
		if metric, ok := cpuSummaryMetrics.PerState[node.State]; ok {
			metric.Cpus += node.Cpus
			metric.Count++
		} else {
			cpuSummaryMetrics.PerState[node.State] = &PerStateMetric{Cpus: node.Cpus, Count: 1}
		}
	}
	return cpuSummaryMetrics
}

type MemSummaryMetric struct {
	AllocMemory float64
	FreeMemory  float64
	RealMemory  float64
}

func fetchNodeTotalMemMetrics(nodes []NodeMetric) *MemSummaryMetric {
	memSummary := new(MemSummaryMetric)
	for _, node := range nodes {
		memSummary.AllocMemory += node.AllocMemory
		memSummary.FreeMemory += node.FreeMemory
		memSummary.RealMemory += node.RealMemory
	}
	return memSummary
}

type NodesCollector struct {
	// collector state
	fetcher  SlurmFetcher
	fallback bool
	// partition summary metrics
	partitionCpus        *prometheus.Desc
	partitionRealMemory  *prometheus.Desc
	partitionFreeMemory  *prometheus.Desc
	partitionAllocMemory *prometheus.Desc
	partitionAllocCpus   *prometheus.Desc
	partitionIdleCpus    *prometheus.Desc
	partitionWeight      *prometheus.Desc
	partitionCpuLoad     *prometheus.Desc
	// cpu summary stats
	cpusPerState      *prometheus.Desc
	totalCpus         *prometheus.Desc
	totalIdleCpus     *prometheus.Desc
	totalCpuLoad      *prometheus.Desc
	nodeCountPerState *prometheus.Desc
	// memory summary stats
	totalRealMemory  *prometheus.Desc
	totalFreeMemory  *prometheus.Desc
	totalAllocMemory *prometheus.Desc
	// exporter metrics
	nodeScrapeDuration *prometheus.Desc
	nodeScrapeErrors   prometheus.Counter
}

func NewNodeCollecter(config *Config) *NodesCollector {
	cliOpts := config.cliOpts
	fetcher := NewCliFetcher(cliOpts.sinfo...)
	fetcher.cache = NewAtomicThrottledCache(config.pollLimit)
	return &NodesCollector{
		fetcher:  fetcher,
		fallback: cliOpts.fallback,
		// partition stats
		partitionCpus:        prometheus.NewDesc("slurm_partition_total_cpus", "Total cpus per partition", []string{"partition"}, nil),
		partitionRealMemory:  prometheus.NewDesc("slurm_partition_real_mem", "Real mem per partition", []string{"partition"}, nil),
		partitionFreeMemory:  prometheus.NewDesc("slurm_partition_free_mem", "Free mem per partition", []string{"partition"}, nil),
		partitionAllocMemory: prometheus.NewDesc("slurm_partition_alloc_mem", "Alloc mem per partition", []string{"partition"}, nil),
		partitionAllocCpus:   prometheus.NewDesc("slurm_partition_alloc_cpus", "Alloc cpus per partition", []string{"partition"}, nil),
		partitionIdleCpus:    prometheus.NewDesc("slurm_partition_idle_cpus", "Idle cpus per partition", []string{"partition"}, nil),
		partitionWeight:      prometheus.NewDesc("slurm_partition_weight", "Total node weight per partition??", []string{"partition"}, nil),
		partitionCpuLoad:     prometheus.NewDesc("slurm_partition_cpu_load", "Total cpu load per partition", []string{"partition"}, nil),
		// node cpu summary stats
		totalCpus:         prometheus.NewDesc("slurm_cpus_total", "Total cpus", nil, nil),
		totalIdleCpus:     prometheus.NewDesc("slurm_cpus_idle", "Total idle cpus", nil, nil),
		totalCpuLoad:      prometheus.NewDesc("slurm_cpu_load", "Total cpu load", nil, nil),
		cpusPerState:      prometheus.NewDesc("slurm_cpus_per_state", "Cpus per state i.e alloc, mixed, draining, etc.", []string{"state"}, nil),
		nodeCountPerState: prometheus.NewDesc("slurm_node_count_per_state", "nodes per state", []string{"state"}, nil),
		// node memory summary stats
		totalRealMemory:  prometheus.NewDesc("slurm_mem_real", "Total real mem", nil, nil),
		totalFreeMemory:  prometheus.NewDesc("slurm_mem_free", "Total free mem", nil, nil),
		totalAllocMemory: prometheus.NewDesc("slurm_mem_alloc", "Total alloc mem", nil, nil),
		// exporter stats
		nodeScrapeDuration: prometheus.NewDesc("slurm_node_scrape_duration", fmt.Sprintf("how long the cmd %v took (ms)", cliOpts.sinfo), nil, nil),
		nodeScrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "slurm_node_scrape_error",
			Help: "slurm node info scrape errors",
		}),
	}
}

func (nc *NodesCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.partitionCpus
	ch <- nc.partitionRealMemory
	ch <- nc.partitionFreeMemory
	ch <- nc.partitionAllocMemory
	ch <- nc.partitionAllocCpus
	ch <- nc.partitionIdleCpus
	ch <- nc.partitionWeight
	ch <- nc.partitionCpuLoad
	ch <- nc.totalCpus
	ch <- nc.totalIdleCpus
	ch <- nc.cpusPerState
	ch <- nc.totalRealMemory
	ch <- nc.totalFreeMemory
	ch <- nc.totalAllocMemory
	ch <- nc.nodeScrapeDuration
	ch <- nc.nodeScrapeErrors.Desc()
}

func (nc *NodesCollector) Collect(ch chan<- prometheus.Metric) {
	defer func() {
		ch <- nc.nodeScrapeErrors
	}()
	sinfo, err := nc.fetcher.Fetch()
	if err != nil {
		slog.Error("node fetch error" + err.Error())
		nc.nodeScrapeErrors.Inc()
		return
	}
	ch <- prometheus.MustNewConstMetric(nc.nodeScrapeDuration, prometheus.GaugeValue, float64(nc.fetcher.Duration().Milliseconds()))
	var nodeMetrics []NodeMetric
	if nc.fallback {
		nodeMetrics, err = parseNodeCliFallback(sinfo)
	} else {
		nodeMetrics, err = parseNodeMetrics(sinfo)
	}
	if err != nil {
		nc.nodeScrapeErrors.Inc()
		slog.Error("Failed to parse node metrics: " + err.Error())
		return
	}
	// partition set
	partitionMetrics := fetchNodePartitionMetrics(nodeMetrics)
	for partition, metric := range partitionMetrics {
		if metric.Cpus > 0 {
			ch <- prometheus.MustNewConstMetric(nc.partitionCpus, prometheus.GaugeValue, metric.Cpus, partition)
		}
		if metric.RealMemory > 0 {
			ch <- prometheus.MustNewConstMetric(nc.partitionRealMemory, prometheus.GaugeValue, metric.RealMemory, partition)
		}
		if metric.FreeMemory > 0 {
			ch <- prometheus.MustNewConstMetric(nc.partitionFreeMemory, prometheus.GaugeValue, metric.FreeMemory, partition)
		}
		if metric.AllocMemory > 0 {
			ch <- prometheus.MustNewConstMetric(nc.partitionAllocMemory, prometheus.GaugeValue, metric.AllocMemory, partition)
		}
		if metric.AllocCpus > 0 {
			ch <- prometheus.MustNewConstMetric(nc.partitionAllocCpus, prometheus.GaugeValue, metric.AllocCpus, partition)
		}
		if metric.IdleCpus > 0 {
			ch <- prometheus.MustNewConstMetric(nc.partitionIdleCpus, prometheus.GaugeValue, metric.IdleCpus, partition)
		}
		if metric.Weight > 0 {
			ch <- prometheus.MustNewConstMetric(nc.partitionWeight, prometheus.GaugeValue, metric.Weight, partition)
		}
		if metric.CpuLoad > 0 {
			ch <- prometheus.MustNewConstMetric(nc.partitionCpuLoad, prometheus.GaugeValue, metric.CpuLoad, partition)
		}
	}
	// node cpu summary set
	nodeCpuMetrics := fetchNodeTotalCpuMetrics(nodeMetrics)
	ch <- prometheus.MustNewConstMetric(nc.totalCpus, prometheus.GaugeValue, nodeCpuMetrics.Total)
	ch <- prometheus.MustNewConstMetric(nc.totalIdleCpus, prometheus.GaugeValue, nodeCpuMetrics.Idle)
	ch <- prometheus.MustNewConstMetric(nc.totalCpuLoad, prometheus.GaugeValue, nodeCpuMetrics.Load)
	for state, psm := range nodeCpuMetrics.PerState {
		ch <- prometheus.MustNewConstMetric(nc.cpusPerState, prometheus.GaugeValue, psm.Cpus, state)
		ch <- prometheus.MustNewConstMetric(nc.nodeCountPerState, prometheus.GaugeValue, psm.Count, state)
	}
	// node mem summary set
	memMetrics := fetchNodeTotalMemMetrics(nodeMetrics)
	ch <- prometheus.MustNewConstMetric(nc.totalRealMemory, prometheus.GaugeValue, memMetrics.RealMemory)
	ch <- prometheus.MustNewConstMetric(nc.totalFreeMemory, prometheus.GaugeValue, memMetrics.FreeMemory)
	ch <- prometheus.MustNewConstMetric(nc.totalAllocMemory, prometheus.GaugeValue, memMetrics.AllocMemory)
}
