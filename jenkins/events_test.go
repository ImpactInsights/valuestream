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
	assert.Equal(t, "test", be.BranchID())
}

func TestBuildEvent_BranchID_NoModifiers(t *testing.T) {
	branch := "origiin/test"
	be := BuildEvent{
		ScmInfo: &ScmInfo{
			Branch: &branch,
		},
	}
	assert.Equal(t, "origiin/test", be.BranchID())
}
