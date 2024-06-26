// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeGardenlets implements GardenletInterface
type FakeGardenlets struct {
	Fake *FakeSeedmanagementV1alpha1
	ns   string
}

var gardenletsResource = v1alpha1.SchemeGroupVersion.WithResource("gardenlets")

var gardenletsKind = v1alpha1.SchemeGroupVersion.WithKind("Gardenlet")

// Get takes name of the gardenlet, and returns the corresponding gardenlet object, and an error if there is any.
func (c *FakeGardenlets) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.Gardenlet, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(gardenletsResource, c.ns, name), &v1alpha1.Gardenlet{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Gardenlet), err
}

// List takes label and field selectors, and returns the list of Gardenlets that match those selectors.
func (c *FakeGardenlets) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.GardenletList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(gardenletsResource, gardenletsKind, c.ns, opts), &v1alpha1.GardenletList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.GardenletList{ListMeta: obj.(*v1alpha1.GardenletList).ListMeta}
	for _, item := range obj.(*v1alpha1.GardenletList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested gardenlets.
func (c *FakeGardenlets) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(gardenletsResource, c.ns, opts))

}

// Create takes the representation of a gardenlet and creates it.  Returns the server's representation of the gardenlet, and an error, if there is any.
func (c *FakeGardenlets) Create(ctx context.Context, gardenlet *v1alpha1.Gardenlet, opts v1.CreateOptions) (result *v1alpha1.Gardenlet, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(gardenletsResource, c.ns, gardenlet), &v1alpha1.Gardenlet{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Gardenlet), err
}

// Update takes the representation of a gardenlet and updates it. Returns the server's representation of the gardenlet, and an error, if there is any.
func (c *FakeGardenlets) Update(ctx context.Context, gardenlet *v1alpha1.Gardenlet, opts v1.UpdateOptions) (result *v1alpha1.Gardenlet, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(gardenletsResource, c.ns, gardenlet), &v1alpha1.Gardenlet{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Gardenlet), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeGardenlets) UpdateStatus(ctx context.Context, gardenlet *v1alpha1.Gardenlet, opts v1.UpdateOptions) (*v1alpha1.Gardenlet, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(gardenletsResource, "status", c.ns, gardenlet), &v1alpha1.Gardenlet{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Gardenlet), err
}

// Delete takes name of the gardenlet and deletes it. Returns an error if one occurs.
func (c *FakeGardenlets) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(gardenletsResource, c.ns, name, opts), &v1alpha1.Gardenlet{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeGardenlets) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(gardenletsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.GardenletList{})
	return err
}

// Patch applies the patch and returns the patched gardenlet.
func (c *FakeGardenlets) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.Gardenlet, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(gardenletsResource, c.ns, name, pt, data, subresources...), &v1alpha1.Gardenlet{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Gardenlet), err
}
