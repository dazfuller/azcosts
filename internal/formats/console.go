package formats

import (
	"fmt"
	"github.com/dazfuller/azcosts/internal/model"
	"strings"
)

type ConsoleFormatter struct{}

func MakeConsoleFormatter() ConsoleFormatter {
	return ConsoleFormatter{}
}

func (ConsoleFormatter) Generate(costs []model.ResourceGroupSummary) error {
	fmt.Printf("%-50s %-30s", "Resource Group", "Subscription")

	for _, bp := range costs[0].Costs {
		fmt.Printf(" %12s", bp.Period)
	}

	fmt.Printf("%12s\n", "Total Costs")

	fmt.Printf("%-50s %-30s", strings.Repeat("=", 50), strings.Repeat("=", 30))

	for range costs[0].Costs {
		fmt.Printf(" %12s", strings.Repeat("=", 12))
	}

	fmt.Printf(" %12s\n", strings.Repeat("=", 12))

	for _, rg := range costs {
		fmt.Printf("%-50s %-30s", trimValue(rg.Name, 50), trimValue(rg.SubscriptionName, 30))
		for _, cost := range rg.Costs {
			fmt.Printf(" %12.2f", cost.Total)
		}
		fmt.Printf(" %12.2f\n", rg.TotalCost)
	}

	return nil
}

func trimValue(value string, maxLen int) string {
	if len(value) > maxLen {
		return value[0:maxLen]
	}
	return value
}
