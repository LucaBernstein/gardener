// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers_test

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/bootstrappers"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
)

var _ = Describe("GardenState", func() {
	var (
		ctx         = context.Background()
		garden      *operatorv1alpha1.Garden
		extension   *operatorv1alpha1.Extension
		stateSecret *corev1.Secret
		fakeClient  client.Client
	)

	BeforeEach(func() {
		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{Name: "garden"},
			Status: operatorv1alpha1.GardenStatus{
				LastOperation: &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeReconcile, State: gardencorev1beta1.LastOperationStateSucceeded},
			},
		}
		extension = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "provider-local",
				Labels:          map[string]string{"test": "label"},
				Annotations:     map[string]string{"test": "annotation"},
				UID:             "old-uid",
				ResourceVersion: "123",
				Finalizers:      []string{operatorv1alpha1.FinalizerName},
			},
			Spec: operatorv1alpha1.ExtensionSpec{
				Resources: []gardencorev1beta1.ControllerResource{{Kind: "BackupBucket", Type: "local"}},
				Deployment: &operatorv1alpha1.Deployment{
					ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
						RuntimeClusterValues: &apiextensionsv1.JSON{Raw: []byte(`{"replicas":1}`)},
					},
				},
			},
			Status: operatorv1alpha1.ExtensionStatus{
				ObservedGeneration: 7,
				Conditions:         []gardencorev1beta1.Condition{{Type: operatorv1alpha1.ExtensionInstalled, Status: gardencorev1beta1.ConditionTrue}},
			},
		}
		var err error
		stateSecret, err = EncodeGardenStateSecret(garden, nil, []operatorv1alpha1.Extension{*extension}, nil, nil)
		Expect(err).NotTo(HaveOccurred())
		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).
			WithStatusSubresource(garden, extension).WithObjects(stateSecret).Build()
	})

	It("persists operator definitions separately from extension controller state", func() {
		extensionStates := []gardencorev1beta1.ExtensionResourceState{{Kind: "BackupEntry", Name: ptr.To("garden"), State: &runtime.RawExtension{Raw: []byte(`{"state":"saved"}`)}}}
		secret, err := EncodeGardenStateSecret(garden, nil, []operatorv1alpha1.Extension{*extension}, extensionStates, nil)
		Expect(err).NotTo(HaveOccurred())
		state, err := DecodeGardenStateSecret(secret)
		Expect(err).NotTo(HaveOccurred())
		Expect(state.OperatorExtensions).To(Equal([]operatorv1alpha1.GardenStateExtension{{
			Name: extension.Name, Labels: extension.Labels, Annotations: extension.Annotations, Spec: extension.Spec,
		}}))
		Expect(state.Extensions).To(Equal(extensionStates))
		raw := secret.Data["state"]
		Expect(string(raw)).NotTo(ContainSubstring("old-uid"))
		Expect(string(raw)).NotTo(ContainSubstring("observedGeneration"))
		Expect(string(raw)).NotTo(ContainSubstring("finalizers"))
	})

	It("does not persist deleting operator Extensions", func() {
		extension.DeletionTimestamp = ptr.To(metav1.Now())
		secret, err := EncodeGardenStateSecret(garden, nil, []operatorv1alpha1.Extension{*extension}, nil, nil)
		Expect(err).NotTo(HaveOccurred())
		state, err := DecodeGardenStateSecret(secret)
		Expect(err).NotTo(HaveOccurred())
		Expect(state.OperatorExtensions).To(BeEmpty())
	})

	It("restores operator definitions before the Garden without stale metadata or status", func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).
			WithStatusSubresource(garden, extension).WithObjects(stateSecret).
			WithInterceptorFuncs(interceptor.Funcs{Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if _, ok := obj.(*operatorv1alpha1.Garden); ok {
					Expect(c.Get(ctx, client.ObjectKey{Name: extension.Name}, &operatorv1alpha1.Extension{})).To(Succeed())
				}
				return c.Create(ctx, obj, opts...)
			}}).Build()
		Expect(RestoreGardenState(ctx, logr.Discard(), fakeClient, v1beta1constants.GardenNamespace)).To(Succeed())

		restored := &operatorv1alpha1.Extension{}
		Expect(fakeClient.Get(ctx, client.ObjectKey{Name: extension.Name}, restored)).To(Succeed())
		Expect(restored.Labels).To(Equal(extension.Labels))
		Expect(restored.Annotations).To(Equal(extension.Annotations))
		Expect(restored.Spec).To(Equal(extension.Spec))
		Expect(restored.UID).NotTo(Equal(extension.UID))
		Expect(restored.Finalizers).To(BeEmpty())
		Expect(restored.Status).To(Equal(operatorv1alpha1.ExtensionStatus{}))
		Expect(fakeClient.Get(ctx, client.ObjectKey{Name: garden.Name}, garden)).To(Succeed())
		Expect(garden.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeRestore))
		Expect(garden.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStatePending))
	})

	It("does not overwrite existing operator Extensions", func() {
		existing := extension.DeepCopy()
		existing.ResourceVersion = ""
		existing.Spec.Resources[0].Type = "other"
		Expect(fakeClient.Create(ctx, existing)).To(Succeed())
		before := existing.DeepCopy()
		Expect(RestoreGardenState(ctx, logr.Discard(), fakeClient, v1beta1constants.GardenNamespace)).To(Succeed())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(existing), existing)).To(Succeed())
		Expect(existing).To(Equal(before))
	})

	It("can retry a failed operator Extension restore before creating the Garden", func() {
		fail := true
		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).
			WithStatusSubresource(garden, extension).WithObjects(stateSecret).
			WithInterceptorFuncs(interceptor.Funcs{Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if _, ok := obj.(*operatorv1alpha1.Extension); ok && fail {
					return fmt.Errorf("extension create failed")
				}
				return c.Create(ctx, obj, opts...)
			}}).Build()
		Expect(RestoreGardenState(ctx, logr.Discard(), fakeClient, v1beta1constants.GardenNamespace)).To(MatchError(ContainSubstring(`failed restoring operator Extension "provider-local"`)))
		gardens := &operatorv1alpha1.GardenList{}
		Expect(fakeClient.List(ctx, gardens)).To(Succeed())
		Expect(gardens.Items).To(BeEmpty())
		fail = false
		Expect(RestoreGardenState(ctx, logr.Discard(), fakeClient, v1beta1constants.GardenNamespace)).To(Succeed())
	})

	It("restores GardenStates without operator definitions", func() {
		secret, err := EncodeGardenStateSecret(garden, nil, nil, nil, nil)
		Expect(err).NotTo(HaveOccurred())
		raw := secret.Data["state"]
		Expect(string(raw)).NotTo(ContainSubstring("operatorExtensions"))
		secret.ResourceVersion = stateSecret.ResourceVersion
		Expect(fakeClient.Update(ctx, secret)).To(Succeed())
		Expect(RestoreGardenState(ctx, logr.Discard(), fakeClient, v1beta1constants.GardenNamespace)).To(Succeed())
	})

	It("skips restoring operator definitions when a Garden already exists", func() {
		Expect(fakeClient.Create(ctx, garden)).To(Succeed())
		Expect(RestoreGardenState(ctx, logr.Discard(), fakeClient, v1beta1constants.GardenNamespace)).To(Succeed())
		extensions := &operatorv1alpha1.ExtensionList{}
		Expect(fakeClient.List(ctx, extensions)).To(Succeed())
		Expect(extensions.Items).To(BeEmpty())
	})

})
