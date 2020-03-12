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
	"os/exec"
	"strings"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"gopkg.in/alecthomas/kingpin.v2"
)

func TestParseMmcesStateShow(t *testing.T) {
	execCommand = fakeExecCommand
	mockedStdout = `
mmcesstate::HEADER:version:reserved:reserved:NODE:AUTH:BLOCK:NETWORK:AUTH_OBJ:NFS:OBJ:SMB:CES:
mmcesstate::0:1:::ib-protocol01.domain:HEALTHY:DISABLED:HEALTHY:DISABLED:HEALTHY:DISABLED:HEALTHY:HEALTHY:
`
	defer func() { execCommand = exec.CommandContext }()
	metrics, err := mmces_state_show_parse(mockedStdout)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if len(metrics) != 8 {
		t.Errorf("Expected 8 metrics returned, got %d", len(metrics))
		return
	}
	if val := metrics[0].Service; val != "AUTH" {
		t.Errorf("Unexpected Service got %s", val)
	}
	if val := metrics[0].State; val != "HEALTHY" {
		t.Errorf("Unexpected State got %s", val)
	}
}

func TestParseMmcesState(t *testing.T) {
	if val := parseMmcesState("HEALTHY"); val != 1 {
		t.Errorf("Expected 1 for HEALTHY, got %v", val)
	}
	if val := parseMmcesState("DISABLED"); val != 0 {
		t.Errorf("Expected 0 for DISABLED, got %v", val)
	}
	if val := parseMmcesState("DEGRADED"); val != 0 {
		t.Errorf("Expected 0 for DEGRADED, got %v", val)
	}
}

func TestMMcesCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{"--collector.mmces.nodename=ib-protocol01.domain --exporter.use-cache"}); err != nil {
		t.Fatal(err)
	}
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = `
mmcesstate::HEADER:version:reserved:reserved:NODE:AUTH:BLOCK:NETWORK:AUTH_OBJ:NFS:OBJ:SMB:CES:
mmcesstate::0:1:::ib-protocol01.domain:HEALTHY:DISABLED:HEALTHY:DISABLED:HEALTHY:DISABLED:HEALTHY:HEALTHY:
`
	defer func() { execCommand = exec.CommandContext }()
	metadata := `
			# HELP gpfs_ces_state GPFS CES health status, 1=healthy 0=not healthy
			# TYPE gpfs_ces_state gauge`
	expected := `
		gpfs_ces_state{service="AUTH",state="HEALTHY"} 1
		gpfs_ces_state{service="AUTH_OBJ",state="DISABLED"} 0
		gpfs_ces_state{service="BLOCK",state="DISABLED"} 0
		gpfs_ces_state{service="CES",state="HEALTHY"} 1
		gpfs_ces_state{service="NETWORK",state="HEALTHY"} 1
		gpfs_ces_state{service="NFS",state="HEALTHY"} 1
		gpfs_ces_state{service="OBJ",state="DISABLED"} 0
		gpfs_ces_state{service="SMB",state="HEALTHY"} 1
	`
	collector := NewMmcesCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val := testutil.CollectAndCount(collector); val != 11 {
		t.Errorf("Unexpected collection count %d, expected 11", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(metadata+expected), "gpfs_ces_state"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMMcesCollectorError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{"--collector.mmces.nodename=ib-protocol01.domain --exporter.use-cache"}); err != nil {
		t.Fatal(err)
	}
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	defer func() { execCommand = exec.CommandContext }()
	metadata := `
			# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
			# TYPE gpfs_exporter_collect_error gauge`
	expected := `
		gpfs_exporter_collect_error{collector="mmces"} 1
	`
	collector := NewMmcesCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val := testutil.CollectAndCount(collector); val != 3 {
		t.Errorf("Unexpected collection count %d, expected 3", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(metadata+expected), "gpfs_exporter_collect_error"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}
