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
  {{vpython}} -m pip install pre-commit psutil requests

build:
  rm -rf {{build_dir}}
  mkdir {{build_dir}}
  go build -o {{build_dir}}/slurm_exporter .

devel: build
  {{build_dir}}/slurm_exporter \
  -trace.enabled \
  -slurm.squeue-cli "cat fixtures/squeue_out.json" \
  -slurm.sinfo-cli "cat fixtures/sinfo_out.json" \
  -slurm.collect-licenses \
  -slurm.lic-cli "cat fixtures/license_out.json"

prod *args: build
  {{build_dir}}/slurm_exporter -slurm.cli-fallback *args

test:
  source venv/bin/activate && go test -coverprofile {{coverage}}.out
  go tool cover -html {{coverage}}.out -o {{coverage}}.html
  open {{coverage}}.html

fmt:
  go fmt
