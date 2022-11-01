/*
Copyright 2018-2019 Kaloom Inc.

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

package common

import (
	"crypto/md5"
	"fmt"
	"io"
)

const (
	maxIfNameLength        = 15 // the Max on Linux (i.e. IFNAMSIZ)
	auxNetworkIfnamePrefix = "net"
)

// NetworkConfig is a struct of the Pod's annotations networks element
// (a json array of NetworkConfig)
type NetworkConfig struct {
	NetworkName  string   `json:"name"`                   // required parameter: the network name for the CRD network resource in k8s
	IfMAC        string   `json:"ifMac,omitempty"`        // optional parameter: the network device mac address in the form of 00:11:22:33:44:55
	IsPrimary    bool     `json:"isPrimary,omitempty"`    // optional parameter: specify that this network is associated with the primary device in the Pod i.e. eth0
	UpperLayers  []string `json:"upperLayers,omitempty"`  // optional parameter: specify the upper layers for this network i.e. a hierarchy in which this network is a lower layer of another network(s)
	Namespace    string   `json:"namespace,omitempty"`    // optional parameter: the namespace to which this network belongs to, if not specified it would be the namespace of the pod
	PodagentSkip bool     `json:"podagentSkip,omitempty"` // optional parameter: if true, then podagent won't try to configure this network
}

// GetNetworkIfname from a networkName return a device name by
// hashing the networkName and prefixing it with auxNetworkIfnamePrefix
func GetNetworkIfname(networkName string) string {
	h := md5.New()
	io.WriteString(h, networkName)
	nif := fmt.Sprintf("%s%x", auxNetworkIfnamePrefix, h.Sum(nil))
	return nif[:maxIfNameLength]
}
