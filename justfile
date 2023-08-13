build_dir := "./build"
coverage := "coverage"

# Implicitly source '.env' files when running commands.
set dotenv-load := true
set shell := ["bash", "-ceuo", "pipefail"]

# list all recipes
default:
  just --list

init:
  go mod tidy
  rm -rf venv
  python -m venv venv
  source venv/bin/activate
  pip install pre-commit psutil requests

build:
  rm -rf {{build_dir}}
  mkdir {{build_dir}}
  go build -o {{build_dir}}/slurm_exporter .

devel: build
  {{build_dir}}/slurm_exporter --trace.enabled

test:
  go test -coverprofile {{coverage}}.out
  go tool cover -html {{coverage}}.out -o {{coverage}}.html
  open {{coverage}}.html

fmt:
  go fmt
