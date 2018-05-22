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

package e2e

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver/pkg/mount-manager"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/uuid"

	gce "github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver/pkg/gce-cloud-provider"
	driver "github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver/pkg/gce-csi-driver"
	csipb "github.com/container-storage-interface/spec/lib/go/csi/v0"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	endpoint       = "unix://tmp/csi.sock"
	addr           = "/tmp/csi.sock"
	network        = "unix"
	testNamePrefix = "gcepd-csi-e2e-"

	defaultSizeGb    int64 = 5
	readyState             = "READY"
	standardDiskType       = "pd-standard"
	ssdDiskType            = "pd-ssd"
)

var (
	client   *csiClient
	gceCloud *gce.CloudProvider
	nodeID   string
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Google Compute Engine Persistent Disk Container Storage Interface Driver Tests")
}

var _ = BeforeSuite(func() {
	// TODO(dyzz): better defaults
	driverName := "testdriver"
	nodeID = "gce-pd-csi-e2e"

	// TODO(dyzz): Start a driver
	gceDriver := driver.GetGCEDriver()
	cloudProvider, err := gce.CreateCloudProvider()

	Expect(err).To(BeNil(), "Failed to get cloud provider: %v", err)

	// TODO(dyzz): Change this to a fake mounter
	mounter, err := mountmanager.CreateMounter()

	Expect(err).To(BeNil(), "Failed to get mounter %v", err)

	//Initialize GCE Driver
	err = gceDriver.SetupGCEDriver(cloudProvider, mounter, driverName, nodeID)
	Expect(err).To(BeNil(), "Failed to initialize GCE CSI Driver: %v", err)

	go func() {
		gceDriver.Run(endpoint)
	}()

	client = createCSIClient()

	gceCloud, err = gce.CreateCloudProvider()
	Expect(err).To(BeNil(), "Failed to create cloud service")
	// TODO: This is a hack to make sure the driver is fully up before running the tests, theres probably a better way to do this.
	time.Sleep(20 * time.Second)
})

var _ = AfterSuite(func() {
	// Close the client
	err := client.conn.Close()
	if err != nil {
		Logf("Failed to close the client")
	} else {
		Logf("Closed the client")
	}
	// TODO(dyzz): Clean up driver and other things
})

