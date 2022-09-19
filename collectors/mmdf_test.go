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

	"github.com/go-kit/log"
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
mmdf:poolTotal:0:1:::system:783308292096:380564840448:49:10024464464:1:1153081262080:
mmdf:data:0:1:::3647786188800:475190722560:13:12059515296:0:
mmdf:metadata:0:1:::13891534848:6011299328:43:58139768:0:
mmdf:poolTotal:0:1:::data:3064453922816:1342362296320:44:1999215152:0:10143773212672:
mmdf:fsTotal:0:1:::3661677723648:481202021888:14:12117655064:0:
mmdf:inode:0:1:::430741822:484301506:915043328:1332164000:
`
	mmdfStdoutMissingMetadata = `
mmdf:nsd:HEADER:version:reserved:reserved:nsdName:storagePool:diskSize:failureGroup:metadata:data:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:diskAvailableForAlloc:
mmdf:poolTotal:HEADER:version:reserved:reserved:poolName:poolSize:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:maxDiskSize:
mmdf:data:HEADER:version:reserved:reserved:totalData:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:
mmdf:metadata:HEADER:version:reserved:reserved:totalMetadata:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:
mmdf:fsTotal:HEADER:version:reserved:reserved:fsSize:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:
mmdf:inode:HEADER:version:reserved:reserved:usedInodes:freeInodes:allocatedInodes:maxInodes:
mmdf:nsd:0:1:::P_META_VD102:system:771751936:300:Yes:No:320274944:41:5005384:1::
mmdf:nsd:0:1:::P_DATA_VD02:data:46766489600:200:No:Yes:6092915712:13:154966272:0::
mmdf:poolTotal:0:1:::system:783308292096:380564840448:49:10024464464:1:1153081262080:
mmdf:data:0:1:::3647786188800:475190722560:13:12059515296:0:
mmdf:poolTotal:0:1:::data:3064453922816:1342362296320:44:1999215152:0:10143773212672:
mmdf:fsTotal:0:1:::3661677723648:481202021888:14:12117655064:0:
mmdf:inode:0:1:::430741822:484301506:915043328:1332164000:
`
	mmdfStdoutErrors = `
