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
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	mmhealthStdout = `
mmhealth:Event:HEADER:version:reserved:reserved:node:component:entityname:entitytype:event:arguments:activesince:identifier:ishidden:
mmhealth:State:HEADER:version:reserved:reserved:node:component:entityname:entitytype:status:laststatuschange:
mmhealth:State:0:1:::ib-haswell1.example.com:NODE:ib-haswell1.example.com:NODE:TIPS:2020-01-27 09%3A35%3A21.859186 EST:
mmhealth:State:0:1:::ib-haswell1.example.com:GPFS:ib-haswell1.example.com:NODE:TIPS:2020-01-27 09%3A35%3A21.791895 EST:
mmhealth:Event:0:1:::ib-haswell1.example.com:GPFS:ib-haswell1.example.com:NODE:gpfs_pagepool_small::2020-01-07 16%3A47%3A43.892296 EST::no:
mmhealth:State:0:1:::ib-haswell1.example.com:NETWORK:ib-haswell1.example.com:NODE:HEALTHY:2020-01-07 17%3A02%3A40.131272 EST:
mmhealth:State:0:1:::ib-haswell1.example.com:NETWORK:ib0:NIC:HEALTHY:2020-01-07 16%3A47%3A39.397852 EST:
mmhealth:State:0:1:::ib-haswell1.example.com:NETWORK:mlx5_0/1:IB_RDMA:HEALTHY:2020-01-07 17%3A02%3A40.205075 EST:
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
	metrics := mmhealth_parse(mmhealthStdout, log.NewNopLogger())
	if len(metrics) != 9 {
		t.Errorf("Expected 8 metrics returned, got %d", len(metrics))
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
}

func TestMmhealthCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	mmhealthExec = func(ctx context.Context) (string, error) {
		return mmhealthStdout, nil
	}
	expected := `
		# HELP gpfs_health_status GPFS health status
		# TYPE gpfs_health_status gauge
		gpfs_health_status{component="FILESYSTEM",entityname="ib-haswell1.example.com",entitytype="NODE",status="HEALTHY"} 1
		gpfs_health_status{component="FILESYSTEM",entityname="project",entitytype="FILESYSTEM",status="HEALTHY"} 1
		gpfs_health_status{component="FILESYSTEM",entityname="scratch",entitytype="FILESYSTEM",status="HEALTHY"} 1
		gpfs_health_status{component="FILESYSTEM",entityname="ess",entitytype="FILESYSTEM",status="HEALTHY"} 1
		gpfs_health_status{component="GPFS",entityname="ib-haswell1.example.com",entitytype="NODE",status="TIPS"} 1
		gpfs_health_status{component="NETWORK",entityname="ib-haswell1.example.com",entitytype="NODE",status="HEALTHY"} 1
		gpfs_health_status{component="NETWORK",entityname="ib0",entitytype="NIC",status="HEALTHY"} 1
		gpfs_health_status{component="NETWORK",entityname="mlx5_0/1",entitytype="IB_RDMA",status="HEALTHY"} 1
		gpfs_health_status{component="NODE",entityname="ib-haswell1.example.com",entitytype="NODE",status="TIPS"} 1
	`
	collector := NewMmhealthCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 12 {
		t.Errorf("Unexpected collection count %d, expected 12", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected), "gpfs_health_status"); err != nil {
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
