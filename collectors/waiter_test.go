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
=== mmdiag: waiters ===
Waiting 6861.7395 sec since 11:04:20, ignored, thread 19428 FsckClientReaperThread: on ThCond 0x7F138008EC10 (FsckReaperCondvar), reason 'Waiting to reap fsck pointer'
Waiting 40.4231 sec since 13:08:39, monitored, thread 120656 EventsExporterSenderThread: for poll on sock 1379
Waiting 64.3890 sec since 17:55:45, monitored, thread 120655 NSDThread: for I/O completion
Waiting 44.3890 sec since 17:55:45, monitored, thread 120656 NSDThread: for I/O completion
Waiting 44.3890 sec since 17:55:45, monitored, thread 120657 foobar
Waiting 0.0409 sec since 10:24:00, monitored, thread 23170 NSDThread: for I/O completion
Waiting 0.0259 sec since 10:24:00, monitored, thread 23241 NSDThread: for I/O completion
Waiting 0.0251 sec since 10:24:00, monitored, thread 23243 NSDThread: for I/O completion
Waiting 0.0173 sec since 10:24:00, monitored, thread 22893 NSDThread: for I/O completion
Waiting 0.0158 sec since 10:24:00, monitored, thread 22933 NSDThread: for I/O completion
Waiting 0.0153 sec since 10:24:00, monitored, thread 22953 NSDThread: for I/O completion
Waiting 0.0143 sec since 10:24:00, monitored, thread 22978 NSDThread: for I/O completion
Waiting 0.0139 sec since 10:24:00, monitored, thread 22996 NSDThread: for I/O completion
Waiting 0.0128 sec since 10:24:00, monitored, thread 23025 NSDThread: for I/O completion
Waiting 0.0121 sec since 10:24:00, monitored, thread 23047 NSDThread: for I/O completion
Waiting 0.0114 sec since 10:24:00, monitored, thread 23075 NSDThread: for I/O completion
Waiting 0.0109 sec since 10:24:00, monitored, thread 23097 NSDThread: for I/O completion
Waiting 0.0099 sec since 10:24:00, monitored, thread 23130 NSDThread: for I/O completion
Waiting 0.0069 sec since 10:24:00, monitored, thread 23337 NSDThread: for I/O completion
Waiting 0.0063 sec since 10:24:00, monitored, thread 23227 NSDThread: for I/O completion
Waiting 0.0039 sec since 10:24:00, monitored, thread 23267 NSDThread: for I/O completion
Waiting 0.0023 sec since 10:24:00, monitored, thread 22922 NSDThread: for I/O completion
Waiting 0.0022 sec since 10:24:00, monitored, thread 22931 NSDThread: for I/O completion
Waiting 0.0002 sec since 10:24:00, monitored, thread 22987 NSDThread: for I/O completion
`
)

func TestParseMmdiagWaiters(t *testing.T) {
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)
	waiters := parse_mmdiag_waiters(waitersStdout, logger)
	if val := len(waiters); val != 22 {
		t.Errorf("Unexpected Waiters len got %v", val)
		return
	}
	if val := waiters[0].name; val != "NSDThread" {
		t.Errorf("Unexpected name for waiter, got %s", val)
	}
	if val := waiters[0].reason; val != "for I/O completion" {
		t.Errorf("Unexpected reason for waiter, got %s", val)
	}
	if val := waiters[0].seconds; val != 64.3890 {
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
		gpfs_waiter_seconds_bucket{le="1"} 19
		gpfs_waiter_seconds_bucket{le="5"} 19
		gpfs_waiter_seconds_bucket{le="15"} 19
		gpfs_waiter_seconds_bucket{le="60"} 21
		gpfs_waiter_seconds_bucket{le="300"} 22
		gpfs_waiter_seconds_bucket{le="3600"} 22
		gpfs_waiter_seconds_bucket{le="+Inf"} 22
		gpfs_waiter_seconds_sum 153.4144
		gpfs_waiter_seconds_count 22
		# HELP gpfs_waiter_info_count GPFS waiter info
		# TYPE gpfs_waiter_info_count gauge
		gpfs_waiter_info_count{waiter="NSDThread"} 21
	`
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)
	collector1 := NewWaiterCollector(logger)
	collector2 := NewWaiterCollector(logger)
	gatherers1 := setupGatherer(collector1)
	gatherers2 := setupGatherer(collector2)
	if val, err := testutil.GatherAndCount(gatherers1); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 5 {
		t.Errorf("Unexpected collection count %d, expected 5", val)
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
