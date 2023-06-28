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
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	configMmrepquotaFilesystems = kingpin.Flag("collector.mmrepquota.filesystems", "Filesystems to query with mmrepquota, comma separated. Defaults to all filesystems.").Default("").String()
	configMmrepquotaTypes       = kingpin.Flag("collector.mmrepquota.quotatypes", "Quota Types to query with mmrepquota, Default to fileset only").Default("j").String()
	mmrepquotaTimeout           = kingpin.Flag("collector.mmrepquota.timeout", "Timeout for mmrepquota execution").Default("20").Int()
	quotaMap                    = map[string]string{
		"name":           "Name",
		"filesystemName": "FS",
		"quotaType":      "QuotaType",
		"blockUsage":     "BlockUsage",
		"blockQuota":     "BlockQuota",
		"blockLimit":     "BlockLimit",
		"blockInDoubt":   "BlockInDoubt",
		"filesUsage":     "FilesUsage",
		"filesQuota":     "FilesQuota",
		"filesLimit":     "FilesLimit",
		"filesInDoubt":   "FilesInDoubt",
	}
	quotaTypeMap = map[string]rune{
		"user":    'u',
		"group":   'g',
		"fileset": 'j',
	}
	mmrepquotaExec = mmrepquota
)

type QuotaMetric struct {
	Name         string
	FS           string
	QuotaType    string
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
	FilesetBlockUsage   *prometheus.Desc
	FilesetBlockQuota   *prometheus.Desc
	FilesetBlockLimit   *prometheus.Desc
	FilesetBlockInDoubt *prometheus.Desc
	FilesetFilesUsage   *prometheus.Desc
	FilesetFilesQuota   *prometheus.Desc
	FilesetFilesLimit   *prometheus.Desc
	FilesetFilesInDoubt *prometheus.Desc

	UserBlockUsage   *prometheus.Desc
	UserBlockQuota   *prometheus.Desc
	UserBlockLimit   *prometheus.Desc
	UserBlockInDoubt *prometheus.Desc
	UserFilesUsage   *prometheus.Desc
	UserFilesQuota   *prometheus.Desc
	UserFilesLimit   *prometheus.Desc
	UserFilesInDoubt *prometheus.Desc

	GroupBlockUsage   *prometheus.Desc
	GroupBlockQuota   *prometheus.Desc
	GroupBlockLimit   *prometheus.Desc
	GroupBlockInDoubt *prometheus.Desc
	GroupFilesUsage   *prometheus.Desc
	GroupFilesQuota   *prometheus.Desc
	GroupFilesLimit   *prometheus.Desc
	GroupFilesInDoubt *prometheus.Desc

	logger log.Logger
}

func init() {
	registerCollector("mmrepquota", false, NewMmrepquotaCollector)
}

