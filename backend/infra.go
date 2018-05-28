package backend

import (
	"reflect"

	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	InfraPlural   = "infrastructures"
	InfraGroup    = "cube.rancher.io"
	InfraVersion  = "v1alpha1"
	InfraFullName = InfraPlural + "." + InfraGroup
)

var (
	dashboardNs         = "kube-system"
	dashboardName       = "kubernetes-dashboard"
	replicas      int32 = 1
	configMap     *v1.ConfigMap
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

func (c *ClientGenerator) DashBoardList() (*v1alpha1.InfrastructureList, error) {
	info, err := getConfigMapInfo(c)
	if err != nil {
		return nil, err
	}
	return c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[Dashboard_Namespace]).List(metav1.ListOptions{})
}

func (c *ClientGenerator) DashBoardGet() (*v1alpha1.Infrastructure, error) {
	info, err := getConfigMapInfo(c)
	if err != nil {
		return nil, err
	}
	return c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[Dashboard_Namespace]).Get(info.Data[Dashboard_Name], metav1.GetOptions{})

}

func (c *ClientGenerator) DashBoardDeploy() (*v1alpha1.Infrastructure, error) {
	info, err := getConfigMapInfo(c)
	if err != nil {
		return nil, err
	}
	db, err := c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[Dashboard_Namespace]).Create(&v1alpha1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      info.Data[Dashboard_Name],
			Namespace: info.Data[Dashboard_Namespace],
		},
		Spec: v1alpha1.InfraSpec{
			DisplayName: info.Data[Dashboard_Name],
			Description: info.Data[Dashboard_Desc],
			Icon:        info.Data[Dashboard_Icon],
			InfraKind:   "Dashboard",
			Replicas:    &replicas,
		},
	})
	if err != nil {
		return nil, err
	}

	watcher, err := c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[Dashboard_Namespace]).Watch(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	loop(watcher, db)

	return db, nil
}

func loop(watcher watch.Interface, db *v1alpha1.Infrastructure) {
Loop:
	for {
		select {
		case data := <-watcher.ResultChan():
			object := data.Object.(*v1alpha1.Infrastructure)
			if data.Type == "MODIFIED" && object.Name == db.Name {
				break Loop
			}
		case <-time.After(time.Duration(10) * time.Second):
			break Loop
		}
	}
	return
}

func (c *ClientGenerator) DashBoardDelete() error {
	info, err := getConfigMapInfo(c)
	if err != nil {
		return err
	}
	return c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[Dashboard_Namespace]).Delete(info.Data[Dashboard_Name], &metav1.DeleteOptions{})
}

func getConfigMapInfo(c *ClientGenerator) (*v1.ConfigMap, error) {
	if configMap == nil {
		var err error
		configMap, err = c.ConfigMapGet(ConfigMapNs, ConfigMapName)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return c.ConfigMapDeploy()
			}
			return nil, err
		}
	}
	return configMap, nil

}
