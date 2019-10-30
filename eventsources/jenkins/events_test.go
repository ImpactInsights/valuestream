package jenkins

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBuildEvent_BranchID_Origin(t *testing.T) {
	branch := "origin/test"
	be := BuildEvent{
		ScmInfo: &ScmInfo{
			Branch: &branch,
		},
	}
	assert.Equal(t, "test", be.branchID())
}

func TestBuildEvent_BranchID_NoModifiers(t *testing.T) {
	branch := "origiin/test"
	be := BuildEvent{
		ScmInfo: &ScmInfo{
			Branch: &branch,
		},
	}
	assert.Equal(t, "origiin/test", be.branchID())
}

func TestBuildEvent_OperationName_Deploy(t *testing.T) {
	assert.Equal(t, "deploy", BuildEvent{
		Parameters: map[string]string{
			"key":  "value",
			"type": "deploy",
			"key1": "value",
		},
	}.OperationName())
}

func TestBuildEvent_OperationName_Build(t *testing.T) {
	assert.Equal(t, "build", BuildEvent{
		Parameters: map[string]string{
			"key":  "value",
			"type": "deploy1",
			"key1": "value",
		},
	}.OperationName())
}
