// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

#include <slurm/slurm.h>
#include <string>
#include <map>

using namespace std;

class PromJobMetric
{
    slurm_job_info_t job_info;

public:
    PromJobMetric(slurm_job_info_t &job_ref);
    PromJobMetric();
    ~PromJobMetric();
    string GetAccount();
    int GetJobId();
    double GetEndTime();
    double GetAllocCpus();
    double GetAllocMem();
    int GetJobState();
    string GetPartitions();
    string GetUserName();
};

class JobMetricScraper
{
private:
    job_info_msg_t *new_job_ptr, *old_job_ptr;
    map<int, PromJobMetric> job_metric_map;
    map<int, PromJobMetric>::const_iterator it;

public:
    JobMetricScraper(string conf);
    int CollectJobInfo();
    int IterNext(PromJobMetric *metric);
    void IterReset();
};
