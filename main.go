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

package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AliyunContainerService/disk-snapshot/pkg/controller"
	log "github.com/Sirupsen/logrus"
)

func init() {
	flag.Set("logtostderr", "true")
}

const (
	LOGFILE_PREFIX   = "/var/log/alicloud/"
	MB_SIZE          = 1024 * 1024
	TYPE_PLUGIN_DISK = "disk-snapshot"
)

var (
	endpoint = flag.String("endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	nodeId   = flag.String("nodeid", "", "node id")
)

func init() {
	setLogAttribute()
}

// Nas CSI Plugin
func main() {
	flag.Parse()
	drivername := filepath.Base(os.Args[0])
	log.Infof("Disk Snapshot Driver Starting: ", drivername, *nodeId, *endpoint)
	if drivername == TYPE_PLUGIN_DISK {
		driver := controller.NewDriver(*nodeId, *endpoint)
		driver.Run()
	}

	os.Exit(0)
}

// rotate log file by 2M bytes
func setLogAttribute() {
	logType := os.Getenv("LOG_TYPE")
	if strings.ToLower(logType) == "" || strings.ToLower(logType) == "stdout" {
		return
	}

	driver := filepath.Base(os.Args[0])
	os.MkdirAll(LOGFILE_PREFIX, os.FileMode(0755))

	logFile := LOGFILE_PREFIX + driver + ".log"
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		os.Exit(1)
	}

	// rotate the log file if too large
	if fi, err := f.Stat(); err == nil && fi.Size() > 2*MB_SIZE {
		f.Close()
		timeStr := time.Now().Format("-2006-01-02-15:04:05")
		timedLogfile := LOGFILE_PREFIX + driver + timeStr + ".log"
		os.Rename(logFile, timedLogfile)
		f, err = os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			os.Exit(1)
		}
	}
	log.SetOutput(f)
}
