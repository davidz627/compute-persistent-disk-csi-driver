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
)

const(
	TokenURL = "https://accounts.google.com/o/oauth2/token"

)

func CreateCloudService() (*compute.Service, error){
	client, err := newOauthClient()
	if err != nil {
		return nil, err
	}
	service, err := compute.New(client)
	if err != nil {
		return nil, err
	}
	//TODO(dyzz) maybe add version here?
	service.UserAgent = fmt.Sprintf("GCE CSI Driver/%s (%s %s)", "0.1.0", runtime.GOOS, runtime.GOARCH)

	return service, nil
}

//TODO: Authenticate smarter. Check Kubernetes for better methods of auth.
//TODO: Add alternative methods of authentication
func newOauthClient() (*http.Client, error) {

	var err error
	tokenSource := google.ComputeTokenSource("")
	glog.Infof("COMPUTE TOKEN SOURCE: %#v", tokenSource)
	if err := wait.PollImmediate(5*time.Second, 30*time.Second, func() (bool, error) {
		if _, err := tokenSource.Token(); err != nil {
			glog.Errorf("error fetching initial token: %v", err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		glog.Warningf("Cannot find compute token source. Are you running this on a non-GCE Instance?")
	} else{
		return oauth2.NewClient(oauth2.NoContext, tokenSource), nil
	}

	// No compute token source, fallback on default
	tokenSource, err = google.DefaultTokenSource(
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

func WaitForOp(op *compute.Operation, project, zone string, svc *compute.Service) error{
	//TODO: Vendor API MACHINERY
	//TODO: timeouts?
	return wait.Poll( 3 * time.Second, 5 * time.Minute, func() (bool, error) {
		pollOp, err := svc.ZoneOperations.Get(project, zone, op.Name).Do()
		if err != nil {
			//TODO some sort of error handling here
			glog.Errorf("error")
			return false, err
		}

		done := opIsDone(pollOp)
		return done, err
	})
}

func opIsDone(op *compute.Operation) bool {
	return op != nil && op.Status == "DONE"
}