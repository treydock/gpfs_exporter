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

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/common/model"
	"github.com/treydock/gpfs_exporter/collectors"
)

var (
	outputPath         string
	mmlssnapshotStdout = `
mmlssnapshot::HEADER:version:reserved:reserved:filesystemName:directory:snapID:status:created:quotas:data:metadata:fileset:snapType:
mmlssnapshot::0:1:::ess:20210120:27107:Valid:Wed Jan 20 00%3A30%3A02 2021::823587352320:529437984:::
mmlssnapshot::0:1:::ess:20201115_PAS1736:16337:Valid:Sun Nov 15 02%3A47%3A48 2020::0:205184:PAS1736::
`
	expected = `
# HELP gpfs_snapshot_created_timestamp_seconds GPFS snapshot creation timestamp
# TYPE gpfs_snapshot_created_timestamp_seconds gauge
gpfs_snapshot_created_timestamp_seconds{fileset="",fs="ess",id="27107",snapshot="20210120"} 1.611120602e+09
gpfs_snapshot_created_timestamp_seconds{fileset="PAS1736",fs="ess",id="16337",snapshot="20201115_PAS1736"} 1.605426468e+09
# HELP gpfs_snapshot_data_size_bytes GPFS snapshot data size
# TYPE gpfs_snapshot_data_size_bytes gauge
gpfs_snapshot_data_size_bytes{fileset="",fs="ess",id="27107",snapshot="20210120"} 8.4335344877568e+14
gpfs_snapshot_data_size_bytes{fileset="PAS1736",fs="ess",id="16337",snapshot="20201115_PAS1736"} 0
# HELP gpfs_snapshot_metadata_size_bytes GPFS snapshot metadata size
# TYPE gpfs_snapshot_metadata_size_bytes gauge
gpfs_snapshot_metadata_size_bytes{fileset="",fs="ess",id="27107",snapshot="20210120"} 5.42144495616e+11
gpfs_snapshot_metadata_size_bytes{fileset="PAS1736",fs="ess",id="16337",snapshot="20201115_PAS1736"} 2.10108416e+08
# HELP gpfs_snapshot_status_info GPFS snapshot status
# TYPE gpfs_snapshot_status_info gauge
gpfs_snapshot_status_info{fileset="",fs="ess",id="27107",snapshot="20210120",status="Valid"} 1
gpfs_snapshot_status_info{fileset="PAS1736",fs="ess",id="16337",snapshot="20201115_PAS1736",status="Valid"} 1`
	expectedNoError = `# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
# TYPE gpfs_exporter_collect_error gauge
gpfs_exporter_collect_error{collector="mmlssnapshot-ess"} 0
# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
# TYPE gpfs_exporter_collect_timeout gauge
gpfs_exporter_collect_timeout{collector="mmlssnapshot-ess"} 0`
	expectedError = `# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
# TYPE gpfs_exporter_collect_error gauge
gpfs_exporter_collect_error{collector="mmlssnapshot-ess"} 1
# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
# TYPE gpfs_exporter_collect_timeout gauge
gpfs_exporter_collect_timeout{collector="mmlssnapshot-ess"} 0`
	expectedTimeout = `# HELP gpfs_exporter_collect_error Indicates if error has occurred during collection
# TYPE gpfs_exporter_collect_error gauge
gpfs_exporter_collect_error{collector="mmlssnapshot-ess"} 0
# HELP gpfs_exporter_collect_timeout Indicates the collector timed out
# TYPE gpfs_exporter_collect_timeout gauge
gpfs_exporter_collect_timeout{collector="mmlssnapshot-ess"} 1`
)

func TestMain(m *testing.M) {
	model.SetNameValidationScheme(model.LegacyValidation)
	tmpDir, err := os.MkdirTemp(os.TempDir(), "output")
	if err != nil {
		os.Exit(1)
	}
	outputPath = tmpDir + "/output"
	defer os.RemoveAll(tmpDir)
	if _, err := kingpin.CommandLine.Parse([]string{fmt.Sprintf("--output=%s", outputPath), "--collector.mmlssnapshot.filesystems=ess", "--collector.mmlssnapshot.get-size"}); err != nil {
		os.Exit(1)
	}
	collectors.NowLocation = func() *time.Location {
		return time.FixedZone("EST", -5*60*60)
	}
	exitVal := m.Run()
	os.Exit(exitVal)
}

func TestCollect(t *testing.T) {
	collectors.MmlssnapshotExec = func(fs string, ctx context.Context) (string, error) {
		return mmlssnapshotStdout, nil
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := collect(logger)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if !strings.Contains(string(content), expected) {
		t.Errorf("Unexpected content:\n%s\nExpected:\n%s", string(content), expected)
	}
	if !strings.Contains(string(content), expectedNoError) {
		t.Errorf("Unexpected error metrics:\n%s\nExpected:\n%s", string(content), expectedError)
	}
}

func TestCollectError(t *testing.T) {
	collectors.MmlssnapshotExec = func(fs string, ctx context.Context) (string, error) {
		return "", fmt.Errorf("Error")
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := collect(logger)
	if err == nil {
		t.Errorf("Expected error")
		return
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if !strings.Contains(string(content), expected) {
		t.Errorf("Unexpected content:\n%s\nExpected:\n%s", string(content), expected)
	}
	if !strings.Contains(string(content), expectedError) {
		t.Errorf("Unexpected error metrics:\n%s\nExpected:\n%s", string(content), expectedError)
	}
}

func TestCollectTimeout(t *testing.T) {
	collectors.MmlssnapshotExec = func(fs string, ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := collect(logger)
	if err == nil {
		t.Errorf("Expected error")
		return
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if !strings.Contains(string(content), expected) {
		t.Errorf("Unexpected content:\n%s\nExpected:\n%s", string(content), expected)
	}
	if !strings.Contains(string(content), expectedTimeout) {
		t.Errorf("Unexpected error metrics:\n%s\nExpected:\n%s", string(content), expectedError)
	}
}
