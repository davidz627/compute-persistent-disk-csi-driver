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

import(
		compute "google.golang.org/api/compute/v1"
		"github.com/golang/glog"
		"golang.org/x/oauth2"
		"golang.org/x/oauth2/google"
		"net/http"
		"fmt"
		"runtime"
		"strings"
		"google.golang.org/api/googleapi"
		"cloud.google.com/go/compute/metadata"
		"time"
		"k8s.io/apimachinery/pkg/util/wait"
		"google.golang.org/grpc/codes"
		"google.golang.org/grpc/status"
		"golang.org/x/net/context"
)

const(
	TokenURL = "https://accounts.google.com/o/oauth2/token"
	diskSourceURITemplateSingleZone = "%s/zones/%s/disks/%s"   // {gce.projectID}/zones/{disk.Zone}/disks/{disk.Name}"
	diskTypeURITemplateSingleZone = "projects/%s/zones/%s/diskTypes/%s"   // projects/{gce.projectID}/zones/{disk.Zone}/diskTypes/{disk.Type}"

	gceComputeAPIEndpoint      = "https://www.googleapis.com/compute/v1/"
)


type CloudProvider struct{
	Service *compute.Service
	Project string
	Zone string
}

func CreateCloudProvider() (*CloudProvider, error){
	svc, err := createCloudService()
	if err != nil{
		return nil, err
	}
	// TODO: Use metadata server or flags to retrieve project and zone. Fallback on flag if necessary
	/*
	project, zone, err := gce.GetProjectAndZone()
	if err != nil{
		return fmt.Errorf("Failed creating GCE Cloud Service: %v", err)
	}
	gceDriver.project = project
	gceDriver.zone = zone
	*/

	return &CloudProvider{
		Service: svc,
		Project: "dyzz-test",
		Zone: "us-centra1-b",
	}, nil

}

func createCloudService() (*compute.Service, error){
	// TODO: support alternate methods of authentication
	svc, err := createCloudServiceWithDefaultServiceAccount()
	return svc, err
}

func createCloudServiceWithDefaultServiceAccount() (*compute.Service, error){
	client, err := newDefaultOauthClient()
	if err != nil {
		return nil, err
	}
	service, err := compute.New(client)
	if err != nil {
		return nil, err
	}
	// TODO: Plumb version through to here? Not sure if necessary
	service.UserAgent = fmt.Sprintf("GCE CSI Driver/%s (%s %s)", "0.1.0", runtime.GOOS, runtime.GOARCH)
	return service, nil
}

func newDefaultOauthClient() (*http.Client, error) {
	// No compute token source, fallback on default
	tokenSource, err := google.DefaultTokenSource(
		oauth2.NoContext,
		compute.CloudPlatformScope,
		compute.ComputeScope)
	glog.Infof("Using DefaultTokenSource %#v", tokenSource)
	if err != nil {
		return nil, err
	}

	if err := wait.PollImmediate(5*time.Second, 30*time.Second, func() (bool, error) {
		if _, err := tokenSource.Token(); err != nil {
			glog.Errorf("error fetching initial token: %v", err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}

	return oauth2.NewClient(oauth2.NoContext, tokenSource), nil
}

func GetProjectAndZone() (string, string, error) {
	result, err := metadata.Get("instance/zone")
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(result, "/")
	if len(parts) != 4 {
		return "", "", fmt.Errorf("unexpected response: %s", result)
	}
	zone := parts[3]
	projectID, err := metadata.ProjectID()
	if err != nil {
		return "", "", err
	}
	return projectID, zone, nil
}

// isGCEError returns true if given error is a googleapi.Error with given
// reason (e.g. "resourceInUseByAnotherResource")
func IsGCEError(err error, reason string) bool {
	apiErr, ok := err.(*googleapi.Error)
	if !ok {
		return false
	}

	for _, e := range apiErr.Errors {
		if e.Reason == reason {
			return true
		}
	}
	return false
}

func (cloud *CloudProvider) WaitForOp(ctx context.Context, op *compute.Operation, zone string) error{
	svc := cloud.Service
	project := cloud.Project
	// TODO: Double check that these timeouts are reasonable
	return wait.Poll( 3 * time.Second, 5 * time.Minute, func() (bool, error) {
		pollOp, err := svc.ZoneOperations.Get(project, zone, op.Name).Context(ctx).Do()
		if err != nil {
			glog.Errorf("WaitForOp(op: %#v, zone: %#v) failed to poll the operation", op, zone)
			return false, err
		}
		done := opIsDone(pollOp)
		return done, err
	})
}

func opIsDone(op *compute.Operation) bool {
	return op != nil && op.Status == "DONE"
}



func (cloud *CloudProvider) GetInstanceOrError(ctx context.Context, instanceZone, instanceName string) (*compute.Instance, error){
	svc := cloud.Service
	project := cloud.Project
	glog.Infof("Getting instance %v from zone %v", instanceName, instanceZone)
	instance, err := svc.Instances.Get(project, instanceZone, instanceName).Do()
	if err != nil {
		if IsGCEError(err, "notFound"){
			return nil, status.Error(codes.NotFound, fmt.Sprintf("instance %v does not exist", instanceName)) 
		}

		return nil, status.Error(codes.Internal, fmt.Sprintf("unknown instance GET error: %v", err))
	}
	glog.Infof("Got instance %v from zone %v", instanceName, instanceZone)
	return instance, nil
}

func (cloud *CloudProvider) GetDiskSourceURI(disk *compute.Disk, zone string) string {
	projectsApiEndpoint := gceComputeAPIEndpoint + "projects/"
	if cloud.Service != nil {
		projectsApiEndpoint = cloud.Service.BasePath
	}

	return projectsApiEndpoint+ fmt.Sprintf(
		diskSourceURITemplateSingleZone,
		cloud.Project,
		zone,
		disk.Name)
}

func (cloud *CloudProvider) GetDiskTypeURI(zone, diskType string) string{
	return fmt.Sprintf(diskTypeURITemplateSingleZone, cloud.Project, zone, diskType)
}