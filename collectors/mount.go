package collectors

import (
	"time"

	linuxproc "github.com/c9s/goprocinfo/linux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/treydock/gpfs_exporter/config"
)

var (
	procMounts      = "/proc/mounts"
)

type MountCollector struct {
    target          config.Target
    fs_mount_status *prometheus.Desc
}

func NewMountCollector(target config.Target) *MountCollector {
    return &MountCollector{
        target: target,
        fs_mount_status: prometheus.NewDesc("gpfs_fs_mount_status", "Status of GPFS filesystems, 1=mounted 0=not mounted", []string{"mount"}, nil),
    }
}

func (c *MountCollector) Describe(ch chan<- *prometheus.Desc) {
    ch <- c.fs_mount_status
}

func (c *MountCollector) Collect(ch chan<- prometheus.Metric) error {
	collectTime := time.Now()
	gpfsMounts, err := getGPFSMounts()
	if err != nil {
        ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "mmpmon")
		return nil
	}
	check_mounts := c.target.FSMounts
	if check_mounts == nil {
		check_mounts = gpfsMounts
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

