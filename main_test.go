package main

import (
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
	textHandler := slog.NewTextHandler(os.Stdout, &opts)
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

// TODO: add integration test
