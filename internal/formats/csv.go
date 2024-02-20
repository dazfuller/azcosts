package formats

import (
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/dazfuller/azcosts/internal/model"
	"os"
)

type CsvFormatter struct {
	useStdOut  bool
	outputPath string
}

func MakeCsvFormatter(useStdOut bool, outputPath string) (CsvFormatter, error) {
	if !useStdOut && len(outputPath) == 0 {
		return CsvFormatter{}, fmt.Errorf("when writing to file and file path must be specified")
	}

	if !useStdOut {
		_, err := os.Stat(outputPath)
		if !errors.Is(err, os.ErrNotExist) {
			err := os.Remove(outputPath)
			if err != nil {
				return CsvFormatter{}, err
			}
		}
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
	header := []string{"Name", "Subscription Name"}
	for _, cost := range costs[0].Costs {
		header = append(header, cost.Period)
	}
	header = append(header, "Total Costs")
	err := writer.Write(header)
	if err != nil {
		return err
	}

	for _, rg := range costs {
		record := []string{rg.Name, rg.SubscriptionName}
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
