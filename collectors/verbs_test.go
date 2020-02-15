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
	"os/exec"
	"testing"
)

func TestParseVerbsDisabled(t *testing.T) {
	execCommand = fakeExecCommand
	mockedStdout = `
VERBS RDMA status: disabled
`
	defer func() { execCommand = exec.Command }()
	metric, err := verbs_parse(mockedStdout)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if metric.Status != "disabled" {
		t.Errorf("Unexpected value for status, expected disabled, got %s", metric.Status)
	}
}

func TestParseVerbsStarted(t *testing.T) {
	execCommand = fakeExecCommand
	mockedStdout = `
VERBS RDMA status: started
`
	defer func() { execCommand = exec.Command }()
	metric, err := verbs_parse(mockedStdout)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if metric.Status != "started" {
		t.Errorf("Unexpected value for status, expected started, got %s", metric.Status)
	}
}
