// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog"
)

type NodeMetric struct {
	AllocMemory float64  `json:"alloc_memory"`
	AllocCpus   float64  `json:"alloc_cpus"`
	Cpus        float64  `json:"cpus"`
	CpuLoad     float64  `json:"cpu_load"`
	FreeMemory  float64  `json:"free_memory"`
	Hostname    string   `json:"hostname"`
	IdleCpus    float64  `json:"idle_cpus"`
	Partitions  []string `json:"partitions"`
	RealMemory  float64  `json:"real_memory"`
	State       string   `json:"state"`
	Weight      float64  `json:"weight"`
}

type sinfoDataParserResponse struct {
	Meta struct {
		Plugins map[string]string `json:"plugins"`
	} `json:"meta"`
	SlurmVersion struct {
		Version struct {
			Major int `json:"major"`
			Micro int `json:"micro"`
			Minor int `json:"minor"`
		} `json:"version"`
		Release string `json:"release"`
	} `json:"Slurm"`
	Sinfo []struct {
		Node struct {
			State []string `json:"state"`
		} `json:"node"`
		Nodes struct {
			Allocated int      `json:"allocated"`
			Idle      int      `json:"idle"`
			Other     int      `json:"other"`
			Total     int      `json:"total"`
			Nodes     []string `json:"nodes"`
		} `json:"nodes"`
	} `json:"sinfo"`
}

type sinfoDataParserResponse struct {
	Meta struct {
		Plugins map[string]string `json:"plugins"`
	} `json:"meta"`
	SlurmVersion struct {
		Version struct {
			Major int `json:"major"`
			Micro int `json:"micro"`
			Minor int `json:"minor"`
		} `json:"version"`
		Release string `json:"release"`
	} `json:"Slurm"`
	Sinfo []struct {
		Node struct {
			State []string `json:"state"`
		} `json:"node"`
		Nodes struct {
			Allocated int      `json:"allocated"`
			Idle      int      `json:"idle"`
			Other     int      `json:"other"`
			Total     int      `json:"total"`
			Nodes     []string `json:"nodes"`
		} `json:"nodes"`
		Cpus struct {
			Allocated int `json:"allocated"`
			Idle      int `json:"idle"`
			Other     int `json:"other"`
			Total     int `json:"total"`
		}
		Memory struct {
			Minimum   int `json:"minimum"`
			Maximum   int `json:"maximum"`
			Allocated int `json:"allocated"`
			Free      struct {
				Minimum struct {
					Set    bool `json:"set"`
					Number int  `json:"number"`
				} `json:"minimum"`
				Maximum struct {
					Set    bool `json:"set"`
					Number int  `json:"number"`
				} `json:"maximum"`
			} `json:"free"`
		}
		Partition struct {
			Name      string `json:"name"`
			Alternate string `json:"alternate"`
		} `json:"parittion"`
	} `json:"sinfo"`
}

type DataParserJsonFetcher struct {
	scraper      SlurmByteScraper
	errorCounter prometheus.Counter
	cache        *AtomicThrottledCache[NodeMetric]
}

