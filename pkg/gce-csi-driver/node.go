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
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	utils "github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver/pkg/utils"
	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/exec"
)

type GCENodeServer struct {
	Driver *GCEDriver
}

const (
	diskByIdPath         = "/host/dev/disk/by-id/"
	diskGooglePrefix     = "google-"
	diskScsiGooglePrefix = "scsi-0Google_PersistentDisk_"
	diskPartitionSuffix  = "-part"
	diskSDPath           = "/host/dev/sd"
	diskSDPattern        = "/host/dev/sd*"
	// How many times to retry for a consistent read of /proc/mounts.
	maxListTries = 3
	// Number of fields per line in /proc/mounts as per the fstab man page.
	expectedNumFieldsPerLine = 6
	// Location of the mount file to use
	procMountsPath = "/proc/mounts"
	// Location of the mountinfo file
	procMountInfoPath = "/proc/self/mountinfo"
	// 'fsck' found errors and corrected them
	fsckErrorsCorrected = 1
	// 'fsck' found errors but exited without correcting them
	fsckErrorsUncorrected = 4
	defaultMountCommand   = "mount"
)

func (ns *GCENodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	glog.Infof("NodePublishVolume called with req: %#v", req)

	// TODO(dyzz): Check for invalid arguments
	targetPath := req.GetTargetPath()
	stagingTargetPath := req.GetStagingTargetPath()

	notMnt, err := isLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		glog.Errorf("cannot validate mount point: %s %v", targetPath, err)
		return nil, err
	}
	if !notMnt {
		// TODO(dyzz): check if mount is compatible. Return OK if it is, or appropriate error.
		return nil, nil
	}

	if err := os.MkdirAll(targetPath, 0750); err != nil {
		glog.Errorf("mkdir failed on disk %s (%v)", targetPath, err)
		return nil, err
	}

	// Perform a bind mount to the full path to allow duplicate mounts of the same PD.
	options := []string{"bind"}
	if req.Readonly {
		options = append(options, "ro")
	}

	err = doMount(defaultMountCommand, stagingTargetPath, targetPath, "ext4", options)
	if err != nil {
		notMnt, mntErr := isLikelyNotMountPoint(targetPath)
		if mntErr != nil {
			glog.Errorf("IsLikelyNotMountPoint check failed: %v", mntErr)
			return nil, status.Error(codes.Internal, fmt.Sprintf("TODO: %v", err))
		}
		if !notMnt {
			if _, mntErr = unmountVolume(targetPath); mntErr != nil {
				glog.Errorf("Failed to unmount: %v", mntErr)
				return nil, status.Error(codes.Internal, fmt.Sprintf("TODO: %v", err))
			}
			notMnt, mntErr := isLikelyNotMountPoint(targetPath)
			if mntErr != nil {
				glog.Errorf("IsLikelyNotMountPoint check failed: %v", mntErr)
				return nil, status.Error(codes.Internal, fmt.Sprintf("TODO: %v", err))
			}
			if !notMnt {
				// This is very odd, we don't expect it.  We'll try again next sync loop.
				glog.Errorf("%s is still mounted, despite call to unmount().  Will try again next sync loop.", targetPath)
				return nil, status.Error(codes.Internal, fmt.Sprintf("TODO: %v", err))
			}
		}
		os.Remove(targetPath)
		glog.Errorf("Mount of disk %s failed: %v", targetPath, err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("TODO: %v", err))
	}

	glog.V(4).Infof("Successfully mounted %s", targetPath)
	return &csi.NodePublishVolumeResponse{}, nil
}

func matchDiskMode(disk *compute.AttachedDisk, readonly bool) bool {
	if (disk.Mode == "READ_ONLY" && !readonly) || (disk.Mode == "READ_WRITE" && readonly) {
		return false
	}
	return true
}

