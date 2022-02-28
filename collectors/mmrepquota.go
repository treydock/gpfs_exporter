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
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	configMmrepquotaFilesystems = kingpin.Flag("collector.mmrepquota.filesystems", "Filesystems to query with mmrepquota, comma separated. Defaults to all filesystems.").Default("").String()
	mmrepquotaTimeout           = kingpin.Flag("collector.mmrepquota.timeout", "Timeout for mmrepquota execution").Default("20").Int()
	quotaMap                    = map[string]string{
		"name":           "Name",
		"filesystemName": "FS",
		"blockUsage":     "BlockUsage",
		"blockQuota":     "BlockQuota",
		"blockLimit":     "BlockLimit",
		"blockInDoubt":   "BlockInDoubt",
		"filesUsage":     "FilesUsage",
		"filesQuota":     "FilesQuota",
		"filesLimit":     "FilesLimit",
		"filesInDoubt":   "FilesInDoubt",
	}
	mmrepquotaExec = mmrepquota
)

type QuotaMetric struct {
	Name         string
	FS           string
	BlockUsage   float64
	BlockQuota   float64
	BlockLimit   float64
	BlockInDoubt float64
	FilesUsage   float64
	FilesQuota   float64
	FilesLimit   float64
	FilesInDoubt float64
}

type MmrepquotaCollector struct {
	BlockUsage   *prometheus.Desc
	BlockQuota   *prometheus.Desc
	BlockLimit   *prometheus.Desc
	BlockInDoubt *prometheus.Desc
	FilesUsage   *prometheus.Desc
	FilesQuota   *prometheus.Desc
	FilesLimit   *prometheus.Desc
	FilesInDoubt *prometheus.Desc
	logger       log.Logger
}

func init() {
	registerCollector("mmrepquota", false, NewMmrepquotaCollector)
}

func NewMmrepquotaCollector(logger log.Logger) Collector {
	labels := []string{"fileset", "fs"}
	return &MmrepquotaCollector{
		BlockUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "used_bytes"),
			"GPFS fileset quota used", labels, nil),
		BlockQuota: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "quota_bytes"),
			"GPFS fileset block quota", labels, nil),
		BlockLimit: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "limit_bytes"),
			"GPFS fileset quota block limit", labels, nil),
		BlockInDoubt: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "in_doubt_bytes"),
			"GPFS fileset quota block in doubt", labels, nil),
		FilesUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "used_files"),
			"GPFS fileset quota files used", labels, nil),
		FilesQuota: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "quota_files"),
			"GPFS fileset files quota", labels, nil),
		FilesLimit: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "limit_files"),
			"GPFS fileset quota files limit", labels, nil),
		FilesInDoubt: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "in_doubt_files"),
			"GPFS fileset quota files in doubt", labels, nil),
		logger: logger,
	}
}

func (c *MmrepquotaCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.BlockUsage
	ch <- c.BlockQuota
	ch <- c.BlockLimit
	ch <- c.BlockInDoubt
	ch <- c.FilesUsage
	ch <- c.FilesQuota
	ch <- c.FilesLimit
	ch <- c.FilesInDoubt
}

func (c *MmrepquotaCollector) Collect(ch chan<- prometheus.Metric) {
	level.Debug(c.logger).Log("msg", "Collecting mmrepquota metrics")
	collectTime := time.Now()
	timeout := 0
	errorMetric := 0
	metrics, err := c.collect()
	if err == context.DeadlineExceeded {
		timeout = 1
		level.Error(c.logger).Log("msg", "Timeout executing mmrepquota")
	} else if err != nil {
		level.Error(c.logger).Log("msg", err)
		errorMetric = 1
	}

	for _, m := range metrics {
		ch <- prometheus.MustNewConstMetric(c.BlockUsage, prometheus.GaugeValue, m.BlockUsage, m.Name, m.FS)
		ch <- prometheus.MustNewConstMetric(c.BlockQuota, prometheus.GaugeValue, m.BlockQuota, m.Name, m.FS)
		ch <- prometheus.MustNewConstMetric(c.BlockLimit, prometheus.GaugeValue, m.BlockLimit, m.Name, m.FS)
		ch <- prometheus.MustNewConstMetric(c.BlockInDoubt, prometheus.GaugeValue, m.BlockInDoubt, m.Name, m.FS)
		ch <- prometheus.MustNewConstMetric(c.FilesUsage, prometheus.GaugeValue, m.FilesUsage, m.Name, m.FS)
		ch <- prometheus.MustNewConstMetric(c.FilesQuota, prometheus.GaugeValue, m.FilesQuota, m.Name, m.FS)
		ch <- prometheus.MustNewConstMetric(c.FilesLimit, prometheus.GaugeValue, m.FilesLimit, m.Name, m.FS)
		ch <- prometheus.MustNewConstMetric(c.FilesInDoubt, prometheus.GaugeValue, m.FilesInDoubt, m.Name, m.FS)
	}
	ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, float64(errorMetric), "mmrepquota")
	ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, float64(timeout), "mmrepquota")
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "mmrepquota")
}

func (c *MmrepquotaCollector) collect() ([]QuotaMetric, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*mmrepquotaTimeout)*time.Second)
	defer cancel()
	out, err := mmrepquotaExec(ctx)
	if err != nil {
		return nil, err
	}
	metric := parse_mmrepquota(out, c.logger)
	return metric, nil
}

func mmrepquota(ctx context.Context) (string, error) {
	args := []string{"/usr/lpp/mmfs/bin/mmrepquota", "-j", "-Y"}
	if *configMmrepquotaFilesystems == "" {
		args = append(args, "-a")
	} else {
		args = append(args, strings.Split(*configMmrepquotaFilesystems, ",")...)
	}
	cmd := execCommand(ctx, "sudo", args...)
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

func parse_mmrepquota(out string, logger log.Logger) []QuotaMetric {
	var metrics []QuotaMetric
	var headers []string
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		if !strings.HasPrefix(l, "mmrepquota") {
			continue
		}
		items := strings.Split(l, ":")
		if len(items) < 3 {
			continue
		}
		var values []string
		if items[2] == "HEADER" {
			if len(headers) != 0 {
				headers = nil
			}
			headers = append(headers, items...)
			continue
		} else {
			values = append(values, items...)
		}
		if len(headers) != len(values) {
			level.Error(logger).Log("msg", "Header value mismatch", "headers", len(headers), "values", len(values), "line", l)
			continue
		}
		var metric QuotaMetric
		ps := reflect.ValueOf(&metric) // pointer to struct - addressable
		s := ps.Elem()                 // struct
		for i, h := range headers {
			if field, ok := quotaMap[h]; ok {
				f := s.FieldByName(field)
				value := values[i]
				if f.Kind() == reflect.String {
					f.SetString(value)
				} else if f.Kind() == reflect.Float64 {
					if val, err := strconv.ParseFloat(value, 64); err == nil {
						if strings.HasPrefix(field, "Block") {
							val = val * 1024
						}
						f.SetFloat(val)
					} else {
						level.Error(logger).Log("msg", "Error parsing value", "key", h, "value", value, "err", err)
					}
				}
			}
		}
		metrics = append(metrics, metric)
	}
	return metrics
}
