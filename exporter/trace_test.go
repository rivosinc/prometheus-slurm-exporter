// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAtomicFetcher_Cleanup(t *testing.T) {
	assert := assert.New(t)
	sampleRate := 10
	fetcher := NewAtomicProFetcher(uint64(sampleRate))
	fetcher.Info[11] = &TraceInfo{JobId: 11, uploadAt: time.Now().Add(-time.Second * 11)}
	fetcher.Info[10] = &TraceInfo{JobId: 10, uploadAt: time.Now()}
	fetcher.cleanup()
	assert.Contains(fetcher.Info, int64(10))
}

func TestAtomicFetcher_Add(t *testing.T) {
	assert := assert.New(t)
	fetcher := NewAtomicProFetcher(10)
	info := TraceInfo{JobId: 10}
	err := fetcher.Add(&info)
	assert.Nil(err)
	assert.Equal(1, len(fetcher.Info))
	assert.Contains(fetcher.Info, int64(10))
}

func TestAtomicFetcher_AddOverflow(t *testing.T) {
	assert := assert.New(t)
	sampleRate := 10
	fetcher := NewAtomicProFetcher(uint64(sampleRate))
	fetcher.cleanupThreshold = 1
	fetcher.Info[11] = &TraceInfo{JobId: 11, uploadAt: time.Now().Add(-time.Second * 11)}
	fetcher.Add(&TraceInfo{JobId: 10})
	assert.Equal(1, len(fetcher.Info))
	// assert.Contains(10, fetcher.Info)
}

func TestAtomicFetcher_AddNoJobid(t *testing.T) {
	assert := assert.New(t)
	fetcher := AtomicProcFetcher{Info: make(map[int64]*TraceInfo)}
	info := TraceInfo{JobId: 0}
	err := fetcher.Add(&info)
	assert.NotNil(err)
}

func TestAtomicFetcher_FetchStale(t *testing.T) {
	assert := assert.New(t)
	fetcher := NewAtomicProFetcher(1)
	fetcher.Info[10] = &TraceInfo{uploadAt: time.Now().Add(-time.Second * 10)}
	traces := fetcher.Fetch()
	assert.Equal(0, len(traces))
}

func TestAtomicFetcher_Fetch(t *testing.T) {
	assert := assert.New(t)
	fetcher := NewAtomicProFetcher(10)
	fetcher.Info[10] = &TraceInfo{uploadAt: time.Now()}
	traces := fetcher.Fetch()
	assert.Equal(1, len(traces))
}

func TestUploadTracePost(t *testing.T) {
	assert := assert.New(t)
	fixture, err := os.ReadFile("fixtures/trace_info_body.json")
	assert.Nil(err)
	config, err := NewConfig(new(CliFlags))
	assert.Nil(err)
	r := httptest.NewRequest(http.MethodPost, "dummy.url:8092/trace", bytes.NewBuffer(fixture))
	w := httptest.NewRecorder()
	c := NewTraceCollector(config)
	c.uploadTrace(w, r)
	assert.Equal(1, len(c.ProcessFetcher.Info))
}

func TestUploadTraceGet(t *testing.T) {
	assert := assert.New(t)
	r := httptest.NewRequest(http.MethodGet, "dummy.url:8092/trace", nil)
	w := httptest.NewRecorder()
	config, err := NewConfig(new(CliFlags))
	assert.Nil(err)
	c := NewTraceCollector(config)
	c.ProcessFetcher.Info[10] = &TraceInfo{}
	c.uploadTrace(w, r)
	assert.Equal(200, w.Code)
	assert.Positive(w.Body.Len())
}

func TestTraceControllerCollect(t *testing.T) {
	assert := assert.New(t)
	config := &Config{
		PollLimit: 10,
		TraceConf: &TraceConfig{
			rate: 10,
			sharedFetcher: &JobJsonFetcher{
				scraper:    MockJobInfoScraper,
				cache:      NewAtomicThrottledCache[JobMetric](1),
				errCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
			},
		},
		cliOpts: new(CliOpts),
	}
	c := NewTraceCollector(config)
	c.ProcessFetcher.Add(&TraceInfo{JobId: 26515966})
	assert.NotEmpty(c.ProcessFetcher.Info)
	metricChan := make(chan prometheus.Metric)
	go func() {
		c.Collect(metricChan)
		close(metricChan)
	}()

	metrics := make([]prometheus.Metric, 0)
	for m, ok := <-metricChan; ok; m, ok = <-metricChan {
		metrics = append(metrics, m)
	}
	assert.NotEmpty(metrics)
}

