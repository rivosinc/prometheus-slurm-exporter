// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
#include <chrono>
#include <slurmcprom.hpp>
#include <assert.h>
#include <memory>


struct TestWrapper {
    TestWrapper(string testName, int errnum);
    string TestName;
    int Passed;
};

TestWrapper::TestWrapper(string testName, int errnum) {
    TestName = testName;
    Passed = errnum;
}

class TestHandler
{
    vector<TestWrapper> tests;
    chrono::system_clock::time_point start;
    public:
    void Register(TestWrapper wrp);
    int Report();
    TestHandler();
};

TestHandler::TestHandler() {
    start = chrono::high_resolution_clock::now();
}

void TestHandler::Register(TestWrapper wrp)
{
    tests.push_back(wrp);
}

int TestHandler::Report()
{
    auto duration = chrono::duration_cast<chrono::milliseconds>(chrono::high_resolution_clock::now() - start);
    int fails = 0;
    for (auto const& tw : tests) {
        if (tw.Passed) continue;
        fails++;
        cout << "Test " << tw.TestName;
        cout << " errored with code " << tw.Passed << endl;
    }
    cout << "Summary: " << endl;
    cout << "    Ran: " << tests.size() << endl;
    if (fails)
        cout << "    Failed: " << fails << endl;
    cout << "    Passed: " << tests.size() - fails << endl;
    cout << "Took " << duration.count() << "ms" << endl;
    return fails;
}

void NodeMetricScraper_CollectHappy(TestHandler &th) {
    auto scraper = NodeMetricScraper("");
    int errnum = scraper.CollectNodeInfo();
    string testname("Node Metric Scraper Collect Happy");
    th.Register(TestWrapper(testname, errnum == 0));
}

void NodeMetricScraper_CollectTwice(TestHandler &th) {
    auto scraper = NodeMetricScraper("");
    int errnum = scraper.CollectNodeInfo();
    int errnum2 = scraper.CollectNodeInfo();
    string testname("Node Metric Scraper Cache hit Works");
    th.Register(TestWrapper(testname, errnum == 0 && errnum2 == 0));
}

void TestIter(TestHandler &th) {
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

void TestIter_Empty(TestHandler &th) {
    auto scraper = NodeMetricScraper("");
    auto metric = new PromNodeMetric;
    string testname("Test Map Iteration Before Collection");
    th.Register(TestWrapper(testname, scraper.IterNext(metric) != 0));
}

int main() {
    TestHandler handler;
    NodeMetricScraper_CollectHappy(handler);
    NodeMetricScraper_CollectTwice(handler);
    TestIter(handler);
    TestIter_Empty(handler);
    return handler.Report();
}
