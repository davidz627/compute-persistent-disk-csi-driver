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
)

func CreateCloudService() (*compute.Service, error){
	client, err := newDefaultOauthClient()
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
func newDefaultOauthClient() (*http.Client, error) {
	var err error
	tokenSource, err := google.DefaultTokenSource(
		oauth2.NoContext,
		compute.CloudPlatformScope,
		compute.ComputeScope)
	glog.Infof("Using DefaultTokenSource %#v", tokenSource)
	if err != nil {
		return nil, err
	}

	return oauth2.NewClient(oauth2.NoContext, tokenSource), nil
}