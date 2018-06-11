package backend

import (
	"reflect"

	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/cnrancher/cube-apiserver/controller"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	InfraPlural   = "infrastructures"
	InfraGroup    = "cube.rancher.io"
	InfraVersion  = "v1alpha1"
	InfraFullName = InfraPlural + "." + InfraGroup
)

var (
	replicas int32 = 1
	status   map[string]string
	state    map[string]string
)

func (c *ClientGenerator) InfrastructureCRDDeploy() error {
	crd := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{Name: "infrastructures.cube.rancher.io"},
		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:   "cube.rancher.io",
			Version: "v1alpha1",
			Scope:   v1beta1.NamespaceScoped,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural: "infrastructures",
				Kind:   reflect.TypeOf(v1alpha1.Infrastructure{}).Name(),
			},
		},
	}

	_, err := c.Apiclientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err == nil || (err != nil && apierrors.IsAlreadyExists(err)) {
		return nil
	}

	return err
}

func (c *ClientGenerator) BaseInfoGet() ([]map[string]string, error) {
	info, err := getConfigMapInfo(c)
	if err != nil {
		return nil, err
	}

	c.statusGet()
	if status == nil {
		return nil, errors.New("failed to syncService infrastructures status")
	}

	return []map[string]string{
		{
			"name":   "k8s dashboard",
			"kind":   info.Data[Infrastructures.dashboard.name],
			"icon":   info.Data[Infrastructures.dashboard.icon],
			"desc":   info.Data[Infrastructures.dashboard.desc],
			"status": status[ info.Data[Infrastructures.dashboard.name] ],
			"state":  state[ info.Data[Infrastructures.dashboard.name] ],
			"url":    "/infrastructures",
		},
		{
			"name":   "Longhorn",
			"kind":   info.Data[Infrastructures.longhorn.name],
			"icon":   info.Data[Infrastructures.longhorn.icon],
			"desc":   info.Data[Infrastructures.longhorn.desc],
			"status": status[ info.Data[Infrastructures.longhorn.name] ],
			"state":  state[ info.Data[Infrastructures.longhorn.name] ],
			"url":    "/infrastructures",
		},
		{
			"name":   "RancherVM",
			"kind":   info.Data[Infrastructures.rancherVM.name],
			"icon":   info.Data[Infrastructures.rancherVM.icon],
			"desc":   info.Data[Infrastructures.rancherVM.desc],
			"status": status[ info.Data[Infrastructures.rancherVM.name] ],
			"state":  state[ info.Data[Infrastructures.rancherVM.name]],
			"url":    "/infrastructures",
		},
	}, nil
}

func (c *ClientGenerator) statusGet() error {
	status = make(map[string]string)
	state = make(map[string]string)
	db, err := c.InfrastructureGet(controller.DashboardName, true)
	if err != nil || db == nil {
		checkStatus(nil, controller.DashboardName)
	} else {
		checkStatus(db, controller.DashboardName)
	}

	lh, err := c.InfrastructureGet(controller.LonghornName, true)
	if err != nil || lh == nil {
		checkStatus(nil, controller.LonghornName)
	} else {
		checkStatus(lh, controller.LonghornName)
	}

	vm, err := c.InfrastructureGet(controller.RancherVMName, true)
	if err != nil || vm == nil {
		checkStatus(nil, controller.RancherVMName)
	} else {
		checkStatus(vm, controller.RancherVMName)
	}

	return nil
}

func checkStatus(item *v1alpha1.Infrastructure, name string) {
	if item != nil {
		status[name] = "True"
		state[name] = item.Status.State
	} else {
		status[name] = "False"
		state[name] = ""
	}
}

func (c *ClientGenerator) getInfra(kind string) *infrastructure {
	switch kind {
	case controller.DashboardName:
		return Infrastructures.dashboard
	case controller.LonghornName:
		return Infrastructures.longhorn
	case controller.RancherVMName:
		return Infrastructures.rancherVM
	default:
		return nil
	}
}

func (c *ClientGenerator) InfrastructureList() (*v1alpha1.InfrastructureList, error) {
	return c.Infraclientset.CubeV1alpha1().Infrastructures("").List(metav1.ListOptions{})
}

func (c *ClientGenerator) InfrastructureGet(kind string, isBaseInfo bool) (*v1alpha1.Infrastructure, error) {
	infra := c.getInfra(kind)
	info, err := getConfigMapInfo(c)
	if err != nil {
		return nil, err
	}

	if !isBaseInfo {
		err = c.ServiceGet(info.Data[infra.namespace], info.Data[infra.name])
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get infrastructure service:%s", info.Data[infra.name])
		}
	}

	return c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[infra.namespace]).Get(info.Data[infra.name], metav1.GetOptions{})
}

func (c *ClientGenerator) InfrastructureDeploy(kind string) (*v1alpha1.Infrastructure, error) {
	infra := c.getInfra(kind)
	info, err := getConfigMapInfo(c)
	if err != nil {
		return nil, err
	}
	err = ensureNamespaceExists(c, info.Data[infra.namespace])
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, err
	}
	db, err := c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[infra.namespace]).Create(&v1alpha1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      info.Data[infra.name],
			Namespace: info.Data[infra.namespace],
		},
		Spec: v1alpha1.InfraSpec{
			DisplayName: info.Data[infra.name],
			Description: info.Data[infra.desc],
			Icon:        info.Data[infra.icon],
			InfraKind:   info.Data[infra.kind],
			Replicas:    &replicas,
			Images:      *c.CubeImages,
		},
	})
	if err != nil {
		return nil, err
	}

	err = c.ServiceGet(info.Data[infra.namespace], info.Data[infra.name])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get infrastructure service:%s", info.Data[infra.name])
	}

	return db, nil
}

func (c *ClientGenerator) InfrastructureDelete(kind string) error {
	infra := c.getInfra(kind)
	info, err := getConfigMapInfo(c)
	if err != nil {
		return err
	}
	return c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[infra.namespace]).Delete(info.Data[infra.name], &metav1.DeleteOptions{})
}
