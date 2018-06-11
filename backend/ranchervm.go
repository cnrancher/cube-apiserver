package backend

import (
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RancherVMGroup   = "vm.rancher.com"
	RancherVMVersion = "v1alpha1"
	RancherVMLabel   = "ranchervm-system"

	VirtualMachineKind     = "VirtualMachine"
	VirtualMachine         = "virtualmachine"
	VirtualMachinePlural   = "virtualmachines"
	VirtualMachineList     = "VirtualMachineList"
	VirtualMachineFullName = VirtualMachinePlural + "." + RancherVMGroup

	ArptableKind     = "ARPTable"
	Arptable         = "arptable"
	ArptablePlural   = "arptables"
	ArptableList     = "ARPTableList"
	ArptableFullName = ArptablePlural + "." + RancherVMGroup

	CredentialKind     = "Credential"
	Credential         = "credential"
	CredentialPlural   = "credentials"
	CredentialList     = "CredentialList"
	CredentialFullName = CredentialPlural + "." + RancherVMGroup
)

func (c *ClientGenerator) CredentialCRDDeploy() error {
	crd := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{
			Name:   CredentialFullName,
			Labels: map[string]string{RancherVMLabel: "Credential"},
		},

		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:   RancherVMGroup,
			Version: RancherVMVersion,
			Scope:   v1beta1.ClusterScoped,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural:   CredentialPlural,
				Kind:     CredentialKind,
				ListKind: CredentialList,
				ShortNames: []string{
					"cred",
					"creds",
				},
				Singular: Credential,
			},
		},
	}

	_, err := c.Apiclientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err != nil && k8serrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (c *ClientGenerator) ArptableCRDDeploy() error {
	crd := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{
			Name:   ArptableFullName,
			Labels: map[string]string{RancherVMLabel: "ARPTable"},
		},

		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:   RancherVMGroup,
			Version: RancherVMVersion,
			Scope:   v1beta1.ClusterScoped,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural:   ArptablePlural,
				Kind:     ArptableKind,
				ListKind: ArptableList,
				ShortNames: []string{
					"arp",
					"arps",
				},
				Singular: Arptable,
			},
		},
	}

	_, err := c.Apiclientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err != nil && k8serrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (c *ClientGenerator) VirtualMachineCRDDeploy() error {
	crd := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{
			Name:   VirtualMachineFullName,
			Labels: map[string]string{RancherVMLabel: "VirtualMachine"},
		},

		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:   RancherVMGroup,
			Version: RancherVMVersion,
			Scope:   v1beta1.ClusterScoped,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural:   VirtualMachinePlural,
				Kind:     VirtualMachineKind,
				ListKind: VirtualMachineList,
				ShortNames: []string{
					"vm",
					"vms",
				},
				Singular: VirtualMachine,
			},
		},
	}

	_, err := c.Apiclientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err != nil && k8serrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}
