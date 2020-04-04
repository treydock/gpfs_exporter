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
	mmdfStdout = `
mmdf:nsd:HEADER:version:reserved:reserved:nsdName:storagePool:diskSize:failureGroup:metadata:data:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:diskAvailableForAlloc:
mmdf:poolTotal:HEADER:version:reserved:reserved:poolName:poolSize:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:maxDiskSize:
mmdf:data:HEADER:version:reserved:reserved:totalData:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:
mmdf:metadata:HEADER:version:reserved:reserved:totalMetadata:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:
mmdf:fsTotal:HEADER:version:reserved:reserved:fsSize:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:
mmdf:inode:HEADER:version:reserved:reserved:usedInodes:freeInodes:allocatedInodes:maxInodes:
mmdf:nsd:0:1:::P_META_VD102:system:771751936:300:Yes:No:320274944:41:5005384:1::
mmdf:nsd:0:1:::P_DATA_VD02:data:46766489600:200:No:Yes:6092915712:13:154966272:0::
mmdf:poolTotal:0:1:::data:3647786188800:475190722560:13:12059515296:0:3860104580096:
mmdf:data:0:1:::3647786188800:475190722560:13:12059515296:0:
mmdf:metadata:0:1:::13891534848:6011299328:43:58139768:0:
mmdf:fsTotal:0:1:::3661677723648:481202021888:14:12117655064:0:
mmdf:inode:0:1:::430741822:484301506:915043328:1332164000:
`
)

