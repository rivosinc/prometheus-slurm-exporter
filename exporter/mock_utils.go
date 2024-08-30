// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"bytes"
	"errors"
	"os"
	"time"
)

type MockFetchErrored struct{}

func (f *MockFetchErrored) FetchRawBytes() ([]byte, error) {
	return nil, errors.New("mock fetch error")
}

func (f *MockFetchErrored) Duration() time.Duration {
	return 1
}

// implements SlurmByteScraper by pulling fixtures instead
// used exclusively for testing
type MockScraper struct {
	fixture   string
	duration  time.Duration
	CallCount int
}

func (f *MockScraper) FetchRawBytes() ([]byte, error) {
	defer func(t time.Time) {
		f.duration = time.Since(t)
	}(time.Now())
	f.CallCount++
	file, err := os.ReadFile(f.fixture)
	if err != nil {
		return nil, err
	}
	// allow commenting in text files
	sep := []byte("\n")
	lines := bytes.Split(file, sep)
	filtered := make([][]byte, 0)
	for _, line := range lines {
		if !bytes.HasPrefix(line, []byte("#")) {
			filtered = append(filtered, line)
		}
	}
	return bytes.Join(filtered, sep), nil
}

func (f *MockScraper) Duration() time.Duration {
	return f.duration
}

// implements SlurmByteScraper by emmiting string payload instead
// used exclusively for testing
type StringByteScraper struct {
	msg       string
	Callcount int
}

func (es *StringByteScraper) FetchRawBytes() ([]byte, error) {
	es.Callcount++
	return []byte(es.msg), nil
}

func (es *StringByteScraper) Duration() time.Duration {
	return time.Duration(1)
}
