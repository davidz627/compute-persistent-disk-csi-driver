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
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetNodeID(t *testing.T) {
	d := NewFakeDriver()

	ns := NewGCENodeServer(d)

	// Test invalid request
	req := csi.GetNodeIDRequest{}
	_, err := ns.GetNodeID(context.Background(), &req)
	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, s.Code(), codes.InvalidArgument)

	// Test valid request
	req.Version = &fakeVersion
	resp, err := ns.GetNodeID(context.Background(), &req)
	assert.NoError(t, err)
	assert.Equal(t, resp.GetNodeId(), fakeNodeID)
}

func TestNodeGetCapabilities(t *testing.T) {
	d := NewFakeDriver()

	ns := NewGCENodeServer(d)

	// Test invalid request
	req := csi.NodeGetCapabilitiesRequest{}
	_, err := ns.NodeGetCapabilities(context.Background(), &req)
	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, s.Code(), codes.InvalidArgument)

	// Test valid request
	req.Version = &fakeVersion
	_, err = ns.NodeGetCapabilities(context.Background(), &req)
	assert.NoError(t, err)
}

func TestNodeProbe(t *testing.T) {
	d := NewFakeDriver()

	ns := NewGCENodeServer(d)

	// Test invalid request
	req := csi.NodeProbeRequest{}
	_, err := ns.NodeProbe(context.Background(), &req)
	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, s.Code(), codes.InvalidArgument)

	// Test valid request
	req.Version = &fakeVersion
	_, err = ns.NodeProbe(context.Background(), &req)
	assert.NoError(t, err)
}

func TestNodePublishVolume(t *testing.T) {
	d := NewFakeDriver()

	ns := NewGCENodeServer(d)

	// Test invalid request
	req := csi.NodePublishVolumeRequest{}
	_, err := ns.NodePublishVolume(context.Background(), &req)
	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, s.Code(), codes.InvalidArgument)

	// Test valid node publish request
	req.Version = &fakeVersion
	_, err = ns.NodePublishVolume(context.Background(), &req)
	s, ok = status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, s.Code(), codes.Unimplemented)
}

func TestNodeUnpublishVolume(t *testing.T) {
	d := NewFakeDriver()

	ns := NewGCENodeServer(d)

	// Test invalid request
	req := csi.NodeUnpublishVolumeRequest{}
	_, err := ns.NodeUnpublishVolume(context.Background(), &req)
	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, s.Code(), codes.InvalidArgument)

	// Test valid node publish request
	req.Version = &fakeVersion
	_, err = ns.NodeUnpublishVolume(context.Background(), &req)
	s, ok = status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, s.Code(), codes.Unimplemented)
}