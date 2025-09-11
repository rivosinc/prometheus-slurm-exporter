// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"fmt"
	"slices"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var MockNodeInfoScraper = &MockScraper{fixture: "fixtures/sinfo_out.json"}
var MockNodeInfoDataParserScraper = &MockScraper{fixture: "fixtures/sinfo_dataparser.json"}

func TestNewNodeCollector(t *testing.T) {
	assert := assert.New(t)
	config := &Config{
		cliOpts: &CliOpts{
			fallback: true,
		},
	}
	collector := NewNodeCollecter(config)
	assert.IsType(collector.fetcher, &NodeCliFallbackFetcher{})
	config.cliOpts.fallback = false
	collector = NewNodeCollecter(config)
	assert.IsType(collector.fetcher, &NodeJsonFetcher{})

}

func TestParseNodeMetrics(t *testing.T) {
	fetcher := NodeJsonFetcher{scraper: MockNodeInfoScraper, errorCounter: prometheus.NewCounter(prometheus.CounterOpts{}), cache: NewAtomicThrottledCache[NodeMetric](1)}
	nodeMetrics, err := fetcher.FetchMetrics()
	if err != nil {
		t.Fatalf("Failed to parse metrics with %s", err)
	}
	if len(nodeMetrics) == 0 {
		t.Fatal("No metrics received")
	}
	t.Logf("Node metrics collected %d", len(nodeMetrics))
}

func sumStateMetric(metric map[string]float64) float64 {
	sum := 0.
	for _, val := range metric {
		sum += val
	}
	return sum
}

func TestPartitionMetric(t *testing.T) {
	assert := assert.New(t)
	fetcher := NodeJsonFetcher{scraper: MockNodeInfoScraper, errorCounter: prometheus.NewCounter(prometheus.CounterOpts{}), cache: NewAtomicThrottledCache[NodeMetric](1)}
	nodeMetrics, err := fetcher.FetchMetrics()
	assert.Nil(err)
	metrics := fetchNodePartitionMetrics(nodeMetrics)
	assert.Equal(1, len(metrics))
	_, contains := metrics["hw"]
	assert.True(contains)
	assert.Equal(4., sumStateMetric(metrics["hw"].StateAllocCpus))
	assert.Equal(256., metrics["hw"].TotalCpus)
	assert.Equal(114688., sumStateMetric(metrics["hw"].StateAllocMemory))
	assert.Equal(1.823573e+06, metrics["hw"].FreeMemory)
	assert.Equal(2e+06, metrics["hw"].RealMemory)
	assert.Equal(252., metrics["hw"].IdleCpus)
	assert.Equal(4., sumStateMetric(metrics["hw"].StateNodeCount))
}

func TestNodeSummaryCpuMetric(t *testing.T) {
	assert := assert.New(t)
	fetcher := NodeJsonFetcher{scraper: MockNodeInfoScraper, errorCounter: prometheus.NewCounter(prometheus.CounterOpts{}), cache: NewAtomicThrottledCache[NodeMetric](1)}
	nodeMetrics, err := fetcher.FetchMetrics()
	assert.Nil(err)
	metrics := fetchNodeTotalCpuMetrics(nodeMetrics)
	assert.Equal(4, len(metrics.PerState))
	for _, psm := range metrics.PerState {
		assert.Equal(64., psm.Cpus)
		assert.Equal(1., psm.Count)
	}
}

func TestNodeSummaryMemoryMetrics(t *testing.T) {
	assert := assert.New(t)
	fetcher := NodeJsonFetcher{scraper: MockNodeInfoScraper, errorCounter: prometheus.NewCounter(prometheus.CounterOpts{}), cache: NewAtomicThrottledCache[NodeMetric](1)}
	nodeMetrics, err := fetcher.FetchMetrics()
	assert.Nil(err)
	metrics := fetchNodeTotalMemMetrics(nodeMetrics)
	assert.Equal(114688., metrics.AllocMemory)
	assert.Equal(1.823573e+06, metrics.FreeMemory)
	assert.Equal(2e+06, metrics.RealMemory)
}

func TestNodeCollector(t *testing.T) {
	assert := assert.New(t)
	config, err := NewConfig(new(CliFlags))
	assert.Nil(err)
	nc := NewNodeCollecter(config)
	// cache miss, use our mock fetcher
	nc.fetcher = &NodeJsonFetcher{scraper: MockNodeInfoScraper, errorCounter: prometheus.NewCounter(prometheus.CounterOpts{}), cache: NewAtomicThrottledCache[NodeMetric](1)}
	metricChan := make(chan prometheus.Metric)
	go func() {
		nc.Collect(metricChan)
		close(metricChan)
	}()
	metrics := make([]prometheus.Metric, 0)
	for m, ok := <-metricChan; ok; m, ok = <-metricChan {
		metrics = append(metrics, m)
		t.Logf("Received metric %s", m.Desc().String())
	}
	assert.NotEmpty(metrics)
}

func TestNodeDescribe(t *testing.T) {
	assert := assert.New(t)
	ch := make(chan *prometheus.Desc)
	config, err := NewConfig(new(CliFlags))
	assert.Nil(err)
	jc := NewNodeCollecter(config)
	jc.fetcher = &NodeJsonFetcher{scraper: MockNodeInfoScraper, errorCounter: prometheus.NewCounter(prometheus.CounterOpts{}), cache: NewAtomicThrottledCache[NodeMetric](1)}
	go func() {
		jc.Describe(ch)
		close(ch)
	}()
	descs := make([]*prometheus.Desc, 0)
	for desc, ok := <-ch; ok; desc, ok = <-ch {
		descs = append(descs, desc)
	}
	assert.NotEmpty(descs)
}

func TestParseFallbackNodeMetricsCsv(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	byteFetcher := &MockScraper{fixture: "fixtures/sinfo_fallback.txt"}
	fetcher := NodeCliFallbackFetcher{scraper: byteFetcher, errorCounter: prometheus.NewCounter(prometheus.CounterOpts{}), cache: NewAtomicThrottledCache[NodeMetric](1)}
	metrics, err := fetcher.FetchMetrics()
	assert.Nil(err)
	assert.NotEmpty(metrics)
	cs222Idx := slices.IndexFunc(metrics, func(m NodeMetric) bool { return m.Hostname == "cs222" })
	t.Logf("metric %+v\n", metrics[cs222Idx])
	require.GreaterOrEqual(cs222Idx, 0)
	cs222Metric := metrics[cs222Idx]
	assert.Equal(cs222Metric.CpuLoad, 28.08)
	assert.ElementsMatch(cs222Metric.Partitions, []string{"hw-h", "hw-l*", "hw-m", "hw-h-lmt"})
}

func TestNAbleFloat_NA(t *testing.T) {
	assert := assert.New(t)
	n := NAbleFloat(1.5)
	data := []byte(`"N/A"`)
	assert.NoError(n.UnmarshalJSON(data))
	assert.Equal(0., float64(n))
}

func TestNAbleFloat_Float(t *testing.T) {
	assert := assert.New(t)
	n := NAbleFloat(1.5)
	expected := 3.14
	data := fmt.Appendf(nil, `"%f"`, expected)
	assert.NoError(n.UnmarshalJSON(data))
	assert.Equal(expected, float64(n))
}
