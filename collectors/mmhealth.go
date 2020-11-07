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

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	mmhealthTimeout = kingpin.Flag("collector.mmhealth.timeout", "Timeout for mmhealth execution").Default("5").Int()
	mmhealthMap     = map[string]string{
		"component":  "Component",
		"entityname": "EntityName",
		"entitytype": "EntityType",
		"status":     "Status",
	}
	mmhealthExec = mmhealth
)

type HealthMetric struct {
	Component  string
	EntityName string
	EntityType string
	Status     string
}

type MmhealthCollector struct {
	State  *prometheus.Desc
	logger log.Logger
}

func init() {
	registerCollector("mmhealth", false, NewMmhealthCollector)
}

func NewMmhealthCollector(logger log.Logger) Collector {
	return &MmhealthCollector{
		State: prometheus.NewDesc(prometheus.BuildFQName(namespace, "health", "status"),
			"GPFS health status, 1=healthy 0=not healthy", []string{"component", "entityname", "entitytype", "status"}, nil),
		logger: logger,
	}
}

func (c *MmhealthCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.State
}

func (c *MmhealthCollector) Collect(ch chan<- prometheus.Metric) {
	level.Debug(c.logger).Log("msg", "Collecting mmhealth metrics")
	collectTime := time.Now()
	timeout := 0
	errorMetric := 0
	metrics, err := c.collect()
	if err == context.DeadlineExceeded {
		timeout = 1
		level.Error(c.logger).Log("msg", "Timeout executing mmhealth")
	} else if err != nil {
		level.Error(c.logger).Log("msg", err)
		errorMetric = 1
	}
	for _, m := range metrics {
		statusValue := parseMmhealthStatus(m.Status)
		ch <- prometheus.MustNewConstMetric(c.State, prometheus.GaugeValue, statusValue, m.Component, m.EntityName, m.EntityType, m.Status)
	}
	ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, float64(errorMetric), "mmhealth")
	ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, float64(timeout), "mmhealth")
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "mmhealth")
}

func (c *MmhealthCollector) collect() ([]HealthMetric, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*mmhealthTimeout)*time.Second)
	defer cancel()
	mmhealth_out, err := mmhealthExec(ctx)
	if err != nil {
		return nil, err
	}
	metrics := mmhealth_parse(mmhealth_out, c.logger)
	return metrics, nil
}

func mmhealth(ctx context.Context) (string, error) {
	cmd := execCommand(ctx, "sudo", "/usr/lpp/mmfs/bin/mmhealth", "node", "show", "-Y")
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

func mmhealth_parse(out string, logger log.Logger) []HealthMetric {
	var metrics []HealthMetric
	lines := strings.Split(out, "\n")
	var headers []string
	for _, l := range lines {
		if !strings.HasPrefix(l, "mmhealth") {
			continue
		}
		items := strings.Split(l, ":")
		if len(items) < 3 {
			continue
		}
		if items[1] != "State" {
			continue
		}
		var values []string
		if items[2] == "HEADER" {
			headers = append(headers, items...)
			continue
		} else {
			values = append(values, items...)
		}
		var metric HealthMetric
		ps := reflect.ValueOf(&metric) // pointer to struct - addressable
		s := ps.Elem()                 // struct
		for i, h := range headers {
			if field, ok := mmhealthMap[h]; ok {
				f := s.FieldByName(field)
				if f.Kind() == reflect.String {
					f.SetString(values[i])
				} else if f.Kind() == reflect.Int64 {
					if val, err := strconv.ParseInt(values[i], 10, 64); err == nil {
						f.SetInt(val)
					} else {
						level.Error(logger).Log("msg", fmt.Sprintf("Error parsing %s value %s: %s", h, values[i], err.Error()))
					}
				}
			}
		}
		metrics = append(metrics, metric)
	}
	return metrics
}

func parseMmhealthStatus(status string) float64 {
	if bytes.Equal([]byte(status), []byte("HEALTHY")) {
		return 1
	} else {
		return 0
	}
}
