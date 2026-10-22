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

- The Seed's etcd has been restored and the Seed cluster is operational again.
- The gardenlet on the Seed is running.
- Each Shoot's etcd data was backed up by `etcd-backup-restore` before the disaster.

## Recovery Procedure

### 1. Stop Shoot Reconciliations on the Seed

Before making any changes, prevent gardenlet from reconciling Shoots while you prepare the restore. Annotate the Seed resource:

```bash
kubectl -n garden annotate seed <seed-name> shoot.gardener.cloud/emergency-stop-reconciliations=true
```

This blocks gardenlet from picking up new Shoot reconciliations until you lift the annotation.

### 2. Annotate Each Affected Shoot with `force-restore`

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

### 3. Lift the Emergency Stop

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

### 4. Monitor Progress

Watch the Shoot status for each cluster:

```bash
kubectl -n <project-namespace> get shoot <shoot-name> -w
```

A Shoot in restore phase will have `status.lastOperation.type: Restore`.

## Notes

- **Generation bump on annotation**: the Gardener API server bumps the Shoot generation when `force-restore` is set. gardenlet's `GenerationChangedPredicate` will therefore trigger a reconciliation without requiring any spec change.
- **Annotation removal**: gardenlet removes the `force-restore` annotation automatically once etcd is ready. No manual cleanup is needed.
- **Self-hosted Shoots**: `force-restore` is not applicable to self-hosted Shoots (`IsSelfHosted() == true`). These follow a different recovery path.
