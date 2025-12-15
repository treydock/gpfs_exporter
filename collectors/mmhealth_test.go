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
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var (
	mmhealthStdout = `
mmhealth:Event:HEADER:version:reserved:reserved:node:component:entityname:entitytype:event:arguments:activesince:identifier:ishidden:
mmhealth:State:HEADER:version:reserved:reserved:node:component:entityname:entitytype:status:laststatuschange:
mmhealth:State:0:1:::ib-haswell1.example.com:NODE:ib-haswell1.example.com:NODE:TIPS:2020-01-27 09%3A35%3A21.859186 EST:
mmhealth:State:0:1:::ib-haswell1.example.com:GPFS:ib-haswell1.example.com:NODE:TIPS:2020-01-27 09%3A35%3A21.791895 EST:
mmhealth:Event:0:1:::ib-haswell1.example.com:GPFS:ib-haswell1.example.com:NODE:gpfs_pagepool_small::2020-01-07 16%3A47%3A43.892296 EST::no:
mmhealth:Event:0:1:::ib-haswell1.example.com:GPFS:ib-haswell1.example.com:NODE:cluster_connections_down:10.22.51.57,1,1:2023-07-05 16%3A33%3A11.224969 EDT:10.22.51.57:no:Connection to cluster node 10.22.51.57 has all 1 connection(s) down. (Maximum 1).:STATE_CHANGE:WARNING:
mmhealth:Event:0:1:::ib-haswell1.example.com:GPFS:ib-haswell1.example.com:NODE:cluster_connections_down:10.22.95.17,1,1:2023-07-05 09%3A56%3A59.071165 EDT:10.22.95.17:no:Connection to cluster node 10.22.95.17 has all 1 connection(s) down. (Maximum 1).:STATE_CHANGE:WARNING:
mmhealth:State:0:1:::ib-haswell1.example.com:NETWORK:ib-haswell1.example.com:NODE:HEALTHY:2020-01-07 17%3A02%3A40.131272 EST:
mmhealth:State:0:1:::ib-haswell1.example.com:NETWORK:ib0:NIC:HEALTHY:2020-01-07 16%3A47%3A39.397852 EST:
mmhealth:State:0:1:::ib-haswell1.example.com:NETWORK:mlx5_0/1:IB_RDMA:FOO:2020-01-07 17%3A02%3A40.205075 EST:
mmhealth:State:0:1:::ib-haswell1.example.com:FILESYSTEM:ib-haswell1.example.com:NODE:HEALTHY:2020-01-27 09%3A35%3A21.499264 EST:
mmhealth:State:0:1:::ib-haswell1.example.com:FILESYSTEM:project:FILESYSTEM:HEALTHY:2020-01-27 09%3A35%3A21.573978 EST:
mmhealth:State:0:1:::ib-haswell1.example.com:FILESYSTEM:scratch:FILESYSTEM:HEALTHY:2020-01-27 09%3A35%3A21.657798 EST:
mmhealth:State:0:1:::ib-haswell1.example.com:FILESYSTEM:ess:FILESYSTEM:HEALTHY:2020-01-27 09%3A35%3A21.716417 EST:
`
)

