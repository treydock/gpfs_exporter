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
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var (
	mmlsdiskStdout = `
mmlsdisk::HEADER:version:reserved:reserved:nsdName:driverType:sectorSize:failureGroup:metadata:data:status:availability:diskID:storagePool:remarks:numQuorumDisks:readQuorumValue:writeQuorumValue:diskSizeKB:diskUID:thinDiskType:
mmlsdisk::0:1:::RG001VS003:nsd:512:1:yes:yes:ready:up:1:system:desc:3:2:2:130518089728:C0A82D155E83D23B::
mmlsdisk::0:1:::RG002VS003:nsd:512:2:yes:yes:ready:up:2:system:desc:3:2:2:130518089728:C0A82D155E83D238::
mmlsdisk::0:1:::RG003VS004:nsd:512:1:no:yes:ready:up:3:data:desc:3:2:2:2227566542848:C0A82D155E83D23C::
mmlsdisk::0:1:::RG004VS004:nsd:512:1:no:yes:ready:up:4:data::3:2:2:2227566542848:C0A82D155E83D23A::
mmlsdisk::0:1:::RG005VS005:nsd:512:2:no:yes:ready:up:5:data::3:2:2:2227566542848:C0A82D155E83D23D::
mmlsdisk::0:1:::RG006VS005:nsd:512:2:no:yes:ready:up:6:data::3:2:2:2227566542848:C0A82D155E83D239::
mmlsdisk::0:1:::RG007VS009:nsd:512:1:no:yes:ready:up:7:ess5000::3:2:2:2257848107008:0703160A632377DB::
mmlsdisk::0:1:::RG008VS010:nsd:512:2:no:yes:foo:bar:8:ess5000::3:2:2:2257848107008:0703160A632377DA::
mmlsdisk:invalid
`
)

