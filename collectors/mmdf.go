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
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	configFilesystems = kingpin.Flag("collector.mmdf.filesystems", "Filesystems to query with mmdf, comma separated. Defaults to all filesystems.").Default("").String()
	mmdfTimeout       = kingpin.Flag("collector.mmdf.timeout", "Timeout for mmdf execution").Default("60").Int()
	mappedSections    = []string{"inode", "fsTotal", "metadata", "poolTotal"}
	MmdfExec          = mmdf
)

type DFMetric struct {
	FS              string
	InodesUsed      float64
	InodesFree      float64
	InodesAllocated float64
	InodesTotal     float64
	FSTotal         float64
	FSFree          float64
	Metadata        bool
	MetadataTotal   float64
	MetadataFree    float64
	Pools           []DfPoolMetric
}

type DfPoolMetric struct {
	PoolName          string
	PoolTotal         float64
	PoolFree          float64
	PoolFreeFragments float64
	PoolMaxDiskSize   float64
}

type MmdfCollector struct {
	InodesUsed        *prometheus.Desc
	InodesFree        *prometheus.Desc
	InodesAllocated   *prometheus.Desc
	InodesTotal       *prometheus.Desc
	FSTotal           *prometheus.Desc
	FSFree            *prometheus.Desc
	MetadataTotal     *prometheus.Desc
	MetadataFree      *prometheus.Desc
	PoolTotal         *prometheus.Desc
	PoolFree          *prometheus.Desc
	PoolFreeFragments *prometheus.Desc
	PoolMaxDiskSize   *prometheus.Desc
	logger            *slog.Logger
}

func init() {
	registerCollector("mmdf", false, NewMmdfCollector)
}

func NewMmdfCollector(logger *slog.Logger) Collector {
	return &MmdfCollector{
		InodesUsed: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "used_inodes"),
			"GPFS filesystem inodes used", []string{"fs"}, nil),
		InodesFree: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "free_inodes"),
			"GPFS filesystem inodes free", []string{"fs"}, nil),
		InodesAllocated: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "allocated_inodes"),
			"GPFS filesystem inodes allocated", []string{"fs"}, nil),
		InodesTotal: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "inodes"),
			"GPFS filesystem inodes total", []string{"fs"}, nil),
		FSTotal: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "size_bytes"),
			"GPFS filesystem total size in bytes", []string{"fs"}, nil),
		FSFree: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "free_bytes"),
			"GPFS filesystem free size in bytes", []string{"fs"}, nil),
		MetadataTotal: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "metadata_size_bytes"),
			"GPFS total metadata size in bytes", []string{"fs"}, nil),
		MetadataFree: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "metadata_free_bytes"),
			"GPFS metadata free size in bytes", []string{"fs"}, nil),
		PoolTotal: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "pool_total_bytes"),
			"GPFS pool total size in bytes", []string{"fs", "pool"}, nil),
		PoolFree: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "pool_free_bytes"),
			"GPFS pool free size in bytes", []string{"fs", "pool"}, nil),
		PoolFreeFragments: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "pool_free_fragments_bytes"),
			"GPFS pool free fragments in bytes", []string{"fs", "pool"}, nil),
		PoolMaxDiskSize: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "pool_max_disk_size_bytes"),
			"GPFS pool max disk size in bytes", []string{"fs", "pool"}, nil),
		logger: logger,
	}
}

func (c *MmdfCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.InodesUsed
	ch <- c.InodesFree
	ch <- c.InodesAllocated
	ch <- c.InodesTotal
	ch <- c.FSTotal
	ch <- c.FSFree
	ch <- c.MetadataTotal
	ch <- c.MetadataFree
	ch <- c.PoolTotal
	ch <- c.PoolFree
}

