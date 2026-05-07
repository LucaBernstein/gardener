// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/component/state"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operator/bootstrappers"
)

func (r *Reconciler) syncGardenState(ctx context.Context, log logr.Logger, garden *operatorv1alpha1.Garden) error {
	log.V(1).Info("Syncing garden-state to secret")
	targetClient := r.RuntimeClientSet.Client()
	namespace := r.GardenNamespace
	entries, err := state.ComputeSecretsToPersist(ctx, targetClient, namespace)
	if err != nil {
		return fmt.Errorf("failed computing secrets to persist: %w", err)
	}

	crds := []state.TargetCRD{
		{extensionsv1alpha1.BackupEntryResource, func() client.ObjectList { return &extensionsv1alpha1.BackupEntryList{} }},
		{extensionsv1alpha1.DNSRecordResource, func() client.ObjectList { return &extensionsv1alpha1.DNSRecordList{} }},
		{extensionsv1alpha1.ExtensionResource, func() client.ObjectList { return &extensionsv1alpha1.ExtensionList{} }},
	}
	extensions, resources, err := state.ComputeExtensionsDataAndResources(ctx, targetClient, namespace, crds)
	if err != nil {
		return fmt.Errorf("failed computing extensions data and resources: %w", err)
	}

	stateSecret, err := bootstrappers.EncodeGardenStateSecret(garden, entries, extensions, resources)
	if err != nil {
		return fmt.Errorf("failed encoding garden-state secret: %w", err)
	}

	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stateSecret.Name,
			Namespace: stateSecret.Namespace,
		},
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.RuntimeClientSet.Client(), existing, func() error {
		existing.Labels = stateSecret.Labels
		existing.Data = stateSecret.Data
		return nil
	}); err != nil {
		return fmt.Errorf("failed creating or updating garden-state secret: %w", err)
	}

	return nil
}
