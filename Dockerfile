FROM ubuntu:20.04

ARG DEBIAN_FRONTEND=noninteractive
ARG LD_LIRBARY_PATH=/usr/lib64/lib/slurm
RUN apt-get update -y && apt-get install -y build-essential \
    libjson-c-dev \
    gdb \
    python3-venv \
    python-is-python3 \
    tmux \
    vim \
    wget
# munge
RUN printf '#!/bin/sh\nexit 0' > /usr/sbin/policy-rc.d && apt-get install -y libmunge-dev munge && chown 0 /var/log/munge/munged.log
# install slurm
RUN mkdir -p /etc/slurm && \
    mkdir -p /usr/lib64 && \
    mkdir -p /var/spool/slurmd && \
    wget https://github.com/SchedMD/slurm/archive/refs/tags/slurm-23-02-5-1.tar.gz && \
    tar -xf slurm-23-02-5-1.tar.gz && \
    cd slurm-slurm-23-02-5-1 && \
    ./configure --prefix=/usr/lib64/ --sysconfdir=/etc/slurm/ && \
    make install

ENV PATH=/usr/lib64/bin:/usr/lib64/sbin:$PATH
