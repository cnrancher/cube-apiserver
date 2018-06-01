package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=user
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName        string   `json:"displayName,omitempty"`
	Description        string   `json:"description"`
	Username           string   `json:"username,omitempty"`
	Password           string   `json:"password,omitempty"`
	MustChangePassword bool     `json:"mustChangePassword,omitempty"`
	PrincipalIDs       []string `json:"principalIds,omitempty"`
	Me                 bool     `json:"me,omitempty"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=group
type Group struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName string `json:"displayName,omitempty"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=groupmember
type GroupMember struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	GroupName   string `json:"groupName,omitempty"`
	PrincipalID string `json:"principalId,omitempty"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=principal
type Principal struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName    string            `json:"displayName,omitempty"`
	LoginName      string            `json:"loginName,omitempty"`
	ProfilePicture string            `json:"profilePicture,omitempty"`
	ProfileURL     string            `json:"profileURL,omitempty"`
	PrincipalType  string            `json:"principalType,omitempty"`
	Me             bool              `json:"me,omitempty"`
	MemberOf       bool              `json:"memberOf,omitempty"`
	Provider       string            `json:"provider,omitempty"`
	ExtraInfo      map[string]string `json:"extraInfo,omitempty"`
}

type AuthConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Type                string   `json:"type"`
	Enabled             bool     `json:"enabled,omitempty"`
	AccessMode          string   `json:"accessMode,omitempty"`
	AllowedPrincipalIDs []string `json:"allowedPrincipalIds,omitempty"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=localconfig
type LocalConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthConfig        `json:",inline" mapstructure:",squash"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=token
type Token struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Token           string            `json:"token"`
	UserPrincipal   Principal         `json:"userPrincipal"`
	GroupPrincipals []Principal       `json:"groupPrincipals"`
	ProviderInfo    map[string]string `json:"providerInfo,omitempty"`
	UserID          string            `json:"userId"`
	AuthProvider    string            `json:"authProvider"`
	TTLMillis       int64             `json:"ttl"`
	LastUpdateTime  string            `json:"lastUpdateTime"`
	IsDerived       bool              `json:"isDerived"`
	Description     string            `json:"description"`
	Expired         bool              `json:"expired"`
	ExpiresAt       string            `json:"expiresAt"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=users
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []User `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=groups
type GroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Group `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=groupmembers
type GroupMemberList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []GroupMember `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=principals
type PrincipalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Principal `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=localconfigs
type LocalConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []LocalConfig `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=tokens
type TokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Token `json:"items"`
}
