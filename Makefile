# Copyright 2018 Google Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

STAGINGIMAGE=gcr.io/dyzz-csi-staging/csi/gce-driver
STAGINGVERSION=latest

PRODIMAGE=gcr.io/google-containers/volume-csi/compute-persistent-disk-csi-driver
PRODVERSION=v0.2.0.alpha
all: gce-driver

gce-driver:
	mkdir -p bin
	go build -o bin/gce-csi-driver ./cmd/
	go build -o bin/gce-csi-driver-test ./test/e2e/

build-container: gce-driver
	cp bin/gce-csi-driver deploy/docker
	docker build -t $(STAGINGIMAGE):$(STAGINGVERSION) deploy/docker

push-container: build-container
	gcloud docker -- push $(STAGINGIMAGE):$(STAGINGVERSION)

prod-build-container: gce-driver
	cp bin/gce-csi-driver deploy/docker
	docker build -t $(PRODIMAGE):$(PRODVERSION) deploy/docker

prod-push-container: prod-build-container
	gcloud docker -- push $(PRODIMAGE):$(PRODVERSION)

test-sanity: gce-driver
	go test -timeout 30s github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver/pkg/test -run ^TestSanity$
