---
title: Disaster Recovery: Restoring Shoot Control Planes on the Same Seed
description: How to restore Shoot control planes in place after a Seed etcd disaster using the force-restore operation
categories:
  - Operators
---

# Disaster Recovery: Restoring Shoot Control Planes on the Same Seed

This document describes how to recover Shoot control planes after an etcd disaster on a Seed cluster, when the Seed itself is recovered in place (i.e., reusing the same backup bucket rather than migrating to a different Seed).

This applies to unmanaged Seeds (i.e., Seeds that are not backed by a `ManagedSeed` resource).

## Background

The standard Gardener [control plane migration](./control_plane_migration.md) flow copies the `BackupEntry` from the source to the destination Seed. When recovering on the **same** Seed, this copy step is unnecessary — the backup bucket already points to the right location. The `force-restore` operation annotation skips the backup copy and triggers a direct restore from the existing backup.

## Prerequisites

- The Seed object still exists in the garden cluster (or will be re-registered under the **same name**). The `shoot.gardener.cloud/emergency-stop-reconciliations` annotation below is set on this garden-cluster object, so it can be applied even while the Seed cluster itself is still down.
- The Seed cluster can be recovered in place (same backup bucket), and its gardenlet can be brought back up.
- Each Shoot's etcd data was backed up by `etcd-backup-restore` to the backup bucket before the disaster, and the backup bucket is still intact and reachable.
- A current `ShootState` exists in the garden cluster for each affected Shoot. The restore reconstructs extension state (infrastructure, machines, etc.) from the `ShootState`, so the `ShootState` controller must have been persisting state before the disaster. Without it, there is nothing to restore the control plane's extension state from.

> [!IMPORTANT]
> Set the emergency stop (Step 1) **before** the recovered gardenlet comes back up. A Shoot's
> `status.lastOperation` lives in the garden cluster and survives the Seed etcd loss as `Reconcile`
> `Succeeded`. If gardenlet starts before the emergency stop is in place, it computes an ordinary
> `Reconcile` (the `force-restore` annotation is not set yet) and reconciles against the empty etcd,
> regenerating the control plane's secrets. If that happens, you must delete the affected Shoot's
> control plane and start over. The emergency stop is a garden-cluster annotation, so applying it does
> not require the Seed cluster to be up.

## Recovery Procedure

### 1. Stop Shoot Reconciliations on the Seed (before recovering the Seed)

Before recovering the Seed cluster, prevent gardenlet from reconciling Shoots once it comes back. Annotate the Seed resource in the garden cluster:

```bash
kubectl -n garden annotate seed <seed-name> shoot.gardener.cloud/emergency-stop-reconciliations=true
```

This blocks gardenlet from picking up any Shoot reconciliation until you lift the annotation, so it cannot start an ordinary reconcile against the freshly recovered (empty) etcd.

### 2. Recover the Seed Cluster

Recover the Seed cluster in place (reusing the same backup bucket) and bring its gardenlet back up. Because the emergency stop from Step 1 is already in place, gardenlet will not reconcile any Shoots yet. Wait until the Seed reports ready (`GardenletReady` and `SeedSystemComponentsHealthy` conditions are `True`).

### 3. Annotate Each Affected Shoot with `force-restore`

For each Shoot scheduled on the Seed, set the `force-restore` operation annotation:

```bash
kubectl -n <project-namespace> annotate shoot <shoot-name> gardener.cloud/operation=force-restore
```

The Gardener API server will automatically bump the Shoot's generation when this annotation is set, so gardenlet will reconcile it once the emergency stop is removed.

> [!NOTE]
> If many Shoots are affected, you can annotate all Shoots in a namespace or across all namespaces:
> ```bash
> # All Shoots in one project namespace
> kubectl -n <project-namespace> annotate shoot --all gardener.cloud/operation=force-restore
>
> # All Shoots across all namespaces (requires cluster-admin)
> kubectl annotate shoot --all --all-namespaces gardener.cloud/operation=force-restore
> ```

### 4. Lift the Emergency Stop

Once all affected Shoots are annotated, remove the emergency stop from the Seed to allow reconciliation to proceed:

```bash
kubectl -n garden annotate seed <seed-name> shoot.gardener.cloud/emergency-stop-reconciliations-
```

Gardenlet will now start reconciling the annotated Shoots. For each one, it:

1. Detects the `force-restore` annotation and maps it to a Restore operation.
2. Skips the backup copy step (the backup bucket is unchanged).
3. Deploys etcd, triggering a restore from the latest snapshot.
4. Removes the `force-restore` annotation once etcd is ready.
5. Continues with the normal reconciliation flow (deploying kube-apiserver, etc.).

### 5. Monitor Progress

Watch the Shoot status for each cluster:

```bash
kubectl -n <project-namespace> get shoot <shoot-name> -w
```

A Shoot in restore phase will have `status.lastOperation.type: Restore`.

## Notes

- **Generation bump on annotation**: the Gardener API server bumps the Shoot generation when `force-restore` is set. gardenlet's `GenerationChangedPredicate` will therefore trigger a reconciliation without requiring any spec change. The bump only happens for Shoots that already have a `status.lastOperation` (i.e. were reconciled at least once before the disaster); this is always the case for a Shoot that was healthy before the Seed was lost.
- **Annotation removal**: gardenlet removes the `force-restore` annotation automatically once etcd is ready. No manual cleanup is needed.
- **Self-hosted Shoots**: `force-restore` is not supported for self-hosted Shoots (`IsSelfHosted() == true`) and is rejected by the Gardener API server for them. These follow a different recovery path.
