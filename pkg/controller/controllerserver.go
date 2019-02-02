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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/kubernetes-csi/drivers/pkg/csi-common"
	"github.com/nightlyone/lockfile"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type controllerServer struct {
	EcsClient *ecs.Client
	RegionId  string
	*csicommon.DefaultControllerServer
}

type diskSnapshot struct {
	Name         string              `json:"name"`
	Id           string              `json:"id"`
	VolumeID     string              `json:"volumeID"`
	Path         string              `json:"path"`
	CreationTime timestamp.Timestamp `json:"creationTime"`
	SizeBytes    int64               `json:"sizeBytes"`
	ReadyToUse   bool                `json:"readyToUse"`
}

const (
	KB_BYTES = 1024
	MB_BYTES = KB_BYTES * 1024
	GB_BYTES = MB_BYTES * 1024
	TB_BYTES = GB_BYTES * 1024
	KB_LABEL = "Ki"
	MB_LABEL = "Mi"
	GB_LABEL = "Gi"
	TB_LABEL = "Ti"
)

var diskVolumeSnapshots = map[string]*diskSnapshot{}

//
func (cs *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	log.Infof("Starting to Create Disk Snapshot: %s, %s, %s, %s", req.Name, req.SourceVolumeId, req.Parameters, req.Secrets)

	// Step 1: Check arguments
	if err := cs.Driver.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT); err != nil {
		return nil, err
	}
	if len(req.GetName()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Name missing in request")
	}
	if len(req.GetSourceVolumeId()) == 0 {
		log.Errorf("Snapshot: create snapshot error with volumeId empty")
		return nil, status.Error(codes.InvalidArgument, "SourceVolumeId missing in request")
	}
	CapacityBytes := req.Parameters["storage"]
	if len(CapacityBytes) < 3 {
		return nil, fmt.Errorf("get CapacityBytes error1: %s", CapacityBytes)
	}
	capacityTmp, err := strconv.ParseInt(CapacityBytes[0:len(CapacityBytes)-2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("get CapacityBytes error2: %s", CapacityBytes)
	}
	var capacity int64 = 0
	if strings.HasSuffix(CapacityBytes, GB_LABEL) {
		capacity = capacityTmp * int64(GB_BYTES)
	} else if strings.HasSuffix(CapacityBytes, KB_LABEL) {
		capacity = capacityTmp * int64(KB_BYTES)
	} else if strings.HasSuffix(CapacityBytes, MB_LABEL) {
		capacity = capacityTmp * int64(MB_BYTES)
	} else if strings.HasSuffix(CapacityBytes, TB_LABEL) {
		capacity = capacityTmp * int64(TB_BYTES)
	} else {
		return nil, fmt.Errorf("Get CapacityBytes error3: %s", CapacityBytes)
	}

	// Step 2: init ecs client.
	cs.initEcsClient()
	if cs.EcsClient == nil {
		log.Errorf("Snapshot: init ecs client error while create snapshot")
		return nil, fmt.Errorf("init ecs client error while create snapshot")
	}

	// Setp 3: Set lock
	// multi disk attach at the same time
	lockfileName := "lockfile-" + req.Name + ".lck"
	lock, err := lockfile.New(filepath.Join(os.TempDir(), lockfileName))
	if err != nil {
		return nil, fmt.Errorf("New lockfile error: %s, %s ", lockfileName, err.Error())
	}
	err = lock.TryLock()
	if err != nil {
		return nil, fmt.Errorf("Try lock error: %s, %s ", lockfileName, err.Error())
	}
	defer lock.Unlock()

	// use autosnapshotPolicy is meaningless, ecs auto create snapshots but kubernetes not know
	//cs.doWithAutoSnapshotPolicy(ctx)

	// Step 4: check snapshot is created or not.
	if exSnap, err := cs.getSnapshotByName(req.GetName()); err == nil {
		if exSnap.VolumeID == req.GetSourceVolumeId() {
			if exSnap.ReadyToUse == false {
				snapshotResponse, _ := cs.describeDiskSnapshot(ctx, req.GetName())
				if snapshotResponse != nil {
					log.Infof("Describe Snapshot with response: %v", snapshotResponse)
				}
				if snapshotResponse != nil && len(snapshotResponse.Snapshots.Snapshot) != 0 {
					snapshotInfo := snapshotResponse.Snapshots.Snapshot[0]
					if snapshotInfo.Status == "accomplished" {
						exSnap.ReadyToUse = true
					} else if snapshotInfo.Status != "progressing" {
						return nil, fmt.Errorf("Create Snapshot error, with status: %s ", snapshotInfo.Status)
					}
				}
			}
			log.Infof("Snapshot already created, snapshotName: %s, snapshotInfo: %v", req.GetName(), exSnap)
			return &csi.CreateSnapshotResponse{
				Snapshot: &csi.Snapshot{
					SnapshotId:     exSnap.Id,
					SourceVolumeId: exSnap.VolumeID,
					CreationTime:   &exSnap.CreationTime,
					SizeBytes:      exSnap.SizeBytes,
					ReadyToUse:     exSnap.ReadyToUse,
				},
			}, nil
		}

		// the exist snapshot name used to other disk volume.
		log.Errorf("Snapshot: Snapshot already used by other volumeId, %s, snapshotInfo: %v", req.GetName(), exSnap)
		return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("snapshot with the same name: %s but with different SourceVolumeId already exist", req.GetName()))
	} else {
		snapshotResponse, _ := cs.describeDiskSnapshot(ctx, req.GetName())
		if snapshotResponse != nil && len(snapshotResponse.Snapshots.Snapshot) != 0 {
			readyToUse := false
			snapshotInfo := snapshotResponse.Snapshots.Snapshot[0]
			if snapshotInfo.Status == "accomplished" {
				readyToUse = true
			} else if snapshotInfo.Status != "progressing" {
				return nil, fmt.Errorf("Create Snapshot error, with status: %s ", snapshotInfo.Status)
			}

			timeTime, _ := time.Parse(time.RFC3339, snapshotInfo.CreationTime)
			s := int64(timeTime.Second())
			n := int32(timeTime.Nanosecond())
			timestam := &timestamp.Timestamp{Seconds: s, Nanos: n}
			snapshot := diskSnapshot{}
			snapshot.CreationTime = *timestam
			snapshot.Name = req.GetName()
			snapshot.Id = snapshotInfo.SnapshotId
			snapshot.VolumeID = req.SourceVolumeId
			snapshot.SizeBytes = capacity
			snapshot.ReadyToUse = readyToUse
			diskVolumeSnapshots[snapshot.Id] = &snapshot

			log.Infof("Snapshot already created and need cache, snapshotName: %s, snapshotInfo: %v", req.GetName(), snapshot)
			return &csi.CreateSnapshotResponse{
				Snapshot: &csi.Snapshot{
					SnapshotId:     snapshotInfo.SnapshotId,
					SourceVolumeId: snapshotInfo.SourceDiskId,
					CreationTime:   timestam,
					SizeBytes:      capacity,
					ReadyToUse:     readyToUse,
				},
			}, nil
		}
	}

	// Setp 5: Create Snapshot
	// Create Snapshot with disk api
	createAt := ptypes.TimestampNow()
	createSanpshotArgs := ecs.CreateCreateSnapshotRequest()
	createSanpshotArgs.DiskId = req.SourceVolumeId
	createSanpshotArgs.SnapshotName = req.GetName()
	createSanpshotArgs.RegionId = cs.RegionId
	snapshotResp, err := cs.EcsClient.CreateSnapshot(createSanpshotArgs)
	if err != nil {
		log.Errorf("Snapshot: create snapshot error: %s", err.Error())
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed create snapshot: %v", err))
	}
	log.Infof("Create Snapshot with response: %v", snapshotResp)
	snapshotID := snapshotResp.SnapshotId

	// Step 6: response snapshot
	snapshot := diskSnapshot{}
	snapshot.Name = req.GetName()
	snapshot.Id = snapshotID
	snapshot.VolumeID = req.SourceVolumeId
	snapshot.CreationTime = *createAt
	snapshot.SizeBytes = capacity
	snapshot.ReadyToUse = false
	diskVolumeSnapshots[snapshot.Id] = &snapshot

	log.Infof("Successful Create Disk Snapshot, snapshotName: %s, VolumeId: %s, SnapShotId: %s, SizeBytes: %d, ReadyToUse: %v, CreateTime: %s", snapshot.Name, snapshot.VolumeID, snapshot.Id, snapshot.SizeBytes, snapshot.ReadyToUse, snapshot.CreationTime.String())
	return &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SnapshotId:     snapshotID,
			SourceVolumeId: snapshot.VolumeID,
			CreationTime:   &snapshot.CreationTime,
			SizeBytes:      snapshot.SizeBytes,
			ReadyToUse:     snapshot.ReadyToUse,
		},
	}, nil
}

