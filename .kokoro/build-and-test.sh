#!/bin/bash

set -e
set -x

readonly PKGDIR=github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver

cd ${GOROOT}
mkdir -p ${PKGDIR}/bin
go build -o ${PKGDIR}/bin/gce-csi-driver ${PKGDIR}/cmd
go test -timeout 30s ${PKGDIR}/test/sanity/ -run ^TestSanity$