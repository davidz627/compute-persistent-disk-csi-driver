#!/bin/bash

set -e
set -x

readonly GCPDIR=${GOROOT}/src/github.com/GoogleCloudPlatform

mkdir -p ${GCPDIR}
mv github/compute-persistent-disk-csi-driver ${GCPDIR}
cd ${GCPDIR}
compute-persistent-disk-csi-driver/.kokoro/build-and-test.sh