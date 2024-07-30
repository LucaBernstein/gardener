// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacedcloudprofile

import (
	"context"
	"fmt"
	"github.com/gardener/gardener/plugin/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// Reconciler reconciles CloudProfiles.
type Reconciler struct {
	Client   client.Client
	Config   config.NamespacedCloudProfileControllerConfiguration
	Recorder record.EventRecorder
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	namespacedCloudProfile := &gardencorev1beta1.NamespacedCloudProfile{}
	if err := r.Client.Get(ctx, request.NamespacedName, namespacedCloudProfile); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	parentCloudProfile := &gardencorev1beta1.CloudProfile{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: namespacedCloudProfile.Spec.Parent.Name}, parentCloudProfile); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Parent object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if err := patchCloudProfileStatus(ctx, r.Client, namespacedCloudProfile, parentCloudProfile); err != nil {
		return reconcile.Result{}, err
	}

	// The deletionTimestamp labels the NamespacedCloudProfile as intended to get deleted. Before deletion, it has to be ensured that
	// no Shoots and Seed are assigned to the NamespacedCloudProfile anymore. If this is the case then the controller will remove
	// the finalizers from the NamespacedCloudProfile so that it can be garbage collected.
	if namespacedCloudProfile.DeletionTimestamp != nil {
		if !sets.New(namespacedCloudProfile.Finalizers...).Has(gardencorev1beta1.GardenerName) {
			return reconcile.Result{}, nil
		}

		associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(ctx, r.Client, namespacedCloudProfile)
		if err != nil {
			return reconcile.Result{}, err
		}

		if len(associatedShoots) == 0 {
			log.Info("No Shoots are referencing the NamespacedCloudProfile, deletion accepted")

			if controllerutil.ContainsFinalizer(namespacedCloudProfile, gardencorev1beta1.GardenerName) {
				log.Info("Removing finalizer")
				if err := controllerutils.RemoveFinalizers(ctx, r.Client, namespacedCloudProfile, gardencorev1beta1.GardenerName); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
				}
			}

			return reconcile.Result{}, nil
		}

		message := fmt.Sprintf("Cannot delete NamespacedCloudProfile, because the following Shoots are still referencing it: %+v", associatedShoots)
		r.Recorder.Event(namespacedCloudProfile, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, message)
		return reconcile.Result{}, fmt.Errorf(message)
	}

	if !controllerutil.ContainsFinalizer(namespacedCloudProfile, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, namespacedCloudProfile, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func patchCloudProfileStatus(ctx context.Context, c client.Client, namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile, parentCloudProfile *gardencorev1beta1.CloudProfile) error {
	patch := client.StrategicMergeFrom(namespacedCloudProfile.DeepCopy())
	utils.MergeCloudProfiles(parentCloudProfile, namespacedCloudProfile)
	namespacedCloudProfile.Status.CloudProfileSpec = parentCloudProfile.Spec
	return c.Patch(ctx, namespacedCloudProfile, patch)
}
