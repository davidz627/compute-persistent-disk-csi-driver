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
	gceprovider "github.com/davidz627/gce-csi-driver/pkg/gce-cloud-provider"
	"strings"
	"fmt"
)

type GCEControllerServer struct {
	Driver *GCEDriver
}

const (
	// MaxVolumeSize is the maximum standard and ssd size of 64TB
	MaxVolumeSize uint64 = 64000000000000
	DefaultVolumeSize uint64 = 5000000000

	DiskTypeSSD      = "pd-ssd"
	DiskTypeStandard = "pd-standard"

	diskTypeDefault               = DiskTypeStandard
	diskTypeURITemplateSingleZone = "projects/%s/zones/%s/diskTypes/%s"   // {gce.projectID}/zones/{disk.Zone}/diskTypes/{disk.Type}"
	diskSourceURITemplateSingleZone = "%s/zones/%s/disks/%s"   // {gce.projectID}/zones/{disk.Zone}/disks/{disk.Name}"
)

func getRequestCapacity(capRange *csi.CapacityRange) (capBytes uint64){
	//TODO(dyzz): take another look at these casts/caps
	if tcap := capRange.RequiredBytes; tcap > 0{
		capBytes = tcap
	} else if tcap = capRange.LimitBytes; tcap > 0{
		capBytes = tcap
	} else{
		// Default size
		capBytes = DefaultVolumeSize
	}
	return
}

func (gceCS *GCEControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	glog.Infof("CreateVolume called with request %v", *req)

	svc := gceCS.Driver.cloudService
	project := gceCS.Driver.project

	// Check arguments
	if req.GetVersion() == nil {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Version must be provided")
	}
	if len(req.Name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Name must be provided")
	}
	if req.GetVolumeCapabilities() == nil || len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Volume capabilities must be provided")
	}
	if req.GetCapacityRange() == nil {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume CapacityRange must be specified")
	}
	if req.GetCapacityRange().RequiredBytes == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Capacity range required bytes cannot be zero")
	}
	if req.GetCapacityRange().RequiredBytes > MaxVolumeSize {
		return nil, status.Error(codes.OutOfRange, fmt.Sprintf("CreateVolume Capacity range required bytes cannot be greater than %v", MaxVolumeSize))
	}

	// TODO: validate volume capabilities

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
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid option %q", k))
		}
	}

	if !zonePresent{
		return nil, status.Error(codes.InvalidArgument, "Zone must be specified")
	}

	if diskType == "" {
		return nil, status.Error(codes.InvalidArgument, "DiskType must be specified")
	}

	createResp := &csi.CreateVolumeResponse{
		VolumeInfo: &csi.VolumeInfo{
			CapacityBytes: capBytes,
			Id: combineVolumeId(gceCS.Driver.project, configuredZone, req.Name),
			//TODO: what are attributes for
			Attributes: nil,
		},
	}

	// Check for existing disk of same name in same zone
	exists, err := getAndValidateExistingDisk(svc, project, configuredZone,
		req.Name, diskType,
		req.GetCapacityRange().RequiredBytes,
		req.GetCapacityRange().LimitBytes, ctx)
	if err != nil{
		return nil, err
	}
	if exists{
		glog.Warningf("GCE PD %s already exists, reusing", req.Name)
		return createResp, nil
	}
	
	diskToCreate := &compute.Disk{
		Name:        req.Name,
		SizeGb:      BytesToGb(capBytes),
		//TODO: Is this description important for anything
		Description: "PD Created by CSI Driver",
		Type:        getDiskTypeURI(project, configuredZone, diskType),
	}

	insertOp, err := svc.Disks.Insert(project, configuredZone, diskToCreate).Context(ctx).Do()

	if err != nil{
		if gceprovider.IsGCEError(err, "alreadyExists") {
			_, err := getAndValidateExistingDisk(svc, project, configuredZone,
				req.Name, diskType,
				req.GetCapacityRange().RequiredBytes,
				req.GetCapacityRange().LimitBytes, ctx)
			if err != nil{
				return nil, err
			}
			glog.Warningf("GCE PD %s already exists, reusing", req.Name)
			return createResp, nil
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("unkown Insert disk error: %v", err))
	}
	
	err = gceprovider.WaitForOp(insertOp, project, configuredZone, svc)

	if err != nil{
		if gceprovider.IsGCEError(err, "alreadyExists") {
			_, err := getAndValidateExistingDisk(svc, project, configuredZone,
				req.Name, diskType,
				req.GetCapacityRange().RequiredBytes,
				req.GetCapacityRange().LimitBytes, ctx)
			if err != nil{
				return nil, err
			}
			glog.Warningf("GCE PD %s already exists after wait, reusing", req.Name)
			return createResp, nil
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("unkown Insert disk operation error: %v", err)) 
	}
	
	glog.Infof("Completed creation of disk %v", req.Name)
	return createResp, nil
}

