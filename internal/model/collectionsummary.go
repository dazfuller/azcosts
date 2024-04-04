package model

import "time"

type CollectionSummary struct {
	SubscriptionId   string
	SubscriptionName string
	BillingPeriod    time.Time
}
