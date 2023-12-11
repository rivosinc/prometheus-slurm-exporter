#!/bin/bash

# SPDX-FileCopyrightText: 2023 Rivos Inc.
#
# SPDX-License-Identifier: Apache-2.0

SLURM_CONF_DIR=${SLURM_CONF_DIR:-'/etc/slurm'}
MUNGE_KEY=${MUNGE_KEY:-'/etc/munge/munge.key'}
DOCKER_IMAGE=${1:-'slurm_exporter'}

rm -rf tmp_sconf
mkdir tmp_sconf
cp "$SLURM_CONF_DIR/slurm*" tmp_sconf
cp "$MUNGE_KEY" tmp_sconf
docker build -t $DOCKER_IMAGE .
