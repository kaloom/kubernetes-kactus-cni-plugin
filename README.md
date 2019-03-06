# INTRODUCTION

This repo contains the source code of Kaloom's Kubernetes kactus cni-plugin

Kactus is Kaloom's multi network cni-plugin, it's a meta plugin that delegate the addition/deletion of network device(s) in a Pod to other cni-plugins. kactus started in 2017 from [multus](https://github.com/intel/multus-cni) as a base to fulfill use cases needed for Kaloom Software Defined Fabric (KSDF) product.

When kactus is configured as the system cni-plugin (i.e. the first config in lexical order in `/etc/cni/net.d/`), it can be invoked (via the [podagent](https://github.com/kaloom/kubernetes-podagent)) after a Pod/Deployement/DaemonSet/StatefulSet get deployed in order to add/delete network devices in a running Pod without restarting the latter.

In order to do that, kactus uses Pod's network attachment annotations to associate a network name to a network device in a Pod. A network attachment related configurations is a resource in Kubernetes (a CustomResourceDefinition) that kactus can fetch by calling kubernetes' apiserver.

## Consistent network devices naming

The first network device in a Pod (i.e. the one attached to the default network) is called eth0, auxiliary network devices (i.e. the ones not attached to the default network) would uses a consistent network device name based on the name of the network attachment name.

### Why we this is needed:

* To support multiple network devices attached to networks where these devices can be created/deleted at runtime, network device ordering would not works (eth<X> or net-<X> multus’s way)
* To provide a 1-to-1 mapping between network attachment and a device so that different users (agents and application) have a way to map a network attachment to a device

We use a function that given a network attachment name (key) would return the device name associated with it in a Pod. Currently the function we use would prefix a device name with “net” and add to it the first 13 characters of the md5 digest of the network attachment name; given that the max. size of a device name in linux is 15 characters. There is a small chance of collision but it’s probability minimal

### How the podagent communicate the addition/deletion of a network attachement into a running Pod

When the podagent detects that there is addition/deletion of a network attachement in a Pod’s annotation, it would invoke the master plugin (e.g. kactus) with augmented CNI_ARGS (K8S_POD_NETWORK=<network-attachment-name>) that includes the network attachment name to be added/deleted, kactus than use the hash function to map a network to a device in the Pod

### Additional attributes for the network attachment config annotations in Pods

* To support Pods that would prefer to have a fixed mac address and where it would be expensive if the mac address got changed (a Pod that get re-started on a different node, vrouters for ex.) we added an optional ifMac attribute to the network attachment annotation ( ex. ‘[ { “name”: “mynet”, “ifMac”: “00:11:22:33:44:55”} ]’ )
* When multiples network devices exists in a Pod you might want to override the default network configuration with a one defined in kubernetes network resource definition where a set of subnets would be routed over it and where the default gateway would not be on eth0, to support this use case, an optional attribute to the network annotation is provided ( ex. ‘[ { “name”: “mydefaultnet”, “ifMac”: “00:11:22:33:44:55”, “isPrimary”: true} ]’ )

# kactus cni-plugin config file

kactus cni-plugin configuration follows the cni [specification] (https://github.com/containernetworking/cni/blob/master/SPEC.md)

## Example configuration

```
{
  "name": "kactus-net",
  "type": "kactus",
  "kubeconfig": "/etc/cni/net.d/kactus.d/kactus-kubeconfig.yaml",
  "delegates": [
    {
      "type": "flannel",
      "masterPlugin": true,
      "delegate": {
        "isDefaultGateway": true
      }
    }
  ]
}
```

## Network configuration reference

* `name` (string, required): the name of the network.
* `type` (string, required): "kactus".
* `kubeconfig` (string, optional): kubeconfig file to use in order to authenticate with kubernetes apiserver, if it's missing the in-cluster authentication will be used
* `delegates` (array, required): an array of delegate object, a delegate object is specific to the latter; the example show a delegate config specific to flannel. A delegate object may contains a `masterPlugin` (boolean, optional) that specify which cni-plugin in the array will be responsible to setup the default network attachment on eth0; only one delegate may have `masterPlugin` set to `true`, if `masterPlugin` is not specified it's value would default to `false`

# HOW TO BUILD

> `./build.sh`

## For developpers:

if you're adding a new dependency package to the project you need to use `gradle`, otherwise running the `./build.sh` script should do

`gradle` required `java` to be installed, its used to generate the dependencies (using `gogradle` plugin), update the `gogradle.lock`, build the project and update the go `vendor` directory if needed

* update build.gradle
* generate a new `gogradle.lock` file:
  > `./gradlew lock`
* build the project (the `build` gradle task would trigger an update to the `vendor` directory using the `gogradle.lock` if needed):
  > `./gradlew build`

  or simply

  > `./gradlew`
* submit a merge request


### other useful info:

* updating only the vendor directory can be done with:
  > `./gradlew vendor`
* to get a list of available `gradle` tasks:
  > `./gradlew tasks`

# Setup

How to deploy `kactus`

## As DaemonSet

1. create a Kubernetes service account, cluster role and cluster role binding for kactus cni-plugin:

> $ `kubectl apply -f manifests/kactus-serviceaccount-and-rbac.yaml`

2. create the `kactus-kubeconfig.yaml` file:

> $ `./scripts/create-kubeconfig.sh`

3. copy the produced `/tmp/kubeconfig/kactus-kubeconfig.yaml` to each node in Kubernetes cluster under `/etc/cni/net.d/kactus.d/`

> $ `sudo install -m 755 -d /etc/cni/net.d/kactus.d`

> $ `sudo cp /tmp/kubeconfig/kactus-kubeconfig.yaml /etc/cni/net.d/kactus.d`

4. delopy kactus as a daemon set:

> $ `kubectl apply -f manifests/kactus-ds.yaml`

5. create the network CRD

> $ `kubectl apply -f manifests/network-crd.yaml`

### Note
Currently, to deploy kactus as DaemonSet
* *selinux* should not be in *enforced* mode (*permissive* mode is okay):
  > \# `setenforce permissive`

  > \# `sed -i 's/^SELINUX=.*/SELINUX=permissive/g' /etc/selinux/config`

* in `manifests/kactus-ds.yaml` the delegatation of creating the default network attachment is set to flannel, double check the configuration so it matches with the way things are deployed in your cluster

# Example

