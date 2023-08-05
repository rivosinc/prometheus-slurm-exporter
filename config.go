package main

import (
	"os"
	"strconv"
	"strings"

	"golang.org/x/exp/slog"
)

type TraceConfig struct {
	enabled bool
	path    string
	rate    uint64
}

type Config struct {
	traceConf     *TraceConfig
	pollLimit     float64
	logLevel      slog.Level
	listenAddress string
	metricsPath   string
}

func NewConfig() (*Config, error) {
	config := new(Config)
	var limit float64
	var err error
	if lm, ok := os.LookupEnv("POLL_LIMIT"); ok {
		if limit, err = strconv.ParseFloat(lm, 64); err != nil {
			return nil, err
		}
	}
	config.pollLimit = limit
	logLevel := "info"
	if lvl, ok := os.LookupEnv("LOGLEVEL"); ok {
		logLevel = strings.ToLower(lvl)
	}

	config.logLevel = logLevelMap[logLevel]
	return config, nil
}
