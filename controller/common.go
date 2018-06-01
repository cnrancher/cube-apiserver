package controller

const (
	SuccessSynced         = "Synced"
	ErrResourceExists     = "ErrResourceExists"
	searchIndexDefaultLen = 6

	InfraControllerAgentName   = "infra-controller"
	InfrastructureNamespace    = "kube-system"
	InfraMessageResourceExists = "Resource %q already exists and is not managed by Infrastructure"
	InfraMessageResourceSynced = "Infrastructure synced successfully"
	UserControllerAgentName    = "user-controller"
	UserNamespace              = "kube-system"
	UserMessageResourceExists  = "Resource %q already exists and is not managed by User"
	UserMessageResourceSynced  = "User synced successfully"
	UserByPrincipalIndex       = "auth.user.cube.rancher.io/user-principal-index"
	UserByUsernameIndex        = "auth.user.cube.rancher.io/user-username-index"
	UserSearchIndex            = "auth.user.cube.rancher.io/user-search-index"
	PrincipalByIdIndex         = "auth.user.cube.rancher.io/principal-id-index"
)
