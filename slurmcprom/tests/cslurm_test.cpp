// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

#include <iostream>
#include <cslurm.hpp>
#include <assert.h>

using namespace std;

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
    cout << "all tests passed!\n";
    return 0;
}
