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

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
)

var (
	mmlsfilesetStdout = `
mmlsfileset::HEADER:version:reserved:reserved:filesystemName:filesetName:id:rootInode:status:path:parentId:created:inodes:dataInKB:comment:filesetMode:afmTarget:afmState:afmMode:afmFileLookupRefreshInterval:afmFileOpenRefreshInterval:afmDirLookupRefreshInterval:afmDirOpenRefreshInterval:afmAsyncDelay:afmNeedsRecovery:afmExpirationTimeout:afmRPO:afmLastPSnapId:inodeSpace:isInodeSpaceOwner:maxInodes:allocInodes:inodeSpaceMask:afmShowHomeSnapshots:afmNumReadThreads:reserved:afmReadBufferSize:afmWriteBufferSize:afmReadSparseThreshold:afmParallelReadChunkSize:afmParallelReadThreshold:snapId:afmNumFlushThreads:afmPrefetchThreshold:afmEnableAutoEviction:permChangeFlag:afmParallelWriteThreshold:freeInodes:afmNeedsResync:afmParallelWriteChunkSize:afmNumWriteThreads:afmPrimaryID:afmDRState:afmAssociatedPrimaryId:afmDIO:afmGatewayNode:afmIOFlags:
mmlsfileset::0:1:::project:root:0:3:Linked:%2Ffs%2Fproject:--:Wed May 18 10%3A41%3A35 2016:-:-:root fileset:off:-:-:-:-:-:-:-:-:-:-:-:-:0:1:300000000:102052224:2692530176:-:-:-:-:-:-:-:-:0:-:-:-:chmodAndSetacl:-:102045986:-:-:-:-:-:-:-:-:-:
mmlsfileset::0:1:::project:ibtest:1:524291:Linked:%2Ffs%2Fproject%2Fibtest:0:Tue Jun 28 07%3A08%3A46 2016:-:-::off:-:-:-:-:-:-:-:-:-:-:-:-:1:1:1000000:556032:2692530176:-:-:-:-:-:-:-:-:0:-:-:-:chmodAndSetacl:-:544397:-:-:-:-:-:-:-:-:-:
mmlsfileset::0:1:::project:PAS1136:2:17255366659:Unlinked:%2D%2D:--:Wed Nov 22 14%3A29%3A26 2017:-:-::off:-:-:-:-:-:-:-:-:-:-:-:-:164:1:1100000:1000000:2692530176:-:-:-:-:-:-:-:-:0:-:-:-:chmodAndSetacl:-:989069:-:-:-:-:-:-:-:-:-:
`
	mmlsfilesetStdoutBadTime = `
mmlsfileset::HEADER:version:reserved:reserved:filesystemName:filesetName:id:rootInode:status:path:parentId:created:inodes:dataInKB:comment:filesetMode:afmTarget:afmState:afmMode:afmFileLookupRefreshInterval:afmFileOpenRefreshInterval:afmDirLookupRefreshInterval:afmDirOpenRefreshInterval:afmAsyncDelay:afmNeedsRecovery:afmExpirationTimeout:afmRPO:afmLastPSnapId:inodeSpace:isInodeSpaceOwner:maxInodes:allocInodes:inodeSpaceMask:afmShowHomeSnapshots:afmNumReadThreads:reserved:afmReadBufferSize:afmWriteBufferSize:afmReadSparseThreshold:afmParallelReadChunkSize:afmParallelReadThreshold:snapId:afmNumFlushThreads:afmPrefetchThreshold:afmEnableAutoEviction:permChangeFlag:afmParallelWriteThreshold:freeInodes:afmNeedsResync:afmParallelWriteChunkSize:afmNumWriteThreads:afmPrimaryID:afmDRState:afmAssociatedPrimaryId:afmDIO:afmGatewayNode:afmIOFlags:
mmlsfileset::0:1:::project:root:0:3:Linked:%2Ffs%2Fproject:--:foo:-:-:root fileset:off:-:-:-:-:-:-:-:-:-:-:-:-:0:1:300000000:102052224:2692530176:-:-:-:-:-:-:-:-:0:-:-:-:chmodAndSetacl:-:102045986:-:-:-:-:-:-:-:-:-:
mmlsfileset::0:1:::project:ibtest:1:524291:Linked:%2Ffs%2Fproject%2Fibtest:0:Tue Jun 28 07%3A08%3A46 2016:-:-::off:-:-:-:-:-:-:-:-:-:-:-:-:1:1:1000000:556032:2692530176:-:-:-:-:-:-:-:-:0:-:-:-:chmodAndSetacl:-:544397:-:-:-:-:-:-:-:-:-:
mmlsfileset::0:1:::project:PAS1136:2:17255366659:Unlinked:%2D%2D:--:Wed Nov 22 14%3A29%3A26 2017:-:-::off:-:-:-:-:-:-:-:-:-:-:-:-:164:1:1100000:1000000:2692530176:-:-:-:-:-:-:-:-:0:-:-:-:chmodAndSetacl:-:989069:-:-:-:-:-:-:-:-:-:
`
	mmlsfilesetStdoutBadValue = `
mmlsfileset::HEADER:version:reserved:reserved:filesystemName:filesetName:id:rootInode:status:path:parentId:created:inodes:dataInKB:comment:filesetMode:afmTarget:afmState:afmMode:afmFileLookupRefreshInterval:afmFileOpenRefreshInterval:afmDirLookupRefreshInterval:afmDirOpenRefreshInterval:afmAsyncDelay:afmNeedsRecovery:afmExpirationTimeout:afmRPO:afmLastPSnapId:inodeSpace:isInodeSpaceOwner:maxInodes:allocInodes:inodeSpaceMask:afmShowHomeSnapshots:afmNumReadThreads:reserved:afmReadBufferSize:afmWriteBufferSize:afmReadSparseThreshold:afmParallelReadChunkSize:afmParallelReadThreshold:snapId:afmNumFlushThreads:afmPrefetchThreshold:afmEnableAutoEviction:permChangeFlag:afmParallelWriteThreshold:freeInodes:afmNeedsResync:afmParallelWriteChunkSize:afmNumWriteThreads:afmPrimaryID:afmDRState:afmAssociatedPrimaryId:afmDIO:afmGatewayNode:afmIOFlags:
mmlsfileset::0:1:::project:root:0:3:Linked:%2Ffs%2Fproject:--:Wed May 18 10%3A41%3A35 2016:-:-:root fileset:off:-:-:-:-:-:-:-:-:-:-:-:-:0:1:foo:foo:foo:-:-:-:-:-:-:-:-:0:-:-:-:chmodAndSetacl:-:102045986:-:-:-:-:-:-:-:-:-:
mmlsfileset::0:1:::project:ibtest:1:524291:Linked:%2Ffs%2Fproject%2Fibtest:0:Tue Jun 28 07%3A08%3A46 2016:-:-::off:-:-:-:-:-:-:-:-:-:-:-:-:1:1:1000000:556032:2692530176:-:-:-:-:-:-:-:-:0:-:-:-:chmodAndSetacl:-:544397:-:-:-:-:-:-:-:-:-:
mmlsfileset::0:1:::project:PAS1136:2:17255366659:Unlinked:%2D%2D:--:Wed Nov 22 14%3A29%3A26 2017:-:-::off:-:-:-:-:-:-:-:-:-:-:-:-:164:1:1100000:1000000:2692530176:-:-:-:-:-:-:-:-:0:-:-:-:chmodAndSetacl:-:989069:-:-:-:-:-:-:-:-:-:

`
)

