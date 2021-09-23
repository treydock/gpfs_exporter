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
	"strings"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	waitersStdout = `
mmdiag:waiters:HEADER:version:reserved:reserved:threadId:threadAddr:threadName:waitStartTime:waitTime:isMonitored:condVarAddr:condVarName:condVarReason:mutexAddr:mutexName:auxReason:delayTime:delayReason:
mmdiag:waiters:0:1:::101445:00000000F57FC500:FsckClientReaperThread:2021-09-23_15%3A31%3A33-0400:6861.7395:monitored::::::reason 'Waiting to reap fsck pointer:::
mmdiag:waiters:0:1:::101445:00000000F57FC500:EventsExporterSenderThread:2021-09-23_15%3A31%3A33-0400:44.3:monitored::::::for poll on sock 1379:::
mmdiag:waiters:0:1:::101445:00000000F57FC500:RebuildWorkThread:2021-09-23_15%3A31%3A33-0400:64.3:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::101445:00000000F57FC500:RebuildWorkThread:2021-09-23_15%3A31%3A33-0400:44.3:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::101445:00000000F57FC500:RebuildWorkThread:2021-09-23_15%3A31%3A33-0400:44.3:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::101445:00000000F57FC500:RebuildWorkThread:2021-09-23_15%3A31%3A33-0400:0.3897:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::137940:000000001401AE50:RebuildWorkThread:2021-09-23_15%3A31%3A33-0400:0.2919:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::101392:00000000F57EF6C0:RebuildWorkThread:2021-09-23_15%3A31%3A33-0400:0.2234:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::127780:00000000ED808950:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.1872:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::61817:000000001C02CA80:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.1592:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::47037:0000000088029CD0:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.1491:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::64102:0000000020097320:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.1428:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::47035:0000000088029490:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.1336:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::128451:0000000064004E20:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.1053:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::131854:000000006C00B3E0:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.0918:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::61815:000000001C02C240:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.0890:monitored:00003FF9FFD6D8C0:VdiskPGDrainCondvar:waiting for PG drain::::::
mmdiag:waiters:0:1:::146753:00000000AC02DB90:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.0696:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::47032:0000000088028830:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.0547:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::128454:0000000040001C80:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.0433:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::101497:00000000F5808F20:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.0348:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::131849:000000006001EC80:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.0313:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::101482:00000000F5805980:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.0298:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::47532:0000000088AFE710:NSDThread:2021-09-23_15%3A31%3A34-0400:0.0244:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::64149:00000000200A3500:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.0196:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::48622:0000000088C16F10:NSDThread:2021-09-23_15%3A31%3A34-0400:0.0134:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::127779:000000003BFFFD40:RebuildWorkThread:2021-09-23_15%3A31%3A34-0400:0.0081:monitored::::::for I/O completion:::
mmdiag:waiters:0:1:::48081:0000000088B8BFB0:NSDThread:2021-09-23_15%3A31%3A34-0400:0.0037:monitored::::::for I/O completion:::
`
)

func TestParseMmdiagWaiters(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)
	waiters := parse_mmdiag_waiters(waitersStdout, logger)
	if val := len(waiters); val != 25 {
		t.Errorf("Unexpected Waiters len got %v", val)
		return
	}
	if val := waiters[0].Name; val != "RebuildWorkThread" {
		t.Errorf("Unexpected name for waiter, got %s", val)
	}
	if val := waiters[0].Reason; val != "for I/O completion" {
		t.Errorf("Unexpected reason for waiter, got %s", val)
	}
	if val := waiters[0].Seconds; val != 64.3 {
		t.Errorf("Unexpected seconds for waiter, got %f", val)
	}
}

func TestWaiterCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{"--collector.waiter.log-reason"}); err != nil {
		t.Fatal(err)
	}
	MmdiagExec = func(arg string, ctx context.Context) (string, error) {
		return waitersStdout, nil
	}
	expected := `
		# HELP gpfs_waiter_seconds GPFS waiter in seconds
		# TYPE gpfs_waiter_seconds histogram
		gpfs_waiter_seconds_bucket{le="1"} 22
		gpfs_waiter_seconds_bucket{le="5"} 22
		gpfs_waiter_seconds_bucket{le="15"} 22
		gpfs_waiter_seconds_bucket{le="60"} 24
		gpfs_waiter_seconds_bucket{le="300"} 25
		gpfs_waiter_seconds_bucket{le="3600"} 25
		gpfs_waiter_seconds_bucket{le="+Inf"} 25
		gpfs_waiter_seconds_sum 155.19569999999996
		gpfs_waiter_seconds_count 25
		# HELP gpfs_waiter_info_count GPFS waiter info
		# TYPE gpfs_waiter_info_count gauge
		gpfs_waiter_info_count{waiter="NSDThread"} 3
		gpfs_waiter_info_count{waiter="RebuildWorkThread"} 22
	`
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)
	collector1 := NewWaiterCollector(logger)
	collector2 := NewWaiterCollector(logger)
	gatherers1 := setupGatherer(collector1)
	gatherers2 := setupGatherer(collector2)
	if val, err := testutil.GatherAndCount(gatherers1); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 6 {
		t.Errorf("Unexpected collection count %d, expected 6", val)
	}
	if err := testutil.GatherAndCompare(gatherers2, strings.NewReader(expected),
		"gpfs_waiter_seconds", "gpfs_waiter_info_count"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestWaiterCollectorError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	MmdiagExec = func(arg string, ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
		# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
		# TYPE gpfs_exporter_collect_error gauge
		gpfs_exporter_collect_error{collector="waiter"} 1
	`
	collector := NewWaiterCollector(log.NewNopLogger())
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

func TestWaiterCollectorTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	MmdiagExec = func(arg string, ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
		# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
		# TYPE gpfs_exporter_collect_timeout gauge
		gpfs_exporter_collect_timeout{collector="waiter"} 1
	`
	collector := NewWaiterCollector(log.NewNopLogger())
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
