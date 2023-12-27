// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

#include <slurm/slurm.h>
#include <string>
#include <map>
#include <vector>
#include <iostream>

using namespace std;

struct PromNodeMetric
{
    PromNodeMetric();
    ~PromNodeMetric();
    string Hostname;
    uint16_t Cpus;
    uint64_t RealMemory;
    uint64_t FreeMem;
    // csv formated list
    string Partitions;
    uint32_t NodeState;
    uint16_t AllocCpus;
    uint64_t AllocMem;
    uint32_t Weight;
    uint32_t CpuLoad;
};

struct NodeMetricScraper
{
private:
    partition_info_msg_t *new_part_ptr, *old_part_ptr;
    node_info_msg_t *new_node_ptr, *old_node_ptr;
    int enrichNodeInfo(node_info_t *node_info);
    map<string, PromNodeMetric> enrichedMetrics;
    map<string, PromNodeMetric>::const_iterator it;
public:
    NodeMetricScraper(string conf);
    ~NodeMetricScraper();
    int CollectNodeInfo();
    void Print();
    // public iterator exposure. Swig doesn't properly expose the iterator subclass
    // expects to be IterReset always
    int IterNext(PromNodeMetric *metric);
    void IterReset();
};
