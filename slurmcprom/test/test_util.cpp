// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

#include <test/test_util.hpp>
#include <chrono>
#include <string>
#include <iostream>
using namespace std;

TestWrapper::TestWrapper(string testName, int errnum)
{
    TestName = testName;
    Passed = errnum;
}

TestHandler::TestHandler()
{
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
    for (auto const &tw : tests)
    {
        if (tw.Passed)
            continue;
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
