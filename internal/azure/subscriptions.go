package azure

import (
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/dazfuller/azcosts/internal/model"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"net/http"
	"strings"
	"time"
)

type subscriptionResponse struct {
	Value []struct {
		Id                   string `json:"id"`
		SubscriptionId       string `json:"subscriptionId"`
		TenantId             string `json:"tenantId"`
		DisplayName          string `json:"displayName"`
		State                string `json:"state"`
		SubscriptionPolicies struct {
			LocationPlacementId string `json:"locationPlacementId"`
			QuotaId             string `json:"quotaId"`
			SpendingLimit       string `json:"spendingLimit"`
		} `json:"subscriptionPolicies"`
		AuthorizationSource string `json:"authorizationSource"`
		ManagedByTenants    []struct {
			TenantId string `json:"tenantId"`
		} `json:"managedByTenants"`
		Tags map[string]string `json:"tags"`
	} `json:"value"`
	NextLink string `json:"nextLink"`
}

type SubscriptionService struct {
	azureService
	apiVersion      string
	endpoint        string
	managementScope string
}

func NewSubscriptionService() SubscriptionService {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		panic(err)
	}

	return SubscriptionService{
		azureService: azureService{
			identity: cred,
		},
		apiVersion:      "2022-12-01",
		endpoint:        "https://management.azure.com/subscriptions",
		managementScope: "https://management.azure.com/.default",
	}
}

func (ss *SubscriptionService) FindSubscription(input string) ([]model.Subscription, error) {
	subs, err := ss.GetSubscriptions()
	if err != nil {
		return nil, err
	}

	var filtered []model.Subscription
	for i := range subs {
		subNameParts := strings.Split(subs[i].Name, " ")
		for _, part := range subNameParts {
			if fuzzy.MatchFold(input, part) {
				filtered = append(filtered, subs[i])
				break
			}
		}
	}

	return filtered, nil
}

func (ss *SubscriptionService) GetSubscriptions() ([]model.Subscription, error) {
	url := fmt.Sprintf(ss.endpoint + "?api-version=" + ss.apiVersion)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	token, err := ss.getAccessToken(ss.managementScope)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	client := http.Client{Timeout: time.Second * 10}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var subResp subscriptionResponse
	err = json.NewDecoder(resp.Body).Decode(&subResp)
	if err != nil {
		return nil, err
	}

	subscriptions := make([]model.Subscription, len(subResp.Value))
	for i, v := range subResp.Value {
		subscriptions[i] = model.Subscription{
			Id:       v.SubscriptionId,
			TenantId: v.TenantId,
			Name:     v.DisplayName,
		}
	}

	return subscriptions, nil
}
