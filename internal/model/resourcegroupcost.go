package model

import "time"

type ResourceGroupCost struct {
	SubscriptionId   string
	SubscriptionName string
	Name             string
	BillingPeriod    time.Time
	Cost             float64
	CostUSD          float64
	Currency         string
}
