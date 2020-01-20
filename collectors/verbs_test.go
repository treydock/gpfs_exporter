package collectors

import (
	"os/exec"
	"testing"
)

func TestParseVerbsDisabled(t *testing.T) {
	execCommand = fakeExecCommand
	mockedStdout = `
VERBS RDMA status: disabled
`
	defer func() { execCommand = exec.Command }()
	metric, err := verbs_parse(mockedStdout)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if metric.Status != "disabled" {
		t.Errorf("Unexpected value for status, expected disabled, got %s", metric.Status)
	}
}

func TestParseVerbsStarted(t *testing.T) {
	execCommand = fakeExecCommand
	mockedStdout = `
VERBS RDMA status: started
`
	defer func() { execCommand = exec.Command }()
	metric, err := verbs_parse(mockedStdout)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if metric.Status != "started" {
		t.Errorf("Unexpected value for status, expected started, got %s", metric.Status)
	}
}
