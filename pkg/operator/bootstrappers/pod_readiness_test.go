// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/pkg/operator/bootstrappers"
)

var _ = Describe("Pod readiness", func() {
	var (
		ctx    context.Context
		podKey types.NamespacedName
		scheme *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		podKey = types.NamespacedName{Namespace: "garden", Name: "gardener-operator-12345"}
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
	})

	It("adds the bootstrapped condition while preserving existing conditions", func() {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: podKey.Namespace, Name: podKey.Name},
			Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{
				Type: corev1.PodReady, Status: corev1.ConditionFalse,
			}}},
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(pod).WithObjects(pod).Build()

		Expect(SetPodBootstrappedCondition(ctx, c, podKey)).To(Succeed())
		Expect(c.Get(ctx, podKey, pod)).To(Succeed())
		Expect(pod.Status.Conditions).To(ConsistOf(
			corev1.PodCondition{Type: corev1.PodReady, Status: corev1.ConditionFalse},
			And(
				HaveField("Type", GardenStateBootstrappedCondition),
				HaveField("Status", corev1.ConditionTrue),
				HaveField("Reason", "GardenStateRestoreCompleted"),
				HaveField("LastTransitionTime", Not(BeZero())),
			),
		))
	})

	It("updates an existing condition without changing its transition time", func() {
		transitionTime := metav1.NewTime(time.Date(2026, time.November, 11, 12, 0, 0, 0, time.UTC))
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: podKey.Namespace, Name: podKey.Name},
			Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{
				Type: GardenStateBootstrappedCondition, Status: corev1.ConditionTrue, LastTransitionTime: transitionTime,
			}}},
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(pod).WithObjects(pod).Build()

		Expect(SetPodBootstrappedCondition(ctx, c, podKey)).To(Succeed())
		Expect(c.Get(ctx, podKey, pod)).To(Succeed())
		Expect(pod.Status.Conditions).To(HaveLen(1))
		Expect(pod.Status.Conditions[0].LastTransitionTime.Time.Equal(transitionTime.Time)).To(BeTrue())
		Expect(pod.Status.Conditions[0].Reason).To(Equal("GardenStateRestoreCompleted"))
	})
})
