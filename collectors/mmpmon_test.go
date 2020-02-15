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

func TestParsePerf(t *testing.T) {
	execCommand = fakeExecCommand
	mockedStdout = `
_fs_io_s_ _n_ 10.22.0.106 _nn_ ib-pitzer-rw02.ten _rc_ 0 _t_ 1579358234 _tu_ 53212 _cl_ gpfs.osc.edu _fs_ scratch _d_ 48 _br_ 205607400434 _bw_ 74839282351 _oc_ 2377656 _cc_ 2201576 _rdc_ 59420404 _wc_ 18874626 _dir_ 40971 _iu_ 544768
_fs_io_s_ _n_ 10.22.0.106 _nn_ ib-pitzer-rw02.ten _rc_ 0 _t_ 1579358234 _tu_ 53212 _cl_ gpfs.osc.edu _fs_ project _d_ 96 _br_ 0 _bw_ 0 _oc_ 513 _cc_ 513 _rdc_ 0 _wc_ 0 _dir_ 0 _iu_ 169
`
	defer func() { execCommand = exec.Command }()
	perfs, err := mmpmon_parse(mockedStdout)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if len(perfs) != 2 {
		t.Errorf("Expected 2 perfs returned, got %d", len(perfs))
		return
	}
	if val := perfs[0].FS; val != "scratch" {
		t.Errorf("Unexpected FS got %s", val)
	}
	if val := perfs[1].FS; val != "project" {
		t.Errorf("Unexpected FS got %s", val)
	}
	if val := perfs[0].NodeName; val != "ib-pitzer-rw02.ten" {
		t.Errorf("Unexpected NodeName got %s", val)
	}
	if val := perfs[1].NodeName; val != "ib-pitzer-rw02.ten" {
		t.Errorf("Unexpected NodeName got %s", val)
	}
	if val := perfs[0].ReadBytes; val != 205607400434 {
		t.Errorf("Unexpected ReadBytes got %d", val)
	}
}
