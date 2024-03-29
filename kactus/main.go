/*
Copyright (c) 2017-2021 Kaloom Inc.
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

This is a "Multi-plugin" (a fork off Intel's Multus plugin) that
delegates work to other CNI plugins. The delegation's concept is
refered to from the CNI project; it reads other plugin netconf, and
then invoke them, e.g. flannel, knf or sriov plugin.
*/

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"

	kc "github.com/kaloom/kubernetes-common"

	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/020"
	"github.com/containernetworking/cni/pkg/version"
)

const (
	defaultCNIDir     = "/var/lib/cni/kactus"
	crdGroupName      = "kaloom.com" // use our namespace to avoid colliding with somebody's else CRD that uses the same networks api extensions
	resourceNameAnnot = "k8s.v1.cni.cncf.io/resourceName"
)

var (
	branch = "unknown"
	commit = "unknown"
	date   = "unknown"

	vethAlreadyExists = regexp.MustCompile(`container veth name provided \([^)]*\) already exists`)
)

type netConf struct {
	types.NetConf
	CNIDir     string                   `json:"cniDir"`
	Delegates  []map[string]interface{} `json:"delegates"`
	Kubeconfig string                   `json:"kubeconfig"`
}

type cniContext struct {
	pod        *v1.Pod
	cniArgs    *CNIArgs
	auxNetOnly bool
	k8sclient  *kubernetes.Clientset
}

// struct of k8s CRD network object
type netObject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" description:"standard object metadata"`
	Spec              struct {
		Plugin string `json:"plugin"`
		Config string `json:"config"`
	} `json:"spec"`
}

// CNIArgs is the valid CNI_ARGS used for Kubernetes
type CNIArgs struct {
	types.CommonArgs
	IP                         net.IP
	K8S_POD_NAME               types.UnmarshallableString
	K8S_POD_NAMESPACE          types.UnmarshallableString
	K8S_POD_INFRA_CONTAINER_ID types.UnmarshallableString
	K8S_POD_NETWORK            types.UnmarshallableString
	K8S_POD_IFMAC              types.UnmarshallableString
}

func logBuildDetails() {
	kc.LogDebug("kactus build details, branch/tag: %s, commit: %s, date: %s\n", branch, commit, date)
}

func isString(i interface{}) bool {
	_, ok := i.(string)
	return ok
}

func isBool(i interface{}) bool {
	_, ok := i.(bool)
	return ok
}

func loadNetConf(bytes []byte) (*netConf, error) {
	nc := &netConf{}
	if err := json.Unmarshal(bytes, nc); err != nil {
		return nil, fmt.Errorf("failed to load netconf: %v", err)
	}

	if nc.CNIDir == "" {
		nc.CNIDir = defaultCNIDir
	}

	return nc, nil
}

func saveScratchNetConf(containerID, dataDir string, netconf []byte) error {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("failed to create the kactus data directory(%q): %v", dataDir, err)
	}

	path := filepath.Join(dataDir, containerID)
	err := ioutil.WriteFile(path, netconf, 0600)
	if err != nil {
		return fmt.Errorf("failed to write container data in the path(%q): %v", path, err)
	}

	return err
}

func getScratchNetConf(containerIDPath string) ([]byte, error) {
	data, err := ioutil.ReadFile(containerIDPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read container data in the path(%q): %v", containerIDPath, err)
	}

	return data, nil
}

func consumeScratchNetConf(containerID, dataDir string) ([]byte, error) {
	path := filepath.Join(dataDir, containerID)
	defer os.Remove(path)

	return getScratchNetConf(path)
}

func sameNetworkName(netConf map[string]interface{}, netConfList []map[string]interface{}) bool {
	for _, nc := range netConfList {
		if nc["networkName"] == netConf["networkName"] {
			return true
		}
	}
	return false
}

func mergeDelegates(containerID, dataDir string, delegates []map[string]interface{}) ([]map[string]interface{}, []map[string]interface{}) {
	path := filepath.Join(dataDir, containerID)
	netconfBytes, err := getScratchNetConf(path)
	if err != nil {
		return nil, delegates
	}
	var currentDelegates []map[string]interface{}
	err = json.Unmarshal(netconfBytes, &currentDelegates)
	if err != nil {
		return nil, delegates
	}
	for _, d := range currentDelegates {
		if d["networkName"] == nil || !isString(d["networkName"]) ||
			!sameNetworkName(d, delegates) {
			delegates = append(delegates, d)
		}
	}
	return currentDelegates, delegates
}

