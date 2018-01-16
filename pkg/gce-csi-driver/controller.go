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

//TODO make error messages better, of form "{Call}{args} error: {error}"
//TODO all functions should actually have real return values according to spec

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	compute "google.golang.org/api/compute/v1"
	"strings"
	"fmt"
)

type GCEControllerServer struct {
	Driver *GCEDriver
}

var(
	//TODO Check default size
	DefaultVolumeSize uint64 = 5000000000
)

func getRequestCapacity(capRange *csi.CapacityRange) (capBytes uint64){
	//TODO(dyzz): take another look at these casts/caps
	if tcap := capRange.GetRequiredBytes(); tcap > 0{
		capBytes = tcap
	} else if tcap = capRange.GetLimitBytes(); tcap > 0{
		capBytes = tcap
	} else{
		// Default size
		capBytes = DefaultVolumeSize
	}
	return
}

func (gceCS *GCEControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	//TODO: Get project from request
	var project string = "dyzz-test"

	glog.Infof("CreateVolume called with request %v", *req)

	// Check arguments
	if req.GetVersion() == nil {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Version must be provided")
	}
	if len(req.GetName()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Name must be provided")
	}
	if req.GetVolumeCapabilities() == nil || len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Volume capabilities must be provided")
	}
	if req.GetCapacityRange() == nil {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume CapacityRange must be specified")
	}
	if req.GetCapacityRange().GetRequiredBytes() == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Capacity range required bytes cannot be zero")
	}

	// TODO: validate volume capabilities

	svc := gceCS.Driver.cloudService

	capBytes := getRequestCapacity(req.GetCapacityRange())

	// TODO: Support replica zones and fs type. Can vendor in api-machinery stuff for sets etc.
	// Apply Parameters (case-insensitive). We leave validation of
	// the values to the cloud provider.
	diskType := ""
	configuredZone := ""
	zonePresent := false
	/*
	configuredZones := ""
	configuredReplicaZones := ""
	zonesPresent := false
	replicaZonesPresent := false
	fstype := ""
	*/
	for k, v := range req.GetParameters() {
		switch strings.ToLower(k) {
		case "type":
			glog.Infof("Setting type: %v", v)
			diskType = v
		case "zone":
			zonePresent = true
			configuredZone = v
		/*
		case "zones":
			zonesPresent = true
			configuredZones = v
		case "replica-zones":
			replicaZonesPresent = true
			configuredReplicaZones = v
		case "fstype":
			fstype = v
		*/
		default:
			glog.Errorf("invalid option %q", k)
			return nil, fmt.Errorf("some sort of error has happened with options")
			//return "", 0, nil, "", fmt.Errorf("invalid option %q for volume plugin %s", k, c.plugin.GetPluginName())
		}
	}

	if !zonePresent{
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Zone must be specified")
	}

	if diskType == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume DiskType must be specified")
	}

	//TODO: validate that volume has not already been created or is in process of creation

	diskToCreate := &compute.Disk{
		Name:        req.GetName(),
		SizeGb:      BytesToGB(capBytes),
		//TODO: Is this description important for anything
		Description: "PD Created by CSI Driver",
		Type:        getDiskType(project, configuredZone, diskType),
	}
	_, err := svc.Disks.Insert(project, configuredZone, diskToCreate).Context(ctx).Do()
	if (err != nil){
		//TODO Probably need to case out different types of creation errors and give different error codes for each
		glog.Errorf("Some creation error: %v", err)
	}

	resp := &csi.CreateVolumeResponse{
		VolumeInfo: &csi.VolumeInfo{
			CapacityBytes: capBytes,
			Id: combineVolumeId(project, configuredZone, req.GetName()),
			//TODO: what are attributes for
			Attributes: nil,
		},
	}
	return resp, nil
}

func getDiskType(project, zone, diskType string) string{
	return fmt.Sprintf("projects/%s/zones/%s/diskTypes/%s", project, zone, diskType)
}

func combineVolumeId(project, zone, name string) string{
	return fmt.Sprintf("%s/%s/%s", project, zone, name)
}

func (gceCS *GCEControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	// Assuming ID is of form {project}/{zone}/{id}
	glog.Infof("DeleteVolume called with request %v", *req)
	svc := gceCS.Driver.cloudService

	project, zone, name, err := splitVolumeId(req.GetVolumeId())
	if err != nil{
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("DeleteVolume error: %v", err))
	}

	_, err = svc.Disks.Delete(project, zone, name).Context(ctx).Do()

	if err != nil{
		//TODO probably case out on different types of errors for different grpc error
		glog.Errorf("DeleteVolume error: %v", err)
	}

	return nil, nil
}

func splitVolumeId(volumeId string) (string, string, string, error){
	splitId := strings.Split(volumeId, "/")
	if len(splitId) != 3{
		return "","","",fmt.Errorf("Failed to get id components. Expected {project}/{zone}/{name}. Got: %s", volumeId)
	}
	return splitId[0], splitId[1], splitId[2], nil
}

func (gceCS *GCEControllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (gceCS *GCEControllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (gceCS *GCEControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	glog.V(5).Infof("Using default ValidateVolumeCapabilities")

	for _, c := range req.GetVolumeCapabilities() {
		found := false
		for _, c1 := range gceCS.Driver.vcap {
			if c1.GetMode() == c.GetAccessMode().GetMode() {
				found = true
			}
		}
		if !found {
			return &csi.ValidateVolumeCapabilitiesResponse{
				Supported: false,
				Message:   "Driver doesnot support mode:" + c.GetAccessMode().GetMode().String(),
			}, status.Error(codes.InvalidArgument, "Driver doesnot support mode:"+c.GetAccessMode().GetMode().String())
		}
		// TODO: Ignoring mount & block tyeps for now.
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Supported: true,
	}, nil
}

func (gceCS *GCEControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (gceCS *GCEControllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (gceCS *GCEControllerServer) ControllerProbe(ctx context.Context, req *csi.ControllerProbeRequest) (*csi.ControllerProbeResponse, error) {
	glog.V(5).Infof("Using default ControllerProbe")

	if err := gceCS.Driver.ValidateControllerServiceRequest(req.Version, csi.ControllerServiceCapability_RPC_UNKNOWN); err != nil {
		return nil, err
	}
	return &csi.ControllerProbeResponse{}, nil
}

// ControllerGetCapabilities implements the default GRPC callout.
// Default supports all capabilities
func (gceCS *GCEControllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	glog.V(5).Infof("Using default ControllerGetCapabilities")

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: gceCS.Driver.cscap,
	}, nil
}