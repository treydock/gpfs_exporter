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

func TestParseMmhealth(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = `
mmhealth:Event:HEADER:version:reserved:reserved:node:component:entityname:entitytype:event:arguments:activesince:identifier:ishidden:
mmhealth:State:HEADER:version:reserved:reserved:node:component:entityname:entitytype:status:laststatuschange:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:NODE:ib-cluster-rw02.example.com:NODE:HEALTHY:2020-01-10 10%3A32%3A17.613885 EST:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:GPFS:ib-cluster-rw02.example.com:NODE:HEALTHY:2020-01-10 10%3A32%3A17.590229 EST:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:NETWORK:ib-cluster-rw02.example.com:NODE:HEALTHY:2020-01-03 15%3A32%3A38.077722 EST:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:NETWORK:ib0:NIC:HEALTHY:2020-01-07 08%3A33%3A41.113905 EST:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:FILESYSTEM:ib-cluster-rw02.example.com:NODE:HEALTHY:2020-01-10 10%3A32%3A17.577151 EST:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:FILESYSTEM:project:FILESYSTEM:HEALTHY:2020-01-07 18%3A03%3A31.834689 EST:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:FILESYSTEM:scratch:FILESYSTEM:HEALTHY:2020-01-07 18%3A03%3A31.842569 EST:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:FILESYSTEM:ess:FILESYSTEM:HEALTHY:2020-01-14 10%3A37%3A33.657052 EST:
`
	defer func() { execCommand = exec.CommandContext }()
	metrics, err := mmhealth_parse(mockedStdout, log.NewNopLogger())
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if len(metrics) != 8 {
		t.Errorf("Expected 8 metrics returned, got %d", len(metrics))
		return
	}
	if val := metrics[0].Component; val != "NODE" {
		t.Errorf("Unexpected Component got %s", val)
	}
	if val := metrics[0].EntityName; val != "ib-cluster-rw02.example.com" {
		t.Errorf("Unexpected EntityName got %s", val)
	}
	if val := metrics[0].EntityType; val != "NODE" {
		t.Errorf("Unexpected EntityType got %s", val)
	}
	if val := metrics[0].Status; val != "HEALTHY" {
		t.Errorf("Unexpected Status got %s", val)
	}
}

func TestParseMmhealthTips(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = `
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
	defer func() { execCommand = exec.CommandContext }()
	metrics, err := mmhealth_parse(mockedStdout, log.NewNopLogger())
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if len(metrics) != 9 {
		t.Errorf("Expected 9 metrics returned, got %d", len(metrics))
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

func TestParseMmhealthStatus(t *testing.T) {
	if val := parseMmhealthStatus("HEALTHY"); val != 1 {
		t.Errorf("Expected 1 for HEALTHY, got %v", val)
	}
	if val := parseMmhealthStatus("TIPS"); val != 0 {
		t.Errorf("Expected 0 for TIPS, got %v", val)
	}
	if val := parseMmhealthStatus("DEGRADED"); val != 0 {
		t.Errorf("Expected 0 for DEGRADED, got %v", val)
	}
}

func TestMmhealthCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{"--exporter.use-cache"}); err != nil {
		t.Fatal(err)
	}
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = `
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
	defer func() { execCommand = exec.CommandContext }()
	metadata := `
		# HELP gpfs_health_status GPFS health status, 1=healthy 0=not healthy
		# TYPE gpfs_health_status gauge`
	expected := `
		gpfs_health_status{component="FILESYSTEM",entityname="ib-haswell1.example.com",entitytype="NODE",status="HEALTHY"} 1
		gpfs_health_status{component="FILESYSTEM",entityname="project",entitytype="FILESYSTEM",status="HEALTHY"} 1
		gpfs_health_status{component="FILESYSTEM",entityname="scratch",entitytype="FILESYSTEM",status="HEALTHY"} 1
		gpfs_health_status{component="FILESYSTEM",entityname="ess",entitytype="FILESYSTEM",status="HEALTHY"} 1
		gpfs_health_status{component="GPFS",entityname="ib-haswell1.example.com",entitytype="NODE",status="TIPS"} 0
		gpfs_health_status{component="NETWORK",entityname="ib-haswell1.example.com",entitytype="NODE",status="HEALTHY"} 1
		gpfs_health_status{component="NETWORK",entityname="ib0",entitytype="NIC",status="HEALTHY"} 1
		gpfs_health_status{component="NETWORK",entityname="mlx5_0/1",entitytype="IB_RDMA",status="HEALTHY"} 1
		gpfs_health_status{component="NODE",entityname="ib-haswell1.example.com",entitytype="NODE",status="TIPS"} 0
	`
	collector := NewMmhealthCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val := testutil.CollectAndCount(collector); val != 12 {
		t.Errorf("Unexpected collection count %d, expected 12", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(metadata+expected), "gpfs_health_status"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}
