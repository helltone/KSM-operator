#!/bin/bash

set -e

CLUSTER_NAME="state-monitor-dev"
NAMESPACE="monitoring"
IMAGE_NAME="state-monitor-operator"
IMAGE_TAG="dev"

echo "🔧 Setting up kind cluster for development..."

if ! command -v kind &> /dev/null; then
    echo "❌ kind is not installed. Please install kind first."
    echo "   Visit: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
    exit 1
fi

if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed. Please install kubectl first."
    exit 1
fi

if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    echo "✅ Cluster '${CLUSTER_NAME}' already exists"
else
    echo "📦 Creating kind cluster..."
    kind create cluster --config config/kind/cluster.yaml
fi

echo "🔄 Setting kubectl context..."
kubectl cluster-info --context kind-${CLUSTER_NAME}

echo "📁 Creating monitoring namespace..."
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

echo "✅ Kind cluster setup complete!"
echo ""
echo "Next steps:"
echo "  1. Build the operator image: make docker-build-kind"
echo "  2. Deploy the operator: make deploy-kind"
echo "  3. Create a sample StateMonitor: kubectl apply -f manifests/sample-statemonitor-dev.yaml"
echo ""
echo "To delete the cluster: kind delete cluster --name ${CLUSTER_NAME}"