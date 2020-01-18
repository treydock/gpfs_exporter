package collector

import (
	linuxproc "github.com/c9s/goprocinfo/linux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/treydock/gpfs_exporter/config"
)

var (
	procMounts      = "/proc/mounts"
	fs_mount_status = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "fs_mount_status"),
		"Status of GPFS filesystems, 1=mounted 0=not mounted",
		[]string{"mount"},
		nil,
	)
)

type ScrapeMount struct{}

func (ScrapeMount) Name() string {
	return "mount"
}

func (ScrapeMount) Scrape(target config.Target, ch chan<- prometheus.Metric) error {
	gpfsMounts, err := getGPFSMounts()
	if err != nil {
		return nil
	}
	check_mounts := target.FSMounts
	if check_mounts == nil {
		check_mounts = gpfsMounts
	}
	for _, mount := range check_mounts {
		if sliceContains(gpfsMounts, mount) {
			ch <- prometheus.MustNewConstMetric(fs_mount_status, prometheus.GaugeValue, 1, mount)
		} else {
			ch <- prometheus.MustNewConstMetric(fs_mount_status, prometheus.GaugeValue, 0, mount)
		}
	}
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

func sliceContains(slice []string, str string) bool {
	for _, s := range slice {
		if str == s {
			return true
		}
	}
	return false
}
