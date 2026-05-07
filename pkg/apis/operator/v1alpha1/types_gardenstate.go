// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GardenState describes a persisted state of the garden.
type GardenState struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Gardener holds the data required to restore resources deployed by the garden operator.
	// +optional
	Gardener []gardencorev1beta1.GardenerResourceData `json:"gardener,omitempty"`
	// Garden holds the garden resource state needed for restore.
	Garden GardenStateGarden `json:"garden,omitempty"`
	// Extensions holds the state of custom resources reconciled by extension controllers in the runtime cluster.
	// +optional
	Extensions []gardencorev1beta1.ExtensionResourceState `json:"extensions,omitempty"`
	// Resources holds the data of resources referred to by extension controller states.
	// +optional
	Resources []gardencorev1beta1.ResourceData `json:"resources,omitempty"`
}

// GardenStateGarden contains the Garden resource data persisted in a GardenState.
// It uses explicit metadata fields instead of embedding ObjectMeta, because nested ObjectMeta
// gets pruned by CRD structural schema validation.
type GardenStateGarden struct {
	// Name is the name of the Garden resource.
	Name string `json:"name"`
	// Labels are the labels of the Garden resource.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations are the annotations of the Garden resource.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// UID is the UID of the Garden resource. It is needed during restore to preserve the etcd
	// backup bucket name which defaults to "garden-<UID>".
	// +optional
	UID string `json:"uid,omitempty"`
	// Spec is the spec of the Garden resource.
	Spec GardenSpec `json:"spec"`
	// Status is the status of the Garden resource.
	// +optional
	Status GardenStatus `json:"status,omitempty"`
}

// GetGardenerResources returns the list of gardener resources to be restored from the GardenState.
func (s *GardenState) GetGardenerResources() []gardencorev1beta1.GardenerResourceData {
	return s.Gardener
}

// GetExtensions returns the list of extension resources to be restored from the GardenState.
func (s *GardenState) GetExtensions() []gardencorev1beta1.ExtensionResourceState {
	return s.Extensions
}

// GetResources returns the list of resources to be restored from the GardenState.
func (s *GardenState) GetResources() []gardencorev1beta1.ResourceData {
	return s.Resources
}
