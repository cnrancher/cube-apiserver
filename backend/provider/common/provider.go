package common

import (
	"fmt"
	"sync"

	"github.com/cnrancher/cube-apiserver/backend"
	"github.com/cnrancher/cube-apiserver/backend/provider"
	"github.com/cnrancher/cube-apiserver/backend/provider/local"
	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"
)

var (
	providers       = make(map[string]provider.AuthProvider)
	localProvider   = "local"
	providersByType = make(map[string]provider.AuthProvider)
	confMu          sync.Mutex
)

func GetProvider(providerName string) (provider.AuthProvider, error) {
	if provider, ok := providers[providerName]; ok {
		if provider != nil {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("RancherCUBE: no such provider '%s'", providerName)
}

func GetProviderByType(configType string) provider.AuthProvider {
	return providersByType[configType]
}

func Configure(clientGenerator *backend.ClientGenerator) {
	confMu.Lock()
	defer confMu.Unlock()
	var p provider.AuthProvider
	p = local.Configure(clientGenerator)
	providers[local.ProviderName] = p
	providersByType[local.ConfigType] = p
	providersByType[local.ProviderName] = p
}

func AuthenticateUser(input interface{}, providerName string) (v1alpha1.Principal, map[string]string, error) {
	return providers[providerName].AuthenticateUser(input, providerName)
}

func GetPrincipal(principalID string, myToken v1alpha1.Token) (v1alpha1.Principal, error) {
	principal, err := providers[myToken.AuthProvider].GetPrincipal(principalID, myToken)
	if err != nil && myToken.AuthProvider != localProvider {
		principal, err = providers[localProvider].GetPrincipal(principalID, myToken)
	}
	return principal, err
}

func SearchPrincipals(name, principalType string, myToken v1alpha1.Token) ([]v1alpha1.Principal, error) {
	principals, err := providers[myToken.AuthProvider].SearchPrincipals(name, principalType, myToken)
	if err != nil {
		return principals, err
	}
	if myToken.AuthProvider != localProvider {
		lp := providers[localProvider]
		if lpDedupe, _ := lp.(*local.Provider); lpDedupe != nil {
			localPrincipals, err := lpDedupe.SearchPrincipalsDedupe(name, principalType, myToken, principals)
			if err != nil {
				return principals, err
			}
			principals = append(principals, localPrincipals...)
		}
	}
	return principals, err
}

func GenerateToken(namespace string, token v1alpha1.Token, providerName string) (*v1alpha1.Token, error) {
	return providers[providerName].GenerateToken(namespace, token, providerName)
}
