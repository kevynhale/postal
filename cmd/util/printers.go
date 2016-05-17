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

package util

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jive/postal/api"
	"github.com/olekukonko/tablewriter"
)

// PrintNetworks prints a slice of Networks in a tabular format
func PrintNetworks(networks []*api.Network) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"id", "cidr", "annotations"})
	for _, n := range networks {
		annotations := []string{}
		for k, v := range n.Annotations {
			annotations = append(annotations, fmt.Sprintf("%s=%s", k, v))
		}
		table.Append([]string{n.ID, n.Cidr, strings.Join(annotations, ", ")})
	}
	table.Render()
}

// PrintNetwork prints a single Network in a tabular format
func PrintNetwork(network *api.Network) {
	PrintNetworks([]*api.Network{network})
}

// PrintPools prints a slice of Pools in a tabular format
func PrintPools(pools []*api.Pool) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"network", "id", "type", "max", "annotations"})
	for _, p := range pools {
		annotations := []string{}
		for k, v := range p.Annotations {
			annotations = append(annotations, fmt.Sprintf("%s=%s", k, v))
		}
		table.Append([]string{p.ID.NetworkID, p.ID.ID, p.Type.String(),
			strconv.Itoa(int(p.MaximumAddresses)), strings.Join(annotations, ", ")})
	}
	table.Render()
}

// PrintPool prints a single Pool in a tabular format
func PrintPool(pool *api.Pool) {
	PrintPools([]*api.Pool{pool})
}

// PrintBindings
func PrintBindings(bindings []*api.Binding, human bool) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetRowSeparator("-")
	table.SetHeader([]string{"network", "pool", "id", "address", "allocated", "bound", "released", "annotations"})
	for _, b := range bindings {
		table.Append([]string{
			formatID(b.PoolID.NetworkID, human),
			formatID(b.PoolID.ID, human),
			formatID(b.ID, human),
			b.Address,
			formatTime(time.Unix(0, b.AllocateTime), human),
			formatTime(time.Unix(0, b.BindTime), human),
			formatTime(time.Unix(0, b.ReleaseTime), human),
			formatAnnotations(b.Annotations)})
	}
	table.Render()
}

func PrintBinding(binding *api.Binding, human bool) {
	PrintBindings([]*api.Binding{binding}, human)
}

func formatID(id string, human bool) string {
	if len(id) > 10 && human {
		return strings.Join([]string{id[0:4], "...", id[len(id)-4 : len(id)]}, "")
	}
	return id
}

func formatTime(t time.Time, human bool) string {
	if t.Unix() == 0 {
		return ""
	}

	if human {
		return humanize.Time(t)
	}

	return t.String()
}

func formatAnnotations(a map[string]string) string {
	annotations := []string{}
	for k, v := range a {
		annotations = append(annotations, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(annotations, ", ")
}
