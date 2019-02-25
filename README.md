# INTRODUCTION

This repo contains the source code of Kaloom's Kubernetes kactus cni-plugin

## TODO
more description about kactus

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

How to deploy the `kactus`

## As DaemonSet

1. create a Kubernetes service account, cluster role and cluster role binding for kactus cni-plugin:

> $ `kubectl apply -f manifests/kactus-serviceaccount-and-rbac.yaml`

2. create the `kactus-kubeconfig.yaml` file:

> $ `./scripts/create-kubeconfig.sh`

3. copy the produced `/tmp/kubeconfig/kactus-kubeconfig.yaml` to each node in Kubernetes cluster under `/etc/cni/net.d/kactus.d/`

> $ `sudo install -d 755 /etc/cni/net.d/kactus.d`

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
