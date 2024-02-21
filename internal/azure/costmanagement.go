package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/dazfuller/azcosts/internal/model"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	apiVersion      = "2023-11-01"
	endpoint        = "https://management.azure.com/subscriptions/%s/providers/Microsoft.CostManagement/query"
	managementScope = "https://management.azure.com/.default"
)

type timePeriod struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type aggregation struct {
	TotalCost    aggregationFunction `json:"totalCost"`
	TotalCostUSD aggregationFunction `json:"totalCostUSD"`
}

type aggregationFunction struct {
	Name     string `json:"name"`
	Function string `json:"function"`
}

type grouping struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type dataset struct {
	Granularity string      `json:"granularity"`
	Aggregation aggregation `json:"aggregation"`
	Grouping    []grouping  `json:"grouping"`
}

type costManagementRequest struct {
	Type       string     `json:"type"`
	TimeFrame  string     `json:"timeFrame"`
	TimePeriod timePeriod `json:"timePeriod"`
	DataSet    dataset    `json:"dataSet"`
}

type costResponse struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Location   string `json:"location"`
	Sku        string `json:"sku"`
	ETag       string `json:"eTag"`
	Properties struct {
		NextLink interface{} `json:"nextLink"`
		Columns  []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"columns"`
		Rows [][]interface{} `json:"rows"`
	} `json:"properties"`
}

type CostService struct {
	identity azcore.TokenCredential
}

func NewCostService() CostService {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		panic(err)
	}

	return CostService{
		identity: cred,
	}
}

func (svc *CostService) getAccessToken(scope string) (string, error) {
	token, err := svc.identity.GetToken(context.Background(), policy.TokenRequestOptions{Scopes: []string{scope}})
	if err != nil {
		return "", err
	}
	return token.Token, nil
}

func (svc *CostService) ResourceGroupCostsForPeriod(subscriptionId string, year int, month int) ([]model.ResourceGroupCost, error) {
	currentTime := time.Now().UTC()

	// Validate that the year is not in the future
	if year < 1970 || year > currentTime.Year() {
		return nil, fmt.Errorf("invalid year")
	}

	// Validate that the month is valid
	if month < 1 || month > 12 {
		return nil, fmt.Errorf("invalid month")
	}

	billingFrom := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	billingTo := billingFrom.AddDate(0, 1, 0).Add(time.Second * -1)

	if billingFrom.After(currentTime) {
		return nil, fmt.Errorf("billing period is in the future")
	}

	token, err := svc.getAccessToken(managementScope)
	if err != nil {
		return nil, fmt.Errorf("unable to acquire token: %s", err.Error())
	}

	requestData := costManagementRequest{
		Type:      "ActualCost",
		TimeFrame: "Custom",
		TimePeriod: timePeriod{
			From: billingFrom,
			To:   billingTo,
		},
		DataSet: dataset{
			Granularity: "None",
			Aggregation: aggregation{
				TotalCost: aggregationFunction{
					Name:     "Cost",
					Function: "Sum",
				},
				TotalCostUSD: aggregationFunction{
					Name:     "CostUSD",
					Function: "Sum",
				},
			},
			Grouping: []grouping{
				{
					Type: "Dimension",
					Name: "ResourceGroupName",
				},
				{
					Type: "Dimension",
					Name: "SubscriptionName",
				},
				{
					Type: "Dimension",
					Name: "SubscriptionId",
				},
			},
		},
	}

	requestContent, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal request data: %s", err.Error())
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf(endpoint, subscriptionId), bytes.NewReader(requestContent))
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %s", err.Error())
	}
	q := req.URL.Query()
	q.Add("api-version", apiVersion)
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("ClientType", "CostManagementAppV1")

	resp, err := makeRequest(req, 3)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseVal := costResponse{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&responseVal)

	if err != nil {
		return nil, fmt.Errorf("unable to decode response: %s", err.Error())
	}

	columns := make(map[string]int)

	for i, v := range responseVal.Properties.Columns {
		columns[v.Name] = i
	}

	costs := make([]model.ResourceGroupCost, len(responseVal.Properties.Rows))

	for i, r := range responseVal.Properties.Rows {
		costs[i] = model.ResourceGroupCost{
			SubscriptionId:   r[columns["SubscriptionId"]].(string),
			SubscriptionName: r[columns["SubscriptionName"]].(string),
			Name:             r[columns["ResourceGroupName"]].(string),
			BillingPeriod:    billingFrom,
			Cost:             r[columns["Cost"]].(float64),
			CostUSD:          r[columns["CostUSD"]].(float64),
			Currency:         r[columns["Currency"]].(string),
		}
	}

	return costs, nil
}

func makeRequest(req *http.Request, retryLimit int) (*http.Response, error) {
	attempt := 1
	for attempt <= retryLimit {
		client := http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("unable to make request: %s", err.Error())
		}

		if resp.StatusCode == http.StatusOK {
			return resp, nil
		} else if resp.StatusCode == 429 {
			retryAfter := resp.Header.Get("X-Ms-Ratelimit-Microsoft.costmanagement-Entity-Retry-After")
			if len(retryAfter) == 0 {
				retryAfter = "40"
			}
			retryDuration, err := time.ParseDuration(fmt.Sprintf("%ss", retryAfter))
			if err != nil {
				return nil, fmt.Errorf("unable to parse retry duration: %s", err.Error())
			}
			log.Printf("API request throttled, attempting again in %ss", retryAfter)
			time.Sleep(retryDuration)
		} else {
			respContent, err := io.ReadAll(resp.Body)
			if err != nil {
				respContent = []byte("No response body")
			}
			resp.Body.Close()

			return nil, fmt.Errorf("invalid request. %s: %s", resp.Status, respContent)
		}

		// Rate limit retry info header item: X-Ms-Ratelimit-Microsoft.costmanagement-Entity-Retry-After

		attempt++
	}

	return nil, fmt.Errorf("unable to successfully query cost management api after %d attempt(s)", retryLimit)
}
