// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

#include <slurm/slurm.h>
#include <cjobfetcher.hpp>

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
