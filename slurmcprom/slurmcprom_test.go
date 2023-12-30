package slurmcprom

// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCtoGoNodeMetrics(t *testing.T) {
	assert := assert.New(t)
	collector := NewNodeFetcher(0)
	defer collector.Deinit()
	metrics, err := collector.CToGoMetricConvert()
	assert.NoError(err)
	assert.Positive(len(metrics))
}

func TestCtoGoNodeMetricsTwice(t *testing.T) {
	assert := assert.New(t)
	// force cache misses
	collector := NewNodeFetcher(0)
	defer collector.Deinit()
	metrics, err := collector.CToGoMetricConvert()
	assert.NoError(err)
	assert.Positive(len(metrics))
	// tests cached partition & node info data path
	metrics, err = collector.CToGoMetricConvert()
	assert.NoError(err)
	assert.Positive(len(metrics))
}
