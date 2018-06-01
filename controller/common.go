package controller

import (
	"strings"

	userv1alpha1 "github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"

	rbacv1 "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SuccessSynced         = "Synced"
	ErrResourceExists     = "ErrResourceExists"
	SearchIndexDefaultLen = 6

	InfraControllerAgentName      = "infra-controller"
	InfrastructureNamespace       = "kube-system"
	InfraMessageResourceExists    = "Resource %q already exists and is not managed by Infrastructure"
	InfraMessageResourceSynced    = "Infrastructure synced successfully"
	UserControllerAgentName       = "user-controller"
	UserNamespace                 = "kube-system"
	UserMessageResourceExists     = "Resource %q already exists and is not managed by User"
	UserMessageResourceSynced     = "User synced successfully"
	UserByPrincipalIndex          = "auth.user.cube.rancher.io/user-principal-index"
	UserByUsernameIndex           = "auth.user.cube.rancher.io/user-username-index"
	UserSearchIndex               = "auth.user.cube.rancher.io/user-search-index"
	PrincipalByIDIndex            = "auth.user.cube.rancher.io/principal-id-index"
	TokenByNameIndex              = "auth.user.cube.rancher.io/token-name-index"
	TokenByKeyIndex               = "auth.user.cube.rancher.io/token-key-index"
	UserIDLabel                   = "auth.user.cube.rancher.io/token-user-id"
	ClusterRoleBindingByNameIndex = "auth.user.cube.rancher.io/crb-name"
	ClusterRoleByNameIndex        = "auth.user.cube.rancher.io/cr-name"
	LonghornNamespace             = "longhorn-system"

	DashboardDesc = "Kubernetes Dashboard is a general purpose, web-based UI for Kubernetes clusters. It allows users to manage applications running in the cluster and troubleshoot them, as well as manage the cluster itself."
	LanghornDesc  = "Longhorn is a distributed block storage system for Kubernetes powered by Rancher Labs."
)

func GetLocalPrincipalID(user *userv1alpha1.User) string {
	var principalID string
	for _, p := range user.PrincipalIDs {
		if strings.HasPrefix(p, "local.") {
			principalID = p
		}
	}
	if principalID == "" {
		principalID = "local." + user.Namespace + "." + user.Name
	}
	return principalID
}

func UserByPrincipal(obj interface{}) ([]string, error) {
	u, ok := obj.(*userv1alpha1.User)
	if !ok {
		return []string{}, nil
	}

	return u.PrincipalIDs, nil
}

func UserByUsername(obj interface{}) ([]string, error) {
	user, ok := obj.(*userv1alpha1.User)
	if !ok {
		return []string{}, nil
	}
	return []string{user.Username}, nil
}

func UserSearchIndexer(obj interface{}) ([]string, error) {
	user, ok := obj.(*userv1alpha1.User)
	if !ok {
		return []string{}, nil
	}
	var fieldIndexes []string

	fieldIndexes = append(fieldIndexes, indexField(user.Username, minOf(len(user.Username), SearchIndexDefaultLen))...)
	fieldIndexes = append(fieldIndexes, indexField(user.DisplayName, minOf(len(user.DisplayName), SearchIndexDefaultLen))...)
	fieldIndexes = append(fieldIndexes, indexField(user.ObjectMeta.Name, minOf(len(user.ObjectMeta.Name), SearchIndexDefaultLen))...)

	return fieldIndexes, nil
}

func PrincipalByID(obj interface{}) ([]string, error) {
	principal, ok := obj.(*userv1alpha1.Principal)
	if !ok {
		return []string{}, nil
	}
	return []string{principal.Name}, nil
}

func TokenByName(obj interface{}) ([]string, error) {
	token, ok := obj.(*userv1alpha1.Token)
	if !ok {
		return []string{}, nil
	}
	return []string{token.Name}, nil
}

func TokenByKey(obj interface{}) ([]string, error) {
	token, ok := obj.(*userv1alpha1.Token)
	if !ok {
		return []string{}, nil
	}

	return []string{token.Token}, nil
}

func ClusterRoleBindingByName(obj interface{}) ([]string, error) {
	crb, ok := obj.(*rbacv1.ClusterRoleBinding)
	if !ok {
		return []string{}, nil
	}
	return []string{crb.Name}, nil
}

func ClusterRoleByName(obj interface{}) ([]string, error) {
	cr, ok := obj.(*rbacv1.ClusterRole)
	if !ok {
		return []string{}, nil
	}
	return []string{cr.Name}, nil
}

func ToPrincipal(principalType, displayName, loginName, namespace, id string, token *userv1alpha1.Token) userv1alpha1.Principal {
	if displayName == "" {
		displayName = loginName
	}

	princ := userv1alpha1.Principal{
		ObjectMeta:  metav1.ObjectMeta{Name: id, Namespace: namespace},
		DisplayName: displayName,
		LoginName:   loginName,
		Provider:    "local",
		Me:          false,
	}

	if principalType == "user" {
		princ.PrincipalType = "user"
		if token != nil {
			princ.Me = isThisUserMe(token.UserPrincipal, princ)
		}
	} else {
		princ.PrincipalType = "group"
		if token != nil {
			princ.MemberOf = isMemberOf(token.GroupPrincipals, princ)
		}
	}

	return princ
}

func isThisUserMe(me userv1alpha1.Principal, other userv1alpha1.Principal) bool {

	if me.ObjectMeta.Name == other.ObjectMeta.Name && me.LoginName == other.LoginName && me.PrincipalType == other.PrincipalType {
		return true
	}
	return false
}

func isMemberOf(myGroups []userv1alpha1.Principal, other userv1alpha1.Principal) bool {
	for _, mygroup := range myGroups {
		if mygroup.ObjectMeta.Name == other.ObjectMeta.Name && mygroup.PrincipalType == other.PrincipalType {
			return true
		}
	}
	return false
}

func minOf(length int, defaultLen int) int {
	if length < defaultLen {
		return length
	}
	return defaultLen
}

func indexField(field string, maxindex int) []string {
	var fieldIndexes []string
	for i := 2; i <= maxindex; i++ {
		fieldIndexes = append(fieldIndexes, field[0:i])
	}
	return fieldIndexes
}

func matchPrincipalID(principalIds []string, id string) bool {
	for _, val := range principalIds {
		if val == id {
			return true
		}
	}
	return false
}
