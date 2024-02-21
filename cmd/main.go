package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/dazfuller/azcosts/internal/azure"
	"github.com/dazfuller/azcosts/internal/formats"
	"github.com/dazfuller/azcosts/internal/sqlite"
	"github.com/google/uuid"
	"os"
	"path"
	"slices"
	"strings"
	"time"
)

var (
	subscriptionId string
	year           int
	month          int
	format         string
	useStdOut      bool
	outputPath     string
	truncateDB     bool
	overwrite      bool
)

func main() {
	flag.StringVar(&subscriptionId, "subscription", "", "The id of the subscription to collect costs for")
	flag.IntVar(&year, "year", time.Now().Year(), "The year of the billing period")
	flag.IntVar(&month, "month", int(time.Now().Month()), "The month of the billing period")
	flag.StringVar(&format, "format", "text", "The output format to use. Allowed values are text, csv, json")
	flag.BoolVar(&useStdOut, "stdout", false, "If set writes the data to stdout")
	flag.StringVar(&outputPath, "path", "", "The output path to write the summary data to when not writing to stdout")
	flag.BoolVar(&truncateDB, "truncate", false, "If specified will truncate the existing data in the database")
	flag.BoolVar(&overwrite, "overwrite", false, "If specified then any existing data for a billing period will be overwritten with new data")

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
		displayErrorMessage("invalid subscription id, must be a valid guid")
	}

	formatLower := strings.ToLower(format)
	if formatLower != "text" && formatLower != "csv" && formatLower != "json" {
		displayErrorMessage("a valid format must be specified")
	}

	if !useStdOut && len(outputPath) == 0 {
		displayErrorMessage("when not writing to stdout an output path must be specified")
	}

	if month > 12 {
		displayErrorMessage("invalid month, must be between 1 and 12")
	}

	billingDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	if billingDate.After(time.Now().UTC()) {
		displayErrorMessage("invalid billing period, must be in the past")
	}

	dbPath, err := getDatabasePath()
	panicIfError(err)

	db, err := sqlite.NewCostManagementStore(dbPath, truncateDB)
	panicIfError(err)
	defer db.Close()

	err = processSubscriptionBillingPeriods(db, billingDate)
	panicIfError(err)

	err = generateBillingSummary(db, formatLower)
	panicIfError(err)
}

func generateBillingSummary(db *sqlite.CostManagementStore, format string) error {
	summary, err := db.GenerateSummaryByResourceGroup()
	if err != nil {
		return err
	}

	var formatter formats.Formatter

	switch format {
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
	if err != nil {
		return err
	}

	err = formatter.Generate(summary)
	return err
}

func displayErrorMessage(msg string) {
	fmt.Printf("%s\n\n", msg)
	flag.Usage()
	os.Exit(1)
}

func processSubscriptionBillingPeriods(db *sqlite.CostManagementStore, fromDate time.Time) error {
	svc := azure.NewCostService()
	billingPeriods := calculateBillingPeriods(fromDate)
	existingPeriods, err := db.GetSubscriptionBillingPeriods(subscriptionId)
	if err != nil {
		return err
	}

	for _, period := range billingPeriods {
		if !overwrite && slices.Contains(existingPeriods, period) {
			continue
		}

		billingDate, err := time.Parse("2006-01", period)
		if err != nil {
			return err
		}

		costs, err := svc.ResourceGroupCostsForPeriod(subscriptionId, billingDate.Year(), int(billingDate.Month()))
		if err != nil {
			return err
		}

		err = db.DeleteSubscriptionBillingPeriod(subscriptionId, period)
		if err != nil {
			return err
		}

		err = db.SaveCosts(costs)
		if err != nil {
			return err
		}
	}

	return nil
}

func calculateBillingPeriods(billingDate time.Time) []string {
	var billingPeriods []string
	for billingDate.Before(time.Now().UTC()) {
		billingPeriods = append(billingPeriods, billingDate.Format("2006-01"))
		billingDate = billingDate.AddDate(0, 1, 0)
	}
	return billingPeriods
}

func getDatabasePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	dbDir := path.Join(homeDir, ".azure-costs")

	_, err = os.Stat(dbDir)
	if errors.Is(err, os.ErrNotExist) {
		err = os.Mkdir(dbDir, os.FileMode(0755))
		if err != nil {
			return "", err
		}
	}

	return path.Join(dbDir, "costs.db"), nil
}

func panicIfError(err error) {
	if err != nil {
		panic(err)
	}
}
