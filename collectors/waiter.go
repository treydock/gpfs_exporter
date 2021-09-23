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
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	defWaiterExclude = "(EventsExporterSenderThread|Fsck)"
	defWaiterBuckets = "1s,5s,15s,1m,5m,60m"
	waiterExclude    = kingpin.Flag("collector.waiter.exclude", "Pattern to exclude for waiters").Default(defWaiterExclude).String()
	waiterBuckets    = DurationBuckets(kingpin.Flag("collector.waiter.buckets", "Buckets for waiter metrics").Default(defWaiterBuckets))
	waiterTimeout    = kingpin.Flag("collector.waiter.timeout", "Timeout for mmdiag execution").Default("5").Int()
	waiterLogReason  = kingpin.Flag("collector.waiter.log-reason", "Log the waiter reason").Default("false").Bool()
	waiterMap        = map[string]string{
		"threadName": "Name",
		"waitTime":   "Seconds",
		"auxReason":  "Reason",
	}
)

type WaiterMetric struct {
	seconds    []float64
	infoCounts map[string]float64
}

type Waiter struct {
	Name    string
	Reason  string
	Seconds float64
}

type WaiterCollector struct {
	Waiter     prometheus.Histogram
	WaiterInfo *prometheus.Desc
	logger     log.Logger
}

func init() {
	registerCollector("waiter", false, NewWaiterCollector)
}

func NewWaiterCollector(logger log.Logger) Collector {
	return &WaiterCollector{
		Waiter: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "waiter",
			Name:      "seconds",
			Help:      "GPFS waiter in seconds",
			Buckets:   *waiterBuckets,
		}),
		WaiterInfo: prometheus.NewDesc(prometheus.BuildFQName(namespace, "waiter", "info_count"),
			"GPFS waiter info", []string{"waiter"}, nil),
		logger: logger,
	}
}

func (c *WaiterCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Waiter.Desc()
	ch <- c.WaiterInfo
}

func (c *WaiterCollector) Collect(ch chan<- prometheus.Metric) {
	level.Debug(c.logger).Log("msg", "Collecting waiter metrics")
	collectTime := time.Now()
	timeout := 0
	errorMetric := 0
	waiterMetric, err := c.collect()
	if err == context.DeadlineExceeded {
		level.Error(c.logger).Log("msg", "Timeout executing mmdiag")
		timeout = 1
	} else if err != nil {
		level.Error(c.logger).Log("msg", err)
		errorMetric = 1
	}
	for _, second := range waiterMetric.seconds {
		c.Waiter.Observe(second)
	}
	if err == nil {
		ch <- c.Waiter
	}
	for waiter, count := range waiterMetric.infoCounts {
		ch <- prometheus.MustNewConstMetric(c.WaiterInfo, prometheus.GaugeValue, count, waiter)
	}
	ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, float64(errorMetric), "waiter")
	ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, float64(timeout), "waiter")
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "waiter")
}

func (c *WaiterCollector) collect() (WaiterMetric, error) {
	var waiterMetric WaiterMetric
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*waiterTimeout)*time.Second)
	defer cancel()
	out, err := MmdiagExec("--waiters", ctx)
	if err != nil {
		return waiterMetric, err
	}
	waiters := parse_mmdiag_waiters(out, c.logger)
	seconds := []float64{}
	infoCounts := make(map[string]float64)
	for _, waiter := range waiters {
		seconds = append(seconds, waiter.Seconds)
		if waiter.Name == "" && waiter.Reason == "" {
			continue
		}
		if *waiterLogReason {
			level.Info(c.logger).Log("msg", "Waiter reason information", "waiter", waiter.Name, "reason", waiter.Reason, "seconds", waiter.Seconds)
		}
		infoCounts[waiter.Name] += 1
	}
	waiterMetric.seconds = seconds
	waiterMetric.infoCounts = infoCounts
	return waiterMetric, nil
}

func parse_mmdiag_waiters(out string, logger log.Logger) []Waiter {
	waiters := []Waiter{}
	lines := strings.Split(out, "\n")
	var headers []string
	excludePattern := regexp.MustCompile(*waiterExclude)
	for _, l := range lines {
		if !strings.HasPrefix(l, "mmdiag") {
			continue
		}
		items := strings.Split(l, ":")
		if len(items) < 3 {
			continue
		}
		if items[1] != "waiters" {
			continue
		}
		var values []string
		if items[2] == "HEADER" {
			headers = append(headers, items...)
			continue
		} else {
			values = append(values, items...)
		}
		var metric Waiter
		ps := reflect.ValueOf(&metric) // pointer to struct - addressable
		s := ps.Elem()                 // struct
		for i, h := range headers {
			if field, ok := waiterMap[h]; ok {
				f := s.FieldByName(field)
				if f.Kind() == reflect.String {
					f.SetString(values[i])
				} else if f.Kind() == reflect.Float64 {
					if val, err := strconv.ParseFloat(values[i], 64); err == nil {
						f.SetFloat(val)
					} else {
						level.Error(logger).Log("msg", fmt.Sprintf("Error parsing %s value %s: %s", h, values[i], err.Error()))
					}
				}
			}
		}
		level.Debug(logger).Log("exclude", *waiterExclude, "name", metric.Name)
		if excludePattern.MatchString(metric.Name) {
			level.Debug(logger).Log("msg", "Skipping waiter due to ignored pattern", "name", metric.Name)
			continue
		}
		waiters = append(waiters, metric)
	}
	return waiters
}
