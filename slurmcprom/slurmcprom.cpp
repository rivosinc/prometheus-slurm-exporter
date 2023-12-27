// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
#include <stdlib.h>
#include <sstream>
#include <iostream>
#include <vector>
#include <slurm/slurm.h>
#include "slurmcprom.hpp"

PromNodeMetric::PromNodeMetric() {}
PromNodeMetric::~PromNodeMetric() {}

NodeMetricScraper::NodeMetricScraper(string conf)
{
    if (conf == "")
        slurm_init(NULL);
    else
        slurm_init(conf.c_str());
    new_node_ptr = NULL;
    old_node_ptr = NULL;
    new_part_ptr = NULL;
    old_part_ptr = NULL;
}

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

int NodeMetricScraper::enrichNodeInfo(node_info_t *node_ptr) {
    string hostname(node_ptr->name);
    string partitions(node_ptr->partitions);
    enrichedMetrics[hostname].Hostname = hostname;
    enrichedMetrics[hostname].Cpus = node_ptr->cpus;
    enrichedMetrics[hostname].RealMemory = node_ptr->real_memory;
    enrichedMetrics[hostname].FreeMem = node_ptr->free_mem;
    enrichedMetrics[hostname].Partitions = partitions;

    int err = slurm_get_select_nodeinfo(node_ptr->select_nodeinfo,
				     SELECT_NODEDATA_SUBCNT,
				     NODE_STATE_ALLOCATED,
				     &enrichedMetrics[hostname].AllocCpus);
    if (err) cout << "WARNING: failed to enrich alloc cpu data\n";
    err += slurm_get_select_nodeinfo(node_ptr->select_nodeinfo,
				     SELECT_NODEDATA_MEM_ALLOC,
				     NODE_STATE_ALLOCATED,
				     &enrichedMetrics[hostname].AllocMem);
    if (err) cout << "WARNING: failed to enrich alloc mem data\n";
    return SLURM_SUCCESS;
}

size_t NodeMetricScraper::NumMetrics() {
    return enrichedMetrics.size();
}

int NodeMetricScraper::CollectNodeInfo() {
    int error_code;
    if (old_node_ptr != nullptr) {
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
    slurm_free_node_info_msg(old_node_ptr);
    old_node_ptr = new_node_ptr;
    // enrich with node info
    slurm_populate_node_partitions(new_node_ptr, new_part_ptr);
    int alloc_errs = 0;
    for (int i = 0; i < new_node_ptr->record_count; i++)
        alloc_errs += enrichNodeInfo(&new_node_ptr->node_array[i]);
    if (alloc_errs) std::cout << "enable to enrich " << alloc_errs << " with fail stats";
    return slurm_get_errno();
}

void NodeMetricScraper::Print() {
    for (auto const& p: enrichedMetrics)
        std::cout << p.first << ":" << p.second.Partitions << "\n";
}
