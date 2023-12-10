//go:build cgo

// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
package github.com/rivosinc/prometheus-slurm-exporter/cslurm_prom

import "C"

import (
	"testing"
)

func TestCGetPartitions(t *testing.T) {
	testCGetPartitions(t)
}
