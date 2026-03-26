package vector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVectorFactory(t *testing.T) {
	factory := NewVectorFactory()
	types := factory.ListSupportedTypes()
	
	assert.NotNil(t, factory)
	assert.NotNil(t, types)
	
	var _ VectorFactory = factory
}