// formatAndMount uses unix utils to format and mount the given disk
func formatAndMount(source string, target string, fstype string, options []string) error {
	options = append(options, "defaults")

	// Run fsck on the disk to fix repairable issues
	glog.V(4).Infof("Checking for issues with fsck on disk: %s", source)
	args := []string{"-a", source}
	out, err := execRun("fsck", args...)
	if err != nil {
		ee, isExitError := err.(exec.ExitError)
		switch {
		case err == exec.ErrExecutableNotFound:
			glog.Warningf("'fsck' not found on system; continuing mount without running 'fsck'.")
		case isExitError && ee.ExitStatus() == fsckErrorsCorrected:
			glog.Infof("Device %s has errors which were corrected by fsck.", source)
		case isExitError && ee.ExitStatus() == fsckErrorsUncorrected:
			return fmt.Errorf("'fsck' found errors on device %s but could not correct them: %s.", source, string(out))
		case isExitError && ee.ExitStatus() > fsckErrorsUncorrected:
			glog.Infof("`fsck` error %s", string(out))
		}
	}

	// Try to mount the disk
	glog.V(4).Infof("Attempting to mount disk: %s %s %s", fstype, source, target)
	mountErr := doMount(defaultMountCommand, source, target, fstype, options)
	if mountErr != nil {
		// Mount failed. This indicates either that the disk is unformatted or
		// it contains an unexpected filesystem.
		existingFormat, err := getDiskFormat(source)
		if err != nil {
			return err
		}
		if existingFormat == "" {
			// Disk is unformatted so format it.
			args = []string{source}
			// Use 'ext4' as the default
			if len(fstype) == 0 {
				fstype = "ext4"
			}

			if fstype == "ext4" || fstype == "ext3" {
				args = []string{"-F", source}
			}
			glog.Infof("Disk %q appears to be unformatted, attempting to format as type: %q with options: %v", source, fstype, args)
			_, err := execRun("mkfs."+fstype, args...)
			if err == nil {
				// the disk has been formatted successfully try to mount it again.
				glog.Infof("Disk successfully formatted (mkfs): %s - %s %s", fstype, source, target)
				return doMount(defaultMountCommand, source, target, fstype, options)
			}
			glog.Errorf("format of disk %q failed: type:(%q) target:(%q) options:(%q)error:(%v)", source, fstype, target, options, err)
			return err
		} else {
			// Disk is already formatted and failed to mount
			if len(fstype) == 0 || fstype == existingFormat {
				// This is mount error
				return mountErr
			} else {
				// Block device is formatted with unexpected filesystem, let the user know
				return fmt.Errorf("failed to mount the volume as %q, it already contains %s. Mount error: %v", fstype, existingFormat, mountErr)
			}
		}
	}
	return mountErr
}

// GetDiskFormat uses 'lsblk' to see if the given disk is unformated
func getDiskFormat(disk string) (string, error) {
	args := []string{"-n", "-o", "FSTYPE", disk}
	glog.V(4).Infof("Attempting to determine if disk %q is formatted using lsblk with args: (%v)", disk, args)
	dataOut, err := execRun("lsblk", args...)
	output := string(dataOut)
	glog.V(4).Infof("Output: %q", output)

	if err != nil {
		glog.Errorf("Could not determine if disk %q is formatted (%v)", disk, err)
		return "", err
	}

	// Split lsblk output into lines. Unformatted devices should contain only
	// "\n". Beware of "\n\n", that's a device with one empty partition.
	output = strings.TrimSuffix(output, "\n") // Avoid last empty line
	lines := strings.Split(output, "\n")
	if lines[0] != "" {
		// The device is formatted
		return lines[0], nil
	}

	if len(lines) == 1 {
		// The device is unformatted and has no dependent devices
		return "", nil
	}

	// The device has dependent devices, most probably partitions (LVM, LUKS
	// and MD RAID are reported as FSTYPE and caught above).
	return "unknown data, probably partitions", nil
}

// doMount runs the mount command. mounterPath is the path to mounter binary if containerized mounter is used.
func doMount(mountCmd string, source string, target string, fstype string, options []string) error {
	mountArgs := makeMountArgs(source, target, fstype, options)

	glog.V(4).Infof("Mounting cmd (%s) with arguments (%s)", mountCmd, mountArgs)
	output, err := execRun(mountCmd, mountArgs...)
	if err != nil {
		args := strings.Join(mountArgs, " ")
		glog.Errorf("Mount failed: %v\nMounting command: %s\nMounting arguments: %s\nOutput: %s\n", err, mountCmd, args, string(output))
		return fmt.Errorf("mount failed: %v\nMounting command: %s\nMounting arguments: %s\nOutput: %s",
			err, mountCmd, args, string(output))
	}
	return err
}

// makeMountArgs makes the arguments to the mount(8) command.
func makeMountArgs(source, target, fstype string, options []string) []string {
	// Build mount command as follows:
	//   mount [-t $fstype] [-o $options] [$source] $target
	mountArgs := []string{}
	if len(fstype) > 0 {
		mountArgs = append(mountArgs, "-t", fstype)
	}
	if len(options) > 0 {
		mountArgs = append(mountArgs, "-o", strings.Join(options, ","))
	}
	if len(source) > 0 {
		mountArgs = append(mountArgs, source)
	}
	mountArgs = append(mountArgs, target)

	return mountArgs
}

