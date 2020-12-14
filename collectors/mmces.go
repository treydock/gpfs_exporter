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
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	osHostname           = os.Hostname
	configNodeName       = kingpin.Flag("collector.mmces.nodename", "CES node name to check, defaults to FQDN").Default("").String()
	mmcesTimeout         = kingpin.Flag("collector.mmces.timeout", "Timeout for mmces execution").Default("5").Int()
	mmcesIgnoredServices = kingpin.Flag("collector.mmces.ignored-services", "Regex of services to ignore").Default("^$").String()
	cesServices          = []string{"AUTH", "BLOCK", "NETWORK", "AUTH_OBJ", "NFS", "OBJ", "SMB", "CES"}
	cesStates            = []string{"DEGRADED", "DEPEND", "DISABLED", "FAILED", "HEALTHY", "STARTING", "STOPPED", "SUSPENDED"}
	mmcesExec            = mmces
)

func getFQDN(logger log.Logger) string {
	hostname, err := osHostname()
	if err != nil {
		level.Info(logger).Log("msg", fmt.Sprintf("Unable to determine FQDN: %s", err.Error()))
		return ""
	}
	return hostname
}

type CESMetric struct {
	Service string
	State   string
}

type MmcesCollector struct {
	State  *prometheus.Desc
	logger log.Logger
}

func init() {
	registerCollector("mmces", false, NewMmcesCollector)
}

func NewMmcesCollector(logger log.Logger) Collector {
	return &MmcesCollector{
		State: prometheus.NewDesc(prometheus.BuildFQName(namespace, "ces", "state"),
			"GPFS CES health status", []string{"service", "state"}, nil),
		logger: logger,
	}
}

func (c *MmcesCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.State
}

func (c *MmcesCollector) Collect(ch chan<- prometheus.Metric) {
	level.Debug(c.logger).Log("msg", "Collecting mmces metrics")
	collectTime := time.Now()
	timeout := 0
	errorMetric := 0
	var nodename string
	if *configNodeName == "" {
		nodename = getFQDN(c.logger)
		if nodename == "" {
			level.Error(c.logger).Log("msg", "collector.mmces.nodename must be defined and could not be determined")
			os.Exit(1)
		}
	} else {
		nodename = *configNodeName
	}
	metrics, err := c.collect(nodename)
	if err == context.DeadlineExceeded {
		level.Error(c.logger).Log("msg", "Timeout executing mmces")
		timeout = 1
	} else if err != nil {
		level.Error(c.logger).Log("msg", err)
		errorMetric = 1
	}
	for _, m := range metrics {
		for _, s := range cesStates {
			var value float64
			if s == m.State {
				value = 1
			}
			ch <- prometheus.MustNewConstMetric(c.State, prometheus.GaugeValue, value, m.Service, s)
		}
		var unknown float64
		if !SliceContains(cesStates, m.State) {
			unknown = 1
			level.Warn(c.logger).Log("msg", "Unknown state encountered", "state", m.State, "service", m.Service)
		}
		ch <- prometheus.MustNewConstMetric(c.State, prometheus.GaugeValue, unknown, m.Service, "UNKNOWN")
	}
	ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, float64(errorMetric), "mmces")
	ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, float64(timeout), "mmces")
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "mmces")
}

func (c *MmcesCollector) collect(nodename string) ([]CESMetric, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*mmcesTimeout)*time.Second)
	defer cancel()
	mmces_state_out, err := mmcesExec(nodename, ctx)
	if err != nil {
		return nil, err
	}
	metrics := mmces_state_show_parse(mmces_state_out)
	return metrics, nil
}

func mmces(nodename string, ctx context.Context) (string, error) {
	cmd := execCommand(ctx, "sudo", "/usr/lpp/mmfs/bin/mmces", "state", "show", "-N", nodename, "-Y")
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

func mmces_state_show_parse(out string) []CESMetric {
	mmcesIgnoredServicesPattern := regexp.MustCompile(*mmcesIgnoredServices)
	var metrics []CESMetric
	lines := strings.Split(out, "\n")
	var headers []string
	var values []string
	for _, l := range lines {
		if !strings.HasPrefix(l, "mmcesstate") {
			continue
		}
		items := strings.Split(l, ":")
		if len(items) < 3 {
			continue
		}
		if items[2] == "HEADER" {
			headers = append(headers, items...)
		} else {
			values = append(values, items...)
		}
	}
	for i, h := range headers {
		if !SliceContains(cesServices, h) {
			continue
		}
		if mmcesIgnoredServicesPattern.MatchString(h) {
			continue
		}
		var metric CESMetric
		metric.Service = h
		metric.State = values[i]
		metrics = append(metrics, metric)
	}
	return metrics
}
