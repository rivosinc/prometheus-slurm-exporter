// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

#include <slurm/slurm.h>
#include <cjobfetcher.hpp>

PromJobMetric::PromJobMetric(slurm_job_info_t &job_ref)
{
    job_info = job_ref;
}

PromJobMetric::PromJobMetric()
{
    job_info = slurm_job_info_t();
}

PromJobMetric::~PromJobMetric() {}

string PromJobMetric::GetAccount()
{
    return job_info.account ? job_info.account : "nil";
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
    return job_info.num_cpus;
}

double PromJobMetric::GetAllocMem()
{
    return job_info.pn_min_memory;
}

int PromJobMetric::GetJobState()
{
    return job_info.job_state;
}

string PromJobMetric::GetPartitions()
{
    return job_info.partition ? job_info.partition : "nil";
}

string PromJobMetric::GetUserName()
{
    return job_info.user_name ? job_info.user_name : "nil";
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
}

int JobMetricScraper::CollectJobInfo()
{
    time_t updated_at = old_job_ptr ? old_job_ptr->last_update : (time_t) nullptr;
    int error_code = slurm_load_jobs(updated_at, &new_job_ptr, SHOW_ALL);
    if (SLURM_SUCCESS != error_code && SLURM_NO_CHANGE_IN_DATA == slurm_get_errno())
    {
        error_code = SLURM_SUCCESS;
        new_job_ptr = old_job_ptr;
    }
    if (SLURM_SUCCESS != error_code)
        return slurm_get_errno();
    for (int i = 0; i < new_job_ptr->record_count; i++)
    {
        PromJobMetric metric(new_job_ptr->job_array[i]);
        job_metric_map[metric.GetJobId()] = metric;
    }
    if (new_job_ptr == old_job_ptr)
        slurm_free_job_info_msg(old_job_ptr);
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
