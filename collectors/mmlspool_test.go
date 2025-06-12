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
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var (
	mmlspoolStdout = `
Storage pools in file system at '/fs/scratch':
Name                    Id   BlkSize Data Meta Total Data in (KB)   Free Data in (KB)   Total Meta in (KB)    Free Meta in (KB)
system                   0      8 MB  yes  yes   783308292096   143079555072 ( 18%)   783308292096   205366984704 ( 26%)
data                 65537      8 MB  yes   no  3064453922816   529111449600 ( 17%)              0              0 (  0%)
`
	mmlspoolError = `
Storage pools in file system at '/fs/scratch':
Name                    Id   BlkSize Data Meta Total Data in (KB)   Free Data in (KB)   Total Meta in (KB)    Free Meta in (KB)
system                   0      8 MB  yes  yes   783308292096   foo ( 18%)   783308292096   205366984704 ( 26%)
data                 65537      8 MB  yes   no  3064453922816   529111449600 ( 17%)              0              0 (  0%)
`
)

func TestMmlspool(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmlspool("test", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if out != mockedStdout {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmlspoolError(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmlspool("test", ctx)
	if err == nil {
		t.Errorf("Expected error")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmlspoolTimeout(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
	defer cancel()
	out, err := mmlspool("test", ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestParseMmlspool(t *testing.T) {
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)
	//pools, err := parse_mmlspool("scratch", mmlspoolStdout, log.NewNopLogger())
	pools, err := parse_mmlspool("scratch", mmlspoolStdout, logger)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if len(pools) != 2 {
		t.Errorf("Unexpected number of pools: %v", len(pools))
	}
	expectedPools := []PoolMetric{}
	system := PoolMetric{
		FS:              "scratch",
		PoolName:        "system",
		Meta:            true,
		PoolTotal:       783308292096,
		PoolFree:        146513464393728,
		PoolFreePercent: 18,
		MetaTotal:       802107691106304,
		MetaFree:        210295792336896,
		MetaFreePercent: 26,
	}
	expectedPools = append(expectedPools, system)
	data := PoolMetric{
		FS:              "scratch",
		PoolName:        "data",
		Meta:            false,
		PoolTotal:       3138000816963584,
		PoolFree:        541810124390400,
		PoolFreePercent: 17,
		MetaTotal:       0,
		MetaFree:        0,
		MetaFreePercent: 0,
	}
	expectedPools = append(expectedPools, data)
	for i, pool := range expectedPools {
		expected := expectedPools[i]
		if pool.FS != expected.FS {
			t.Errorf("Unexpected value for %d FS, got %v", i, pool.FS)
		}
		if pool.PoolName != expected.PoolName {
			t.Errorf("Unexpected value for %d PoolName, got %v", i, pool.PoolName)
		}
		if pool.Meta != expected.Meta {
			t.Errorf("Unexpected value for %d Meta, got %v", i, pool.Meta)
		}
		if pool.PoolTotal != expected.PoolTotal {
			t.Errorf("Unexpected value for %d PoolTotal, got %v", i, pool.PoolTotal)
		}
		if pool.PoolFree != expected.PoolFree {
			t.Errorf("Unexpected value for %d PoolFree, got %v", i, pool.PoolFree)
		}
		if pool.PoolFreePercent != expected.PoolFreePercent {
			t.Errorf("Unexpected value for %d PoolFreePercent, got %v", i, pool.PoolFreePercent)
		}
		if pool.MetaTotal != expected.MetaTotal {
			t.Errorf("Unexpected value for %d MetaTotal, got %v", i, pool.MetaTotal)
		}
		if pool.MetaFree != expected.MetaFree {
			t.Errorf("Unexpected value for %d MetaFree, got %v", i, pool.MetaFree)
		}
		if pool.MetaFreePercent != expected.MetaFreePercent {
			t.Errorf("Unexpected value for %d MetaFreePercent, got %v", i, pool.MetaFreePercent)
		}
	}

	pools, err = parse_mmlspool("scratch", mmlspoolError, log.NewNopLogger())
	if err == nil {
		t.Errorf("Expected an error")
	}
	if pools != nil {
		t.Errorf("Expected nil pools")
	}
}

func TestMmlspoolCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "scratch"
	poolFilesystems = &filesystems
	MmlspoolExec = func(fs string, ctx context.Context) (string, error) {
		return mmlspoolStdout, nil
	}
	expected := `
		# HELP gpfs_pool_free_bytes GPFS pool free size in bytes
		# TYPE gpfs_pool_free_bytes gauge
		gpfs_pool_free_bytes{fs="scratch",pool="data"} 541810124390400
		gpfs_pool_free_bytes{fs="scratch",pool="system"} 146513464393728
		# HELP gpfs_pool_free_percent GPFS pool free percent
		# TYPE gpfs_pool_free_percent gauge
		gpfs_pool_free_percent{fs="scratch",pool="data"} 17
		gpfs_pool_free_percent{fs="scratch",pool="system"} 18
		# HELP gpfs_pool_metadata_free_bytes GPFS pool free metadata in bytes
		# TYPE gpfs_pool_metadata_free_bytes gauge
		gpfs_pool_metadata_free_bytes{fs="scratch",pool="system"} 210295792336896
		# HELP gpfs_pool_metadata_free_percent GPFS pool free percent
		# TYPE gpfs_pool_metadata_free_percent gauge
		gpfs_pool_metadata_free_percent{fs="scratch",pool="system"} 26
		# HELP gpfs_pool_metadata_total_bytes GPFS pool total metadata in bytes
		# TYPE gpfs_pool_metadata_total_bytes gauge
		gpfs_pool_metadata_total_bytes{fs="scratch",pool="system"} 802107691106304
		# HELP gpfs_pool_total_bytes GPFS pool total size in bytes
		# TYPE gpfs_pool_total_bytes gauge
		gpfs_pool_total_bytes{fs="scratch",pool="data"} 3138000816963584
		gpfs_pool_total_bytes{fs="scratch",pool="system"} 802107691106304
	`
	collector := NewMmlspoolCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 13 {
		t.Errorf("Unexpected collection count %d, expected 13", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_pool_free_bytes", "gpfs_pool_free_percent",
		"gpfs_pool_metadata_free_bytes", "gpfs_pool_metadata_free_percent",
		"gpfs_pool_metadata_total_bytes", "gpfs_pool_total_bytes"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmlspoolCollectorMmlsfs(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := ""
	poolFilesystems = &filesystems
	MmlspoolExec = func(fs string, ctx context.Context) (string, error) {
		return mmlspoolStdout, nil
	}
	mmlsfsStdout = `
fs::HEADER:version:reserved:reserved:deviceName:fieldName:data:remarks:
mmlsfs::0:1:::scratch:defaultMountPoint:%2Ffs%2Fscratch::
`
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return mmlsfsStdout, nil
	}
	expected := `
		# HELP gpfs_pool_free_bytes GPFS pool free size in bytes
		# TYPE gpfs_pool_free_bytes gauge
		gpfs_pool_free_bytes{fs="scratch",pool="data"} 541810124390400
		gpfs_pool_free_bytes{fs="scratch",pool="system"} 146513464393728
		# HELP gpfs_pool_free_percent GPFS pool free percent
		# TYPE gpfs_pool_free_percent gauge
		gpfs_pool_free_percent{fs="scratch",pool="data"} 17
		gpfs_pool_free_percent{fs="scratch",pool="system"} 18
		# HELP gpfs_pool_metadata_free_bytes GPFS pool free metadata in bytes
		# TYPE gpfs_pool_metadata_free_bytes gauge
		gpfs_pool_metadata_free_bytes{fs="scratch",pool="system"} 210295792336896
		# HELP gpfs_pool_metadata_free_percent GPFS pool free percent
		# TYPE gpfs_pool_metadata_free_percent gauge
		gpfs_pool_metadata_free_percent{fs="scratch",pool="system"} 26
		# HELP gpfs_pool_metadata_total_bytes GPFS pool total metadata in bytes
		# TYPE gpfs_pool_metadata_total_bytes gauge
		gpfs_pool_metadata_total_bytes{fs="scratch",pool="system"} 802107691106304
		# HELP gpfs_pool_total_bytes GPFS pool total size in bytes
		# TYPE gpfs_pool_total_bytes gauge
		gpfs_pool_total_bytes{fs="scratch",pool="data"} 3138000816963584
		gpfs_pool_total_bytes{fs="scratch",pool="system"} 802107691106304
	`
	collector := NewMmlspoolCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 15 {
		t.Errorf("Unexpected collection count %d, expected 15", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_pool_free_bytes", "gpfs_pool_free_percent",
		"gpfs_pool_metadata_free_bytes", "gpfs_pool_metadata_free_percent",
		"gpfs_pool_metadata_total_bytes", "gpfs_pool_total_bytes"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmlspoolCollectorError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "scratch"
	poolFilesystems = &filesystems
	MmlspoolExec = func(fs string, ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmlspool-scratch"} 1
	`
	collector := NewMmlspoolCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 4 {
		t.Errorf("Unexpected collection count %d, expected 4", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected), "gpfs_exporter_collect_error"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmlspoolCollectorTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "scratch"
	poolFilesystems = &filesystems
	MmlspoolExec = func(fs string, ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmlspool-scratch"} 1
	`
	collector := NewMmlspoolCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 4 {
		t.Errorf("Unexpected collection count %d, expected 4", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected), "gpfs_exporter_collect_timeout"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmlspoolCollectorMmlsfsError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := ""
	poolFilesystems = &filesystems
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmlspool-mmlsfs"} 1
	`
	collector := NewMmlspoolCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 2 {
		t.Errorf("Unexpected collection count %d, expected 2", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected), "gpfs_exporter_collect_error"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmlspoolCollectorMmlsfsTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := ""
	poolFilesystems = &filesystems
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmlspool-mmlsfs"} 1
	`
	collector := NewMmlspoolCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 2 {
		t.Errorf("Unexpected collection count %d, expected 2", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected), "gpfs_exporter_collect_timeout"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}
