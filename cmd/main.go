package main

import (
	"flag"
	"fmt"
	"github.com/dazfuller/azcosts/internal/azure"
	"github.com/dazfuller/azcosts/internal/formats"
	"github.com/dazfuller/azcosts/internal/sqlite"
	"github.com/google/uuid"
	"os"
	"strings"
	"time"
)

func main() {
	var subscriptionId string
	var year int
	var month int
	var format string
	var useStdOut bool
	var outputPath string

	flag.StringVar(&subscriptionId, "subscription", "", "The id of the subscription to collect costs for")
	flag.IntVar(&year, "year", time.Now().Year(), "The year of the billing period")
	flag.IntVar(&month, "month", int(time.Now().Month()), "The month of the billing period")
	flag.StringVar(&format, "format", "text", "The output format to use. Allowed values are text, csv, json")
	flag.BoolVar(&useStdOut, "stdout", false, "If set writes the data to stdout")
	flag.StringVar(&outputPath, "path", "", "The output path to write the summary data to when not writing to stdout")

	flag.Usage = func() {
		fmt.Println("Azure costs summary")
		fmt.Println("Collects cost data from Microsoft Azure and summarises the output. The app makes use of")
		fmt.Println("the DefaultAzureCredential (https://learn.microsoft.com/dotnet/api/azure.identity.defaultazurecredential)")
		fmt.Println("type, and so running locally will use the Azure CLI tool for authentication if available.")
		fmt.Println("The user must have billing reader permissions on the subscription.")
		fmt.Println()
		fmt.Println("Usage:")
		flag.PrintDefaults()
	}

	flag.Parse()

	_, err := uuid.Parse(subscriptionId)
	if err != nil {
		fmt.Print("invalid subscription id, must be a valid guid\n\n")
		flag.Usage()
		os.Exit(1)
	}

	formatLower := strings.ToLower(format)
	if formatLower != "text" && formatLower != "csv" && formatLower != "json" {
		fmt.Print("a valid format must be specified\n\n")
		flag.Usage()
		os.Exit(1)
	}

	if !useStdOut && len(outputPath) == 0 {
		fmt.Print("when not writing to stdout an output path must be specified\n\n")
		flag.Usage()
		os.Exit(1)
	}

	svc := azure.NewCostService()
	costs, err := svc.ResourceGroupCostsForPeriod(subscriptionId, year, month)
	panicIfError(err)

	db, err := sqlite.NewCostManagementStore("./azure_costs.db")
	panicIfError(err)
	defer db.Close()

	err = db.SaveCosts(costs)
	panicIfError(err)

	summary, err := db.GenerateSummaryByResourceGroup()
	panicIfError(err)

	var formatter formats.Formatter

	switch formatLower {
	case "text":
		formatter, err = formats.MakeTextFormatter(useStdOut, outputPath)
		break
	case "csv":
		formatter, err = formats.MakeCsvFormatter(useStdOut, outputPath)
		break
	case "json":
		formatter, err = formats.MakeJsonFormatter(useStdOut, outputPath)
		break
	}
	panicIfError(err)

	err = formatter.Generate(summary)
	panicIfError(err)
}

func panicIfError(err error) {
	if err != nil {
		panic(err)
	}
}
