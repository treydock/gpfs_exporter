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
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/treydock/gpfs_exporter/collectors"
)

const (
	address = "localhost:19303"
)

var (
	mmpmonStdout = `
_fs_io_s_ _n_ 10.22.0.106 _nn_ ib-pitzer-rw02.ten _rc_ 0 _t_ 1579358234 _tu_ 53212 _cl_ gpfs.domain _fs_ scratch _d_ 48 _br_ 205607400434 _bw_ 74839282351 _oc_ 2377656 _cc_ 2201576 _rdc_ 59420404 _wc_ 18874626 _dir_ 40971 _iu_ 544768
_fs_io_s_ _n_ 10.22.0.106 _nn_ ib-pitzer-rw02.ten _rc_ 0 _t_ 1579358234 _tu_ 53212 _cl_ gpfs.domain _fs_ project _d_ 96 _br_ 0 _bw_ 0 _oc_ 513 _cc_ 513 _rdc_ 0 _wc_ 0 _dir_ 0 _iu_ 169
`
	mmgetstateStdout = `
mmgetstate::HEADER:version:reserved:reserved:nodeName:nodeNumber:state:quorum:nodesUp:totalNodes:remarks:cnfsState:
mmgetstate::0:1:::ib-proj-nsd05.domain:11:active:4:7:1122::(undefined):
`
	configStdout = `
mmdiag:config:HEADER:version:reserved:reserved:name:value:changed:
mmdiag:config:0:1:::opensslLibName:/usr/lib64/libssl.so.10%3A/usr/lib64/libssl.so.6%3A/usr/lib64/libssl.so.0.9.8%3A/lib64/libssl.so.6%3Alibssl.so%3Alibss
l.so.0%3Alibssl.so.4%3A/lib64/libssl.so.1.0.0::
mmdiag:config:0:1:::pagepool:4294967296:static:
mmdiag:config:0:1:::pagepoolMaxPhysMemPct:75::
mmdiag:config:0:1:::parallelMetadataWrite:0::
`
)

func TestMain(m *testing.M) {

	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		os.Exit(1)
	}
	varTrue := true
	disableExporterMetrics = &varTrue
	go func() {
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		http.Handle("/metrics", metricsHandler(logger))
		err := http.ListenAndServe(address, nil)
		if err != nil {
			os.Exit(1)
		}
	}()
	time.Sleep(1 * time.Second)

	exitVal := m.Run()

	os.Exit(exitVal)
}

func TestMetricsHandler(t *testing.T) {
	collectors.MmgetstateExec = func(ctx context.Context) (string, error) {
		return mmgetstateStdout, nil
	}
	collectors.MmpmonExec = func(ctx context.Context) (string, error) {
		return mmpmonStdout, nil
	}
	collectors.MmdiagExec = func(arg string, ctx context.Context) (string, error) {
		return configStdout, nil
	}
	body, err := queryExporter()
	if err != nil {
		t.Fatalf("Unexpected error GET /metrics: %s", err.Error())
	}
	if !strings.Contains(body, "gpfs_exporter_collect_error{collector=\"mount\"} 0") {
		t.Errorf("Unexpected value for gpfs_exporter_collect_error")
	}
}

func queryExporter() (string, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", address))
	if err != nil {
		return "", err
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if err := resp.Body.Close(); err != nil {
		return "", err
	}
	if want, have := http.StatusOK, resp.StatusCode; want != have {
		return "", fmt.Errorf("want /metrics status code %d, have %d. Body:\n%s", want, have, b)
	}
	return string(b), nil
}