func execRun(cmd string, args ...string) ([]byte, error) {
	exe := exec.New()
	return exe.Command(cmd, args...).CombinedOutput()
}

// IsLikelyNotMountPoint determines if a directory is not a mountpoint.
// It is fast but not necessarily ALWAYS correct. If the path is in fact
// a bind mount from one part of a mount to another it will not be detected.
// mkdir /tmp/a /tmp/b; mount --bin /tmp/a /tmp/b; IsLikelyNotMountPoint("/tmp/b")
// will return true. When in fact /tmp/b is a mount point. If this situation
// if of interest to you, don't use this function...
func isLikelyNotMountPoint(file string) (bool, error) {
	stat, err := os.Stat(file)
	if err != nil {
		return true, err
	}
	rootStat, err := os.Lstat(file + "/..")
	if err != nil {
		return true, err
	}
	// If the directory has a different device as parent, then it is a mountpoint.
	if stat.Sys().(*syscall.Stat_t).Dev != rootStat.Sys().(*syscall.Stat_t).Dev {
		return false, nil
	}

	return true, nil
}

// Returns list of all /dev/disk/by-id/* paths for given PD.
func getDiskByIdPaths(pdName string, partition string) []string {
	devicePaths := []string{
		path.Join(diskByIdPath, diskGooglePrefix+pdName),
		path.Join(diskByIdPath, diskScsiGooglePrefix+pdName),
	}

	if partition != "" {
		for i, path := range devicePaths {
			devicePaths[i] = path + diskPartitionSuffix + partition
		}
	}

	return devicePaths
}

// Returns the first path that exists, or empty string if none exist.
func verifyDevicePath(devicePaths []string, sdBeforeSet sets.String) (string, error) {
	// TODO: Remove this udevadm stuff. Not applicable because can't access /dev/sd* from container
	if err := udevadmChangeToNewDrives(sdBeforeSet); err != nil {
		// udevadm errors should not block disk detachment, log and continue
		glog.Errorf("udevadmChangeToNewDrives failed with: %v", err)
	}

	for _, path := range devicePaths {
		if pathExists, err := pathExists(path); err != nil {
			return "", fmt.Errorf("Error checking if path exists: %v", err)
		} else if pathExists {
			return path, nil
		}
	}

	return "", nil
}

// PathExists returns true if the specified path exists.
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

// Triggers the application of udev rules by calling "udevadm trigger
// --action=change" for newly created "/dev/sd*" drives (exist only in
// after set). This is workaround for Issue #7972. Once the underlying
// issue has been resolved, this may be removed.
func udevadmChangeToNewDrives(sdBeforeSet sets.String) error {
	sdAfter, err := filepath.Glob(diskSDPattern)
	if err != nil {
		return fmt.Errorf("Error filepath.Glob(\"%s\"): %v\r\n", diskSDPattern, err)
	}

	for _, sd := range sdAfter {
		if !sdBeforeSet.Has(sd) {
			return udevadmChangeToDrive(sd)
		}
	}

	return nil
}

// Calls "udevadm trigger --action=change" on the specified drive.
// drivePath must be the block device path to trigger on, in the format "/dev/sd*", or a symlink to it.
// This is workaround for Issue #7972. Once the underlying issue has been resolved, this may be removed.
func udevadmChangeToDrive(drivePath string) error {
	glog.V(5).Infof("udevadmChangeToDrive: drive=%q", drivePath)

	// Evaluate symlink, if any
	drive, err := filepath.EvalSymlinks(drivePath)
	if err != nil {
		return fmt.Errorf("udevadmChangeToDrive: filepath.EvalSymlinks(%q) failed with %v.", drivePath, err)
	}
	glog.V(5).Infof("udevadmChangeToDrive: symlink path is %q", drive)

	// Check to make sure input is "/dev/sd*"
	if !strings.Contains(drive, diskSDPath) {
		return fmt.Errorf("udevadmChangeToDrive: expected input in the form \"%s\" but drive is %q.", diskSDPattern, drive)
	}

	// Call "udevadm trigger --action=change --property-match=DEVNAME=/dev/sd..."
	_, err = exec.New().Command(
		"udevadm",
		"trigger",
		"--action=change",
		fmt.Sprintf("--property-match=DEVNAME=%s", drive)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("udevadmChangeToDrive: udevadm trigger failed for drive %q with %v.", drive, err)
	}
	return nil
}

