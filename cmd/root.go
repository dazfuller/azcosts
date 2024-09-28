package cmd

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"github.com/dazfuller/azcosts/internal/azure"
	"github.com/dazfuller/azcosts/internal/formats"
	"github.com/dazfuller/azcosts/internal/model"
	"github.com/dazfuller/azcosts/internal/sqlite"
	"github.com/google/uuid"
	"log"
	"os"
	"path"
	"slices"
	"sort"
	"strconv"
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
	subscriptionId   string
	subscriptionName string
	year             int
	month            int
	format           string
	useStdOut        bool
	outputPath       string
	truncateDB       bool
	generateMonths   int
)

func Execute() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("An error occurred running the application", r)
		}
	}()

	subscriptionCmd := flag.NewFlagSet("subscription", flag.ExitOnError)
	collectCmd := flag.NewFlagSet("collect", flag.ExitOnError)
	generateCmd := flag.NewFlagSet("generate", flag.ExitOnError)
	statusCmd := flag.NewFlagSet("status", flag.ExitOnError)

	subscriptionCmd.StringVar(&subscriptionName, "name", "", "Full or partial name to filter by, if not provided then a full list is returned")

	subscriptionCmd.Usage = func() {
		fmt.Println("Azure costs summary")
		fmt.Println("Outputs a list of subscriptions available to the current account")
		fmt.Println()
		fmt.Println("Usage:")
		subscriptionCmd.PrintDefaults()
	}

	collectCmd.StringVar(&subscriptionId, "subscription", "", "The id of the subscription to collect costs for")
	collectCmd.StringVar(&subscriptionName, "name", "", "Full or partial name of the subscription if the id is not known")
	collectCmd.IntVar(&year, "year", time.Now().Year(), "The year of the billing period")
	collectCmd.IntVar(&month, "month", int(time.Now().Month()), "The month of the billing period")
	collectCmd.BoolVar(&truncateDB, "truncate", false, "If specified will truncate the existing data in the database")

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
	generateCmd.IntVar(&generateMonths, "months", 6, "The number of months over which to report")

	generateCmd.Usage = func() {
		fmt.Println("Azure costs summary")
		fmt.Println("Generates a summarized output of the collected billing data.")
		fmt.Println()
		fmt.Println("Usage:")
		generateCmd.PrintDefaults()
	}

	statusCmd.Usage = func() {
		fmt.Println("Azure costs summary")
		fmt.Println("Outputs information showing the collection status of subscriptions collected to date")
	}

	if len(os.Args) < 2 || slices.Contains([]string{"-h", "-help"}, strings.ToLower(os.Args[1])) {
		displayTopLevelUsage()
		os.Exit(0)
	}

	var err error

	switch strings.ToLower(os.Args[1]) {
	case "subscription":
		err = subscriptionCmd.Parse(os.Args[2:])
		if err != nil {
			displayErrorMessage("", subscriptionCmd)
		}
		err = displaySubscriptions()
		break
	case "collect":
		err = collectCmd.Parse(os.Args[2:])
		if err != nil {
			displayErrorMessage("", collectCmd)
		}
		validateCollectFlags(collectCmd)
		err = collectBillingData()
		break
	case "generate":
		err = generateCmd.Parse(os.Args[2:])
		if err != nil {
			displayErrorMessage("", generateCmd)
		}
		validateGenerateFlags(generateCmd)
		err = generateBillingSummary()
		break
	case "status":
		err = statusCmd.Parse(os.Args[2:])
		if err != nil {
			displayErrorMessage("", statusCmd)
		}
		err = displayCollectionStatus()
		break
	default:
		fmt.Println("Unexpected command, expected 'subscription', 'collect', 'generate', or 'status'")
		fmt.Println()
		displayTopLevelUsage()
		os.Exit(1)
	}

	if err != nil {
		panic(err)
	}
}

func validateCollectFlags(flags *flag.FlagSet) {
	if len(subscriptionId) == 0 && len(subscriptionName) == 0 {
		displayErrorMessage("either a subscription id or name must be provided", flags)
	}

	if len(subscriptionId) > 0 {
		_, err := uuid.Parse(subscriptionId)
		if err != nil {
			displayErrorMessage("invalid subscription id, must be a valid guid", flags)
		}
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
	} else if formatLower == ExcelFormat && len(outputPath) == 0 {
		displayErrorMessage("excel output cannot be written to stdout and so an output path must be specified", flags)
	}

	if generateMonths <= 0 {
		displayErrorMessage("number of months must be greater than 0", flags)
	}
}

func displaySubscriptions() error {
	svc := azure.NewSubscriptionService()

	db, err := getCostManagementStore()
	if err != nil {
		return err
	}
	defer func(db *sqlite.CostManagementStore) {
		err := db.Close()
		if err != nil {
			log.Printf("Unable to close data store: %e", err)
		}
	}(db)

	collectedSubs, err := db.ListCollectedSubscriptions()

	var subscriptions []model.Subscription

	if len(subscriptionName) > 0 {
		subscriptions, err = svc.FindSubscription(subscriptionName)
		if err != nil {
			return err
		}
	} else {
		subscriptions, err = svc.GetSubscriptions()
		if err != nil {
			return err
		}
	}

	sort.Slice(subscriptions, func(a, b int) bool {
		return subscriptions[a].Name < subscriptions[b].Name
	})

	fmt.Printf("%-51s%-38s%-11s\n", "Subscription", "Subscription Id", "Collected")
	fmt.Printf("%-51s%-38s%-11s\n", strings.Repeat("=", 50), strings.Repeat("=", 37), strings.Repeat("=", 10))
	for _, sub := range subscriptions {
		name := sub.Name
		if len(name) > 50 {
			name = name[:50]
		}

		collected := "No"

		if slices.ContainsFunc(collectedSubs, func(s model.Subscription) bool {
			return s.Id == sub.Id
		}) {
			collected = "Yes"
		}

		fmt.Printf("%-50s %-37s %-10s\n", name, sub.Id, collected)
	}

	return nil
}

