// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

#include <cslurm.hpp>
#include <assert.h>

void TestNodeCollectorNullConf() {
    auto fetcher = NodeMetricFetcher("");
    assert(fetcher.NumMetrics() == 0);
    fetcher.CollectNodeInfo();
    assert(fetcher.NumMetrics() > 0);
}

void TestNodeCollectorProvidedConf() {
    auto fetcher = NodeMetricFetcher("/etc/slurm/slurm.conf");
    assert(fetcher.NumMetrics() == 0);
    fetcher.CollectNodeInfo();
    assert(fetcher.NumMetrics() > 0);
}

int main() {
    TestNodeCollectorNullConf();
    TestNodeCollectorProvidedConf();
    return 0;
}