func (dpj *DataParserJsonFetcher) fetch() ([]NodeMetric, error) {
	squeue := new(sinfoDataParserResponse)
	cliJson, err := dpj.scraper.FetchRawBytes()
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(cliJson, squeue); err != nil {
		return nil, err
	}
	nodeMetrics := make([]NodeMetric, 0)
	for _, entry := range squeue.Sinfo {
		nodes := entry.Nodes
		// validate single node parse
		if nodes.Total != 1 {
			return nil, fmt.Errorf("must contain only 1 node per entry, please use the -N option exp. `sinfo -N --json`")
		}
		if entry.Memory.Free.Maximum.Set && entry.Memory.Free.Minimum.Set {
			return nil, fmt.Errorf("unable to scrape free mem metrics")
		}
		if entry.Memory.Free.Minimum.Number != entry.Memory.Free.Maximum.Number {
			return nil, fmt.Errorf("must contain only 1 node per entry, please use the -N option exp. `sinfo -N --json`")
		}
		if entry.Memory.Minimum != entry.Memory.Maximum {
			return nil, fmt.Errorf("must contain only 1 node per entry, please use the -N option exp. `sinfo -N --json`")
		}
		metric := NodeMetric{
			Hostname:   nodes.Nodes[0],
			Cpus:       float64(entry.Cpus.Total),
			RealMemory: float64(entry.Memory.Maximum),
			FreeMemory: float64(entry.Memory.Free.Maximum.Number),
			State:      strings.Join(entry.Node.State, "&"),
		}
		if !slices.Contains(metric.Partitions, entry.Partition.Name) {
			metric.Partitions = append(metric.Partitions, entry.Partition.Name)
		}
		if entry.Partition.Alternate != "" && !slices.Contains(metric.Partitions, entry.Partition.Alternate) {
			metric.Partitions = append(metric.Partitions, entry.Partition.Alternate)
		}
		nodeMetrics = append(nodeMetrics, metric)
	}
	return nodeMetrics, nil
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

type NodeJsonFetcher struct {
	scraper      SlurmByteScraper
	errorCounter prometheus.Counter
	cache        *AtomicThrottledCache[NodeMetric]
}

func (cmf *NodeJsonFetcher) fetch() ([]NodeMetric, error) {
	squeue := new(sinfoResponse)
	cliJson, err := cmf.scraper.FetchRawBytes()
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(cliJson, squeue); err != nil {
		slog.Error("Unmarshaling node metrics %q", err)
		return nil, err
	}
	if len(squeue.Errors) > 0 {
		for _, e := range squeue.Errors {
			slog.Error("Api error response %q", e)
		}
		cmf.errorCounter.Add(float64(len(squeue.Errors)))
		return nil, errors.New(squeue.Errors[0])
	}
	return squeue.Nodes, nil
}

func (cmf *NodeJsonFetcher) FetchMetrics() ([]NodeMetric, error) {
	return cmf.cache.FetchOrThrottle(cmf.fetch)
}

func (cmf *NodeJsonFetcher) ScrapeError() prometheus.Counter {
	return cmf.errorCounter
}

func (cmf *NodeJsonFetcher) ScrapeDuration() time.Duration {
	return cmf.scraper.Duration()
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

type NodeCliFallbackFetcher struct {
	scraper      SlurmByteScraper
	errorCounter prometheus.Counter
	cache        *AtomicThrottledCache[NodeMetric]
}

func (cmf *NodeCliFallbackFetcher) fetch() ([]NodeMetric, error) {
	sinfo, err := cmf.scraper.FetchRawBytes()
	if err != nil {
		cmf.errorCounter.Inc()
		return nil, err
	}
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
			cmf.errorCounter.Inc()
			slog.Error(fmt.Sprintf("sinfo failed to parse line %d: %s, got %q", i, line, err))
			continue
		}
		// convert mem units from MB to Bytes
		metric.RealMemory *= 1e6
		metric.FreeMemory *= 1e6
		cpuStates := strings.Split(metric.CpuState, "/")
		if len(cpuStates) != 4 {
			cmf.errorCounter.Inc()
			return nil, fmt.Errorf("unexpected cpu state format. Got %s", metric.CpuState)
		}
		allocated, err := strconv.ParseFloat(cpuStates[0], 64)
		if err != nil {
			cmf.errorCounter.Inc()
			return nil, err
		}
		idle, err := strconv.ParseFloat(cpuStates[1], 64)
		if err != nil {
			cmf.errorCounter.Inc()
			return nil, err
		}
		other, err := strconv.ParseFloat(cpuStates[2], 64)
		if err != nil {
			cmf.errorCounter.Inc()
			return nil, err
		}
		total, err := strconv.ParseFloat(cpuStates[3], 64)
		if err != nil {
			cmf.errorCounter.Inc()
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

func (cmf *NodeCliFallbackFetcher) FetchMetrics() ([]NodeMetric, error) {
	return cmf.cache.FetchOrThrottle(cmf.fetch)
}

type PartitionMetric struct {
	TotalCpus        float64
	RealMemory       float64
	FreeMemory       float64
	StateAllocMemory map[string]float64
	StateAllocCpus   map[string]float64
	CpuLoad          float64
	IdleCpus         float64
	Weight           float64
}

func fetchNodePartitionMetrics(nodes []NodeMetric) map[string]*PartitionMetric {
	partitions := make(map[string]*PartitionMetric)
	for _, node := range nodes {
		for _, p := range node.Partitions {
			partition, ok := partitions[p]
			if !ok {
				partition = &PartitionMetric{
					StateAllocMemory: make(map[string]float64),
					StateAllocCpus:   make(map[string]float64),
				}
				partitions[p] = partition
			}
			partition.StateAllocCpus[node.State] += node.AllocCpus
			partition.StateAllocMemory[node.State] += node.AllocMemory
			partition.TotalCpus += node.Cpus
			partition.CpuLoad += node.CpuLoad
			partition.FreeMemory += node.FreeMemory
			partition.IdleCpus += node.IdleCpus
			partition.RealMemory += node.RealMemory
			partition.Weight += node.Weight
		}
	}
	return partitions
}

func (cmf *NodeCliFallbackFetcher) ScrapeError() prometheus.Counter {
	return cmf.errorCounter
}

func (cmf *NodeCliFallbackFetcher) ScrapeDuration() time.Duration {
	return cmf.scraper.Duration()
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
	fetcher SlurmMetricFetcher[NodeMetric]
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
	byteScraper := NewCliScraper(cliOpts.sinfo...)
	errorCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "slurm_node_scrape_error",
		Help: "slurm node info scrape errors",
	})
	var fetcher SlurmMetricFetcher[NodeMetric]
	if cliOpts.fallback {
		fetcher = &NodeCliFallbackFetcher{scraper: byteScraper, errorCounter: errorCounter, cache: NewAtomicThrottledCache[NodeMetric](config.PollLimit)}
	} else {
		fetcher = &NodeJsonFetcher{scraper: byteScraper, errorCounter: errorCounter, cache: NewAtomicThrottledCache[NodeMetric](config.PollLimit)}
	}
	return &NodesCollector{
		fetcher: fetcher,
		// partition stats
		partitionCpus:        prometheus.NewDesc("slurm_partition_total_cpus", "Total cpus per partition", []string{"partition"}, nil),
		partitionRealMemory:  prometheus.NewDesc("slurm_partition_real_mem", "Real mem per partition", []string{"partition"}, nil),
		partitionFreeMemory:  prometheus.NewDesc("slurm_partition_free_mem", "Free mem per partition", []string{"partition"}, nil),
		partitionAllocMemory: prometheus.NewDesc("slurm_partition_alloc_mem", "Alloc mem per partition", []string{"partition", "state"}, nil),
		partitionAllocCpus:   prometheus.NewDesc("slurm_partition_alloc_cpus", "Alloc cpus per partition", []string{"partition", "state"}, nil),
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
		nodeScrapeErrors:   fetcher.ScrapeError(),
	}
}

func (nc *NodesCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.partitionAllocCpus
	ch <- nc.partitionAllocMemory
	ch <- nc.partitionCpus
	ch <- nc.partitionCpuLoad
	ch <- nc.partitionFreeMemory
	ch <- nc.partitionIdleCpus
	ch <- nc.partitionRealMemory
	ch <- nc.partitionWeight
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
		ch <- nc.fetcher.ScrapeError()
	}()
	nodeMetrics, err := nc.fetcher.FetchMetrics()
	ch <- prometheus.MustNewConstMetric(nc.nodeScrapeDuration, prometheus.GaugeValue, float64(nc.fetcher.ScrapeDuration().Milliseconds()))
	if err != nil {
		slog.Error("Failed to parse node metrics: " + err.Error())
		return
	}
	emitStateVal := func(partition string, stateMap map[string]float64, desc *prometheus.Desc) {
		for state, val := range stateMap {
			if val > 0 {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, val, partition, state)
			}
		}
	}
	// partition set
	partitionMetrics := fetchNodePartitionMetrics(nodeMetrics)
	for partition, metric := range partitionMetrics {
		emitStateVal(partition, metric.StateAllocCpus, nc.partitionAllocCpus)
		emitStateVal(partition, metric.StateAllocMemory, nc.partitionAllocMemory)
		if metric.TotalCpus > 0 {
			ch <- prometheus.MustNewConstMetric(nc.partitionCpus, prometheus.GaugeValue, metric.TotalCpus, partition)
		}
		if metric.CpuLoad > 0 {
			ch <- prometheus.MustNewConstMetric(nc.partitionCpuLoad, prometheus.GaugeValue, metric.CpuLoad, partition)
		}
		if metric.FreeMemory > 0 {
			ch <- prometheus.MustNewConstMetric(nc.partitionFreeMemory, prometheus.GaugeValue, metric.FreeMemory, partition)
		}
		if metric.IdleCpus > 0 {
			ch <- prometheus.MustNewConstMetric(nc.partitionIdleCpus, prometheus.GaugeValue, metric.IdleCpus, partition)
		}
		if metric.RealMemory > 0 {
			ch <- prometheus.MustNewConstMetric(nc.partitionRealMemory, prometheus.GaugeValue, metric.RealMemory, partition)
		}
		if metric.Weight > 0 {
			ch <- prometheus.MustNewConstMetric(nc.partitionWeight, prometheus.GaugeValue, metric.Weight, partition)
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

func (nc *NodesCollector) SetFetcher(fetcher SlurmMetricFetcher[NodeMetric]) {
	nc.fetcher = fetcher
}
