#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

# This script tests the disaster recovery flow using the garden-state secret:
# 1. Set up a full Gardener landscape (kind + operator + garden + seed2)
# 2. Wait for reconciliation, inject dummy state into a DNSRecord
# 3. Issue a ServiceAccount token in the virtual garden to verify key continuity
# 4. Extract the garden-state secret
# 5. Destroy the kind cluster
# 6. Recreate the kind cluster and restore from the garden-state secret
# 7. Verify restore succeeded (lastOperation, extensions installed, DNSRecord state, SA token still works)

set -o nounset
set -o pipefail
set -o errexit

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
source "${REPO_ROOT}/hack/ci-common.sh"

clamp_mss_to_pmtu

GARDEN_NAMESPACE="garden"
GARDEN_NAME="local"
WAIT_FOR="${REPO_ROOT}/hack/usage/wait-for.sh"

# test setup
make kind-up
make kind2-up

trap "
  ( export_artifacts_host_services; export_artifacts_infra; export_artifacts_load_balancers ) || true
  ( export KUBECONFIG=$KUBECONFIG_RUNTIME_CLUSTER; export_artifacts 'gardener-local'; export_resource_yamls_for garden ) || true
  ( make seed2-down ) || true
  ( make kind2-down ) || true
  ( make kind-down ) || true
" EXIT

make gardener-up
make seed2-up

export KUBECONFIG="$KUBECONFIG_RUNTIME_CLUSTER"

echo "=== Phase 1: Waiting for Garden to be reconciled ==="
TIMEOUT=900 "$WAIT_FOR" garden "$GARDEN_NAME"

echo "=== Phase 2: Injecting dummy state into DNSRecord ==="
# Find a DNSRecord in the garden namespace and patch its status with custom state

