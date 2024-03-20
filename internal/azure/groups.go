package azure

import (
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/dazfuller/azcosts/internal/model"
	"net/http"
	"time"
)

type resourceGroupResponse struct {
	Value []struct {
		Id         string `json:"id"`
		Location   string `json:"location"`
		Name       string `json:"name"`
		Properties struct {
			ProvisioningState string `json:"provisioningState"`
		} `json:"properties"`
		Tags      map[string]string `json:"tags,omitempty"`
		Type      string            `json:"type"`
		ManagedBy string            `json:"managedBy,omitempty"`
	} `json:"value"`
}

type ResourceGroupService struct {
	azureService
	apiVersion      string
	endpoint        string
	managementScope string
}

func NewResourceGroupService() ResourceGroupService {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		panic(err)
	}

	return ResourceGroupService{
		azureService: azureService{
			identity: cred,
		},
		apiVersion:      "2021-04-01",
		endpoint:        "https://management.azure.com",
		managementScope: "https://management.azure.com/.default",
	}
}

func (rgs *ResourceGroupService) ListResourceGroups(subscriptionId string) ([]model.ResourceGroup, error) {
	url := fmt.Sprintf("%s/subscriptions/%s/resourcegroups?api-version=%s", rgs.endpoint, subscriptionId, rgs.apiVersion)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	token, err := rgs.getAccessToken(rgs.managementScope)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	client := http.Client{Timeout: time.Second * 10}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var resGroupResp resourceGroupResponse
	err = json.NewDecoder(resp.Body).Decode(&resGroupResp)
	if err != nil {
		return nil, err
	}

	resourceGroups := make([]model.ResourceGroup, len(resGroupResp.Value))
	for i, rg := range resGroupResp.Value {
		resourceGroups[i] = model.ResourceGroup{
			Id:       rg.Id,
			Name:     rg.Name,
			Location: rg.Location,
		}
	}

	return resourceGroups, nil
}
