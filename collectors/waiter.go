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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	waiterInfoDelimiter = "<delim>"
)

var (
	defWaiterExclude = "(EventsExporterSenderThread|Fsck)"
	defWaiterBuckets = "1s,5s,15s,1m,5m,60m"
	waiterExclude    = kingpin.Flag("collector.waiter.exclude", "Pattern to exclude for waiters").Default(defWaiterExclude).String()
	waiterBuckets    = kingpin.Flag("collector.waiter.buckets", "Buckets for waiter metrics").Default(defWaiterBuckets).String()
	waiterTimeout    = kingpin.Flag("collector.waiter.timeout", "Timeout for mmdiag execution").Default("5").Int()
)

type WaiterMetric struct {
	seconds    []float64
	infoCounts map[string]float64
}

type Waiter struct {
	name    string
	reason  string
	seconds float64
}

type WaiterCollector struct {
	Waiter     prometheus.Histogram
	WaiterInfo *prometheus.Desc
	buckets    []float64
	logger     log.Logger
}

func init() {
	registerCollector("waiter", false, NewWaiterCollector)
}

func NewWaiterCollector(logger log.Logger) Collector {
	buckets := []float64{}
	bucketDurations := strings.Split(*waiterBuckets, ",")
	for _, bucketDuration := range bucketDurations {
		duration, err := time.ParseDuration(bucketDuration)
		if err != nil {
			level.Error(logger).Log("msg", "Error parsing bucket duration", "duration", bucketDuration)
			continue
		}
		buckets = append(buckets, duration.Seconds())
	}
	sort.Float64s(buckets)
	return &WaiterCollector{
		Waiter: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "waiter",
			Name:      "seconds",
			Help:      "GPFS waiter in seconds",
			Buckets:   buckets,
		}),
		WaiterInfo: prometheus.NewDesc(prometheus.BuildFQName(namespace, "waiter", "info_count"),
			"GPFS waiter info", []string{"waiter", "reason"}, nil),
		buckets: buckets,
		logger:  logger,
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
		info := strings.Split(waiter, waiterInfoDelimiter)
		ch <- prometheus.MustNewConstMetric(c.WaiterInfo, prometheus.GaugeValue, count, info[0], info[1])
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
		seconds = append(seconds, waiter.seconds)
		if waiter.name == "" && waiter.reason == "" {
			continue
		}
		key := fmt.Sprintf("%s%s%s", waiter.name, waiterInfoDelimiter, waiter.reason)
		infoCounts[key] += 1
	}
	waiterMetric.seconds = seconds
	waiterMetric.infoCounts = infoCounts
	return waiterMetric, nil
}

func parse_mmdiag_waiters(out string, logger log.Logger) []Waiter {
	waiters := []Waiter{}
	lines := strings.Split(out, "\n")
	waitersPattern := regexp.MustCompile(`^Waiting ([0-9.]+) sec.*thread ([0-9]+)`)
	waitersInfoPattern := regexp.MustCompile(`^Waiting ([0-9.]+) sec.*thread ([0-9]+) ([a-zA-Z0-9]+): (.+)`)
	excludePattern := regexp.MustCompile(*waiterExclude)
	for _, l := range lines {
		if excludePattern.MatchString(l) {
			continue
		}
		match := waitersPattern.FindStringSubmatch(l)
		if len(match) != 3 {
			continue
		}
		secs, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			level.Error(logger).Log("msg", fmt.Sprintf("Unable to convert %s to float64", match[1]))
			continue
		}
		infoMatch := waitersInfoPattern.FindStringSubmatch(l)
		var name, reason string
		if len(infoMatch) == 5 {
			name = infoMatch[3]
			reason = infoMatch[4]
		} else {
			level.Error(logger).Log("msg", "Unable to extract waiter info", "line", l, "match", fmt.Sprintf("%+v", infoMatch))
		}
		waiter := Waiter{
			name:    name,
			reason:  reason,
			seconds: secs,
		}
		waiters = append(waiters, waiter)
	}
	return waiters
}
