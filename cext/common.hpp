// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0
#ifndef COMMON_SCRAPE_INCLUDED_
#define COMMON_SCRAPE_INCLUDED_
struct CextPromMetric {};

struct CextScraper {
	static constexpr int MB = 1000000;
	virtual int Scrape() = 0;
	virtual int IterNext(CextPromMetric *metric) = 0;
	virtual void IterReset() = 0;
};

#endif
