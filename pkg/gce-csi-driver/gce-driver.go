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

package gceGCEDriver

import (
	"github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"fmt"
	"github.com/container-storage-interface/spec/lib/go/csi"
	gce "github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver/pkg/gce-cloud-provider"
)

type GCEDriver struct{
	name    string
	nodeID  string

	ids *GCEIdentityServer
	ns  *GCENodeServer
	cs  *GCEControllerServer

	version *csi.Version
	supVers []*csi.Version

	vcap   []*csi.VolumeCapability_AccessMode
	cscap []*csi.ControllerServiceCapability
}

func GetGCEDriver() *GCEDriver {
	return &GCEDriver{}
}

func (gceDriver *GCEDriver) SetupGCEDriver(name string, v *csi.Version, supVers []*csi.Version, nodeID, project string) error{
	if name == "" {
		return fmt.Errorf("Driver name missing")
	}
	if nodeID == "" {
		return fmt.Errorf("NodeID missing")
	}
	if v == nil {
		return fmt.Errorf("Version argument missing")
	}
	if project == ""{
		return fmt.Errorf("Project missing")
	}

	found := false
	for _, sv := range supVers {
		if sv.Major == v.Major && sv.Minor == v.Minor && sv.Patch == v.Patch {
			found = true
		}
	}
	if !found {
		supVers = append(supVers, v)
	}

	gceDriver.name = name
	gceDriver.version = v
	gceDriver.supVers = supVers
	gceDriver.nodeID = nodeID

	// Adding Capabilities
	vcam := []csi.VolumeCapability_AccessMode_Mode{
		csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
	}
	gceDriver.AddVolumeCapabilityAccessModes(vcam)
	csc := []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
	}
	gceDriver.AddControllerServiceCapabilities(csc)

	// Set up RPC Servers
	gceDriver.ids = NewIdentityServer(gceDriver)
	gceDriver.ns = NewNodeServer(gceDriver)

	cloudProvider, err := gce.CreateCloudProvider()
	if err != nil{
		return err
	}
	gceDriver.cs = NewControllerServer(gceDriver, cloudProvider)

	
	return nil
}

func (gceDriver *GCEDriver) AddVolumeCapabilityAccessModes(vc []csi.VolumeCapability_AccessMode_Mode) error {
	var vca []*csi.VolumeCapability_AccessMode
	for _, c := range vc {
		glog.Infof("Enabling volume access mode: %v", c.String())
		vca = append(vca, NewVolumeCapabilityAccessMode(c))
	}
	gceDriver.vcap = vca
	return nil
}

func (gceDriver *GCEDriver) AddControllerServiceCapabilities(cl []csi.ControllerServiceCapability_RPC_Type) error{
	var csc []*csi.ControllerServiceCapability
	for _, c := range cl {
		glog.Infof("Enabling controller service capability: %v", c.String())
		csc = append(csc, NewControllerServiceCapability(c))
	}
	gceDriver.cscap = csc
	return nil
}

func (gceDriver *GCEDriver) ValidateControllerServiceRequest(v *csi.Version, c csi.ControllerServiceCapability_RPC_Type) error {
	if v == nil {
		return status.Error(codes.InvalidArgument, "Version not specified")
	}

	if err := gceDriver.CheckVersion(v); err != nil {
		return status.Error(codes.InvalidArgument, "Unsupported version")
	}

	if c == csi.ControllerServiceCapability_RPC_UNKNOWN {
		return nil
	}

	for _, cap := range gceDriver.cscap {
		if c == cap.GetRpc().Type {
			return nil
		}
	}

	return status.Error(codes.InvalidArgument, "Unsupported version: "+GetVersionString(v))
}

func NewIdentityServer(gceDriver *GCEDriver) *GCEIdentityServer {
	return &GCEIdentityServer{
		Driver: gceDriver,
	}
}

func NewNodeServer(gceDriver *GCEDriver) *GCENodeServer{
	return &GCENodeServer{
		Driver: gceDriver,
	}
}

func NewControllerServer(gceDriver *GCEDriver, cloudProvider *gce.CloudProvider) *GCEControllerServer {
	return &GCEControllerServer{
		Driver: gceDriver,
		CloudProvider: cloudProvider,
	}
}

func (gceDriver *GCEDriver) CheckVersion(v *csi.Version) error {
	if v == nil {
		return status.Error(codes.InvalidArgument, "Version missing")
	}

	// DONT assume always backward compatible
	for _, sv := range gceDriver.supVers {
		if v.Major == sv.Major && v.Minor == sv.Minor {
			return nil
		}
	}

	return status.Error(codes.InvalidArgument, "Unsupported version: "+GetVersionString(v))
}

func (gceDriver *GCEDriver) Run(endpoint string){
	glog.Infof("Driver: %v version: %v", gceDriver.name, GetVersionString(gceDriver.version))

	//Start the nonblocking GRPC
	s := NewNonBlockingGRPCServer()
	// TODO: Only start specific servers based on a flag.
	// In the future have this only run specific combinations of servers depending on which version this is.
	// The schema for that was in util. basically it was just s.start but with some nil servers.
	s.Start(endpoint, gceDriver.ids, gceDriver.cs, gceDriver.ns)
	s.Wait()
}

