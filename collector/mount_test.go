package collector

import (
	"github.com/Flaque/filet"
	"testing"
)

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
	if val := gpfsMounts[0]; val != "/fs/scratch" {
		t.Errorf("Unexpected Path value %s", val)
	}
	if val := gpfsMounts[1]; val != "/fs/project" {
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