func TestMmlsfileset(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmlsfileset("test", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if out != mockedStdout {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmlsfilesetError(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmlsfileset("test", ctx)
	if err == nil {
		t.Errorf("Expected error")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmlsfilesetTimeout(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
	defer cancel()
	out, err := mmlsfileset("test", ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestParseMmlsfileset(t *testing.T) {
	metrics, err := parse_mmlsfileset(mmlsfilesetStdout, promslog.NewNopLogger())
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if len(metrics) != 3 {
		t.Errorf("Unexpected number of metrics, got %d", len(metrics))
		return
	}
	if metrics[0].FS != "project" {
		t.Errorf("Unexpected value for FS, got %v", metrics[0].FS)
	}
	if metrics[0].Fileset != "root" {
		t.Errorf("Unexpected value for Fileset, got %v", metrics[0].Fileset)
	}
	if metrics[0].Status != "Linked" {
		t.Errorf("Unexpected value for Status, got %v", metrics[0].Status)
	}
	if metrics[0].Path != "/fs/project" {
		t.Errorf("Unexpected value for Path, got %v", metrics[0].Path)
	}
	if metrics[0].Created != 1463586095 {
		t.Errorf("Unexpected value for Created, got %v", metrics[0].Created)
	}
}

func TestParseMmlsfilesetErrors(t *testing.T) {
	_, err := parse_mmlsfileset(mmlsfilesetStdoutBadTime, promslog.NewNopLogger())
	if err == nil {
		t.Errorf("Expected error")
		return
	}
	_, err = parse_mmlsfileset(mmlsfilesetStdoutBadValue, promslog.NewNopLogger())
	if err == nil {
		t.Errorf("Expected error")
		return
	}
}

func TestMmlsfilesetCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "project"
	filesetFilesystems = &filesystems
	MmlsfilesetExec = func(fs string, ctx context.Context) (string, error) {
		return mmlsfilesetStdout, nil
	}
	expected := `
		# HELP gpfs_fileset_alloc_inodes GPFS fileset alloc inodes
		# TYPE gpfs_fileset_alloc_inodes gauge
		gpfs_fileset_alloc_inodes{fileset="PAS1136",fs="project"} 1000000
		gpfs_fileset_alloc_inodes{fileset="ibtest",fs="project"} 556032
		gpfs_fileset_alloc_inodes{fileset="root",fs="project"} 102052224
		# HELP gpfs_fileset_created_timestamp_seconds GPFS fileset creation timestamp
		# TYPE gpfs_fileset_created_timestamp_seconds gauge
		gpfs_fileset_created_timestamp_seconds{fileset="PAS1136",fs="project"} 1511378966
		gpfs_fileset_created_timestamp_seconds{fileset="ibtest",fs="project"} 1467115726
		gpfs_fileset_created_timestamp_seconds{fileset="root",fs="project"} 1463586095
		# HELP gpfs_fileset_free_inodes GPFS fileset free inodes
		# TYPE gpfs_fileset_free_inodes gauge
		gpfs_fileset_free_inodes{fileset="PAS1136",fs="project"} 989069
		gpfs_fileset_free_inodes{fileset="ibtest",fs="project"} 544397
		gpfs_fileset_free_inodes{fileset="root",fs="project"} 102045986
		# HELP gpfs_fileset_max_inodes GPFS fileset max inodes
		# TYPE gpfs_fileset_max_inodes gauge
		gpfs_fileset_max_inodes{fileset="PAS1136",fs="project"} 1100000
		gpfs_fileset_max_inodes{fileset="ibtest",fs="project"} 1000000
		gpfs_fileset_max_inodes{fileset="root",fs="project"} 300000000
		# HELP gpfs_fileset_path_info GPFS fileset path
		# TYPE gpfs_fileset_path_info gauge
		gpfs_fileset_path_info{fileset="PAS1136",fs="project",path="--"} 1
		gpfs_fileset_path_info{fileset="ibtest",fs="project",path="/fs/project/ibtest"} 1
		gpfs_fileset_path_info{fileset="root",fs="project",path="/fs/project"} 1
		# HELP gpfs_fileset_status_info GPFS fileset status
		# TYPE gpfs_fileset_status_info gauge
		gpfs_fileset_status_info{fileset="PAS1136",fs="project",status="Unlinked"} 1
		gpfs_fileset_status_info{fileset="ibtest",fs="project",status="Linked"} 1
		gpfs_fileset_status_info{fileset="root",fs="project",status="Linked"} 1
	`
	collector := NewMmlsfilesetCollector(promslog.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 21 {
		t.Errorf("Unexpected collection count %d, expected 21", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_fileset_created_timestamp_seconds", "gpfs_fileset_status_info", "gpfs_fileset_path_info",
		"gpfs_fileset_alloc_inodes", "gpfs_fileset_free_inodes", "gpfs_fileset_max_inodes"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmlsfilesetCollectorMmlsfs(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	MmlsfilesetExec = func(fs string, ctx context.Context) (string, error) {
		return mmlsfilesetStdout, nil
	}
	mmlsfsStdout = `
fs::HEADER:version:reserved:reserved:deviceName:fieldName:data:remarks:
mmlsfs::0:1:::project:defaultMountPoint:%2Ffs%project::
`
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return mmlsfsStdout, nil
	}
	expected := `
		# HELP gpfs_fileset_alloc_inodes GPFS fileset alloc inodes
		# TYPE gpfs_fileset_alloc_inodes gauge
		gpfs_fileset_alloc_inodes{fileset="PAS1136",fs="project"} 1000000
		gpfs_fileset_alloc_inodes{fileset="ibtest",fs="project"} 556032
		gpfs_fileset_alloc_inodes{fileset="root",fs="project"} 102052224
		# HELP gpfs_fileset_created_timestamp_seconds GPFS fileset creation timestamp
		# TYPE gpfs_fileset_created_timestamp_seconds gauge
		gpfs_fileset_created_timestamp_seconds{fileset="PAS1136",fs="project"} 1511378966
		gpfs_fileset_created_timestamp_seconds{fileset="ibtest",fs="project"} 1467115726
		gpfs_fileset_created_timestamp_seconds{fileset="root",fs="project"} 1463586095
		# HELP gpfs_fileset_free_inodes GPFS fileset free inodes
		# TYPE gpfs_fileset_free_inodes gauge
		gpfs_fileset_free_inodes{fileset="PAS1136",fs="project"} 989069
		gpfs_fileset_free_inodes{fileset="ibtest",fs="project"} 544397
		gpfs_fileset_free_inodes{fileset="root",fs="project"} 102045986
		# HELP gpfs_fileset_max_inodes GPFS fileset max inodes
		# TYPE gpfs_fileset_max_inodes gauge
		gpfs_fileset_max_inodes{fileset="PAS1136",fs="project"} 1100000
		gpfs_fileset_max_inodes{fileset="ibtest",fs="project"} 1000000
		gpfs_fileset_max_inodes{fileset="root",fs="project"} 300000000
		# HELP gpfs_fileset_path_info GPFS fileset path
		# TYPE gpfs_fileset_path_info gauge
		gpfs_fileset_path_info{fileset="PAS1136",fs="project",path="--"} 1
		gpfs_fileset_path_info{fileset="ibtest",fs="project",path="/fs/project/ibtest"} 1
		gpfs_fileset_path_info{fileset="root",fs="project",path="/fs/project"} 1
		# HELP gpfs_fileset_status_info GPFS fileset status
		# TYPE gpfs_fileset_status_info gauge
		gpfs_fileset_status_info{fileset="PAS1136",fs="project",status="Unlinked"} 1
		gpfs_fileset_status_info{fileset="ibtest",fs="project",status="Linked"} 1
		gpfs_fileset_status_info{fileset="root",fs="project",status="Linked"} 1
	`
	collector := NewMmlsfilesetCollector(promslog.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 21 {
		t.Errorf("Unexpected collection count %d, expected 21", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_fileset_created_timestamp_seconds", "gpfs_fileset_status_info", "gpfs_fileset_path_info",
		"gpfs_fileset_alloc_inodes", "gpfs_fileset_free_inodes", "gpfs_fileset_max_inodes"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmlsfilesetCollectorError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "project"
	configFilesystems = &filesystems
	MmlsfilesetExec = func(fs string, ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmlsfileset-project"} 1
	`
	collector := NewMmlsfilesetCollector(promslog.NewNopLogger())
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

func TestMmlsfilesetCollectorTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "project"
	configFilesystems = &filesystems
	MmlsfilesetExec = func(fs string, ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmlsfileset-project"} 1
	`
	collector := NewMmlsfilesetCollector(promslog.NewNopLogger())
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

func TestMmlsfilesetCollectorMmlsfsError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystem := ""
	filesetFilesystems = &filesystem
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmlsfileset-mmlsfs"} 1
	`
	collector := NewMmlsfilesetCollector(promslog.NewNopLogger())
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

func TestMmlsfilesetCollectorMmlsfsTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystem := ""
	filesetFilesystems = &filesystem
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmlsfileset-mmlsfs"} 1
	`
	collector := NewMmlsfilesetCollector(promslog.NewNopLogger())
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