func TestMmhealth(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmhealth(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if out != mockedStdout {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmhealthError(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmhealth(ctx)
	if err == nil {
		t.Errorf("Expected error")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmhealthTimeout(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
	defer cancel()
	out, err := mmhealth(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestParseMmhealth(t *testing.T) {
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)
	metrics := mmhealth_parse(mmhealthStdout, logger)
	if len(metrics) != 11 {
		t.Errorf("Expected 11 metrics returned, got %d", len(metrics))
		return
	}
	if val := metrics[0].Component; val != "NODE" {
		t.Errorf("Unexpected Component got %s", val)
	}
	if val := metrics[0].EntityName; val != "ib-haswell1.example.com" {
		t.Errorf("Unexpected EntityName got %s", val)
	}
	if val := metrics[0].EntityType; val != "NODE" {
		t.Errorf("Unexpected EntityType got %s", val)
	}
	if val := metrics[0].Status; val != "TIPS" {
		t.Errorf("Unexpected Status got %s", val)
	}
	if val := metrics[2].Type; val != "Event" {
		t.Errorf("Unexpected Type got %s", val)
	}
	if val := metrics[2].Event; val != "gpfs_pagepool_small" {
		t.Errorf("Unexpected Event got %s", val)
	}
}

func TestParseMmhealthIgnores(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	noignore := "^$"
	empty := ""
	ignore := "FILESYSTEM"
	eventIgnore := "^(gpfs_pagepool_small)$"
	mmhealthIgnoredComponent = &ignore
	mmhealthIgnoredEntityName = &noignore
	mmhealthIgnoredEntityType = &noignore
	mmhealthIgnoredEvent = &eventIgnore
	metrics := mmhealth_parse(mmhealthStdout, log.NewNopLogger())
	if len(metrics) != 6 {
		t.Errorf("Expected 6 metrics returned, got %d", len(metrics))
		return
	}
	ignore = "ess"
	mmhealthIgnoredComponent = &noignore
	mmhealthIgnoredEntityName = &ignore
	mmhealthIgnoredEntityType = &noignore
	mmhealthIgnoredEvent = &empty
	metrics = mmhealth_parse(mmhealthStdout, log.NewNopLogger())
	if len(metrics) != 10 {
		t.Errorf("Expected 10 metrics returned, got %d", len(metrics))
		return
	}
	ignore = "FILESYSTEM"
	mmhealthIgnoredComponent = &noignore
	mmhealthIgnoredEntityName = &noignore
	mmhealthIgnoredEntityType = &ignore
	mmhealthIgnoredEvent = &empty
	metrics = mmhealth_parse(mmhealthStdout, log.NewNopLogger())
	if len(metrics) != 8 {
		t.Errorf("Expected 8 metrics returned, got %d", len(metrics))
		return
	}
}

func TestMmhealthCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	mmhealthExec = func(ctx context.Context) (string, error) {
		return mmhealthStdout, nil
	}
	ignore := "^$"
	mmhealthIgnoredComponent = &ignore
	mmhealthIgnoredEntityName = &ignore
	mmhealthIgnoredEntityType = &ignore
	expected := `
		# HELP gpfs_health_event GPFS health event
		# TYPE gpfs_health_event gauge
		gpfs_health_event{component="GPFS",entityname="ib-haswell1.example.com",entitytype="NODE",event="cluster_connections_down"} 1
		gpfs_health_event{component="GPFS",entityname="ib-haswell1.example.com",entitytype="NODE",event="gpfs_pagepool_small"} 1
		# HELP gpfs_health_status GPFS health status
		# TYPE gpfs_health_status gauge
		gpfs_health_status{component="FILESYSTEM",entityname="ib-haswell1.example.com",entitytype="NODE",status="CHECKING"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ib-haswell1.example.com",entitytype="NODE",status="DEGRADED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ib-haswell1.example.com",entitytype="NODE",status="DEPEND"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ib-haswell1.example.com",entitytype="NODE",status="DISABLED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ib-haswell1.example.com",entitytype="NODE",status="FAILED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ib-haswell1.example.com",entitytype="NODE",status="HEALTHY"} 1
		gpfs_health_status{component="FILESYSTEM",entityname="ib-haswell1.example.com",entitytype="NODE",status="STARTING"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ib-haswell1.example.com",entitytype="NODE",status="STOPPED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ib-haswell1.example.com",entitytype="NODE",status="SUSPENDED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ib-haswell1.example.com",entitytype="NODE",status="TIPS"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ib-haswell1.example.com",entitytype="NODE",status="UNKNOWN"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="project",entitytype="FILESYSTEM",status="CHECKING"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="project",entitytype="FILESYSTEM",status="DEGRADED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="project",entitytype="FILESYSTEM",status="DEPEND"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="project",entitytype="FILESYSTEM",status="DISABLED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="project",entitytype="FILESYSTEM",status="FAILED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="project",entitytype="FILESYSTEM",status="HEALTHY"} 1
		gpfs_health_status{component="FILESYSTEM",entityname="project",entitytype="FILESYSTEM",status="STARTING"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="project",entitytype="FILESYSTEM",status="STOPPED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="project",entitytype="FILESYSTEM",status="SUSPENDED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="project",entitytype="FILESYSTEM",status="TIPS"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="project",entitytype="FILESYSTEM",status="UNKNOWN"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="scratch",entitytype="FILESYSTEM",status="CHECKING"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="scratch",entitytype="FILESYSTEM",status="DEGRADED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="scratch",entitytype="FILESYSTEM",status="DEPEND"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="scratch",entitytype="FILESYSTEM",status="DISABLED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="scratch",entitytype="FILESYSTEM",status="FAILED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="scratch",entitytype="FILESYSTEM",status="HEALTHY"} 1
		gpfs_health_status{component="FILESYSTEM",entityname="scratch",entitytype="FILESYSTEM",status="STARTING"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="scratch",entitytype="FILESYSTEM",status="STOPPED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="scratch",entitytype="FILESYSTEM",status="SUSPENDED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="scratch",entitytype="FILESYSTEM",status="TIPS"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="scratch",entitytype="FILESYSTEM",status="UNKNOWN"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ess",entitytype="FILESYSTEM",status="CHECKING"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ess",entitytype="FILESYSTEM",status="DEGRADED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ess",entitytype="FILESYSTEM",status="DEPEND"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ess",entitytype="FILESYSTEM",status="DISABLED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ess",entitytype="FILESYSTEM",status="FAILED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ess",entitytype="FILESYSTEM",status="HEALTHY"} 1
		gpfs_health_status{component="FILESYSTEM",entityname="ess",entitytype="FILESYSTEM",status="STARTING"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ess",entitytype="FILESYSTEM",status="STOPPED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ess",entitytype="FILESYSTEM",status="SUSPENDED"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ess",entitytype="FILESYSTEM",status="TIPS"} 0
		gpfs_health_status{component="FILESYSTEM",entityname="ess",entitytype="FILESYSTEM",status="UNKNOWN"} 0
		gpfs_health_status{component="GPFS",entityname="ib-haswell1.example.com",entitytype="NODE",status="CHECKING"} 0
		gpfs_health_status{component="GPFS",entityname="ib-haswell1.example.com",entitytype="NODE",status="DEGRADED"} 0
		gpfs_health_status{component="GPFS",entityname="ib-haswell1.example.com",entitytype="NODE",status="DEPEND"} 0
		gpfs_health_status{component="GPFS",entityname="ib-haswell1.example.com",entitytype="NODE",status="DISABLED"} 0
		gpfs_health_status{component="GPFS",entityname="ib-haswell1.example.com",entitytype="NODE",status="FAILED"} 0
		gpfs_health_status{component="GPFS",entityname="ib-haswell1.example.com",entitytype="NODE",status="HEALTHY"} 0
		gpfs_health_status{component="GPFS",entityname="ib-haswell1.example.com",entitytype="NODE",status="STARTING"} 0
		gpfs_health_status{component="GPFS",entityname="ib-haswell1.example.com",entitytype="NODE",status="STOPPED"} 0
		gpfs_health_status{component="GPFS",entityname="ib-haswell1.example.com",entitytype="NODE",status="SUSPENDED"} 0
		gpfs_health_status{component="GPFS",entityname="ib-haswell1.example.com",entitytype="NODE",status="TIPS"} 1
		gpfs_health_status{component="GPFS",entityname="ib-haswell1.example.com",entitytype="NODE",status="UNKNOWN"} 0
		gpfs_health_status{component="NETWORK",entityname="ib-haswell1.example.com",entitytype="NODE",status="CHECKING"} 0
		gpfs_health_status{component="NETWORK",entityname="ib-haswell1.example.com",entitytype="NODE",status="DEGRADED"} 0
		gpfs_health_status{component="NETWORK",entityname="ib-haswell1.example.com",entitytype="NODE",status="DEPEND"} 0
		gpfs_health_status{component="NETWORK",entityname="ib-haswell1.example.com",entitytype="NODE",status="DISABLED"} 0
		gpfs_health_status{component="NETWORK",entityname="ib-haswell1.example.com",entitytype="NODE",status="FAILED"} 0
		gpfs_health_status{component="NETWORK",entityname="ib-haswell1.example.com",entitytype="NODE",status="HEALTHY"} 1
		gpfs_health_status{component="NETWORK",entityname="ib-haswell1.example.com",entitytype="NODE",status="STARTING"} 0
		gpfs_health_status{component="NETWORK",entityname="ib-haswell1.example.com",entitytype="NODE",status="STOPPED"} 0
		gpfs_health_status{component="NETWORK",entityname="ib-haswell1.example.com",entitytype="NODE",status="SUSPENDED"} 0
		gpfs_health_status{component="NETWORK",entityname="ib-haswell1.example.com",entitytype="NODE",status="TIPS"} 0
		gpfs_health_status{component="NETWORK",entityname="ib-haswell1.example.com",entitytype="NODE",status="UNKNOWN"} 0
		gpfs_health_status{component="NETWORK",entityname="ib0",entitytype="NIC",status="CHECKING"} 0
		gpfs_health_status{component="NETWORK",entityname="ib0",entitytype="NIC",status="DEGRADED"} 0
		gpfs_health_status{component="NETWORK",entityname="ib0",entitytype="NIC",status="DEPEND"} 0
		gpfs_health_status{component="NETWORK",entityname="ib0",entitytype="NIC",status="DISABLED"} 0
		gpfs_health_status{component="NETWORK",entityname="ib0",entitytype="NIC",status="FAILED"} 0
		gpfs_health_status{component="NETWORK",entityname="ib0",entitytype="NIC",status="HEALTHY"} 1
		gpfs_health_status{component="NETWORK",entityname="ib0",entitytype="NIC",status="STARTING"} 0
		gpfs_health_status{component="NETWORK",entityname="ib0",entitytype="NIC",status="STOPPED"} 0
		gpfs_health_status{component="NETWORK",entityname="ib0",entitytype="NIC",status="SUSPENDED"} 0
		gpfs_health_status{component="NETWORK",entityname="ib0",entitytype="NIC",status="TIPS"} 0
		gpfs_health_status{component="NETWORK",entityname="ib0",entitytype="NIC",status="UNKNOWN"} 0
		gpfs_health_status{component="NETWORK",entityname="mlx5_0/1",entitytype="IB_RDMA",status="CHECKING"} 0
		gpfs_health_status{component="NETWORK",entityname="mlx5_0/1",entitytype="IB_RDMA",status="DEGRADED"} 0
		gpfs_health_status{component="NETWORK",entityname="mlx5_0/1",entitytype="IB_RDMA",status="DEPEND"} 0
		gpfs_health_status{component="NETWORK",entityname="mlx5_0/1",entitytype="IB_RDMA",status="DISABLED"} 0
		gpfs_health_status{component="NETWORK",entityname="mlx5_0/1",entitytype="IB_RDMA",status="FAILED"} 0
		gpfs_health_status{component="NETWORK",entityname="mlx5_0/1",entitytype="IB_RDMA",status="HEALTHY"} 0
		gpfs_health_status{component="NETWORK",entityname="mlx5_0/1",entitytype="IB_RDMA",status="STARTING"} 0
		gpfs_health_status{component="NETWORK",entityname="mlx5_0/1",entitytype="IB_RDMA",status="STOPPED"} 0
		gpfs_health_status{component="NETWORK",entityname="mlx5_0/1",entitytype="IB_RDMA",status="SUSPENDED"} 0
		gpfs_health_status{component="NETWORK",entityname="mlx5_0/1",entitytype="IB_RDMA",status="TIPS"} 0
		gpfs_health_status{component="NETWORK",entityname="mlx5_0/1",entitytype="IB_RDMA",status="UNKNOWN"} 1
		gpfs_health_status{component="NODE",entityname="ib-haswell1.example.com",entitytype="NODE",status="CHECKING"} 0
		gpfs_health_status{component="NODE",entityname="ib-haswell1.example.com",entitytype="NODE",status="DEGRADED"} 0
		gpfs_health_status{component="NODE",entityname="ib-haswell1.example.com",entitytype="NODE",status="DEPEND"} 0
		gpfs_health_status{component="NODE",entityname="ib-haswell1.example.com",entitytype="NODE",status="DISABLED"} 0
		gpfs_health_status{component="NODE",entityname="ib-haswell1.example.com",entitytype="NODE",status="FAILED"} 0
		gpfs_health_status{component="NODE",entityname="ib-haswell1.example.com",entitytype="NODE",status="HEALTHY"} 0
		gpfs_health_status{component="NODE",entityname="ib-haswell1.example.com",entitytype="NODE",status="STARTING"} 0
		gpfs_health_status{component="NODE",entityname="ib-haswell1.example.com",entitytype="NODE",status="STOPPED"} 0
		gpfs_health_status{component="NODE",entityname="ib-haswell1.example.com",entitytype="NODE",status="SUSPENDED"} 0
		gpfs_health_status{component="NODE",entityname="ib-haswell1.example.com",entitytype="NODE",status="TIPS"} 1
		gpfs_health_status{component="NODE",entityname="ib-haswell1.example.com",entitytype="NODE",status="UNKNOWN"} 0
	`
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)
	collector := NewMmhealthCollector(logger)
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 104 {
		t.Errorf("Unexpected collection count %d, expected 104", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected), "gpfs_health_status", "gpfs_health_event"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMMhealthCollectorError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	mmhealthExec = func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmhealth"} 1
	`
	collector := NewMmhealthCollector(log.NewNopLogger())
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

func TestMMhealthCollectorTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	mmhealthExec = func(ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmhealth"} 1
	`
	collector := NewMmhealthCollector(log.NewNopLogger())
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
