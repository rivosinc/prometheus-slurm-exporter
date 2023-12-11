// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

#include <slurm/slurm.h>
#include <string>
#include <unordered_map>
#include <iostream>

class PromNodeMetric
{
private:

public:
    PromNodeMetric() {};
    ~PromNodeMetric() {};
    std::string Hostname;
    uint16_t Cpus;
    uint64_t RealMemory;
    uint64_t FreeMem;
    // csv formated list
    std::string Partitions;
    uint32_t NodeState;
    uint16_t AllocCpus;
    uint64_t AllocMem;
    uint32_t Weight;
    uint32_t CpuLoad;
};

class MetricExporter
{
private:
    partition_info_msg_t *new_part_ptr, *old_part_ptr;
    node_info_msg_t *new_node_ptr, *old_node_ptr;
    std::unordered_map<std::string, PromNodeMetric> enrichedMetrics;
    int enrichNodeInfo(node_info_t *node_info);
public:
    MetricExporter();
    ~MetricExporter();
    int CollectNodeInfo();
    size_t NumMetrics();
    void Print();
};
