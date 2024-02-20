package formats

import (
	"bufio"
	"fmt"
	"github.com/dazfuller/azcosts/internal/model"
	"os"
	"strings"
)

type TextFormatter struct {
	useStdOut  bool
	outputPath string
}

func MakeTextFormatter(useStdOut bool, outputPath string) (TextFormatter, error) {
	if err := validateOptions(useStdOut, outputPath); err != nil {
		return TextFormatter{}, err
	}

	return TextFormatter{useStdOut: useStdOut, outputPath: outputPath}, nil
}

func (tf TextFormatter) Generate(costs []model.ResourceGroupSummary) error {
	var writer *bufio.Writer

	if tf.useStdOut {
		writer = bufio.NewWriter(os.Stdout)
	} else {
		file, err := os.Create(tf.outputPath)
		if err != nil {
			return err
		}
		writer = bufio.NewWriter(file)
	}

	writer.WriteString(fmt.Sprintf("%-70s %-30s", "Resource Group", "Subscription"))

	for _, bp := range costs[0].Costs {
		writer.WriteString(fmt.Sprintf(" %12s", bp.Period))
	}

	writer.WriteString(fmt.Sprintf("%12s\n", "Total Costs"))

	writer.WriteString(fmt.Sprintf("%-70s %-30s", strings.Repeat("=", 70), strings.Repeat("=", 30)))

	for range costs[0].Costs {
		writer.WriteString(fmt.Sprintf(" %12s", strings.Repeat("=", 12)))
	}

	writer.WriteString(fmt.Sprintf(" %12s\n", strings.Repeat("=", 12)))

	for _, rg := range costs {
		writer.WriteString(fmt.Sprintf("%-70s %-30s", trimValue(rg.Name, 50), trimValue(rg.SubscriptionName, 30)))
		for _, cost := range rg.Costs {
			writer.WriteString(fmt.Sprintf(" %12.2f", cost.Total))
		}
		writer.WriteString(fmt.Sprintf(" %12.2f\n", rg.TotalCost))
	}

	writer.Flush()
	return nil
}

func trimValue(value string, maxLen int) string {
	if len(value) > maxLen {
		return value[0:maxLen]
	}
	return value
}
