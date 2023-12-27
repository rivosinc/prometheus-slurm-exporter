package slurmcprom

// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// func TestNodeMetricFetcherInit(t *testing.T) {
// 	assert := assert.New(t)
// 	fetcher := NewNodeMetricScraper("")
// 	defer DeleteNodeMetricScraper(fetcher)
// 	err := fetcher.CollectNodeInfo()
// 	assert.Zero(err)
// }

func TestCtoGoNodeMetrics(t *testing.T) {
	assert := assert.New(t)
	collector := NewNodeFetcher()
	defer collector.Deinit()
	metrics, err := collector.CToGoMetricConvert()
	assert.NoError(err)
	assert.Positive(len(metrics))
}

func TestCtoGoNodeMetricsTwice(t *testing.T) {
	assert := assert.New(t)
	collector := NewNodeFetcher()
	defer collector.Deinit()
	metrics, err := collector.CToGoMetricConvert()
	assert.NoError(err)
	assert.Positive(len(metrics))
	// tests cached partition & node info data path
	metrics, err = collector.CToGoMetricConvert()
	assert.NoError(err)
	assert.Positive(len(metrics))
}
