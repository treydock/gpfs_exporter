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
	mmlsqosStdout = `
mmlsqos:status:HEADER:version:reserved:reserved:enabled:throttling:monitoring:fineStatsSecs:idStats:
mmlsqos:config:HEADER:version:reserved:reserved:config_enc:
mmlsqos:values:HEADER:version:reserved:reserved:values_enc:
mmlsqos:stats:HEADER:version:reserved:reserved:pool:timeEpoch:class:iops:ioql:qsdl:et:MBs:
mmlsqos:status:0:1:::Yes:Yes:Yes:0:No:
mmlsqos:config:0:1:::pool=sas1,other=inf,maintenance/all_local=50000Iops:
mmlsqos:values:0:1:::pool=system,other=inf,maintenance/all_local=inf%3Apool=sas1,other=inf,maintenance/all_local=50000Iops%3Apool=sata1,other=inf,maintenance/all_local=inf%3Apool=sas2,other=inf,maintenance/all_local=inf%3Apool=nvme1,other=inf,maintenance/all_local=inf:
mmlsqos:stats:0:1:::nvme1:1678438680:misc:33,267:0,013449:1,0751e-05:30:4.675:
mmlsqos:stats:0:1:::nvme1:1678438680:other:829,83:0,85256:77349065,73251:30:1525.5:
mmlsqos:stats:0:1:::system:1678438680:misc:24875:1,7781e+08:0,0055852:30:212.95:
mmlsqos:stats:0:1:::system:1678438680:other:35545:41,399:1,9398e+08:30:149.76:
mmlsqos:stats:0:1:::system:1678438680:maintenance:0,066667:5,579e-05:0,00000:30:0.00026042:
`
	mmlsqosStdoutNanValue = `
mmlsqos:status:HEADER:version:reserved:reserved:enabled:throttling:monitoring:fineStatsSecs:idStats:
mmlsqos:config:HEADER:version:reserved:reserved:config_enc:
mmlsqos:values:HEADER:version:reserved:reserved:values_enc:
mmlsqos:stats:HEADER:version:reserved:reserved:pool:timeEpoch:class:iops:ioql:qsdl:et:MBs:
mmlsqos:status:0:1:::Yes:Yes:Yes:0:No:
mmlsqos:config:0:1:::pool=sas1,other=inf,maintenance/all_local=50000Iops:
mmlsqos:values:0:1:::pool=system,other=inf,maintenance/all_local=inf%3Apool=sas1,other=inf,maintenance/all_local=50000Iops%3Apool=sata1,other=inf,maintenance/all_local=inf%3Apool=sas2,other=inf,maintenance/all_local=inf%3Apool=nvme1,other=inf,maintenance/all_local=inf:
mmlsqos:stats:0:1:::nvme1:1678438680:misc:33,267:0,013449:nan:30:4.675:
mmlsqos:stats:0:1:::nvme1:1678438680:other:829,83:0,85256:0,00019576:30:1525.5:
`
	mmlsqosStdoutBadTime = `
mmlsqos:status:HEADER:version:reserved:reserved:enabled:throttling:monitoring:fineStatsSecs:idStats:
mmlsqos:config:HEADER:version:reserved:reserved:config_enc:
mmlsqos:values:HEADER:version:reserved:reserved:values_enc:
mmlsqos:stats:HEADER:version:reserved:reserved:pool:timeEpoch:class:iops:ioql:qsdl:et:MBs:
mmlsqos:status:0:1:::Yes:Yes:Yes:0:No:
mmlsqos:config:0:1:::pool=sas1,other=inf,maintenance/all_local=50000Iops:
mmlsqos:values:0:1:::pool=system,other=inf,maintenance/all_local=inf%3Apool=sas1,other=inf,maintenance/all_local=50000Iops%3Apool=sata1,other=inf,maintenance/all_local=inf%3Apool=sas2,other=inf,maintenance/all_local=inf%3Apool=nvme1,other=inf,maintenance/all_local=inf:
mmlsqos:stats:0:1:::nvme1:foo:misc:33,267:0,013449:1,0751e-05:30:4.675:
mmlsqos:stats:0:1:::nvme1:1678438680:other:829,83:0,85256:0,00019576:30:1525.5:
`
	mmlsqosStdoutBadValue = `
mmlsqos:status:HEADER:version:reserved:reserved:enabled:throttling:monitoring:fineStatsSecs:idStats:
mmlsqos:config:HEADER:version:reserved:reserved:config_enc:
mmlsqos:values:HEADER:version:reserved:reserved:values_enc:
mmlsqos:stats:HEADER:version:reserved:reserved:pool:timeEpoch:class:iops:ioql:qsdl:et:MBs:
mmlsqos:status:0:1:::Yes:Yes:Yes:0:No:
mmlsqos:config:0:1:::pool=sas1,other=inf,maintenance/all_local=50000Iops:
mmlsqos:values:0:1:::pool=system,other=inf,maintenance/all_local=inf%3Apool=sas1,other=inf,maintenance/all_local=50000Iops%3Apool=sata1,other=inf,maintenance/all_local=inf%3Apool=sas2,other=inf,maintenance/all_local=inf%3Apool=nvme1,other=inf,maintenance/all_local=inf:
mmlsqos:stats:0:1:::nvme1:1678438680:misc:33,267:0,013449:foo:30:4.675:
mmlsqos:stats:0:1:::nvme1:1678438680:other:829,83:0,85256:0,00019576:30:1525.5:
`
)

