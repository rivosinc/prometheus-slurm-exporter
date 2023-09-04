// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

var MockLicFetcher = &MockFetcher{fixture: "fixtures/license_out.json"}

func TestParseLicMetrics(t *testing.T) {
	assert := assert.New(t)
	fetcher := MockFetcher{fixture: "fixtures/license_out.json"}
	data, err := fetcher.Fetch()
	assert.Nil(err)
	lics, err := parseLicenseMetrics(data)
	assert.Nil(err)
	t.Logf("lics %v", lics)
	assert.Equal(1, len(lics))
}

func TestNewLicController(t *testing.T) {
	assert := assert.New(t)
	config := Config{
		pollLimit: 10,
		cliOpts: &CliOpts{
			licEnabled: true,
		},
	}
	lc := NewLicCollector(&config)
	assert.NotNil(lc)
}

func TestLicCollect(t *testing.T) {
	assert := assert.New(t)
	config := Config{
		pollLimit: 10,
		cliOpts: &CliOpts{
			licEnabled: true,
		},
	}
	lc := NewLicCollector(&config)
	lc.fetcher = MockLicFetcher
	lcChan := make(chan prometheus.Metric)
	go func() {
		lc.Collect(lcChan)
		close(lcChan)
	}()
	licMetrics := make([]prometheus.Metric, 0)
	for metric, ok := <-lcChan; ok; metric, ok = <-lcChan {
		t.Log(metric.Desc().String())
		licMetrics = append(licMetrics, metric)
	}
	assert.NotEmpty(licMetrics)
}

func TestLicDescribe(t *testing.T) {
	assert := assert.New(t)
	config := Config{
		pollLimit: 10,
		cliOpts: &CliOpts{
			licEnabled: true,
		},
	}
	lc := NewLicCollector(&config)
	lc.fetcher = MockLicFetcher
	lcChan := make(chan *prometheus.Desc)
	go func() {
		lc.Describe(lcChan)
		close(lcChan)
	}()
	licMetrics := make([]*prometheus.Desc, 0)
	for desc, ok := <-lcChan; ok; desc, ok = <-lcChan {
		t.Log(desc.String())
		licMetrics = append(licMetrics, desc)
	}
	assert.NotEmpty(licMetrics)
}
