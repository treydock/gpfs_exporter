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
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	osHostname     = os.Hostname
	configNodeName = kingpin.Flag("collector.mmces.nodename", "CES node name to check, defaults to FQDN").Default("").String()
	mmcesTimeout   = kingpin.Flag("collector.mmces.timeout", "Timeout for mmces execution").Default("5").Int()
	cesServices    = []string{"AUTH", "BLOCK", "NETWORK", "AUTH_OBJ", "NFS", "OBJ", "SMB", "CES"}
	mmcesCache     = []CESMetric{}
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
			"GPFS CES health status, 1=healthy 0=not healthy", []string{"service", "state"}, nil),
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
	metrics, err := c.collect()
	if err == context.DeadlineExceeded {
		level.Error(c.logger).Log("msg", "Timeout executing mmces")
		timeout = 1
	} else if err != nil {
		level.Error(c.logger).Log("msg", err)
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "mmces")
	} else {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 0, "mmces")
	}
	ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, float64(timeout), "mmces")
	for _, m := range metrics {
		statusValue := parseMmcesState(m.State)
		ch <- prometheus.MustNewConstMetric(c.State, prometheus.GaugeValue, statusValue, m.Service, m.State)
	}
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "mmces")
}

func (c *MmcesCollector) collect() ([]CESMetric, error) {
	var err error
	var mmces_state_out string
	var metrics []CESMetric
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*mmcesTimeout)*time.Second)
	defer cancel()
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
	mmces_state_out, err = mmces_state_show(nodename, ctx)
	if ctx.Err() == context.DeadlineExceeded {
		if *useCache {
			metrics = mmcesCache
		}
		return metrics, ctx.Err()
	}
	if err != nil {
		if *useCache {
			metrics = mmcesCache
		}
		return metrics, err
	}
	metrics, err = mmces_state_show_parse(mmces_state_out)
	if err != nil {
		if *useCache {
			metrics = mmcesCache
		}
		return metrics, err
	}
	if *useCache {
		mmcesCache = metrics
	}
	return metrics, nil
}

func mmces_state_show(nodename string, ctx context.Context) (string, error) {
	cmd := execCommand(ctx, "sudo", "/usr/lpp/mmfs/bin/mmces", "state", "show", "-N", nodename, "-Y")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

func mmces_state_show_parse(out string) ([]CESMetric, error) {
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
		var metric CESMetric
		metric.Service = h
		metric.State = values[i]
		metrics = append(metrics, metric)
	}
	return metrics, nil
}

func parseMmcesState(status string) float64 {
	if bytes.Equal([]byte(status), []byte("HEALTHY")) {
		return 1
	} else {
		return 0
	}
}
