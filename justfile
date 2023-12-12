# SPDX-FileCopyrightText: 2023 Rivos Inc.
#
# SPDX-License-Identifier: Apache-2.0

build_dir := "./build"
coverage := "coverage"
venvdir := "venv"
vpython := "venv/bin/python3"

# Implicitly source '.env' files when running commands.
set dotenv-load
set shell := ["bash", "-ceuo", "pipefail"]

# list all recipes
default:
  just --list

init:
  go mod tidy
  rm -rf {{venvdir}}
  python -m venv {{venvdir}}
  {{venvdir}}/bin/python3 -m pip install -U pip pre-commit psutil requests
  {{venvdir}}/bin/pre-commit install --install-hooks

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

fmt:
  go fmt

docker-ctest:
  rm -rf {{build_dir}} && mkdir -p {{build_dir}}
  g++ cslurm.cpp -I/usr/lib64/include -L/usr/lib64/lib/slurm -lslurmfull -g -o build/cslurm
  # technically this should be ok to run natively...
  if ! [[ `stat /run/munge/munge.socket.2 2> /dev/null` ]]; then munged -f; fi
  LD_LIBRARY_PATH=/usr/lib64/lib/slurm/ ./build/cslurm

test-all:
  if ! [[ `stat /run/munge/munge.socket.2 2> /dev/null` ]]; then munged -f; fi
  source venv/bin/activate && CGO_CXXFLAGS="-I/usr/lib64/include" CGO_LDFLAGS="-L/usr/lib64/lib/slurm -lslurmfull" LD_LIBRARY_PATH=/usr/lib64/lib/slurm go test . ./slurmcprom