func saveDelegates(containerID, dataDir string, mergeExistingDelegates bool, delegates []map[string]interface{}) ([]map[string]interface{}, error) {
	var currentDelegates []map[string]interface{}
	if mergeExistingDelegates {
		currentDelegates, delegates = mergeDelegates(containerID, dataDir, delegates)
	}
	if len(delegates) > 0 {
		delegatesBytes, err := json.Marshal(delegates)
		if err != nil {
			return nil, fmt.Errorf("error serializing delegate netconf: %v", err)
		}

		if err = saveScratchNetConf(containerID, dataDir, delegatesBytes); err != nil {
			return nil, fmt.Errorf("error in saving the  delegates : %v", err)
		}

		return currentDelegates, nil
	}
	return currentDelegates, nil
}

func isMasterplugin(netconf map[string]interface{}) bool {
	if netconf["masterplugin"] == nil && netconf["masterPlugin"] == nil {
		return false
	}

	if isBool(netconf["masterPlugin"]) && netconf["masterPlugin"].(bool) {
		return true
	}
	// for transition, to be removed
	if isBool(netconf["masterplugin"]) && netconf["masterplugin"].(bool) {
		return true
	}
	return false
}

func checkDelegate(netconf map[string]interface{}, masterpluginEnabled *bool) error {
	if netconf["type"] == nil {
		return fmt.Errorf("delegate must have the field 'type'")
	}

	if !isString(netconf["type"]) {
		return fmt.Errorf("delegate field 'type' must be a string")
	}

	if isMasterplugin(netconf) {
		if *masterpluginEnabled {
			return fmt.Errorf("only one delegate can have 'masterPlugin'")
		}
		*masterpluginEnabled = true
	}
	return nil
}

// needed for IPAMs and wherabouts in particular see https://github.com/k8snetworkplumbingwg/whereabouts/blob/ebcf63f836d65f6d50e6ee2569997c5d5f081679/pkg/types/types.go#L63
func getCNIArgsForDelegate(cniArgs *CNIArgs) string {
	return fmt.Sprintf("IgnoreUnknown=1;K8S_POD_NAME=%s;K8S_POD_NAMESPACE=%s;K8S_POD_INFRA_CONTAINER_ID=%s", cniArgs.K8S_POD_NAME, cniArgs.K8S_POD_NAMESPACE, cniArgs.K8S_POD_INFRA_CONTAINER_ID)
}

func shouldIgnoreError(pluginType string, err error) bool {
	if pluginType != "bridge" {
		return false
	}

	// The bridge plugin is not idempotent, so in case that podagent restarts
	// and calls kactus again, we want to ignore if the plugin was already ran.
	// This prevents kactus from sending a DEL, then ADD.
	return vethAlreadyExists.MatchString(err.Error())
}

