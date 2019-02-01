# Alibaba Cloud Disk Snapshot Plugin

[![Build Status](https://travis-ci.org/AliyunContainerService/disk-snapshot.svg?branch=master)](https://travis-ci.org/AliyunContainerService/disk-snapshot) [![CircleCI](https://circleci.com/gh/AliyunContainerService/disk-snapshot.svg?style=svg)](https://circleci.com/gh/AliyunContainerService/disk-snapshot) [![Go Report Card](https://goreportcard.com/badge/github.com/AliyunContainerService/disk-snapshot)](https://goreportcard.com/report/github.com/AliyunContainerService/disk-snapshot)


## Overview

An Disk snapshot plugin is available to help simplify storage management.
Once user creates VolumeSnapshot with the reference to a snapshot class, snapshot and
corresponding VolumeSnapshotContent object gets dynamically created and becomes ready to be used by
workloads. 

This implementation does not depend on CSI feature, but use the csi-snapshot designs which will be used as Alpha feature in k8s 1.13.

Disk Snapshot Plugin will create 3 crds as below.

```
volumesnapshotclasses.snapshot.storage.k8s.io:  define details to create volumesnapshotcontents, like: storageclass;
volumesnapshotcontents.snapshot.storage.k8s.io: define one disk snapshot with the backend, like: pv;
volumesnapshots.snapshot.storage.k8s.io:        the claim of one snapshot, like: pvc;
```

## Requirements

* Disk Snapshot plugin depends on external-snapshotter (registry.cn-hangzhou.aliyuncs.com/plugins/external-snapshotter:snapshot).
* PVC Datasource feature depends on lastest Disk-Controller (registry-vpc.cn-shenzhen.aliyuncs.com/acs/alicloud-disk-controller:v1.11.2.8-ffc6b5b-aliyun).
* Secret object with the authentication key for Disk
* Service Accounts with required RBAC permissions

### Feature Status
Alpha

## Compiling and Package
disk-snapshot can be compiled in a form of a container.

To build a container:
```
$ cd build && sh build.sh
```

## Deploy

### Get Kubernetes cluster

You can create a Kubernetes Cluster on [Alibaba cloud Container Service](https://help.aliyun.com/product/25972.html?spm=a2c4g.750001.2.3.A7g9FZ)

Kubernetes should be 1.12 or after.

### Config api-server、controller manager、scheduler compnents

Edit files as below:

```
/etc/kubernetes/manifests/kube-apiserver.yaml
/etc/kubernetes/manifests/kube-controller-manager.yaml
/etc/kubernetes/manifests/kube-scheduler.yaml
Add:
 - --feature-gates=CSINodeInfo=true,CSIDriverRegistry=true,VolumeSnapshotDataSource=true
```

> Notes：
> 
> cp /etc/kubernetes/manifests/kube-apiserver.yaml /tmp/kube-apiserver.yaml
> 
> edit /tmp/kube-apiserver.yaml
> 
> cp /tmp/kube-apiserver.yaml /etc/kubernetes/manifests/kube-apiserver.yaml

#### Config Kubelet

```
# vi /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
Add:
Environment="KUBELET_FEATURE_GATES_ARGS=--feature-gates=CSINodeInfo=true,CSIDriverRegistry=true,VolumeSnapshotDataSource=true"
```
restart kubelet: 

	systemctl daemon-reload && service kubelet restart


### Deploy Snapshot Controller
If the cluster not in STS mode, you need to config AK info to plugin; Set ACCESS_KEY_ID, ACCESS_KEY_SECRET to environment;

```
# kubectl create -f ./deploy/snapshot-controller.yaml

# kubectl get pod -n kube-system | grep alicloud-disk-snapshot
alicloud-disk-snapshot-89c88c7f7-6z95j                       2/2     Running   0          42s
```

### Deploy SnapshotClass
```
# kubectl create -f ./deploy/snapshotclass.yaml

# kubectl get volumesnapshotclasses.snapshot.storage.k8s.io
NAME                AGE
default-snapclass   15s
```

3 CRDs be created：

```
# kubectl get crd | grep snap
volumesnapshotclasses.snapshot.storage.k8s.io    2019-01-16T12:00:46Z
volumesnapshotcontents.snapshot.storage.k8s.io   2019-01-16T12:00:46Z
volumesnapshots.snapshot.storage.k8s.io          2019-01-16T12:00:46Z
```

## Usage

### Create Pod with Disk
Create a deployment and pvc, using disk volume.

```
# kubectl create -f ./deploy/deploy-origin.yaml

# kubectl get pod
NAME                              READY   STATUS    RESTARTS   AGE
dynamic-create-7fbf55b58f-qnrgm   1/1     Running   0          21s

# kubectl get pvc
NAME       STATUS   VOLUME                   CAPACITY   ACCESS MODES   STORAGECLASS        AGE
pvc-disk   Bound    d-wz9hhnhxu66zs6r6yyyq   20Gi       RWO            alicloud-disk-ssd   25s

# kubectl exec -ti dynamic-create-7fbf55b58f-qnrgm touch /data/snapshot123
```


### Step 4: Create Snapshot with Pvc
```
# kubectl create -f snapshot.yaml
volumesnapshot.snapshot.storage.k8s.io/snapshot-test created

# kubectl get volumesnapshots.snapshot.storage.k8s.io
NAME            AGE
snapshot-test   2m

# kubectl get volumesnapshotcontents.snapshot.storage.k8s.io
NAME                                               AGE
snapcontent-77a75b63-1987-11e9-a520-00163e024341   2m
```

> snapshot.yaml define the data source pvc name, and snapshotClass name.


### Create Pvc with Snapshot

```
# kubectl create -f deploy-datasource.yaml
# kubectl get pod
NAME                               READY   STATUS    RESTARTS   AGE
dynamic-create-7fbf55b58f-qnrgm    1/1     Running   0          10m
dynamic-snapshot-fdc86b6d8-bl6h5   1/1     Running   0          13s

# kubectl get pvc
NAME            STATUS   VOLUME                   CAPACITY   ACCESS MODES   STORAGECLASS        AGE
disk-snapshot   Bound    d-wz98cj3vphycosrm6joa   20Gi       RWO            alicloud-disk-ssd   4s
pvc-disk        Bound    d-wz9hhnhxu66zs6r6yyyq   20Gi       RWO            alicloud-disk-ssd   10m

# kubectl exec -ti dynamic-snapshot-fdc86b6d8-bl6h5 ls /data
lost+found  snapshot123
```

New pvc create a new disk, which using the snapshot. And the new disk contains old disk file(snapshot123).


## Troubleshooting

