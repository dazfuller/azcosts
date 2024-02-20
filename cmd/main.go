package main

import (
	"github.com/dazfuller/azcosts/internal/azure"
	"github.com/dazfuller/azcosts/internal/formats"
	"github.com/dazfuller/azcosts/internal/sqlite"
)

func main() {
	svc := azure.NewCostService()
	costs, err := svc.ResourceGroupCostsForPeriod("0798fe24-1af4-4cb1-8a32-8529f10153f2", 2024, 1)
	panicIfError(err)

	db, err := sqlite.NewCostManagementStore("./test.db")
	panicIfError(err)
	defer db.Close()

	err = db.SaveCosts(costs)
	panicIfError(err)

	summary, err := db.GenerateSummaryByResourceGroup()
	panicIfError(err)

	formatter := formats.MakeConsoleFormatter()
	err = formatter.Generate(summary)
	panicIfError(err)
}

func panicIfError(err error) {
	if err != nil {
		panic(err)
	}
}
