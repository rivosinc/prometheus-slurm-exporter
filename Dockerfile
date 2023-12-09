# SPDX-FileCopyrightText: 2023 Rivos Inc.
#
# SPDX-License-Identifier: Apache-2.0
FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive
ENV LD_LIRBARY_PATH=/usr/lib64/lib/slurm
ENV PATH=/usr/lib64/bin:/usr/lib64/sbin:/root/.cargo/bin:/usr/local/go/bin:$PATH
RUN apt-get update -y && apt-get install -y build-essential \
    cargo \
    libjson-c-dev \
    gdb \
    python3-venv \
    python-is-python3 \
    python3-pip \
    tmux \
    vim \
    wget
RUN cargo install just
# munge
RUN printf '#!/bin/sh\nexit 0' > /usr/sbin/policy-rc.d && apt-get install -y libmunge-dev munge && chown 0 /var/log/munge/munged.log
# install slurm
RUN mkdir -p /etc/slurm && \
    mkdir -p /usr/lib64 && \
    mkdir -p /var/spool/slurmd && \
    wget https://github.com/SchedMD/slurm/archive/refs/tags/slurm-23-02-5-1.tar.gz && \
    tar -xf slurm-23-02-5-1.tar.gz && \
    cd slurm-slurm-23-02-5-1 && \
    ./configure --prefix=/usr/lib64 --sysconfdir=/etc/slurm/ && \
    make install
# install go deps
RUN arch="" && \
    if [ `uname -m` == "aarch64" ]; then arch="arm64"; else arch="amd64";fi && \
    wget "https://go.dev/dl/go1.20.12.linux-${arch}.tar.gz" && \
    tar -C /usr/local -xzf "go1.20.12.linux-${arch}.tar.gz"

ADD . .
ARG USER=$USER
RUN mv tmp_sconfs/slurm* /etc/slurm && mv tmp_sconfs/munge.key /etc/munge && \
    sed -i '/SlurmUser=/d' /etc/slurm/slurm.conf
