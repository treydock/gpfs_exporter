package collector

import (
	"os/exec"
	"testing"
)

func TestParseMmlsfs(t *testing.T) {
	execCommand = fakeExecCommand
	mockedStdout = `
fs::HEADER:version:reserved:reserved:deviceName:fieldName:data:remarks:
mmlsfs::0:1:::project:defaultMountPoint:%2Ffs%2Fproject::
mmlsfs::0:1:::scratch:defaultMountPoint:%2Ffs%2Fscratch::
mmlsfs::0:1:::ess:defaultMountPoint:%2Ffs%2Fess::
`
	defer func() { execCommand = exec.Command }()
	filesystems, err := parse_mmlsfs(mockedStdout)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if len(filesystems) != 3 {
		t.Errorf("Expected 3 perfs returned, got %d", len(filesystems))
		return
	}
}

func TestParseMmdf(t *testing.T) {
    execCommand = fakeExecCommand
    mockedStdout = `
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
    defer func() { execCommand = exec.Command }()
    dfmetrics, err := Parse_mmdf(mockedStdout)
    if err != nil {
        t.Errorf("Unexpected error: %s", err.Error())
    }
    if dfmetrics.InodesFree != 484301506 {
        t.Errorf("Unexpected value for InodesFree, got %d", dfmetrics.InodesFree)
    }
    if dfmetrics.FSTotal != 3749557989015552 {
        t.Errorf("Unexpected value for FSTotal, got %d", dfmetrics.FSTotal)
    }
    if dfmetrics.FSFreePercent != 14 {
        t.Errorf("Unexpected value for FSFreePercent, got %d", dfmetrics.FSFreePercent)
    }
    if dfmetrics.MetadataTotal != 14224931684352 {
        t.Errorf("Unexpected value for MetadataTotal, got %d", dfmetrics.MetadataTotal)
    }
    if dfmetrics.MetadataFreePercent != 43 {
        t.Errorf("Unexpected value for MetadataFreePercent, got %d", dfmetrics.MetadataFreePercent)
    }
}
