// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	. "github.com/gardener/gardener/plugin/pkg/namespacedcloudprofile/validator"
)

var _ = Describe("Admission", func() {
	Describe("#Validate", func() {
		var (
			ctx                 context.Context
			admissionHandler    *ValidateNamespacedCloudProfile
			coreInformerFactory gardencoreinformers.SharedInformerFactory

			namespacedCloudProfile       gardencore.NamespacedCloudProfile
			namespacedCloudProfileParent gardencore.CloudProfileReference
			parentCloudProfile           gardencorev1beta1.CloudProfile
			machineType                  gardencorev1beta1.MachineType
			machineTypeCore              gardencore.MachineType
			//machineImageCore             gardencore.MachineImage

			namespacedCloudProfileBase = gardencore.NamespacedCloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "profile",
				},
			}
			parentCloudProfileBase = gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "parent-profile",
				},
				Spec: gardencorev1beta1.CloudProfileSpec{
					Kubernetes: gardencorev1beta1.KubernetesSettings{Versions: []gardencorev1beta1.ExpirableVersion{{Version: "1.30.0"}}},
				},
			}
			machineTypeBase = gardencorev1beta1.MachineType{
				Name:         "my-machine",
				Architecture: ptr.To("arm64"),
			}
			machineTypeCoreBase = gardencore.MachineType{
				Name:         "my-machine",
				Architecture: ptr.To("arm64"),
			}
			//machineImageCoreBase = gardencore.MachineImage{
			//	Name: "my-image",
			//	Versions: []gardencore.MachineImageVersion{{
			//		ExpirableVersion: gardencore.ExpirableVersion{Version: "1.0.0"},
			//		CRI:              []gardencore.CRI{{Name: "containerd"}},
			//	}},
			//}
		)

		BeforeEach(func() {
			ctx = context.TODO()

			namespacedCloudProfile = *namespacedCloudProfileBase.DeepCopy()
			namespacedCloudProfileParent = gardencore.CloudProfileReference{
				Kind: "CloudProfile",
				Name: parentCloudProfileBase.Name,
			}
			parentCloudProfile = *parentCloudProfileBase.DeepCopy()
			machineType = machineTypeBase
			machineTypeCore = machineTypeCoreBase
			//machineImageCore = machineImageCoreBase

			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)
		})

		Describe("parent", func() {
			It("should not allow creating a NamespacedCloudProfile with an invalid parent reference", func() {
				namespacedCloudProfile.Spec.Parent = gardencore.CloudProfileReference{Kind: "CloudProfile", Name: "idontexist"}

				attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("parent CloudProfile could not be found")))
			})

			It("should allow creating a NamespacedCloudProfile with a valid parent reference", func() {
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent

				attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
			})
		})

		Describe("Kubernetes versions", func() {
			It("should not allow creating a (Namespaced)CloudProfile if the resulting Kubernetes versions are empty", func() {
				parentCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent

				attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("must provide at least one Kubernetes version")))
			})

			It("should fail for the latest Kubernetes version being set an expiration date after a potential merge", func() {
				parentCloudProfile.Spec.Kubernetes = gardencorev1beta1.KubernetesSettings{Versions: []gardencorev1beta1.ExpirableVersion{
					{Version: "1.30.0"},
				}}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
				namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
					{Version: "1.30.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(time.Hour)})},
				}}

				attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("expiration date of latest kubernetes version ('1.30.0') must not be set")))
			})

			It("should allow creating a NamespacedCloudProfile that specifies a Kubernetes version from the parent CloudProfile and extends the expiration date", func() {
				parentCloudProfile.Spec.Kubernetes = gardencorev1beta1.KubernetesSettings{Versions: []gardencorev1beta1.ExpirableVersion{
					{Version: "1.30.0"}, {Version: "1.29.1"},
				}}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
				namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
					{Version: "1.29.1", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(time.Hour)})},
				}}

				attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
			})

			It("should fail for creating a NamespacedCloudProfile that specifies a Kubernetes version not in the parent CloudProfile", func() {
				parentCloudProfile.Spec.Kubernetes = gardencorev1beta1.KubernetesSettings{Versions: []gardencorev1beta1.ExpirableVersion{
					{Version: "1.29.0"},
				}}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
				namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
					{Version: "1.30.0", ExpirationDate: ptr.To(metav1.Now())},
				}}

				attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("invalid kubernetes version specified: '1.30.0' does not exist in parent")))
			})

			It("should fail for creating a NamespacedCloudProfile that specifies a Kubernetes version without an expiration date", func() {
				parentCloudProfile.Spec.Kubernetes = gardencorev1beta1.KubernetesSettings{Versions: []gardencorev1beta1.ExpirableVersion{
					{Version: "1.29.0"}, {Version: "1.30.0"},
				}}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
				namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
					{Version: "1.29.0"},
				}}

				attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("specified version '1.29.0' does not set expiration date")))
			})

			It("should fail for past expiration dates", func() {
				parentCloudProfile.Spec.Kubernetes = gardencorev1beta1.KubernetesSettings{Versions: []gardencorev1beta1.ExpirableVersion{
					{Version: "1.29.0"}, {Version: "1.30.0"},
				}}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				pastExpirationDate := &metav1.Time{Time: time.Now().Add(-1 * time.Hour)}
				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
				namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
					{Version: "1.29.0", ExpirationDate: pastExpirationDate},
				}}

				attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("expiration date of version '1.29.0' is in the past")))
			})

			It("should allow updates to a NamespacedCloudProfile even if one unchanged overridden Kubernetes version is already expired", func() {
				parentCloudProfile.Spec.Kubernetes = gardencorev1beta1.KubernetesSettings{Versions: []gardencorev1beta1.ExpirableVersion{
					{Version: "1.28.0"}, {Version: "1.29.0"}, {Version: "1.30.0"},
				}}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				pastExpirationDate := &metav1.Time{Time: time.Now().Add(-1 * time.Hour)}
				futureExpirationDate := &metav1.Time{Time: time.Now().Add(1 * time.Hour)}
				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
				namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
					{Version: "1.28.0", ExpirationDate: pastExpirationDate},
				}}
				namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions = []gardencore.ExpirableVersion{
					{Version: "1.28.0", ExpirationDate: pastExpirationDate}, {Version: "1.29.0"}, {Version: "1.30.0"},
				}

				updatedNamespacedCloudProfile := (&namespacedCloudProfile).DeepCopy()
				namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
					{Version: "1.28.0", ExpirationDate: pastExpirationDate},
					{Version: "1.29.0", ExpirationDate: futureExpirationDate},
				}}

				attrs := admission.NewAttributesRecord(updatedNamespacedCloudProfile, &namespacedCloudProfile, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
			})

			It("should fail for updating a expiration date to a still invalid value", func() {
				parentCloudProfile.Spec.Kubernetes = gardencorev1beta1.KubernetesSettings{Versions: []gardencorev1beta1.ExpirableVersion{
					{Version: "1.28.0"}, {Version: "1.29.0"},
				}}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				pastExpirationDate := &metav1.Time{Time: time.Now().Add(-1 * time.Hour)}
				stillExpiredDate := &metav1.Time{Time: time.Now().Add(-30 * time.Minute)}
				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
				namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
					{Version: "1.28.0", ExpirationDate: pastExpirationDate},
				}}
				namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions = []gardencore.ExpirableVersion{
					{Version: "1.28.0", ExpirationDate: pastExpirationDate}, {Version: "1.29.0"},
				}

				updatedNamespacedCloudProfile := (&namespacedCloudProfile).DeepCopy()
				updatedNamespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
					{Version: "1.28.0", ExpirationDate: stillExpiredDate},
				}}

				attrs := admission.NewAttributesRecord(updatedNamespacedCloudProfile, &namespacedCloudProfile, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("expiration date of version '1.28.0' is in the past")))
			})
		})

		Describe("machineType", func() {
			It("should not allow creating a NamespacedCloudProfile that defines a machineType of the parent CloudProfile", func() {
				parentCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{machineType}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
				namespacedCloudProfile.Spec.MachineTypes = []gardencore.MachineType{machineTypeCore}

				attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("NamespacedCloudProfile attempts to overwrite parent CloudProfile with machineType")))
				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("my-machine")))
			})

			It("should allow creating a NamespacedCloudProfile that defines a machineType of the parent CloudProfile if it was added to the NamespacedCloudProfile first", func() {
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
				namespacedCloudProfile.Spec.MachineTypes = []gardencore.MachineType{machineTypeCore}

				attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())

				parentCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{machineType}

				attrs = admission.NewAttributesRecord(&namespacedCloudProfile, &namespacedCloudProfile, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
			})

			It("should allow creating a NamespacedCloudProfile that defines a machineType of the parent CloudProfile if it was added to the NamespacedCloudProfile first but is changed", func() {
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
				namespacedCloudProfile.Spec.MachineTypes = []gardencore.MachineType{machineTypeCore}

				attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())

				oldNamespacedCloudProfile := *namespacedCloudProfile.DeepCopy()
				machineType.Usable = ptr.To(false)
				parentCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{machineType}

				attrs = admission.NewAttributesRecord(&namespacedCloudProfile, &oldNamespacedCloudProfile, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
			})

			It("should allow creating a NamespacedCloudProfile that defines a different machineType than the parent CloudProfile", func() {
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
				namespacedCloudProfile.Spec.MachineTypes = []gardencore.MachineType{{Name: "my-other-machine", Architecture: ptr.To("amd64")}}

				parentCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{machineType}

				attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
			})
		})

		Describe("machineImages", func() {
			It("should allow creating a NamespacedCloudProfile that specifies a MachineImage version from the parent CloudProfile and extends the expiration date", func() {
				parentCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{Name: "test-image", Versions: []gardencorev1beta1.MachineImageVersion{{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}}}},
				}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
				namespacedCloudProfile.Spec.MachineImages = []gardencore.MachineImage{
					{Name: "test-image", Versions: []gardencore.MachineImageVersion{{ExpirableVersion: gardencore.ExpirableVersion{Version: "1.0.0", ExpirationDate: ptr.To(metav1.Now())}, CRI: []gardencore.CRI{{Name: "containerd"}}}}},
				}

				attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
			})

			It("should fail for creating a NamespacedCloudProfile that specifies a MachineImage entry not in the parent CloudProfile", func() {
				parentCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{Name: "test-image", Versions: []gardencorev1beta1.MachineImageVersion{{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}}}},
				}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
				namespacedCloudProfile.Spec.MachineImages = []gardencore.MachineImage{
					{Name: "another-image", Versions: []gardencore.MachineImageVersion{{ExpirableVersion: gardencore.ExpirableVersion{Version: "1.0.0", ExpirationDate: ptr.To(metav1.Now())}, CRI: []gardencore.CRI{{Name: "containerd"}}}}},
				}

				attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("invalid machine image specified: 'another-image' does not exist in parent")))
			})

			It("should fail for creating a NamespacedCloudProfile that specifies a MachineImage entry version not in the parent CloudProfile", func() {
				parentCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{Name: "test-image", Versions: []gardencorev1beta1.MachineImageVersion{{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}}}},
				}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

				namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
				namespacedCloudProfile.Spec.MachineImages = []gardencore.MachineImage{
					{Name: "test-image", Versions: []gardencore.MachineImageVersion{{ExpirableVersion: gardencore.ExpirableVersion{Version: "1.2.0", ExpirationDate: ptr.To(metav1.Now())}, CRI: []gardencore.CRI{{Name: "containerd"}}}}},
				}

				attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("invalid machine image specified: 'test-image@1.2.0' does not exist in parent")))
			})
		})
	})

	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement("NamespacedCloudProfileValidator"))
		})
	})

	Describe("#New", func() {
		It("should only handle CREATE and UPDATE operations", func() {
			dr, err := New()
			Expect(err).ToNot(HaveOccurred())
			Expect(dr.Handles(admission.Create)).To(BeTrue())
			Expect(dr.Handles(admission.Update)).To(BeTrue())
			Expect(dr.Handles(admission.Connect)).To(BeFalse())
			Expect(dr.Handles(admission.Delete)).To(BeFalse())
		})
	})
})
