#!/bin/bash

export NUM_NODES=2
export KUBE_RUNTIME_CONFIG="storage.k8s.io/v1alpha1=true"
export KUBE_FEATURE_GATES="CSIPersistentVolume=true"

${GOPATH}/src/k8s.io/kubernetes/cluster/kube-up.sh
