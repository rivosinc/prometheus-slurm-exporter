# SPDX-FileCopyrightText: 2023 Rivos Inc.
#
# SPDX-License-Identifier: Apache-2.0

build_dir := "./build"
coverage := "coverage"
vpython := "venv/bin/python3"

# Implicitly source '.env' files when running commands.
set dotenv-load
set shell := ["bash", "-ceuo", "pipefail"]

# list all recipes
default:
  just --list

init:
  go mod tidy
  rm -rf venv
  python -m venv venv
  {{vpython}} -m pip install -U pip pre-commit psutil requests

build:
  rm -rf {{build_dir}}
  mkdir {{build_dir}}
  go build -o {{build_dir}}/slurm_exporter .

devel: build
  {{build_dir}}/slurm_exporter \
  -trace.enabled \
  -slurm.collect-diags \
  -slurm.collect-licenses \
  -slurm.squeue-cli "cat fixtures/squeue_out.json" \
  -slurm.sinfo-cli "cat fixtures/sinfo_out.json" \
  -slurm.diag-cli "cat fixtures/sdiag.json" \
  -slurm.lic-cli "cat fixtures/license_out.json"

prod: build
  {{build_dir}}/slurm_exporter -slurm.cli-fallback

test:
  source venv/bin/activate && go test -coverprofile {{coverage}}.out
  go tool cover -html {{coverage}}.out -o {{coverage}}.html
  open {{coverage}}.html

fmt:
  go fmt

ctest:
  export LD_LIBRARY_PATH=/usr/lib64/slurm
  gcc cslurm.c -L/usr/lib64/slurm -lslurmfull -g -o build/cslurm
  ./build/cslurm