func (cc *cniContext) delegateAdd(network kc.NetworkConfig, argif string, netconf map[string]interface{}) (error, types.Result) {
	kc.LogDebug("delegateAdd: network '%+v', argif '%s', netconf '%+v'\n", network, argif, netconf)
	netconfBytes, err := json.Marshal(netconf)
	if err != nil {
		return fmt.Errorf("Kactus: error serializing kactus delegate netconf: %v", err), nil
	}

	if !isMasterplugin(netconf) {
		podif := kc.GetNetworkIfname(network.NetworkName)

		if os.Setenv("CNI_IFNAME", podif) != nil {
			return fmt.Errorf("Kactus: error in setting CNI_IFNAME"), nil
		}
		cniArgs := getCNIArgsForDelegate(cc.cniArgs)
		if network.IfMAC != "" {
			cniArgs = fmt.Sprintf("%s;CNI_IFMAC=%s;MAC=%s", cniArgs, network.IfMAC, network.IfMAC)
			if os.Setenv("CNI_ARGS", cniArgs); err != nil {
				return fmt.Errorf("Kactus: error in setting CNI_ARGS to %s", cniArgs), nil
			}
			kc.LogDebug("delegateAdd: will invoke.DelegateAdd with a CNI_IFNAME set to: %s and CNI_ARGS set to: '%s' (not a master plugin)\n", podif, cniArgs)
		} else {
			kc.LogDebug("delegateAdd: will invoke.DelegateAdd with a CNI_IFNAME set to: %s and CNI_ARGS set to: '%s' (not a master plugin)\n", podif, cniArgs)
		}
		if os.Setenv("CNI_ARGS", cniArgs); err != nil {
			return fmt.Errorf("Kactus: error in setting CNI_ARGS to %s", cniArgs), nil
		}
	} else {
		if os.Setenv("CNI_IFNAME", argif) != nil {
			return fmt.Errorf("Kactus: error in setting CNI_IFNAME"), nil
		}
		kc.LogDebug("delegateAdd: will invoke.DelegateAdd with a CNI_IFNAME set to: %s' (not a master plugin)\n", argif)
	}
	delegatePluginType := netconf["type"].(string)
	kc.LogDebug("delegateAdd: will call invoke.DelegateAdd for plugin: %s, with: '%s'\n", delegatePluginType, netconfBytes)
	result, err := invoke.DelegateAdd(context.Background(), delegatePluginType, netconfBytes, nil)
	if err != nil {
		if !shouldIgnoreError(delegatePluginType, err) {
			kc.LogError("delegateAdd: invoke.DelegateAdd errored: %s: %v\n", delegatePluginType, err)
			return fmt.Errorf("Kactus: error in invoke Delegate add - %q: %v", delegatePluginType, err), nil
		}

		// podagent currently ignores the result so in this case it's fine
		result = &types020.Result{}
	}

	// return result if the delegate is the master plugin or
	// dynamically injected in a pod, for the latter the podagent
	// would invoke kactus one network at a time
	if !isMasterplugin(netconf) && !cc.auxNetOnly {
		return nil, nil
	}

	return nil, result
}

func (cc *cniContext) delegateDel(argIfName string, netconf map[string]interface{}) error {
	kc.LogDebug("delegateDel: argIfname %s, netconf = '%v'\n", argIfName, netconf)
	ifName := getIfName(argIfName, netconf)
	netconfBytes, err := json.Marshal(netconf)
	if err != nil {
		return fmt.Errorf("Kactus: error serializing kactus delegate netconf: %v", err)
	}

	if os.Setenv("CNI_IFNAME", ifName) != nil {
		return fmt.Errorf("Kactus: error in setting CNI_IFNAME to %s", ifName)
	}
	cniArgs := getCNIArgsForDelegate(cc.cniArgs)
	if os.Setenv("CNI_ARGS", cniArgs); err != nil {
		return fmt.Errorf("Kactus: error in setting CNI_ARGS to %s", cniArgs)
	}
	kc.LogDebug("delegateDel: will invoke.DelegateDel with a CNI_IFNAME set to: %s\n", ifName)
	delegatePluginType := netconf["type"].(string)
	err = invoke.DelegateDel(context.Background(), delegatePluginType, netconfBytes, nil)
	if err != nil {
		return fmt.Errorf("Kactus: error in invoke Delegate del - %q: %v", delegatePluginType, err)
	}

	return err
}

func (cc *cniContext) clearPlugins(idx int, argIfName string, delegates []map[string]interface{}) {
	if os.Setenv("CNI_COMMAND", "DEL") != nil {
		kc.LogError("failed to set CNI_COMMAND to DEL")
		return
	}

	kc.LogDebug("clearPlugins: idx=%d, argIfName=%s\n", idx, argIfName)
	for i := 0; i <= idx; i++ {
		cc.delegateDel(argIfName, delegates[i])
	}
}

func createK8sClient(kubeconfig string) (*kubernetes.Clientset, error) {
	var err error

	cfg := &rest.Config{}
	if kubeconfig != "" {
		// get a config from the provided kubeconfig file and use the current context
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("Kactus: failed to get context for the kubeconfig %v, refer Kactus README.md for the usage guide: %v", kubeconfig, err)
		}
	} else {
		// get a config from within the pod for in-cluster authentication
		cfg, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("couldn't initialize InClusterConfig %v", err)
		}
	}

	// creates the clientset
	return kubernetes.NewForConfig(cfg)
}

func getPodNetworkAnnotation(client *kubernetes.Clientset, nameSpace, podName string) (string, *v1.Pod, error) {
	pod, err := client.CoreV1().Pods(nameSpace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return "", nil, fmt.Errorf("Kactus: failed to fetch pod %s info off k8s apiserver: %v", podName, err)
	}

	return pod.Annotations["networks"], pod, nil
}

