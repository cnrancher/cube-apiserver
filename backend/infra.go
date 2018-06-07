package backend

import (
	"reflect"

	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/cnrancher/cube-apiserver/controller"
	"github.com/pkg/errors"
)

const (
	InfraPlural   = "infrastructures"
	InfraGroup    = "cube.rancher.io"
	InfraVersion  = "v1alpha1"
	InfraFullName = InfraPlural + "." + InfraGroup
)

var status map[string]string
var state map[string]string

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
			"kind":   info.Data[dashboardName],
			"icon":   info.Data[dashboardIcon],
			"desc":   info.Data[dashboardDesc],
			"status": status[info.Data[dashboardName]],
			"state":  state[info.Data[dashboardName]],
			"url":    "/dashboards",
		},
		{
			"name":   "Longhorn",
			"kind":   info.Data[langhornName],
			"icon":   info.Data[langhornIcon],
			"desc":   info.Data[langhornDesc],
			"status": status[info.Data[langhornName]],
			"state":  state[info.Data[langhornName]],
			"url":    "/longhorns",
		},
		{
			"name":   "RancherVM",
			"kind":   info.Data[rancherVMName],
			"icon":   info.Data[rancherVMIcon],
			"desc":   info.Data[rancherVMDesc],
			"status": status[info.Data[rancherVMName]],
			"state":  state[info.Data[rancherVMName]],
			"url":    "",
		},
	}, nil
}

func (c *ClientGenerator) statusGet() error {
	status = make(map[string]string)
	state = make(map[string]string)
	dblist, err := c.DashBoardList()
	if err != nil {
		return errors.Wrap(err, "failed to read dashboard info")
	}
	checkStatus(dblist, controller.DashboardName)

	lhlist, err := c.LonghornList()
	if err != nil {
		return errors.Wrap(err, "failed to read longhorn info")
	}
	checkStatus(lhlist, controller.LonghornName)

	return nil
}

func checkStatus(list *v1alpha1.InfrastructureList, name string) {
	if len(list.Items) > 0 {
		status[name] = "True"
		checkHealthy(list.Items[0], name)
	} else {
		status[name] = "False"
	}
}

func checkHealthy(infra v1alpha1.Infrastructure, name string) {
	state[name] = infra.Status.State
}

func (c *ClientGenerator) urlGet() error {
	status = make(map[string]string)
	state = make(map[string]string)
	dblist, err := c.DashBoardList()
	if err != nil {
		return errors.Wrap(err, "failed to read dashboard info")
	}
	checkStatus(dblist, controller.DashboardName)

	lhlist, err := c.LonghornList()
	if err != nil {
		return errors.Wrap(err, "failed to read longhorn info")
	}
	checkStatus(lhlist, controller.LonghornName)

	return nil
}
