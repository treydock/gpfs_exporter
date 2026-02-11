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
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
)

var (
	configStdout = `
mmdiag:config:HEADER:version:reserved:reserved:name:value:changed:
mmdiag:config:0:1:::opensslLibName:/usr/lib64/libssl.so.10%3A/usr/lib64/libssl.so.6%3A/usr/lib64/libssl.so.0.9.8%3A/lib64/libssl.so.6%3Alibssl.so%3Alibss
l.so.0%3Alibssl.so.4%3A/lib64/libssl.so.1.0.0::
mmdiag:config:0:1:::pagepool:4294967296:static:
mmdiag:config:0:1:::pagepoolMaxPhysMemPct:75::
mmdiag:config:0:1:::parallelMetadataWrite:0::
`
)

func TestParseMmdiagConfig(t *testing.T) {
	var metric ConfigMetric
	configs = []string{"pagepool", "opensslLibName"}
	parse_mmdiag_config(configStdout, &metric, promslog.NewNopLogger())
	if val := metric.PagePool; val != 4294967296 {
		t.Errorf("Unexpected page pool value %v", val)
	}
}

func TestConfigCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	MmdiagExec = func(arg string, ctx context.Context) (string, error) {
		return configStdout, nil
	}
	expected := `
		# HELP gpfs_config_page_pool_bytes GPFS configured page pool size
		# TYPE gpfs_config_page_pool_bytes gauge
		gpfs_config_page_pool_bytes 4294967296
	`
	collector := NewConfigCollector(promslog.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 4 {
		t.Errorf("Unexpected collection count %d, expected 4", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected), "gpfs_config_page_pool_bytes"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestConfigCollectorError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	MmdiagExec = func(arg string, ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="config"} 1
	`
	collector := NewConfigCollector(promslog.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 3 {
		t.Errorf("Unexpected collection count %d, expected 3", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected), "gpfs_exporter_collect_error"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestConfigCollectorTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	MmdiagExec = func(arg string, ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="config"} 1
	`
	collector := NewConfigCollector(promslog.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 3 {
		t.Errorf("Unexpected collection count %d, expected 3", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected), "gpfs_exporter_collect_timeout"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}
