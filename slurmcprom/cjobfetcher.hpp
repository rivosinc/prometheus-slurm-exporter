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
    struct job_resrcs
    {
        bitstr_t *core_bitmap;
        bitstr_t *core_bitmap_used;
        uint32_t cpu_array_cnt;
        uint16_t *cpu_array_value;
        uint32_t *cpu_array_reps;
        uint16_t *cpus;
        uint16_t *cpus_used;
        uint16_t *cores_per_socket;
        uint16_t cr_type;
        uint64_t *memory_allocated;
        uint64_t *memory_used;
        uint32_t nhosts;
        bitstr_t *node_bitmap;
        uint32_t node_req;
        char *nodes;
        uint32_t ncpus;
        uint32_t *sock_core_rep_count;
        uint16_t *sockets_per_node;
        uint16_t *tasks_per_node;
        uint16_t threads_per_core;
        uint8_t whole_node;
    };

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
