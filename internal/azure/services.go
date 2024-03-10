package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

type azureService struct {
	identity azcore.TokenCredential
}

func (svc *azureService) getAccessToken(scope string) (string, error) {
	token, err := svc.identity.GetToken(context.Background(), policy.TokenRequestOptions{Scopes: []string{scope}})
	if err != nil {
		return "", err
	}
	return token.Token, nil
}
