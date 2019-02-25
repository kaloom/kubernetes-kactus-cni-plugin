#!/bin/sh

set -eo pipefail

config_dir=${1:-/tmp/kubeconfig}
kubecfg_file=${2:-kactus-kubeconfig.yaml}
namespace=${3:-kube-system}

if [ ! -d $config_dir ]; then
    mkdir -p $config_dir
fi

kubecfg_path=${config_dir}/${kubecfg_file}

# create the service account and RBAC permissions
kubectl apply -f kactus-serviceaccount-and-rbac.yaml
# get the secret name from the service account
secret_name=$(kubectl get sa kactus -n $namespace -o jsonpath="{.secrets[*].name}")
# extract the ca.crt from the secret
kubectl get secret $secret_name -n $namespace -o jsonpath="{.data['ca\.crt']}" | base64 -d > ${config_dir}/ca.crt
# extract the token from the secret
token=$(kubectl get secret $secret_name -n $namespace -o jsonpath="{.data['token']}" | base64 -d)

context=$(kubectl config current-context)
cluster_name=$(kubectl config get-contexts "$context" | awk '{print $3}' | tail -n 1)
endpoint=$(kubectl config view -o jsonpath="{.clusters[?(@.name == \"${cluster_name}\")].cluster.server}")

# Set up the config
kubectl config set-cluster "${cluster_name}" --kubeconfig="${kubecfg_path}" --server="${endpoint}" --certificate-authority="${config_dir}/ca.crt" --embed-certs=true

# Set token credentials entry in kubeconfig
kubectl config set-credentials "kactus-${cluster_name}" --kubeconfig="${kubecfg_path}" --token="${token}"

# Set a context entry in kubeconfig
kubectl config set-context "kactus-${cluster_name}" --kubeconfig="${kubecfg_path}" --cluster="${cluster_name}" --user="kactus-${cluster_name}"

# Set the current-context in the kubeconfig file
kubectl config use-context "kactus-${cluster_name}" --kubeconfig="${kubecfg_path}"
