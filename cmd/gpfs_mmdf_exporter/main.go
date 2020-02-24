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
	"github.com/gofrs/flock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"github.com/treydock/gpfs_exporter/collectors"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	output   = kingpin.Flag("output", "Path to node exporter collected file").Required().String()
	lockFile = kingpin.Flag("lockfile", "Lock file path").Default("/tmp/gpfs_mmdf_exporter.lock").String()
)

func collect() {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewMmdfCollector())
	err := prometheus.WriteToTextfile(*output, registry)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("gpfs_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	fileLock := flock.New(*lockFile)
	//nolint:errcheck
	defer fileLock.Unlock()
	locked, err := fileLock.TryLock()
	if err != nil {
		log.Errorln("Unable to obtain lock on lock file", *lockFile)
		log.Fatal(err)
	}
	if !locked {
		log.Fatalf("Lock file %s is locked", *lockFile)
	}
	collect()
}