func (cs *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	log.Infof("Starting delete snapshot %s", req.SnapshotId)
	// Step 1: Check arguments
	if len(req.GetSnapshotId()) == 0 {
		log.Errorf("Snapshot: Delete snapshot error with snapshot ID empty")
		return nil, status.Error(codes.InvalidArgument, "Snapshot ID missing in request")
	}
	if err := cs.Driver.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT); err != nil {
		return nil, err
	}
	snapshotID := req.GetSnapshotId()

	// Step 2: Delete snapshot
	cs.initEcsClient()
	if cs.EcsClient == nil {
		log.Errorf("Snapshot: init ecs client error while delete snapshot")
		return nil, fmt.Errorf("init ecs client error while delete snapshot")
	}
	deleteSnapshot := ecs.CreateDeleteSnapshotRequest()
	deleteSnapshot.SnapshotId = snapshotID
	resp, err := cs.EcsClient.DeleteSnapshot(deleteSnapshot)
	if err != nil {
		log.Errorf("Snapshot: Delete Snapshot error with: %s", err.Error())
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed create snapshot: %v", err.Error()))
	}

	log.Infof("Delete snapshot Successful %s, with response: %v", req.SnapshotId, resp)
	return &csi.DeleteSnapshotResponse{}, nil
}

func (cs *controllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	log.Infof("ListSnapshots: starting to list snapshot, %s, %s", req.SnapshotId, req.SourceVolumeId)
	if err := cs.Driver.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS); err != nil {
		return nil, err
	}

	// case 1: SnapshotId is not empty, return snapshots that match the snapshot id.
	if len(req.GetSnapshotId()) != 0 {
		snapshotID := req.SnapshotId
		if snapshot, ok := diskVolumeSnapshots[snapshotID]; ok {
			return convertSnapshot(*snapshot), nil
		}
	}

	// case 2: SourceVolumeId is not empty, return snapshots that match the source volume id.
	if len(req.GetSourceVolumeId()) != 0 {
		for _, snapshot := range diskVolumeSnapshots {
			if snapshot.VolumeID == req.SourceVolumeId {
				return convertSnapshot(*snapshot), nil
			}
		}
	}

	return &csi.ListSnapshotsResponse{
		Entries:   nil,
		NextToken: "",
	}, nil
}

