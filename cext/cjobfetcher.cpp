// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

#include <slurm/slurm.h>
#include <cjobfetcher.hpp>
#include <iostream>

const string STRING_NULL = "(null)";
constexpr int MB = 1000000;

PromJobMetric::PromJobMetric(slurm_job_info_t &job_ref)
{
    job_info = job_ref;
    if ((JOB_STATE_BASE & job_info.job_state) != JOB_RUNNING)
        return;
    slurm_job_cpus_allocated_on_node(job_info.job_resrcs, job_info.nodes);
    int error_code = slurm_get_errno();
    if (SLURM_SUCCESS != error_code && SLURM_NO_CHANGE_IN_DATA != error_code)
        printf("failed to add alloc cpus with errno %d \n", error_code);
}

PromJobMetric::PromJobMetric()
{
    job_info = slurm_job_info_t();
}

PromJobMetric::~PromJobMetric() {}

string PromJobMetric::GetAccount()
{
    if (job_info.account)
        return job_info.account;
    return STRING_NULL;
}

int PromJobMetric::GetJobId()
{
    return job_info.job_id;
}

double PromJobMetric::GetEndTime()
{
    return job_info.end_time;
}

double PromJobMetric::GetAllocCpus()
{
    if (nullptr == job_info.job_resrcs)
        return job_info.pn_min_cpus;
    job_resrcs *resc = (job_resrcs *)job_info.job_resrcs;
    return (double)resc->ncpus;
}

double PromJobMetric::GetAllocMem()
{
    if (job_info.gres_total) {
        cout << "gres total " << job_info.gres_total << "state: " << job_info.job_state << endl;
    }
    if (nullptr == job_info.job_resrcs) {
        cout << "min_mem " << job_info.mem_per_tres << " num nodes " << job_info.num_nodes << "\n";
        return job_info.pn_min_memory * job_info.num_nodes;
    }
    job_resrcs *resc = (job_resrcs *)job_info.job_resrcs;
    uint64_t alloc_mem = 0;
    for (int i = 0; i < resc->nhosts; i++)
        alloc_mem += resc->memory_allocated[i];
    return (double)alloc_mem * MB;
}

int PromJobMetric::GetJobState()
{
    return job_info.job_state;
}

string PromJobMetric::GetPartitions()
{
    return job_info.partition ? job_info.partition : STRING_NULL;
}

string PromJobMetric::GetUserName()
{
    if (0 == job_info.user_id)
        return "root";
    if (nullptr == job_info.user_name)
        return STRING_NULL;
    return job_info.user_name;
}

JobMetricScraper::JobMetricScraper(string conf)
{
    if (conf == "")
    {
        slurm_init(nullptr);
    }
    else
    {
        slurm_init(conf.c_str());
    }
    new_job_ptr = nullptr;
    old_job_ptr = nullptr;
    IterReset();
}

int JobMetricScraper::CollectJobInfo()
{
    time_t updated_at = old_job_ptr ? old_job_ptr->last_update : (time_t) nullptr;
    int error_code = slurm_load_jobs(updated_at, &new_job_ptr, SHOW_DETAIL);
    if (SLURM_SUCCESS != error_code && SLURM_NO_CHANGE_IN_DATA == slurm_get_errno())
    {
        error_code = SLURM_SUCCESS;
        new_job_ptr = old_job_ptr;
    }
    if (SLURM_SUCCESS != error_code)
        return slurm_get_errno();

    // want to ensure stale members aren't kept in the map i.e new job array is a subset of old job array
    // also old_job_array + new_job_array could still be a subset of collection map
    // delete all stale members in map
    if (old_job_ptr && new_job_ptr != old_job_ptr){
        job_metric_map.clear();
        slurm_free_job_info_msg(old_job_ptr);
    }
    // enrich with new members
    for (int i = 0; i < new_job_ptr->record_count; i++)
    {
        slurm_job_info_t job = new_job_ptr->job_array[i];
        PromJobMetric metric(job);
        job_metric_map[metric.GetJobId()] = metric;
    }
    old_job_ptr = new_job_ptr;
    return SLURM_SUCCESS;
}

int JobMetricScraper::IterNext(PromJobMetric *metric)
{
    if (it == job_metric_map.cend())
        return 1;
    *metric = it->second;
    it++;
    return 0;
}

void JobMetricScraper::IterReset()
{
    it = job_metric_map.cbegin();
}
