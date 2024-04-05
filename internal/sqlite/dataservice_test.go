package sqlite

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestNewCostManagementStore(t *testing.T) {
	test := assert.New(t)
	file, err := os.CreateTemp("", "tempdb")
	if err != nil {
		test.FailNowf("could not create temp database", "%v", err)
	}
	defer os.Remove(file.Name())

	store, err := NewCostManagementStore(file.Name(), false)
	if err != nil {
		test.FailNowf("could not create temp database", "%v", err)
	}
	defer store.Close()

	test.Equal(store.dbPath, file.Name())
}
