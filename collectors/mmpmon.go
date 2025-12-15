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
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	mmpmonTimeout = kingpin.Flag("collector.mmpmon.timeout", "Timeout for mmpmon execution").Default("5").Int()
	mmpmonMap     = map[string]string{
		"_fs_":  "FS",
		"_nn_":  "NodeName",
		"_br_":  "ReadBytes",
		"_bw_":  "WriteBytes",
		"_rdc_": "Reads",
		"_wc_":  "Writes",
		"_oc_":  "Opens",
		"_cc_":  "Closes",
		"_dir_": "ReadDir",
		"_iu_":  "InodeUpdates",
	}
	MmpmonExec = mmpmon
)

type PerfMetrics struct {
	FS           string
	NodeName     string
	ReadBytes    int64
	WriteBytes   int64
	Reads        int64
	Writes       int64
	Opens        int64
	Closes       int64
	ReadDir      int64
	InodeUpdates int64
}

type MmpmonCollector struct {
	read_bytes  *prometheus.Desc
	write_bytes *prometheus.Desc
	operations  *prometheus.Desc
	info        *prometheus.Desc
	logger      *slog.Logger
}

func init() {
	registerCollector("mmpmon", true, NewMmpmonCollector)
}

func NewMmpmonCollector(logger *slog.Logger) Collector {
	return &MmpmonCollector{
		read_bytes: prometheus.NewDesc(prometheus.BuildFQName(namespace, "perf", "read_bytes_total"),
			"GPFS read bytes", []string{"fs"}, nil),
		write_bytes: prometheus.NewDesc(prometheus.BuildFQName(namespace, "perf", "write_bytes_total"),
			"GPFS write bytes", []string{"fs"}, nil),
		operations: prometheus.NewDesc(prometheus.BuildFQName(namespace, "perf", "operations_total"),
			"GPFS operationgs reported by mmpmon", []string{"fs", "operation"}, nil),
		info: prometheus.NewDesc(prometheus.BuildFQName(namespace, "perf", "info"),
			"GPFS client information", []string{"fs", "nodename"}, nil),
		logger: logger,
	}
}

func (c *MmpmonCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.read_bytes
	ch <- c.write_bytes
	ch <- c.operations
	ch <- c.info
}

func (c *MmpmonCollector) Collect(ch chan<- prometheus.Metric) {
	c.logger.Debug("Collecting mmpmon metrics")
	collectTime := time.Now()
	timeout := 0
	errorMetric := 0
	perfs, err := c.collect()
	if err == context.DeadlineExceeded {
		timeout = 1
		c.logger.Error("Timeout executing mmpmon")
	} else if err != nil {
		c.logger.Error("Cannot collect", slog.Any("err", err))
		errorMetric = 1
	}
	for _, perf := range perfs {
		ch <- prometheus.MustNewConstMetric(c.read_bytes, prometheus.CounterValue, float64(perf.ReadBytes), perf.FS)
		ch <- prometheus.MustNewConstMetric(c.write_bytes, prometheus.CounterValue, float64(perf.WriteBytes), perf.FS)
		ch <- prometheus.MustNewConstMetric(c.operations, prometheus.CounterValue, float64(perf.Reads), perf.FS, "reads")
		ch <- prometheus.MustNewConstMetric(c.operations, prometheus.CounterValue, float64(perf.Writes), perf.FS, "writes")
		ch <- prometheus.MustNewConstMetric(c.operations, prometheus.CounterValue, float64(perf.Opens), perf.FS, "opens")
		ch <- prometheus.MustNewConstMetric(c.operations, prometheus.CounterValue, float64(perf.Closes), perf.FS, "closes")
		ch <- prometheus.MustNewConstMetric(c.operations, prometheus.CounterValue, float64(perf.ReadDir), perf.FS, "read_dir")
		ch <- prometheus.MustNewConstMetric(c.operations, prometheus.CounterValue, float64(perf.InodeUpdates), perf.FS, "inode_updates")
		ch <- prometheus.MustNewConstMetric(c.info, prometheus.GaugeValue, 1, perf.FS, perf.NodeName)
	}
	ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, float64(errorMetric), "mmpmon")
	ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, float64(timeout), "mmpmon")
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "mmpmon")
}

func (c *MmpmonCollector) collect() ([]PerfMetrics, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*mmpmonTimeout)*time.Second)
	defer cancel()
	mmpmon_out, err := MmpmonExec(ctx)
	if err != nil {
		return nil, err
	}
	perfs := mmpmon_parse(mmpmon_out, c.logger)
	return perfs, nil
}

func mmpmon(ctx context.Context) (string, error) {
	cmd := execCommand(ctx, *sudoCmd, "/usr/lpp/mmfs/bin/mmpmon", "-s", "-p")
	cmd.Stdin = strings.NewReader("fs_io_s\n")
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

func mmpmon_parse(out string, logger *slog.Logger) []PerfMetrics {
	var metrics []PerfMetrics
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		if !strings.HasPrefix(l, "_") {
			continue
		}
		var headers []string
		var values []string
		items := strings.Split(l, " ")
		for _, i := range items[1:] {
			if strings.HasPrefix(i, "_") {
				headers = append(headers, i)
			} else {
				values = append(values, i)
			}
		}
		var perf PerfMetrics
		ps := reflect.ValueOf(&perf) // pointer to struct - addressable
		s := ps.Elem()               // struct
		for i, h := range headers {
			if field, ok := mmpmonMap[h]; ok {
				f := s.FieldByName(field)
				if f.Kind() == reflect.String {
					f.SetString(values[i])
				} else if f.Kind() == reflect.Int64 {
					if val, err := strconv.ParseInt(values[i], 10, 64); err == nil {
						f.SetInt(val)
					} else {
						logger.Error(fmt.Sprintf("Error parsing %s value %s: %s", h, values[i], err.Error()))
					}
				}
			}
		}
		metrics = append(metrics, perf)
	}
	return metrics
}
