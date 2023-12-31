// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

#include <slurm/slurm.h>
#include <string>

using namespace std;

class PromJobMetric
{
    slurm_job_info_t job_info;

public:
    PromJobMetric(slurm_job_info_t &job_ref);
    PromJobMetric();
    ~PromJobMetric();
    string GetAccount();
    double GetJobId();
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

public:
    JobMetricScraper(string conf);
    ~JobMetricScraper();
    int CollectJobInfo();
    int IterNext(PromJobMetric *metric);
    void IterReset();
};
