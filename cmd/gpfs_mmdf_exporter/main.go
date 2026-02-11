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
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gofrs/flock"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
	"github.com/treydock/gpfs_exporter/collectors"
)

var (
	output   = kingpin.Flag("output", "Path to node exporter collected file").Required().String()
	lockFile = kingpin.Flag("lockfile", "Lock file path").Default("/tmp/gpfs_mmdf_exporter.lock").String()
)

func writeMetrics(mfs []*dto.MetricFamily, logger *slog.Logger) error {
	tmp, err := os.CreateTemp(filepath.Dir(*output), filepath.Base(*output))
	if err != nil {
		logger.Error("Unable to create temp file", "err", err)
		return err
	}
	defer os.Remove(tmp.Name())
	for _, mf := range mfs {
		if _, err := expfmt.MetricFamilyToText(tmp, mf); err != nil {
			logger.Error("Error generating metric text", "err", err)
			return err
		}
	}
	if err := tmp.Close(); err != nil {
		logger.Error("Error closing tmp file", "err", err)
		return err
	}
	if err := os.Chmod(tmp.Name(), 0644); err != nil {
		logger.Error("Error executing chmod 0644 on tmp file", "err", err)
		return err
	}
	logger.Debug("Renaming temp file to output", "temp", tmp.Name(), "output", *output)
	if err := os.Rename(tmp.Name(), *output); err != nil {
		logger.Error("Error renaming tmp file to output", "err", err)
		return err
	}
	return nil
}

func collect(logger *slog.Logger) error {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewMmdfCollector(logger))
	var newMfs []*dto.MetricFamily
	var failures []string
	mfs, err := registry.Gather()
	if err != nil {
		logger.Error("Error executing Gather", "err", err)
		return err
	}
	for _, mf := range mfs {
		if strings.HasPrefix(mf.GetName(), "gpfs_exporter") {
			newMfs = append(newMfs, mf)
		}
		if mf.GetName() != "gpfs_exporter_collect_error" && mf.GetName() != "gpfs_exporter_collect_timeout" {
			continue
		}
		for _, m := range mf.GetMetric() {
			if m.GetGauge().GetValue() != 1 {
				continue
			}
			for _, l := range m.GetLabel() {
				if l.GetName() == "collector" && strings.HasPrefix(l.GetValue(), "mmdf-") {
					failures = append(failures, l.GetValue())
				}
			}
		}
	}

	if len(failures) != 0 && collectors.FileExists(*output) {
		file, err := os.Open(*output)
		if err != nil {
			logger.Error("Error opening metrics file", "err", err)
			goto failure
		}
		parser := expfmt.NewTextParser(model.UTF8Validation)
		prevMfs, err := parser.TextToMetricFamilies(file)
		file.Close()
		if err != nil {
			logger.Error("Error parsing output metrics", "err", err)
			goto failure
		}
		keys := make([]string, 0, len(prevMfs))
		for k := range prevMfs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, n := range keys {
			mf := prevMfs[n]
			if !strings.HasPrefix(n, "gpfs_exporter") {
				newMfs = append(newMfs, mf)
			}
		}
	} else {
		newMfs = mfs
	}

	if err := writeMetrics(newMfs, logger); err != nil {
		return err
	}
	if len(failures) != 0 {
		return fmt.Errorf("Error with collection")
	}
	return nil

failure:
	if err := writeMetrics(mfs, logger); err != nil {
		return err
	}
	return err
}

func main() {
	promslogConfig := &promslog.Config{}
	flag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.Version(version.Print("gpfs_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promslog.New(promslogConfig)

	fileLock := flock.New(*lockFile)
	locked, err := fileLock.TryLock()
	if err != nil {
		logger.Error("Unable to obtain lock on lock file", "lockfile", *lockFile)
		os.Exit(1)
	}
	if !locked {
		logger.Error(fmt.Sprintf("Lock file %s is locked", *lockFile))
		os.Exit(1)
	}
	err = collect(logger)
	if err != nil {
		os.Exit(1)
	}
	_ = fileLock.Unlock()
}