func getAndValidateExistingDisk(svc *compute.Service, project, configuredZone, name, diskType string, reqBytes, limBytes uint64, ctx context.Context) (exists bool, err error){
	resp, err := svc.Disks.Get(project, configuredZone, name).Context(ctx).Do()
	if err != nil{
		if gceprovider.IsGCEError(err, "notFound"){
			glog.Infof("Disk %v does not already exist. Continuing with creation.", name)
		} else{
			glog.Warningf("Unknown disk GET error: %v", err)
		}
	}

	if resp != nil{
		// Disk already exists
		if GbToBytes(resp.SizeGb) < reqBytes ||
		 GbToBytes(resp.SizeGb) > limBytes {
			return true, status.Error(codes.AlreadyExists, fmt.Sprintf(
				"Disk already exists with incompatible capacity. Need %v (Required) < %v (Existing) < %v (Limit)",
				reqBytes, GbToBytes(resp.SizeGb), limBytes))
		} else if respType := strings.Split(resp.Type, "/"); respType[len(respType)-1] != diskType{
			return true, status.Error(codes.AlreadyExists, fmt.Sprintf(
				"Disk already exists with incompatible type. Need %v. Got %v",
				diskType, respType[len(respType)-1]))
		} else{
			// Volume exists with matching name, capacity, type.
			glog.Infof("Compatible disk already exists. Reusing existing.")
			return  true, nil
		}
	}

	return false, nil
}

func getDiskTypeURI(project, zone, diskType string) string{
	return fmt.Sprintf(diskTypeURITemplateSingleZone, project, zone, diskType)
}

func combineVolumeId(project, zone, name string) string{
	return fmt.Sprintf("%s/%s/%s", project, zone, name)
}

