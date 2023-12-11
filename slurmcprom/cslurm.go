package slurmcprom

// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

import "C"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// tests work around due to the fact that we can't use the C and testing packages together. Thus we wrap them here
func testCGetPartitions(t *testing.T) {
	assert := assert.New(t)
	v := NewMetricExporter()
	defer DeleteMetricExporter(v)
	n := v.NumMetrics()
	assert.Zero(n)
	v.CollectNodeInfo()
	n = v.NumMetrics()
	assert.NotZero(n)
}
