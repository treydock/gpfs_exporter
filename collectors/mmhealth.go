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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	mmhealthTimeout           = kingpin.Flag("collector.mmhealth.timeout", "Timeout for mmhealth execution").Default("5").Int()
	mmhealthIgnoredComponent  = kingpin.Flag("collector.mmhealth.ignored-component", "Regex of components to ignore").Default("^$").String()
	mmhealthIgnoredEntityName = kingpin.Flag("collector.mmhealth.ignored-entityname", "Regex of entity names to ignore").Default("^$").String()
	mmhealthIgnoredEntityType = kingpin.Flag("collector.mmhealth.ignored-entitytype", "Regex of entity types to ignore").Default("^$").String()
	mmhealthIgnoredEvent      = kingpin.Flag("collector.mmhealth.ignored-event", "Regex of events to ignore").Default("").String()
	mmhealthMap               = map[string]string{
		"component":  "Component",
		"entityname": "EntityName",
		"entitytype": "EntityType",
		"status":     "Status",
		"event":      "Event",
	}
	mmhealthStatuses = []string{"CHECKING", "DEGRADED", "DEPEND", "DISABLED", "FAILED", "HEALTHY", "STARTING", "STOPPED", "SUSPENDED", "TIPS"}
	mmhealthExec     = mmhealth
)

type HealthMetric struct {
	Type       string
	Component  string
	EntityName string
	EntityType string
	Status     string
	Event      string
}

type MmhealthCollector struct {
	State  *prometheus.Desc
	Event  *prometheus.Desc
	logger log.Logger
}

func init() {
	registerCollector("mmhealth", false, NewMmhealthCollector)
}

func NewMmhealthCollector(logger log.Logger) Collector {
	return &MmhealthCollector{
		State: prometheus.NewDesc(prometheus.BuildFQName(namespace, "health", "status"),
			"GPFS health status", []string{"component", "entityname", "entitytype", "status"}, nil),
		Event: prometheus.NewDesc(prometheus.BuildFQName(namespace, "health", "event"),
			"GPFS health event", []string{"component", "entityname", "entitytype", "event"}, nil),
		logger: logger,
	}
}

func (c *MmhealthCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.State
	ch <- c.Event
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
		if m.Type == "Event" {
			ch <- prometheus.MustNewConstMetric(c.Event, prometheus.GaugeValue, 1, m.Component, m.EntityName, m.EntityType, m.Event)
			continue
		}
		for _, s := range mmhealthStatuses {
			var value float64
			if s == m.Status {
				value = 1
			}
			ch <- prometheus.MustNewConstMetric(c.State, prometheus.GaugeValue, value, m.Component, m.EntityName, m.EntityType, s)
		}
		var unknown float64
		if !SliceContains(mmhealthStatuses, m.Status) {
			unknown = 1
			level.Warn(c.logger).Log("msg", "Unknown status encountered", "status", m.Status,
				"component", m.Component, "entityname", m.EntityName, "entitytype", m.EntityType)
		}
		ch <- prometheus.MustNewConstMetric(c.State, prometheus.GaugeValue, unknown, m.Component, m.EntityName, m.EntityType, "UNKNOWN")
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
	cmd := execCommand(ctx, *sudoCmd, "/usr/lpp/mmfs/bin/mmhealth", "node", "show", "-Y")
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
	mmhealthIgnoredComponentPattern := regexp.MustCompile(*mmhealthIgnoredComponent)
	mmhealthIgnoredEntityNamePattern := regexp.MustCompile(*mmhealthIgnoredEntityName)
	mmhealthIgnoredEntityTypePattern := regexp.MustCompile(*mmhealthIgnoredEntityType)
	mmhealthIgnoredEventPattern := regexp.MustCompile(*mmhealthIgnoredEvent)
	var metrics []HealthMetric
	var eventKeys []string
	lines := strings.Split(out, "\n")
	typeHeaders := make(map[string][]string)
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if !strings.HasPrefix(l, "mmhealth") {
			level.Debug(logger).Log("msg", "Skip due to prefix", "line", l)
			continue
		}
		items := strings.Split(l, ":")
		if len(items) < 3 {
			level.Debug(logger).Log("msg", "Skip due to length", "len", len(items), "line", l)
			continue
		}
		var metric HealthMetric
		metric.Type = items[1]
		if metric.Type != "State" && metric.Type != "Event" {
			level.Debug(logger).Log("msg", "Skip due to type", "type", metric.Type, "line", l)
			continue
		}
		var headers []string
		var values []string
		if items[2] == "HEADER" {
			typeHeaders[metric.Type] = items
			continue
		} else {
			headers = typeHeaders[metric.Type]
			values = items
		}
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
		if mmhealthIgnoredComponentPattern.MatchString(metric.Component) {
			level.Debug(logger).Log("msg", "Skipping component due to ignored pattern", "component", metric.Component)
			continue
		}
		if mmhealthIgnoredEntityNamePattern.MatchString(metric.EntityName) {
			level.Debug(logger).Log("msg", "Skipping entity name due to ignored pattern", "entityname", metric.EntityName)
			continue
		}
		if mmhealthIgnoredEntityTypePattern.MatchString(metric.EntityType) {
			level.Debug(logger).Log("msg", "Skipping entity type due to ignored pattern", "entitytype", metric.EntityType)
			continue
		}
		if metric.Type == "Event" && *mmhealthIgnoredEvent != "" && mmhealthIgnoredEventPattern.MatchString(metric.Event) {
			level.Debug(logger).Log("msg", "Skipping event due to ignored pattern", "event", metric.Event)
			continue
		}
		if metric.Type == "Event" {
			eventKey := fmt.Sprintf("%s-%s-%s-%s", metric.Component, metric.EntityName, metric.EntityType, metric.Event)
			if SliceContains(eventKeys, eventKey) {
				level.Debug(logger).Log("msg", "Skipping event as already encountered", "event", metric.Event)
				continue
			} else {
				eventKeys = append(eventKeys, eventKey)
			}
		}
		metrics = append(metrics, metric)
	}
	return metrics
}
