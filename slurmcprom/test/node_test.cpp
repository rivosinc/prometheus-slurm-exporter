// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
#include <chrono>
#include <slurmcprom.hpp>
#include <assert.h>


struct TestWrapper {
    TestWrapper(string testName, int errnum);
    string TestName;
    int Errno;
};

TestWrapper::TestWrapper(string testName, int errnum) {
    TestName = testName;
    Errno = errnum;
}

class TestHandler
{
    vector<TestWrapper> tests;
    chrono::system_clock::time_point start;
    public:
    void Register(TestWrapper wrp);
    void Report();
    TestHandler();
};

TestHandler::TestHandler() {
    start = chrono::high_resolution_clock::now();
}

void TestHandler::Register(TestWrapper wrp)
{
    tests.push_back(wrp);
}

void TestHandler::Report()
{
    auto duration = chrono::duration_cast<chrono::milliseconds>(chrono::high_resolution_clock::now() - start);
    int fails = 0;
    for (auto const& tw : tests) {
        if (!tw.Errno) continue;
        fails++;
        cout << "Test " << tw.TestName;
        cout << " errored with code " << tw.Errno << endl;
    }
    cout << "Summary: " << endl;
    cout << "    Ran: " << tests.size() << endl;
    if (fails)
        cout << "    Failed: " << fails << endl;
    cout << "    Passed: " << tests.size() - fails << endl;
    cout << "Took " << duration.count() << "ms" << endl;
}

void NodeMetricScraper_CollectHappy(TestHandler &th) {
    auto scraper = NodeMetricScraper("");
    int errnum = scraper.CollectNodeInfo();
    string testname("Node Metric Scraper Collect Happy");
    th.Register(TestWrapper(testname, errnum));
}

void NodeMetricScraper_CollectTwice(TestHandler &th) {
    auto scraper = NodeMetricScraper("");
    int errnum = scraper.CollectNodeInfo();
    int errnum2 = scraper.CollectNodeInfo();
    string testname("Node Metric Scraper Collect Happy");
    th.Register(TestWrapper(testname, errnum + errnum2));
}

void TestIter(TestHandler &th) {
    auto scraper = NodeMetricScraper("");
    int errnum = scraper.CollectNodeInfo();
    PromNodeMetric metric;
    int count = 0;
    while (scraper.IterNext(&metric) == 0) {
        cout << metric.Hostname << endl;
        count++;
    }
    string testname("Test Map Iteration After Collection");
    th.Register(TestWrapper(testname, count > 0));
}

int main() {
    TestHandler handler;
    // NodeMetricScraper_CollectHappy(handler);
    TestIter(handler);
    handler.Report();
}
