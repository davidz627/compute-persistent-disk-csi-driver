/*
Copyright 2017 The Kubernetes Authors.
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

package gcecsidriver

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DefaultControllerServer struct {
	Driver *CSIDriver
}

func (cs *DefaultControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *DefaultControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *DefaultControllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *DefaultControllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *DefaultControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	glog.V(5).Infof("Using default ValidateVolumeCapabilities")

	for _, c := range req.GetVolumeCapabilities() {
		found := false
		for _, c1 := range cs.Driver.vc {
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

func (cs *DefaultControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *DefaultControllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *DefaultControllerServer) ControllerProbe(ctx context.Context, req *csi.ControllerProbeRequest) (*csi.ControllerProbeResponse, error) {
	glog.V(5).Infof("Using default ControllerProbe")

	if err := cs.Driver.ValidateControllerServiceRequest(req.Version, csi.ControllerServiceCapability_RPC_UNKNOWN); err != nil {
		return nil, err
	}
	return &csi.ControllerProbeResponse{}, nil
}

// ControllerGetCapabilities implements the default GRPC callout.
// Default supports all capabilities
func (cs *DefaultControllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	glog.V(5).Infof("Using default ControllerGetCapabilities")

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: cs.Driver.cap,
	}, nil
}