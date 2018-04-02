#!/bin/bash

set -e
set -x

readonly ROOT=$(dirname "${BASH_SOURCE}")/..


mkdir -p ${ROOT}/bin
go build -o ${ROOT}/bin/gce-csi-driver ${ROOT}/cmd/
go test -timeout 30s ${ROOT}/pkg/test -run ^TestSanity$
