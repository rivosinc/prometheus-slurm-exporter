# SPDX-FileCopyrightText: 2023 Rivos Inc.
#
# SPDX-License-Identifier: Apache-2.0

build_dir := "./build"
coverage := "coverage"
vpython := "venv/bin/python3"
# default ld_library and include paths that work within container
ld_library := "/usr/lib64/lib/slurm"
include_path := "/usr/lib64/include"

set dotenv-load
set shell := ["bash", "-ceuo", "pipefail"]

# list all recipes
default:
  just --list

init:
  go mod tidy
  rm -rf venv
  python3 -m venv venv
  {{vpython}} -m pip install -U pip pre-commit psutil requests
  ./venv/bin/pre-commit install --install-hooks
  if ! [ -f .env ]; then printf "SLURM_LIB_DIR={{ld_library}}\nSLURM_INCLUDE_DIR={{include_path}}\n" > .env; fi

build:
  rm -rf {{build_dir}}
  mkdir {{build_dir}}
  CGO_ENABLED=0 go build -o {{build_dir}}/slurm_exporter .

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
  source venv/bin/activate && CGO_ENABLED=0 go test

cover:
  CGO_ENABLED=0 go test -coverprofile=c.out
  go tool cover -html="c.out"

fmt:
  go fmt

docker:
  docker build -t slurmcprom .

test-all:
  #!/bin/bash
  set -aeuxo pipefail
  CGO_CXXFLAGS="-I${SLURM_INCLUDE_DIR}"
  CGO_LDFLAGS="-L${SLURM_LIB_DIR} -lslurmfull"
  go test . ./slurmcprom
