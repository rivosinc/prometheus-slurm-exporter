package exporter

import (
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestParseCredits(t *testing.T) {
	data, err := os.ReadFile("fixtures/scredits_out.txt")
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	credits, err := parseCredits(data)
	if err != nil {
		t.Fatalf("Failed to parse credits: %v", err)
	}

	if len(credits) == 0 {
		t.Fatal("Expected credits data, got none")
	}

	// Check first account
	if credits[0].Account != "cinfo.prova" {
		t.Errorf("Expected account 'cinfo.prova', got '%s'", credits[0].Account)
	}
	if credits[0].Allocation != 10.0 {
		t.Errorf("Expected allocation 10.0, got %f", credits[0].Allocation)
	}
	if credits[0].Remaining != 10.0 {
		t.Errorf("Expected remaining 10.0, got %f", credits[0].Remaining)
	}
	if credits[0].Used != 0.0 {
		t.Errorf("Expected used 0.0, got %f", credits[0].Used)
	}
	if credits[0].UsedPct != 0.0 {
		t.Errorf("Expected used percent 0.0, got %f", credits[0].UsedPct)
	}

	// Check sgtest account (highest usage)
	var sgtestFound bool
	for _, c := range credits {
		if c.Account == "sgtest" {
			sgtestFound = true
			if c.Allocation != 4000000.0 {
				t.Errorf("Expected sgtest allocation 4000000.0, got %f", c.Allocation)
			}
			if c.Remaining != 2755812.0 {
				t.Errorf("Expected sgtest remaining 2755812.0, got %f", c.Remaining)
			}
			if c.Used != 1244188.0 {
				t.Errorf("Expected sgtest used 1244188.0, got %f", c.Used)
			}
			if c.UsedPct != 31.1 {
				t.Errorf("Expected sgtest used percent 31.1, got %f", c.UsedPct)
			}
			break
		}
	}
	if !sgtestFound {
		t.Error("Expected to find sgtest account")
	}
}

// MockCreditsFetcher for testing
type MockCreditsFetcher struct {
	credits []AccountCredits
	err     error
}

func (m *MockCreditsFetcher) FetchMetrics() ([]AccountCredits, error) {
	return m.credits, m.err
}

func (m *MockCreditsFetcher) ScrapeDuration() time.Duration {
	return 10 * time.Millisecond
}

func (m *MockCreditsFetcher) ScrapeError() prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_error",
		Help: "test error counter",
	})
}

func TestCreditsCollector(t *testing.T) {
	data, err := os.ReadFile("fixtures/scredits_out.txt")
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	credits, err := parseCredits(data)
	if err != nil {
		t.Fatalf("Failed to parse credits: %v", err)
	}

	mockFetcher := &MockCreditsFetcher{credits: credits}
	collector := &CreditsCollector{
		fetcher: mockFetcher,
		accountAllocation: prometheus.NewDesc(
			"slurm_scredits_allocation_total",
			"Total allocation in SU per account",
			[]string{"account"},
			nil,
		),
		accountRemaining: prometheus.NewDesc(
			"slurm_scredits_remaining",
			"Remaining credits in SU per account",
			[]string{"account"},
			nil,
		),
		accountUsed: prometheus.NewDesc(
			"slurm_scredits_used",
			"Used credits in SU per account",
			[]string{"account"},
			nil,
		),
		accountUsedPercent: prometheus.NewDesc(
			"slurm_scredits_used_percent",
			"Percentage of credits used per account",
			[]string{"account"},
			nil,
		),
		scrapeDuration: prometheus.NewDesc(
			"slurm_scredits_scrape_duration",
			"Time taken to scrape credits data (ms)",
			nil,
			nil,
		),
		scrapeError: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "slurm_credits_scrape_error",
			Help: "Slurm credits scrape errors",
		}),
	}

	// Register and collect metrics
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// Collect metrics
	metricCount := testutil.CollectAndCount(collector)
	if metricCount == 0 {
		t.Fatal("Expected metrics to be collected")
	}

	t.Logf("Collected %d metrics", metricCount)

	// Verify specific metrics exist
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	var foundAllocation, foundRemaining, foundUsed, foundPercent bool
	for _, mf := range metricFamilies {
		switch *mf.Name {
		case "slurm_scredits_allocation_total":
			foundAllocation = true
		case "slurm_scredits_remaining":
			foundRemaining = true
		case "slurm_scredits_used":
			foundUsed = true
		case "slurm_scredits_used_percent":
			foundPercent = true
		}
	}

	if !foundAllocation {
		t.Error("Expected slurm_scredits_allocation_total metric")
	}
	if !foundRemaining {
		t.Error("Expected slurm_scredits_remaining metric")
	}
	if !foundUsed {
		t.Error("Expected slurm_scredits_used metric")
	}
	if !foundPercent {
		t.Error("Expected slurm_scredits_used_percent metric")
	}
}
