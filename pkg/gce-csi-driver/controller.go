/*
Copyright 2018 The Kubernetes Authors.
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
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GCEControllerServer struct {
	Driver *GCEDriver
}

func (gceCS *GCEControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

/*
The create volume call from GCE PD

// CreateVolume creates a GCE PD.
// Returns: gcePDName, volumeSizeGB, labels, fsType, error
func (gceutil *GCEDiskUtil) CreateVolume(c *gcePersistentDiskProvisioner) (string, int, map[string]string, string, error) {
	cloud, err := getCloudProvider(c.gcePersistentDisk.plugin.host.GetCloudProvider())
	if err != nil {
		return "", 0, nil, "", err
	}

	name := volume.GenerateVolumeName(c.options.ClusterName, c.options.PVName, 63) // GCE PD name can have up to 63 characters
	capacity := c.options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	// GCE PDs are allocated in chunks of GBs (not GiBs)
	requestGB := volume.RoundUpToGB(capacity)

	// Apply Parameters (case-insensitive). We leave validation of
	// the values to the cloud provider.
	diskType := ""
	configuredZone := ""
	configuredZones := ""
	configuredReplicaZones := ""
	zonePresent := false
	zonesPresent := false
	replicaZonesPresent := false
	fstype := ""
	for k, v := range c.options.Parameters {
		switch strings.ToLower(k) {
		case "type":
			diskType = v
		case "zone":
			zonePresent = true
			configuredZone = v
		case "zones":
			zonesPresent = true
			configuredZones = v
		case "replica-zones":
			replicaZonesPresent = true
			configuredReplicaZones = v
		case volume.VolumeParameterFSType:
			fstype = v
		default:
			return "", 0, nil, "", fmt.Errorf("invalid option %q for volume plugin %s", k, c.plugin.GetPluginName())
		}
	}

	if ((zonePresent || zonesPresent) && replicaZonesPresent) ||
		(zonePresent && zonesPresent) {
		// 011, 101, 111, 110
		return "", 0, nil, "", fmt.Errorf("a combination of zone, zones, and replica-zones StorageClass parameters must not be used at the same time")
	}

	// TODO: implement PVC.Selector parsing
	if c.options.PVC.Spec.Selector != nil {
		return "", 0, nil, "", fmt.Errorf("claim.Spec.Selector is not supported for dynamic provisioning on GCE")
	}

	if !zonePresent && !zonesPresent && replicaZonesPresent {
		// 001 - "replica-zones" specified
		replicaZones, err := volumeutil.ZonesToSet(configuredReplicaZones)
		if err != nil {
			return "", 0, nil, "", err
		}

		err = createRegionalPD(
			name,
			c.options.PVC.Name,
			diskType,
			replicaZones,
			requestGB,
			c.options.CloudTags,
			cloud)
		if err != nil {
			glog.V(2).Infof("Error creating regional GCE PD volume: %v", err)
			return "", 0, nil, "", err
		}

		glog.V(2).Infof("Successfully created Regional GCE PD volume %s", name)
	} else {
		var zones sets.String
		if !zonePresent && !zonesPresent {
			// 000 - neither "zone", "zones", or "replica-zones" specified
			// Pick a zone randomly selected from all active zones where
			// Kubernetes cluster has a node.
			zones, err = cloud.GetAllCurrentZones()
			if err != nil {
				glog.V(2).Infof("error getting zone information from GCE: %v", err)
				return "", 0, nil, "", err
			}
		} else if !zonePresent && zonesPresent {
			// 010 - "zones" specified
			// Pick a zone randomly selected from specified set.
			if zones, err = volumeutil.ZonesToSet(configuredZones); err != nil {
				return "", 0, nil, "", err
			}
		} else if zonePresent && !zonesPresent {
			// 100 - "zone" specified
			// Use specified zone
			if err := volume.ValidateZone(configuredZone); err != nil {
				return "", 0, nil, "", err
			}
			zones = make(sets.String)
			zones.Insert(configuredZone)
		}
		zone := volume.ChooseZoneForVolume(zones, c.options.PVC.Name)

		if err := cloud.CreateDisk(
			name,
			diskType,
			zone,
			int64(requestGB),
			*c.options.CloudTags); err != nil {
			glog.V(2).Infof("Error creating single-zone GCE PD volume: %v", err)
			return "", 0, nil, "", err
		}

		glog.V(2).Infof("Successfully created single-zone GCE PD volume %s", name)
	}

	labels, err := cloud.GetAutoLabelsForPD(name, "" )
	if err != nil {
		// We don't really want to leak the volume here...
		glog.Errorf("error getting labels for volume %q: %v", name, err)
	}

	return name, int(requestGB), labels, fstype, nil
}

*/

func (gceCS *GCEControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
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