// from the CRD networks's config, create a netconf for the delegate cni-plugin
func getPluginNetConf(plugin, config, networkName, deviceID, resourceName string, primary bool) (string, error) {
	var netconf bytes.Buffer

	if plugin == "" || config == "" {
		return "", fmt.Errorf("Kactus: plugin name/config can't be empty")
	}

	tmpconfig := []string{`{"type": "`, plugin, `","networkName": "`, networkName}
	if deviceID != "" {
		tmpconfig = append(tmpconfig, []string{`","deviceID": "`, deviceID, `","resourceName": "`, resourceName}...)
	}
	if primary {
		tmpconfig = append(tmpconfig, []string{`","masterPlugin": true,`, config[strings.Index(config, "\""):len(config)]}...)
	} else {
		tmpconfig = append(tmpconfig, []string{`",`, config[strings.Index(config, "\""):len(config)]}...)
	}

	for _, c := range tmpconfig {
		netconf.WriteString(c)
	}

	return netconf.String(), nil
}

// call the CRD API extension for the crdGroupName and fetch the network configuration
func (cc *cniContext) getDelegateNetConf(networkName string, resourceMap map[string]*ResourceInfo, primary bool) (string, map[string]*ResourceInfo, error) {
	if networkName == "" {
		return "", nil, fmt.Errorf("network name can't be empty")
	}

	crd := fmt.Sprintf("/apis/%s/v1/namespaces/default/networks/%s", crdGroupName, networkName)
	netObjectData, err := cc.k8sclient.ExtensionsV1beta1().RESTClient().Get().AbsPath(crd).DoRaw(context.TODO())
	if err != nil {
		return "", nil, fmt.Errorf("failed to get CRD, refer Kactus README.md for the usage guide: %v", err)
	}

	no := netObject{}
	if err := json.Unmarshal(netObjectData, &no); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal the netObject data for network %s: %v", networkName, err)
	}

	updatedResourceMap, deviceID, resourceName, err := cc.getResourceMap(&no, resourceMap)
	if err != nil {
		return "", nil, err
	}

	nc, err := getPluginNetConf(no.Spec.Plugin, no.Spec.Config, networkName, deviceID, resourceName, primary)
	if err != nil {
		return "", nil, err
	}

	return nc, updatedResourceMap, nil
}

func (cc *cniContext) getNetworkConfig(networks []kc.NetworkConfig) (string, error) {
	var netConf bytes.Buffer
	var resourceMap map[string]*ResourceInfo

	netConf.WriteString("[")
	for i, podNet := range networks {
		if i != 0 {
			netConf.WriteString(",")
		}

		primary := false
		if !cc.auxNetOnly && podNet.IsPrimary {
			primary = true
		}

		nc, updatedResourceMap, err := cc.getDelegateNetConf(podNet.NetworkName, resourceMap, primary)
		if err != nil {
			return "", fmt.Errorf("Kactus: failed getting the netplugin: %v", err)
		}
		resourceMap = updatedResourceMap
		netConf.WriteString(nc)
	}
	netConf.WriteString("]")

	return netConf.String(), nil
}

func parseDelegatesNetConf(nc string) ([]map[string]interface{}, error) {
	delegateNetconf := netConf{}

	if nc == "" {
		return nil, fmt.Errorf("Kactus: CRD network object data can't be empty")
	}

	dec := json.NewDecoder(strings.NewReader("{\"delegates\": " + nc + "}"))
	dec.UseNumber()
	if err := dec.Decode(&delegateNetconf); err != nil {
		return nil, fmt.Errorf("Kactus: failed to load netconf: %v", err)
	}

	if delegateNetconf.Delegates == nil {
		return nil, fmt.Errorf("Kactus: \"delegates\" is must, refer Kactus README.md for the usage guide")
	}

	return delegateNetconf.Delegates, nil
}

