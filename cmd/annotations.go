/*
Copyright 2016 Jive Communications All rights reserved.

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

package cmd

import (
	"fmt"
	"strings"
)

// ParseAnnotations converts a slice of key=val pairs to a map
func parseAnnotations(a []string) map[string]string {
	annotations := map[string]string{}
	for idx := range a {
		split := strings.Split(a[idx], "=")
		annotations[split[0]] = strings.Join(split[1:len(split)], "=")
	}
	return annotations
}

func flattenAnnotations(a map[string]string) []string {
	annotations := []string{}
	for k, v := range a {
		annotations = append(annotations, fmt.Sprintf("%s=%s", k, v))
	}
	return annotations
}
