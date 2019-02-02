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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
)

const (
	KUBERNETES_ALICLOUD_DISK_DRIVER = "alicloud/disk"
	METADATA_URL                    = "http://100.100.100.200/latest/meta-data/"
	REGIONID_TAG                    = "region-id"
	LOGFILE_PREFIX                  = "/var/log/alicloud/provisioner"
	DISK_NOTAVAILABLE               = "InvalidDataDiskCategory.NotSupported"
	DISK_HIGH_AVAIL                 = "available"
	DISK_COMMON                     = "cloud"
	DISK_EFFICIENCY                 = "cloud_efficiency"
	DISK_SSD                        = "cloud_ssd"
	MB_SIZE                         = 1024 * 1024
	DEFAULT_REGION                  = "cn-hangzhou"
	INSTANCEID_TAG                  = "instance-id"
)

var (
	// VERSION should be updated by hand at each release
	VERSION = "v1.10.4"

	// GITCOMMIT will be overwritten automatically by the build system
	GITCOMMIT                    = "HEAD"
	KUBERNETES_ALICLOUD_IDENTITY = fmt.Sprintf("Kubernetes.Alicloud/Snapshot.Disk-%s", SnapshotVersion())
)

func SnapshotVersion() string {
	return VERSION
}

//var attachdetachMutex = keymutex.NewKeyMutex()

// struct for access key
type DefaultOptions struct {
	Global struct {
		KubernetesClusterTag string
		AccessKeyID          string `json:"accessKeyID"`
		AccessKeySecret      string `json:"accessKeySecret"`
		Region               string `json:"region"`
	}
}

func newEcsClient(access_key_id, access_key_secret, access_token string) *ecs.Client {
	ecsClient := &ecs.Client{}
	var err error = nil
	if access_token == "" {
		ecsClient, err = ecs.NewClientWithAccessKey(DEFAULT_REGION, access_key_id, access_key_secret)
		if err != nil {
			return nil
		}
	}
	if access_token != "" {
		ecsClient, err = ecs.NewClientWithStsToken(DEFAULT_REGION, access_key_id, access_key_secret, access_token)
		if err != nil {
			return nil
		}
	}
	return ecsClient
}

// read default ak from local file or from STS
func GetDefaultAK() (string, string, string) {
	accessKeyID, accessSecret := GetLocalAK()
	accessToken := ""
	if accessKeyID == "" || accessSecret == "" {
		accessKeyID, accessSecret, accessToken = GetSTSAK()
	}
	return accessKeyID, accessSecret, accessToken
}

// get host regionid, zoneid
func GetMetaData(resource string) string {
	resp, err := http.Get(METADATA_URL + resource)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(body)
}

// get STS AK
func GetSTSAK() (string, string, string) {
	createAssumeRoleReq := sts.CreateAssumeRoleRequest()
	client, err := sts.NewClient()
	if err != nil {
		log.Infof("get sts token error with: %s", err.Error())
		return "", "", ""
	}
	response, err := client.AssumeRole(createAssumeRoleReq)
	if err != nil {
		log.Infof("AssumeRole: Get sts token error with: %s", err.Error())
		return "", "", ""
	}

	role := response.Credentials
	return role.AccessKeyId, role.AccessKeySecret, role.SecurityToken
}

func GetLocalAK() (string, string) {
	var accessKeyID, accessSecret string

	// first check if the environment setting
	accessKeyID = os.Getenv("ACCESS_KEY_ID")
	accessSecret = os.Getenv("ACCESS_KEY_SECRET")
	if accessKeyID != "" && accessSecret != "" {
		return accessKeyID, accessSecret
	}

	return accessKeyID, accessSecret
}

// check file exist in volume driver;
func IsFileExisting(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}

func run(cmd string) (string, error) {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Failed to run cmd: %s, with out: %s, error: %s ", cmd, out, err.Error())
	}
	return string(out), nil
}

func execCommand(command string, args []string) ([]byte, error) {
	cmd := exec.Command(command, args...)
	return cmd.CombinedOutput()
}

// run shell command
func Run(cmd string) (string, error) {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Failed to run cmd: " + cmd + ", with out: " + string(out) + ", with error: " + err.Error())
	}
	return string(out), nil
}

func CreateDest(dest string) error {
	fi, err := os.Lstat(dest)

	if os.IsNotExist(err) {
		if err := os.MkdirAll(dest, 0777); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if fi != nil && !fi.IsDir() {
		return fmt.Errorf("%v already exist but it's not a directory", dest)
	}
	return nil
}

func IsMounted(mountPath string) bool {
	cmd := fmt.Sprintf("mount | grep \"%s type\" | grep -v grep", mountPath)
	out, err := Run(cmd)
	if err != nil || out == "" {
		return false
	}
	return true
}

func Umount(mountPath string) bool {
	cmd := fmt.Sprintf("umount -f %s", mountPath)
	_, err := Run(cmd)
	if err != nil {
		return false
	}
	return true
}

// Get regionid instanceid;
func GetRegionAndInstanceId() (string, string, error) {
	regionId := GetMetaData(REGIONID_TAG)
	instanceId := GetMetaData(INSTANCEID_TAG)
	return regionId, instanceId, nil
}

func GetRegionIdAndInstanceId(nodeName string) (string, string, error) {
	strs := strings.SplitN(nodeName, ".", 2)
	if len(strs) < 2 {
		return "", "", fmt.Errorf("failed to get regionID and instanceId from nodeName")
	}
	return strs[0], strs[1], nil
}

// save json data to file
func WriteJosnFile(obj interface{}, file string) error {
	maps := make(map[string]interface{})
	t := reflect.TypeOf(obj)
	v := reflect.ValueOf(obj)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).String() != "" {
			maps[t.Field(i).Name] = v.Field(i).String()
		}
	}
	rankingsJson, _ := json.Marshal(maps)
	if err := ioutil.WriteFile(file, rankingsJson, 0644); err != nil {
		return err
	}
	return nil
}

// parse json to struct
func ReadJsonFile(file string) (map[string]string, error) {
	jsonObj := map[string]string{}
	raw, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(raw, &jsonObj)
	if err != nil {
		return nil, err
	}
	return jsonObj, nil
}
