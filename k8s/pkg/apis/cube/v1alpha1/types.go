package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=infrastructure
type Infrastructure struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   InfraSpec   `json:"spec"`
	Status InfraStatus `json:"status"`
}

type InfraSpec struct {
	DisplayName string `json:"displayName"`
	Desc        string `json:"desc"`
	Icon        string `json:"icon"`
	Replicas    *int32 `json:"replicas"`
	InfraKind   string `json:"infraKind"`
}

type InfraStatus struct {
	State             string `json:"state"`
	AvailableReplicas int32  `json:"availableReplicas"`
	Message           string `json:"message"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=infrastructures
type InfrastructureList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Infrastructure `json:"items"`
}