func collectBillingData() error {
	db, err := getCostManagementStore()
	if err != nil {
		return err
	}
	defer func(db *sqlite.CostManagementStore) {
		err := db.Close()
		if err != nil {
			log.Printf("Unable to close data store: %e", err)
		}
	}(db)

	if len(subscriptionId) == 0 {
		subscriptionId, err = getSubscriptionId()
		if err != nil {
			return err
		}
	}

	billingDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)

	err = processSubscriptionBillingPeriods(db, billingDate)
	return err
}

func getSubscriptionId() (string, error) {
	svc := azure.NewSubscriptionService()
	subscriptions, err := svc.FindSubscription(subscriptionName)
	if err != nil {
		return "", err
	}

	if len(subscriptions) == 0 {
		return "", fmt.Errorf("no subscriptions found matching the provided name")
	} else if len(subscriptions) == 1 {
		return subscriptions[0].Id, nil
	} else if len(subscriptions) >= 10 {
		return "", fmt.Errorf("too many subscriptions returned from filter, please try providing a more precise matching term")
	}

	validSelection := false
	selectedSub := ""
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Please select one of the following subscriptions")
	for i, sub := range subscriptions {
		fmt.Printf("%d: %s\n", i, sub.Name)
	}

	for !validSelection {
		fmt.Print("> ")

		selected, _ := reader.ReadString('\n')
		selected = strings.TrimSpace(selected)
		index, err := strconv.Atoi(selected)
		if err != nil || index < 0 || index >= len(subscriptions) {
			fmt.Println("Invalid selection. Please try again.")
			continue
		}
		selectedSub = subscriptions[index].Id
		validSelection = true
	}

	return selectedSub, nil
}

func generateBillingSummary() error {
	db, err := getCostManagementStore()
	if err != nil {
		return err
	}
	defer func(db *sqlite.CostManagementStore) {
		err := db.Close()
		if err != nil {
			log.Printf("Unable to close data store: %e", err)
		}
	}(db)

	summary, err := db.GenerateSummaryByResourceGroup(generateMonths)
	if err != nil {
		return err
	}

	var formatter formats.Formatter

	switch strings.ToLower(format) {
	case TextFormat:
		formatter, err = formats.NewTextFormatter(useStdOut, outputPath)
		break
	case CsvFormat:
		formatter, err = formats.NewCsvFormatter(useStdOut, outputPath)
		break
	case JsonFormat:
		formatter, err = formats.NewJsonFormatter(useStdOut, outputPath)
		break
	case ExcelFormat:
		formatter, err = formats.NewExcelFormatter(outputPath)
		break
	}
	if err != nil {
		return err
	}

	err = formatter.Generate(summary)
	return err
}

func displayCollectionStatus() error {
	db, err := getCostManagementStore()
	if err != nil {
		return err
	}
	defer func(db *sqlite.CostManagementStore) {
		err := db.Close()
		if err != nil {
			log.Printf("Unable to close data store: %e", err)
		}
	}(db)

	summaries, err := db.GetCollectionSummary()
	if err != nil {
		return err
	}

	fmt.Printf("%-51s%-38s%-9s\n", "Subscription", "Subscription Id", "Period")
	fmt.Printf("%-51s%-38s%-9s\n", strings.Repeat("=", 50), strings.Repeat("=", 37), strings.Repeat("=", 8))

	for _, summary := range summaries {
		name := summary.SubscriptionName
		if len(name) > 50 {
			name = name[:50]
		}

		fmt.Printf("%-50s %-37s %-9s\n", name, summary.SubscriptionId, summary.BillingPeriod.Format("2006-01"))
	}

	return nil
}

func displayTopLevelUsage() {
	fmt.Println(`Azure costs summary
A tool for collecting billing data from Azure, and producing summarized outputs

Author:
    Darren Fuller    https://github.com/dazfuller

Usage:
    azcosts [command]

Available Commands:
    subscription     Displays subscriptions available to the current user
    collect          Collects data from Azure and persists into a local store
    generate         Produces a summarized output of the billing data in multiple formats
    status           Displays the billing periods collected for each subscription

Flags:
    -h, -help        Help for azcosts`)
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
	rgSvc := azure.NewResourceGroupService()

	period := billingDate.Format("2006-01")

	rgs, err := rgSvc.ListResourceGroups(subscriptionId)
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

	err = db.SaveCosts(costs, rgs)
	if err != nil {
		return err
	}

	log.Printf("Successfully collected and saved billing data for subscription %s for %s", subscriptionId, period)

	return nil
}

func getCostManagementStore() (*sqlite.CostManagementStore, error) {
	dbPath, err := getDatabasePath()
	if err != nil {
		return nil, err
	}

	db, err := sqlite.NewCostManagementStore(dbPath, truncateDB)
	if err != nil {
		return nil, err
	}

	return db, nil
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
