// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestParseDiagJson(t *testing.T) {
	assert := assert.New(t)
	fetcher := MockScraper{fixture: "fixtures/sdiag.json"}
	sdiag, err := fetcher.FetchRawBytes()
	assert.NoError(err)
	resp, err := parseDiagMetrics(sdiag)
	assert.NoError(err)
	assert.Contains(resp.Meta.Plugins, "data_parser")
}

func TestDiagCollect(t *testing.T) {
	assert := assert.New(t)
	config, err := NewConfig(new(CliFlags))
	assert.NoError(err)
	dc := NewDiagsCollector(config)
	dc.fetcher = &MockScraper{fixture: "fixtures/sdiag.json"}
	metricChan := make(chan prometheus.Metric)
	go func() {
		dc.Collect(metricChan)
		close(metricChan)
	}()
	metrics := make([]prometheus.Metric, 0)
	for m, ok := <-metricChan; ok; m, ok = <-metricChan {
		metrics = append(metrics, m)
		t.Logf("Received metric %s", m.Desc().String())
	}
	assert.NotEmpty(metrics)
}

func TestDiagCollect_2405(t *testing.T) {
	assert := assert.New(t)
	config, err := NewConfig(new(CliFlags))
	assert.NoError(err)
	dc := NewDiagsCollector(config)
	dc.fetcher = &MockScraper{fixture: "fixtures/sdiag_2405.json"}
	metricChan := make(chan prometheus.Metric)
	go func() {
		dc.Collect(metricChan)
		close(metricChan)
	}()
	metrics := make([]prometheus.Metric, 0)
	for m, ok := <-metricChan; ok; m, ok = <-metricChan {
		metrics = append(metrics, m)
		t.Logf("Received metric %s", m.Desc().String())
	}
	assert.NotEmpty(metrics)
}

func TestDiagDescribe(t *testing.T) {
	assert := assert.New(t)
	ch := make(chan *prometheus.Desc)
	config, err := NewConfig(new(CliFlags))
	assert.Nil(err)
	dc := NewDiagsCollector(config)
	dc.fetcher = &MockScraper{fixture: "fixtures/sdiag.json"}
	go func() {
		dc.Describe(ch)
		close(ch)
	}()
	descs := make([]*prometheus.Desc, 0)
	for desc, ok := <-ch; ok; desc, ok = <-ch {
		descs = append(descs, desc)
	}
	assert.NotEmpty(descs)
}

func TestDataParserVersionDiscovery_Slurm23(t *testing.T) {
	assert := assert.New(t)
	fetcher := MockScraper{fixture: "fixtures/sdiag.json"}
	sdiag, err := fetcher.FetchRawBytes()
	assert.NoError(err)
	resp, err := parseDiagMetrics(sdiag)
	assert.NoError(err)
	assert.True(resp.IsDataParserPlugin())
}

func TestDataParserVersionDiscovery_Slurm24(t *testing.T) {
	assert := assert.New(t)
	fetcher := MockScraper{fixture: "fixtures/sdiag_2405.json"}
	sdiag, err := fetcher.FetchRawBytes()
	assert.NoError(err)
	resp, err := parseDiagMetrics(sdiag)
	assert.NoError(err)
	assert.Truef(resp.IsDataParserPlugin(), "parsed metadata struct %+v", resp.Meta)
}
