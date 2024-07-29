// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacedcloudprofile_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	namespacedcloudprofilecontroller "github.com/gardener/gardener/pkg/controllermanager/controller/namespacedcloudprofile"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("NamespacedCloudProfile Reconciler", func() {
	const finalizerName = "gardener"

	var (
		ctx        context.Context
		ctrl       *gomock.Controller
		c          *mockclient.MockClient
		reconciler reconcile.Reconciler

		fakeErr error

		namespaceName              string
		cloudProfileName           string
		namespacedCloudProfileName string

		cloudProfile           *gardencorev1beta1.CloudProfile
		namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile
	)

	BeforeEach(func() {
		ctx = context.Background()

		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		fakeErr = errors.New("fake err")
		reconciler = &namespacedcloudprofilecontroller.Reconciler{Client: c, Recorder: &record.FakeRecorder{}}

		namespaceName = "test-namespace"
		cloudProfileName = "test-cloudprofile"
		namespacedCloudProfileName = "test-namespacedcloudprofile"

		cloudProfile = &gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: cloudProfileName,
			},
			Spec: gardencorev1beta1.CloudProfileSpec{
				Kubernetes: gardencorev1beta1.KubernetesSettings{
					Versions: []gardencorev1beta1.ExpirableVersion{
						{
							Version: "1.0.0",
						},
					},
				},
			},
		}
		namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:            namespacedCloudProfileName,
				Namespace:       namespaceName,
				ResourceVersion: "42",
			},
			Spec: gardencorev1beta1.NamespacedCloudProfileSpec{
				Parent: gardencorev1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileName,
				},
				Kubernetes: &gardencorev1beta1.KubernetesSettings{
					Versions: []gardencorev1beta1.ExpirableVersion{
						{
							Version: "1.2.3",
						},
					},
				},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should return nil because object not found", func() {
		c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return err because object reading failed", func() {
		c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).Return(fakeErr)

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).To(MatchError(fakeErr))
	})

	Context("merge Kubernetes versions", func() {
		It("should ignore versions specified only in NamespacedCloudProfile", func() {
			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.NamespacedCloudProfile, _ ...client.GetOption) error {
				namespacedCloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...client.GetOption) error {
				cloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
				Expect(patch.Data(o)).To(BeEquivalentTo(`{"status":{"cloudProfileSpec":{"kubernetes":{"versions":[{"version":"1.0.0"}]}}}}`))
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should merge Kubernetes versions correctly", func() {
			newExpiryDate := metav1.Now()
			namespacedCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
				{Version: "1.0.0", ExpirationDate: &newExpiryDate},
			}

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.NamespacedCloudProfile, _ ...client.GetOption) error {
				namespacedCloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...client.GetOption) error {
				cloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
				Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"status":{"cloudProfileSpec":{"kubernetes":{"versions":[{"expirationDate":"%s","version":"1.0.0"}]}}}}`, newExpiryDate.UTC().Format(time.RFC3339)))) //
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())
		})

		// TODO(LucaBernstein): More test cases needed for changes (invalid) -> different cloud provider (forbidden), shoots still using details (versions, ...) of former valid profile now invalid after switching?

		Context("when deletion timestamp not set", func() {
			BeforeEach(func() {
				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.NamespacedCloudProfile, _ ...client.GetOption) error {
					namespacedCloudProfile.DeepCopyInto(obj)
					return nil
				})

				cloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{}
				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...client.GetOption) error {
					cloudProfile.DeepCopyInto(obj)
					return nil
				})
			})

			It("should ensure the finalizer (error)", func() {
				errToReturn := apierrors.NewNotFound(schema.GroupResource{}, namespaceName+"/"+namespacedCloudProfileName)

				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any())

				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"metadata":{"finalizers":["%s"],"resourceVersion":"42"}}`, finalizerName)))
					return errToReturn
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(err))
			})

			It("should ensure the finalizer (no error)", func() {
				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any())

				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"metadata":{"finalizers":["%s"],"resourceVersion":"42"}}`, finalizerName)))
					return nil
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when deletion timestamp set", func() {
			BeforeEach(func() {
				now := metav1.Now()
				namespacedCloudProfile.DeletionTimestamp = &now
				namespacedCloudProfile.Finalizers = []string{finalizerName}

				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.NamespacedCloudProfile, _ ...client.GetOption) error {
					*obj = *namespacedCloudProfile
					return nil
				})

				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...client.GetOption) error {
					cloudProfile.DeepCopyInto(obj)
					return nil
				})

				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"status":{"cloudProfileSpec":{"kubernetes":{"versions":[{"version":"1.0.0"}]}}}}`)))
					return nil
				})
			})

			It("should do nothing because finalizer is not present", func() {
				namespacedCloudProfile.Finalizers = nil

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error because Shoot referencing NamespacedCloudProfile exists", func() {
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ShootList{Items: []gardencorev1beta1.Shoot{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "test-shoot", Namespace: "test-namespace"},
							Spec: gardencorev1beta1.ShootSpec{
								CloudProfile: &gardencorev1beta1.CloudProfileReference{
									Kind: "NamespacedCloudProfile",
									Name: namespacedCloudProfileName,
								},
							},
						},
					}}).DeepCopyInto(obj)
					return nil
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(ContainSubstring("Cannot delete NamespacedCloudProfile")))
			})

			It("should remove the finalizer (error)", func() {
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ShootList{}).DeepCopyInto(obj)
					return nil
				})

				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(`{"metadata":{"finalizers":null,"resourceVersion":"42"}}`))
					return fakeErr
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should remove the finalizer (no error)", func() {
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ShootList{}).DeepCopyInto(obj)
					return nil
				})

				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(`{"metadata":{"finalizers":null,"resourceVersion":"42"}}`))
					return nil
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
