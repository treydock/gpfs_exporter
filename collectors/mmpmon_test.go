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
	mmpmonStdout = `
_fs_io_s_ _n_ 10.22.0.106 _nn_ ib-pitzer-rw02.ten _rc_ 0 _t_ 1579358234 _tu_ 53212 _cl_ gpfs.domain _fs_ scratch _d_ 48 _br_ 205607400434 _bw_ 74839282351 _oc_ 2377656 _cc_ 2201576 _rdc_ 59420404 _wc_ 18874626 _dir_ 40971 _iu_ 544768
_fs_io_s_ _n_ 10.22.0.106 _nn_ ib-pitzer-rw02.ten _rc_ 0 _t_ 1579358234 _tu_ 53212 _cl_ gpfs.domain _fs_ project _d_ 96 _br_ 0 _bw_ 0 _oc_ 513 _cc_ 513 _rdc_ 0 _wc_ 0 _dir_ 0 _iu_ 169
`
)

func TestMmpmon(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmpmon(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if out != mockedStdout {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmpmonError(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmpmon(ctx)
	if err == nil {
		t.Errorf("Expected error")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmpmonTimeout(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
	defer cancel()
	out, err := mmpmon(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestParsePerf(t *testing.T) {
	perfs := mmpmon_parse(mmpmonStdout, log.NewNopLogger())
	if len(perfs) != 2 {
		t.Errorf("Expected 2 perfs returned, got %d", len(perfs))
		return
	}
	if val := perfs[0].FS; val != "scratch" {
		t.Errorf("Unexpected FS got %s", val)
	}
	if val := perfs[1].FS; val != "project" {
		t.Errorf("Unexpected FS got %s", val)
	}
	if val := perfs[0].NodeName; val != "ib-pitzer-rw02.ten" {
		t.Errorf("Unexpected NodeName got %s", val)
	}
	if val := perfs[1].NodeName; val != "ib-pitzer-rw02.ten" {
		t.Errorf("Unexpected NodeName got %s", val)
	}
	if val := perfs[0].ReadBytes; val != 205607400434 {
		t.Errorf("Unexpected ReadBytes got %d", val)
	}
}

func TestMmpmonCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	MmpmonExec = func(ctx context.Context) (string, error) {
		return mmpmonStdout, nil
	}
	expected := `
		# HELP gpfs_perf_info GPFS client information
		# TYPE gpfs_perf_info gauge
		gpfs_perf_info{fs="project",nodename="ib-pitzer-rw02.ten"} 1
		gpfs_perf_info{fs="scratch",nodename="ib-pitzer-rw02.ten"} 1
		# HELP gpfs_perf_operations_total GPFS operationgs reported by mmpmon
		# TYPE gpfs_perf_operations_total counter
		gpfs_perf_operations_total{fs="project",operation="closes"} 513
		gpfs_perf_operations_total{fs="project",operation="inode_updates"} 169
		gpfs_perf_operations_total{fs="project",operation="opens"} 513
		gpfs_perf_operations_total{fs="project",operation="read_dir"} 0
		gpfs_perf_operations_total{fs="project",operation="reads"} 0
		gpfs_perf_operations_total{fs="project",operation="writes"} 0
		gpfs_perf_operations_total{fs="scratch",operation="closes"} 2201576
		gpfs_perf_operations_total{fs="scratch",operation="inode_updates"} 544768
		gpfs_perf_operations_total{fs="scratch",operation="opens"} 2377656
		gpfs_perf_operations_total{fs="scratch",operation="read_dir"} 40971
		gpfs_perf_operations_total{fs="scratch",operation="reads"} 59420404
		gpfs_perf_operations_total{fs="scratch",operation="writes"} 18874626
		# HELP gpfs_perf_read_bytes_total GPFS read bytes
		# TYPE gpfs_perf_read_bytes_total counter
		gpfs_perf_read_bytes_total{fs="project"} 0
		gpfs_perf_read_bytes_total{fs="scratch"} 2.05607400434e+11
		# HELP gpfs_perf_write_bytes_total GPFS write bytes
		# TYPE gpfs_perf_write_bytes_total counter
		gpfs_perf_write_bytes_total{fs="project"} 0
		gpfs_perf_write_bytes_total{fs="scratch"} 74839282351
	`
	collector := NewMmpmonCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 21 {
		t.Errorf("Unexpected collection count %d, expected 21", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_perf_info",
		"gpfs_perf_read_bytes_total", "gpfs_perf_write_bytes_total", "gpfs_perf_operations_total"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMMpmonCollectorError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	MmpmonExec = func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmpmon"} 1
	`
	collector := NewMmpmonCollector(log.NewNopLogger())
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

func TestMMpmonCollectorTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	MmpmonExec = func(ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmpmon"} 1
	`
	collector := NewMmpmonCollector(log.NewNopLogger())
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
