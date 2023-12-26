# SPDX-FileCopyrightText: 2023 Rivos Inc.
#
# SPDX-License-Identifier: Apache-2.0
FROM --platform=linux/amd64 ubuntu:20.04
ARG SLURM_VERSION="23-02-5-1"
ENV DEBIAN_FRONTEND=noninteractive
ENV LD_LIRBARY_PATH=/usr/lib64/lib/slurm
ENV PATH=/usr/lib64/bin:/usr/lib64/sbin:/root/.cargo/bin:/usr/local/go/bin:$PATH
RUN apt-get update -y && apt-get install -y build-essential \
    cargo \
    git \
    git-lfs \
    gdb \
    libjson-c-dev \
    python3-venv \
    python-is-python3 \
    python3-pip \
    tmux \
    vim \
    swig3.0 \
    wget && \
    apt-get autoclean && \
    ln -s /usr/bin/swig3.0 /usr/bin/swig
# munge
RUN printf '#!/bin/sh\nexit 0' > /usr/sbin/policy-rc.d && apt-get install -y libmunge-dev munge && apt-get autoclean && chown 0 /var/log/munge/munged.log
# install slurm
RUN mkdir -p /etc/slurm && \
    mkdir -p /usr/lib64 && \
    mkdir -p /var/log/slurm && \
    mkdir -p /var/spool/slurmd && \
    wget "https://github.com/SchedMD/slurm/archive/refs/tags/slurm-${SLURM_VERSION}.tar.gz" && \
    tar -xf "slurm-${SLURM_VERSION}.tar.gz" && \
    cd "slurm-slurm-${SLURM_VERSION}" && \
    ./configure --prefix=/usr/lib64 --sysconfdir=/etc/slurm/ && \
    make install && \
    cd .. && \
    rm -rf "slurm-slurm-${SLURM_VERSION}" && \
    rm "slurm-${SLURM_VERSION}.tar.gz"
# install go deps
RUN arch="" && \
    if [ `uname -m` == "aarch64" ]; then arch="arm64"; else arch="amd64";fi && \
    wget "https://go.dev/dl/go1.20.12.linux-${arch}.tar.gz" && \
    tar -C /usr/local -xzf "go1.20.12.linux-${arch}.tar.gz" && \
    rm "go1.20.12.linux-${arch}.tar.gz" && \
    mkdir /src

# default wrapper deps for e2e tests
RUN pip install -U pip requests psutil
WORKDIR /src
RUN cargo install just
# load project and cluster configs
ADD . .
RUN cp init_cgroup.conf /etc/slurm/cgroup.conf && \
    cp init_slurm.conf /etc/slurm/slurm.conf
ARG USER=$USER