func (ns *GCENodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	glog.Infof("NodeUnpublishVolume called with args: %v", req)
	// TODO: Check arguments
	targetPath := req.GetTargetPath()

	// TODO: Check volume still exists

	output, err := unmountVolume(targetPath)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Unmount failed: %v\nUnmounting arguments: %s\nOutput: %s\n", err, targetPath, output))
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func unmountVolume(targetPath string) (string, error) {
	output, err := execRun("umount", targetPath)
	return string(output), err
}

func (ns *GCENodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	glog.Infof("NodeStageVolume called with req: %#v", req)
	// TODO(dyzz): Check Arguments
	stagingTargetPath := req.GetStagingTargetPath()

	_, _, volumeName, err := utils.SplitProjectZoneNameId(req.VolumeId)
	if err != nil {
		return nil, err
	}

	// TODO: Check volume still exists?

	// Part 1: Get device path of attached device
	sdBefore, err := filepath.Glob(diskSDPattern)
	if err != nil {
		// Seeing this error means that the diskSDPattern is malformed.
		glog.Errorf("Error filepath.Glob(\"%s\"): %v\r\n", diskSDPattern, err)
	}
	sdBeforeSet := sets.NewString(sdBefore...)

	// TODO: Get real partitions
	partition := ""

	devicePaths := getDiskByIdPaths(volumeName, partition)
	devicePath, err := verifyDevicePath(devicePaths, sdBeforeSet)

	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Error verifying GCE PD (%q) is attached: %v", volumeName, err))
	}
	if devicePath == "" {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Unable to find device path out of attempted paths: %v", devicePaths))
	}

	glog.Infof("Successfully found attached GCE PD %q at device path %s.", volumeName, devicePath)

	// Part 2: Check if mount already exists at targetpath
	notMnt, err := isLikelyNotMountPoint(stagingTargetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(stagingTargetPath, 0750); err != nil {
				return nil, status.Error(codes.Internal, fmt.Sprintf("Failed to create directory (%q): %v", stagingTargetPath, err))
			}
			notMnt = true
		} else {
			return nil, status.Error(codes.Internal, fmt.Sprintf("Unknown error when checking mount point (%q): %v", stagingTargetPath, err))
		}
	}

	if !notMnt {
		// TODO: Validate disk mode

		// TODO: Check who is mounted here. No error if its us
		return nil, fmt.Errorf("already a mount point")

	}

	// Part 3: Mount device to stagingTargetPath
	// Default fstype is ext4
	fstype := "ext4"
	options := []string{}
	if mnt := req.GetVolumeCapability().GetMount(); mnt != nil {
		if mnt.FsType != "" {
			fstype = mnt.FsType
		}
		for _, flag := range mnt.MountFlags {
			// TODO: Not sure if MountFlags == options
			options = append(options, flag)
		}
	} else if blk := req.GetVolumeCapability().GetBlock(); blk != nil {
		// TODO: Block volume support
		return nil, status.Error(codes.Unimplemented, fmt.Sprintf("Block volume support is not yet implemented"))
	}

	err = formatAndMount(devicePath, stagingTargetPath, fstype, options)
	if err != nil {
		return nil, status.Error(codes.Internal,
			fmt.Sprintf("Failed to format and mount device from (%q) to (%q) with fstype (%q) and options (%q): %v",
				devicePath, stagingTargetPath, fstype, options, err))
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *GCENodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	glog.Infof("NodeUnstageVolume called with req: %#v", req)
	// TODO(dyzz) Validate arguments
	stagingTargetPath := req.GetStagingTargetPath()

	_, err := unmountVolume(stagingTargetPath)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("NodeUnstageVolume failed to unmount at path %s: %v", req.GetStagingTargetPath, err))
	}
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *GCENodeServer) NodeGetId(ctx context.Context, req *csi.NodeGetIdRequest) (*csi.NodeGetIdResponse, error) {
	glog.Infof("NodeGetId called with req: %#v", req)

	return &csi.NodeGetIdResponse{
		NodeId: ns.Driver.nodeID,
	}, nil
}

func (ns *GCENodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	glog.Infof("NodeGetCapabilities called with req: %#v", req)

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: ns.Driver.nscap,
	}, nil
}
