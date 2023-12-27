// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
#include <stdlib.h>
#include <sstream>
#include <iostream>
#include <stdexcept>
#include <utility>
#include <slurm/slurm.h>
#include "slurmcprom.hpp"

PromNodeMetric::PromNodeMetric(node_info_t &node_ptr) {
    node_info = node_ptr;
    int err = slurm_get_select_nodeinfo(node_info.select_nodeinfo,
                    SELECT_NODEDATA_SUBCNT,
                    NODE_STATE_ALLOCATED,
                    &alloc_cpus);
    if (err) cout << "WARNING: failed to enrich alloc cpu data\n";
    err += slurm_get_select_nodeinfo(node_info.select_nodeinfo,
				     SELECT_NODEDATA_MEM_ALLOC,
				     NODE_STATE_ALLOCATED,
				     &alloc_mem);
    if (err) cout << "WARNING: failed to enrich alloc mem data\n";
}

PromNodeMetric::PromNodeMetric() {
    node_info_t *dummy = (node_info_t*)malloc(sizeof(node_info_msg_t));

}

string PromNodeMetric::GetHostname() {
    string hostname(node_info.node_hostname);
    return hostname;
}

string PromNodeMetric::GetPartitions() {
    string partitions(node_info.partitions);
    return partitions;
}

double PromNodeMetric::GetCpuLoad() {
    return (double) node_info.cpu_load;
}

double PromNodeMetric::GetCpus() {
    return (double) node_info.cpus;
}

double PromNodeMetric::GetFreeMem() {
    return (double) node_info.free_mem;
}

double PromNodeMetric::GetRealMemory() {
    return (double) node_info.real_memory;
}

double PromNodeMetric::GetWeight() {
    return (double) node_info.weight;
}

double PromNodeMetric::GetAllocCpus() {
    return (double) alloc_cpus;
}

double PromNodeMetric::GetAllocMem() {
    return (double) alloc_mem;
}

// destruction should happen slurm_free_node_info_msg not via individual destructors
PromNodeMetric::~PromNodeMetric() {}

NodeMetricScraper::~NodeMetricScraper() {
    if (new_node_ptr)
        slurm_free_node_info_msg(new_node_ptr);
    if (old_node_ptr != new_node_ptr && old_node_ptr)
        slurm_free_node_info_msg(old_node_ptr);
    old_node_ptr = NULL;
    new_node_ptr = NULL;
    if (new_part_ptr)
        slurm_free_partition_info_msg(new_part_ptr);
    if (old_part_ptr != new_part_ptr && old_part_ptr)
        slurm_free_partition_info_msg(old_part_ptr);
    old_part_ptr = NULL;
    new_part_ptr = NULL;
    slurm_fini();
}

int NodeMetricScraper::CollectNodeInfo() {
    int error_code;
    if (old_node_ptr && old_part_ptr) {
        error_code = slurm_load_partitions(old_part_ptr->last_update, &new_part_ptr, SHOW_ALL);
        if (SLURM_SUCCESS == error_code)
            slurm_free_partition_info_msg(old_part_ptr);
        else if (SLURM_NO_CHANGE_IN_DATA == slurm_get_errno()) {
            new_part_ptr = old_part_ptr;
            return SLURM_SUCCESS;
        }
    } else
       error_code = slurm_load_partitions((time_t) nullptr, &new_part_ptr, SHOW_ALL);
    if (SLURM_SUCCESS != error_code) return error_code;
    if (old_node_ptr != nullptr)
        error_code = slurm_load_node(old_node_ptr->last_update, &new_node_ptr, SHOW_ALL);
    else
        error_code = slurm_load_node((time_t) nullptr, &new_node_ptr, SHOW_ALL);
    if (SLURM_SUCCESS != error_code)
        return error_code;
    // enrich with node info
    slurm_populate_node_partitions(new_node_ptr, new_part_ptr);
    int alloc_errs = 0;
    for (int i = 0; i < new_node_ptr->record_count; i++) {
        PromNodeMetric metric(new_node_ptr->node_array[i]);
        enriched_metrics.insert_or_assign(metric.GetHostname(), metric);
    }
    slurm_free_node_info_msg(old_node_ptr);
    slurm_free_partition_info_msg(old_part_ptr);
    old_node_ptr = new_node_ptr;
    old_part_ptr = new_part_ptr;
    return slurm_get_errno();
}

void NodeMetricScraper::Print() {
    cout << "NodeMetrics: [";
    for (auto const& p: enriched_metrics)
        cout << "{" << p.first << "},";
    cout << "]" << endl;
}

int NodeMetricScraper::IterNext(PromNodeMetric *metric) {
    if (it == enriched_metrics.cend())
        return 1;
    *metric = it->second;
    it++;
    return 0;
}

void NodeMetricScraper::IterReset() {
    it = enriched_metrics.cbegin();
}

NodeMetricScraper::NodeMetricScraper(string conf)
{
    if (conf == "")
        slurm_init(nullptr);
    else
        slurm_init(conf.c_str());
    new_node_ptr = nullptr;
    old_node_ptr = nullptr;
    new_part_ptr = nullptr;
    old_part_ptr = nullptr;
    IterReset();
}