func TestMmdf(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmdf("test", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if out != mockedStdout {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmdfError(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmdf("test", ctx)
	if err == nil {
		t.Errorf("Expected error")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmdfTimeout(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
	defer cancel()
	out, err := mmdf("test", ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestParseMmdf(t *testing.T) {
	dfmetrics, err := parse_mmdf(mmdfStdout, log.NewNopLogger())
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if dfmetrics.InodesFree != 484301506 {
		t.Errorf("Unexpected value for InodesFree, got %d", dfmetrics.InodesFree)
	}
	if dfmetrics.FSTotal != 3749557989015552 {
		t.Errorf("Unexpected value for FSTotal, got %d", dfmetrics.FSTotal)
	}
	if dfmetrics.FSFreePercent != 14 {
		t.Errorf("Unexpected value for FSFreePercent, got %d", dfmetrics.FSFreePercent)
	}
	if dfmetrics.MetadataTotal != 14224931684352 {
		t.Errorf("Unexpected value for MetadataTotal, got %d", dfmetrics.MetadataTotal)
	}
	if dfmetrics.MetadataFreePercent != 43 {
		t.Errorf("Unexpected value for MetadataFreePercent, got %d", dfmetrics.MetadataFreePercent)
	}
}

func TestMmdfCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "project"
	configFilesystems = &filesystems
	MmdfExec = func(fs string, ctx context.Context) (string, error) {
		return mmdfStdout, nil
	}
	expected := `
		# HELP gpfs_fs_free_bytes GPFS filesystem free size in bytes
		# TYPE gpfs_fs_free_bytes gauge
		gpfs_fs_free_bytes{fs="project"} 492750870413312
		# HELP gpfs_fs_free_percent GPFS filesystem free percent
		# TYPE gpfs_fs_free_percent gauge
		gpfs_fs_free_percent{fs="project"} 14
		# HELP gpfs_fs_inodes_allocated GPFS filesystem inodes allocated
		# TYPE gpfs_fs_inodes_allocated gauge
		gpfs_fs_inodes_allocated{fs="project"} 915043328
		# HELP gpfs_fs_inodes_free GPFS filesystem inodes free
		# TYPE gpfs_fs_inodes_free gauge
		gpfs_fs_inodes_free{fs="project"} 484301506
		# HELP gpfs_fs_inodes_total GPFS filesystem inodes total
		# TYPE gpfs_fs_inodes_total gauge
		gpfs_fs_inodes_total{fs="project"} 1332164000
		# HELP gpfs_fs_inodes_used GPFS filesystem inodes used
		# TYPE gpfs_fs_inodes_used gauge
		gpfs_fs_inodes_used{fs="project"} 430741822
		# HELP gpfs_fs_metadata_free_bytes GPFS metadata free size in bytes
		# TYPE gpfs_fs_metadata_free_bytes gauge
		gpfs_fs_metadata_free_bytes{fs="project"} 6155570511872
		# HELP gpfs_fs_metadata_free_percent GPFS metadata free percent
		# TYPE gpfs_fs_metadata_free_percent gauge
		gpfs_fs_metadata_free_percent{fs="project"} 43
		# HELP gpfs_fs_metadata_total_bytes GPFS total metadata size in bytes
		# TYPE gpfs_fs_metadata_total_bytes gauge
		gpfs_fs_metadata_total_bytes{fs="project"} 14224931684352
		# HELP gpfs_fs_total_bytes GPFS filesystem total size in bytes
		# TYPE gpfs_fs_total_bytes gauge
		gpfs_fs_total_bytes{fs="project"} 3749557989015552
	`
	collector := NewMmdfCollector(log.NewNopLogger(), false)
	gatherers := setupGatherer(collector)
	if val := testutil.CollectAndCount(collector); val != 14 {
		t.Errorf("Unexpected collection count %d, expected 14", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_fs_inodes_used", "gpfs_fs_inodes_free", "gpfs_fs_inodes_allocated", "gpfs_fs_inodes_total",
		"gpfs_fs_free_bytes", "gpfs_fs_free_percent", "gpfs_fs_total_bytes",
		"gpfs_fs_metadata_total_bytes", "gpfs_fs_metadata_free_bytes", "gpfs_fs_metadata_free_percent"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmdfCollectorError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "project"
	configFilesystems = &filesystems
	MmdfExec = func(fs string, ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmdf-project"} 1
	`
	collector := NewMmdfCollector(log.NewNopLogger(), false)
	gatherers := setupGatherer(collector)
	if val := testutil.CollectAndCount(collector); val != 3 {
		t.Errorf("Unexpected collection count %d, expected 3", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected), "gpfs_exporter_collect_error"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmdfCollectorTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "project"
	configFilesystems = &filesystems
	MmdfExec = func(fs string, ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmdf-project"} 1
	`
	collector := NewMmdfCollector(log.NewNopLogger(), false)
	gatherers := setupGatherer(collector)
	if val := testutil.CollectAndCount(collector); val != 3 {
		t.Errorf("Unexpected collection count %d, expected 3", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected), "gpfs_exporter_collect_timeout"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmdfCollectorCache(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "project"
	configFilesystems = &filesystems
	// build cache
	MmdfExec = func(fs string, ctx context.Context) (string, error) {
		return mmdfStdout, nil
	}
	collector := NewMmdfCollector(log.NewNopLogger(), true)
	gatherers := setupGatherer(collector)
	if val := testutil.CollectAndCount(collector); val != 14 {
		t.Errorf("Unexpected collection count %d, expected 14", val)
	}

	MmdfExec = func(fs string, ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
	# HELP gpfs_fs_free_bytes GPFS filesystem free size in bytes
	# TYPE gpfs_fs_free_bytes gauge
	gpfs_fs_free_bytes{fs="project"} 492750870413312
	# HELP gpfs_fs_free_percent GPFS filesystem free percent
	# TYPE gpfs_fs_free_percent gauge
	gpfs_fs_free_percent{fs="project"} 14
	# HELP gpfs_fs_inodes_allocated GPFS filesystem inodes allocated
	# TYPE gpfs_fs_inodes_allocated gauge
	gpfs_fs_inodes_allocated{fs="project"} 915043328
	# HELP gpfs_fs_inodes_free GPFS filesystem inodes free
	# TYPE gpfs_fs_inodes_free gauge
	gpfs_fs_inodes_free{fs="project"} 484301506
	# HELP gpfs_fs_inodes_total GPFS filesystem inodes total
	# TYPE gpfs_fs_inodes_total gauge
	gpfs_fs_inodes_total{fs="project"} 1332164000
	# HELP gpfs_fs_inodes_used GPFS filesystem inodes used
	# TYPE gpfs_fs_inodes_used gauge
	gpfs_fs_inodes_used{fs="project"} 430741822
	# HELP gpfs_fs_metadata_free_bytes GPFS metadata free size in bytes
	# TYPE gpfs_fs_metadata_free_bytes gauge
	gpfs_fs_metadata_free_bytes{fs="project"} 6155570511872
	# HELP gpfs_fs_metadata_free_percent GPFS metadata free percent
	# TYPE gpfs_fs_metadata_free_percent gauge
	gpfs_fs_metadata_free_percent{fs="project"} 43
	# HELP gpfs_fs_metadata_total_bytes GPFS total metadata size in bytes
	# TYPE gpfs_fs_metadata_total_bytes gauge
	gpfs_fs_metadata_total_bytes{fs="project"} 14224931684352
	# HELP gpfs_fs_total_bytes GPFS filesystem total size in bytes
	# TYPE gpfs_fs_total_bytes gauge
	gpfs_fs_total_bytes{fs="project"} 3749557989015552
	`
	errorMetrics := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmdf-project"} 1
	`
	if val := testutil.CollectAndCount(collector); val != 14 {
		t.Errorf("Unexpected collection count %d, expected 14", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected+errorMetrics),
		"gpfs_fs_inodes_used", "gpfs_fs_inodes_free", "gpfs_fs_inodes_allocated", "gpfs_fs_inodes_total",
		"gpfs_fs_free_bytes", "gpfs_fs_free_percent", "gpfs_fs_total_bytes",
		"gpfs_fs_metadata_total_bytes", "gpfs_fs_metadata_free_bytes", "gpfs_fs_metadata_free_percent", "gpfs_exporter_collect_error"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}

	timeoutMetrics := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmdf-project"} 1
	`
	MmdfExec = func(fs string, ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	if val := testutil.CollectAndCount(collector); val != 14 {
		t.Errorf("Unexpected collection count %d, expected 14", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected+timeoutMetrics),
		"gpfs_fs_inodes_used", "gpfs_fs_inodes_free", "gpfs_fs_inodes_allocated", "gpfs_fs_inodes_total",
		"gpfs_fs_free_bytes", "gpfs_fs_free_percent", "gpfs_fs_total_bytes",
		"gpfs_fs_metadata_total_bytes", "gpfs_fs_metadata_free_bytes", "gpfs_fs_metadata_free_percent", "gpfs_exporter_collect_timeout"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}

	mmdfCache = make(map[string]DFMetric)
	MmdfExec = func(fs string, ctx context.Context) (string, error) {
		return mmdfStdout, nil
	}
	if val := testutil.CollectAndCount(collector); val != 14 {
		t.Errorf("Unexpected collection count %d, expected 14", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_fs_inodes_used", "gpfs_fs_inodes_free", "gpfs_fs_inodes_allocated", "gpfs_fs_inodes_total",
		"gpfs_fs_free_bytes", "gpfs_fs_free_percent", "gpfs_fs_total_bytes",
		"gpfs_fs_metadata_total_bytes", "gpfs_fs_metadata_free_bytes", "gpfs_fs_metadata_free_percent"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
	if val, ok := mmdfCache["project"]; !ok {
		t.Errorf("Cache entry not found")
	} else if val.InodesFree != 484301506 {
		t.Errorf("Unexpected InodesFree value in cache")
	}
}
