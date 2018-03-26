#!/bin/bash

kubectl delete -f demo-pod.yaml
kubectl delete -f node.yaml
kubectl delete -f controller.yaml
kubectl delete -f setup.yaml

