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
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

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

func collect(logger log.Logger) error {
	tmp, err := ioutil.TempFile(filepath.Dir(*output), filepath.Base(*output))
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create temp file", "err", err)
		return err
	}
	defer os.Remove(tmp.Name())
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewMmdfCollector(logger))
	err = prometheus.WriteToTextfile(tmp.Name(), registry)
	if err != nil {
		level.Error(logger).Log("msg", "Error writing Prometheus metrics to file", "err", err)
		return err
	}
	content, err := ioutil.ReadFile(tmp.Name())
	if err != nil {
		level.Error(logger).Log("msg", "Error reading temp file", "err", err)
		return err
	}
	errorRE := regexp.MustCompile(`gpfs_exporter_collect_error{collector="mmdf-(.+)"} 1`)
	if errorRE.Match(content) {
		level.Error(logger).Log("msg", "Error detected with scrape")
		if collectors.FileExists(*output) {
			newTmp, err := ioutil.TempFile(filepath.Dir(*output), filepath.Base(*output))
			if err != nil {
				level.Error(logger).Log("msg", "Unable to create new temp file", "err", err)
				return err
			}
			defer os.Remove(newTmp.Name())
			oldContent, err := ioutil.ReadFile(*output)
			if err != nil {
				level.Error(logger).Log("msg", "Unable to read previous metrics", "err", err)
				return err
			}
			match := errorRE.FindStringSubmatch(string(content))
			newContent := string(oldContent)
			for _, m := range match {
				level.Debug(logger).Log("msg", "Update error metric for failed mmdf", "match", m)
				errorRE = regexp.MustCompile(fmt.Sprintf("gpfs_exporter_collect_error{collector=\"mmdf-(%s)\"} 0", m))
				newContent = errorRE.ReplaceAllString(newContent, "gpfs_exporter_collect_error{collector=\"mmdf-$1\"} 1")
			}
			level.Debug(logger).Log("msg", "Write new content", "content", newContent)
			if _, err = newTmp.Write([]byte(newContent)); err != nil {
				level.Error(logger).Log("msg", "Error writing to temp file", "err", err)
				return err
			}
			os.Rename(newTmp.Name(), *output)
		} else {
			os.Rename(tmp.Name(), *output)
		}
		return fmt.Errorf("Error detected with scrape")
	}
	level.Debug(logger).Log("msg", "Renaming temp file to output", "temp", tmp.Name(), "output", *output)
	os.Rename(tmp.Name(), *output)
	return nil
}

func main() {
	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("gpfs_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promlog.New(promlogConfig)

	fileLock := flock.New(*lockFile)
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
	err = collect(logger)
	if err != nil {
		os.Exit(1)
	}
	_ = fileLock.Unlock()
}
