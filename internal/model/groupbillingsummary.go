package model

type BillingPeriodCost struct {
	Period string  `json:"period"`
	Total  float64 `json:"total"`
}

type ResourceGroupSummary struct {
	Name             string              `json:"name"`
	SubscriptionName string              `json:"subscriptionName"`
	Active           bool                `json:"active"`
	Costs            []BillingPeriodCost `json:"costs"`
	TotalCost        float64             `json:"totalCost"`
}

type SubscriptionSummary struct {
	Name      string              `json:"name"`
	Costs     []BillingPeriodCost `json:"costs"`
	TotalCost float64             `json:"totalCost"`
}
