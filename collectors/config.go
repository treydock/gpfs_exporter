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
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	configs       = []string{"pagepool"}
	configTimeout = kingpin.Flag("collector.config.timeout", "Timeout for 'mmdiag --config' execution").Default("5").Int()
)

type ConfigMetric struct {
	PagePool float64
}

type ConfigCollector struct {
	PagePool *prometheus.Desc
	logger   log.Logger
}

func init() {
	registerCollector("config", true, NewConfigCollector)
}

func NewConfigCollector(logger log.Logger) Collector {
	return &ConfigCollector{
		PagePool: prometheus.NewDesc(prometheus.BuildFQName(namespace, "config", "page_pool_bytes"),
			"GPFS configured page pool size", nil, nil),
		logger: logger,
	}
}

func (c *ConfigCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.PagePool
}

func (c *ConfigCollector) Collect(ch chan<- prometheus.Metric) {
	level.Debug(c.logger).Log("msg", "Collecting config metrics")
	collectTime := time.Now()
	timeout := 0
	errorMetric := 0
	metrics, err := c.collect()
	if err == context.DeadlineExceeded {
		level.Error(c.logger).Log("msg", "Timeout executing 'mmdiag --config'")
		timeout = 1
	} else if err != nil {
		level.Error(c.logger).Log("msg", err)
		errorMetric = 1
	}

	if err == nil {
		ch <- prometheus.MustNewConstMetric(c.PagePool, prometheus.GaugeValue, metrics.PagePool)
	}

	ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, float64(errorMetric), "config")
	ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, float64(timeout), "config")
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "config")
}

func (c *ConfigCollector) collect() (ConfigMetric, error) {
	var configMetric ConfigMetric
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*configTimeout)*time.Second)
	defer cancel()
	out, err := MmdiagExec("--config", ctx)
	if err != nil {
		return configMetric, err
	}
	parse_mmdiag_config(out, &configMetric, c.logger)
	return configMetric, nil
}

func parse_mmdiag_config(out string, configMetric *ConfigMetric, logger log.Logger) {
	lines := strings.Split(out, "\n")
	var keyIdx int
	var valueIdx int
	for _, line := range lines {
		items := strings.Split(line, ":")
		if len(items) < 3 {
			continue
		}
		if items[2] == "HEADER" {
			for i, header := range items {
				if header == "name" {
					keyIdx = i
				} else if header == "value" {
					valueIdx = i
				}
			}
			continue
		}
		if (len(items) - 1) < keyIdx {
			continue
		}
		if !SliceContains(configs, items[keyIdx]) {
			continue
		}
		value, err := strconv.ParseFloat(items[valueIdx], 64)
		if err != nil {
			level.Error(logger).Log("msg", fmt.Sprintf("Unable to convert %s to float64", items[valueIdx]), "err", err)
			continue
		}
		switch items[keyIdx] {
		case "pagepool":
			configMetric.PagePool = value
		}
	}
}
