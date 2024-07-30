// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"

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
			machineImageCore             gardencore.MachineImage

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
			machineImageCoreBase = gardencore.MachineImage{
				Name: "my-image",
				Versions: []gardencore.MachineImageVersion{{
					ExpirableVersion: gardencore.ExpirableVersion{Version: "1.0.0"},
					CRI:              []gardencore.CRI{{Name: "containerd"}},
				}},
			}
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
			machineImageCore = machineImageCoreBase

			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)
		})

		It("should not allow creating a NamespacedCloudProfile with an invalid parent reference", func() {
			namespacedCloudProfile.Spec.Parent = gardencore.CloudProfileReference{Kind: "CloudProfile", Name: "idontexist"}

			attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("parent CloudProfile could not be found")))
		})

		It("should not allow creating a (Namespaced)CloudProfile if the resulting Kubernetes versions are empty", func() {
			parentCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{}
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

			namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent

			attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("must provide at least one Kubernetes version")))
		})

		It("should allow creating a NamespacedCloudProfile with a valid parent reference", func() {
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

			namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent

			attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
		})

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
			namespacedCloudProfile.Spec.MachineImages = []gardencore.MachineImage{machineImageCore}
			parentCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{machineType}

			attrs = admission.NewAttributesRecord(&namespacedCloudProfile, &oldNamespacedCloudProfile, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)

			Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
		})

		It("should allow creating a NamespacedCloudProfile that defines a different machineType than the parent CloudProfile", func() {
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

			namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
			namespacedCloudProfile.Spec.MachineTypes = []gardencore.MachineType{{Name: "my-other-machine"}}

			parentCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{machineType}

			attrs := admission.NewAttributesRecord(&namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
		})

		It("should allow creating a NamespacedCloudProfile that specifies a Kubernetes version from the parent CloudProfile and extends the expiration date", func() {
			parentCloudProfile.Spec.Kubernetes = gardencorev1beta1.KubernetesSettings{Versions: []gardencorev1beta1.ExpirableVersion{
				{Version: "1.30.0"},
			}}
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&parentCloudProfile)).To(Succeed())

			namespacedCloudProfile.Spec.Parent = namespacedCloudProfileParent
			namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
				{Version: "1.30.0", ExpirationDate: ptr.To(metav1.Now())},
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

			Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("invalid version specified: '1.30.0' does not exist in parent")))
		})

		// TODO(LucaBernstein): NamespacedCLoudProfile machineImages
		//  - new versions may be added without expiration date
		//  - new versions may be added with expiration date
		//  - existing versions expiration date may be overridden
		//  - Q: May new categories of machine images be added?
		//  - Q: Should checks on the specified expiration dates be performed (e.g. only extend, not reduce)?
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
