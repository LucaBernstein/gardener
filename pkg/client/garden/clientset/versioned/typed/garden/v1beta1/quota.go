package v1beta1

import (
	v1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	scheme "github.com/gardener/gardener/pkg/client/garden/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// QuotasGetter has a method to return a QuotaInterface.
// A group's client should implement this interface.
type QuotasGetter interface {
	Quotas(namespace string) QuotaInterface
}

// QuotaInterface has methods to work with Quota resources.
type QuotaInterface interface {
	Create(*v1beta1.Quota) (*v1beta1.Quota, error)
	Update(*v1beta1.Quota) (*v1beta1.Quota, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1beta1.Quota, error)
	List(opts v1.ListOptions) (*v1beta1.QuotaList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.Quota, err error)
	QuotaExpansion
}

// quotas implements QuotaInterface
type quotas struct {
	client rest.Interface
	ns     string
}

// newQuotas returns a Quotas
func newQuotas(c *GardenV1beta1Client, namespace string) *quotas {
	return &quotas{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the quota, and returns the corresponding quota object, and an error if there is any.
func (c *quotas) Get(name string, options v1.GetOptions) (result *v1beta1.Quota, err error) {
	result = &v1beta1.Quota{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("quotas").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Quotas that match those selectors.
func (c *quotas) List(opts v1.ListOptions) (result *v1beta1.QuotaList, err error) {
	result = &v1beta1.QuotaList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("quotas").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested quotas.
func (c *quotas) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("quotas").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a quota and creates it.  Returns the server's representation of the quota, and an error, if there is any.
func (c *quotas) Create(quota *v1beta1.Quota) (result *v1beta1.Quota, err error) {
	result = &v1beta1.Quota{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("quotas").
		Body(quota).
		Do().
		Into(result)
	return
}

// Update takes the representation of a quota and updates it. Returns the server's representation of the quota, and an error, if there is any.
func (c *quotas) Update(quota *v1beta1.Quota) (result *v1beta1.Quota, err error) {
	result = &v1beta1.Quota{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("quotas").
		Name(quota.Name).
		Body(quota).
		Do().
		Into(result)
	return
}

// Delete takes name of the quota and deletes it. Returns an error if one occurs.
func (c *quotas) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("quotas").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *quotas) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("quotas").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched quota.
func (c *quotas) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.Quota, err error) {
	result = &v1beta1.Quota{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("quotas").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