func NewMmrepquotaCollector(logger log.Logger) Collector {
	fileset_labels := []string{"fileset", "fs"}
	user_labels := []string{"user", "fs"}
	group_labels := []string{"group", "fs"}
	return &MmrepquotaCollector{
		FilesetBlockUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "used_bytes"),
			"GPFS fileset quota used", fileset_labels, nil),
		FilesetBlockQuota: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "quota_bytes"),
			"GPFS fileset block quota", fileset_labels, nil),
		FilesetBlockLimit: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "limit_bytes"),
			"GPFS fileset quota block limit", fileset_labels, nil),
		FilesetBlockInDoubt: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "in_doubt_bytes"),
			"GPFS fileset quota block in doubt", fileset_labels, nil),
		FilesetFilesUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "used_files"),
			"GPFS fileset quota files used", fileset_labels, nil),
		FilesetFilesQuota: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "quota_files"),
			"GPFS fileset files quota", fileset_labels, nil),
		FilesetFilesLimit: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "limit_files"),
			"GPFS fileset quota files limit", fileset_labels, nil),
		FilesetFilesInDoubt: prometheus.NewDesc(prometheus.BuildFQName(namespace, "fileset", "in_doubt_files"),
			"GPFS fileset quota files in doubt", fileset_labels, nil),

		UserBlockUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "user", "used_bytes"),
			"GPFS user quota used", user_labels, nil),
		UserBlockQuota: prometheus.NewDesc(prometheus.BuildFQName(namespace, "user", "quota_bytes"),
			"GPFS user block quota", user_labels, nil),
		UserBlockLimit: prometheus.NewDesc(prometheus.BuildFQName(namespace, "user", "limit_bytes"),
			"GPFS user quota block limit", user_labels, nil),
		UserBlockInDoubt: prometheus.NewDesc(prometheus.BuildFQName(namespace, "user", "in_doubt_bytes"),
			"GPFS user quota block in doubt", user_labels, nil),
		UserFilesUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "user", "used_files"),
			"GPFS user quota files used", user_labels, nil),
		UserFilesQuota: prometheus.NewDesc(prometheus.BuildFQName(namespace, "user", "quota_files"),
			"GPFS user files quota", user_labels, nil),
		UserFilesLimit: prometheus.NewDesc(prometheus.BuildFQName(namespace, "user", "limit_files"),
			"GPFS user quota files limit", user_labels, nil),
		UserFilesInDoubt: prometheus.NewDesc(prometheus.BuildFQName(namespace, "user", "in_doubt_files"),
			"GPFS user quota files in doubt", user_labels, nil),

		GroupBlockUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "group", "used_bytes"),
			"GPFS group quota used", group_labels, nil),
		GroupBlockQuota: prometheus.NewDesc(prometheus.BuildFQName(namespace, "group", "quota_bytes"),
			"GPFS group block quota", group_labels, nil),
		GroupBlockLimit: prometheus.NewDesc(prometheus.BuildFQName(namespace, "group", "limit_bytes"),
			"GPFS group quota block limit", group_labels, nil),
		GroupBlockInDoubt: prometheus.NewDesc(prometheus.BuildFQName(namespace, "group", "in_doubt_bytes"),
			"GPFS group quota block in doubt", group_labels, nil),
		GroupFilesUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "group", "used_files"),
			"GPFS group quota files used", group_labels, nil),
		GroupFilesQuota: prometheus.NewDesc(prometheus.BuildFQName(namespace, "group", "quota_files"),
			"GPFS group files quota", group_labels, nil),
		GroupFilesLimit: prometheus.NewDesc(prometheus.BuildFQName(namespace, "group", "limit_files"),
			"GPFS group quota files limit", group_labels, nil),
		GroupFilesInDoubt: prometheus.NewDesc(prometheus.BuildFQName(namespace, "group", "in_doubt_files"),
			"GPFS group quota files in doubt", group_labels, nil),

		logger: logger,
	}
}

func (c *MmrepquotaCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.FilesetBlockUsage
	ch <- c.FilesetBlockQuota
	ch <- c.FilesetBlockLimit
	ch <- c.FilesetBlockInDoubt
	ch <- c.FilesetFilesUsage
	ch <- c.FilesetFilesQuota
	ch <- c.FilesetFilesLimit
	ch <- c.FilesetFilesInDoubt

	ch <- c.UserBlockUsage
	ch <- c.UserBlockQuota
	ch <- c.UserBlockLimit
	ch <- c.UserBlockInDoubt
	ch <- c.UserFilesUsage
	ch <- c.UserFilesQuota
	ch <- c.UserFilesLimit
	ch <- c.UserFilesInDoubt

	ch <- c.GroupBlockUsage
	ch <- c.GroupBlockQuota
	ch <- c.GroupBlockLimit
	ch <- c.GroupBlockInDoubt
	ch <- c.GroupFilesUsage
	ch <- c.GroupFilesQuota
	ch <- c.GroupFilesLimit
	ch <- c.GroupFilesInDoubt
}

