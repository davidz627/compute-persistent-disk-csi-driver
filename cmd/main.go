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

package main

import (
	"flag"
	"os"
	"github.com/golang/glog"

	driver "github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver/pkg/gce-csi-driver"
	"github.com/container-storage-interface/spec/lib/go/csi"
)

func init() {
	flag.Set("logtostderr", "true")
}

var (
	endpoint   = flag.String("endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	driverName = flag.String("drivername", "csi-gce", "name of the driver")
	nodeID     = flag.String("nodeid", "", "node id")
	project = flag.String("project", "", "project to provision storage in")
	version = csi.Version{
		Minor: 1,
	}
)

func main() {
	flag.Parse()

	handle()
	os.Exit(0)
}

func handle() {
	gceDriver := driver.GetGCEDriver()

	//Initialize GCE Driver (Move setup to main?)
	err := gceDriver.SetupGCEDriver(*driverName, &version, []*csi.Version{&version}, *nodeID, *project)
	if err != nil{
		glog.Fatalf("Failed to initialize GCE CSI Driver: %v", err)
	}

	gceDriver.Run(*endpoint)
}