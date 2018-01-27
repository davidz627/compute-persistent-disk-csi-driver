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

package utils

import (
	"strings"
	"fmt"
)

func BytesToGb(bytes uint64) int64{
	return int64(bytes/1000000000)
}

func GbToBytes(Gb int64) uint64{
	return uint64(Gb*1000000000)
}

func SplitProjectZoneNameId(id string) (string, string, string, error){
	splitId := strings.Split(id, "/")
	if len(splitId) != 3{
		return "","","", fmt.Errorf("Failed to get id components. Expected {project}/{zone}/{name}. Got: %s", id)
	}
	return splitId[0], splitId[1], splitId[2], nil
}

func CombineVolumeId(project, zone, name string) string{
	return fmt.Sprintf("%s/%s/%s", project, zone, name)
}