// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// GardenStateBootstrappedCondition is the pod readiness gate condition set after the garden-state restore bootstrapper completed.
	GardenStateBootstrappedCondition corev1.PodConditionType = "operator.gardener.cloud/bootstrapped"

	gardenStateBootstrappedReason = "GardenStateRestoreCompleted"
)

// SetPodBootstrappedCondition marks the given gardener-operator pod as bootstrapped.
func SetPodBootstrappedCondition(ctx context.Context, c client.Client, podKey types.NamespacedName) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pod := &corev1.Pod{}
		if err := c.Get(ctx, podKey, pod); err != nil {
			return err
		}

		now := metav1.Now()
		condition := corev1.PodCondition{
			Type:               GardenStateBootstrappedCondition,
			Status:             corev1.ConditionTrue,
			LastProbeTime:      now,
			LastTransitionTime: now,
			Reason:             gardenStateBootstrappedReason,
			Message:            "The garden-state restore bootstrapper completed successfully",
		}

		for i := range pod.Status.Conditions {
			if pod.Status.Conditions[i].Type != GardenStateBootstrappedCondition {
				continue
			}

			if pod.Status.Conditions[i].Status == condition.Status {
				condition.LastTransitionTime = pod.Status.Conditions[i].LastTransitionTime
			}
			pod.Status.Conditions[i] = condition
			return c.Status().Update(ctx, pod)
		}

		pod.Status.Conditions = append(pod.Status.Conditions, condition)
		return c.Status().Update(ctx, pod)
	})
}
