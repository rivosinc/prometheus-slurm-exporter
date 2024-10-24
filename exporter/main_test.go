// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slog"
)

// global test setups
func TestMain(m *testing.M) {
	opts := slog.HandlerOptions{
		Level: slog.LevelError,
	}
	textHandler := slog.NewTextHandler(io.Discard, &opts)
	slog.SetDefault(slog.New(textHandler))
	code := m.Run()
	os.Exit(code)
}

func TestPromServer(t *testing.T) {
	assert := assert.New(t)
	cliOpts := &CliOpts{
		sinfo:  []string{"cat", "fixtures/sinfo_out.json"},
		squeue: []string{"cat", "fixtures/squeue_out.json"},
	}

	config := &Config{
		PollLimit: 10,
		cliOpts:   cliOpts,
		TraceConf: &TraceConfig{
			enabled: false,
			sharedFetcher: &JobJsonFetcher{
				scraper: NewCliScraper(cliOpts.squeue...),
				cache:   NewAtomicThrottledCache[JobMetric](1),
				errCounter: prometheus.NewCounter(prometheus.CounterOpts{
					Name: "slurm_job_scrape_error",
					Help: "job scrape error",
				}),
			},
		},
	}
	server := InitPromServer(config)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	server.ServeHTTP(w, r)
	assert.Equal(200, w.Code)
	txt := w.Body.String()
	assert.Contains(txt, "slurm_job_scrape_error 0")
	assert.Contains(txt, "slurm_node_scrape_error 0")
}

func TestNewConfig_Default(t *testing.T) {
	assert := assert.New(t)
	config, err := NewConfig(new(CliFlags))
	assert.Nil(err)
	assert.Equal([]string{"sinfo", "--json"}, config.cliOpts.sinfo)
	assert.Equal([]string{"squeue", "--json"}, config.cliOpts.squeue)
	assert.Equal([]string{"scontrol", "show", "lic", "--json"}, config.cliOpts.lic)
	assert.Equal(uint64(10), config.TraceConf.rate)
}

func TestNewConfig_NonDefault(t *testing.T) {
	assert := assert.New(t)
	cliFlags := CliFlags{SlurmCliFallback: true}
	config, err := NewConfig(&cliFlags)
	assert.Nil(err)
	expected := []string{"squeue", "--states=all", "-h", "-r", "-o", `{"a": "%a", "id": %A, "end_time": "%e", "u": "%u", "state": "%T", "p": "%P", "cpu": %C, "mem": "%m", "array_id": "%K", "r": "%R"}`}
	assert.Equal(expected, config.cliOpts.squeue)
}

// TODO: add integration test
