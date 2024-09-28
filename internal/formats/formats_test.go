package formats

import (
	"github.com/dazfuller/azcosts/internal/model"
	"slices"
	"strings"
	"testing"
)

func TestValidateOptions(t *testing.T) {
	err := validateOptions(true, "")

	if err != nil {
		t.Errorf("expected nil error but got %v", err)
	}
}

func TestValidateOptionsForEmptyPath(t *testing.T) {
	err := validateOptions(false, "")

	if err == nil {
		t.Fatal("expected an error but got none")
	}

	if !strings.Contains(err.Error(), "file path must be specified") {
		t.Fatalf("expected an error stating the file path must be specified but received: %s", err.Error())
	}
}

func TestGenerateSubscriptionSummary(t *testing.T) {
	testData := []model.ResourceGroupSummary{
		{
			Name:             "rg1",
			SubscriptionName: "sub1",
			Active:           true,
			Costs: []model.BillingPeriodCost{
				{
					Period: "2024-01",
					Total:  10,
				},
				{
					Period: "2024-03",
					Total:  20,
				},
			},
			TotalCost: 30,
		},
		{
			Name:             "rg2",
			SubscriptionName: "sub1",
			Active:           true,
			Costs: []model.BillingPeriodCost{
				{
					Period: "2024-01",
					Total:  40,
				},
				{
					Period: "2024-02",
					Total:  10,
				},
			},
			TotalCost: 50,
		},
		{
			Name:             "rg3",
			SubscriptionName: "sub2",
			Active:           true,
			Costs: []model.BillingPeriodCost{
				{
					Period: "2024-03",
					Total:  10,
				},
			},
			TotalCost: 10,
		},
	}

	summary := generateSubscriptionSummary(testData)

	// Check we have the correct number of results back
	if len(summary) != 2 {
		t.Errorf("expected 2 records but got %d", len(summary))
	}

	// Find the entries for sub1 and sub2
	sub1Index := slices.IndexFunc(summary, func(s model.SubscriptionSummary) bool {
		return s.Name == "sub1"
	})

	sub2Index := slices.IndexFunc(summary, func(s model.SubscriptionSummary) bool {
		return s.Name == "sub2"
	})

	if sub1Index == -1 {
		t.Fatal("Expected to find entry for sub1 but none found")
	}

	if sub2Index == -1 {
		t.Fatal("Expected to find entry for sub2 but none found")
	}

	// Make sure that the subscription costs add up to the expected value
	if summary[sub1Index].TotalCost != 80 {
		t.Errorf("Expected sub1 total cost to be 80 but got %f", summary[sub1Index].TotalCost)
	}

	if summary[sub2Index].TotalCost != 10 {
		t.Errorf("Expected sub2 total cost to be 10 but got %f", summary[sub2Index].TotalCost)
	}

	// Make sure that there is a cost entry for each period for the subscription
	sub1ExpectedPeriods := []string{"2024-01", "2024-02", "2024-03"}
	for _, period := range sub1ExpectedPeriods {
		if ok := slices.ContainsFunc(summary[sub1Index].Costs, func(bpc model.BillingPeriodCost) bool {
			return bpc.Period == period
		}); !ok {
			t.Errorf("sub1 did not contain the expected billing period %s", period)
		}
	}

	sub2ExpectedPeriods := []string{"2024-03"}
	for _, period := range sub2ExpectedPeriods {
		if ok := slices.ContainsFunc(summary[sub2Index].Costs, func(bpc model.BillingPeriodCost) bool {
			return bpc.Period == period
		}); !ok {
			t.Errorf("sub2 did not contain the expected billing period %s", period)
		}
	}
}
