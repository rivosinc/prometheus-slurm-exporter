// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/exp/slog"
)

type SlurmPrimitiveMetric interface {
	NodeMetric | JobMetric | DiagMetric | LicenseMetric | AccountLimitMetric
}

// interface for getting data from slurm
// used for dep injection/ease of testing & for add slurmrestd support later
type SlurmByteScraper interface {
	FetchRawBytes() ([]byte, error)
	Duration() time.Duration
}

type SlurmMetricFetcher[M SlurmPrimitiveMetric] interface {
	FetchMetrics() ([]M, error)
	ScrapeDuration() time.Duration
	ScrapeError() prometheus.Counter
}

type AtomicThrottledCache[C SlurmPrimitiveMetric] struct {
	sync.Mutex
	t     time.Time
	limit float64
	cache []C
	// duration of last cache miss
	duration time.Duration
}

// atomic fetch of either the cache or the collector
// reset & hydrate as necessary
func (atc *AtomicThrottledCache[C]) FetchOrThrottle(fetchFunc func() ([]C, error)) ([]C, error) {
	atc.Lock()
	defer atc.Unlock()
	if len(atc.cache) > 0 && time.Since(atc.t).Seconds() < atc.limit {
		return atc.cache, nil
	}
	t := time.Now()
	slurmData, err := fetchFunc()
	if err != nil {
		return nil, err
	}
	atc.duration = time.Since(t)
	atc.cache = slurmData
	atc.t = time.Now()
	return slurmData, nil
}

func NewAtomicThrottledCache[C SlurmPrimitiveMetric](limit float64) *AtomicThrottledCache[C] {
	return &AtomicThrottledCache[C]{
		t:     time.Now(),
		limit: limit,
	}
}

func track(cmd []string) (string, time.Time) {
	return strings.Join(cmd, " "), time.Now()
}

func duration(msg string, start time.Time) {
	slog.Debug(fmt.Sprintf("cmd %s took %s secs", msg, time.Since(start)))
}

// implements SlurmByteScraper by fetch data from cli
type CliScraper struct {
	args     []string
	timeout  time.Duration
	duration time.Duration
}

func (cf *CliScraper) Duration() time.Duration {
	return cf.duration
}

func (cf *CliScraper) FetchRawBytes() ([]byte, error) {
	defer func(t time.Time) { cf.duration = time.Since(t) }(time.Now())
	if len(cf.args) == 0 {
		return nil, errors.New("need at least 1 args")
	}
	defer duration(track(cf.args))
	cmd := exec.Command(cf.args[0], cf.args[1:]...)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	timer := time.AfterFunc(cf.timeout, func() {
		if err := cmd.Process.Kill(); err != nil {
			slog.Error(fmt.Sprintf("failed to cancel cmd: %v", cf.args))
		}
	})
	defer timer.Stop()
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	if errb.Len() > 0 {
		return nil, fmt.Errorf("cmd failed with %s", errb.String())
	}
	return outb.Bytes(), nil
}

func NewCliScraper(args ...string) *CliScraper {
	var limit float64 = 10
	var err error
	if tm, ok := os.LookupEnv("CLI_TIMEOUT"); ok {
		if limit, err = strconv.ParseFloat(tm, 64); err != nil {
			slog.Error("`CLI_TIMEOUT` env var parse error")
		}
	}
	return &CliScraper{
		args:    args,
		timeout: time.Duration(limit) * time.Second,
	}
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

// convert slurm mem string to float64 bytes
func MemToFloat(mem string) (float64, error) {
	if num, err := strconv.ParseFloat(mem, 64); err == nil {
		return num, nil
	}
	memUnits := map[string]float64{
		"M": 1e+6,
		"G": 1e+9,
		"T": 1e+12,
	}
	re := regexp.MustCompile(`^(?P<num>([0-9]*[.])?[0-9]+)(?P<memunit>G|M|T)$`)
	matches := re.FindStringSubmatch(mem)
	if len(matches) < 2 {
		return -1, fmt.Errorf("mem string %s doesn't match regex %s nor is a float", mem, re)
	}
	// err here should be impossible due to regex
	num, err := strconv.ParseFloat(matches[re.SubexpIndex("num")], 64)
	memunit := memUnits[matches[re.SubexpIndex("memunit")]]
	return num * memunit, err
}
