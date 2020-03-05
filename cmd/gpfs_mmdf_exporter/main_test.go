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
	kingpin "gopkg.in/alecthomas/kingpin.v2"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if _, err := kingpin.CommandLine.Parse([]string{"--output=/dne"}); err != nil {
		os.Exit(1)
	}
	exitVal := m.Run()
	os.Exit(exitVal)
}

func TestCollect(t *testing.T) {
}
