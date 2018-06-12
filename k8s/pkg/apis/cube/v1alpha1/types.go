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
	DisplayName string      `json:"displayName"`
	Description string      `json:"description"`
	Icon        string      `json:"icon"`
	Replicas    *int32      `json:"replicas"`
	InfraKind   string      `json:"infraKind"`
	Images      InfraImages `json:"images"`
}

type InfraStatus struct {
	State             string `json:"state"`
	AvailableReplicas int32  `json:"availableReplicas"`
	Message           string `json:"message"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=infrastructures

type InfrastructureList struct {
	metav1.TypeMeta        `json:",inline"`
	metav1.ListMeta        `json:"metadata"`
	Items []Infrastructure `json:"items"`
}

type InfraImages struct {
	// Dashboard image
	Dashboard string `yaml:"dashboard" json:"dashboard,omitempty"`
	//Longhorn Manager image
	LonghornManager string `yaml:"longhorn_manager" json:"longhornManager,omitempty"`
	//Longhorn Driver image
	LonghornFlexvolumeDriver string `yaml:"longhorn_flexvolume-driver" json:"longhornFlexvolumeDriver,omitempty"`
	//Longhorn Ui image
	LonghornUi string `yaml:"longhorn_ui" json:"longhornUi,omitempty"`
	//RancherVM image
	RancherVM string `yaml:"ranchervm" json:"rancherVM,omitempty"`
	//RancherVM Frontend image
	RancherVMFrontend string `yaml:"ranchervm_frontend" json:"rancherVMFrontend,omitempty"`
}
