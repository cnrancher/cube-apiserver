/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	time "time"

	cube_v1alpha1 "github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"
	versioned "github.com/cnrancher/cube-apiserver/k8s/pkg/client/clientset/versioned"
	internalinterfaces "github.com/cnrancher/cube-apiserver/k8s/pkg/client/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/cnrancher/cube-apiserver/k8s/pkg/client/listers/cube/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// GroupMemberInformer provides access to a shared informer and lister for
// GroupMembers.
type GroupMemberInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.GroupMemberLister
}

type groupMemberInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewGroupMemberInformer constructs a new informer for GroupMember type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewGroupMemberInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredGroupMemberInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredGroupMemberInformer constructs a new informer for GroupMember type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredGroupMemberInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CubeV1alpha1().GroupMembers(namespace).List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CubeV1alpha1().GroupMembers(namespace).Watch(options)
			},
		},
		&cube_v1alpha1.GroupMember{},
		resyncPeriod,
		indexers,
	)
}

func (f *groupMemberInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredGroupMemberInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *groupMemberInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&cube_v1alpha1.GroupMember{}, f.defaultInformer)
}

func (f *groupMemberInformer) Lister() v1alpha1.GroupMemberLister {
	return v1alpha1.NewGroupMemberLister(f.Informer().GetIndexer())
}
