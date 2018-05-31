package controller

const (
	SuccessSynced              = "Synced"
	ErrResourceExists          = "ErrResourceExists"
	InfraControllerAgentName   = "infra-controller"
	InfrastructureNamespace    = "kube-system"
	InfraMessageResourceExists = "Resource %q already exists and is not managed by Infrastructure"
	InfraMessageResourceSynced = "Infrastructure synced successfully"
	UserControllerAgentName    = "user-controller"
	UserMessageResourceExists  = "Resource %q already exists and is not managed by User"
	UserMessageResourceSynced  = "User synced successfully"
	UserByPrincipalIndex       = "auth.management.cattle.io/userByPrincipal"
)
