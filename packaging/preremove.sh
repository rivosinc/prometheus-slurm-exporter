#!/bin/bash
# SPDX-FileCopyrightText: 2023 Rivos Inc.
#
# SPDX-License-Identifier: Apache-2.0

systemctl disable prometheus-slurm-exporter.service || /bin/true
