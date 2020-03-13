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
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	mockedExitStatus = 0
	mockedStdout     string
	_, cancel        = context.WithTimeout(context.Background(), 5*time.Second)
)

func fakeExecCommand(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestExecCommandHelper", "--", command}
	cs = append(cs, args...)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	es := strconv.Itoa(mockedExitStatus)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1",
		"STDOUT=" + mockedStdout,
		"EXIT_STATUS=" + es}
	return cmd
}

func TestExecCommandHelper(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	//nolint:staticcheck
	fmt.Fprintf(os.Stdout, os.Getenv("STDOUT"))
	i, _ := strconv.Atoi(os.Getenv("EXIT_STATUS"))
	os.Exit(i)
}

func setupGatherer(collector Collector) prometheus.Gatherer {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	gatherers := prometheus.Gatherers{registry}
	return gatherers
}

func TestParseMmlsfs(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = `
fs::HEADER:version:reserved:reserved:deviceName:fieldName:data:remarks:
mmlsfs::0:1:::project:defaultMountPoint:%2Ffs%2Fproject::
mmlsfs::0:1:::scratch:defaultMountPoint:%2Ffs%2Fscratch::
mmlsfs::0:1:::ess:defaultMountPoint:%2Ffs%2Fess::
`
	defer func() { execCommand = exec.CommandContext }()
	filesystems, err := parse_mmlsfs(mockedStdout)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if len(filesystems) != 3 {
		t.Errorf("Expected 3 perfs returned, got %d", len(filesystems))
		return
	}
	if val := filesystems[0].Name; val != "project" {
		t.Errorf("Unexpected Name, got %v", val)
	}
	if val := filesystems[0].Mountpoint; val != "/fs/project" {
		t.Errorf("Unexpected Mounpoint, got %v", val)
	}
}
