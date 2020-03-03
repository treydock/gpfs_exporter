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

package main

import (
	"fmt"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gofrs/flock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/treydock/gpfs_exporter/collectors"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	output   = kingpin.Flag("output", "Path to node exporter collected file").Required().String()
	lockFile = kingpin.Flag("lockfile", "Lock file path").Default("/tmp/gpfs_mmdf_exporter.lock").String()
)

func collect(logger log.Logger) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewMmdfCollector(logger))
	err := prometheus.WriteToTextfile(*output, registry)
	if err != nil {
		level.Error(logger).Log("msg", err)
		os.Exit(1)
	}
}

func main() {
	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("gpfs_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promlog.New(promlogConfig)

	fileLock := flock.New(*lockFile)
	//nolint:errcheck
	defer fileLock.Unlock()
	locked, err := fileLock.TryLock()
	if err != nil {
		level.Error(logger).Log("msg", "Unable to obtain lock on lock file", "lockfile", *lockFile)
		level.Error(logger).Log("msg", err)
		os.Exit(1)
	}
	if !locked {
		level.Error(logger).Log("msg", fmt.Sprintf("Lock file %s is locked", *lockFile))
		os.Exit(1)
	}
	collect(logger)
}
