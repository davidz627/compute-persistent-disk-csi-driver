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

package gcecloudprovider

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver/pkg/utils"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (cloud *CloudProvider) GetDiskOrError(ctx context.Context, volumeZone, volumeName string) (*compute.Disk, error) {
	svc := cloud.Service
	project := cloud.Project
	glog.Infof("Getting disk %v from zone %v", volumeName, volumeZone)
	disk, err := svc.Disks.Get(project, volumeZone, volumeName).Context(ctx).Do()
	if err != nil {
		if IsGCEError(err, "notFound") {
			return nil, status.Error(codes.NotFound, fmt.Sprintf("disk %v does not exist", volumeName))
		}

		return nil, status.Error(codes.Internal, fmt.Sprintf("unknown disk GET error: %v", err))
	}
	glog.Infof("Got disk %v from zone %v", volumeName, volumeZone)
	return disk, nil
}

func (cloud *CloudProvider) GetAndValidateExistingDisk(ctx context.Context, configuredZone, name, diskType string, reqBytes, limBytes int64) (exists bool, err error) {
	svc := cloud.Service
	project := cloud.Project
	resp, err := svc.Disks.Get(project, configuredZone, name).Context(ctx).Do()
	if err != nil {
		if IsGCEError(err, "notFound") {
			glog.Infof("Disk %v does not already exist. Continuing with creation.", name)
		} else {
			glog.Warningf("Unknown disk GET error: %v", err)
		}
	}

	if resp != nil {
		// Disk already exists
		if utils.GbToBytes(resp.SizeGb) < reqBytes ||
			utils.GbToBytes(resp.SizeGb) > limBytes {
			return true, status.Error(codes.AlreadyExists, fmt.Sprintf(
				"Disk already exists with incompatible capacity. Need %v (Required) < %v (Existing) < %v (Limit)",
				reqBytes, utils.GbToBytes(resp.SizeGb), limBytes))
		} else if respType := strings.Split(resp.Type, "/"); respType[len(respType)-1] != diskType {
			return true, status.Error(codes.AlreadyExists, fmt.Sprintf(
				"Disk already exists with incompatible type. Need %v. Got %v",
				diskType, respType[len(respType)-1]))
		} else {
			// Volume exists with matching name, capacity, type.
			glog.Infof("Compatible disk already exists. Reusing existing.")
			return true, nil
		}
	}

	return false, nil
}

func (cloud *CloudProvider) InsertDisk(ctx context.Context, zone string, diskToCreate *compute.Disk) (*compute.Operation, error) {
	return cloud.Service.Disks.Insert(cloud.Project, zone, diskToCreate).Context(ctx).Do()
}

func (cloud *CloudProvider) DeleteDisk(ctx context.Context, zone, name string) (*compute.Operation, error) {
	return cloud.Service.Disks.Delete(cloud.Project, zone, name).Context(ctx).Do()
}

func (cloud *CloudProvider) AttachDisk(ctx context.Context, zone, instanceName string, attachedDisk *compute.AttachedDisk) (*compute.Operation, error) {
	return cloud.Service.Instances.AttachDisk(cloud.Project, zone, instanceName, attachedDisk).Context(ctx).Do()
}

func (cloud *CloudProvider) DetachDisk(ctx context.Context, volumeZone, instanceName, volumeName string) (*compute.Operation, error) {
	return cloud.Service.Instances.DetachDisk(cloud.Project, volumeZone, instanceName, volumeName).Context(ctx).Do()
}
