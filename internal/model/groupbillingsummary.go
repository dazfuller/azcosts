package model

type BillingPeriodCost struct {
	Period string
	Total  float64
}

type ResourceGroupSummary struct {
	Name             string
	SubscriptionName string
	Costs            []BillingPeriodCost
	TotalCost        float64
}
