// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
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
    public:
    void Register(TestWrapper wrp);
    void Report();
};

void TestHandler::Register(TestWrapper wrp)
{
    tests.push_back(wrp);
}

void TestHandler::Report()
{
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
}

void NodeMetricScraper_CollectHappy(TestHandler &th) {
    auto scraper = NodeMetricScraper("");
    int errnum = scraper.CollectNodeInfo();
    string testname("Node Metric Scraper Collect Happy");
    th.Register(TestWrapper(testname, errnum));
}

void NodeMetricScrape_EnrichedMetricsView(TestHandler &th) {
    auto scraper = NodeMetricScraper("");
    scraper.Print();
    scraper.CollectNodeInfo();
    scraper.Print();
    for (auto const& p: scraper.EnrichedMetricsView()) {
        cout << p.Hostname << endl;
    }
    auto metrics = scraper.EnrichedMetricsView();
    th.Register(TestWrapper("Node Metrics Vector View Happy", (int)(metrics.size() != scraper.NumMetrics())));
}

int main() {
    TestHandler handler;
    NodeMetricScraper_CollectHappy(handler);
    NodeMetricScrape_EnrichedMetricsView(handler);
    handler.Report();
}