func (gceCS *GCEControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	// TODO: should we only be able to delete volumes created by the CSI driver, or all volumes in that project?
	// Assuming ID is of form {project}/{zone}/{id}
	glog.Infof("DeleteVolume called with request %v", *req)

	svc := gceCS.Driver.cloudService

	project, zone, name, err := splitProjectZoneNameId(req.VolumeId)
	if err != nil{
		return nil, err
	}

	deleteOp, err := svc.Disks.Delete(project, zone, name).Context(ctx).Do()
	if err != nil{
		if gceprovider.IsGCEError(err, "resourceInUseByAnotherResource") {
			return nil, status.Error(codes.FailedPrecondition, fmt.Sprintf("Volume in use: %v", err))
		}
		if gceprovider.IsGCEError(err, "notFound"){
			// Already deleted
			return &csi.DeleteVolumeResponse{}, nil
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("unknown Delete disk error: %v", err))
	}

	err = gceprovider.WaitForOp(deleteOp, project, zone, svc)
	if err != nil{
		return nil, status.Error(codes.Internal, fmt.Sprintf("unknown Delete disk operation error: %v", err))
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func splitProjectZoneNameId(id string) (string, string, string, error){
	splitId := strings.Split(id, "/")
	if len(splitId) != 3{
		return "","","",status.Error(codes.InvalidArgument, fmt.Sprintf("Failed to get id components. Expected {project}/{zone}/{name}. Got: %s", id))
	}
	return splitId[0], splitId[1], splitId[2], nil
}

func (gceCS *GCEControllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	glog.Infof("ControllerPublishVolume called with request %v", *req)
	
	svc := gceCS.Driver.cloudService
	project := gceCS.Driver.project

	// Check arguments
	if req.GetVersion() == nil {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Version must be provided")
	}
	if len(req.VolumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Volume ID must be provided")
	}
	if len(req.NodeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Node ID must be provided")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Volume capability must be provided")
	}

	instanceProject, instanceZone, instanceName, err := splitProjectZoneNameId(req.NodeId)
	if err != nil{
		return nil, err
	}
	volumeProject , volumeZone, volumeName, err := splitProjectZoneNameId(req.VolumeId)
	if err != nil{
		return nil, err
	}

	pubVolResp := &csi.ControllerPublishVolumeResponse{
		// TODO: Should this have anything?
		PublishVolumeInfo: nil,
	}

	disk, err := getDiskOrError(ctx, svc, volumeProject, volumeZone, volumeName)
	if err != nil{
		return nil, err
	}
	instance, err := getInstanceOrError(ctx, svc, instanceProject, instanceZone, instanceName)
	if err != nil{
		return nil, err
	}

	readWrite := "READ_WRITE"
	if req.Readonly {
		readWrite = "READ_ONLY"
	}	

	attached, err := diskIsAttachedAndCompatible(disk, instance, req.GetVolumeCapability(), readWrite)
	if err != nil{
		return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("disk %v already published to node %v but incompatbile: %v", volumeName, instanceName, err))
	}
	if attached {
		// Volume is attached to node. Success!
		glog.Infof("Attach operation is successful. PD %q was already attached to node %q.", volumeName, instanceName)
		return pubVolResp, nil
	}

	// TODO: check if disk is attached to any other instances, if so. Volume published to another node ERROR!	

	source := getDiskSourceURI(svc, disk, project, volumeZone)

	attachedDiskV1 := &compute.AttachedDisk{
		DeviceName: disk.Name,
		Kind:       disk.Kind,
		Mode:       readWrite,
		Source:     source,
		Type:       disk.Type,
	}

	attachOp, err := svc.Instances.AttachDisk(
		project, instanceZone, instanceName, attachedDiskV1).Do()
	if err != nil{
		return nil, status.Error(codes.Internal, fmt.Sprintf("unknown Attach error: %v", err))
	}

	err = gceprovider.WaitForOp(attachOp, project, instanceZone, svc)
	if err != nil{
		return nil, status.Error(codes.Internal, fmt.Sprintf("unknown Attach operation error: %v", err))
	}
	
	return pubVolResp, nil
}

func getDiskSourceURI(svc *compute.Service, disk *compute.Disk, project, zone string) string {
	return svc.BasePath + fmt.Sprintf(
		diskSourceURITemplateSingleZone,
		project,
		zone,
		disk.Name)
}