func getPodNetworks(cniArgs *CNIArgs, k8sclient *kubernetes.Clientset) ([]kc.NetworkConfig, bool, *v1.Pod, error) {
	kc.LogDebug("getPodNetworks: cniArgs = '%+v'", cniArgs)
	networks := []kc.NetworkConfig{}
	if string(cniArgs.K8S_POD_NETWORK) != "" {
		// this is a network that got dynamically added to a Pod, kactus was invoked by the podagant
		podNet := kc.NetworkConfig{
			NetworkName: string(cniArgs.K8S_POD_NETWORK),
		}
		if mac := string(cniArgs.K8S_POD_IFMAC); mac != "" {
			podNet.IfMAC = mac
		}
		networks = append(networks, podNet)
		return networks, true, nil, nil
	}

	netAnnot, pod, err := getPodNetworkAnnotation(k8sclient, string(cniArgs.K8S_POD_NAMESPACE), string(cniArgs.K8S_POD_NAME))
	if err != nil {
		return nil, false, nil, err
	}

	if netAnnot == "" {
		networks = append(networks, kc.NetworkConfig{IsPrimary: true}) // fill this slot with an empty network
		kc.LogDebug("getPodNetworks: len(netAnnot) = 0, nonet\n")
		return networks, false, pod, nil
	}

	podNetworks := []kc.NetworkConfig{}
	if err := json.Unmarshal([]byte(netAnnot), &podNetworks); err != nil {
		err = fmt.Errorf("Kactus: failed to unmarshal pod network annotations '%q', err: %v", netAnnot, err)
		return nil, false, nil, err
	}

	return append(networks, podNetworks...), false, pod, nil
}

func (cc *cniContext) getDelegatesNetConf(networks []kc.NetworkConfig) ([]map[string]interface{}, error) {
	kc.LogDebug("getDelegatesNetConf: networks: %v\n", networks)
	networkConf, err := cc.getNetworkConfig(networks)
	if err != nil {
		return nil, err
	}
	kc.LogDebug("getDelegatesNetConf: networkConf %+v\n", networkConf)

	delegatesNetConf, err := parseDelegatesNetConf(networkConf)
	if err != nil {
		return nil, err
	}

	kc.LogDebug("getDelegatesNetConf: delegatesNetConf %+v\n", delegatesNetConf)
	return delegatesNetConf, nil
}

func validatePodNetworksConfig(networks []kc.NetworkConfig) (bool, error) {
	var havePrimary bool

	for _, podNet := range networks {
		if podNet.IsPrimary {
			if !havePrimary {
				havePrimary = true
			} else {
				return false, fmt.Errorf("Only one network can be primary")
			}
		}
		if podNet.IfMAC != "" {
			if _, err := net.ParseMAC(podNet.IfMAC); err != nil {
				return false, fmt.Errorf("Network %s has an invalid mac address %s: %v", podNet.NetworkName, podNet.IfMAC, err)
			}
		}
	}
	return havePrimary, nil
}

func getIfName(argsIfName string, delegate map[string]interface{}) string {
	var ifName string
	if isMasterplugin(delegate) || !isString(delegate["networkName"]) {
		ifName = argsIfName
	} else {
		ifName = kc.GetNetworkIfname(delegate["networkName"].(string))
	}
	return ifName
}

func (cc *cniContext) getResourceMap(no *netObject, resourceMap map[string]*ResourceInfo) (map[string]*ResourceInfo, string, string, error) {
	// Get resourceName annotation from the Network CR
	deviceID := ""
	resourceName, ok := no.GetAnnotations()[resourceNameAnnot]
	if !ok || cc.pod == nil || cc.pod.Name == "" || cc.pod.Namespace == "" {
		return resourceMap, deviceID, resourceName, nil
	}

	// ResourceName annotation is found; try to get device info from resourceMap
	kc.LogDebug("getResourceMap: found resourceName annotation : %s\n", resourceName)

	if resourceMap == nil {
		ck, err := GetResourceClient()
		if err != nil {
			return nil, deviceID, resourceName, fmt.Errorf("getResourceMap: failed to get a ResourceClient instance: %v", err)
		}
		resourceMap, err = ck.GetPodResourceMap(cc.pod)
		if err != nil {
			return resourceMap, deviceID, resourceName, fmt.Errorf("getResourceMap: failed to get resourceMap from ResourceClient: %v", err)
		}
		kc.LogDebug("getResourceMap: resourceMap instance: %+v\n", resourceMap)
	}

	entry, ok := resourceMap[resourceName]
	if ok {
		if idCount := len(entry.DeviceIDs); idCount > 0 && idCount > entry.Index {
			deviceID = entry.DeviceIDs[entry.Index]
			kc.LogDebug("getResourceMap: podName: %s deviceID: %s\n", cc.pod.Name, deviceID)
			entry.Index++ // increment Index for next delegate
		}
	}

	return resourceMap, deviceID, resourceName, nil
}

