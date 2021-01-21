/*
Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

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
	"context"
	time "time"

	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	versioned "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned"
	internalinterfaces "github.com/gardener/gardener/pkg/client/seedmanagement/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/gardener/gardener/pkg/client/seedmanagement/listers/seedmanagement/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ManagedSeedInformer provides access to a shared informer and lister for
// ManagedSeeds.
type ManagedSeedInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.ManagedSeedLister
}

type managedSeedInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewManagedSeedInformer constructs a new informer for ManagedSeed type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewManagedSeedInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredManagedSeedInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredManagedSeedInformer constructs a new informer for ManagedSeed type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredManagedSeedInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SeedmanagementV1alpha1().ManagedSeeds(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SeedmanagementV1alpha1().ManagedSeeds(namespace).Watch(context.TODO(), options)
			},
		},
		&seedmanagementv1alpha1.ManagedSeed{},
		resyncPeriod,
		indexers,
	)
}

func (f *managedSeedInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredManagedSeedInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *managedSeedInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&seedmanagementv1alpha1.ManagedSeed{}, f.defaultInformer)
}

func (f *managedSeedInformer) Lister() v1alpha1.ManagedSeedLister {
	return v1alpha1.NewManagedSeedLister(f.Informer().GetIndexer())
}
