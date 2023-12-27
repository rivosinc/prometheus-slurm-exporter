package slurmcprom

// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

import "C"

import (
	psm "github.com/rivosinc/prometheus-slurm-exporter"
)

type CNodeInfoFetcher struct {
	cache         psm.AtomicThrottledCache[psm.NodeMetric]
	pluginScraper nodeMetricScraper
}