func (c *MmdfCollector) Collect(ch chan<- prometheus.Metric) {
	wg := &sync.WaitGroup{}
	var filesystems []string
	if *configFilesystems == "" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*mmlsfsTimeout)*time.Second)
		defer cancel()
		var mmlsfsTimeout float64
		var mmlsfsError float64
		mmlfsfs_filesystems, err := mmlfsfsFilesystems(ctx, c.logger)
		if err == context.DeadlineExceeded {
			mmlsfsTimeout = 1
			c.logger.Error("Timeout executing mmlsfs")
		} else if err != nil {
			mmlsfsError = 1
			c.logger.Error("Cannot collect", slog.Any("err", err))
		}
		ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, mmlsfsTimeout, "mmdf-mmlsfs")
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, mmlsfsError, "mmdf-mmlsfs")
		filesystems = mmlfsfs_filesystems
	} else {
		filesystems = strings.Split(*configFilesystems, ",")
	}
	for _, fs := range filesystems {
		c.logger.Debug("Collecting mmdf metrics", "fs", fs)
		wg.Add(1)
		collectTime := time.Now()
		go func(fs string) {
			defer wg.Done()
			label := fmt.Sprintf("mmdf-%s", fs)
			timeout := 0
			errorMetric := 0
			metric, err := c.mmdfCollect(fs)
			if err == context.DeadlineExceeded {
				c.logger.Error(fmt.Sprintf("Timeout executing %s", label))
				timeout = 1
			} else if err != nil {
				c.logger.Error("Cannot collect", slog.Any("err", err), "fs", fs)
				errorMetric = 1
			}
			ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, float64(errorMetric), label)
			ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, float64(timeout), label)
			ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), label)
			if err == nil {
				ch <- prometheus.MustNewConstMetric(c.InodesUsed, prometheus.GaugeValue, metric.InodesUsed, fs)
				ch <- prometheus.MustNewConstMetric(c.InodesFree, prometheus.GaugeValue, metric.InodesFree, fs)
				ch <- prometheus.MustNewConstMetric(c.InodesAllocated, prometheus.GaugeValue, metric.InodesAllocated, fs)
				ch <- prometheus.MustNewConstMetric(c.InodesTotal, prometheus.GaugeValue, metric.InodesTotal, fs)
				ch <- prometheus.MustNewConstMetric(c.FSTotal, prometheus.GaugeValue, metric.FSTotal, fs)
				ch <- prometheus.MustNewConstMetric(c.FSFree, prometheus.GaugeValue, metric.FSFree, fs)
				if metric.Metadata {
					ch <- prometheus.MustNewConstMetric(c.MetadataTotal, prometheus.GaugeValue, metric.MetadataTotal, fs)
					ch <- prometheus.MustNewConstMetric(c.MetadataFree, prometheus.GaugeValue, metric.MetadataFree, fs)
				}
				for _, pool := range metric.Pools {
					ch <- prometheus.MustNewConstMetric(c.PoolTotal, prometheus.GaugeValue, pool.PoolTotal, fs, pool.PoolName)
					ch <- prometheus.MustNewConstMetric(c.PoolFree, prometheus.GaugeValue, pool.PoolFree, fs, pool.PoolName)
					ch <- prometheus.MustNewConstMetric(c.PoolFreeFragments, prometheus.GaugeValue, pool.PoolFreeFragments, fs, pool.PoolName)
					ch <- prometheus.MustNewConstMetric(c.PoolMaxDiskSize, prometheus.GaugeValue, pool.PoolMaxDiskSize, fs, pool.PoolName)
				}
			}
			ch <- prometheus.MustNewConstMetric(lastExecution, prometheus.GaugeValue, float64(time.Now().Unix()), label)
		}(fs)
	}
	wg.Wait()
}

func (c *MmdfCollector) mmdfCollect(fs string) (DFMetric, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*mmdfTimeout)*time.Second)
	defer cancel()
	out, err := MmdfExec(fs, ctx)
	if err != nil {
		return DFMetric{}, err
	}
	dfMetric := parse_mmdf(out, c.logger)
	return dfMetric, nil
}

func mmdf(fs string, ctx context.Context) (string, error) {
	cmd := execCommand(ctx, *sudoCmd, "/usr/lpp/mmfs/bin/mmdf", fs, "-Y")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", ctx.Err()
	} else if err != nil {
		return "", err
	}
	return out.String(), nil
}

