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
	filesetFilesystems = kingpin.Flag("collector.mmlsfileset.filesystems", "Filesystems to query with mmlsfileset, comma separated. Defaults to all filesystems.").Default("").String()
	filesetTimeout     = kingpin.Flag("collector.mmlsfileset.timeout", "Timeout for mmlsfileset execution").Default("60").Int()
	filesetMap         = map[string]string{
		"filesystemName": "FS",
		"filesetName":    "Fileset",
		"status":         "Status",
		"path":           "Path",
		"created":        "Created",
		"maxInodes":      "MaxInodes",
		"allocInodes":    "AllocInodes",
		"freeInodes":     "FreeInodes",
	}
	MmlsfilesetExec = mmlsfileset
)

type FilesetMetric struct {
	FS          string
	Fileset     string
	Status      string
	Path        string
	Created     float64
	MaxInodes   float64
	AllocInodes float64
	FreeInodes  float64
}

type MmlsfilesetCollector struct {
	Status      *prometheus.Desc
	Path        *prometheus.Desc
	Created     *prometheus.Desc
	MaxInodes   *prometheus.Desc
	AllocInodes *prometheus.Desc
	FreeInodes  *prometheus.Desc
	logger      *slog.Logger
}

func init() {
	registerCollector("mmlsfileset", false, NewMmlsfilesetCollector)
}

func NewMmlsfilesetCollector(logger *slog.Logger) Collector {
	labels := []string{"fs", "fileset"}
	return &MmlsfilesetCollector{
		Status: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "status_info"),
			"GPFS fileset status", append(labels, []string{"status"}...), nil),
		Path: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "path_info"),
			"GPFS fileset path", append(labels, []string{"path"}...), nil),
		Created: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "created_timestamp_seconds"),
			"GPFS fileset creation timestamp", labels, nil),
		MaxInodes: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "max_inodes"),
			"GPFS fileset max inodes", labels, nil),
		AllocInodes: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "alloc_inodes"),
			"GPFS fileset alloc inodes", labels, nil),
		FreeInodes: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "free_inodes"),
			"GPFS fileset free inodes", labels, nil),
		logger: logger,
	}
}

func (c *MmlsfilesetCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Status
	ch <- c.Path
	ch <- c.Created
	ch <- c.MaxInodes
	ch <- c.AllocInodes
	ch <- c.FreeInodes
}

func (c *MmlsfilesetCollector) Collect(ch chan<- prometheus.Metric) {
	wg := &sync.WaitGroup{}
	var filesystems []string
	if *filesetFilesystems == "" {
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
			c.logger.Error("Error collecting metrics", "err", err)
		}
		ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, mmlsfsTimeout, "mmlsfileset-mmlsfs")
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, mmlsfsError, "mmlsfileset-mmlsfs")
		filesystems = mmlfsfs_filesystems
	} else {
		filesystems = strings.Split(*filesetFilesystems, ",")
	}
	for _, fs := range filesystems {
		c.logger.Debug("Collecting mmlsfileset metrics", "fs", fs)
		wg.Add(1)
		collectTime := time.Now()
		go func(fs string) {
			defer wg.Done()
			label := fmt.Sprintf("mmlsfileset-%s", fs)
			timeout := 0
			errorMetric := 0
			metrics, err := c.mmlsfilesetCollect(fs)
			if err == context.DeadlineExceeded {
				c.logger.Error(fmt.Sprintf("Timeout executing %s", label))
				timeout = 1
			} else if err != nil {
				c.logger.Error("Error collecting metrics", "err", err, "fs", fs)
				errorMetric = 1
			}
			ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, float64(errorMetric), label)
			ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, float64(timeout), label)
			ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), label)
			if err != nil {
				return
			}
			for _, m := range metrics {
				ch <- prometheus.MustNewConstMetric(c.Status, prometheus.GaugeValue, 1, m.FS, m.Fileset, m.Status)
				ch <- prometheus.MustNewConstMetric(c.Path, prometheus.GaugeValue, 1, m.FS, m.Fileset, m.Path)
				ch <- prometheus.MustNewConstMetric(c.Created, prometheus.GaugeValue, m.Created, m.FS, m.Fileset)
				ch <- prometheus.MustNewConstMetric(c.MaxInodes, prometheus.GaugeValue, m.MaxInodes, m.FS, m.Fileset)
				ch <- prometheus.MustNewConstMetric(c.AllocInodes, prometheus.GaugeValue, m.AllocInodes, m.FS, m.Fileset)
				ch <- prometheus.MustNewConstMetric(c.FreeInodes, prometheus.GaugeValue, m.FreeInodes, m.FS, m.Fileset)
			}
		}(fs)
	}
	wg.Wait()
}

func (c *MmlsfilesetCollector) mmlsfilesetCollect(fs string) ([]FilesetMetric, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*filesetTimeout)*time.Second)
	defer cancel()
	out, err := MmlsfilesetExec(fs, ctx)
	if err != nil {
		return nil, err
	}
	metrics, err := parse_mmlsfileset(out, c.logger)
	return metrics, err
}

func mmlsfileset(fs string, ctx context.Context) (string, error) {
	cmd := execCommand(ctx, *sudoCmd, "/usr/lpp/mmfs/bin/mmlsfileset", fs, "-Y")
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

func parse_mmlsfileset(out string, logger *slog.Logger) ([]FilesetMetric, error) {
	var metrics []FilesetMetric
	headers := []string{}
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		if !strings.HasPrefix(l, "mmlsfileset") {
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
		var metric FilesetMetric
		ps := reflect.ValueOf(&metric) // pointer to struct - addressable
		s := ps.Elem()                 // struct
		for i, h := range headers {
			if field, ok := filesetMap[h]; ok {
				f := s.FieldByName(field)
				if f.Kind() == reflect.String {
					value := values[i]
					if h == "path" {
						pathParsed, err := url.QueryUnescape(values[i])
						if err != nil {
							logger.Error("Unable to unescape path", "value", values[i])
							return nil, err
						}
						value = pathParsed
					}
					f.SetString(value)
				} else if f.Kind() == reflect.Float64 {
					var value float64
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
						value = float64(createdTime.Unix())
					} else if val, err := strconv.ParseFloat(values[i], 64); err == nil {
						value = val
					} else {
						logger.Error(fmt.Sprintf("Error parsing %s value %s: %s", h, values[i], err.Error()))
						return nil, err
					}
					f.SetFloat(value)
				}
			}
		}

		metrics = append(metrics, metric)
	}
	return metrics, nil
}
