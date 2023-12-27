package slurmcprom

// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodeMetricFetcherInit(t *testing.T) {
	assert := assert.New(t)
	fetcher := NewNodeMetricScraper("")
	defer DeleteNodeMetricScraper(fetcher)
	assert.Zero(fetcher.NumMetrics())
	err := fetcher.CollectNodeInfo()
	assert.Zero(err)
	assert.NotZero(fetcher.NumMetrics())
}

func TestNodeMetricFetcherCustomConf(t *testing.T) {
	assert := assert.New(t)
	fetcher := NewNodeMetricFetcher("/etc/slurm/slurm.conf")
	defer DeleteNodeMetricFetcher(fetcher)
	assert.Zero(fetcher.NumMetrics())
	err := fetcher.CollectNodeInfo()
	assert.Zero(err)
	assert.NotZero(fetcher.NumMetrics())
}
