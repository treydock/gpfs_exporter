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
	mmlssnapshotStdout = `
mmlssnapshot::HEADER:version:reserved:reserved:filesystemName:directory:snapID:status:created:quotas:data:metadata:fileset:snapType:
mmlssnapshot::0:1:::ess:20201115_PAS1736:16337:Valid:Sun Nov 15 02%3A47%3A48 2020::0:0:PAS1736::
mmlssnapshot::0:1:::ess:20210120:27107:Valid:Wed Jan 20 00%3A30%3A02 2021::0:0:::
`
	mmlssnapshotStdoutData = `
mmlssnapshot::HEADER:version:reserved:reserved:filesystemName:directory:snapID:status:created:quotas:data:metadata:fileset:snapType:
mmlssnapshot::0:1:::ess:20210120:27107:Valid:Wed Jan 20 00%3A30%3A02 2021::823587352320:529437984:::
mmlssnapshot::0:1:::ess:20201115_PAS1736:16337:Valid:Sun Nov 15 02%3A47%3A48 2020::0:205184:PAS1736::
`
	mmlssnapshotStdoutBadTime = `
mmlssnapshot::HEADER:version:reserved:reserved:filesystemName:directory:snapID:status:created:quotas:data:metadata:fileset:snapType:
mmlssnapshot::0:1:::ess:20201115_PAS1736:16337:Valid:Sun Nov 15 foo::0:205184:PAS1736::
`
	mmlssnapshotStdoutBadValue = `
mmlssnapshot::HEADER:version:reserved:reserved:filesystemName:directory:snapID:status:created:quotas:data:metadata:fileset:snapType:
mmlssnapshot::0:1:::ess:20201115_PAS1736:16337:Valid:Sun Nov 15 02%3A47%3A48 2020::0:foo:PAS1736::
`
)