mmdf:nsd:HEADER:version:reserved:reserved:nsdName:storagePool:diskSize:failureGroup:metadata:data:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:diskAvailableForAlloc:
mmdf:poolTotal:HEADER:version:reserved:reserved:poolName:poolSize:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:maxDiskSize:
mmdf:data:HEADER:version:reserved:reserved:totalData:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:
mmdf:metadata:HEADER:version:reserved:reserved:totalMetadata:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:
mmdf:fsTotal:HEADER:version:reserved:reserved:foo:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:
mmdf:inode:HEADER:version:reserved:reserved:usedInodes:freeInodes:allocatedInodes:maxInodes:
mmdf:nsd:0:1:::P_META_VD102:system:771751936:300:Yes:No:320274944:41:5005384:1::
mmdf:nsd:0:1:::P_DATA_VD02:data:46766489600:200:No:Yes:6092915712:13:154966272:0::
mmdf:poolTotal:0:1:::system:foo:380564840448:49:10024464464:1:1153081262080:
mmdf:data:0:1:::3647786188800:475190722560:13:12059515296:0:
mmdf:metadata:0:1:::13891534848:6011299328:43:58139768:0:
mmdf:poolTotal:0:1:::data:3064453922816:1342362296320:44:1999215152:0:10143773212672:
mmdf:fsTotal:0:1:::foo:481202021888:14:12117655064:0:
mmdf:inode:0:1:::foo:484301506:915043328:1332164000:
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
	dfmetrics := parse_mmdf(mmdfStdout, log.NewNopLogger())
	if dfmetrics.InodesFree != 484301506 {
		t.Errorf("Unexpected value for InodesFree, got %v", dfmetrics.InodesFree)
	}
	if dfmetrics.FSTotal != 3749557989015552 {
		t.Errorf("Unexpected value for FSTotal, got %v", dfmetrics.FSTotal)
	}
	if dfmetrics.Metadata != true {
		t.Errorf("Unexpected value for Metadata, got %v", dfmetrics.Metadata)
	}
	if dfmetrics.MetadataTotal != 14224931684352 {
		t.Errorf("Unexpected value for MetadataTotal, got %v", dfmetrics.MetadataTotal)
	}
	if len(dfmetrics.Pools) != 2 {
		t.Errorf("Unexpected number of pools, got %v", len(dfmetrics.Pools))
	}
	dfmetrics = parse_mmdf(mmdfStdoutErrors, log.NewNopLogger())
	if dfmetrics.InodesFree != 484301506 {
		t.Errorf("Unexpected value for InodesFree, got %v", dfmetrics.InodesFree)
	}
	if dfmetrics.FSTotal != 0 {
		t.Errorf("Unexpected value for FSTotal, got %v", dfmetrics.FSTotal)
	}
	if dfmetrics.Metadata != true {
		t.Errorf("Unexpected value for Metadata, got %v", dfmetrics.Metadata)
	}
	if dfmetrics.MetadataTotal != 14224931684352 {
		t.Errorf("Unexpected value for MetadataTotal, got %v", dfmetrics.MetadataTotal)
	}
	if len(dfmetrics.Pools) != 2 {
		t.Errorf("Unexpected number of pools, got %v", len(dfmetrics.Pools))
	}
	dfmetrics = parse_mmdf(mmdfStdoutMissingMetadata, log.NewNopLogger())
	if dfmetrics.InodesFree != 484301506 {
		t.Errorf("Unexpected value for InodesFree, got %v", dfmetrics.InodesFree)
	}
	if dfmetrics.FSTotal != 3749557989015552 {
		t.Errorf("Unexpected value for FSTotal, got %v", dfmetrics.FSTotal)
	}
	if dfmetrics.Metadata != false {
		t.Errorf("Unexpected value for Metadata, got %v", dfmetrics.Metadata)
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
		# HELP gpfs_fs_allocated_inodes GPFS filesystem inodes allocated
		# TYPE gpfs_fs_allocated_inodes gauge
		gpfs_fs_allocated_inodes{fs="project"} 915043328
		# HELP gpfs_fs_free_bytes GPFS filesystem free size in bytes
		# TYPE gpfs_fs_free_bytes gauge
		gpfs_fs_free_bytes{fs="project"} 492750870413312
		# HELP gpfs_fs_free_inodes GPFS filesystem inodes free
		# TYPE gpfs_fs_free_inodes gauge
		gpfs_fs_free_inodes{fs="project"} 484301506
		# HELP gpfs_fs_inodes GPFS filesystem inodes total
		# TYPE gpfs_fs_inodes gauge
		gpfs_fs_inodes{fs="project"} 1332164000
		# HELP gpfs_fs_metadata_free_bytes GPFS metadata free size in bytes
		# TYPE gpfs_fs_metadata_free_bytes gauge
		gpfs_fs_metadata_free_bytes{fs="project"} 6155570511872
		# HELP gpfs_fs_metadata_size_bytes GPFS total metadata size in bytes
		# TYPE gpfs_fs_metadata_size_bytes gauge
		gpfs_fs_metadata_size_bytes{fs="project"} 14224931684352
		# HELP gpfs_fs_pool_free_bytes GPFS pool free size in bytes
		# TYPE gpfs_fs_pool_free_bytes gauge
		gpfs_fs_pool_free_bytes{fs="project",pool="data"} 1374578991431680
		gpfs_fs_pool_free_bytes{fs="project",pool="system"} 389698396618752
		# HELP gpfs_fs_pool_free_fragments_bytes GPFS pool free fragments in bytes
		# TYPE gpfs_fs_pool_free_fragments_bytes gauge
		gpfs_fs_pool_free_fragments_bytes{fs="project",pool="data"} 2047196315648
		gpfs_fs_pool_free_fragments_bytes{fs="project",pool="system"} 10265051611136
		# HELP gpfs_fs_pool_max_disk_size_bytes GPFS pool max disk size in bytes
		# TYPE gpfs_fs_pool_max_disk_size_bytes gauge
		gpfs_fs_pool_max_disk_size_bytes{fs="project",pool="data"} 10387223769776128
		gpfs_fs_pool_max_disk_size_bytes{fs="project",pool="system"} 1180755212369920
		# HELP gpfs_fs_pool_total_bytes GPFS pool total size in bytes
		# TYPE gpfs_fs_pool_total_bytes gauge
		gpfs_fs_pool_total_bytes{fs="project",pool="data"} 3138000816963584
		gpfs_fs_pool_total_bytes{fs="project",pool="system"} 802107691106304
		# HELP gpfs_fs_size_bytes GPFS filesystem total size in bytes
		# TYPE gpfs_fs_size_bytes gauge
		gpfs_fs_size_bytes{fs="project"} 3749557989015552
		# HELP gpfs_fs_used_inodes GPFS filesystem inodes used
		# TYPE gpfs_fs_used_inodes gauge
		gpfs_fs_used_inodes{fs="project"} 430741822
	`
	collector := NewMmdfCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 20 {
		t.Errorf("Unexpected collection count %d, expected 20", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_fs_used_inodes", "gpfs_fs_free_inodes", "gpfs_fs_allocated_inodes", "gpfs_fs_inodes",
		"gpfs_fs_free_bytes", "gpfs_fs_free_percent", "gpfs_fs_size_bytes",
		"gpfs_fs_pool_free_bytes", "gpfs_fs_pool_free_fragments_bytes",
		"gpfs_fs_pool_max_disk_size_bytes", "gpfs_fs_pool_total_bytes",
		"gpfs_fs_metadata_size_bytes", "gpfs_fs_metadata_free_bytes", "gpfs_fs_metadata_free_percent"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmdfCollectorNoMetadata(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "project"
	configFilesystems = &filesystems
	MmdfExec = func(fs string, ctx context.Context) (string, error) {
		return mmdfStdoutMissingMetadata, nil
	}
	expected := `
		# HELP gpfs_fs_allocated_inodes GPFS filesystem inodes allocated
		# TYPE gpfs_fs_allocated_inodes gauge
		gpfs_fs_allocated_inodes{fs="project"} 915043328
		# HELP gpfs_fs_free_bytes GPFS filesystem free size in bytes
		# TYPE gpfs_fs_free_bytes gauge
		gpfs_fs_free_bytes{fs="project"} 492750870413312
		# HELP gpfs_fs_free_inodes GPFS filesystem inodes free
		# TYPE gpfs_fs_free_inodes gauge
		gpfs_fs_free_inodes{fs="project"} 484301506
		# HELP gpfs_fs_inodes GPFS filesystem inodes total
		# TYPE gpfs_fs_inodes gauge
		gpfs_fs_inodes{fs="project"} 1332164000
		# HELP gpfs_fs_pool_free_bytes GPFS pool free size in bytes
		# TYPE gpfs_fs_pool_free_bytes gauge
		gpfs_fs_pool_free_bytes{fs="project",pool="data"} 1374578991431680
		gpfs_fs_pool_free_bytes{fs="project",pool="system"} 389698396618752
		# HELP gpfs_fs_pool_free_fragments_bytes GPFS pool free fragments in bytes
		# TYPE gpfs_fs_pool_free_fragments_bytes gauge
		gpfs_fs_pool_free_fragments_bytes{fs="project",pool="data"} 2047196315648
		gpfs_fs_pool_free_fragments_bytes{fs="project",pool="system"} 10265051611136
		# HELP gpfs_fs_pool_max_disk_size_bytes GPFS pool max disk size in bytes
		# TYPE gpfs_fs_pool_max_disk_size_bytes gauge
		gpfs_fs_pool_max_disk_size_bytes{fs="project",pool="data"} 10387223769776128
		gpfs_fs_pool_max_disk_size_bytes{fs="project",pool="system"} 1180755212369920
		# HELP gpfs_fs_pool_total_bytes GPFS pool total size in bytes
		# TYPE gpfs_fs_pool_total_bytes gauge
		gpfs_fs_pool_total_bytes{fs="project",pool="data"} 3138000816963584
		gpfs_fs_pool_total_bytes{fs="project",pool="system"} 802107691106304
		# HELP gpfs_fs_size_bytes GPFS filesystem total size in bytes
		# TYPE gpfs_fs_size_bytes gauge
		gpfs_fs_size_bytes{fs="project"} 3749557989015552
		# HELP gpfs_fs_used_inodes GPFS filesystem inodes used
		# TYPE gpfs_fs_used_inodes gauge
		gpfs_fs_used_inodes{fs="project"} 430741822
	`
	collector := NewMmdfCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 18 {
		t.Errorf("Unexpected collection count %d, expected 18", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_fs_used_inodes", "gpfs_fs_free_inodes", "gpfs_fs_allocated_inodes", "gpfs_fs_inodes",
		"gpfs_fs_free_bytes", "gpfs_fs_free_percent", "gpfs_fs_size_bytes",
		"gpfs_fs_pool_free_bytes", "gpfs_fs_pool_free_fragments_bytes",
		"gpfs_fs_pool_max_disk_size_bytes", "gpfs_fs_pool_total_bytes",
		"gpfs_fs_metadata_size_bytes", "gpfs_fs_metadata_free_bytes", "gpfs_fs_metadata_free_percent"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmdfCollectorMmlsfs(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := ""
	configFilesystems = &filesystems
	MmdfExec = func(fs string, ctx context.Context) (string, error) {
		return mmdfStdout, nil
	}
	mmlsfsStdout = `
fs::HEADER:version:reserved:reserved:deviceName:fieldName:data:remarks:
mmlsfs::0:1:::project:defaultMountPoint:%2Ffs%2Fproject::
`
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return mmlsfsStdout, nil
	}
	expected := `
		# HELP gpfs_fs_free_bytes GPFS filesystem free size in bytes
		# TYPE gpfs_fs_free_bytes gauge
		gpfs_fs_free_bytes{fs="project"} 492750870413312
		# HELP gpfs_fs_allocated_inodes GPFS filesystem inodes allocated
		# TYPE gpfs_fs_allocated_inodes gauge
		gpfs_fs_allocated_inodes{fs="project"} 915043328
		# HELP gpfs_fs_free_inodes GPFS filesystem inodes free
		# TYPE gpfs_fs_free_inodes gauge
		gpfs_fs_free_inodes{fs="project"} 484301506
		# HELP gpfs_fs_inodes GPFS filesystem inodes total
		# TYPE gpfs_fs_inodes gauge
		gpfs_fs_inodes{fs="project"} 1332164000
		# HELP gpfs_fs_used_inodes GPFS filesystem inodes used
		# TYPE gpfs_fs_used_inodes gauge
		gpfs_fs_used_inodes{fs="project"} 430741822
		# HELP gpfs_fs_metadata_free_bytes GPFS metadata free size in bytes
		# TYPE gpfs_fs_metadata_free_bytes gauge
		gpfs_fs_metadata_free_bytes{fs="project"} 6155570511872
		# HELP gpfs_fs_metadata_size_bytes GPFS total metadata size in bytes
		# TYPE gpfs_fs_metadata_size_bytes gauge
		gpfs_fs_metadata_size_bytes{fs="project"} 14224931684352
		# HELP gpfs_fs_pool_free_bytes GPFS pool free size in bytes
		# TYPE gpfs_fs_pool_free_bytes gauge
		gpfs_fs_pool_free_bytes{fs="project",pool="data"} 1374578991431680
		gpfs_fs_pool_free_bytes{fs="project",pool="system"} 389698396618752
		# HELP gpfs_fs_pool_free_fragments_bytes GPFS pool free fragments in bytes
		# TYPE gpfs_fs_pool_free_fragments_bytes gauge
		gpfs_fs_pool_free_fragments_bytes{fs="project",pool="data"} 2047196315648
		gpfs_fs_pool_free_fragments_bytes{fs="project",pool="system"} 10265051611136
		# HELP gpfs_fs_pool_max_disk_size_bytes GPFS pool max disk size in bytes
		# TYPE gpfs_fs_pool_max_disk_size_bytes gauge
		gpfs_fs_pool_max_disk_size_bytes{fs="project",pool="data"} 10387223769776128
		gpfs_fs_pool_max_disk_size_bytes{fs="project",pool="system"} 1180755212369920
		# HELP gpfs_fs_pool_total_bytes GPFS pool total size in bytes
		# TYPE gpfs_fs_pool_total_bytes gauge
		gpfs_fs_pool_total_bytes{fs="project",pool="data"} 3138000816963584
		gpfs_fs_pool_total_bytes{fs="project",pool="system"} 802107691106304
		# HELP gpfs_fs_size_bytes GPFS filesystem total size in bytes
		# TYPE gpfs_fs_size_bytes gauge
		gpfs_fs_size_bytes{fs="project"} 3749557989015552
	`
	collector := NewMmdfCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 22 {
		t.Errorf("Unexpected collection count %d, expected 22", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_fs_used_inodes", "gpfs_fs_free_inodes", "gpfs_fs_allocated_inodes", "gpfs_fs_inodes",
		"gpfs_fs_free_bytes", "gpfs_fs_free_percent", "gpfs_fs_size_bytes",
		"gpfs_fs_pool_free_bytes", "gpfs_fs_pool_free_fragments_bytes",
		"gpfs_fs_pool_max_disk_size_bytes", "gpfs_fs_pool_total_bytes",
		"gpfs_fs_metadata_size_bytes", "gpfs_fs_metadata_free_bytes", "gpfs_fs_metadata_free_percent"); err != nil {
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
	collector := NewMmdfCollector(log.NewNopLogger())
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
	collector := NewMmdfCollector(log.NewNopLogger())
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

func TestMmdfCollectorMmlsfsError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := ""
	configFilesystems = &filesystems
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmdf-mmlsfs"} 1
	`
	collector := NewMmdfCollector(log.NewNopLogger())
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

func TestMmdfCollectorMmlsfsTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := ""
	configFilesystems = &filesystems
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmdf-mmlsfs"} 1
	`
	collector := NewMmdfCollector(log.NewNopLogger())
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
