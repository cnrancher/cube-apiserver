package backend

import (
	"github.com/cnrancher/cube-apiserver/util"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

type ClusterResource struct {
	Capacity    v1.ResourceList `json:"capacity,omitempty"`
	Allocatable v1.ResourceList `json:"allocatable,omitempty"`
	Requested   v1.ResourceList `json:"requested,omitempty"`
	Limits      v1.ResourceList `json:"limits,omitempty"`
}

func (c *ClientGenerator) ClusterResources() (*ClusterResource, error) {
	nodes, err := c.Clientset.CoreV1().Nodes().List(util.ListEverything)
	if err != nil {
		return nil, err
	}
	clusterResources := make([]ClusterResource, len(nodes.Items))

	for _, node := range nodes.Items {
		pods, err := c.getNodePods(node)
		if err != nil {
			continue
		}
		requests, limits := c.aggregateRequestAndLimitsForNode(pods)

		clusterResource := &ClusterResource{
			Requested: v1.ResourceList{},
			Limits:    v1.ResourceList{},
		}

		clusterResource.Capacity = node.Status.Capacity
		clusterResource.Allocatable = node.Status.Allocatable

		for name, quantity := range requests {
			clusterResource.Requested[name] = quantity
		}
		for name, quantity := range limits {
			clusterResource.Limits[name] = quantity
		}
		clusterResources = append(clusterResources, *clusterResource)
	}

	// capacity keys
	pods, mem, cpu := resource.Quantity{}, resource.Quantity{}, resource.Quantity{}
	// allocatable keys
	apods, amem, acpu := resource.Quantity{}, resource.Quantity{}, resource.Quantity{}
	// requested keys
	rpods, rmem, rcpu := resource.Quantity{}, resource.Quantity{}, resource.Quantity{}
	// limited keys
	lpods, lmem, lcpu := resource.Quantity{}, resource.Quantity{}, resource.Quantity{}

	for _, clusterResource := range clusterResources {
		capacity := clusterResource.Capacity
		if capacity != nil {
			pods.Add(*capacity.Pods())
			mem.Add(*capacity.Memory())
			cpu.Add(*capacity.Cpu())
		}
		allocatable := clusterResource.Allocatable
		if allocatable != nil {
			apods.Add(*allocatable.Pods())
			amem.Add(*allocatable.Memory())
			acpu.Add(*allocatable.Cpu())
		}
		requested := clusterResource.Requested
		if requested != nil {
			rpods.Add(*requested.Pods())
			rmem.Add(*requested.Memory())
			rcpu.Add(*requested.Cpu())
		}
		limits := clusterResource.Limits
		if limits != nil {
			lpods.Add(*limits.Pods())
			lmem.Add(*limits.Memory())
			lcpu.Add(*limits.Cpu())
		}
	}

	return &ClusterResource{
		Capacity:    v1.ResourceList{v1.ResourcePods: pods, v1.ResourceMemory: mem, v1.ResourceCPU: cpu},
		Allocatable: v1.ResourceList{v1.ResourcePods: apods, v1.ResourceMemory: amem, v1.ResourceCPU: acpu},
		Requested:   v1.ResourceList{v1.ResourcePods: rpods, v1.ResourceMemory: rmem, v1.ResourceCPU: rcpu},
		Limits:      v1.ResourceList{v1.ResourcePods: lpods, v1.ResourceMemory: lmem, v1.ResourceCPU: lcpu},
	}, nil
}

func (c *ClientGenerator) ClusterComponentStatuses() (*v1.ComponentStatusList, error) {
	componentStatuses, err := c.Clientset.CoreV1().ComponentStatuses().List(util.ListEverything)
	return componentStatuses, err
}

func (c *ClientGenerator) ClusterComponentStatus(component string) (*v1.ComponentStatus, error) {
	componentStatus, err := c.Clientset.CoreV1().ComponentStatuses().Get(component, util.GetOptions)
	return componentStatus, err
}

func (c *ClientGenerator) getNodePods(node v1.Node) (*v1.PodList, error) {
	fieldSelector, err := fields.ParseSelector("spec.nodeName=" + node.Name +
		",status.phase!=" + string(v1.PodSucceeded) +
		",status.phase!=" + string(v1.PodFailed))

	if err != nil {
		return nil, err
	}

	pods, err := c.Clientset.CoreV1().Pods(v1.NamespaceAll).List(metaV1.ListOptions{
		FieldSelector: fieldSelector.String(),
	})

	if err != nil {
		return nil, err
	}

	matchPods := make([]v1.Pod, 0)

	for _, pod := range pods.Items {
		if pod.Spec.NodeName == "" || pod.DeletionTimestamp != nil {
			continue
		}
		// kubectl uses this cache to filter out the pods
		if pod.Status.Phase == "Succeeded" || pod.Status.Phase == "Failed" {
			continue
		}
		matchPods = append(matchPods, pod)
	}

	pods.Items = matchPods

	return pods, nil
}

func (c *ClientGenerator) aggregateRequestAndLimitsForNode(pods *v1.PodList) (map[v1.ResourceName]resource.Quantity, map[v1.ResourceName]resource.Quantity) {
	requests, limits := map[v1.ResourceName]resource.Quantity{}, map[v1.ResourceName]resource.Quantity{}
	podsData := make(map[string]map[string]map[v1.ResourceName]resource.Quantity)
	if pods != nil {
		//podName -> req/limit -> data
		for _, pod := range pods.Items {
			podsData[pod.Name] = make(map[string]map[v1.ResourceName]resource.Quantity)
			requests, limits := getPodData(pod)
			podsData[pod.Name]["requests"] = requests
			podsData[pod.Name]["limits"] = limits
		}
		requests[v1.ResourcePods] = *resource.NewQuantity(int64(len(pods.Items)), resource.DecimalSI)
	}
	for _, podData := range podsData {
		podRequests, podLimits := podData["requests"], podData["limits"]
		addMap(podRequests, requests)
		addMap(podLimits, limits)
	}
	return requests, limits
}

func getPodData(pod v1.Pod) (map[v1.ResourceName]resource.Quantity, map[v1.ResourceName]resource.Quantity) {
	requests, limits := map[v1.ResourceName]resource.Quantity{}, map[v1.ResourceName]resource.Quantity{}
	for _, container := range pod.Spec.Containers {
		addMap(container.Resources.Requests, requests)
		addMap(container.Resources.Limits, limits)
	}

	for _, container := range pod.Spec.InitContainers {
		addMapForInit(container.Resources.Requests, requests)
		addMapForInit(container.Resources.Limits, limits)
	}
	return requests, limits
}

func addMap(data1 map[v1.ResourceName]resource.Quantity, data2 map[v1.ResourceName]resource.Quantity) {
	for name, quantity := range data1 {
		if value, ok := data2[name]; !ok {
			data2[name] = *quantity.Copy()
		} else {
			value.Add(quantity)
			data2[name] = value
		}
	}
}

func addMapForInit(data1 map[v1.ResourceName]resource.Quantity, data2 map[v1.ResourceName]resource.Quantity) {
	for name, quantity := range data1 {
		value, ok := data2[name]
		if !ok {
			data2[name] = *quantity.Copy()
			continue
		}
		if quantity.Cmp(value) > 0 {
			data2[name] = *quantity.Copy()
		}
	}
}
