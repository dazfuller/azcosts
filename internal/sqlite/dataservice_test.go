package sqlite

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
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
			DbPath:               "./test.db",
			Truncate:             false,
			CreateBeforeTruncate: false,
			Expected:             nil,
			CleanDb:              true,
		},
		{
			Name:                 "Valid path with truncate no existing file",
			DbPath:               "./test.db",
			Truncate:             true,
			CreateBeforeTruncate: false,
			Expected:             nil,
			CleanDb:              true,
		},
		{
			Name:                 "Valid path with truncate with existing file",
			DbPath:               "./test.db",
			Truncate:             true,
			CreateBeforeTruncate: true,
			Expected:             nil,
			CleanDb:              true,
		},
		{
			Name:                 "Invalid path no truncate",
			DbPath:               "./does/not/exist/test.db",
			Truncate:             false,
			CreateBeforeTruncate: false,
			Expected:             fmt.Errorf("unable to open database file"),
			CleanDb:              false,
		},
		{
			Name:                 "Invalid path with truncate",
			DbPath:               "./does/not/exist/test.db",
			Truncate:             true,
			CreateBeforeTruncate: false,
			Expected:             fmt.Errorf("unable to open database file"),
			CleanDb:              false,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			if test.CreateBeforeTruncate {
				_, err := os.Create(test.DbPath)
				if err != nil {
					t.Fatalf("Failed to create database file: %v", err)
				}
			}

			_, err := NewCostManagementStore(test.DbPath, test.Truncate)

			if test.Expected != nil {
				require.ErrorContains(t, err, test.Expected.Error())
			} else {
				require.NoError(t, err)
			}

			if test.CleanDb {
				os.Remove(test.DbPath)
			}
		})
	}
}
