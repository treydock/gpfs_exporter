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
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	snapshotFilesystems = kingpin.Flag("collector.mmlssnapshot.filesystems", "Filesystems to query with mmlssnapshot, comma separated. Defaults to all filesystems.").Default("").String()
	snapshotTimeout     = kingpin.Flag("collector.mmlssnapshot.timeout", "Timeout for mmlssnapshot execution").Default("60").Int()
	snapshotGetSize     = kingpin.Flag("collector.mmlssnapshot.get-size", "Collect snapshot sizes, long running operation").Default("false").Bool()
	SnapshotKbToBytes   = []string{"data", "metadata"}
	snapshotMap         = map[string]string{
		"filesystemName": "FS",
		"directory":      "Name",
		"snapID":         "ID",
		"status":         "Status",
		"created":        "Created",
		"fileset":        "Fileset",
		"data":           "Data",
		"metadata":       "Metadata",
	}
	MmlssnapshotExec = mmlssnapshot
)

type SnapshotMetric struct {
	FS       string
	Name     string
	ID       string
	Status   string
	Created  float64
	Fileset  string
	Data     float64
	Metadata float64
}

type MmlssnapshotCollector struct {
	Status   *prometheus.Desc
	Created  *prometheus.Desc
	Data     *prometheus.Desc
	Metadata *prometheus.Desc
	logger   *slog.Logger
}

func init() {
	registerCollector("mmlssnapshot", false, NewMmlssnapshotCollector)
}

func NewMmlssnapshotCollector(logger *slog.Logger) Collector {
	labels := []string{"fs", "fileset", "snapshot", "id"}
	return &MmlssnapshotCollector{
		Status: prometheus.NewDesc(prometheus.BuildFQName(namespace, "snapshot", "status_info"),
			"GPFS snapshot status", append(labels, []string{"status"}...), nil),
		Created: prometheus.NewDesc(prometheus.BuildFQName(namespace, "snapshot", "created_timestamp_seconds"),
			"GPFS snapshot creation timestamp", labels, nil),
		Data: prometheus.NewDesc(prometheus.BuildFQName(namespace, "snapshot", "data_size_bytes"),
			"GPFS snapshot data size", labels, nil),
		Metadata: prometheus.NewDesc(prometheus.BuildFQName(namespace, "snapshot", "metadata_size_bytes"),
			"GPFS snapshot metadata size", labels, nil),
		logger: logger,
	}
}

func (c *MmlssnapshotCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Status
	ch <- c.Created
	if *snapshotGetSize {
		ch <- c.Data
		ch <- c.Metadata
	}
}

func (c *MmlssnapshotCollector) Collect(ch chan<- prometheus.Metric) {
	wg := &sync.WaitGroup{}
	var filesystems []string
	if *snapshotFilesystems == "" {
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
			c.logger.Error("Cannot collect", err)
		}
		ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, mmlsfsTimeout, "mmlssnapshot-mmlsfs")
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, mmlsfsError, "mmlssnapshot-mmlsfs")
		filesystems = mmlfsfs_filesystems
	} else {
		filesystems = strings.Split(*snapshotFilesystems, ",")
	}
	for _, fs := range filesystems {
		c.logger.Debug("Collecting mmlssnapshot metrics", "fs", fs)
		wg.Add(1)
		collectTime := time.Now()
		go func(fs string) {
			defer wg.Done()
			label := fmt.Sprintf("mmlssnapshot-%s", fs)
			timeout := 0
			errorMetric := 0
			metrics, err := c.mmlssnapshotCollect(fs)
			if err == context.DeadlineExceeded {
				c.logger.Error(fmt.Sprintf("Timeout executing %s", label))
				timeout = 1
			} else if err != nil {
				c.logger.Error("Cannot collect", err, "fs", fs)
				errorMetric = 1
			}
			ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, float64(errorMetric), label)
			ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, float64(timeout), label)
			ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), label)
			if err != nil {
				return
			}
			for _, m := range metrics {
				ch <- prometheus.MustNewConstMetric(c.Status, prometheus.GaugeValue, 1, m.FS, m.Fileset, m.Name, m.ID, m.Status)
				ch <- prometheus.MustNewConstMetric(c.Created, prometheus.GaugeValue, m.Created, m.FS, m.Fileset, m.Name, m.ID)
				if *snapshotGetSize {
					ch <- prometheus.MustNewConstMetric(c.Data, prometheus.GaugeValue, m.Data, m.FS, m.Fileset, m.Name, m.ID)
					ch <- prometheus.MustNewConstMetric(c.Metadata, prometheus.GaugeValue, m.Metadata, m.FS, m.Fileset, m.Name, m.ID)
				}
			}
			ch <- prometheus.MustNewConstMetric(lastExecution, prometheus.GaugeValue, float64(time.Now().Unix()), label)
		}(fs)
	}
	wg.Wait()
}

func (c *MmlssnapshotCollector) mmlssnapshotCollect(fs string) ([]SnapshotMetric, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*snapshotTimeout)*time.Second)
	defer cancel()
	out, err := MmlssnapshotExec(fs, ctx)
	if err != nil {
		return nil, err
	}
	metrics, err := parse_mmlssnapshot(out, c.logger)
	return metrics, err
}

func mmlssnapshot(fs string, ctx context.Context) (string, error) {
	args := []string{"/usr/lpp/mmfs/bin/mmlssnapshot", fs, "-s", "all", "-Y"}
	if *snapshotGetSize {
		args = append(args, "-d")
	}
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

func parse_mmlssnapshot(out string, logger *slog.Logger) ([]SnapshotMetric, error) {
	var metrics []SnapshotMetric
	headers := []string{}
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		if !strings.HasPrefix(l, "mmlssnapshot") {
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
		var metric SnapshotMetric
		ps := reflect.ValueOf(&metric) // pointer to struct - addressable
		s := ps.Elem()                 // struct
		for i, h := range headers {
			if field, ok := snapshotMap[h]; ok {
				f := s.FieldByName(field)
				if f.Kind() == reflect.String {
					f.SetString(values[i])
				} else if f.Kind() == reflect.Float64 {
					if h == "created" {
						createdStr, err := url.QueryUnescape(values[i])
						if err != nil {
							logger.Error("Unable to unescape created time", "value", values[i])
							return nil, err
						}
						createdTime, err := time.ParseInLocation(time.ANSIC, createdStr, NowLocation())
						if err != nil {
							logger.Error("Unable to parse time", "value", createdStr)
							return nil, err
						}
						f.SetFloat(float64(createdTime.Unix()))
						continue
					}
					if val, err := strconv.ParseFloat(values[i], 64); err == nil {
						if SliceContains(SnapshotKbToBytes, h) {
							val = val * 1024
						}
						f.SetFloat(val)
					} else {
						logger.Error(fmt.Sprintf("Error parsing %s value %s: %s", h, values[i], err.Error()))
						return nil, err
					}
				}
			}
		}

		metrics = append(metrics, metric)
	}
	return metrics, nil
}