func TestTraceControllerCollect_Fallback(t *testing.T) {
	assert := assert.New(t)
	config := &Config{
		PollLimit: 10,
		TraceConf: &TraceConfig{
			rate: 10,
			sharedFetcher: &JobCliFallbackFetcher{
				scraper:    &MockScraper{fixture: "fixtures/squeue_fallback.txt"},
				cache:      NewAtomicThrottledCache[JobMetric](1),
				errCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
			},
		},
		cliOpts: &CliOpts{fallback: true},
	}
	c := NewTraceCollector(config)
	c.ProcessFetcher.Add(&TraceInfo{JobId: 26515966})
	assert.NotEmpty(c.ProcessFetcher.Info)
	metricChan := make(chan prometheus.Metric)
	go func() {
		c.Collect(metricChan)
		close(metricChan)
	}()

	metrics := make([]prometheus.Metric, 0)
	for m, ok := <-metricChan; ok; m, ok = <-metricChan {
		metrics = append(metrics, m)
	}
	assert.NotEmpty(metrics)
}

func TestTraceControllerDescribe(t *testing.T) {
	assert := assert.New(t)
	config := &Config{
		PollLimit: 10,
		TraceConf: &TraceConfig{
			rate: 10,
			sharedFetcher: &JobJsonFetcher{
				scraper:    MockJobInfoScraper,
				cache:      NewAtomicThrottledCache[JobMetric](1),
				errCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
			},
		},
		cliOpts: new(CliOpts),
	}
	c := NewTraceCollector(config)
	c.ProcessFetcher.Add(&TraceInfo{JobId: 26515966})
	assert.Positive(len(c.ProcessFetcher.Info))
	metricChan := make(chan *prometheus.Desc)
	go func() {
		assert.Positive(len(c.ProcessFetcher.Info))
		c.Describe(metricChan)
		close(metricChan)
	}()

	metrics := make([]*prometheus.Desc, 0)
	for m, ok := <-metricChan; ok; m, ok = <-metricChan {
		metrics = append(metrics, m)
		t.Logf("Received metric %s", m.String())
	}
	assert.Positive(len(metrics))
}

func TestPython3Wrapper(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	assert := assert.New(t)
	fetcher := NewCliScraper("python3", "../wrappers/proctrac.py", "--cmd", "sleep", "100", "--jobid=10", "--validate")
	t.Logf("cmd: %+v", fetcher.args)
	wrapperOut, err := fetcher.FetchRawBytes()
	assert.Nil(err)
	var info TraceInfo
	json.Unmarshal(wrapperOut, &info)
	assert.Equal(int64(10), info.JobId)
}

func TestDetectTraceRootPath_Env(t *testing.T) {
	testDir := t.TempDir()
	t.Setenv("TRACE_ROOT_PATH", testDir)
	// Ensure that the function panics if given a TRACE_ROOT_PATh with no 'templates' subdirectory
	assert.PanicsWithValue(t, "TRACE_ROOT_PATH must include a directory called: templates", func() { detectTraceTemplatePath() })
	require.NoError(t, os.Mkdir(filepath.Join(testDir, templateDirName), 0o700))

	// Now that we have a 'templates' subdir, it should no longer panic
	assert.Equal(t, filepath.Join(testDir, templateDirName), detectTraceTemplatePath())
}

func TestDetectTraceRootPath_Default(t *testing.T) {
	testDir := t.TempDir()
	os.Chdir(testDir)

	// Should come back empty if since we don't yet have a 'templates' subdir
	assert.Equal(t, detectTraceTemplatePath(), "")
	require.NoError(t, os.Mkdir(filepath.Join(testDir, templateDirName), 0o700))

	// Now that we have 'templates' subdir, cwd is a valid path
	assert.Equal(t, templateDirName, detectTraceTemplatePath())
}