func TestMmlsqos(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmlsqos("test", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if out != mockedStdout {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmlsqosError(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := mmlsqos("test", ctx)
	if err == nil {
		t.Errorf("Expected error")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestMmlsqosTimeout(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 1
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
	defer cancel()
	out, err := mmlsqos("test", ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded")
	}
	if out != "" {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestParseMmlsqos(t *testing.T) {
	metrics, err := parse_mmlsqos(mmlsqosStdout, log.NewNopLogger())
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if len(metrics) != 5 {
		t.Errorf("Unexpected number of metrics, got %d", len(metrics))
		return
	}
	if metrics[0].Pool != "nvme1" {
		t.Errorf("Unexpected value for Pool, got %v", metrics[0].Pool)
	}
	if metrics[0].Time != 1678438680 {
		t.Errorf("Unexpected value for Time, got %v", metrics[0].Time)
	}
	if metrics[0].Class != "misc" {
		t.Errorf("Unexpected value for Class, got %v", metrics[0].Class)
	}
	if metrics[0].Iops != 33.267 {
		t.Errorf("Unexpected value for Iops, got %v", metrics[0].Iops)
	}
	if metrics[0].Ioql != 0.013449 {
		t.Errorf("Unexpected value for Ioql, got %v", metrics[0].Ioql)
	}
	if metrics[0].Qsdl != 1.0751e-05 {
		t.Errorf("Unexpected value for Qsdl, got %v", metrics[0].Qsdl)
	}
	if metrics[0].ET != 30 {
		t.Errorf("Unexpected value for ET, got %v", metrics[0].ET)
	}
	if metrics[0].MBs != 4.675 {
		t.Errorf("Unexpected value for MBs, got %v", metrics[0].MBs)
	}
	metrics, err = parse_mmlsqos(mmlsqosStdoutNanValue, log.NewNopLogger())
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if metrics[0].Qsdl != 0 {
		t.Errorf("Unexpected value for Qsdl, got %v", metrics[0].Qsdl)
	}
}

func TestParseMmlsqosErrors(t *testing.T) {
	_, err := parse_mmlsqos(mmlsqosStdoutBadTime, log.NewNopLogger())
	if err == nil {
		t.Errorf("Expected error")
		return
	}
	_, err = parse_mmlsqos(mmlsqosStdoutBadValue, log.NewNopLogger())
	if err == nil {
		t.Errorf("Expected error")
		return
	}
}

func TestMmlsqosCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "mmfs1"
	qosFilesystems = &filesystems
	MmlsqosExec = func(fs string, ctx context.Context) (string, error) {
		return mmlsqosStdout, nil
	}
	expected := `
		# HELP gpfs_qos_epoch_timestamp_seconds GPFS measurement period ends timestamp in seconds
		# TYPE gpfs_qos_epoch_timestamp_seconds gauge
		gpfs_qos_epoch_timestamp_seconds{class="maintenance",fs="mmfs1",pool="system"} 1.67843868e+09
		gpfs_qos_epoch_timestamp_seconds{class="misc",fs="mmfs1",pool="nvme1"} 1.67843868e+09
		gpfs_qos_epoch_timestamp_seconds{class="misc",fs="mmfs1",pool="system"} 1.67843868e+09
		gpfs_qos_epoch_timestamp_seconds{class="other",fs="mmfs1",pool="nvme1"} 1.67843868e+09
		gpfs_qos_epoch_timestamp_seconds{class="other",fs="mmfs1",pool="system"} 1.67843868e+09
		# HELP gpfs_qos_et_seconds GPFS interval in seconds during which the measurement was made
		# TYPE gpfs_qos_et_seconds gauge
		gpfs_qos_et_seconds{class="maintenance",fs="mmfs1",pool="system"} 30
		gpfs_qos_et_seconds{class="misc",fs="mmfs1",pool="nvme1"} 30
		gpfs_qos_et_seconds{class="misc",fs="mmfs1",pool="system"} 30
		gpfs_qos_et_seconds{class="other",fs="mmfs1",pool="nvme1"} 30
		gpfs_qos_et_seconds{class="other",fs="mmfs1",pool="system"} 30
		# HELP gpfs_qos_iops GPFS performance of the class in I/O operations per second
		# TYPE gpfs_qos_iops gauge
		gpfs_qos_iops{class="maintenance",fs="mmfs1",pool="system"} 0.066667
		gpfs_qos_iops{class="misc",fs="mmfs1",pool="nvme1"} 33.267
		gpfs_qos_iops{class="misc",fs="mmfs1",pool="system"} 24875
		gpfs_qos_iops{class="other",fs="mmfs1",pool="nvme1"} 829.83
		gpfs_qos_iops{class="other",fs="mmfs1",pool="system"} 35545
		# HELP gpfs_qos_ioql GPFS average number of I/O requests in the class that are pending for reasons other than being queued by QoS
		# TYPE gpfs_qos_ioql gauge
		gpfs_qos_ioql{class="maintenance",fs="mmfs1",pool="system"} 5.579e-05
		gpfs_qos_ioql{class="misc",fs="mmfs1",pool="nvme1"} 0.013449
		gpfs_qos_ioql{class="misc",fs="mmfs1",pool="system"} 1.7781e+08
		gpfs_qos_ioql{class="other",fs="mmfs1",pool="nvme1"} 0.85256
		gpfs_qos_ioql{class="other",fs="mmfs1",pool="system"} 41.399
		# HELP gpfs_qos_mbs GPFS performance of the class in MegaBytes per second
		# TYPE gpfs_qos_mbs gauge
		gpfs_qos_mbs{class="maintenance",fs="mmfs1",pool="system"} 0.00026042
		gpfs_qos_mbs{class="misc",fs="mmfs1",pool="nvme1"} 4.675
		gpfs_qos_mbs{class="misc",fs="mmfs1",pool="system"} 212.95
		gpfs_qos_mbs{class="other",fs="mmfs1",pool="nvme1"} 1525.5
		gpfs_qos_mbs{class="other",fs="mmfs1",pool="system"} 149.76
		# HELP gpfs_qos_qsdl GPFS average number of I/O requests in the class that are queued by QoS
		# TYPE gpfs_qos_qsdl gauge
		gpfs_qos_qsdl{class="maintenance",fs="mmfs1",pool="system"} 0
		gpfs_qos_qsdl{class="misc",fs="mmfs1",pool="nvme1"} 1.0751e-05
		gpfs_qos_qsdl{class="misc",fs="mmfs1",pool="system"} 0.0055852
		gpfs_qos_qsdl{class="other",fs="mmfs1",pool="nvme1"} 7.734906573251e+07
		gpfs_qos_qsdl{class="other",fs="mmfs1",pool="system"} 1.9398e+08
	`
	collector := NewMmlsqosCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 33 {
		t.Errorf("Unexpected collection count %d, expected 33", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_qos_epoch_timestamp_seconds", "gpfs_qos_et_seconds",
		"gpfs_qos_iops", "gpfs_qos_ioql", "gpfs_qos_mbs", "gpfs_qos_qsdl"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmlsqosCollectorMmlsfs(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	MmlsqosExec = func(fs string, ctx context.Context) (string, error) {
		return mmlsqosStdout, nil
	}
	mmlsfsStdout = `
		fs::HEADER:version:reserved:reserved:deviceName:fieldName:data:remarks:
		mmlsfs::0:1:::mmfs1:defaultMountPoint:%2Ffs%mmfs1::
	`
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return mmlsfsStdout, nil
	}
	expected := `
		# HELP gpfs_qos_epoch_timestamp_seconds GPFS measurement period ends timestamp in seconds
		# TYPE gpfs_qos_epoch_timestamp_seconds gauge
		gpfs_qos_epoch_timestamp_seconds{class="maintenance",fs="mmfs1",pool="system"} 1.67843868e+09
		gpfs_qos_epoch_timestamp_seconds{class="misc",fs="mmfs1",pool="nvme1"} 1.67843868e+09
		gpfs_qos_epoch_timestamp_seconds{class="misc",fs="mmfs1",pool="system"} 1.67843868e+09
		gpfs_qos_epoch_timestamp_seconds{class="other",fs="mmfs1",pool="nvme1"} 1.67843868e+09
		gpfs_qos_epoch_timestamp_seconds{class="other",fs="mmfs1",pool="system"} 1.67843868e+09
		# HELP gpfs_qos_et_seconds GPFS interval in seconds during which the measurement was made
		# TYPE gpfs_qos_et_seconds gauge
		gpfs_qos_et_seconds{class="maintenance",fs="mmfs1",pool="system"} 30
		gpfs_qos_et_seconds{class="misc",fs="mmfs1",pool="nvme1"} 30
		gpfs_qos_et_seconds{class="misc",fs="mmfs1",pool="system"} 30
		gpfs_qos_et_seconds{class="other",fs="mmfs1",pool="nvme1"} 30
		gpfs_qos_et_seconds{class="other",fs="mmfs1",pool="system"} 30
		# HELP gpfs_qos_iops GPFS performance of the class in I/O operations per second
		# TYPE gpfs_qos_iops gauge
		gpfs_qos_iops{class="maintenance",fs="mmfs1",pool="system"} 0.066667
		gpfs_qos_iops{class="misc",fs="mmfs1",pool="nvme1"} 33.267
		gpfs_qos_iops{class="misc",fs="mmfs1",pool="system"} 24875
		gpfs_qos_iops{class="other",fs="mmfs1",pool="nvme1"} 829.83
		gpfs_qos_iops{class="other",fs="mmfs1",pool="system"} 35545
		# HELP gpfs_qos_ioql GPFS average number of I/O requests in the class that are pending for reasons other than being queued by QoS
		# TYPE gpfs_qos_ioql gauge
		gpfs_qos_ioql{class="maintenance",fs="mmfs1",pool="system"} 5.579e-05
		gpfs_qos_ioql{class="misc",fs="mmfs1",pool="nvme1"} 0.013449
		gpfs_qos_ioql{class="misc",fs="mmfs1",pool="system"} 1.7781e+08
		gpfs_qos_ioql{class="other",fs="mmfs1",pool="nvme1"} 0.85256
		gpfs_qos_ioql{class="other",fs="mmfs1",pool="system"} 41.399
		# HELP gpfs_qos_mbs GPFS performance of the class in MegaBytes per second
		# TYPE gpfs_qos_mbs gauge
		gpfs_qos_mbs{class="maintenance",fs="mmfs1",pool="system"} 0.00026042
		gpfs_qos_mbs{class="misc",fs="mmfs1",pool="nvme1"} 4.675
		gpfs_qos_mbs{class="misc",fs="mmfs1",pool="system"} 212.95
		gpfs_qos_mbs{class="other",fs="mmfs1",pool="nvme1"} 1525.5
		gpfs_qos_mbs{class="other",fs="mmfs1",pool="system"} 149.76
		# HELP gpfs_qos_qsdl GPFS average number of I/O requests in the class that are queued by QoS
		# TYPE gpfs_qos_qsdl gauge
		gpfs_qos_qsdl{class="maintenance",fs="mmfs1",pool="system"} 0
		gpfs_qos_qsdl{class="misc",fs="mmfs1",pool="nvme1"} 1.0751e-05
		gpfs_qos_qsdl{class="misc",fs="mmfs1",pool="system"} 0.0055852
		gpfs_qos_qsdl{class="other",fs="mmfs1",pool="nvme1"} 7.734906573251e+07
		gpfs_qos_qsdl{class="other",fs="mmfs1",pool="system"} 1.9398e+08
	`
	collector := NewMmlsqosCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 33 {
		t.Errorf("Unexpected collection count %d, expected 33", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"gpfs_qos_epoch_timestamp_seconds", "gpfs_qos_et_seconds",
		"gpfs_qos_iops", "gpfs_qos_ioql", "gpfs_qos_mbs", "gpfs_qos_qsdl"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMmlsqosCollectorError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "mmfs1"
	configFilesystems = &filesystems
	MmlsqosExec = func(fs string, ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmlsqos-mmfs1"} 1
	`
	collector := NewMmlsqosCollector(log.NewNopLogger())
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

func TestMmlsqosCollectorTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystems := "mmfs1"
	configFilesystems = &filesystems
	MmlsqosExec = func(fs string, ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmlsqos-mmfs1"} 1
	`
	collector := NewMmlsqosCollector(log.NewNopLogger())
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

func TestMmlsqosCollectorMmlsfsError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystem := ""
	qosFilesystems = &filesystem
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="mmlsqos-mmlsfs"} 1
	`
	collector := NewMmlsqosCollector(log.NewNopLogger())
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

func TestMmlsqosCollectorMmlsfsTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	filesystem := ""
	qosFilesystems = &filesystem
	MmlsfsExec = func(ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="mmlsqos-mmlsfs"} 1
	`
	collector := NewMmlsqosCollector(log.NewNopLogger())
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
