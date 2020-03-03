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
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	configFilesystems = kingpin.Flag("collector.mmdf.filesystems", "Filesystems to query with mmdf, comma separated. Defaults to all filesystems.").Default("").String()
	mmdfTimeout       = kingpin.Flag("collector.mmdf.timeout", "Timeout for mmdf execution").Default("60").Int()
	mappedSections    = []string{"inode", "fsTotal", "metadata"}
	KbToBytes         = []string{"fsSize", "freeBlocks", "totalMetadata"}
	dfMap             = map[string]string{
		"inode:usedInodes":       "InodesUsed",
		"inode:freeInodes":       "InodesFree",
		"inode:allocatedInodes":  "InodesAllocated",
		"inode:maxInodes":        "InodesTotal",
		"fsTotal:fsSize":         "FSTotal",
		"fsTotal:freeBlocks":     "FSFree",
		"fsTotal:freeBlocksPct":  "FSFreePercent",
		"metadata:totalMetadata": "MetadataTotal",
		"metadata:freeBlocks":    "MetadataFree",
		"metadata:freeBlocksPct": "MetadataFreePercent",
	}
)

type DFMetric struct {
	FS                  string
	InodesUsed          int64
	InodesFree          int64
	InodesAllocated     int64
	InodesTotal         int64
	FSTotal             int64
	FSFree              int64
	FSFreePercent       int64
	MetadataTotal       int64
	MetadataFree        int64
	MetadataFreePercent int64
}

type MmdfCollector struct {
	InodesUsed          *prometheus.Desc
	InodesFree          *prometheus.Desc
	InodesAllocated     *prometheus.Desc
	InodesTotal         *prometheus.Desc
	FSTotal             *prometheus.Desc
	FSFree              *prometheus.Desc
	FSFreePercent       *prometheus.Desc
	MetadataTotal       *prometheus.Desc
	MetadataFree        *prometheus.Desc
	MetadataFreePercent *prometheus.Desc
	logger              log.Logger
}

func init() {
	registerCollector("mmdf", false, NewMmdfCollector)
}

func NewMmdfCollector(logger log.Logger) Collector {
	return &MmdfCollector{
		InodesUsed: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "inodes_used"),
			"GPFS filesystem inodes used", []string{"fs"}, nil),
		InodesFree: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "inodes_free"),
			"GPFS filesystem inodes free", []string{"fs"}, nil),
		InodesAllocated: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "inodes_allocated"),
			"GPFS filesystem inodes allocated", []string{"fs"}, nil),
		InodesTotal: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "inodes_total"),
			"GPFS filesystem inodes total", []string{"fs"}, nil),
		FSTotal: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "total_bytes"),
			"GPFS filesystem total size in bytes", []string{"fs"}, nil),
		FSFree: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "free_bytes"),
			"GPFS filesystem free size in bytes", []string{"fs"}, nil),
		FSFreePercent: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "free_percent"),
			"GPFS filesystem free percent", []string{"fs"}, nil),
		MetadataTotal: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "metadata_total_bytes"),
			"GPFS total metadata size in bytes", []string{"fs"}, nil),
		MetadataFree: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "metadata_free_bytes"),
			"GPFS metadata free size in bytes", []string{"fs"}, nil),
		MetadataFreePercent: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fs", "metadata_free_percent"),
			"GPFS metadata free percent", []string{"fs"}, nil),
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
	ch <- c.FSFreePercent
	ch <- c.MetadataTotal
	ch <- c.MetadataFree
	ch <- c.MetadataFreePercent
}

func (c *MmdfCollector) Collect(ch chan<- prometheus.Metric) {
	wg := &sync.WaitGroup{}
	var filesystems []string
	if *configFilesystems == "" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*mmlsfsTimeout)*time.Second)
		defer cancel()
		mmlfsfs_filesystems, err := mmlfsfsFilesystems(ctx, c.logger)
		if ctx.Err() == context.DeadlineExceeded {
			ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, 1, "mmdf-mmlsfs")
			level.Error(c.logger).Log("msg", "Timeout executing mmlsfs")
		} else {
			ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, 0, "mmdf-mmlsfs")
		}
		if err != nil {
			ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "mmdf-mmlsfs")
		} else {
			ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 0, "mmdf-mmlsfs")
		}
		filesystems = mmlfsfs_filesystems
	} else {
		filesystems = strings.Split(*configFilesystems, ",")
	}
	for _, fs := range filesystems {
		level.Debug(c.logger).Log("msg", "Collecting mmdf metrics", "fs", fs)
		wg.Add(1)
		collectTime := time.Now()
		go func(fs string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*mmdfTimeout)*time.Second)
			defer cancel()
			label := fmt.Sprintf("mmdf-%s", fs)
			err := c.mmdfCollect(fs, ch, ctx)
			if ctx.Err() == context.DeadlineExceeded {
				ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, 1, label)
				level.Error(c.logger).Log("msg", fmt.Sprintf("Timeout executing %s", label))
				return
			}
			ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, 0, label)
			if err != nil {
				ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, label)
			} else {
				ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 0, label)
			}
			ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), label)
			ch <- prometheus.MustNewConstMetric(lastExecution, prometheus.GaugeValue, float64(time.Now().Unix()), label)
		}(fs)
	}
	wg.Wait()
}

