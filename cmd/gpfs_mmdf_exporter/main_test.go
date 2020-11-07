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
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/treydock/gpfs_exporter/collectors"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	outputPath string
)

func TestMain(m *testing.M) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "output")
	if err != nil {
		os.Exit(1)
	}
	outputPath = tmpDir + "/output"
	defer os.RemoveAll(tmpDir)
	if _, err := kingpin.CommandLine.Parse([]string{fmt.Sprintf("--output=%s", outputPath), "--collector.mmdf.filesystems=project"}); err != nil {
		os.Exit(1)
	}
	exitVal := m.Run()
	os.Exit(exitVal)
}

func TestCollect(t *testing.T) {
	mmdfStdout := `
mmdf:nsd:HEADER:version:reserved:reserved:nsdName:storagePool:diskSize:failureGroup:metadata:data:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:diskAvailableForAlloc:
mmdf:poolTotal:HEADER:version:reserved:reserved:poolName:poolSize:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:maxDiskSize:
mmdf:data:HEADER:version:reserved:reserved:totalData:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:
mmdf:metadata:HEADER:version:reserved:reserved:totalMetadata:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:
mmdf:fsTotal:HEADER:version:reserved:reserved:fsSize:freeBlocks:freeBlocksPct:freeFragments:freeFragmentsPct:
mmdf:inode:HEADER:version:reserved:reserved:usedInodes:freeInodes:allocatedInodes:maxInodes:
mmdf:nsd:0:1:::P_META_VD102:system:771751936:300:Yes:No:320274944:41:5005384:1::
mmdf:nsd:0:1:::P_DATA_VD02:data:46766489600:200:No:Yes:6092915712:13:154966272:0::
mmdf:poolTotal:0:1:::data:3647786188800:475190722560:13:12059515296:0:3860104580096:
mmdf:data:0:1:::3647786188800:475190722560:13:12059515296:0:
mmdf:metadata:0:1:::13891534848:6011299328:43:58139768:0:
mmdf:fsTotal:0:1:::3661677723648:481202021888:14:12117655064:0:
mmdf:inode:0:1:::430741822:484301506:915043328:1332164000:
`
	expected := `
# HELP gpfs_fs_allocated_inodes GPFS filesystem inodes allocated
# TYPE gpfs_fs_allocated_inodes gauge
gpfs_fs_allocated_inodes{fs="project"} 9.15043328e+08
# HELP gpfs_fs_free_bytes GPFS filesystem free size in bytes
# TYPE gpfs_fs_free_bytes gauge
gpfs_fs_free_bytes{fs="project"} 4.92750870413312e+14
# HELP gpfs_fs_free_inodes GPFS filesystem inodes free
# TYPE gpfs_fs_free_inodes gauge
gpfs_fs_free_inodes{fs="project"} 4.84301506e+08
# HELP gpfs_fs_free_percent GPFS filesystem free percent
# TYPE gpfs_fs_free_percent gauge
gpfs_fs_free_percent{fs="project"} 14
# HELP gpfs_fs_metadata_free_bytes GPFS metadata free size in bytes
# TYPE gpfs_fs_metadata_free_bytes gauge
gpfs_fs_metadata_free_bytes{fs="project"} 6.155570511872e+12
# HELP gpfs_fs_metadata_free_percent GPFS metadata free percent
# TYPE gpfs_fs_metadata_free_percent gauge
gpfs_fs_metadata_free_percent{fs="project"} 43
# HELP gpfs_fs_metadata_total_bytes GPFS total metadata size in bytes
# TYPE gpfs_fs_metadata_total_bytes gauge
gpfs_fs_metadata_total_bytes{fs="project"} 1.4224931684352e+13
# HELP gpfs_fs_total_bytes GPFS filesystem total size in bytes
# TYPE gpfs_fs_total_bytes gauge
gpfs_fs_total_bytes{fs="project"} 3.749557989015552e+15
# HELP gpfs_fs_total_inodes GPFS filesystem inodes total
# TYPE gpfs_fs_total_inodes gauge
gpfs_fs_total_inodes{fs="project"} 1.332164e+09
# HELP gpfs_fs_used_inodes GPFS filesystem inodes used
# TYPE gpfs_fs_used_inodes gauge
gpfs_fs_used_inodes{fs="project"} 4.30741822e+08`
	collectors.MmdfExec = func(fs string, ctx context.Context) (string, error) {
		return mmdfStdout, nil
	}
	collect(log.NewNopLogger())
	content, err := ioutil.ReadFile(outputPath)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if !strings.Contains(string(content), expected) {
		t.Errorf("Unexpected content:\n%s\nExpected:\n%s", string(content), expected)
	}
}
