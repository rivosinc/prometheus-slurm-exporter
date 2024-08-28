#!/bin/bash

systemctl daemon-reload
systemctl enable --now prometheus-slurm-exporter.service
