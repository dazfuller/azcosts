package formats

import (
	"encoding/csv"
	"fmt"
	"github.com/dazfuller/azcosts/internal/model"
	"os"
	"strconv"
)

type CsvFormatter struct {
	useStdOut  bool
	outputPath string
}

func MakeCsvFormatter(useStdOut bool, outputPath string) (CsvFormatter, error) {
	if err := validateOptions(useStdOut, outputPath); err != nil {
		return CsvFormatter{}, err
	}

	return CsvFormatter{useStdOut: useStdOut, outputPath: outputPath}, nil
}

func (cf CsvFormatter) Generate(costs []model.ResourceGroupSummary) error {
	var writer *csv.Writer

	if cf.useStdOut {
		writer = csv.NewWriter(os.Stdout)
	} else {
		file, err := os.Create(cf.outputPath)
		if err != nil {
			return err
		}
		writer = csv.NewWriter(file)
	}

	// Write header
	header := []string{"Name", "Subscription Name", "Active"}
	for _, cost := range costs[0].Costs {
		header = append(header, cost.Period)
	}
	header = append(header, "Total Costs")
	err := writer.Write(header)
	if err != nil {
		return err
	}

	for _, rg := range costs {
		record := []string{rg.Name, rg.SubscriptionName, strconv.FormatBool(rg.Active)}
		for _, cost := range rg.Costs {
			record = append(record, fmt.Sprintf("%.2f", cost.Total))
		}
		record = append(record, fmt.Sprintf("%.2f", rg.TotalCost))

		err := writer.Write(record)
		if err != nil {
			return err
		}
	}

	writer.Flush()
	return nil
}
