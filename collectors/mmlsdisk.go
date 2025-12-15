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
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	diskFilesystems = kingpin.Flag("collector.mmlsdisk.filesystems", "Filesystems to query with mmlsdisk, comma separated. Defaults to all filesystems.").Default("").String()
	diskTimeout     = kingpin.Flag("collector.mmlsdisk.timeout", "Timeout for mmlsdisk execution").Default("30").Int()
	diskMap         = map[string]string{
		"nsdName":      "Name",
		"metadata":     "Metadata",
		"data":         "Data",
		"status":       "Status",
		"availability": "Availability",
		"diskID":       "DiskID",
		"storagePool":  "StoragePool",
	}
	diskStatuses     = []string{"ready", "suspended", "to be emptied", "being emptied", "emptied", "replacing", "replacement"}
	diskAvailability = []string{"up", "down", "recovering", "unrecovered"}
	MmlsdiskExec     = mmlsdisk
)

type DiskMetric struct {
	Name         string
	FS           string
	Metadata     string
	Data         string
	Status       string
	Availability string
	DiskID       string
	StoragePool  string
}

type MmlsdiskCollector struct {
	Status       *prometheus.Desc
	Availability *prometheus.Desc
	logger       *slog.Logger
}

func init() {
	registerCollector("mmlsdisk", false, NewMmlsdiskCollector)
}

func NewMmlsdiskCollector(logger *slog.Logger) Collector {
	return &MmlsdiskCollector{
		Status: prometheus.NewDesc(prometheus.BuildFQName(namespace, "disk", "status"),
			"GPFS disk status", []string{"name", "fs", "metadata", "data", "diskid", "storagepool", "status"}, nil),
		Availability: prometheus.NewDesc(prometheus.BuildFQName(namespace, "disk", "availability"),
			"GPFS disk availability", []string{"name", "fs", "metadata", "data", "diskid", "storagepool", "availability"}, nil),
		logger: logger,
	}
}

func (c *MmlsdiskCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Status
	ch <- c.Availability
}

func (c *MmlsdiskCollector) Collect(ch chan<- prometheus.Metric) {
	wg := &sync.WaitGroup{}
	var filesystems []string
	if *diskFilesystems == "" {
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
		ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, mmlsfsTimeout, "mmlsdisk-mmlsfs")
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, mmlsfsError, "mmlsdisk-mmlsfs")
		filesystems = mmlfsfs_filesystems
	} else {
		filesystems = strings.Split(*diskFilesystems, ",")
	}
	for _, fs := range filesystems {
		c.logger.Debug("Collecting mmlsdisk metrics", "fs", fs)
		wg.Add(1)
		collectTime := time.Now()
		go func(fs string) {
			defer wg.Done()
			label := fmt.Sprintf("mmlsdisk-%s", fs)
			timeout := 0
			errorMetric := 0
			metrics, err := c.mmlsdiskCollect(fs)
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
			if err != nil {
				return
			}
			for _, m := range metrics {
				for _, status := range diskStatuses {
					var value float64
					if m.Status == status {
						value = 1
					}
					ch <- prometheus.MustNewConstMetric(c.Status, prometheus.GaugeValue, value, m.Name, fs, m.Metadata, m.Data, m.DiskID, m.StoragePool, status)
				}
				var unknown float64
				if !SliceContains(diskStatuses, m.Status) {
					unknown = 1
				}
				ch <- prometheus.MustNewConstMetric(c.Status, prometheus.GaugeValue, unknown, m.Name, fs, m.Metadata, m.Data, m.DiskID, m.StoragePool, "unknown")
				for _, avail := range diskAvailability {
					var value float64
					if m.Availability == avail {
						value = 1
					}
					ch <- prometheus.MustNewConstMetric(c.Availability, prometheus.GaugeValue, value, m.Name, fs, m.Metadata, m.Data, m.DiskID, m.StoragePool, avail)
				}
				var availUnknown float64
				if !SliceContains(diskAvailability, m.Availability) {
					availUnknown = 1
				}
				ch <- prometheus.MustNewConstMetric(c.Availability, prometheus.GaugeValue, availUnknown, m.Name, fs, m.Metadata, m.Data, m.DiskID, m.StoragePool, "unknown")
			}
		}(fs)
	}
	wg.Wait()
}

func (c *MmlsdiskCollector) mmlsdiskCollect(fs string) ([]DiskMetric, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*diskTimeout)*time.Second)
	defer cancel()
	out, err := MmlsdiskExec(fs, ctx)
	if err != nil {
		return nil, err
	}
	metrics, err := parse_mmlsdisk(out, c.logger)
	return metrics, err
}

func mmlsdisk(fs string, ctx context.Context) (string, error) {
	args := []string{"/usr/lpp/mmfs/bin/mmlsdisk", fs, "-Y"}
	cmd := execCommand(ctx, *sudoCmd, args...)
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

func parse_mmlsdisk(out string, logger *slog.Logger) ([]DiskMetric, error) {
	var metrics []DiskMetric
	headers := []string{}
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		if !strings.HasPrefix(l, "mmlsdisk") {
			continue
		}
		items := strings.Split(l, ":")
		if len(items) < 3 {
			continue
		}
		var values []string
		if items[2] == "HEADER" {
			headers = append(headers, items...)
			continue
		} else {
			values = append(values, items...)
		}
		var metric DiskMetric
		ps := reflect.ValueOf(&metric) // pointer to struct - addressable
		s := ps.Elem()                 // struct
		for i, h := range headers {
			if field, ok := diskMap[h]; ok {
				f := s.FieldByName(field)
				if f.Kind() == reflect.String {
					f.SetString(values[i])
				}
			}
		}

		metrics = append(metrics, metric)
	}
	return metrics, nil
}
