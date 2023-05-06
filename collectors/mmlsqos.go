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

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	qosFilesystems = kingpin.Flag("collector.mmlsqos.filesystems", "Filesystems to query with mmlsqos, comma separated. Defaults to all filesystems.").Default("").String()
	qosTimeout     = kingpin.Flag("collector.mmlsqos.timeout", "Timeout for mmlsqos execution").Default("60").Int()
	qosSeconds     = kingpin.Flag("collector.mmlsqos.seconds", "Display the I/O performance values for the previous number of seconds. The valid range of seconds is 1-999").Default("60").Int()
	qosMap         = map[string]string{
		"pool":      "Pool",
		"timeEpoch": "Time",
		"class":     "Class",
		"iops":      "Iops",
		"ioql":      "AvegarePendingRequests",
		"qsdl":      "AvegareQueuedRequests",
		"et":        "MeasurementInterval",
		"MBs":       "Bs",
	}
	MmlsqosExec = mmlsqos
)

type QosMetric struct {
	Pool                   string
	Time                   float64
	Class                  string
	Iops                   float64
	AvegarePendingRequests float64
	AvegareQueuedRequests  float64
	MeasurementInterval    float64
	Bs                     float64
}

type MmlsqosCollector struct {
	Iops                   *prometheus.Desc
	AvegarePendingRequests *prometheus.Desc
	AvegareQueuedRequests  *prometheus.Desc
	MeasurementInterval    *prometheus.Desc
	Bs                     *prometheus.Desc
	logger                 log.Logger
}

func init() {
	registerCollector("mmlsqos", false, NewMmlsqosCollector)
}

func NewMmlsqosCollector(logger log.Logger) Collector {
	labels := []string{"fs", "pool", "class", "measurement_period_seconds"}
	return &MmlsqosCollector{
		Iops: prometheus.NewDesc(prometheus.BuildFQName(namespace, "qos", "iops"),
			"GPFS performance of the class in I/O operations per second", labels, nil),
		AvegarePendingRequests: prometheus.NewDesc(prometheus.BuildFQName(namespace, "qos", "average_pending_requests"),
			"GPFS average number of I/O requests in the class that are pending for reasons other than being queued by QoS", labels, nil),
		AvegareQueuedRequests: prometheus.NewDesc(prometheus.BuildFQName(namespace, "qos", "average_queued_requests"),
			"GPFS average number of I/O requests in the class that are queued by QoS", labels, nil),
		MeasurementInterval: prometheus.NewDesc(prometheus.BuildFQName(namespace, "qos", "measurement_interval_seconds"),
			"GPFS interval in seconds during which the measurement was made", labels, nil),
		Bs: prometheus.NewDesc(prometheus.BuildFQName(namespace, "qos", "bytes_per_second"),
			"GPFS performance of the class in Bytes per second", labels, nil),
		logger: logger,
	}
}

func (c *MmlsqosCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Iops
	ch <- c.AvegarePendingRequests
	ch <- c.AvegareQueuedRequests
	ch <- c.MeasurementInterval
	ch <- c.Bs
}

