package collectors

import (
	"os/exec"
	"testing"
)

func TestParseMmhealth(t *testing.T) {
	execCommand = fakeExecCommand
	mockedStdout = `
mmhealth:Event:HEADER:version:reserved:reserved:node:component:entityname:entitytype:event:arguments:activesince:identifier:ishidden:
mmhealth:State:HEADER:version:reserved:reserved:node:component:entityname:entitytype:status:laststatuschange:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:NODE:ib-cluster-rw02.example.com:NODE:HEALTHY:2020-01-10 10%3A32%3A17.613885 EST:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:GPFS:ib-cluster-rw02.example.com:NODE:HEALTHY:2020-01-10 10%3A32%3A17.590229 EST:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:NETWORK:ib-cluster-rw02.example.com:NODE:HEALTHY:2020-01-03 15%3A32%3A38.077722 EST:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:NETWORK:ib0:NIC:HEALTHY:2020-01-07 08%3A33%3A41.113905 EST:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:FILESYSTEM:ib-cluster-rw02.example.com:NODE:HEALTHY:2020-01-10 10%3A32%3A17.577151 EST:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:FILESYSTEM:project:FILESYSTEM:HEALTHY:2020-01-07 18%3A03%3A31.834689 EST:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:FILESYSTEM:scratch:FILESYSTEM:HEALTHY:2020-01-07 18%3A03%3A31.842569 EST:
mmhealth:State:0:1:::ib-cluster-rw02.example.com:FILESYSTEM:ess:FILESYSTEM:HEALTHY:2020-01-14 10%3A37%3A33.657052 EST:
`
	defer func() { execCommand = exec.Command }()
	metrics, err := mmhealth_parse(mockedStdout)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if len(metrics) != 8 {
		t.Errorf("Expected 8 metrics returned, got %d", len(metrics))
		return
	}
	if val := metrics[0].Component; val != "NODE" {
		t.Errorf("Unexpected Component got %s", val)
	}
	if val := metrics[0].EntityName; val != "ib-cluster-rw02.example.com" {
		t.Errorf("Unexpected EntityName got %s", val)
	}
	if val := metrics[0].EntityType; val != "NODE" {
		t.Errorf("Unexpected EntityType got %s", val)
	}
}
