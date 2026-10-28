#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

set -o nounset
set -o pipefail
set -o errexit

source "$(dirname "${0}")/ci-common.sh"

clamp_mss_to_pmtu

# The local2 gardenlet must run with the force-restore overlay (ShootState controller enabled via
# shootState.concurrentSyncs=2) so that a current ShootState exists to restore from and is not deleted
# after the restore. GARDENLET_PROFILE_SUFFIX is honored by dev-setup/scenario.sh for the single-node2
# scenario; the e2e spec passes the same value for the seed2-up it runs after recreating the node.
export GARDENLET_PROFILE_SUFFIX=force-restore

# test setup
make kind-up
make kind2-up

# export all container logs and events after test execution
trap "
  ( export_artifacts_host_services; export_artifacts_infra; export_artifacts_load_balancers )
  ( export KUBECONFIG=$KUBECONFIG_RUNTIME_CLUSTER; export_artifacts 'gardener-local'; export_resource_yamls_for garden )
  ( export KUBECONFIG=$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER; export cluster_name='virtual-garden'; export_resource_yamls_for seeds shoots bastions.operations.gardener.cloud etcds leases; export_events_for_shoots )
  ( export KUBECONFIG=$KUBECONFIG_SEED2_CLUSTER; export_artifacts 'gardener-local2' )
  ( make seed2-down )
  ( make gardener-down )
  ( make kind2-down )
  ( make kind-down )
" EXIT

make gardener-up
make seed2-up

make test-e2e-local-force-restore
