package formats

import (
	"errors"
	"fmt"
	"github.com/dazfuller/azcosts/internal/model"
	"os"
)

type Formatter interface {
	Generate(costs []model.ResourceGroupSummary) error
}

func validateOptions(useStdOut bool, outputPath string) error {
	if !useStdOut && len(outputPath) == 0 {
		return fmt.Errorf("when writing to file and file path must be specified")
	}

	if !useStdOut {
		_, err := os.Stat(outputPath)
		if !errors.Is(err, os.ErrNotExist) {
			err := os.Remove(outputPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func generateSubscriptionSummary(costs []model.ResourceGroupSummary) []model.SubscriptionSummary {
	subscriptions := make(map[string]*model.SubscriptionSummary)

	for _, cost := range costs {
		name := cost.SubscriptionName
		if _, ok := subscriptions[name]; !ok {
			subCosts := make([]model.BillingPeriodCost, len(cost.Costs))

			for i, cost := range cost.Costs {
				subCosts[i] = model.BillingPeriodCost{
					Period: cost.Period,
					Total:  0,
				}
			}

			subscription := &model.SubscriptionSummary{
				Name:      name,
				Costs:     subCosts,
				TotalCost: 0,
			}

			subscriptions[name] = subscription
		}

		subscription := subscriptions[name]

		for i, bp := range cost.Costs {
			subscription.Costs[i].Total += bp.Total
			subscription.TotalCost += bp.Total
		}
	}

	subscriptionSummary := make([]model.SubscriptionSummary, 0, len(subscriptions))
	for _, sub := range subscriptions {
		subscriptionSummary = append(subscriptionSummary, *sub)
	}

	return subscriptionSummary
}