func TestMmlsdisk(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmlsdisk("test", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if out != mockedStdout {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmlsdiskError(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmlsdisk("test", ctx)
	if err == nil {
		t.Errorf("Expected error")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmlsdiskTimeout(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
	defer cancel()
	out, err := mmlsdisk("test", ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestParseMmlsdisk(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	metrics, err := parse_mmlsdisk(mmlsdiskStdout, logger)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if len(metrics) != 8 {
		t.Errorf("Unexpected number of metrics, got %d", len(metrics))
		return
	}
	if metrics[0].Name != "RG001VS003" {
		t.Errorf("Unexpected value for Name, got %v", metrics[0].Name)
	}
	if metrics[0].Metadata != "yes" {
		t.Errorf("Unexpected value for Metadata, got %v", metrics[0].Metadata)
	}
	if metrics[0].Data != "yes" {
		t.Errorf("Unexpected value for Data, got %v", metrics[0].Data)
	}
	if metrics[0].Status != "ready" {
		t.Errorf("Unexpected value for Status, got %v", metrics[0].Status)
	}
	if metrics[0].Availability != "up" {
		t.Errorf("Unexpected value for Availability, got %v", metrics[0].Availability)
	}
	if metrics[0].DiskID != "1" {
		t.Errorf("Unexpected value for DiskID, got %v", metrics[0].DiskID)
	}
	if metrics[0].StoragePool != "system" {
		t.Errorf("Unexpected value for StoragePool, got %v", metrics[0].StoragePool)
	}
}

func TestMmlsdiskCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "ess"
	diskFilesystems = &filesystems
	MmlsdiskExec = func(fs string, ctx context.Context) (string, error) {
		return mmlsdiskStdout, nil
	}
	expected := `
		# HELP gpfs_disk_availability GPFS disk availability
        # TYPE gpfs_disk_availability gauge
        gpfs_disk_availability{availability="down",data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",storagepool="system"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",storagepool="system"} 1
        gpfs_disk_availability{availability="recovering",data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",storagepool="system"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",storagepool="system"} 0
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",storagepool="system"} 0

        gpfs_disk_availability{availability="down",data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",storagepool="system"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",storagepool="system"} 1
        gpfs_disk_availability{availability="recovering",data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",storagepool="system"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",storagepool="system"} 0
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",storagepool="system"} 0

        gpfs_disk_availability{availability="down",data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",storagepool="data"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",storagepool="data"} 1
        gpfs_disk_availability{availability="recovering",data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",storagepool="data"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",storagepool="data"} 0
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",storagepool="data"} 0

        gpfs_disk_availability{availability="down",data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",storagepool="data"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",storagepool="data"} 1
        gpfs_disk_availability{availability="recovering",data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",storagepool="data"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",storagepool="data"} 0
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",storagepool="data"} 0

        gpfs_disk_availability{availability="down",data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",storagepool="data"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",storagepool="data"} 1
        gpfs_disk_availability{availability="recovering",data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",storagepool="data"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",storagepool="data"} 0
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",storagepool="data"} 0

        gpfs_disk_availability{availability="down",data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",storagepool="data"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",storagepool="data"} 1
        gpfs_disk_availability{availability="recovering",data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",storagepool="data"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",storagepool="data"} 0
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",storagepool="data"} 0

        gpfs_disk_availability{availability="down",data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",storagepool="ess5000"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",storagepool="ess5000"} 1
        gpfs_disk_availability{availability="recovering",data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",storagepool="ess5000"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",storagepool="ess5000"} 0
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",storagepool="ess5000"} 0

        gpfs_disk_availability{availability="down",data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",storagepool="ess5000"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",storagepool="ess5000"} 0
        gpfs_disk_availability{availability="recovering",data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",storagepool="ess5000"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",storagepool="ess5000"} 1
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",storagepool="ess5000"} 0
        # HELP gpfs_disk_status GPFS disk status
        # TYPE gpfs_disk_status gauge
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="being emptied",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="emptied",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="ready",storagepool="system"} 1
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="replacement",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="replacing",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="suspended",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="to be emptied",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="unknown",storagepool="system"} 0
		
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="being emptied",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="emptied",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="ready",storagepool="system"} 1
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="replacement",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="replacing",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="suspended",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="to be emptied",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="unknown",storagepool="system"} 0

        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="being emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="ready",storagepool="data"} 1
        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="replacement",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="replacing",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="suspended",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="to be emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="unknown",storagepool="data"} 0

        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="being emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="ready",storagepool="data"} 1
        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="replacement",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="replacing",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="suspended",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="to be emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="unknown",storagepool="data"} 0

        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="being emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="ready",storagepool="data"} 1
        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="replacement",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="replacing",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="suspended",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="to be emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="unknown",storagepool="data"} 0

        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="being emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="ready",storagepool="data"} 1
        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="replacement",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="replacing",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="suspended",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="to be emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="unknown",storagepool="data"} 0

        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="being emptied",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="emptied",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="ready",storagepool="ess5000"} 1
        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="replacement",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="replacing",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="suspended",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="to be emptied",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="unknown",storagepool="ess5000"} 0

        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="being emptied",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="emptied",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="ready",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="replacement",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="replacing",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="suspended",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="to be emptied",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="unknown",storagepool="ess5000"} 1
	`
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	collector := NewMmlsdiskCollector(logger)
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 107 {
		t.Errorf("Unexpected collection count %d, expected 107", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_disk_availability", "gpfs_disk_status"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmlsdiskCollectorMmlsfs(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	MmlsdiskExec = func(fs string, ctx context.Context) (string, error) {
		return mmlsdiskStdout, nil
	}
	mmlsfsStdout = `
		fs::HEADER:version:reserved:reserved:deviceName:fieldName:data:remarks:
		mmlsfs::0:1:::mmfs1:defaultMountPoint:%2Ffs%mmfs1::
	`
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return mmlsfsStdout, nil
	}
	expected := `
		# HELP gpfs_disk_availability GPFS disk availability
        # TYPE gpfs_disk_availability gauge
        gpfs_disk_availability{availability="down",data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",storagepool="system"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",storagepool="system"} 1
        gpfs_disk_availability{availability="recovering",data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",storagepool="system"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",storagepool="system"} 0
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",storagepool="system"} 0

        gpfs_disk_availability{availability="down",data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",storagepool="system"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",storagepool="system"} 1
        gpfs_disk_availability{availability="recovering",data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",storagepool="system"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",storagepool="system"} 0
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",storagepool="system"} 0

        gpfs_disk_availability{availability="down",data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",storagepool="data"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",storagepool="data"} 1
        gpfs_disk_availability{availability="recovering",data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",storagepool="data"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",storagepool="data"} 0
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",storagepool="data"} 0

        gpfs_disk_availability{availability="down",data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",storagepool="data"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",storagepool="data"} 1
        gpfs_disk_availability{availability="recovering",data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",storagepool="data"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",storagepool="data"} 0
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",storagepool="data"} 0

        gpfs_disk_availability{availability="down",data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",storagepool="data"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",storagepool="data"} 1
        gpfs_disk_availability{availability="recovering",data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",storagepool="data"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",storagepool="data"} 0
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",storagepool="data"} 0

        gpfs_disk_availability{availability="down",data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",storagepool="data"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",storagepool="data"} 1
        gpfs_disk_availability{availability="recovering",data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",storagepool="data"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",storagepool="data"} 0
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",storagepool="data"} 0

        gpfs_disk_availability{availability="down",data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",storagepool="ess5000"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",storagepool="ess5000"} 1
        gpfs_disk_availability{availability="recovering",data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",storagepool="ess5000"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",storagepool="ess5000"} 0
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",storagepool="ess5000"} 0

        gpfs_disk_availability{availability="down",data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",storagepool="ess5000"} 0
        gpfs_disk_availability{availability="up",data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",storagepool="ess5000"} 0
        gpfs_disk_availability{availability="recovering",data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",storagepool="ess5000"} 0
        gpfs_disk_availability{availability="unknown",data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",storagepool="ess5000"} 1
        gpfs_disk_availability{availability="unrecovered",data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",storagepool="ess5000"} 0
        # HELP gpfs_disk_status GPFS disk status
        # TYPE gpfs_disk_status gauge
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="being emptied",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="emptied",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="ready",storagepool="system"} 1
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="replacement",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="replacing",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="suspended",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="to be emptied",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="1",fs="ess",metadata="yes",name="RG001VS003",status="unknown",storagepool="system"} 0
		
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="being emptied",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="emptied",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="ready",storagepool="system"} 1
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="replacement",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="replacing",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="suspended",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="to be emptied",storagepool="system"} 0
        gpfs_disk_status{data="yes",diskid="2",fs="ess",metadata="yes",name="RG002VS003",status="unknown",storagepool="system"} 0

        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="being emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="ready",storagepool="data"} 1
        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="replacement",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="replacing",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="suspended",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="to be emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="3",fs="ess",metadata="no",name="RG003VS004",status="unknown",storagepool="data"} 0

        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="being emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="ready",storagepool="data"} 1
        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="replacement",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="replacing",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="suspended",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="to be emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="4",fs="ess",metadata="no",name="RG004VS004",status="unknown",storagepool="data"} 0

        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="being emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="ready",storagepool="data"} 1
        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="replacement",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="replacing",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="suspended",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="to be emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="5",fs="ess",metadata="no",name="RG005VS005",status="unknown",storagepool="data"} 0

        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="being emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="ready",storagepool="data"} 1
        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="replacement",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="replacing",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="suspended",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="to be emptied",storagepool="data"} 0
        gpfs_disk_status{data="yes",diskid="6",fs="ess",metadata="no",name="RG006VS005",status="unknown",storagepool="data"} 0

        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="being emptied",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="emptied",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="ready",storagepool="ess5000"} 1
        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="replacement",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="replacing",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="suspended",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="to be emptied",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="7",fs="ess",metadata="no",name="RG007VS009",status="unknown",storagepool="ess5000"} 0

        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="being emptied",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="emptied",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="ready",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="replacement",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="replacing",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="suspended",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="to be emptied",storagepool="ess5000"} 0
        gpfs_disk_status{data="yes",diskid="8",fs="ess",metadata="no",name="RG008VS010",status="unknown",storagepool="ess5000"} 1
	`
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	collector := NewMmlsdiskCollector(logger)
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 107 {
		t.Errorf("Unexpected collection count %d, expected 107", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_disk_availability", "gpfs_disk_status"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmlsdiskCollectorError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "mmfs1"
	diskFilesystems = &filesystems
	MmlsdiskExec = func(fs string, ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmlsdisk-mmfs1"} 1
	`
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	collector := NewMmlsdiskCollector(logger)
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

func TestMmlsdiskCollectorTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "mmfs1"
	diskFilesystems = &filesystems
	MmlsdiskExec = func(fs string, ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmlsdisk-mmfs1"} 1
	`
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	collector := NewMmlsdiskCollector(logger)
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

func TestMmlsdiskCollectorMmlsfsError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystem := ""
	diskFilesystems = &filesystem
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmlsdisk-mmlsfs"} 1
	`
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	collector := NewMmlsdiskCollector(logger)
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

func TestMmlsdiskCollectorMmlsfsTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystem := ""
	diskFilesystems = &filesystem
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmlsdisk-mmlsfs"} 1
	`
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	collector := NewMmlsdiskCollector(logger)
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
