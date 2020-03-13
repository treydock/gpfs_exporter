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
	"os/exec"
	"strings"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"gopkg.in/alecthomas/kingpin.v2"
)

func TestParseMmgetstate(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = `
mmgetstate::HEADER:version:reserved:reserved:nodeName:nodeNumber:state:quorum:nodesUp:totalNodes:remarks:cnfsState:
mmgetstate::0:1:::ib-proj-nsd05.domain:11:active:4:7:1122::(undefined):
`
	defer func() { execCommand = exec.CommandContext }()
	metric, err := mmgetstate_parse(mockedStdout)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if val := metric.state; val != "active" {
		t.Errorf("Unexpected state got %s", val)
	}
}

func TestMmgetstateCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{"--exporter.use-cache"}); err != nil {
		t.Fatal(err)
	}
	stdout := `
mmgetstate::HEADER:version:reserved:reserved:nodeName:nodeNumber:state:quorum:nodesUp:totalNodes:remarks:cnfsState:
mmgetstate::0:1:::ib-proj-nsd05.domain:11:active:4:7:1122::(undefined):
`
	mmgetstateExec = func(ctx context.Context) (string, error) {
		return stdout, nil
	}
	metadata := `
		# HELP gpfs_state GPFS state
		# TYPE gpfs_state gauge`
	expected := `
		gpfs_state{state="active"} 1
		gpfs_state{state="arbitrating"} 0
		gpfs_state{state="down"} 0
		gpfs_state{state="unknown"} 0
	`
	collector := NewMmgetstateCollector(log.NewNopLogger(), false)
	gatherers := setupGatherer(collector)
	if val := testutil.CollectAndCount(collector); val != 7 {
		t.Errorf("Unexpected collection count %d, expected 7", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(metadata+expected), "gpfs_state"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}
