// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

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
	config := &Config{
		pollLimit: 10,
		cliOpts: &CliOpts{
			sinfo:  []string{"cat", "fixtures/sinfo_out.json"},
			squeue: []string{"cat", "fixtures/squeue_out.json"},
		},
		traceConf: new(TraceConfig),
	}
	cliOpts := config.cliOpts
	config.SetFetcher(NewCliFetcher(cliOpts.squeue...))
	server := initPromServer(config)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	server.ServeHTTP(w, r)
	assert.Equal(200, w.Code)
	txt := strings.Split(w.Body.String(), "\n")
	assert.Contains(txt, "slurm_job_scrape_error 0")
	assert.Contains(txt, "slurm_node_scrape_error 0")
}

func TestNewConfig(t *testing.T) {
	assert := assert.New(t)
	config, err := NewConfig()
	assert.Nil(err)
	assert.Equal([]string{"sinfo", "--json"}, config.cliOpts.sinfo)
	assert.Equal([]string{"squeue", "--json"}, config.cliOpts.squeue)
	assert.Equal([]string{"scontrol", "show", "lic", "--json"}, config.cliOpts.lic)
	assert.Equal(uint64(10), config.traceConf.rate)
}

// TODO: add integration test
