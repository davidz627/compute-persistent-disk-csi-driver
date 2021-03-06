/*
Copyright 2018 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sanitytest

import (
	"testing"

	"github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver/pkg/mount-manager"

	gce "github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver/pkg/gce-cloud-provider"
	driver "github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver/pkg/gce-csi-driver"
	sanity "github.com/kubernetes-csi/csi-test/pkg/sanity"
	compute "google.golang.org/api/compute/v1"
)

func TestSanity(t *testing.T) {
	// Set up variables
	driverName := "test-driver"
	nodeID := "io.kubernetes.storage.mock"
	project := "test-project"
	zone := "test-zone"
	// TODO(dyzz): Only one of these can be correct, the way endpoint is defined in GCE driver is INCORRECT
	endpoint := "unix://tmp/csi.sock"
	csiSanityEndpoint := "unix:/tmp/csi.sock"
	mountPath := "/tmp/csi/mount"
	stagePath := "/tmp/csi/stage"
	// Set up driver and env
	gceDriver := driver.GetGCEDriver()

	cloudProvider, err := gce.FakeCreateCloudProvider(project, zone)
	if err != nil {
		t.Fatalf("Failed to get cloud provider: %v", err)
	}

	// TODO(dyzz): Change this to a fake mounter
	mounter, err := mountmanager.CreateFakeMounter()
	if err != nil {
		t.Fatalf("Failed to get mounter %v", err)
	}

	//Initialize GCE Driver
	err = gceDriver.SetupGCEDriver(cloudProvider, mounter, driverName, nodeID)
	if err != nil {
		t.Fatalf("Failed to initialize GCE CSI Driver: %v", err)
	}

	instance := &compute.Instance{
		Name:  nodeID,
		Disks: []*compute.AttachedDisk{},
	}
	cloudProvider.InsertInstance(instance, nodeID)

	go func() {
		gceDriver.Run(endpoint)
	}()

	// Run test
	config := &sanity.Config{
		TargetPath:  mountPath,
		StagingPath: stagePath,
		Address:     csiSanityEndpoint,
	}
	sanity.Test(t, config)

}
