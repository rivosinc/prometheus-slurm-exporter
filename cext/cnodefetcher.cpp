// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
#include <stdlib.h>
#include <sstream>
#include <iostream>
#include <stdexcept>
#include <utility>
#include <slurm/slurm.h>
#include "cnodefetcher.hpp"

PromNodeMetric::PromNodeMetric(node_info_t &node_ptr)
{
    node_info = node_ptr;
    int err = slurm_get_select_nodeinfo(node_info.select_nodeinfo,
                                        SELECT_NODEDATA_SUBCNT,
                                        NODE_STATE_ALLOCATED,
                                        &alloc_cpus);
    if (err)
        cout << "WARNING: failed to enrich alloc cpu data\n";
    err += slurm_get_select_nodeinfo(node_info.select_nodeinfo,
                                     SELECT_NODEDATA_MEM_ALLOC,
                                     NODE_STATE_ALLOCATED,
                                     &alloc_mem);
    if (err)
        cout << "WARNING: failed to enrich alloc mem data\n";
    printf("instantiated alloc mem %ld\n", alloc_mem);
}

PromNodeMetric::PromNodeMetric()
{
    node_info = node_info_t();
}

string PromNodeMetric::GetHostname()
{
    return node_info.node_hostname;
}

string PromNodeMetric::GetPartitions()
{
    return node_info.partitions;
}

double PromNodeMetric::GetCpuLoad()
{
    return (double)node_info.cpu_load / 100;
}

double PromNodeMetric::GetCpus()
{
    return (double)node_info.cpus;
}

double PromNodeMetric::GetFreeMem()
{
    return (double)node_info.free_mem * MB;
}

double PromNodeMetric::GetRealMemory()
{
    return (double)node_info.real_memory * MB;
}

double PromNodeMetric::GetWeight()
{
    return (double)node_info.weight;
}

double PromNodeMetric::GetAllocCpus()
{
    return (double)alloc_cpus;
}

double PromNodeMetric::GetAllocMem()
{
    printf("getter alloc mem %ld\n", *(uint64_t*)alloc_mem);
    return (double)alloc_mem * MB;
}

uint64_t PromNodeMetric::GetNodeState()
{
    return (uint64_t)node_info.node_state;
}

// destruction should happen slurm_free_node_info_msg not via individual destructors
PromNodeMetric::~PromNodeMetric() {}

NodeMetricScraper::~NodeMetricScraper()
{
    if (new_node_ptr)
        slurm_free_node_info_msg(new_node_ptr);
    if (old_node_ptr != new_node_ptr && old_node_ptr)
        slurm_free_node_info_msg(old_node_ptr);
    old_node_ptr = nullptr;
    new_node_ptr = nullptr;
    if (new_part_ptr)
        slurm_free_partition_info_msg(new_part_ptr);
    if (old_part_ptr != new_part_ptr && old_part_ptr)
        slurm_free_partition_info_msg(old_part_ptr);
    old_part_ptr = nullptr;
    new_part_ptr = nullptr;
    slurm_fini();
}

int NodeMetricScraper::Scrape()
{
    int error_code;
    time_t part_update_at, node_update_at;
    part_update_at = old_part_ptr ? old_part_ptr->last_update : (time_t) nullptr;
    error_code = slurm_load_partitions(part_update_at, &new_part_ptr, SHOW_ALL);
    if (SLURM_SUCCESS != error_code && SLURM_NO_CHANGE_IN_DATA == slurm_get_errno())
    {
        error_code = SLURM_SUCCESS;
        new_part_ptr = old_part_ptr;
    }
    if (SLURM_SUCCESS != error_code)
        return slurm_get_errno();
    node_update_at = old_node_ptr ? old_node_ptr->last_update : (time_t) nullptr;
    error_code = slurm_load_node(node_update_at, &new_node_ptr, SHOW_ALL);
    if (SLURM_SUCCESS != error_code && SLURM_NO_CHANGE_IN_DATA == slurm_get_errno())
    {
        error_code = SLURM_SUCCESS;
        new_node_ptr = old_node_ptr;
    }
    if (SLURM_SUCCESS != error_code)
        return error_code;
    // enrich with node info
    slurm_populate_node_partitions(new_node_ptr, new_part_ptr);
    if (old_node_ptr && old_node_ptr != new_node_ptr){
        for (int i = 0; i < old_node_ptr->record_count; i++) {
            node_info_t stale_node_info = old_node_ptr->node_array[i];
            enriched_metrics.erase(stale_node_info.node_hostname);
        }
        slurm_free_node_info_msg(old_node_ptr);
    }
    if (old_part_ptr != new_part_ptr)
        slurm_free_partition_info_msg(old_part_ptr);
    int alloc_errs = 0;
    for (int i = 0; i < new_node_ptr->record_count; i++)
    {
        PromNodeMetric metric(new_node_ptr->node_array[i]);
        enriched_metrics[metric.GetHostname()] = metric;
    }

    old_node_ptr = new_node_ptr;
    old_part_ptr = new_part_ptr;
    return SLURM_SUCCESS;
}

void NodeMetricScraper::Print()
{
    cout << "NodeMetrics: [";
    for (auto const &p : enriched_metrics)
        cout << "{" << p.first << "},";
    cout << "]" << endl;
}

int NodeMetricScraper::IterNext(CextPromMetric *metric)
{
    if (it == enriched_metrics.cend())
        return SLURM_ERROR;
    *metric = it->second;
    it++;
    return SLURM_SUCCESS;
}

void NodeMetricScraper::IterReset()
{
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
