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
	defWaiterExclude      = "(EventsExporterSenderThread|Fsck)"
	configWaiterThreshold = kingpin.Flag("collector.mmdiag.waiter-threshold", "Threshold for collected waiters").Default("30").Int()
	configWaiterExclude   = kingpin.Flag("collector.mmdiag.waiter-exclude", "Pattern to exclude for waiters").Default(defWaiterExclude).String()
	mmdiagTimeout         = kingpin.Flag("collector.mmdiag.timeout", "Timeout for mmdiag execution").Default("5").Int()
	mmdiagCache           = DiagMetric{}
	mmdiagExec            = mmdiag
)

type DiagMetric struct {
	Waiters []DiagWaiter
}

type DiagWaiter struct {
	Seconds float64
	Thread  string
}

type MmdiagCollector struct {
	Waiter   *prometheus.Desc
	logger   log.Logger
	useCache bool
}

func init() {
	registerCollector("mmdiag", false, NewMmdiagCollector)
}

func NewMmdiagCollector(logger log.Logger, useCache bool) Collector {
	return &MmdiagCollector{
		Waiter: prometheus.NewDesc(prometheus.BuildFQName(namespace, "mmdiag", "waiter"),
			"GPFS max waiter in seconds", []string{"thread"}, nil),
		logger:   logger,
		useCache: useCache,
	}
}

func (c *MmdiagCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Waiter
}

func (c *MmdiagCollector) Collect(ch chan<- prometheus.Metric) {
	level.Debug(c.logger).Log("msg", "Collecting mmdiag metrics")
	collectTime := time.Now()
	timeout := 0
	errorMetric := 0
	diagMetric, err := c.collect()
	if err == context.DeadlineExceeded {
		level.Error(c.logger).Log("msg", "Timeout executing mmdiag")
		timeout = 1
	} else if err != nil {
		level.Error(c.logger).Log("msg", err)
		errorMetric = 1
	}
	for _, waiter := range diagMetric.Waiters {
		ch <- prometheus.MustNewConstMetric(c.Waiter, prometheus.GaugeValue, waiter.Seconds, waiter.Thread)
	}
	ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, float64(errorMetric), "mmdiag")
	ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, float64(timeout), "mmdiag")
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "mmdiag")
}

func (c *MmdiagCollector) collect() (DiagMetric, error) {
	var diagMetric DiagMetric
	var out string
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*mmdiagTimeout)*time.Second)
	defer cancel()
	out, err = mmdiagExec("--waiters", ctx)
	if ctx.Err() == context.DeadlineExceeded {
		if c.useCache {
			diagMetric = mmdiagCache
		}
		return diagMetric, ctx.Err()
	}
	if err != nil {
		if c.useCache {
			diagMetric = mmdiagCache
		}
		return diagMetric, err
	}
	err = parse_mmdiag_waiters(out, &diagMetric, c.logger)
	if err != nil {
		if c.useCache {
			diagMetric = mmdiagCache
		}
		return diagMetric, err
	}
	if c.useCache {
		mmdiagCache = diagMetric
	}
	return diagMetric, nil
}

func mmdiag(arg string, ctx context.Context) (string, error) {
	cmd := execCommand(ctx, "sudo", "/usr/lpp/mmfs/bin/mmdiag", arg, "-Y")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

func parse_mmdiag_waiters(out string, diagMetric *DiagMetric, logger log.Logger) error {
	lines := strings.Split(out, "\n")
	waitersPattern := regexp.MustCompile(`^Waiting ([0-9.]+) sec.*thread ([0-9]+)`)
	excludePattern := regexp.MustCompile(*configWaiterExclude)
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
		threshold := float64(*configWaiterThreshold)
		if secs >= threshold {
			waiter := DiagWaiter{Seconds: secs, Thread: match[2]}
			diagMetric.Waiters = append(diagMetric.Waiters, waiter)
		}
	}
	return nil
}
