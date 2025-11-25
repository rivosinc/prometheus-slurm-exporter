// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFairShareFetch(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	fetcher := FairShareFetcher{
		scraper: &StringByteScraper{
			msg: `parentaccount1|
account1|1.000000
account2|15.5
account2.sub1|inf
account2.sub2|inf
account3|0.5
account4|inf
account5|2.25
`,
		},
		errorCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
		cache:        NewAtomicThrottledCache[FairShareMetric](10),
	}
	fairshareMetrics, err := fetcher.fetchFromCli()
	require.NoError(err)

	// Should get 4 accounts with numeric values (skipping empty and inf)
	assert.Len(fairshareMetrics, 4)

	expectedAccounts := map[string]float64{
		"account1": 1.000000,
		"account2": 15.5,
		"account3": 0.5,
		"account5": 2.25,
	}

	for _, metric := range fairshareMetrics {
		expectedValue, exists := expectedAccounts[metric.Account]
		assert.True(exists, "Unexpected account: %s", metric.Account)
		assert.Equal(expectedValue, metric.FairShare, "Account %s fairshare mismatch", metric.Account)
	}
}

func TestFairShareSkipsInf(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	fetcher := FairShareFetcher{
		scraper: &StringByteScraper{
			msg: "account1|inf\naccount2|0.5\n",
		},
		errorCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
		cache:        NewAtomicThrottledCache[FairShareMetric](10),
	}
	fairshareMetrics, err := fetcher.fetchFromCli()
	require.NoError(err)
	assert.Len(fairshareMetrics, 1)
	assert.Equal("account2", fairshareMetrics[0].Account)
	assert.Equal(0.5, fairshareMetrics[0].FairShare)
}

func TestFairShareSkipsEmpty(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	fetcher := FairShareFetcher{
		scraper: &StringByteScraper{
			msg: "account1|\naccount2|0.5\n",
		},
		errorCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
		cache:        NewAtomicThrottledCache[FairShareMetric](10),
	}
	fairshareMetrics, err := fetcher.fetchFromCli()
	require.NoError(err)
	assert.Len(fairshareMetrics, 1)
	assert.Equal("account2", fairshareMetrics[0].Account)
	assert.Equal(0.5, fairshareMetrics[0].FairShare)
}

func TestFairShareInvalidNumber(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	fetcher := FairShareFetcher{
		scraper: &StringByteScraper{
			msg: "account1|invalid\naccount2|0.5\n",
		},
		errorCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
		cache:        NewAtomicThrottledCache[FairShareMetric](10),
	}
	fairshareMetrics, err := fetcher.fetchFromCli()
	require.NoError(err)
	assert.Len(fairshareMetrics, 1)
	assert.Equal("account2", fairshareMetrics[0].Account)
}
