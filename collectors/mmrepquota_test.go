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
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var (
	mmrepquotaStdout = `
*** Report for FILESET quotas on project
mmrepquota::HEADER:version:reserved:reserved:filesystemName:quotaType:id:name:blockUsage:blockQuota:blockLimit:blockInDoubt:blockGrace:filesUsage:filesQuota:filesLimit:filesInDoubt:filesGrace:remarks:quota:defQuota:fid:filesetname:
mmrepquota::0:1:::project:FILESET:0:root:337419744:0:0:163840:none:1395:0:0:400:none:i:on:off:::
mmrepquota::0:1:::project:FILESET:408:PZS1003:341467872:2147483648:2147483648:0:none:6286:2000000:2000000:0:none:e:on:off:::
*** Report for FILESET quotas on scratch
mmrepquota::HEADER:version:reserved:reserved:filesystemName:quotaType:id:name:blockUsage:blockQuota:blockLimit:blockInDoubt:blockGrace:filesUsage:filesQuota:filesLimit:filesInDoubt:filesGrace:remarks:quota:defQuota:fid:filesetname:
mmrepquota::0:1:::scratch:FILESET:0:root:928235294208:0:0:5308909920:none:141909093:0:0:140497:none:i:on:off:::
`
)

func TestMmrepquota(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmrepquota(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if out != mockedStdout {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmrepquotaError(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmrepquota(ctx)
	if err == nil {
		t.Errorf("Expected error")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmrepquotaTimeout(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
	defer cancel()
	out, err := mmrepquota(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestParseMmrepquota(t *testing.T) {
	metrics := parse_mmrepquota(mmrepquotaStdout, log.NewNopLogger())
	if len(metrics) != 3 {
		t.Errorf("Unexpected metric count: %d", len(metrics))
		return
	}
	if val := metrics[0].BlockUsage; val != 345517817856 {
		t.Errorf("Unexpected BlockUsage got %v", val)
	}
	if val := metrics[0].BlockQuota; val != 0 {
		t.Errorf("Unexpected BlockQuota got %v", val)
	}
	if val := metrics[0].BlockLimit; val != 0 {
		t.Errorf("Unexpected BlockLimit got %v", val)
	}
	if val := metrics[0].BlockInDoubt; val != 167772160 {
		t.Errorf("Unexpected BlockInDoubt got %v", val)
	}
}

func TestMmrepquotaCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	mmrepquotaExec = func(ctx context.Context) (string, error) {
		return mmrepquotaStdout, nil
	}
	expected := `
# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
# TYPE gpfs_exporter_collect_error gauge
gpfs_exporter_collect_error{collector="mmrepquota"} 0
# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
# TYPE gpfs_exporter_collect_timeout gauge
gpfs_exporter_collect_timeout{collector="mmrepquota"} 0
# HELP gpfs_fileset_in_doubt_bytes GPFS fileset quota block in doubt
# TYPE gpfs_fileset_in_doubt_bytes gauge
gpfs_fileset_in_doubt_bytes{fileset="PZS1003",fs="project"} 0
gpfs_fileset_in_doubt_bytes{fileset="root",fs="project"} 167772160
gpfs_fileset_in_doubt_bytes{fileset="root",fs="scratch"} 5436323758080
# HELP gpfs_fileset_in_doubt_files GPFS fileset quota files in doubt
# TYPE gpfs_fileset_in_doubt_files gauge
gpfs_fileset_in_doubt_files{fileset="PZS1003",fs="project"} 0
gpfs_fileset_in_doubt_files{fileset="root",fs="project"} 400
gpfs_fileset_in_doubt_files{fileset="root",fs="scratch"} 140497
# HELP gpfs_fileset_limit_bytes GPFS fileset quota block limit
# TYPE gpfs_fileset_limit_bytes gauge
gpfs_fileset_limit_bytes{fileset="PZS1003",fs="project"} 2199023255552
gpfs_fileset_limit_bytes{fileset="root",fs="project"} 0
gpfs_fileset_limit_bytes{fileset="root",fs="scratch"} 0
# HELP gpfs_fileset_limit_files GPFS fileset quota files limit
# TYPE gpfs_fileset_limit_files gauge
gpfs_fileset_limit_files{fileset="PZS1003",fs="project"} 2000000
gpfs_fileset_limit_files{fileset="root",fs="project"} 0
gpfs_fileset_limit_files{fileset="root",fs="scratch"} 0
# HELP gpfs_fileset_quota_bytes GPFS fileset block quota
# TYPE gpfs_fileset_quota_bytes gauge
gpfs_fileset_quota_bytes{fileset="PZS1003",fs="project"} 2199023255552
gpfs_fileset_quota_bytes{fileset="root",fs="project"} 0
gpfs_fileset_quota_bytes{fileset="root",fs="scratch"} 0
# HELP gpfs_fileset_quota_files GPFS fileset files quota
# TYPE gpfs_fileset_quota_files gauge
gpfs_fileset_quota_files{fileset="PZS1003",fs="project"} 2000000
gpfs_fileset_quota_files{fileset="root",fs="project"} 0
gpfs_fileset_quota_files{fileset="root",fs="scratch"} 0
# HELP gpfs_fileset_used_bytes GPFS fileset quota used
# TYPE gpfs_fileset_used_bytes gauge
gpfs_fileset_used_bytes{fileset="PZS1003",fs="project"} 349663100928
gpfs_fileset_used_bytes{fileset="root",fs="project"} 345517817856
gpfs_fileset_used_bytes{fileset="root",fs="scratch"} 950512941268992
# HELP gpfs_fileset_used_files GPFS fileset quota files used
# TYPE gpfs_fileset_used_files gauge
gpfs_fileset_used_files{fileset="PZS1003",fs="project"} 6286
gpfs_fileset_used_files{fileset="root",fs="project"} 1395
gpfs_fileset_used_files{fileset="root",fs="scratch"} 141909093
	`
	collector := NewMmrepquotaCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 27 {
		t.Errorf("Unexpected collection count %d, expected 27", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_exporter_collect_error", "gpfs_exporter_collect_timeout",
		"gpfs_fileset_in_doubt_bytes", "gpfs_fileset_in_doubt_files",
		"gpfs_fileset_limit_bytes", "gpfs_fileset_limit_files",
		"gpfs_fileset_quota_bytes", "gpfs_fileset_quota_files",
		"gpfs_fileset_used_bytes", "gpfs_fileset_used_files"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMMrepquotaCollectorError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	mmrepquotaExec = func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmrepquota"} 1
	`
	collector := NewMmrepquotaCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 3 {
		t.Errorf("Unexpected collection count %d, expected 3", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_exporter_collect_error", "gpfs_fileset_used_bytes"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMMrepquotaCollectorTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	mmrepquotaExec = func(ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmrepquota"} 1
	`
	collector := NewMmrepquotaCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 3 {
		t.Errorf("Unexpected collection count %d, expected 3", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_exporter_collect_timeout", "gpfs_fileset_used_bytes"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}
