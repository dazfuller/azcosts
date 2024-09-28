package formats

import (
	"cmp"
	"errors"
	"fmt"
	"github.com/dazfuller/azcosts/internal/model"
	"log"
	"os"
	"slices"
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

		// Create a new entry for the subscription if it doesn't already exist
		if _, ok := subscriptions[name]; !ok {
			subscription := &model.SubscriptionSummary{
				Name:      name,
				Costs:     []model.BillingPeriodCost{},
				TotalCost: 0,
			}

			subscriptions[name] = subscription
		}

		// Get the subscription entry for the current resource group
		subscription := subscriptions[name]

		// Iterate over the billing period values for the resource group
		for _, bp := range cost.Costs {
			// Find the billing period entry for the current resource group billing period
			subscriptionBillingPeriodIndex := slices.IndexFunc(subscription.Costs, func(bpc model.BillingPeriodCost) bool {
				return bpc.Period == bp.Period
			})

			// If no billing period exists then create one, otherwise increment the subscriptions billing period
			// values by the resource groups costs
			if subscriptionBillingPeriodIndex == -1 {
				subscription.Costs = append(subscription.Costs, model.BillingPeriodCost{
					Period: bp.Period,
					Total:  bp.Total,
				})

				subscription.TotalCost += bp.Total
			} else {
				subscription.Costs[subscriptionBillingPeriodIndex].Total += bp.Total
				subscription.TotalCost += bp.Total
			}
		}
	}

	subscriptionSummary := make([]model.SubscriptionSummary, 0, len(subscriptions))
	for _, sub := range subscriptions {
		slices.SortFunc(sub.Costs, func(a, b model.BillingPeriodCost) int {
			aTime, err := a.PeriodAsTime()
			if err != nil {
				log.Fatal("Unable to cast billing period as time")
			}

			bTime, err := b.PeriodAsTime()
			if err != nil {
				log.Fatal("Unable to cast billing period as time")
			}

			return cmp.Compare(aTime.Unix(), bTime.Unix())
		})

		subscriptionSummary = append(subscriptionSummary, *sub)
	}

	return subscriptionSummary
}