func parse_mmdf(out string, logger *slog.Logger) DFMetric {
	dfMetrics := DFMetric{Metadata: false}
	pools := []DfPoolMetric{}
	headers := make(map[string][]string)
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		if !strings.HasPrefix(l, "mmdf") {
			continue
		}
		items := strings.Split(l, ":")
		if len(items) < 3 {
			continue
		}
		section := items[1]
		if !SliceContains(mappedSections, section) {
			continue
		}
		if items[2] == "HEADER" {
			headers[items[1]] = append(headers[items[1]], items...)
			continue
		}
		if section == "inode" {
			if inodesUsedIndex := SliceIndex(headers["inode"], "usedInodes"); inodesUsedIndex != -1 {
				if inodesUsed, err := ParseFloat(items[inodesUsedIndex], false, logger); err == nil {
					dfMetrics.InodesUsed = inodesUsed
				}
			}
			if inodesFreeIndex := SliceIndex(headers["inode"], "freeInodes"); inodesFreeIndex != -1 {
				if inodesFree, err := ParseFloat(items[inodesFreeIndex], false, logger); err == nil {
					dfMetrics.InodesFree = inodesFree
				}
			}
			if inodesAllocatedIndex := SliceIndex(headers["inode"], "allocatedInodes"); inodesAllocatedIndex != -1 {
				if inodesAllocated, err := ParseFloat(items[inodesAllocatedIndex], false, logger); err == nil {
					dfMetrics.InodesAllocated = inodesAllocated
				}
			}
			if inodesTotalIndex := SliceIndex(headers["inode"], "maxInodes"); inodesTotalIndex != -1 {
				if inodesTotal, err := ParseFloat(items[inodesTotalIndex], false, logger); err == nil {
					dfMetrics.InodesTotal = inodesTotal
				}
			}
		}
		if section == "fsTotal" {
			if fsTotalIndex := SliceIndex(headers["fsTotal"], "fsSize"); fsTotalIndex != -1 {
				if fsTotal, err := ParseFloat(items[fsTotalIndex], true, logger); err == nil {
					dfMetrics.FSTotal = fsTotal
				}
			}
			if fsFreeIndex := SliceIndex(headers["fsTotal"], "freeBlocks"); fsFreeIndex != -1 {
				if fsFree, err := ParseFloat(items[fsFreeIndex], true, logger); err == nil {
					dfMetrics.FSFree = fsFree
				}
			}
		}
		if section == "metadata" {
			dfMetrics.Metadata = true
			if metadataTotalIndex := SliceIndex(headers["metadata"], "totalMetadata"); metadataTotalIndex != -1 {
				if metadataTotal, err := ParseFloat(items[metadataTotalIndex], true, logger); err == nil {
					dfMetrics.MetadataTotal = metadataTotal
				}
			}
			if metadataFreeIndex := SliceIndex(headers["metadata"], "freeBlocks"); metadataFreeIndex != -1 {
				if metadataFree, err := ParseFloat(items[metadataFreeIndex], true, logger); err == nil {
					dfMetrics.MetadataFree = metadataFree
				}
			}
		}
		if section == "poolTotal" {
			poolMetric := DfPoolMetric{}
			if poolNameIndex := SliceIndex(headers["poolTotal"], "poolName"); poolNameIndex != -1 {
				poolMetric.PoolName = items[poolNameIndex]
			}
			if poolTotalIndex := SliceIndex(headers["poolTotal"], "poolSize"); poolTotalIndex != -1 {
				if poolTotal, err := ParseFloat(items[poolTotalIndex], true, logger); err == nil {
					poolMetric.PoolTotal = poolTotal
				}
			}
			if poolFreeIndex := SliceIndex(headers["poolTotal"], "freeBlocks"); poolFreeIndex != -1 {
				if poolFree, err := ParseFloat(items[poolFreeIndex], true, logger); err == nil {
					poolMetric.PoolFree = poolFree
				}
			}
			if poolFreeFragmentsIndex := SliceIndex(headers["poolTotal"], "freeFragments"); poolFreeFragmentsIndex != -1 {
				if poolFreeFragments, err := ParseFloat(items[poolFreeFragmentsIndex], true, logger); err == nil {
					poolMetric.PoolFreeFragments = poolFreeFragments
				}
			}
			if poolMaxDiskSizeIndex := SliceIndex(headers["poolTotal"], "maxDiskSize"); poolMaxDiskSizeIndex != -1 {
				if poolMaxDiskSize, err := ParseFloat(items[poolMaxDiskSizeIndex], true, logger); err == nil {
					poolMetric.PoolMaxDiskSize = poolMaxDiskSize
				}
			}
			pools = append(pools, poolMetric)
		}
	}
	dfMetrics.Pools = pools
	return dfMetrics
}
