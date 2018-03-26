#!/bin/bash

kubectl create -f setup.yaml
kubectl create -f node.yaml
kubectl create -f controller.yaml
