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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

type DiagMetric struct {
	Waiters []float64
}

type MmdiagCollector struct {
	Waiters    prometheus.Histogram
	WaitersMax *prometheus.Desc
}

func init() {
	registerCollector("mmdiag", false, NewMmdiagCollector)
}

func NewMmdiagCollector() Collector {
	return &MmdiagCollector{
		Waiters: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "mmdiag",
			Name:      "waiters",
			Help:      "GPFS number of waiters",
			Buckets:   []float64{0.5, 1.0, 10.0, 30.0, 60.0, 120.0, 300.0}}),
		WaitersMax: prometheus.NewDesc(prometheus.BuildFQName(namespace, "mmdiag", "waiters_max"),
			"GPFS max waiter in seconds", nil, nil),
	}
}

func (c *MmdiagCollector) Describe(ch chan<- *prometheus.Desc) {
	c.Waiters.Describe(ch)
	ch <- c.WaitersMax
}

func (c *MmdiagCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debug("Collecting mmdiag metrics")
	err := c.collect(ch)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "mmdiag")
	} else {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 0, "mmdiag")
	}
}

func (c *MmdiagCollector) collect(ch chan<- prometheus.Metric) error {
	collectTime := time.Now()
	out, err := mmdiag("--waiters")
	if err != nil {
		return err
	}
	var diagMetric DiagMetric
	err = parse_mmdiag_waiters(out, &diagMetric)
	if err != nil {
		return err
	}
	for _, waiter := range diagMetric.Waiters {
		c.Waiters.Observe(waiter)
	}
	c.Waiters.Collect(ch)
	sort.Float64s(diagMetric.Waiters)
	ch <- prometheus.MustNewConstMetric(c.WaitersMax, prometheus.GaugeValue, diagMetric.Waiters[len(diagMetric.Waiters)-1])
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "mmdiag")
	return nil
}

func mmdiag(arg string) (string, error) {
	cmd := execCommand("sudo", "/usr/lpp/mmfs/bin/mmdiag", arg, "-Y")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Error(err)
		return "", err
	}
	return out.String(), nil
}

func parse_mmdiag_waiters(out string, diagMetric *DiagMetric) error {
	lines := strings.Split(out, "\n")
	waitersPatter := regexp.MustCompile(`^Waiting ([0-9.]+) sec`)
	for _, l := range lines {
		match := waitersPatter.FindStringSubmatch(l)
		if len(match) != 2 {
			continue
		}
		secs, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			log.Errorf("Unable to convert %s to float64", match[1])
			continue
		}
		diagMetric.Waiters = append(diagMetric.Waiters, secs)
	}
	return nil
}
