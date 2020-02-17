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

func TestParseMmdiagWaiters(t *testing.T) {
	execCommand = fakeExecCommand
	mockedStdout = `

=== mmdiag: waiters ===
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
	defer func() { execCommand = exec.Command }()
	var metric DiagMetric
	err := parse_mmdiag_waiters(mockedStdout, &metric)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if val := len(metric.Waiters); val != 19 {
		t.Errorf("Unexpected Waiters len got %v", val)
	}
}
