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
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	mmgetstateTimeout = kingpin.Flag("collector.mmgetstate.timeout", "Timeout for executing mmgetstate").Default("5").Int()
	mmgetstateStates  = []string{"active", "arbitrating", "down"}
	MmgetstateExec    = mmgetstate
)

type MmgetstateMetrics struct {
	state string
}

type MmgetstateCollector struct {
	state  *prometheus.Desc
	logger log.Logger
}

func init() {
	registerCollector("mmgetstate", true, NewMmgetstateCollector)
}

func NewMmgetstateCollector(logger log.Logger) Collector {
	return &MmgetstateCollector{
		state: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "state"),
			"GPFS state", []string{"state"}, nil),
		logger: logger,
	}
}

func (c *MmgetstateCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.state
}

func (c *MmgetstateCollector) Collect(ch chan<- prometheus.Metric) {
	level.Debug(c.logger).Log("msg", "Collecting mmgetstate metrics")
	collectTime := time.Now()
	timeout := 0
	errorMetric := 0
	metric, err := c.collect()
	if err == context.DeadlineExceeded {
		level.Error(c.logger).Log("msg", "Timeout executing mmgetstate")
		timeout = 1
	} else if err != nil {
		level.Error(c.logger).Log("msg", err)
		errorMetric = 1
	}
	for _, state := range mmgetstateStates {
		if state == metric.state {
			ch <- prometheus.MustNewConstMetric(c.state, prometheus.GaugeValue, 1, state)
		} else {
			ch <- prometheus.MustNewConstMetric(c.state, prometheus.GaugeValue, 0, state)
		}
	}
	if !SliceContains(mmgetstateStates, metric.state) {
		ch <- prometheus.MustNewConstMetric(c.state, prometheus.GaugeValue, 1, "unknown")
	} else {
		ch <- prometheus.MustNewConstMetric(c.state, prometheus.GaugeValue, 0, "unknown")
	}
	ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, float64(errorMetric), "mmgetstate")
	ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, float64(timeout), "mmgetstate")
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "mmgetstate")
}

func (c *MmgetstateCollector) collect() (MmgetstateMetrics, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*mmgetstateTimeout)*time.Second)
	defer cancel()
	out, err := MmgetstateExec(ctx)
	if err != nil {
		return MmgetstateMetrics{}, err
	}
	metric := mmgetstate_parse(out)
	return metric, nil
}

func mmgetstate(ctx context.Context) (string, error) {
	cmd := execCommand(ctx, "sudo", "/usr/lpp/mmfs/bin/mmgetstate", "-Y")
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

func mmgetstate_parse(out string) MmgetstateMetrics {
	metric := MmgetstateMetrics{}
	lines := strings.Split(out, "\n")
	var headers []string
	for _, l := range lines {
		if !strings.HasPrefix(l, "mmgetstate") {
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
		for i, h := range headers {
			switch h {
			case "state":
				metric.state = values[i]
			}
		}
	}
	return metric
}