func (cs *controllerServer) describeDiskSnapshot(ctx context.Context, snapshotName string) (*ecs.DescribeSnapshotsResponse, error) {
	snapshotDescribeArgs := ecs.CreateDescribeSnapshotsRequest()
	snapshotDescribeArgs.SnapshotName = snapshotName
	snapshotDescribeArgs.RegionId = cs.RegionId

	snapshots, err := cs.EcsClient.DescribeSnapshots(snapshotDescribeArgs)
	if err != nil {
		log.Errorf("Snapshot: describe snapshot error: %s", err.Error())
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed describe snapshot: %v", err))
	}
	return snapshots, nil
}

func (cs *controllerServer) getSnapshotByName(name string) (*diskSnapshot, error) {
	for _, snapshot := range diskVolumeSnapshots {
		if snapshot.Name == name {
			return snapshot, nil
		}
	}
	return nil, fmt.Errorf("snapshot name %s does not exit in the snapshots list", name)
}

func convertSnapshot(snap diskSnapshot) *csi.ListSnapshotsResponse {
	entries := []*csi.ListSnapshotsResponse_Entry{
		{
			Snapshot: &csi.Snapshot{
				SnapshotId:     snap.Id,
				SourceVolumeId: snap.VolumeID,
				CreationTime:   &snap.CreationTime,
				SizeBytes:      snap.SizeBytes,
				ReadyToUse:     snap.ReadyToUse,
			},
		},
	}
	rsp := &csi.ListSnapshotsResponse{
		Entries: entries,
	}
	return rsp
}

func (cs *controllerServer) initEcsClient() {
	accessKeyID, accessSecret, accessToken := GetDefaultAK()
	cs.EcsClient = newEcsClient(accessKeyID, accessSecret, accessToken)
}

func (cs *controllerServer) doWithAutoSnapshotPolicy(ctx context.Context) {

}
