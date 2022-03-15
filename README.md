# INTRODUCTION

This repo contains the source code of Kaloom's Kubernetes kactus cni-plugin

Kactus is Kaloom's multi network cni-plugin, it's a meta plugin that delegate the addition/deletion of network device(s) in a Pod to other cni-plugins. kactus started in 2017 from [multus](https://github.com/intel/multus-cni) as a base to fulfill use cases needed for Kaloom Software Defined Fabric (KSDF) product.

When kactus is configured as the system cni-plugin (i.e. the first config in lexical order under `/etc/cni/net.d/`), it can be invoked (via the [podagent](https://github.com/kaloom/kubernetes-podagent)) after a Pod/Deployement/DaemonSet/StatefulSet get deployed in order to add/delete network devices in a running Pod without restarting the latter.

In order to do that, kactus uses Pod's network attachment annotations to associate a network attachment name to a network device in a Pod. A network attachment related configurations is a resource in Kubernetes (a CustomResourceDefinition) that kactus can fetch by calling kubernetes' apiserver.

## Consistent network devices naming

The first network device in a Pod (i.e. the one attached to the default network) is called `eth0`, auxiliary network devices (i.e. the ones not attached to the default network) would uses a consistent network device name based on the name of the network attachment.

### Why this is needed:

* To support multiple network devices attached to networks where these devices can be created/deleted dynamically, network device ordering would not works (e.g. `eth`\<X\> or `net-`\<X\> multus’s way)
* To provide a 1-to-1 mapping between network attachment and a device so that different users (agents and application) have a way to map a network attachment to a device

We use a function that given a network attachment name (key) would return the device name associated with it in a Pod. Currently the function we use would prefix a device name with “net” and add to it the first 13 characters of the md5 digest of the network attachment name; given that the max. size of a device name in linux is 15 characters. There is a small chance of collision but it’s probability minimal

### How the podagent communicate the addition/deletion of a network attachment into a running Pod

When the podagent detects that there is addition/deletion of a network attachment in a Pod’s annotation, it would invoke the system cni-plugin (e.g. kactus) with augmented CNI_ARGS (K8S_POD_NETWORK=<network-attachment-name>) that includes the network attachment name to be added/deleted, kactus than uses a hash function to map a network attachment to a device in the Pod

### Additional attributes for the network attachment config annotations in Pods

* To support Pods that would prefer to have a fixed mac address and where it would be expensive if the mac address got changed (a Pod that get re-started on a different node, vrouters for ex.) we added an optional ifMac attribute to the network attachment annotation ( ex. `‘[ { “name”: “mynet”, “ifMac”: “00:11:22:33:44:55”} ]’` )
* When multiples network devices exists in a Pod you might want to override the default network configuration with a one defined in kubernetes network resource definition where a set of subnets would be routed over it and where the default gateway would not be on `eth0`, to support this use case, an optional attribute to the network annotation is provided ( ex. `‘[ { “name”: “mydefaultnet”, “ifMac”: “00:11:22:33:44:55”, “isPrimary”: true} ]’` )

# kactus cni-plugin config file

kactus cni-plugin configuration follows the cni [specification](https://github.com/containernetworking/cni/blob/master/SPEC.md)

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
* `kubeconfig` (string, optional): kubeconfig file to use in order to authenticate with kubernetes apiserver, if it's missing the in-cluster authentication will be used.
* `delegates` (array, required): an array of delegate object, a delegate object is specific to the latter; the example show a delegate config specific to flannel. A delegate object may contains a `masterPlugin` (boolean, optional) that specify which cni-plugin in the array will be responsible to setup the default network attachment on `eth0`; only one delegate may have `masterPlugin` set to `true`, if `masterPlugin` is not specified it's value would default to `false`.

# HOW TO BUILD

> `./build.sh`

## For developpers:

The repo uses `go mod` to manage go packages dependencies, if you're importing a new go package you need to:
* > `go mod tidy`
* > `cd kactus && go build -mod=mod`
* > `go mod vendor`
* submit a merge request

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

As pre-requiste:
1. make sure that you have setup your Kubernetes cluster with kactus cni-plugin, see the Setup section for details.
2. install the `null` ipam cni-plugin see [setup null cni-plugin](https://github.com/kaloom/kubernetes-null-cni-plugin/blob/master/README.md)

For the sake of simplicity, the networking technologies we're going to use in order to isolate the L2 networks is vlan (i.e. IEEE 802.1Q) where the master network device on the host is `eth0` (if the network device on the host is not `eth0` you need to update `examples/green-net.yaml` and `examples/blue-net.yaml`)

## create 2 Pod each of which is attached to 2 networks `green` and `blue`, these would be available at Pod startup

We need to provision first the 2 network attachment resources `geen` and `blue` in kubernetes. The `green` network attachment will have a vlan id 42 and the `blue` network attachment will have a vlan id 43.

provision the `green` network attachment from the `examples/green-net.yaml` spec. file

> $ `kubectl apply -f examples/green-net.yaml`

provision the `blue` network attachment from the `examples/blue-net.yaml` spec. file

> $ `kubectl apply -f examples/blue-net.yaml`

The app1 kubernetes Deployement spec file in `examples/app1.yaml` contains one container with one Pod instance, the container image for this example is Linux alpine.

The annotation in the spec. file defines that we want to start the Pod with 2 additional network devices attached to the `green` and `blue`:

```
networks: '[ { "name": "green" }, { "name": "blue" } ]'
```

### create the app1 deployement

> $ `kubectl apply -f examples/app1.yaml`

if we check the app1 Pod, we should see in addition to `lo` and `eth0`, two network devices associated with the network attachments `green` and `blue`:

> $ `kubectl exec -t $(kubectl get pod -l app=app1 -o jsonpath='{.items[*].metadata.name}') -- ip a`

```
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN qlen 1
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host
       valid_lft forever preferred_lft forever
3: eth0@if48: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1450 qdisc noqueue state UP
    link/ether 76:d0:a6:af:66:e2 brd ff:ff:ff:ff:ff:ff
    inet 10.244.2.39/24 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::74d0:a6ff:feaf:66e2/64 scope link
       valid_lft forever preferred_lft forever
4: net9f27410725ab@if2: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP
    link/ether 52:54:00:ac:3c:ca brd ff:ff:ff:ff:ff:ff
    inet 192.168.42.10/24 scope global net9f27410725ab
       valid_lft forever preferred_lft forever
    inet6 fd10:42::1/64 scope global
       valid_lft forever preferred_lft forever
    inet6 fe80::5054:ff:feac:3cca/64 scope link
       valid_lft forever preferred_lft forever
5: net48d6215903df@if2: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP
    link/ether 52:54:00:ac:3c:ca brd ff:ff:ff:ff:ff:ff
    inet 192.168.43.10/24 scope global net48d6215903df
       valid_lft forever preferred_lft forever
    inet6 fd10:43::1/64 scope global
       valid_lft forever preferred_lft forever
    inet6 fe80::5054:ff:feac:3cca/64 scope link
       valid_lft forever preferred_lft forever
```

Note: the network device attached to the `green` network attachment can be found:

> $ `(echo -n net; echo -n green | md5sum - | cut -b1-12)`

```
net9f27410725ab
```

and for the device attached to the `blue` network attachment:

> $ `(echo -n net; echo -n blue | md5sum - | cut -b1-12)`

```
net48d6215903df
```

### create the app2 deployment

> $ `kubectl apply -f examples/app2.yaml`

if we check the app2 Pod, we should see in addition to `lo` and `eth0`, two network devices associated with the network attachments `green` and `blue`:

> $ `kubectl exec -t $(kubectl get pod -l app=app2 -o jsonpath='{.items[*].metadata.name}') -- ip a`

```
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN qlen 1
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host
       valid_lft forever preferred_lft forever
3: eth0@if60: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1450 qdisc noqueue state UP
    link/ether 5a:81:c0:8a:09:58 brd ff:ff:ff:ff:ff:ff
    inet 10.244.1.50/24 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::5881:c0ff:fe8a:958/64 scope link
       valid_lft forever preferred_lft forever
4: net9f27410725ab@if2: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP
    link/ether 52:54:00:30:ad:93 brd ff:ff:ff:ff:ff:ff
    inet 192.168.42.20/24 scope global net9f27410725ab
       valid_lft forever preferred_lft forever
    inet6 fd10:42::2/64 scope global
       valid_lft forever preferred_lft forever
    inet6 fe80::5054:ff:fe30:ad93/64 scope link
       valid_lft forever preferred_lft forever
5: net48d6215903df@if2: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP
    link/ether 52:54:00:30:ad:93 brd ff:ff:ff:ff:ff:ff
    inet 192.168.43.20/24 scope global net48d6215903df
       valid_lft forever preferred_lft forever
    inet6 fd10:43::2/64 scope global
       valid_lft forever preferred_lft forever
    inet6 fe80::5054:ff:fe30:ad93/64 scope link
       valid_lft forever preferred_lft forever
```

### test connectivity over `green` and `blue` network attachments

We should be able now to `ping 192.168.42.20` from app1

> `kubectl exec -t $(kubectl get pod -l app=app1 -o jsonpath='{.items[*].metadata.name}') -- ping -c3 192.168.42.20`

```
PING 192.168.42.20 (192.168.42.20): 56 data bytes
64 bytes from 192.168.42.20: seq=0 ttl=64 time=0.452 ms
64 bytes from 192.168.42.20: seq=1 ttl=64 time=0.249 ms
64 bytes from 192.168.42.20: seq=2 ttl=64 time=0.501 ms

--- 192.168.42.20 ping statistics ---
3 packets transmitted, 3 packets received, 0% packet loss
```

from app2 we should also be able to `ping 192.168.43.10` as well

> `kubectl exec -t $(kubectl get pod -l app=app2 -o jsonpath='{.items[*].metadata.name}') -- ping -c3 192.168.43.10`

```
PING 192.168.43.10 (192.168.43.10): 56 data bytes
64 bytes from 192.168.43.10: seq=0 ttl=64 time=0.298 ms
64 bytes from 192.168.43.10: seq=1 ttl=64 time=0.493 ms
64 bytes from 192.168.43.10: seq=2 ttl=64 time=0.333 ms

--- 192.168.43.10 ping statistics ---
3 packets transmitted, 3 packets received, 0% packet loss
round-trip min/avg/max = 0.298/0.374/0.493 ms
```

# Debugging

To help trouble-shooting, add to the `[Service]` section of `/etc/systemd/system/kubelet.service` the following environment variables:

```
Environment="_CNI_LOGGING_LEVEL=3"
Environment="_CNI_LOGGING_FILE=/var/log/cni.log"
```

> $ `sudo systemctl daemon-reload`

> $ `sudo systemctl restart kubelet`

now logs related to kactus will be sent to `/var/log/cni.log`
