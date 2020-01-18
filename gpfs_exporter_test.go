package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"github.com/Flaque/filet"
)

var (
	mockedExitStatus = 0
	mockedStdout     string
)

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestExecCommandHelper", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
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

	fmt.Fprintf(os.Stdout, os.Getenv("STDOUT"))
	i, _ := strconv.Atoi(os.Getenv("EXIT_STATUS"))
	os.Exit(i)
}

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
    if val := perfs[0].FS ; val != "scratch" {
        t.Errorf("Unexpected FS got %s", val)
    }
    if val := perfs[1].FS ; val != "project" {
        t.Errorf("Unexpected FS got %s", val)
    }
    if val := perfs[0].NodeName ; val != "ib-pitzer-rw02.ten" {
        t.Errorf("Unexpected NodeName got %s", val)
    }
    if val := perfs[1].NodeName ; val != "ib-pitzer-rw02.ten" {
        t.Errorf("Unexpected NodeName got %s", val)
    }
    if val := perfs[0].ReadBytes ; val != 205607400434 {
        t.Errorf("Unexpected ReadBytes got %d", val)
    }
}

func TestGetGPFSMounts(t *testing.T) {
    procMounts = "/tmp/proc-mounts"
    mockedProcMounts := `pitzer_root.ten.osc.edu:/pitzer_root_rhel76_1 / nfs rw,relatime,vers=3,rsize=65536,wsize=65536,namlen=255,acregmin=240,acregmax=240,acdirmin=240,acdirmax=240,hard,nolock,proto=tcp,timeo=600,retrans=2,sec=sys,mountaddr=10.27.2.2,mountvers=3,mountport=635,mountproto=tcp,fsc,local_lock=all,addr=10.27.2.2 0 0
/dev/mapper/vg0-lv_tmp /tmp xfs rw,relatime,attr2,inode64,noquota 0 0
scratch /fs/scratch gpfs rw,relatime 0 0
project /fs/project gpfs rw,relatime 0 0
10.11.200.17:/PZS0710 /users/PZS0710 nfs4 rw,relatime,vers=4.0,rsize=65536,wsize=65536,namlen=255,hard,proto=tcp,timeo=600,retrans=2,sec=sys,clientaddr=10.4.0.102,local_lock=none,addr=10.11.200.17 0 0
`
	filet.File(t, "/tmp/proc-mounts", mockedProcMounts)
	defer filet.CleanUp(t)
    gpfsMounts, err := getGPFSMounts()
    if err != nil {
        t.Errorf("Unexpected error: %s", err.Error())
    }
    if len(gpfsMounts) != 2 {
        t.Errorf("Incorrect number of GPFS mounts, expected 2, got %d", len(gpfsMounts))
        return
    }
    if val := gpfsMounts[0] ; val != "/fs/scratch" {
        t.Errorf("Unexpected Path value %s", val)
    }
    if val := gpfsMounts[1] ; val != "/fs/project" {
        t.Errorf("Unexpected Path value %s", val)
    }
/*
    if val := gpfsMounts[0].Path ; val != "/fs/scratch" {
        t.Errorf("Unexpected Path value %s", val)
    }
    if val := gpfsMounts[0].Mounted ; !val {
        t.Errorf("Unexpected value for Mounted, %v", val)
    }
    if val := gpfsMounts[1].Path ; val != "/fs/project" {
        t.Errorf("Unexpected Path value %s", val)
    }
    if val := gpfsMounts[1].Mounted ; !val {
        t.Errorf("Unexpected value for Mounted, %v", val)
    }
*/
}
