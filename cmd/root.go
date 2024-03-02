package cmd

import (
	"errors"
	"flag"
	"fmt"
	"github.com/dazfuller/azcosts/internal/azure"
	"github.com/dazfuller/azcosts/internal/formats"
	"github.com/dazfuller/azcosts/internal/sqlite"
	"github.com/google/uuid"
	"log"
	"os"
	"path"
	"slices"
	"strings"
	"time"
)

const (
	TextFormat  = "text"
	CsvFormat   = "csv"
	JsonFormat  = "json"
	ExcelFormat = "excel"
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

func Execute() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("An error occurred running the application", r)
		}
	}()

	collectCmd := flag.NewFlagSet("collect", flag.ExitOnError)
	generateCmd := flag.NewFlagSet("generate", flag.ExitOnError)

	collectCmd.StringVar(&subscriptionId, "subscription", "", "The id of the subscription to collect costs for")
	collectCmd.IntVar(&year, "year", time.Now().Year(), "The year of the billing period")
	collectCmd.IntVar(&month, "month", int(time.Now().Month()), "The month of the billing period")
	collectCmd.BoolVar(&truncateDB, "truncate", false, "If specified will truncate the existing data in the database")
	collectCmd.BoolVar(&overwrite, "overwrite", false, "If specified then any existing data for a billing period will be overwritten with new data")

	collectCmd.Usage = func() {
		fmt.Println("Azure costs summary")
		fmt.Println("Collects cost data from Microsoft Azure. The app makes use of")
		fmt.Println("the DefaultAzureCredential (https://learn.microsoft.com/dotnet/api/azure.identity.defaultazurecredential)")
		fmt.Println("type, and so running locally will use the Azure CLI tool for authentication if available.")
		fmt.Println("The user must have billing reader permissions on the subscription.")
		fmt.Println()
		fmt.Println("Usage:")
		collectCmd.PrintDefaults()
	}

	generateCmd.StringVar(&format, "format", "text", fmt.Sprintf(
		"The output format to use. Allowed values are '%s', '%s', '%s', and '%s'", TextFormat, CsvFormat, JsonFormat, ExcelFormat))
	generateCmd.BoolVar(&useStdOut, "stdout", false, "If set writes the data to stdout")
	generateCmd.StringVar(&outputPath, "path", "", "The output path to write the summary data to when not writing to stdout")

	generateCmd.Usage = func() {
		fmt.Println("Azure costs summary")
		fmt.Println("Generates a summarized output of the collected billing data.")
		fmt.Println()
		fmt.Println("Usage:")
		generateCmd.PrintDefaults()
	}

	if len(os.Args) < 2 || strings.Contains(strings.ToLower(os.Args[1]), "help") {
		displayTopLevelUsage()
		os.Exit(1)
	}

	var err error

	switch strings.ToLower(os.Args[1]) {
	case "collect":
		err := collectCmd.Parse(os.Args[2:])
		if err != nil {
			displayErrorMessage("", collectCmd)
		}
		validateCollectFlags(collectCmd)
		err = collectBillingData()
		break
	case "generate":
		err := generateCmd.Parse(os.Args[2:])
		if err != nil {
			displayErrorMessage("", generateCmd)
		}
		validateGenerateFlags(generateCmd)
		err = generateBillingSummary()
		break
	default:
		fmt.Println("Unexpected command, expected 'collect' or 'generate'")
		fmt.Println()
		displayTopLevelUsage()
		os.Exit(1)
	}

	if err != nil {
		panic(err)
	}
}

func validateCollectFlags(flags *flag.FlagSet) {
	_, err := uuid.Parse(subscriptionId)
	if err != nil {
		displayErrorMessage("invalid subscription id, must be a valid guid", flags)
	}

	if month > 12 {
		displayErrorMessage("invalid month, must be between 1 and 12", flags)
	}

	billingDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	if billingDate.After(time.Now().UTC()) {
		displayErrorMessage("invalid billing period, must be in the past", flags)
	}
}

func validateGenerateFlags(flags *flag.FlagSet) {
	allowedFormats := []string{
		TextFormat,
		CsvFormat,
		JsonFormat,
		ExcelFormat,
	}

	formatLower := strings.ToLower(format)
	if !slices.Contains(allowedFormats, formatLower) {
		displayErrorMessage("a valid format must be specified", flags)
	}

	if !useStdOut && len(outputPath) == 0 {
		displayErrorMessage("when not writing to stdout an output path must be specified", flags)
	} else if formatLower == "excel" && len(outputPath) == 0 {
		displayErrorMessage("excel output cannot be written to stdout and so an output path must be specified", flags)
	}
}

func collectBillingData() error {
	dbPath, err := getDatabasePath()
	if err != nil {
		return err
	}

	db, err := sqlite.NewCostManagementStore(dbPath, truncateDB)
	if err != nil {
		return err
	}
	defer func(db *sqlite.CostManagementStore) {
		err := db.Close()
		if err != nil {
			log.Printf("Unable to close data store")
		}
	}(db)

	billingDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)

	err = processSubscriptionBillingPeriods(db, billingDate)
	return err
}

func generateBillingSummary() error {
	dbPath, err := getDatabasePath()
	if err != nil {
		return err
	}

	db, err := sqlite.NewCostManagementStore(dbPath, truncateDB)
	if err != nil {
		return err
	}
	defer func(db *sqlite.CostManagementStore) {
		err := db.Close()
		if err != nil {
			log.Printf("Unable to close data store")
		}
	}(db)

	summary, err := db.GenerateSummaryByResourceGroup()
	if err != nil {
		return err
	}

	var formatter formats.Formatter

	switch strings.ToLower(format) {
	case TextFormat:
		formatter, err = formats.MakeTextFormatter(useStdOut, outputPath)
		break
	case CsvFormat:
		formatter, err = formats.MakeCsvFormatter(useStdOut, outputPath)
		break
	case JsonFormat:
		formatter, err = formats.MakeJsonFormatter(useStdOut, outputPath)
		break
	case ExcelFormat:
		formatter, err = formats.MakeExcelFormatter(outputPath)
		break
	}
	if err != nil {
		return err
	}

	err = formatter.Generate(summary)
	return err
}

func displayTopLevelUsage() {
	fmt.Println("Azure costs summary")
	fmt.Println("A tool for collecting billing data from Azure, and producing summarized outputs")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  collect")
	fmt.Println("        Collects data from Azure and persists into a local store")
	fmt.Println("  generate")
	fmt.Println("        Produces a summarized output of the billing data in multiple formats")
	fmt.Println()
}

func displayErrorMessage(msg string, flags *flag.FlagSet) {
	if len(msg) > 0 {
		fmt.Printf("%s\n\n", msg)
	}
	flags.Usage()
	os.Exit(1)
}

func processSubscriptionBillingPeriods(db *sqlite.CostManagementStore, billingDate time.Time) error {
	svc := azure.NewCostService()
	existingPeriods, err := db.GetSubscriptionBillingPeriods(subscriptionId)
	if err != nil {
		return err
	}

	period := billingDate.Format("2006-01")

	if !overwrite && slices.Contains(existingPeriods, period) {
		log.Println("Data for the selected billing period already exists, use the overwrite option to replace this data")
		return nil
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

	log.Printf("Successfully collected and saved billing data for subscription %s for %s", subscriptionId, period)

	return nil
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