func (gceCS *GCEControllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	glog.Infof("ControllerUnpublishVolume called with request %v", *req)

	// Check arguments
	if req.GetVersion() == nil {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Version must be provided")
	}
	if len(req.VolumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Volume ID must be provided")
	}
	if len(req.NodeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Node ID must be provided")
	}

	svc := gceCS.Driver.cloudService
	project := gceCS.Driver.project

	instanceProject, instanceZone, instanceName, err := splitProjectZoneNameId(req.NodeId)
	if err != nil{
		return nil, err
	}
	volumeProject, volumeZone, volumeName, err := splitProjectZoneNameId(req.VolumeId)
	if err != nil{
		return nil, err
	}

	disk, err := getDiskOrError(ctx, svc, volumeProject, volumeZone, volumeName)
	if err != nil{
		return nil, err
	}
	instance, err := getInstanceOrError(ctx, svc, instanceProject, instanceZone, instanceName)
	if err != nil{
		return nil, err
	}

	attached := diskIsAttached(disk, instance)

	if !attached {
		// Volume is not attached to node. Success!
		glog.Infof("Detach operation is successful. PD %q was not attached to node %q.", volumeName, instanceName)
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}
	
	detachOp, err := svc.Instances.DetachDisk(
		project, instanceZone, instanceName, volumeName).Do()
	if err != nil{
		return nil, status.Error(codes.Internal, fmt.Sprintf("unknown detach error: %v", err))
	}

	err = gceprovider.WaitForOp(detachOp, project, instanceZone, svc)
	if err != nil{
		return nil, status.Error(codes.Internal, fmt.Sprintf("unknown detach operation error: %v", err))
	}

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func getDiskOrError(ctx context.Context, svc *compute.Service, project, volumeZone, volumeName string) (*compute.Disk, error){
	disk, err := svc.Disks.Get(project, volumeZone, volumeName).Context(ctx).Do()
	if err != nil{
		if gceprovider.IsGCEError(err, "notFound"){
			return nil, status.Error(codes.NotFound, fmt.Sprintf("disk %v does not exist", volumeName)) 
		}

		return  nil, status.Error(codes.Internal, fmt.Sprintf("unknown disk GET error: %v", err))
	}

	return disk, nil
}

func getInstanceOrError(ctx context.Context, svc *compute.Service, project, instanceZone, instanceName string) (*compute.Instance, error){
	instance, err := svc.Instances.Get(project, instanceZone, instanceName).Do()
	if err != nil {
		if gceprovider.IsGCEError(err, "notFound"){
			return nil, status.Error(codes.NotFound, fmt.Sprintf("instance %v does not exist", instanceName)) 
		}

		return nil, status.Error(codes.Internal, fmt.Sprintf("unknown instance GET error: %v", err))
	}

	return instance, nil
}

//TODO: this abstraction isn't great. We shouldn't need diskIsAttached AND diskIsAttachedAndCompatible to duplicate code
func diskIsAttached(volume *compute.Disk, instance *compute.Instance) (bool) {
	for _, disk := range instance.Disks {
		if disk.DeviceName == volume.Name {
			// Disk is attached to node
			return true
		}
	}

	return false
}

func diskIsAttachedAndCompatible(volume *compute.Disk, instance *compute.Instance, volumeCapability *csi.VolumeCapability, readWrite string) (bool, error) {
	for _, disk := range instance.Disks {
		if disk.DeviceName == volume.Name {
			// Disk is attached to node
			if disk.Mode != readWrite{
				return true, fmt.Errorf("disk mode does not match. Got %v. Want %v", disk.Mode, readWrite)
			}
			// TODO: check volume_capability.
			return true, nil
		}
	}

	return false, nil
}


func (gceCS *GCEControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	// TODO: revisit this, this is just the default
	glog.V(5).Infof("Using default ValidateVolumeCapabilities")

	for _, c := range req.GetVolumeCapabilities() {
		found := false
		for _, c1 := range gceCS.Driver.vcap {
			if c1.Mode == c.GetAccessMode().Mode {
				found = true
			}
		}
		if !found {
			return &csi.ValidateVolumeCapabilitiesResponse{
				Supported: false,
				Message:   "Driver doesnot support mode:" + c.GetAccessMode().Mode.String(),
			}, status.Error(codes.InvalidArgument, "Driver doesnot support mode:"+c.GetAccessMode().Mode.String())
		}
		// TODO: Ignoring mount & block types for now.
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Supported: true,
	}, nil
}

func (gceCS *GCEControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	// https://cloud.google.com/compute/docs/reference/beta/disks/list
	// List volumes in the whole region? In only the zone that this controller is running?
	return nil, status.Error(codes.Unimplemented, "")
}

func (gceCS *GCEControllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	// https://cloud.google.com/compute/quotas
	// DISKS_TOTAL_GB.
	return nil, status.Error(codes.Unimplemented, "")
}

func (gceCS *GCEControllerServer) ControllerProbe(ctx context.Context, req *csi.ControllerProbeRequest) (*csi.ControllerProbeResponse, error) {
	// TODO: revisit this because just using default
	glog.V(5).Infof("Using default ControllerProbe")

	if err := gceCS.Driver.ValidateControllerServiceRequest(req.Version, csi.ControllerServiceCapability_RPC_UNKNOWN); err != nil {
		return nil, err
	}
	return &csi.ControllerProbeResponse{}, nil
}

// ControllerGetCapabilities implements the default GRPC callout.
// Default supports all capabilities
func (gceCS *GCEControllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	// TODO: revisit this because just using default
	glog.V(5).Infof("Using default ControllerGetCapabilities")

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: gceCS.Driver.cscap,
	}, nil
}