func (c *MmrepquotaCollector) Collect(ch chan<- prometheus.Metric) {
	level.Debug(c.logger).Log("msg", "Collecting mmrepquota metrics")
	collectTime := time.Now()
	timeout := 0
	errorMetric := 0
	metrics := []QuotaMetric{}

	for _, quotaType := range strings.Split(*configMmrepquotaTypes, ",") {
		quotaType = strings.TrimSpace(quotaType)
		quotaArg := quotaTypeMap[quotaType]
		metric, err := c.collect(fmt.Sprintf("-%c", quotaArg))
		metrics = append(metrics, metric...)

		if err == context.DeadlineExceeded {
			timeout = 1
			level.Error(c.logger).Log("msg", "Timeout executing mmrepquota")
		} else if err != nil {
			level.Error(c.logger).Log("msg", err)
			errorMetric = 1
		}
	}

	for _, m := range metrics {
		if m.QuotaType == "FILESET" {
			ch <- prometheus.MustNewConstMetric(c.FilesetBlockUsage, prometheus.GaugeValue, m.BlockUsage, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.FilesetBlockQuota, prometheus.GaugeValue, m.BlockQuota, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.FilesetBlockLimit, prometheus.GaugeValue, m.BlockLimit, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.FilesetBlockInDoubt, prometheus.GaugeValue, m.BlockInDoubt, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.FilesetFilesUsage, prometheus.GaugeValue, m.FilesUsage, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.FilesetFilesQuota, prometheus.GaugeValue, m.FilesQuota, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.FilesetFilesLimit, prometheus.GaugeValue, m.FilesLimit, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.FilesetFilesInDoubt, prometheus.GaugeValue, m.FilesInDoubt, m.Name, m.FS)
		} else if m.QuotaType == "USR" {
			ch <- prometheus.MustNewConstMetric(c.UserBlockUsage, prometheus.GaugeValue, m.BlockUsage, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.UserBlockQuota, prometheus.GaugeValue, m.BlockQuota, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.UserBlockLimit, prometheus.GaugeValue, m.BlockLimit, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.UserBlockInDoubt, prometheus.GaugeValue, m.BlockInDoubt, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.UserFilesUsage, prometheus.GaugeValue, m.FilesUsage, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.UserFilesQuota, prometheus.GaugeValue, m.FilesQuota, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.UserFilesLimit, prometheus.GaugeValue, m.FilesLimit, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.UserFilesInDoubt, prometheus.GaugeValue, m.FilesInDoubt, m.Name, m.FS)
		} else if m.QuotaType == "GRP" {
			ch <- prometheus.MustNewConstMetric(c.GroupBlockUsage, prometheus.GaugeValue, m.BlockUsage, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.GroupBlockQuota, prometheus.GaugeValue, m.BlockQuota, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.GroupBlockLimit, prometheus.GaugeValue, m.BlockLimit, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.GroupBlockInDoubt, prometheus.GaugeValue, m.BlockInDoubt, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.GroupFilesUsage, prometheus.GaugeValue, m.FilesUsage, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.GroupFilesQuota, prometheus.GaugeValue, m.FilesQuota, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.GroupFilesLimit, prometheus.GaugeValue, m.FilesLimit, m.Name, m.FS)
			ch <- prometheus.MustNewConstMetric(c.GroupFilesInDoubt, prometheus.GaugeValue, m.FilesInDoubt, m.Name, m.FS)
		}
	}
	ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, float64(errorMetric), "mmrepquota")
	ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, float64(timeout), "mmrepquota")
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "mmrepquota")
}

func (c *MmrepquotaCollector) collect(typeArg string) ([]QuotaMetric, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*mmrepquotaTimeout)*time.Second)
	defer cancel()
	out, err := mmrepquotaExec(ctx, typeArg)
	if err != nil {
		return nil, err
	}
	metric := parse_mmrepquota(out, c.logger)
	return metric, nil
}

func mmrepquota(ctx context.Context, typeArg string) (string, error) {
	args := []string{"/usr/lpp/mmfs/bin/mmrepquota", typeArg, "-Y"}

	if *configMmrepquotaFilesystems == "" {
		args = append(args, "-a")
	} else {
		args = append(args, strings.Split(*configMmrepquotaFilesystems, ",")...)
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
