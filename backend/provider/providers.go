package provider

import (
	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"
)

type GenericLogin struct {
	TTLMillis    int64  `json:"ttl,omitempty"`
	ProviderType string `json:"providerType,omitempty"`
	Description  string `json:"description,omitempty"`
	ResponseType string `json:"responseType,omitempty"`
}

type BasicLogin struct {
	GenericLogin `json:",inline"`
	Username     string `json:"username"`
	Password     string `json:"password"`
}

type AuthProvider interface {
	GetName() string
	AuthenticateUser(input interface{}, providerName string) (v1alpha1.Principal, map[string]string, error)
	SearchPrincipals(name, principalType string, myToken v1alpha1.Token) ([]v1alpha1.Principal, error)
	GetPrincipal(principalID string, token v1alpha1.Token) (v1alpha1.Principal, error)
	SearchToken(tokenName, providerName string) (*v1alpha1.Token, error)
}
