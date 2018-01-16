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
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GCENodeServer struct {
	Driver *GCEDriver
}

func (ns *GCENodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	err := ns.Driver.CheckVersion(req.GetVersion())
	if err != nil {
		return nil, err
	}
	return nil, status.Error(codes.Unimplemented, "")
}

func (ns *GCENodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	err := ns.Driver.CheckVersion(req.GetVersion())
	if err != nil {
		return nil, err
	}
	return nil, status.Error(codes.Unimplemented, "")
}

func (ns *GCENodeServer) GetNodeID(ctx context.Context, req *csi.GetNodeIDRequest) (*csi.GetNodeIDResponse, error) {
	glog.V(5).Infof("Using default GetNodeID")

	err := ns.Driver.CheckVersion(req.GetVersion())
	if err != nil {
		return nil, err
	}

	return &csi.GetNodeIDResponse{
		NodeId: ns.Driver.nodeID,
	}, nil
}

func (ns *GCENodeServer) NodeProbe(ctx context.Context, req *csi.NodeProbeRequest) (*csi.NodeProbeResponse, error) {
	glog.V(5).Infof("Using default NodeProbe")

	err := ns.Driver.CheckVersion(req.GetVersion())
	if err != nil {
		return nil, err
	}

	return &csi.NodeProbeResponse{}, nil
}

func (ns *GCENodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	glog.V(5).Infof("Using default NodeGetCapabilities")

	err := ns.Driver.CheckVersion(req.GetVersion())
	if err != nil {
		return nil, err
	}

	return &csi.NodeGetCapabilitiesResponse{}, nil
}