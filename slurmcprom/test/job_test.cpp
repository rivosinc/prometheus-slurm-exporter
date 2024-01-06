// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
#include <chrono>
#include <cjobfetcher.hpp>
#include <test/test_util.hpp>
#include <assert.h>
#include <memory>
#include <iostream>

using namespace std;

void JobMetricScraper_CollectHappy(TestHandler &th)
{
    auto scraper = JobMetricScraper("");
    int errnum = scraper.CollectJobInfo();
    string testname("Node Metric Scraper Collect Happy");
    th.Register(TestWrapper(testname, errnum == 0));
}

void JobMetricScraper_CollectTwice(TestHandler &th)
{
    auto scraper = JobMetricScraper("");
    int errnum = scraper.CollectJobInfo();
    int errnum2 = scraper.CollectJobInfo();
    string testname("Node Metric Scraper Cache hit Works");
    th.Register(TestWrapper(testname, errnum == 0 && errnum2 == 0));
}

void JobMetricScraper_CollectThrice(TestHandler &th)
{
    auto scraper = JobMetricScraper("");
    int errnum = scraper.CollectJobInfo();
    int errnum2 = scraper.CollectJobInfo();
    int errnum3 = scraper.CollectJobInfo();
    cout << "end" << endl;
    string testname("Node Metric Catch Seg");
    th.Register(TestWrapper(testname, errnum == 0 && errnum2 == 0 && errnum3 == 0));
}

void TestIter(TestHandler &th)
{
    JobMetricScraper scraper("");
    int errnum = scraper.CollectJobInfo();
    scraper.IterReset();
    auto metric = new PromJobMetric;
    int count = 0;
    assert(errnum == 0);
    while (scraper.IterNext(metric) == 0)
        count++;
    string testname("Test Map Iteration After Collection");
    th.Register(TestWrapper(testname, count > 0));
}

void TestIter_Empty(TestHandler &th)
{
    auto scraper = JobMetricScraper("");
    auto metric = new PromJobMetric;
    string testname("Test Map Iteration Before Collection");
    th.Register(TestWrapper(testname, scraper.IterNext(metric) != 0));
}

void TestGetAllocCpus(TestHandler &th)
{
    auto scraper = JobMetricScraper("");
    scraper.CollectJobInfo();
    auto metric = PromJobMetric();
    scraper.IterReset();
    scraper.IterNext(&metric);

    string testname("Test Get Alloc Cpus");
    int cpus = metric.GetAllocCpus();
    printf("cpus = %d\n", cpus);
    // this is identical to whats reported by squeue --json ??
    // with a running job
    th.Register(TestWrapper(testname, cpus == 1));
}

void TestGetAllocMem(TestHandler &th)
{
    auto scraper = JobMetricScraper("");
    scraper.CollectJobInfo();
    auto metric = PromJobMetric();
    scraper.IterReset();
    scraper.IterNext(&metric);

    string testname("Test Get Alloc Mem");
    int mem = metric.GetAllocMem();
    printf("mem = %d\n", mem);
    // this is identical to whats reported by squeue --json ??
    // with a running job
    th.Register(TestWrapper(testname, mem == 0));
}

int main()
{
    TestHandler handler;
    JobMetricScraper_CollectHappy(handler);
    JobMetricScraper_CollectTwice(handler);
    JobMetricScraper_CollectThrice(handler);
    TestGetAllocCpus(handler);
    TestGetAllocMem(handler);
    TestIter(handler);
    TestIter_Empty(handler);
    return handler.Report();
}
