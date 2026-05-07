// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/api/operator/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/component/state"
)

const (
	// LabelKeyGardenStatePurpose is the label key used to identify the garden-state secret.
	LabelKeyGardenStatePurpose = "operator.gardener.cloud/purpose"
	// LabelValueGardenState is the label value used to identify the garden-state secret.
	LabelValueGardenState = "garden-state"
	// GardenStateSecretName is the name of the garden-state secret.
	GardenStateSecretName = "garden-state"

	// gardenStateSecretStateKey is the key in the secret data where the garden-state is stored.
	gardenStateSecretStateKey = "state"
)

// RestoreGardenState checks for a garden-state secret and restores the Garden resource from it
// if no Garden currently exists. This blocks operator startup until restore is complete.
func RestoreGardenState(ctx context.Context, log logr.Logger, c client.Client, gardenNamespace string) error {
	gardenList := &operatorv1alpha1.GardenList{}
	if err := c.List(ctx, gardenList, client.Limit(1)); err != nil {
		return fmt.Errorf("failed listing Gardens: %w", err)
	}
	if len(gardenList.Items) > 0 {
		log.V(1).Info("Garden already exists, skipping garden-state restore")
		return nil
	}

	secret, err := GetGardenStateSecret(ctx, c, gardenNamespace)
	if err != nil {
		return err
	}
	if secret == nil {
		log.V(1).Info("No garden-state secret found, nothing to restore")
		return nil
	}

	log.Info("Found garden-state secret, restoring Garden")

	stateData, err := DecodeGardenStateSecret(secret)
	if err != nil {
		return fmt.Errorf("failed decoding garden-state secret: %w", err)
	}

	if err := state.RestoreSecrets(ctx, c, stateData, gardenNamespace); err != nil {
		return fmt.Errorf("failed restoring secrets from garden-state: %w", err)
	}
	log.Info("Restored secrets from garden-state")

	garden, err := createGardenFromState(ctx, c, stateData)
	if err != nil {
		return fmt.Errorf("failed creating Garden from garden-state: %w", err)
	}
	log.Info("Created Garden resource from garden-state")

	if err := patchGardenStatus(ctx, c, garden, stateData); err != nil {
		return fmt.Errorf("failed patching Garden status from garden-state: %w", err)
	}
	log.Info("Patched Garden status from garden-state")

	log.Info("Garden restore from garden-state completed successfully")
	return nil
}

// GetGardenStateSecret retrieves the garden-state secret from the given namespace.
// It returns nil if no such secret exists.
func GetGardenStateSecret(ctx context.Context, c client.Reader, gardenNamespace string) (*corev1.Secret, error) {
	secretList := &corev1.SecretList{}
	if err := c.List(ctx, secretList,
		client.InNamespace(gardenNamespace),
		client.MatchingLabels{LabelKeyGardenStatePurpose: LabelValueGardenState},
		client.Limit(1),
	); err != nil {
		return nil, fmt.Errorf("failed listing garden-state secrets: %w", err)
	}

	if len(secretList.Items) == 0 {
		return nil, nil
	}

	return &secretList.Items[0], nil
}

// DecodeGardenStateSecret decodes the garden-state from the given secret.
// It expects the secret to have a specific format and returns an error if decoding fails.
func DecodeGardenStateSecret(secret *corev1.Secret) (*operatorv1alpha1.GardenState, error) {
	raw, ok := secret.Data[gardenStateSecretStateKey]
	if !ok {
		return nil, fmt.Errorf("garden-state secret is missing 'state' key")
	}

	data := &operatorv1alpha1.GardenState{}
	if err := json.Unmarshal(raw, data); err != nil {
		return nil, fmt.Errorf("failed unmarshalling garden-state data: %w", err)
	}

	return data, nil
}

func createGardenFromState(ctx context.Context, c client.Client, stateData *operatorv1alpha1.GardenState) (*operatorv1alpha1.Garden, error) {
	garden := &operatorv1alpha1.Garden{
		ObjectMeta: metav1.ObjectMeta{
			Name:        stateData.Garden.Name,
			Labels:      stateData.Garden.Labels,
			Annotations: stateData.Garden.Annotations,
		},
		Spec: stateData.Garden.Spec,
	}

	ensureBackupBucketName(garden, types.UID(stateData.Garden.UID))

	if err := client.IgnoreAlreadyExists(c.Create(ctx, garden)); err != nil {
		return nil, err
	}

	return garden, nil
}

func ensureBackupBucketName(garden *operatorv1alpha1.Garden, oldUID types.UID) {
	if oldUID == "" {
		return
	}

	backup := helper.GetETCDMainBackup(garden)
	if backup == nil {
		return
	}

	if backup.BucketName != nil {
		return
	}

	backup.BucketName = ptr.To("garden-" + string(oldUID))
}

func patchGardenStatus(ctx context.Context, c client.Client, garden *operatorv1alpha1.Garden, stateData *operatorv1alpha1.GardenState) error {
	patch := client.MergeFrom(garden.DeepCopy())
	garden.Status = stateData.Garden.Status
	garden.Status.LastOperation.State = gardencorev1beta1.LastOperationStatePending
	garden.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeRestore
	garden.Status.Conditions = nil
	return c.Status().Patch(ctx, garden, patch)
}

// EncodeGardenStateSecret encodes the garden-state into a Secret. This is called at the end of
// garden reconciliation to persist the current state.
func EncodeGardenStateSecret(garden *operatorv1alpha1.Garden, secrets []gardencorev1beta1.GardenerResourceData, extensions []gardencorev1beta1.ExtensionResourceState, resources []gardencorev1beta1.ResourceData) (*corev1.Secret, error) {
	stateData := &operatorv1alpha1.GardenState{
		Gardener: secrets,
		Garden: operatorv1alpha1.GardenStateGarden{
			Name:        garden.Name,
			Labels:      garden.Labels,
			Annotations: garden.Annotations,
			UID:         string(garden.UID),
			Spec:        garden.Spec,
			Status:      pruneGardenStatus(garden.Status),
		},
		Extensions: extensions,
		Resources:  resources,
	}

	raw, err := json.Marshal(stateData)
	if err != nil {
		return nil, fmt.Errorf("failed marshalling garden-state data: %w", err)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GardenStateSecretName,
			Namespace: v1beta1constants.GardenNamespace,
			Labels: map[string]string{
				LabelKeyGardenStatePurpose: LabelValueGardenState,
			},
		},
		Data: map[string][]byte{
			gardenStateSecretStateKey: raw,
		},
	}, nil
}

func pruneGardenStatus(status operatorv1alpha1.GardenStatus) operatorv1alpha1.GardenStatus {
	status.Conditions = nil
	return status
}