func cmdAdd(args *skel.CmdArgs) error {
	logBuildDetails()
	cniArgs := CNIArgs{}
	err := types.LoadArgs(args.Args, &cniArgs)
	if err != nil {
		kc.LogError("cmdAdd: args: %v Err in loading args: %v\n", args.Args, err)
		return err
	}
	kc.LogDebug("cmdAdd: args: %+v\n", string(args.StdinData[:]))
	nc, err := loadNetConf(args.StdinData)
	if err != nil {
		kc.LogError("cmdAdd: args: %v Err in loading netconf: %v\n", string(args.StdinData[:]), err)
		return fmt.Errorf("Kactus: Err in loading netconf: %v", err)
	}
	kc.LogDebug("cmdAdd: netconf %+v\n", nc)

	k8sclient, err := createK8sClient(nc.Kubeconfig)
	if err != nil {
		kc.LogError("cmdAdd: Err failed to create a k8s client: %v", err)
		return err
	}
	networks, auxNetOnly, pod, err := getPodNetworks(&cniArgs, k8sclient)
	if err != nil {
		err = fmt.Errorf("Kactus: Err in getting k8s network from pod: %v", err)
		kc.LogError("cmdAdd: %v\n", err)
		return err
	}
	havePrimary, err := validatePodNetworksConfig(networks)
	if err != nil {
		err = fmt.Errorf("Kactus: Err in the Pod networks configuration: %v", err)
		kc.LogError("cmdAdd: %v\n", err)
		return err
	}
	cc := cniContext{
		pod:        pod,
		cniArgs:    &cniArgs,
		auxNetOnly: auxNetOnly,
		k8sclient:  k8sclient,
	}
	kc.LogDebug("cmdAdd: len(networks) = %d, networks = '%+v'", len(networks), networks)
	if len(networks) > 0 && networks[0].NetworkName != "" {
		delegates, err := cc.getDelegatesNetConf(networks)
		if err != nil {
			kc.LogError("cmdAdd: %v\n", err)
			return err
		}
		if !havePrimary && !auxNetOnly {
			// Pod with networks annotations but with no primary network
			nc.Delegates = append(nc.Delegates, delegates...)
			networks = append(append([]kc.NetworkConfig{}, kc.NetworkConfig{IsPrimary: true}), networks...)
		} else {
			nc.Delegates = delegates
		}
	}

	kc.LogDebug("cmdAdd: len(nc.Delegates) = %d, nc.Delegates = '%+v'", len(nc.Delegates), nc.Delegates)
	var masterPluginEnabled bool
	for _, delegate := range nc.Delegates {
		// make sure we have only one master plugin among the delegates
		if err := checkDelegate(delegate, &masterPluginEnabled); err != nil {
			err = fmt.Errorf("Kactus: Err in delegate conf: %v", err)
			kc.LogError("cmdAdd: %v\n", err)
			return err
		}
	}

	var result, r types.Result
	idx := -1
	for i, delegate := range nc.Delegates {
		idx = i
		if nc.CNIVersion != "" {
			delegate["cniVersion"] = nc.CNIVersion
		}
		err, r = cc.delegateAdd(networks[i], args.IfName, delegate)
		if err != nil {
			kc.LogError("cmdAdd: %v\n", err)
			break
		}
		// among the list picks the result related to eth0
		// interface or to an auxiliary interface in case
		// kactus was invoked by the podagent
		if result == nil {
			result = r
		}
	}

	if err != nil {
		cc.clearPlugins(idx, args.IfName, nc.Delegates)
		return err
	}
	// should not happens
	if result == nil {
		err = fmt.Errorf("Kactus: result is nil, this is not expected")
		kc.LogError("cmdAdd: %v\n", err)
		cc.clearPlugins(idx, args.IfName, nc.Delegates)
		return err
	}

	_, err = saveDelegates(args.ContainerID, nc.CNIDir, true, nc.Delegates)
	if err != nil {
		err = fmt.Errorf("Kactus: Err in saving the delegates: %v", err)
		kc.LogError("cmdAdd: %v\n", err)
		return err
	}

	kc.LogInfo("cmdAdd: delegated the creation of networks %+v\n", networks)

	return result.Print()
}

