// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by client-gen. DO NOT EDIT.

package v1beta1

import (
	context "context"

	corev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	scheme "github.com/gardener/gardener/pkg/client/core/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// NamespacedCloudProfilesGetter has a method to return a NamespacedCloudProfileInterface.
// A group's client should implement this interface.
type NamespacedCloudProfilesGetter interface {
	NamespacedCloudProfiles(namespace string) NamespacedCloudProfileInterface
}

// NamespacedCloudProfileInterface has methods to work with NamespacedCloudProfile resources.
type NamespacedCloudProfileInterface interface {
	Create(ctx context.Context, namespacedCloudProfile *corev1beta1.NamespacedCloudProfile, opts v1.CreateOptions) (*corev1beta1.NamespacedCloudProfile, error)
	Update(ctx context.Context, namespacedCloudProfile *corev1beta1.NamespacedCloudProfile, opts v1.UpdateOptions) (*corev1beta1.NamespacedCloudProfile, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, namespacedCloudProfile *corev1beta1.NamespacedCloudProfile, opts v1.UpdateOptions) (*corev1beta1.NamespacedCloudProfile, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*corev1beta1.NamespacedCloudProfile, error)
	List(ctx context.Context, opts v1.ListOptions) (*corev1beta1.NamespacedCloudProfileList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *corev1beta1.NamespacedCloudProfile, err error)
	NamespacedCloudProfileExpansion
}

// namespacedCloudProfiles implements NamespacedCloudProfileInterface
type namespacedCloudProfiles struct {
	*gentype.ClientWithList[*corev1beta1.NamespacedCloudProfile, *corev1beta1.NamespacedCloudProfileList]
}

// newNamespacedCloudProfiles returns a NamespacedCloudProfiles
func newNamespacedCloudProfiles(c *CoreV1beta1Client, namespace string) *namespacedCloudProfiles {
	return &namespacedCloudProfiles{
		gentype.NewClientWithList[*corev1beta1.NamespacedCloudProfile, *corev1beta1.NamespacedCloudProfileList](
			"namespacedcloudprofiles",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *corev1beta1.NamespacedCloudProfile { return &corev1beta1.NamespacedCloudProfile{} },
			func() *corev1beta1.NamespacedCloudProfileList { return &corev1beta1.NamespacedCloudProfileList{} },
		),
	}
}
