// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiextensions "github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/flow"
	unstructuredutils "github.com/gardener/gardener/pkg/utils/kubernetes/unstructured"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// RestoreSecrets restores Kubernetes Secrets from the given component state.
// It filters for GardenerResources of type Secret and restores them in the specified namespace.
func RestoreSecrets(ctx context.Context, c client.Client, state component.State, namespace string) error {
	var fns []flow.TaskFn

	for _, v := range state.GetGardenerResources() {
		entry := v

		if entry.Type != v1beta1constants.DataTypeSecret {
			continue
		}

		fns = append(fns, func(ctx context.Context) error {
			objectMeta := metav1.ObjectMeta{
				Name:      entry.Name,
				Namespace: namespace,
				Labels:    entry.Labels,
			}

			return restoreSecretFromPersistedData(ctx, c, objectMeta, entry.Data.Raw)
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// restoreSecretFromPersistedData restores a Kubernetes Secret from persisted GardenerResourceData.
// It handles both formats (with Immutable and Type fields) and (plain map[string][]byte).
func restoreSecretFromPersistedData(ctx context.Context, seedClient client.Client, objectMeta metav1.ObjectMeta, rawData []byte) error {
	var newSecretInfo SecretState

	var (
		secretData map[string][]byte
		immutable  *bool
		secretType = corev1.SecretTypeOpaque
	)

	if err := json.Unmarshal(rawData, &newSecretInfo); err != nil || newSecretInfo.Data == nil {
		// TODO(tobschli): Remove this fallback after v1.143 has been released, as ShootStates will be reconciled and use the new format.
		// plain map[string][]byte
		if err := json.Unmarshal(rawData, &secretData); err != nil {
			return fmt.Errorf("failed unmarshalling secret data for secret %s: neither new nor old format matched: %w", objectMeta.Name, err)
		}

		if objectMeta.Labels[secretsmanager.LabelKeyManagedBy] == secretsmanager.LabelValueSecretsManager {
			secret := secretsmanager.Secret(objectMeta, secretData)
			return client.IgnoreAlreadyExists(seedClient.Create(ctx, secret))
		}
	} else {
		secretData = newSecretInfo.Data
		immutable = newSecretInfo.Immutable
		if newSecretInfo.Type != "" {
			secretType = newSecretInfo.Type
		}
	}

	secret := &corev1.Secret{
		ObjectMeta: objectMeta,
		Type:       secretType,
		Data:       secretData,
		Immutable:  immutable,
	}

	return client.IgnoreAlreadyExists(seedClient.Create(ctx, secret))
}

// SecretState stores the data, immutability and type of a secret to be persisted in the State.
type SecretState struct {
	Data      map[string][]byte `json:"data"`
	Immutable *bool             `json:"immutable,omitempty"`
	Type      corev1.SecretType `json:"type,omitempty"`
}

// ComputeSecretsToPersist computes the secrets that need to be persisted for the ShootState by listing all secrets in the specified namespace with the label persist=true.
func ComputeSecretsToPersist(
	ctx context.Context,
	c client.Client,
	namespace string,
) (
	[]gardencorev1beta1.GardenerResourceData,
	error,
) {
	secretList := &corev1.SecretList{}
	if err := c.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
		secretsmanager.LabelKeyPersist: secretsmanager.LabelValueTrue,
	}); err != nil {
		return nil, fmt.Errorf("failed listing all secrets that must be persisted: %w", err)
	}

	dataList := make([]gardencorev1beta1.GardenerResourceData, 0, len(secretList.Items))

	for _, secret := range secretList.Items {
		secretInfo := SecretState{
			Data:      secret.Data,
			Immutable: secret.Immutable,
			Type:      secret.Type,
		}

		dataJSON, err := json.Marshal(secretInfo)
		if err != nil {
			return nil, fmt.Errorf("failed marshalling secret data to JSON for secret %s: %w", client.ObjectKeyFromObject(&secret), err)
		}

		dataList = append(dataList, gardencorev1beta1.GardenerResourceData{
			Name:   secret.Name,
			Labels: secret.Labels,
			Type:   v1beta1constants.DataTypeSecret,
			Data:   runtime.RawExtension{Raw: dataJSON},
		})
	}

	return dataList, nil
}

// TargetCRD contains the functional interface to list objects of a specific CRD kind.
// It is used to compute the state of extensions and their referenced resources for the ShootState.
type TargetCRD struct {
	ObjKind           string
	NewObjectListFunc func() client.ObjectList
}

// ComputeExtensionsDataAndResources computes the state of extension resources and their referenced resources for the State.
func ComputeExtensionsDataAndResources(
	ctx context.Context,
	c client.Client,
	namespace string,
	crds []TargetCRD,
) (
	[]gardencorev1beta1.ExtensionResourceState,
	[]gardencorev1beta1.ResourceData,
	error,
) {
	var (
		dataList  []gardencorev1beta1.ExtensionResourceState
		resources []gardencorev1beta1.ResourceData
	)

	for _, extension := range crds {
		objList := extension.NewObjectListFunc()
		if err := c.List(ctx, objList, client.InNamespace(namespace)); err != nil {
			return nil, nil, fmt.Errorf("failed to list extension resources of kind %s: %w", extension.ObjKind, err)
		}

		if err := meta.EachListItem(objList, func(obj runtime.Object) error {
			extensionObj, err := apiextensions.Accessor(obj)
			if err != nil {
				return fmt.Errorf("failed accessing extension object: %w", err)
			}

			if extensionObj.GetDeletionTimestamp() != nil ||
				(extensionObj.GetExtensionStatus().GetState() == nil && len(extensionObj.GetExtensionStatus().GetResources()) == 0) {
				return nil
			}

			dataList = append(dataList, gardencorev1beta1.ExtensionResourceState{
				Kind:      extension.ObjKind,
				Name:      ptr.To(extensionObj.GetName()),
				Purpose:   extensionObj.GetExtensionSpec().GetExtensionPurpose(),
				State:     extensionObj.GetExtensionStatus().GetState(),
				Resources: extensionObj.GetExtensionStatus().GetResources(),
			})

			for _, newResource := range extensionObj.GetExtensionStatus().GetResources() {
				referencedObj, err := unstructuredutils.GetObjectByRef(ctx, c, &newResource.ResourceRef, namespace)
				if err != nil {
					return fmt.Errorf("failed reading referenced object %s: %w", client.ObjectKey{Name: newResource.ResourceRef.Name, Namespace: namespace}, err)
				}
				if obj == nil {
					return fmt.Errorf("object %v not found", newResource.ResourceRef)
				}

				raw := &runtime.RawExtension{}
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(referencedObj, raw); err != nil {
					return fmt.Errorf("failed converting referenced object %s to raw extension: %w", client.ObjectKey{Name: newResource.ResourceRef.Name, Namespace: namespace}, err)
				}

				resources = append(resources, gardencorev1beta1.ResourceData{
					CrossVersionObjectReference: newResource.ResourceRef,
					Data:                        *raw,
				})
			}

			return nil
		}); err != nil {
			return nil, nil, fmt.Errorf("failed computing extension data for kind %s: %w", extension.ObjKind, err)
		}
	}

	return dataList, resources, nil
}
