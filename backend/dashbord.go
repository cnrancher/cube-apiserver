package backend

import (
	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	dbReplicas int32 = 1
)

func (c *ClientGenerator) DashBoardList() (*v1alpha1.InfrastructureList, error) {
	info, err := getConfigMapInfo(c)
	if err != nil {
		return nil, err
	}
	return c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[DashboardNamespace]).List(metav1.ListOptions{})
}

func (c *ClientGenerator) DashBoardGet() (*v1alpha1.Infrastructure, error) {
	info, err := getConfigMapInfo(c)
	if err != nil {
		return nil, err
	}
	return c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[DashboardNamespace]).Get(info.Data[DashboardName], metav1.GetOptions{})

}

func (c *ClientGenerator) DashBoardDeploy() (*v1alpha1.Infrastructure, error) {
	info, err := getConfigMapInfo(c)
	if err != nil {
		return nil, err
	}

	err = ensureNamespaceExists(c, info.Data[DashboardNamespace])
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, err
	}

	// generate & deploy ingress resources for visit
	err = c.DashboardIngressDeploy()
	if err != nil {
		return nil, err
	}

	db, err := c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[DashboardNamespace]).Create(&v1alpha1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      info.Data[DashboardName],
			Namespace: info.Data[DashboardNamespace],
		},
		Spec: v1alpha1.InfraSpec{
			DisplayName: info.Data[DashboardName],
			Description: info.Data[DashboardDesc],
			Icon:        info.Data[DashboardIcon],
			InfraKind:   "Dashboard",
			Replicas:    &dbReplicas,
		},
	})
	if err != nil {
		return nil, err
	}

	//watcher, err := c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[DashboardNamespace]).Watch(metav1.ListOptions{})
	//if err != nil {
	//	return nil, err
	//}
	//loop(watcher, db)

	return db, nil
}

func (c *ClientGenerator) DashBoardDelete() error {
	info, err := getConfigMapInfo(c)
	if err != nil {
		return err
	}
	return c.Infraclientset.CubeV1alpha1().Infrastructures(info.Data[DashboardNamespace]).Delete(info.Data[DashboardName], &metav1.DeleteOptions{})
}