func TestMmlssnapshot(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmlssnapshot("test", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if out != mockedStdout {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmlssnapshotError(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmlssnapshot("test", ctx)
	if err == nil {
		t.Errorf("Expected error")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmlssnapshotTimeout(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
	defer cancel()
	out, err := mmlssnapshot("test", ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestParseMmlssnapshot(t *testing.T) {
	metrics, err := parse_mmlssnapshot(mmlssnapshotStdout, promslog.NewNopLogger())
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if len(metrics) != 2 {
		t.Errorf("Unexpected number of metrics, got %d", len(metrics))
		return
	}
	if metrics[0].Name != "20201115_PAS1736" {
		t.Errorf("Unexpected value for Name, got %v", metrics[0].Name)
	}
	if metrics[0].Fileset != "PAS1736" {
		t.Errorf("Unexpected value for Fileset, got %v", metrics[0].Fileset)
	}
	if metrics[0].Status != "Valid" {
		t.Errorf("Unexpected value for Status, got %v", metrics[0].Status)
	}
	if metrics[0].Created != 1605426468 {
		t.Errorf("Unexpected value for Created, got %v", metrics[0].Created)
	}
	metrics, err = parse_mmlssnapshot(mmlssnapshotStdoutData, promslog.NewNopLogger())
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if metrics[0].Data != 843353448775680 {
		t.Errorf("Unexpected value for Data, got %v", metrics[0].Data)
	}
	if metrics[0].Metadata != 542144495616 {
		t.Errorf("Unexpected value for Metadata, got %v", metrics[0].Metadata)
	}
}

func TestParseMmlssnapshotErrors(t *testing.T) {
	_, err := parse_mmlssnapshot(mmlssnapshotStdoutBadTime, promslog.NewNopLogger())
	if err == nil {
		t.Errorf("Expected error")
		return
	}
	_, err = parse_mmlssnapshot(mmlssnapshotStdoutBadValue, promslog.NewNopLogger())
	if err == nil {
		t.Errorf("Expected error")
		return
	}
}

func TestMmlssnapshotCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "ess"
	snapshotFilesystems = &filesystems
	MmlssnapshotExec = func(fs string, ctx context.Context) (string, error) {
		return mmlssnapshotStdout, nil
	}
	expected := `
		# HELP gpfs_snapshot_created_timestamp_seconds GPFS snapshot creation timestamp
		# TYPE gpfs_snapshot_created_timestamp_seconds gauge
		gpfs_snapshot_created_timestamp_seconds{fileset="PAS1736",fs="ess",id="16337",snapshot="20201115_PAS1736"} 1605426468
		gpfs_snapshot_created_timestamp_seconds{fileset="",fs="ess",id="27107",snapshot="20210120"} 1611120602
		# HELP gpfs_snapshot_status_info GPFS snapshot status
		# TYPE gpfs_snapshot_status_info gauge
		gpfs_snapshot_status_info{fileset="PAS1736",fs="ess",id="16337",snapshot="20201115_PAS1736",status="Valid"} 1
		gpfs_snapshot_status_info{fileset="",fs="ess",id="27107",snapshot="20210120",status="Valid"} 1
	`
	collector := NewMmlssnapshotCollector(promslog.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 8 {
		t.Errorf("Unexpected collection count %d, expected 8", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_snapshot_created_timestamp_seconds", "gpfs_snapshot_status_info",
		"gpfs_snapshot_data_size_bytes", "gpfs_snapshot_metadata_size_bytes"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmlssnapshotCollectorData(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{"--collector.mmlssnapshot.get-size"}); err != nil {
		t.Fatal(err)
	}
	filesystems := "ess"
	snapshotFilesystems = &filesystems
	MmlssnapshotExec = func(fs string, ctx context.Context) (string, error) {
		return mmlssnapshotStdoutData, nil
	}
	expected := `
		# HELP gpfs_snapshot_created_timestamp_seconds GPFS snapshot creation timestamp
		# TYPE gpfs_snapshot_created_timestamp_seconds gauge
		gpfs_snapshot_created_timestamp_seconds{fileset="PAS1736",fs="ess",id="16337",snapshot="20201115_PAS1736"} 1605426468
		gpfs_snapshot_created_timestamp_seconds{fileset="",fs="ess",id="27107",snapshot="20210120"} 1611120602
		# HELP gpfs_snapshot_data_size_bytes GPFS snapshot data size
		# TYPE gpfs_snapshot_data_size_bytes gauge
		gpfs_snapshot_data_size_bytes{fileset="PAS1736",fs="ess",id="16337",snapshot="20201115_PAS1736"} 0
		gpfs_snapshot_data_size_bytes{fileset="",fs="ess",id="27107",snapshot="20210120"} 843353448775680
		# HELP gpfs_snapshot_metadata_size_bytes GPFS snapshot metadata size
		# TYPE gpfs_snapshot_metadata_size_bytes gauge
		gpfs_snapshot_metadata_size_bytes{fileset="PAS1736",fs="ess",id="16337",snapshot="20201115_PAS1736"} 210108416
		gpfs_snapshot_metadata_size_bytes{fileset="",fs="ess",id="27107",snapshot="20210120"} 542144495616
		# HELP gpfs_snapshot_status_info GPFS snapshot status
		# TYPE gpfs_snapshot_status_info gauge
		gpfs_snapshot_status_info{fileset="PAS1736",fs="ess",id="16337",snapshot="20201115_PAS1736",status="Valid"} 1
		gpfs_snapshot_status_info{fileset="",fs="ess",id="27107",snapshot="20210120",status="Valid"} 1
	`
	collector := NewMmlssnapshotCollector(promslog.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 12 {
		t.Errorf("Unexpected collection count %d, expected 12", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_snapshot_created_timestamp_seconds", "gpfs_snapshot_status_info",
		"gpfs_snapshot_data_size_bytes", "gpfs_snapshot_metadata_size_bytes"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmlssnapshotCollectorMmlsfs(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	MmlssnapshotExec = func(fs string, ctx context.Context) (string, error) {
		return mmlssnapshotStdout, nil
	}
	mmlsfsStdout = `
fs::HEADER:version:reserved:reserved:deviceName:fieldName:data:remarks:
mmlsfs::0:1:::ess:defaultMountPoint:%2Ffs%ess::
`
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return mmlsfsStdout, nil
	}
	expected := `
		# HELP gpfs_snapshot_created_timestamp_seconds GPFS snapshot creation timestamp
		# TYPE gpfs_snapshot_created_timestamp_seconds gauge
		gpfs_snapshot_created_timestamp_seconds{fileset="PAS1736",fs="ess",id="16337",snapshot="20201115_PAS1736"} 1605426468
		gpfs_snapshot_created_timestamp_seconds{fileset="",fs="ess",id="27107",snapshot="20210120"} 1611120602
		# HELP gpfs_snapshot_status_info GPFS snapshot status
		# TYPE gpfs_snapshot_status_info gauge
		gpfs_snapshot_status_info{fileset="PAS1736",fs="ess",id="16337",snapshot="20201115_PAS1736",status="Valid"} 1
		gpfs_snapshot_status_info{fileset="",fs="ess",id="27107",snapshot="20210120",status="Valid"} 1
	`
	collector := NewMmlssnapshotCollector(promslog.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 8 {
		t.Errorf("Unexpected collection count %d, expected 8", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_snapshot_created_timestamp_seconds", "gpfs_snapshot_status_info",
		"gpfs_snapshot_data_size_bytes", "gpfs_snapshot_metadata_size_bytes"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmlssnapshotCollectorError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "ess"
	configFilesystems = &filesystems
	MmlssnapshotExec = func(fs string, ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmlssnapshot-ess"} 1
	`
	collector := NewMmlssnapshotCollector(promslog.NewNopLogger())
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

func TestMmlssnapshotCollectorTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "ess"
	configFilesystems = &filesystems
	MmlssnapshotExec = func(fs string, ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmlssnapshot-ess"} 1
	`
	collector := NewMmlssnapshotCollector(promslog.NewNopLogger())
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

func TestMmlssnapshotCollectorMmlsfsError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystem := ""
	snapshotFilesystems = &filesystem
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmlssnapshot-mmlsfs"} 1
	`
	collector := NewMmlssnapshotCollector(promslog.NewNopLogger())
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

func TestMmlssnapshotCollectorMmlsfsTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystem := ""
	snapshotFilesystems = &filesystem
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmlssnapshot-mmlsfs"} 1
	`
	collector := NewMmlssnapshotCollector(promslog.NewNopLogger())
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
