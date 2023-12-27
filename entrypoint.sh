#!/bin/bash
# SPDX-FileCopyrightText: 2023 Rivos Inc.
#
# SPDX-License-Identifier: Apache-2.0
munged -f 2> /dev/null
slurmctld
slurmd
exec $@
