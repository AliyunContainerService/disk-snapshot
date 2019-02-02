/*
Copyright 2018 The Kubernetes Authors.

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

package controller

import (
	"os"
	"path"

	log "github.com/Sirupsen/logrus"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/drivers/pkg/csi-common"
)

// PluginFolder defines the location of disk-snapshot
const (
	PluginFolder = "/var/lib/kubelet/plugins/disk-snapshot"
	driverName   = "disk-snapshot"
)

var (
	version = "1.0.0"
)

type identityServer struct {
	*csicommon.DefaultIdentityServer
}

type disk struct {
	driver           *csicommon.CSIDriver
	endpoint         string
	idServer         *identityServer
	controllerServer *controllerServer

	cap   []*csi.VolumeCapability_AccessMode
	cscap []*csi.ControllerServiceCapability
}

// Init checks for the persistent volume file and loads all found volumes
// into a memory structure
func initDriver() {
	if _, err := os.Stat(path.Join(PluginFolder, "controller")); os.IsNotExist(err) {
		log.Infof("disk: folder %s not found. Creating... \n", path.Join(PluginFolder, "controller"))
		if err := os.Mkdir(path.Join(PluginFolder, "controller"), 0755); err != nil {
			log.Fatalf("Failed to create a controller's volumes folder with error: %v\n", err)
		}
		return
	}
}

func NewDriver(nodeID, endpoint string) *disk {
	initDriver()
	tmpdisk := &disk{}
	tmpdisk.endpoint = endpoint

	csiDriver := csicommon.NewCSIDriver(driverName, version, nodeID)
	tmpdisk.driver = csiDriver

	tmpdisk.driver.AddControllerServiceCapabilities([]csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
		csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
	})
	tmpdisk.driver.AddVolumeCapabilityAccessModes([]csi.VolumeCapability_AccessMode_Mode{csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER})

	return tmpdisk
}

func NewIdentityServer(d *csicommon.CSIDriver) *identityServer {
	return &identityServer{
		DefaultIdentityServer: csicommon.NewDefaultIdentityServer(d),
	}
}

func NewControllerServer(d *csicommon.CSIDriver) *controllerServer {
	regionId := GetMetaData(REGIONID_TAG)
	return &controllerServer{
		DefaultControllerServer: csicommon.NewDefaultControllerServer(d),
		RegionId:                regionId,
	}
}

func (disk *disk) Run() {
	log.Infof("Snapshot Driver: %v version: %v", driverName, version)

	// Create GRPC servers
	disk.idServer = NewIdentityServer(disk.driver)
	disk.controllerServer = NewControllerServer(disk.driver)

	s := csicommon.NewNonBlockingGRPCServer()
	s.Start(disk.endpoint, disk.idServer, disk.controllerServer, nil)
	s.Wait()
}
