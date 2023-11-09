package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testClustername = "test-cluster"
	testCIDR        = "100.96.0.0/24"
	testNodeName    = "fakeNode1"
)

func TestGetRouteName(t *testing.T) {
	name := GetRouteName(testNodeName, testCIDR, testClustername)
	expectedName := testNodeName + "-100.96.0.0-24-" + testClustername
	assert.Equal(t, name, expectedName)
}
