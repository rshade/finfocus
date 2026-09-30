#!/usr/bin/env bash
# Create a kind cluster whose nodes look like us-east-1 m5.large EC2 instances.
set -euo pipefail
CLUSTER=${KIND_CLUSTER:-finfocus-e2e}
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! kind get clusters | grep -qx "$CLUSTER"; then
  kind create cluster --name "$CLUSTER" --wait 120s
fi
kubectl --context "kind-$CLUSTER" label nodes --all --overwrite \
  node.kubernetes.io/instance-type=m5.large \
  topology.kubernetes.io/region=us-east-1 \
  finfocus.dev/provider=aws
kubectl --context "kind-$CLUSTER" apply -f "$DIR/workloads.yaml"
kubectl --context "kind-$CLUSTER" -n e2e rollout status deployment/web --timeout=120s
kubectl --context "kind-$CLUSTER" -n e2e rollout status statefulset/db --timeout=120s
kubectl --context "kind-$CLUSTER" -n e2e rollout status daemonset/agent --timeout=120s
