package exporter

import (
	"testing"
)

func TestParseFairShare(t *testing.T) {
	fetcher := &FairShareFetcher{}

	output := `parentaccount1|
 account1|1.000000
 account2|15.5
  account2.sub1|inf
  account2.sub2|inf
 account3|0.5
 account4|inf
 account5|2.25
`

	metrics, err := fetcher.parseFairShare(output)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should only get accounts with numeric fairshare values (not empty or inf)
	expectedAccounts := map[string]float64{
		"account1": 1.000000,
		"account2": 15.5,
		"account3": 0.5,
		"account5": 2.25,
	}

	if len(metrics) != len(expectedAccounts) {
		t.Errorf("Expected %d accounts, got %d", len(expectedAccounts), len(metrics))
		for _, m := range metrics {
			t.Logf("Got account: %s = %f", m.Account, m.FairShare)
		}
	}

	for _, metric := range metrics {
		expectedValue, exists := expectedAccounts[metric.Account]
		if !exists {
			t.Errorf("Unexpected account: %s", metric.Account)
		}
		if metric.FairShare != expectedValue {
			t.Errorf("Account %s: expected fairshare %f, got %f",
				metric.Account, expectedValue, metric.FairShare)
		}
	}
}

func TestParseFairShareSubAccountsSkipped(t *testing.T) {
	fetcher := &FairShareFetcher{}

	// Test that accounts with inf are skipped
	output := "  user1|inf"
	metrics, err := fetcher.parseFairShare(output)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(metrics) != 0 {
		t.Errorf("Accounts with inf should be skipped, but got %d metrics", len(metrics))
	}
}

func TestParseFairShareSingleAccount(t *testing.T) {
	fetcher := &FairShareFetcher{}

	output := "testaccount|0.999999"
	metrics, err := fetcher.parseFairShare(output)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(metrics) != 1 {
		t.Fatalf("Expected 1 metric, got %d", len(metrics))
	}

	if metrics[0].Account != "testaccount" {
		t.Errorf("Expected account 'testaccount', got '%s'", metrics[0].Account)
	}
	if metrics[0].FairShare != 0.999999 {
		t.Errorf("Expected fairshare 0.999999, got %f", metrics[0].FairShare)
	}
}

func TestParseFairShareInvalidLines(t *testing.T) {
	fetcher := &FairShareFetcher{}

	// Test with invalid data, empty values and inf
	output := `account1|0.500000
invalid_line_without_pipe
account2|invalid_number
account3|
account4|inf
account5|0.750000
`

	metrics, err := fetcher.parseFairShare(output)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should get 2 valid accounts (account1 and account5)
	// account2 has invalid number, account3 is empty, account4 is inf
	if len(metrics) != 2 {
		t.Errorf("Expected 2 valid metrics, got %d", len(metrics))
		for _, m := range metrics {
			t.Logf("Got account: %s = %f", m.Account, m.FairShare)
		}
	}
}