func cmdDel(args *skel.CmdArgs) error {
	var result error

	logBuildDetails()
	cniArgs := CNIArgs{}
	err := types.LoadArgs(args.Args, &cniArgs)
	if err != nil {
		kc.LogError("cmdDel: args: %v Err in loading args: %v\n", args.Args, err)
		return err
	}
	kc.LogDebug("cmdDel: args: %+v\n", string(args.StdinData[:]))
	nc, err := loadNetConf(args.StdinData)
	if err != nil {
		kc.LogError("cmdDel: args: %v Err in loading netconf: %v\n", string(args.StdinData[:]), err)
		return fmt.Errorf("Kactus: Err in loading netconf: %v", err)
	}
	kc.LogDebug("cmdDel: netconf %+v\n", nc)

	k8sclient, err := createK8sClient(nc.Kubeconfig)
	if err != nil {
		kc.LogError("cmdDel: Err failed to create a k8s client: %v", err)
		return err
	}
	networks, auxNetOnly, pod, err := getPodNetworks(&cniArgs, k8sclient)
	if err != nil {
		podsNotFoundErr := fmt.Sprintf("pods \"%s\" not found", cniArgs.K8S_POD_NAME)
		if strings.HasSuffix(err.Error(), podsNotFoundErr) {
			kc.LogDebug("cmdDel: %v, assume the pod is gone\n", err)
			return nil
		}
		err = fmt.Errorf("Kactus: Err in getting k8s network from pod: %v", err)
		kc.LogError("cmdDel: %v\n", err)
		return err
	}
	cc := cniContext{
		pod:        pod,
		cniArgs:    &cniArgs,
		auxNetOnly: auxNetOnly,
		k8sclient:  k8sclient,
	}
	kc.LogDebug("cmdDel: len(networks) = %d, networks = '%+v'", len(networks), networks)

	netconfBytes, err := consumeScratchNetConf(args.ContainerID, nc.CNIDir)
	if err != nil {
		kc.LogDebug("Can't read container netconf file: %v\n", err)
		return nil
	}
	// set delegates to nil to make sure there is not leftover from loadNetConf
	nc.Delegates = nil
	if err := json.Unmarshal(netconfBytes, &nc.Delegates); err != nil {
		err = fmt.Errorf("Kactus: failed to load netconf: %v", err)
		kc.LogError("cmdDel: %v\n", err)
		return err
	}

	kc.LogDebug("cmdDel: nc.Delegates = '%+v'", nc.Delegates)
	var delegateToDelete, remainingDelegates []map[string]interface{}
	for _, delegate := range nc.Delegates {
		if delegate["networkName"] == nil || !isString(delegate["networkName"]) {
			if !auxNetOnly {
				delegateToDelete = append(delegateToDelete, delegate)
				// masterPlugin with no Pod networks annotation
				continue
			}
		}
		netFound := false
		for _, network := range networks {
			if delegate["networkName"] == network.NetworkName {
				delegateToDelete = append(delegateToDelete, delegate)
				netFound = true
				break
			}
		}
		if !netFound {
			remainingDelegates = append(remainingDelegates, delegate)
		}
	}
	saveDelegates(args.ContainerID, nc.CNIDir, false, remainingDelegates)
	nc.Delegates = delegateToDelete

	for _, delegate := range nc.Delegates {
		err := cc.delegateDel(args.IfName, delegate)
		if err != nil {
			kc.LogError("cmdDel: %v\n", err)
			return err
		}
		result = err
	}

	kc.LogInfo("cmdDel: delegated the deletion networks %+v\n", networks)
	return result
}

func cmdCheck(args *skel.CmdArgs) error {
	// TODO: implement
	return fmt.Errorf("not implemented")
}

func main() {
	logParams := kc.LoggingParams{
		Prefix: "KACTUS ",
	}
	// will get a file object if _CNI_LOGGING_LEVEL environment variable is
	// set to a value >= 1, otherwise logging goes to /dev/null
	lf := kc.OpenLogFile(&logParams)
	defer kc.CloseLogFile(lf)

	// Makes sure we recover upon panic to not lose any logs, etc
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("panic: %v\n%v", r, string(debug.Stack()))
			e := types.NewError(types.ErrInternal, msg, "")
			if err := e.Print(); err != nil {
				kc.LogError("Error writing error JSON to stdout: %w", err)
			}
			os.Exit(1)
		}
	}()

	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All,
		"meta-plugin that delegates to other CNI plugins")
}