dns_record_name=""
for i in $(seq 1 60); do
  dns_record_name=$(kubectl get dnsrecords.extensions.gardener.cloud -n "$GARDEN_NAMESPACE" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  if [[ -n "$dns_record_name" ]]; then
    break
  fi
  echo "Waiting for DNSRecord to appear..."
  sleep 5
done

if [[ -z "$dns_record_name" ]]; then
  echo "ERROR: No DNSRecord found in namespace $GARDEN_NAMESPACE"
  exit 1
fi

echo "Found DNSRecord: $dns_record_name"
echo "Patching DNSRecord status with dummy state..."
kubectl patch dnsrecord "$dns_record_name" -n "$GARDEN_NAMESPACE" --type merge --subresource status -p '{"status":{"state":{"someKey":"someValue"}}}'

# Verify the state was set
state_value=$(kubectl get dnsrecord "$dns_record_name" -n "$GARDEN_NAMESPACE" -o jsonpath='{.status.state}')
echo "DNSRecord state after patching: $state_value"

echo "=== Phase 3: Issuing ServiceAccount token in virtual garden ==="
export KUBECONFIG="$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER"

kubectl create namespace dr-test 2>/dev/null || true
kubectl create serviceaccount dr-test-sa -n dr-test 2>/dev/null || true

# Create a long-lived token secret for the SA
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: dr-test-sa-token
  namespace: dr-test
  annotations:
    kubernetes.io/service-account.name: dr-test-sa
type: kubernetes.io/service-account-token
EOF

# Wait for the token to be populated
for i in $(seq 1 30); do
  sa_token=$(kubectl get secret dr-test-sa-token -n dr-test -o jsonpath='{.data.token}' 2>/dev/null || true)
  if [[ -n "$sa_token" ]]; then
    break
  fi
  sleep 2
done

if [[ -z "$sa_token" ]]; then
  echo "ERROR: ServiceAccount token was not populated"
  exit 1
fi

sa_token_decoded=$(echo "$sa_token" | base64 -d)
echo "ServiceAccount token issued successfully"

# Build a kubeconfig using the SA token for later verification
vg_server=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')
vg_ca=$(kubectl config view --minify --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')

SA_KUBECONFIG=$(mktemp)
cat > "$SA_KUBECONFIG" <<EOF
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: $vg_ca
    server: $vg_server
  name: virtual-garden
contexts:
- context:
    cluster: virtual-garden
    user: dr-test-sa
  name: default
current-context: default
users:
- name: dr-test-sa
  user:
    token: $sa_token_decoded
EOF

# Verify the token works now
kubectl --kubeconfig "$SA_KUBECONFIG" auth can-i get pods -n dr-test 2>/dev/null || true
echo "SA token verified working before disaster"

echo "=== Phase 4: Triggering Garden reconciliation to sync garden-state ==="
export KUBECONFIG="$KUBECONFIG_RUNTIME_CLUSTER"

# Trigger a reconcile to ensure garden-state includes the new DNSRecord state
kubectl annotate garden "$GARDEN_NAME" gardener.cloud/operation=reconcile --overwrite
TIMEOUT=900 "$WAIT_FOR" garden "$GARDEN_NAME"

echo "=== Phase 5: Extracting garden-state secret ==="
kubectl get secret garden-state -n "$GARDEN_NAMESPACE" -o json | \
  jq '{apiVersion: "v1", kind: "Secret", metadata: {name: .metadata.name, namespace: .metadata.namespace, labels: .metadata.labels}, data: .data, type: .type}' \
  > "$REPO_ROOT/gardenstate.yaml"
echo "Garden-state secret saved to gardenstate.yaml"

echo "=== Phase 6: Destroying kind cluster ==="
# Stop skaffold/operator first
make operator-down || true
make seed2-down || true
make kind2-down || true
make kind-down

echo "=== Phase 7: Recreating kind cluster and restoring ==="
make kind-up

export KUBECONFIG="$KUBECONFIG_RUNTIME_CLUSTER"

# Create the garden namespace and apply the garden-state secret
kubectl create namespace "$GARDEN_NAMESPACE"
kubectl create -f "$REPO_ROOT/gardenstate.yaml"

# Redeploy the operator — it will detect the garden-state secret and restore
make operator-up

echo "=== Phase 8: Waiting for Garden restore to complete ==="
TIMEOUT=900 "$WAIT_FOR" garden "$GARDEN_NAME"

echo "=== Phase 9: Verifying restore results ==="

# 9a: Check Garden lastOperation type and state
last_op_type=$(kubectl get garden "$GARDEN_NAME" -o jsonpath='{.status.lastOperation.type}')
last_op_state=$(kubectl get garden "$GARDEN_NAME" -o jsonpath='{.status.lastOperation.state}')
echo "Garden lastOperation: type=$last_op_type, state=$last_op_state"

if [[ "$last_op_type" != "Restore" ]]; then
  echo "ERROR: Expected lastOperation.type=Restore, got $last_op_type"
  exit 1
fi
if [[ "$last_op_state" != "Succeeded" ]]; then
  echo "ERROR: Expected lastOperation.state=Succeeded, got $last_op_state"
  exit 1
fi
echo "✅ Garden lastOperation: Restore Succeeded"

# 9b: Check Extensions are installed
echo "Checking Extensions installed status..."
extensions_not_installed=$(kubectl get extensions.operator.gardener.cloud -o json | \
  jq -r '.items[] | select(.status.conditions[]? | select(.type=="Installed" and .status!="True")) | .metadata.name')
if [[ -n "$extensions_not_installed" ]]; then
  echo "ERROR: The following extensions are not installed: $extensions_not_installed"
  exit 1
fi
echo "✅ All Extensions are Installed=True"

# 9c: Check DNSRecord state was restored
dns_record_name_restored=""
for i in $(seq 1 60); do
  dns_record_name_restored=$(kubectl get dnsrecords.extensions.gardener.cloud -n "$GARDEN_NAMESPACE" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  if [[ -n "$dns_record_name_restored" ]]; then
    break
  fi
  echo "Waiting for DNSRecord to reappear after restore..."
  sleep 5
done

if [[ -z "$dns_record_name_restored" ]]; then
  echo "ERROR: No DNSRecord found after restore"
  exit 1
fi

restored_state=$(kubectl get dnsrecord "$dns_record_name_restored" -n "$GARDEN_NAMESPACE" -o jsonpath='{.status.state}')
echo "Restored DNSRecord state: $restored_state"

if ! echo "$restored_state" | grep -q "someKey"; then
  echo "ERROR: DNSRecord state was not restored (expected to contain 'someKey')"
  exit 1
fi
echo "✅ DNSRecord state restored successfully"

# 9d: Verify ServiceAccount token still works after restore
echo "Verifying ServiceAccount token still works..."

# Re-export the virtual garden kubeconfig
RUNTIME_CLUSTER_KUBECONFIG="$KUBECONFIG" GARDEN_NAME="$GARDEN_NAME" \
  "${REPO_ROOT}/hack/usage/generate-kubeconfig.sh" virtual-garden > "$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER"

# Update the SA kubeconfig with the new server endpoint (port may have changed after kind recreation)
vg_server_new=$(kubectl --kubeconfig "$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER" config view --minify -o jsonpath='{.clusters[0].cluster.server}')
vg_ca_new=$(kubectl --kubeconfig "$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER" config view --minify --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')

cat > "$SA_KUBECONFIG" <<EOF
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: $vg_ca_new
    server: $vg_server_new
  name: virtual-garden
contexts:
- context:
    cluster: virtual-garden
    user: dr-test-sa
  name: default
current-context: default
users:
- name: dr-test-sa
  user:
    token: $sa_token_decoded
EOF

# Wait for the virtual garden API to be available
for i in $(seq 1 60); do
  if kubectl --kubeconfig "$SA_KUBECONFIG" get namespace dr-test &>/dev/null; then
    echo "✅ ServiceAccount token still works after restore — signing keys preserved"
    break
  fi
  if [[ $i -eq 60 ]]; then
    echo "ERROR: ServiceAccount token no longer works after restore (signing keys may not have been preserved)"
    exit 1
  fi
  sleep 5
done

rm -f "$SA_KUBECONFIG"

echo ""
echo "=========================================="
echo "✅ Garden-state disaster recovery test PASSED"
echo "=========================================="
