// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

#include <vector>
#include <string>
#include <chrono>
using namespace std;

struct TestWrapper
{
    TestWrapper(string testName, int errnum);
    string TestName;
    int Passed;
};

class TestHandler
{
    vector<TestWrapper> tests;
    chrono::system_clock::time_point start;

public:
    void Register(TestWrapper wrp);
    int Report();
    TestHandler();
};
