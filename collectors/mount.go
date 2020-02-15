// Copyright 2020 Trey Dockendorf
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collectors

import (
	"fmt"
	"strings"
	"time"

	linuxproc "github.com/c9s/goprocinfo/linux"
	fstab "github.com/deniswernert/go-fstab"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	procMounts   = "/proc/mounts"
	fstabPath    = "/etc/fstab"
	configMounts = kingpin.Flag("collector.mount.mounts", "Mountpoints to monitor, comma separated. Defaults to all filesystems.").Default("").String()
)

type MountCollector struct {
	fs_mount_status *prometheus.Desc
}

func init() {
	registerCollector("mount", true, NewMountCollector)
}

func NewMountCollector() Collector {
	return &MountCollector{
		fs_mount_status: prometheus.NewDesc(prometheus.BuildFQName(namespace, "mount", "status"),
			"Status of GPFS filesystems, 1=mounted 0=not mounted", []string{"mount"}, nil),
	}
}

func (c *MountCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.fs_mount_status
}

func (c *MountCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debug("Collecting mount metrics")
	err := c.collect(ch)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "mount")
	} else {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 0, "mount")
	}
}

func (c *MountCollector) collect(ch chan<- prometheus.Metric) error {
	collectTime := time.Now()
	gpfsMounts, err := getGPFSMounts()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "mount")
		return nil
	}
	gpfsMountsFstab, err := getGPFSMountsFSTab()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "mount")
		return nil
	}
	for _, m := range gpfsMountsFstab {
		if !SliceContains(gpfsMounts, m) {
			gpfsMounts = append(gpfsMounts, m)
		}
	}
	var check_mounts []string
	if *configMounts == "" {
		check_mounts = gpfsMounts
	} else {
		check_mounts = strings.Split(*configMounts, ",")
	}
	for _, mount := range check_mounts {
		if SliceContains(gpfsMounts, mount) {
			ch <- prometheus.MustNewConstMetric(c.fs_mount_status, prometheus.GaugeValue, 1, mount)
		} else {
			ch <- prometheus.MustNewConstMetric(c.fs_mount_status, prometheus.GaugeValue, 0, mount)
		}
	}
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "mount")
	return nil
}

func getGPFSMounts() ([]string, error) {
	var gpfsMounts []string
	mounts, err := linuxproc.ReadMounts(procMounts)
	if err != nil {
		return nil, err
	}
	for _, mount := range mounts.Mounts {
		if mount.FSType != "gpfs" {
			continue
		}
		gpfsMount := mount.MountPoint
		gpfsMounts = append(gpfsMounts, gpfsMount)
	}
	return gpfsMounts, err
}

func getGPFSMountsFSTab() ([]string, error) {
	var gpfsMounts []string
	if exists := fileExists(fstabPath); !exists {
		return nil, fmt.Errorf("%s does not exist", fstabPath)
	}
	mounts, err := fstab.ParseFile(fstabPath)
	if err != nil {
		return nil, err
	}
	for _, m := range mounts {
		if m.VfsType != "gpfs" {
			continue
		}
		gpfsMounts = append(gpfsMounts, m.File)
	}
	return gpfsMounts, nil
}
