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
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	procMounts   = "/proc/mounts"
	fstabPath    = "/etc/fstab"
	configMounts = kingpin.Flag("collector.mount.mounts", "Mountpoints to monitor, comma separated. Defaults to all filesystems.").Default("").String()
	mountTimeout = kingpin.Flag("collector.mount.timeout", "Timeout for mount collection").Default("5").Int()
)

type MountCollector struct {
	fs_mount_status *prometheus.Desc
	logger          log.Logger
}

func init() {
	registerCollector("mount", true, NewMountCollector)
}

func NewMountCollector(logger log.Logger) Collector {
	return &MountCollector{
		fs_mount_status: prometheus.NewDesc(prometheus.BuildFQName(namespace, "mount", "status"),
			"Status of GPFS filesystems, 1=mounted 0=not mounted", []string{"mount"}, nil),
		logger: logger,
	}
}

func (c *MountCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.fs_mount_status
}

func (c *MountCollector) Collect(ch chan<- prometheus.Metric) {
	level.Debug(c.logger).Log("msg", "Collecting mount metrics")
	err := c.collect(ch)
	if err != nil {
		level.Error(c.logger).Log("msg", err)
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "mount")
	} else {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 0, "mount")
	}
}

func (c *MountCollector) collect(ch chan<- prometheus.Metric) error {
	collectTime := time.Now()
	var gpfsMounts []string
	var gpfsMountsFstab []string
	var err error

	c1 := make(chan int, 1)
	timeout := false

	go func() {
		gpfsMounts, err = getGPFSMounts()
		if err != nil {
			return
		}
		gpfsMountsFstab, err = getGPFSMountsFSTab()
		if err != nil {
			return
		}
		if !timeout {
			c1 <- 1
		}
	}()

	select {
	case <-c1:
	case <-time.After(time.Duration(*mountTimeout) * time.Second):
		timeout = true
		close(c1)
		ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, 1, "mount")
		level.Error(c.logger).Log("msg", "Timeout collecting mount information")
		return nil
	}
	close(c1)
	ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, 0, "mount")

	if err != nil {
		level.Error(c.logger).Log("msg", err)
		return err
	}

	var gpfsFoundMounts []string
	for _, m := range gpfsMountsFstab {
		if !SliceContains(gpfsFoundMounts, m) {
			gpfsFoundMounts = append(gpfsFoundMounts, m)
		}
	}
	var checkMounts []string
	if *configMounts == "" {
		checkMounts = gpfsFoundMounts
	} else {
		checkMounts = strings.Split(*configMounts, ",")
	}
	for _, mount := range checkMounts {
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
	if exists := FileExists(fstabPath); !exists {
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
