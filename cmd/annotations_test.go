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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAnnotations(t *testing.T) {
	assert := assert.New(t)

	cases := []struct {
		input  []string
		output map[string]string
	}{
		{
			input:  []string{"foo=bar"},
			output: map[string]string{"foo": "bar"},
		},
		{
			input:  []string{"a=b", "c=d"},
			output: map[string]string{"a": "b", "c": "d"},
		},
		{
			input:  []string{"foo=bar=baz"},
			output: map[string]string{"foo": "bar=baz"},
		},
		{
			input:  []string{"foo=bar="},
			output: map[string]string{"foo": "bar="},
		},
	}

	for idx := range cases {
		ann := parseAnnotations(cases[idx].input)
		assert.Equal(cases[idx].output, ann)
	}
}