func (c *MmdfCollector) mmdfCollect(fs string, ch chan<- prometheus.Metric, ctx context.Context) error {
	out, err := mmdf(fs, ctx)
	if err != nil {
		level.Error(c.logger).Log("msg", err)
		return err
	}
	dfMetric, err := parse_mmdf(out, c.logger)
	if err != nil {
		level.Error(c.logger).Log("msg", err)
		return err
	}
	ch <- prometheus.MustNewConstMetric(c.InodesUsed, prometheus.GaugeValue, float64(dfMetric.InodesUsed), fs)
	ch <- prometheus.MustNewConstMetric(c.InodesFree, prometheus.GaugeValue, float64(dfMetric.InodesFree), fs)
	ch <- prometheus.MustNewConstMetric(c.InodesAllocated, prometheus.GaugeValue, float64(dfMetric.InodesAllocated), fs)
	ch <- prometheus.MustNewConstMetric(c.InodesTotal, prometheus.GaugeValue, float64(dfMetric.InodesTotal), fs)
	ch <- prometheus.MustNewConstMetric(c.FSTotal, prometheus.GaugeValue, float64(dfMetric.FSTotal), fs)
	ch <- prometheus.MustNewConstMetric(c.FSFree, prometheus.GaugeValue, float64(dfMetric.FSFree), fs)
	ch <- prometheus.MustNewConstMetric(c.FSFreePercent, prometheus.GaugeValue, float64(dfMetric.FSFreePercent), fs)
	ch <- prometheus.MustNewConstMetric(c.MetadataTotal, prometheus.GaugeValue, float64(dfMetric.MetadataTotal), fs)
	ch <- prometheus.MustNewConstMetric(c.MetadataFree, prometheus.GaugeValue, float64(dfMetric.MetadataFree), fs)
	ch <- prometheus.MustNewConstMetric(c.MetadataFreePercent, prometheus.GaugeValue, float64(dfMetric.MetadataFreePercent), fs)
	return nil
}

func mmlfsfsFilesystems(ctx context.Context, logger log.Logger) ([]string, error) {
	var filesystems []string
	out, err := mmlsfs(ctx)
	if err != nil {
		level.Error(logger).Log("msg", err)
		return nil, err
	}
	mmlsfs_filesystems, err := parse_mmlsfs(out)
	if err != nil {
		level.Error(logger).Log("msg", err)
		return nil, err
	}
	for _, fs := range mmlsfs_filesystems {
		filesystems = append(filesystems, fs.Name)
	}
	return filesystems, nil
}

func mmdf(fs string, ctx context.Context) (string, error) {
	cmd := execCommand(ctx, "sudo", "/usr/lpp/mmfs/bin/mmdf", fs, "-Y")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

func parse_mmdf(out string, logger log.Logger) (DFMetric, error) {
	var dfMetrics DFMetric
	headers := make(map[string][]string)
	values := make(map[string][]string)
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		if !strings.HasPrefix(l, "mmdf") {
			continue
		}
		items := strings.Split(l, ":")
		if len(items) < 3 {
			continue
		}
		if !SliceContains(mappedSections, items[1]) {
			continue
		}
		if items[2] == "HEADER" {
			headers[items[1]] = append(headers[items[1]], items...)
		} else {
			values[items[1]] = append(values[items[1]], items...)
		}
	}
	ps := reflect.ValueOf(&dfMetrics) // pointer to struct - addressable
	s := ps.Elem()                    // struct
	for k, vals := range headers {
		for i, v := range vals {
			mapKey := fmt.Sprintf("%s:%s", k, v)
			value := values[k][i]
			if field, ok := dfMap[mapKey]; ok {
				f := s.FieldByName(field)
				if f.Kind() == reflect.String {
					f.SetString(value)
				} else if f.Kind() == reflect.Int64 {
					if val, err := strconv.ParseInt(value, 10, 64); err == nil {
						if SliceContains(KbToBytes, v) {
							val = val * 1024
						}
						f.SetInt(val)
					} else {
						level.Error(logger).Log("msg", fmt.Sprintf("Error parsing %s value %s: %s", mapKey, value, err.Error()))
					}
				}
			}
		}
	}
	return dfMetrics, nil
}
