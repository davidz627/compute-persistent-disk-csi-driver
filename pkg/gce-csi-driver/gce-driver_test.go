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

/*
import (
	"testing"

	"github.com/golang/glog"
	"github.com/kubernetes-csi/csi-test/pkg/sanity"
)

func TestSanity(t *testing.T) {
	// Set up variables
	driverName := "test-driver"
	nodeID := "test-node"
	project := "test-project"
	endpoint := "unix://tmp/csi.sock"
	mountpoint := "unix://tmp/tempmount"

	// Set up driver and env
	gceDriver := GetGCEDriver()

	//Initialize GCE Driver (Move setup to main?)
	err := gceDriver.SetupGCEDriver(driverName, nodeID, project)
	if err != nil {
		glog.Fatalf("Failed to initialize GCE CSI Driver: %v", err)
	}

	gceDriver.Run(endpoint)

	// Run test
	sanity.Test(t, endpoint, mountpoint)
}
*/
