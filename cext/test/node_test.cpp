// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
#include <chrono>
#include <cnodefetcher.hpp>
#include <test/test_util.hpp>
#include <assert.h>
#include <memory>
#include <cstdio>

void NodeMetricScraper_CollectHappy(TestHandler &th)
{
    auto scraper = NodeMetricScraper("");
    int errnum = scraper.CollectNodeInfo();
    string testname("Node Metric Scraper Collect Happy");
    th.Register(TestWrapper(testname, errnum == 0));
}

void NodeMetricScraper_CollectTwice(TestHandler &th)
{
    auto scraper = NodeMetricScraper("");
    int errnum = scraper.CollectNodeInfo();
    int errnum2 = scraper.CollectNodeInfo();
    string testname("Node Metric Scraper Cache hit Works");
    th.Register(TestWrapper(testname, errnum == 0 && errnum2 == 0));
}

void NodeMetricScraper_CollectThrice(TestHandler &th)
{
    auto scraper = NodeMetricScraper("");
    int errnum = scraper.CollectNodeInfo();
    int errnum2 = scraper.CollectNodeInfo();
    int errnum3 = scraper.CollectNodeInfo();
    string testname("Node Metric Catch Seg");
    th.Register(TestWrapper(testname, errnum == 0 && errnum2 == 0 && errnum3 == 0));
}

void TestIter(TestHandler &th)
{
    NodeMetricScraper scraper("");
    int errnum = scraper.CollectNodeInfo();
    scraper.IterReset();
    auto metric = new PromNodeMetric;
    int count = 0;
    assert(errnum == 0);
    while (scraper.IterNext(metric) == 0)
        count++;
    string testname("Test Map Iteration After Collection");
    th.Register(TestWrapper(testname, count > 0));
}

void TestIter_Empty(TestHandler &th)
{
    auto scraper = NodeMetricScraper("");
    auto metric = new PromNodeMetric;
    string testname("Test Map Iteration Before Collection");
    th.Register(TestWrapper(testname, scraper.IterNext(metric) != 0));
}

int main()
{
    TestHandler handler;
    NodeMetricScraper_CollectHappy(handler);
    NodeMetricScraper_CollectTwice(handler);
    NodeMetricScraper_CollectThrice(handler);
    TestIter(handler);
    TestIter_Empty(handler);
    return handler.Report();
}