var _ = Describe("GCE PD CSI Driver", func() {
	var (
		stdVolCap = &csipb.VolumeCapability{
			AccessType: &csipb.VolumeCapability_Mount{
				Mount: &csipb.VolumeCapability_MountVolume{},
			},
			AccessMode: &csipb.VolumeCapability_AccessMode{
				Mode: csipb.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		}
		stdVolCaps = []*csipb.VolumeCapability{
			stdVolCap,
		}
	)

	BeforeEach(func() {
		err := client.assertCSIConnection()
		Expect(err).To(BeNil(), "Failed to assert csi client connection: %v", err)
	})

	It("Should create and delete a default volume successfully", func() {
		// Create Disk
		volName := testNamePrefix + string(uuid.NewUUID())
		cvr := &csipb.CreateVolumeRequest{
			Name:               volName,
			VolumeCapabilities: stdVolCaps,
		}
		cresp, err := client.ctrlClient.CreateVolume(context.Background(), cvr)
		Expect(err).To(BeNil(), "CreateVolume failed with error: %v", err)
		volId := cresp.GetVolume().GetId()

		// Validate Disk Created
		cloudDisk, err := gceCloud.GetDiskOrError(context.Background(), gceCloud.GetZone(), volName)
		Expect(err).To(BeNil(), "Could not get disk from cloud directly")
		Expect(cloudDisk.Type).To(ContainSubstring(standardDiskType))
		Expect(cloudDisk.Status).To(Equal(readyState))
		Expect(cloudDisk.SizeGb).To(Equal(defaultSizeGb))
		Expect(cloudDisk.Name).To(Equal(volName))

		// Attach Disk
		cpreq := &csipb.ControllerPublishVolumeRequest{
			VolumeId:         volId,
			NodeId:           nodeID,
			VolumeCapability: stdVolCap,
			Readonly:         false,
		}
		_, err = client.ctrlClient.ControllerPublishVolume(context.Background(), cpreq)
		Expect(err).To(BeNil(), "ControllerPublishVolume failed with error")

		// TODO: Verify Attachment

		// Stage Disk
		stageDir := filepath.Join("/tmp/", volName, "stage")
		nodeStageReq := &csipb.NodeStageVolumeRequest{
			VolumeId:          volId,
			StagingTargetPath: stageDir,
			VolumeCapability:  stdVolCap,
		}
		_, err = client.nodeClient.NodeStageVolume(context.Background(), nodeStageReq)
		Expect(err).To(BeNil(), "NodeStageVolume failed with error")

		// TODO: Verify Staged

		// Mount Disk
		publishDir := filepath.Join("/tmp/", volName, "mount")
		nodePublishReq := &csipb.NodePublishVolumeRequest{
			VolumeId:          volId,
			StagingTargetPath: stageDir,
			TargetPath:        publishDir,
			VolumeCapability:  stdVolCap,
			Readonly:          false,
		}
		_, err = client.nodeClient.NodePublishVolume(context.Background(), nodePublishReq)
		Expect(err).To(BeNil(), "NodePublishVolume failed with error")

		// TODO: Validate Mount disk

		// Unmount Disk
		nodeUnpublishReq := &csipb.NodeUnpublishVolumeRequest{
			VolumeId:   volId,
			TargetPath: publishDir,
		}
		_, err = client.nodeClient.NodeUnpublishVolume(context.Background(), nodeUnpublishReq)
		Expect(err).To(BeNil(), "NodeUnpublishVolume failed with error")

		// TODO: Validate Unmount disk.

		// Unstage Disk
		nodeUnstageReq := &csipb.NodeUnstageVolumeRequest{
			VolumeId:          volId,
			StagingTargetPath: stageDir,
		}
		_, err = client.nodeClient.NodeUnstageVolume(context.Background(), nodeUnstageReq)
		Expect(err).To(BeNil(), "NodeUnstageVolume failed with error")

		// TODO: Verify Unstaged

		// Detatch Disk
		cupreq := &csipb.ControllerUnpublishVolumeRequest{
			VolumeId: volId,
			NodeId:   nodeID,
		}
		_, err = client.ctrlClient.ControllerUnpublishVolume(context.Background(), cupreq)
		Expect(err).To(BeNil(), "ControllerUnpublishVolume failed with error")

		// TODO: Verify Detatchment

		// Delete Disk
		dvr := &csipb.DeleteVolumeRequest{
			VolumeId: volId,
		}
		_, err = client.ctrlClient.DeleteVolume(context.Background(), dvr)
		Expect(err).To(BeNil(), "DeleteVolume failed")

		// Validate Disk Deleted
		_, err = gceCloud.GetDiskOrError(context.Background(), gceCloud.GetZone(), volName)
		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.NotFound))
	})

	It("SHould RUN AND FAIL", func() {
		Expect("hello").To(Equal("world"))
	})

})

func Logf(format string, args ...interface{}) {
	fmt.Fprint(GinkgoWriter, args...)
}

type csiClient struct {
	conn       *grpc.ClientConn
	idClient   csipb.IdentityClient
	nodeClient csipb.NodeClient
	ctrlClient csipb.ControllerClient
}

func createCSIClient() *csiClient {
	return &csiClient{}
}

func (c *csiClient) assertCSIConnection() error {
	if c.conn == nil {
		conn, err := grpc.Dial(
			addr,
			grpc.WithInsecure(),
			grpc.WithDialer(func(target string, timeout time.Duration) (net.Conn, error) {
				return net.Dial(network, target)
			}),
		)
		if err != nil {
			return err
		}
		c.conn = conn
		c.idClient = csipb.NewIdentityClient(conn)
		c.nodeClient = csipb.NewNodeClient(conn)
		c.ctrlClient = csipb.NewControllerClient(conn)
	}
	return nil
}
