// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/gardener/gardener/pkg/client/extensions/clientset/versioned/typed/extensions/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeExtensionsV1alpha1 struct {
	*testing.Fake
}

func (c *FakeExtensionsV1alpha1) Clusters() v1alpha1.ClusterInterface {
	return &FakeClusters{c}
}

func (c *FakeExtensionsV1alpha1) ControlPlanes(namespace string) v1alpha1.ControlPlaneInterface {
	return &FakeControlPlanes{c, namespace}
}

func (c *FakeExtensionsV1alpha1) Extensions(namespace string) v1alpha1.ExtensionInterface {
	return &FakeExtensions{c, namespace}
}

func (c *FakeExtensionsV1alpha1) Infrastructures(namespace string) v1alpha1.InfrastructureInterface {
	return &FakeInfrastructures{c, namespace}
}

func (c *FakeExtensionsV1alpha1) OperatingSystemConfigs(namespace string) v1alpha1.OperatingSystemConfigInterface {
	return &FakeOperatingSystemConfigs{c, namespace}
}

func (c *FakeExtensionsV1alpha1) Workers(namespace string) v1alpha1.WorkerInterface {
	return &FakeWorkers{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeExtensionsV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
