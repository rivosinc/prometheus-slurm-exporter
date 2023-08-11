package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/slog"
)

// interface for getting data from slurm
// used for dep injection/ease of testing & for add slurmrestd support later
type SlurmFetcher interface {
	Fetch() ([]byte, error)
}

type AtomicThrottledCache struct {
	sync.Mutex
	t     time.Time
	limit float64
	cache []byte
}

// atomic fetch of either the cache or the collector
// reset & hydrate as neccesary
func (atc *AtomicThrottledCache) fetchOrThrottle(fetchFunc func() ([]byte, error)) ([]byte, error) {
	atc.Lock()
	defer atc.Unlock()
	if len(atc.cache) > 0 && time.Since(atc.t).Seconds() < atc.limit {
		return atc.cache, nil
	}
	slurmData, err := fetchFunc()
	if err != nil {
		return nil, err
	}
	atc.cache = slurmData
	atc.t = time.Now()
	return slurmData, nil
}

func NewAtomicThrottledCache(limit float64) *AtomicThrottledCache {
	return &AtomicThrottledCache{
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

// implements SlurmFetcher by fetch data from cli
type CliFetcher struct {
	args    []string
	timeout time.Duration
	cache   *AtomicThrottledCache
}

func (cf *CliFetcher) Fetch() ([]byte, error) {
	return cf.cache.fetchOrThrottle(cf.captureCli)
}

func (cf *CliFetcher) captureCli() ([]byte, error) {
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
		return nil, errors.New(errb.String())
	}
	return outb.Bytes(), nil
}

func NewCliFetcher(args ...string) *CliFetcher {
	return &CliFetcher{
		args:    args,
		timeout: 10 * time.Second,
		cache:   NewAtomicThrottledCache(1),
	}
}

// implements SlurmFetcher by pulling fixtures instead
// used exclusively for testing
type MockFetcher struct {
	fixture string
}

func (f *MockFetcher) Fetch() ([]byte, error) {
	return os.ReadFile(f.fixture)
}
