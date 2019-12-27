#!/bin/bash

BASE_FOLDER="/var/run/secrets/kubernetes.io/serviceaccount"
KUBECFG_FILE_NAME=$1
context="kubernetes-admin@kubernetes"
CLUSTER_NAME="kubernetes"
ENDPOINT="https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_PORT_443_TCP_PORT"
kubectl config set-cluster "${CLUSTER_NAME}" \
--kubeconfig="${KUBECFG_FILE_NAME}" \
--server="${ENDPOINT}" \
--certificate-authority="${BASE_FOLDER}/ca.crt" \
--embed-certs=true
USER_TOKEN=$(cat $BASE_FOLDER/token)
NAMESPACE=$(cat $BASE_FOLDER/namespace)
kubectl config set-credentials \
    "sa-${NAMESPACE}-${CLUSTER_NAME}" \
    --kubeconfig="${KUBECFG_FILE_NAME}" \
    --token="${USER_TOKEN}"
kubectl config set-context \
    "sa-${NAMESPACE}-${CLUSTER_NAME}" \
    --kubeconfig="${KUBECFG_FILE_NAME}" \
    --cluster="${CLUSTER_NAME}" \
    --user="sa-${NAMESPACE}-${CLUSTER_NAME}" \
    --namespace="${NAMESPACE}"
kubectl config use-context "sa-${NAMESPACE}-${CLUSTER_NAME}" \
    --kubeconfig="${KUBECFG_FILE_NAME}"
