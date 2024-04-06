package sqlite

import (
	"fmt"
	"github.com/dazfuller/azcosts/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func TestNewCostManagementStore(t *testing.T) {
	tests := []struct {
		Name                 string
		DbPath               string
		Truncate             bool
		CreateBeforeTruncate bool
		Expected             error
		CleanDb              bool
	}{
		{
			Name:                 "Valid path no truncate",
			DbPath:               "./tt.db",
			Truncate:             false,
			CreateBeforeTruncate: false,
			Expected:             nil,
			CleanDb:              true,
		},
		{
			Name:                 "Valid path with truncate no existing file",
			DbPath:               "./tt.db",
			Truncate:             true,
			CreateBeforeTruncate: false,
			Expected:             nil,
			CleanDb:              true,
		},
		{
			Name:                 "Valid path with truncate with existing file",
			DbPath:               "./tt.db",
			Truncate:             true,
			CreateBeforeTruncate: true,
			Expected:             nil,
			CleanDb:              true,
		},
		{
			Name:                 "Invalid path no truncate",
			DbPath:               "./does/not/exist/tt.db",
			Truncate:             false,
			CreateBeforeTruncate: false,
			Expected:             fmt.Errorf("unable to open database file"),
			CleanDb:              false,
		},
		{
			Name:                 "Invalid path with truncate",
			DbPath:               "./does/not/exist/tt.db",
			Truncate:             true,
			CreateBeforeTruncate: false,
			Expected:             fmt.Errorf("unable to open database file"),
			CleanDb:              false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.CreateBeforeTruncate {
				_, err := os.Create(tt.DbPath)
				if err != nil {
					t.Fatalf("Failed to create database file: %v", err)
				}
			}

			_, err := NewCostManagementStore(tt.DbPath, tt.Truncate)

			if tt.Expected != nil {
				require.ErrorContains(t, err, tt.Expected.Error())
			} else {
				require.NoError(t, err)
			}

			if tt.CleanDb {
				os.Remove(tt.DbPath)
			}
		})
	}
}

func TestSaveCosts(t *testing.T) {
	// Prepare test cases
	testCases := []struct {
		Name           string
		DbPath         string
		CostsInput     []model.ResourceGroupCost
		ResourceGroups []model.ResourceGroup
		ExpectedStatus string
	}{
		// Add more test cases as needed
		{
			Name:   "Valid inputs matching resource group",
			DbPath: "./test.db",
			CostsInput: []model.ResourceGroupCost{
				{
					SubscriptionId:   "abc123",
					SubscriptionName: "test",
					Name:             "testrg",
					BillingPeriod:    time.Now().UTC(),
					Cost:             100,
					CostUSD:          110,
					Currency:         "GBP",
				},
			},
			ResourceGroups: []model.ResourceGroup{
				{
					Id:       "abc123",
					Name:     "testrg",
					Location: "centralus",
				},
			},
			ExpectedStatus: "active",
		},
		{
			Name:   "Valid inputs without matching resource group",
			DbPath: "./test.db",
			CostsInput: []model.ResourceGroupCost{
				{
					SubscriptionId:   "abc123",
					SubscriptionName: "test",
					Name:             "testrg",
					BillingPeriod:    time.Now().UTC(),
					Cost:             100,
					CostUSD:          110,
					Currency:         "GBP",
				},
			},
			ResourceGroups: []model.ResourceGroup{
				{
					Id:       "abc123",
					Name:     "not-a-match",
					Location: "centralus",
				},
			},
			ExpectedStatus: "inactive",
		},
		{
			Name:           "Nil costs input",
			DbPath:         "./test.db",
			CostsInput:     nil,
			ResourceGroups: []model.ResourceGroup{{}},
			ExpectedStatus: "inactive",
		},
		{
			Name:           "Nil resource groups",
			DbPath:         "./test.db",
			CostsInput:     []model.ResourceGroupCost{{}},
			ResourceGroups: nil,
			ExpectedStatus: "inactive",
		},
		{
			Name:           "Both inputs nil",
			DbPath:         "./test.db",
			CostsInput:     nil,
			ResourceGroups: nil,
			ExpectedStatus: "inactive",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			cm, err := NewCostManagementStore(tt.DbPath, false)
			if err != nil {
				assert.FailNowf(t, "Failed to create CostManagementStore", "%v", err)
			}

			err = cm.SaveCosts(tt.CostsInput, tt.ResourceGroups)
			assert.NoError(t, err, tt.Name)

			if tt.CostsInput != nil && len(tt.CostsInput) > 0 {
				savedRow := cm.db.QueryRow("SELECT resource_group_status FROM costs")
				var status string
				err = savedRow.Scan(&status)
				assert.NoError(t, err, tt.Name)
				assert.Equal(t, tt.ExpectedStatus, status)
			}

			// Cleanup after running each test
			err = cm.Close()
			assert.NoError(t, err, tt.Name)

			os.Remove(tt.DbPath)
		})
	}
}
