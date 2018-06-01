package backend

import (
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	LonghornGroup   = "longhorn.rancher.io"
	LonghornVersion = "v1alpha1"
	LonghornLabel   = "longhorn-manager"

	EngineKind     = "Engine"
	Engine         = "engine"
	EnginePlural   = "engines"
	EngineList     = "EngineList"
	EngineFullName = EnginePlural + "." + LonghornGroup

	ReplicaKind     = "Replica"
	Replica         = "replica"
	ReplicaPlural   = "replicas"
	ReplicaList     = "ReplicaList"
	ReplicaFullName = ReplicaPlural + "." + LonghornGroup

	SettingKind     = "Setting"
	Setting         = "setting"
	SettingPlural   = "settings"
	SettingList     = "SettingList"
	SettingFullName = SettingPlural + "." + LonghornGroup

	VolumeKind     = "Volume"
	Volume         = "volume"
	VolumePlural   = "volumes"
	VolumeList     = "VolumeList"
	VolumeFullName = VolumePlural + "." + LonghornGroup
)

func (c *ClientGenerator) LonghornVolumeCRDDeploy() error {
	crd := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{
			Name:   VolumeFullName,
			Labels: map[string]string{LonghornLabel: "Volume"},
		},

		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:   LonghornGroup,
			Version: LonghornVersion,
			Scope:   v1beta1.NamespaceScoped,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural:   VolumePlural,
				Kind:     VolumeKind,
				ListKind: VolumeList,
				Singular: Volume,
			},
		},
	}

	_, err := c.Apiclientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err != nil && k8serrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (c *ClientGenerator) LonghornSettingCRDDeploy() error {
	crd := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{
			Name:   SettingFullName,
			Labels: map[string]string{LonghornLabel: "Setting"},
		},

		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:   LonghornGroup,
			Version: LonghornVersion,
			Scope:   v1beta1.NamespaceScoped,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural:   SettingPlural,
				Kind:     SettingKind,
				ListKind: SettingList,
				Singular: Setting,
			},
		},
	}

	_, err := c.Apiclientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err != nil && k8serrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (c *ClientGenerator) LonghornReplicaCRDDeploy() error {
	crd := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{
			Name:   ReplicaFullName,
			Labels: map[string]string{LonghornLabel: "Replica"},
		},

		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:   LonghornGroup,
			Version: LonghornVersion,
			Scope:   v1beta1.NamespaceScoped,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural:   ReplicaPlural,
				Kind:     ReplicaKind,
				ListKind: ReplicaList,
				Singular: Replica,
			},
		},
	}

	_, err := c.Apiclientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err != nil && k8serrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (c *ClientGenerator) LonghornEngineCRDDeploy() error {
	crd := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{
			Name:   EngineFullName,
			Labels: map[string]string{LonghornLabel: "Engine"},
		},

		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:   LonghornGroup,
			Version: LonghornVersion,
			Scope:   v1beta1.NamespaceScoped,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural:   EnginePlural,
				Kind:     EngineKind,
				ListKind: EngineList,
				Singular: Engine,
			},
		},
	}

	_, err := c.Apiclientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err != nil && k8serrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

var (
	lhReplicas int32 = 1
)

func (c *ClientGenerator) LonghornList() (*v1alpha1.InfrastructureList, error) {
	info, err := getConfigMapInfo(c)
	if err != nil {
		return nil, err
	}
	return c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[LanghornNamespace]).List(metav1.ListOptions{})
}

func (c *ClientGenerator) LonghornGet() (*v1alpha1.Infrastructure, error) {
	info, err := getConfigMapInfo(c)
	if err != nil {
		return nil, err
	}
	return c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[LanghornNamespace]).Get(info.Data[LanghornName], metav1.GetOptions{})

}

func (c *ClientGenerator) LonghornDeploy() (*v1alpha1.Infrastructure, error) {
	info, err := getConfigMapInfo(c)
	if err != nil {
		return nil, err
	}

	err = ensureNamespaceExists(c, info.Data[LanghornNamespace])
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, err
	}

	// generate & deploy ingress resources for visit
	err = c.LonghornIngressDeploy()
	if err != nil {
		return nil, err
	}

	lh, err := c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[LanghornNamespace]).Create(&v1alpha1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      info.Data[LanghornName],
			Namespace: info.Data[LanghornNamespace],
		},
		Spec: v1alpha1.InfraSpec{
			DisplayName: info.Data[LanghornName],
			Description: info.Data[LanghornDesc],
			Icon:        info.Data[LanghornIcon],
			InfraKind:   "Longhorn",
			Replicas:    &lhReplicas,
		},
	})
	if err != nil {
		return nil, err
	}

	//watcher, err := c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[LanghornNamespace]).Watch(metav1.ListOptions{})
	//if err != nil {
	//	return nil, err
	//}
	//loop(watcher, lh)

	return lh, nil
}

func (c *ClientGenerator) LonghornDelete() error {
	info, err := getConfigMapInfo(c)
	if err != nil {
		return err
	}
	return c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[LanghornNamespace]).Delete(info.Data[LanghornName], &metav1.DeleteOptions{})
}
