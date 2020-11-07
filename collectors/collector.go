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
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "gpfs"
)

var (
	collectorState  = make(map[string]*bool)
	factories       = make(map[string]func(logger log.Logger) Collector)
	execCommand     = exec.CommandContext
	MmlsfsExec      = mmlsfs
	collectDuration = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "exporter", "collector_duration_seconds"),
		"Collector time duration.",
		[]string{"collector"}, nil)
	collectError = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "exporter", "collect_error"),
		"Indicates if error has occurred during collection",
		[]string{"collector"}, nil)
	collecTimeout = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "exporter", "collect_timeout"),
		"Indicates the collector timed out",
		[]string{"collector"}, nil)
	lastExecution = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "exporter", "last_execution"),
		"Last execution time of ", []string{"collector"}, nil)
	mmlsfsTimeout = kingpin.Flag("config.mmlsfs.timeout", "Timeout for mmlsfs execution").Default("5").Int()
)

type GPFSFilesystem struct {
	Name       string
	Mountpoint string
}

type GPFSCollector struct {
	sync.Mutex
	Collectors map[string]Collector
}

type Collector interface {
	// Get new metrics and expose them via prometheus registry.
	Describe(ch chan<- *prometheus.Desc)
	Collect(ch chan<- prometheus.Metric)
}

func registerCollector(collector string, isDefaultEnabled bool, factory func(logger log.Logger) Collector) {
	var helpDefaultState string
	if isDefaultEnabled {
		helpDefaultState = "enabled"
	} else {
		helpDefaultState = "disabled"
	}
	flagName := fmt.Sprintf("collector.%s", collector)
	flagHelp := fmt.Sprintf("Enable the %s collector (default: %s).", collector, helpDefaultState)
	defaultValue := fmt.Sprintf("%v", isDefaultEnabled)
	flag := kingpin.Flag(flagName, flagHelp).Default(defaultValue).Bool()
	collectorState[collector] = flag
	factories[collector] = factory
}

func NewGPFSCollector(logger log.Logger) *GPFSCollector {
	collectors := make(map[string]Collector)
	for key, enabled := range collectorState {
		var collector Collector
		if *enabled {
			collector = factories[key](log.With(logger, "collector", key))
			collectors[key] = collector
		}
	}
	return &GPFSCollector{Collectors: collectors}
}

func SliceContains(slice []string, str string) bool {
	for _, s := range slice {
		if str == s {
			return true
		}
	}
	return false
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func mmlsfs(ctx context.Context) (string, error) {
	cmd := execCommand(ctx, "sudo", "/usr/lpp/mmfs/bin/mmlsfs", "all", "-Y", "-T")
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

func parse_mmlsfs(out string) []GPFSFilesystem {
	var filesystems []GPFSFilesystem
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		items := strings.Split(line, ":")
		if len(items) < 7 {
			continue
		}
		if items[2] == "HEADER" {
			continue
		}
		var fs GPFSFilesystem
		fs.Name = items[6]
		mountpoint, err := url.QueryUnescape(items[8])
		if err != nil {
			continue
		}
		fs.Mountpoint = mountpoint
		filesystems = append(filesystems, fs)
	}
	return filesystems
}
