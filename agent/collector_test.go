package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectMetrics_ReturnsValidRanges(t *testing.T) {
	m, err := CollectMetrics()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, m.CPU, 0.0)
	assert.LessOrEqual(t, m.CPU, 100.0)
	assert.GreaterOrEqual(t, m.Memory, 0.0)
	assert.LessOrEqual(t, m.Memory, 100.0)
	assert.GreaterOrEqual(t, m.Disk, 0.0)
	assert.LessOrEqual(t, m.Disk, 100.0)
}
