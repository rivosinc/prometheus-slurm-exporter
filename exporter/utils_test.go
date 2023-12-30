// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slog"
)

const chars string = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand *rand.Rand

func init() {
	seed := time.Now().UnixNano()
	seededRand = rand.New(rand.NewSource(seed))
	slog.Debug(fmt.Sprintf("rand seed: %d", seed))
}

func generateRandString(n int) string {
	randBytes := make([]byte, n)
	for i := 0; i < n; i++ {
		randBytes[i] = chars[seededRand.Int()%len(chars)]
	}
	return string(randBytes)
}

// used to ensure the fetch function was called
type MockFetchTriggered struct {
	msg    []byte
	called bool
}

func (f *MockFetchTriggered) Fetch() ([]byte, error) {
	f.called = true
	return f.msg, nil
}

func (f *MockFetchTriggered) Duration() time.Duration {
	return 1
}

type MockFetchErrored struct{}

func (f *MockFetchErrored) FetchRawBytes() ([]byte, error) {
	return nil, errors.New("mock fetch error")
}

func (f *MockFetchErrored) Duration() time.Duration {
	return 1
}

func TestCliFetcher(t *testing.T) {
	assert := assert.New(t)
	cliFetcher := NewCliScraper("ls")
	data, err := cliFetcher.FetchRawBytes()
	assert.Nil(err)
	assert.NotNil(data)
}

func TestCliFetcher_Timeout(t *testing.T) {
	assert := assert.New(t)
	cliFetcher := NewCliScraper("sleep", "100")
	cliFetcher.timeout = 0
	data, err := cliFetcher.FetchRawBytes()
	assert.EqualError(err, "signal: killed")
	assert.Nil(data)
}

func TestCliFetcher_EmptyArgs(t *testing.T) {
	assert := assert.New(t)
	cliFetcher := NewCliScraper()
	data, err := cliFetcher.FetchRawBytes()
	assert.EqualError(err, "need at least 1 args")
	assert.Nil(data)
}

func TestCliFetcher_ExitCodeCmd(t *testing.T) {
	assert := assert.New(t)
	cliFetcher := NewCliScraper("ls", generateRandString(64))
	data, err := cliFetcher.FetchRawBytes()
	assert.NotNil(err)
	assert.Nil(data)
}

func TestCliFetcher_StdErr(t *testing.T) {
	assert := assert.New(t)
	// the rare case where stderr is written but exit code is still 0
	cmd := `echo -e "error" 1>&2`
	cliFetcher := NewCliScraper("/bin/bash", "-c", cmd)
	data, err := cliFetcher.FetchRawBytes()
	assert.NotNil(err)
	assert.Nil(data)
}

func TestAtomicThrottledCache_CompMiss(t *testing.T) {
	assert := assert.New(t)
	cache := NewAtomicThrottledCache[NodeMetric](10)
	// empty cache scenario
	called := false
	host := "host1"
	info, err := cache.FetchOrThrottle(func() ([]NodeMetric, error) {
		called = true
		return []NodeMetric{{Hostname: host}}, nil
	})
	assert.Nil(err)
	assert.Equal(info[0].Hostname, host)
	// assert no cache hit
	assert.True(called)
	// assert cache populated
	assert.Positive(cache.cache[0].Hostname, host)
}

func TestAtomicThrottledCache_Hit(t *testing.T) {
	assert := assert.New(t)
	cache := NewAtomicThrottledCache[NodeMetric](math.MaxFloat64)
	cache.cache = []NodeMetric{{Hostname: "host1"}}
	// empty cache scenario
	called := false
	info, err := cache.FetchOrThrottle(func() ([]NodeMetric, error) {
		called = true
		return []NodeMetric{{Hostname: "host2"}}, nil
	})
	assert.Nil(err)
	assert.Equal(info[0].Hostname, "host1")
	// assert fetch not called
	assert.False(called)
	// assert cache populated
	assert.NotEmpty(cache.cache)
}

func TestAtomicThrottledCache_Stale(t *testing.T) {
	assert := assert.New(t)
	cache := NewAtomicThrottledCache[NodeMetric](0)
	cache.cache = []NodeMetric{{Hostname: "host1"}}
	called := false
	info, err := cache.FetchOrThrottle(func() ([]NodeMetric, error) {
		called = true
		return []NodeMetric{{Hostname: "host2"}}, nil
	})
	assert.Nil(err)
	assert.Equal(info[0].Hostname, "host2")
	// assert fetch not called
	assert.True(called)
	// assert cache populated
	assert.Equal(cache.cache[0].Hostname, "host2")
}

func TestConvertMemToFloat(t *testing.T) {
	assert := assert.New(t)
	e := 1.2e+7
	for _, s := range []string{"12M", "12G", "12T"} {
		n, err := MemToFloat(s)
		assert.Nil(err)
		assert.Equal(e, n)
		e *= 1e+3
	}
	n, err := MemToFloat("0")
	assert.Nil(err)
	assert.Zero(n)
}

func TestConvertMemToFloat_Sad(t *testing.T) {
	assert := assert.New(t)
	n, err := MemToFloat("afal")
	assert.Error(err)
	assert.Equal(-1., n)
}