func (c *MmlsqosCollector) Collect(ch chan<- prometheus.Metric) {
	wg := &sync.WaitGroup{}
	var filesystems []string
	if *qosFilesystems == "" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*mmlsfsTimeout)*time.Second)
		defer cancel()
		var mmlsfsTimeout float64
		var mmlsfsError float64
		mmlfsfs_filesystems, err := mmlfsfsFilesystems(ctx, c.logger)
		if err == context.DeadlineExceeded {
			mmlsfsTimeout = 1
			level.Error(c.logger).Log("msg", "Timeout executing mmlsfs")
		} else if err != nil {
			mmlsfsError = 1
			level.Error(c.logger).Log("msg", err)
		}
		ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, mmlsfsTimeout, "mmlsqos-mmlsfs")
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, mmlsfsError, "mmlsqos-mmlsfs")
		filesystems = mmlfsfs_filesystems
	} else {
		filesystems = strings.Split(*qosFilesystems, ",")
	}
	for _, fs := range filesystems {
		level.Debug(c.logger).Log("msg", "Collecting mmlsqos metrics", "fs", fs)
		wg.Add(1)
		collectTime := time.Now()
		go func(fs string) {
			defer wg.Done()
			label := fmt.Sprintf("mmlsqos-%s", fs)
			timeout := 0
			errorMetric := 0
			metrics, err := c.mmlsqosCollect(fs)
			if err == context.DeadlineExceeded {
				level.Error(c.logger).Log("msg", fmt.Sprintf("Timeout executing %s", label))
				timeout = 1
			} else if err != nil {
				level.Error(c.logger).Log("msg", err, "fs", fs)
				errorMetric = 1
			}
			ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, float64(errorMetric), label)
			ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, float64(timeout), label)
			ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), label)
			if err != nil {
				return
			}
			for _, m := range metrics {
				ch <- prometheus.MustNewConstMetric(c.Iops, prometheus.GaugeValue, m.Iops, fs, m.Pool, m.Class, fmt.Sprintf("%.f", m.Time))
				ch <- prometheus.MustNewConstMetric(c.AvegarePendingRequests, prometheus.GaugeValue, m.AvegarePendingRequests, fs, m.Pool, m.Class, fmt.Sprintf("%.f", m.Time))
				ch <- prometheus.MustNewConstMetric(c.AvegareQueuedRequests, prometheus.GaugeValue, m.AvegareQueuedRequests, fs, m.Pool, m.Class, fmt.Sprintf("%.f", m.Time))
				ch <- prometheus.MustNewConstMetric(c.MeasurementInterval, prometheus.GaugeValue, m.MeasurementInterval, fs, m.Pool, m.Class, fmt.Sprintf("%.f", m.Time))
				ch <- prometheus.MustNewConstMetric(c.Bs, prometheus.GaugeValue, m.Bs, fs, m.Pool, m.Class, fmt.Sprintf("%.f", m.Time))
			}
		}(fs)
	}
	wg.Wait()
}

func (c *MmlsqosCollector) mmlsqosCollect(fs string) ([]QosMetric, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*qosTimeout)*time.Second)
	defer cancel()
	out, err := MmlsqosExec(fs, ctx)
	if err != nil {
		return nil, err
	}
	metrics, err := parse_mmlsqos(out, c.logger)
	return metrics, err
}

func mmlsqos(fs string, ctx context.Context) (string, error) {
	args := []string{"/usr/lpp/mmfs/bin/mmlsqos", fs, "-Y", "--seconds", strconv.Itoa(*qosSeconds)}
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

func parse_mmlsqos(out string, logger log.Logger) ([]QosMetric, error) {
	var metrics []QosMetric
	headers := []string{}
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		if !strings.HasPrefix(l, "mmlsqos") {
			continue
		}
		items := strings.Split(l, ":")
		if len(items) < 3 {
			continue
		}
		if items[1] != "stats" {
			continue
		}
		var values []string
		if items[2] == "HEADER" {
			headers = append(headers, items...)
			continue
		} else {
			values = append(values, items...)
		}
		var metric QosMetric
		ps := reflect.ValueOf(&metric) // pointer to struct - addressable
		s := ps.Elem()                 // struct
		for i, h := range headers {
			if field, ok := qosMap[h]; ok {
				f := s.FieldByName(field)
				if f.Kind() == reflect.String {
					f.SetString(values[i])
				} else if f.Kind() == reflect.Float64 {
					if strings.Contains(values[i], "nan") {
						f.SetFloat(0)
					} else if val, err := strconv.ParseFloat(strings.Replace(values[i], ",", ".", -1), 64); err == nil {
						if field == "Bs" {
							val = val * 1024 * 1024
						}
						f.SetFloat(val)
					} else {
						level.Error(logger).Log("msg", fmt.Sprintf("Error parsing %s value %s: %s", h, values[i], err.Error()))
						return nil, err
					}
				}
			}
		}

		metrics = append(metrics, metric)
	}
	return metrics, nil
}
