// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

#include <slurm/slurm.h>
#include <string>
#include <map>
#include <vector>
#include <iostream>
#include <memory>
using namespace std;

class PromNodeMetric
{
private:
    node_info_t node_info;
    uint16_t alloc_cpus;
    uint64_t alloc_mem;

public:
    PromNodeMetric(node_info_t &node_info);
    PromNodeMetric();
    ~PromNodeMetric();
    // return double to easily coerce to go float64
    double GetCpus();
    double GetRealMemory();
    double GetFreeMem();
    uint64_t GetNodeState();
    double GetAllocCpus();
    double GetAllocMem();
    double GetWeight();
    double GetCpuLoad();
    string GetHostname();
    string GetPartitions();
};

struct NodeMetricScraper
{
private:
    partition_info_msg_t *new_part_ptr, *old_part_ptr;
    node_info_msg_t *new_node_ptr, *old_node_ptr;
    map<string, PromNodeMetric> enriched_metrics;
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
