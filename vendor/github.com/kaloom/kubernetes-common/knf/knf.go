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

package knf

import (
	"net"
)

// Define an iota of KvsErrors' key map
const (
	ErrNoPort = iota
	ErrAlreadyExists
	ErrSourceIPforKTEP
	ErrNoL2NetworkFound
)

var (
	// KvsErrors is a map of error messages from kvs, we should've
	// have an iota of error messages in kvs and use them used
	// instead of this map and trying to guess what's the error is
	KvsErrors = map[int]string{
		ErrNoPort:           "No port",
		ErrAlreadyExists:    "already exists",
		ErrSourceIPforKTEP:  "source IP configured on KTEP",
		ErrNoL2NetworkFound: "No L2 network found",
	}
)

// NetworkConf is knf cni-plugin netconf parameters
type NetworkConf struct {
	Master             string   `json:"master"`
	KNID               uint64   `json:"knid"`
	ParticipatingNodes []net.IP `json:"participatingNodes"`
	MTU                int      `json:"mtu"`
}
