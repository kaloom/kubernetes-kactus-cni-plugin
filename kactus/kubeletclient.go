/*
Copyright (c) 2021 Kaloom Inc.
Copyright (c) 2017 Intel Corporation

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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	kc "github.com/kaloom/kubernetes-common"

	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/kubelet/apis/podresources"
	podresourcesapi "k8s.io/kubernetes/pkg/kubelet/apis/podresources/v1alpha1"
	"k8s.io/kubernetes/pkg/kubelet/util"
)

const (
	defaultKubeletSocketFile   = "kubelet.sock"
	defaultPodResourcesMaxSize = 1024 * 1024 * 16 // 16 Mb
)

var (
	kubeletSocket           string
	defaultPodResourcesPath = "/var/lib/kubelet/pod-resources"
)

// ResourceInfo is struct to hold Pod device allocation information
type ResourceInfo struct {
	Index     int
	DeviceIDs []string
}

// ResourceClient provides a kubelet Pod resource handle
type ResourceClient interface {
	// GetPodResourceMap returns an instance of a map of Pod ResourceInfo given a (Pod name, namespace) tuple
	GetPodResourceMap(*v1.Pod) (map[string]*ResourceInfo, error)
}

// GetResourceClient returns an instance of ResourceClient interface initialized with Pod resource information
func GetResourceClient() (ResourceClient, error) {
	// If Kubelet resource API endpoint exist use that by default
	// Or else fallback with checkpoint file
	if hasKubeletAPIEndpoint() {
		kc.LogDebug("GetResourceClient: using Kubelet resource API endpoint\n")
		return getKubeletClient()
	}

	kc.LogDebug("GetResourceClient: using Kubelet device plugin checkpoint\nnn")
	return GetCheckpoint()
}

func getKubeletClient() (ResourceClient, error) {
	newClient := &kubeletClient{}
	if kubeletSocket == "" {
		kubeletSocket = util.LocalEndpoint(defaultPodResourcesPath, podresources.Socket)
	}

	client, conn, err := podresources.GetClient(kubeletSocket, 10*time.Second, defaultPodResourcesMaxSize)
	if err != nil {
		return nil, fmt.Errorf("getKubeletClient: error getting grpc client: %v", err)
	}
	defer conn.Close()

	if err := newClient.getPodResources(client); err != nil {
		return nil, fmt.Errorf("getKubeletClient: error ge tting pod resources from client: %v", err)
	}

	return newClient, nil
}

type kubeletClient struct {
	resources []*podresourcesapi.PodResources
}

func (rc *kubeletClient) getPodResources(client podresourcesapi.PodResourcesListerClient) error {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.List(ctx, &podresourcesapi.ListPodResourcesRequest{})
	if err != nil {
		return fmt.Errorf("getPodResources: failed to list pod resources, %v.Get(_) = _, %v", client, err)
	}

	rc.resources = resp.PodResources
	return nil
}

// GetPodResourceMap returns an instance of a map of Pod ResourceInfo given a (Pod name, namespace) tuple
func (rc *kubeletClient) GetPodResourceMap(pod *v1.Pod) (map[string]*ResourceInfo, error) {
	resourceMap := make(map[string]*ResourceInfo)

	name := pod.Name
	ns := pod.Namespace

	if name == "" || ns == "" {
		return nil, fmt.Errorf("GetPodResourcesMap: Pod name or namespace cannot be empty")
	}

	for _, pr := range rc.resources {
		if pr.Name == name && pr.Namespace == ns {
			for _, cnt := range pr.Containers {
				for _, dev := range cnt.Devices {
					if rInfo, ok := resourceMap[dev.ResourceName]; ok {
						rInfo.DeviceIDs = append(rInfo.DeviceIDs, dev.DeviceIds...)
					} else {
						resourceMap[dev.ResourceName] = &ResourceInfo{DeviceIDs: dev.DeviceIds}
					}
				}
			}
		}
	}
	return resourceMap, nil
}

func hasKubeletAPIEndpoint() bool {
	// Check for kubelet resource API socket file
	kubeletAPISocket := filepath.Join(defaultPodResourcesPath, defaultKubeletSocketFile)
	if _, err := os.Stat(kubeletAPISocket); err != nil {
		kc.LogDebug("hasKubeletAPIEndpoint: error looking up kubelet resource api socket file: %q\n", err)
		return false
	}
	return true
}
