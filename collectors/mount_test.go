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
	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus/testutil"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestGetGPFSMounts(t *testing.T) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "proc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	procMounts = tmpDir + "/mounts"
	mockedProcMounts := `pitzer_root.ten.osc.edu:/pitzer_root_rhel76_1 / nfs rw,relatime,vers=3,rsize=65536,wsize=65536,namlen=255,acregmin=240,acregmax=240,acdirmin=240,acdirmax=240,hard,nolock,proto=tcp,timeo=600,retrans=2,sec=sys,mountaddr=10.27.2.2,mountvers=3,mountport=635,mountproto=tcp,fsc,local_lock=all,addr=10.27.2.2 0 0
/dev/mapper/vg0-lv_tmp /tmp xfs rw,relatime,attr2,inode64,noquota 0 0
scratch /fs/scratch gpfs rw,relatime 0 0
project /fs/project gpfs rw,relatime 0 0
10.11.200.17:/PZS0710 /users/PZS0710 nfs4 rw,relatime,vers=4.0,rsize=65536,wsize=65536,namlen=255,hard,proto=tcp,timeo=600,retrans=2,sec=sys,clientaddr=10.4.0.102,local_lock=none,addr=10.11.200.17 0 0
`
	if err := ioutil.WriteFile(procMounts, []byte(mockedProcMounts), 0644); err != nil {
		t.Fatal(err)
	}
	gpfsMounts, err := getGPFSMounts()
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if len(gpfsMounts) != 2 {
		t.Errorf("Incorrect number of GPFS mounts, expected 2, got %d", len(gpfsMounts))
		return
	}
	if val := gpfsMounts[0]; val != "/fs/scratch" {
		t.Errorf("Unexpected Path value %s", val)
	}
	if val := gpfsMounts[1]; val != "/fs/project" {
		t.Errorf("Unexpected Path value %s", val)
	}
}

func TestGetGPFSMountsFSTab(t *testing.T) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "proc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	fstabPath = tmpDir + "/fstab"
	mockedFstab := `
LABEL=tmp       /tmp    xfs     defaults        1       2
project              /fs/project          gpfs       rw,mtime,atime,quota=userquota;groupquota;filesetquota;perfileset,dev=project,noauto 0 0
scratch              /fs/scratch          gpfs       rw,mtime,atime,quota=userquota;groupquota;filesetquota;perfileset,dev=scratch,noauto 0 0
	`
	if err := ioutil.WriteFile(fstabPath, []byte(mockedFstab), 0644); err != nil {
		t.Fatal(err)
	}
	gpfsMounts, err := getGPFSMountsFSTab()
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if len(gpfsMounts) != 2 {
		t.Errorf("Incorrect number fo GPFS mounts, expected 2, got %d", len(gpfsMounts))
	}
	if val := gpfsMounts[0]; val != "/fs/project" {
		t.Errorf("Unexpected value %s", val)
	}
	if val := gpfsMounts[1]; val != "/fs/scratch" {
		t.Errorf("Unexpected value %s", val)
	}
}

func TestMountCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	mounts := "/fs/project,/fs/scratch,/fs/ess"
	configMounts = &mounts
	tmpDir, err := ioutil.TempDir(os.TempDir(), "proc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	procMounts = tmpDir + "/mounts"
	fstabPath = tmpDir + "/fstab"
	mockedProcMounts := `pitzer_root.ten.osc.edu:/pitzer_root_rhel76_1 / nfs rw,relatime,vers=3,rsize=65536,wsize=65536,namlen=255,acregmin=240,acregmax=240,acdirmin=240,acdirmax=240,hard,nolock,proto=tcp,timeo=600,retrans=2,sec=sys,mountaddr=10.27.2.2,mountvers=3,mountport=635,mountproto=tcp,fsc,local_lock=all,addr=10.27.2.2 0 0
/dev/mapper/vg0-lv_tmp /tmp xfs rw,relatime,attr2,inode64,noquota 0 0
scratch /fs/scratch gpfs rw,relatime 0 0
project /fs/project gpfs rw,relatime 0 0
10.11.200.17:/PZS0710 /users/PZS0710 nfs4 rw,relatime,vers=4.0,rsize=65536,wsize=65536,namlen=255,hard,proto=tcp,timeo=600,retrans=2,sec=sys,clientaddr=10.4.0.102,local_lock=none,addr=10.11.200.17 0 0
`
	mockedFstab := `
project              /fs/project          gpfs       rw,mtime,atime,quota=userquota;groupquota;filesetquota;perfileset,dev=project,noauto 0 0
scratch              /fs/scratch          gpfs       rw,mtime,atime,quota=userquota;groupquota;filesetquota;perfileset,dev=scratch,noauto 0 0
	`
	if err := ioutil.WriteFile(procMounts, []byte(mockedProcMounts), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(fstabPath, []byte(mockedFstab), 0644); err != nil {
		t.Fatal(err)
	}
	metadata := `
		# HELP gpfs_mount_status Status of GPFS filesystems, 1=mounted 0=not mounted
		# TYPE gpfs_mount_status gauge`
	expected := `
		gpfs_mount_status{mount="/fs/ess"} 0
		gpfs_mount_status{mount="/fs/project"} 1
		gpfs_mount_status{mount="/fs/scratch"} 1
	`
	collector := NewMountCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val := testutil.CollectAndCount(collector); val != 6 {
		t.Errorf("Unexpected collection count %d, expected 6", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(metadata+expected), "gpfs_mount_status"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}
