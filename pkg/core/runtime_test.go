package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRuntime_ValidHTTPClient(t *testing.T) {
	mockHTTP := NewMockHTTPClient(t)

	runtime, err := NewRuntime(mockHTTP)

	assert.NoError(t, err)
	assert.NotNil(t, runtime)
	assert.Equal(t, mockHTTP, runtime.GetHTTPClient())
}

func TestNewRuntime_NilHTTPClient(t *testing.T) {
	runtime, err := NewRuntime(nil)

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidRuntime)
	assert.Nil(t, runtime)
}

func TestRuntimeImpl_GetHTTPClient(t *testing.T) {
	mockHTTP := NewMockHTTPClient(t)

	runtime, err := NewRuntime(mockHTTP)

	assert.NoError(t, err)
	assert.NotNil(t, runtime.GetHTTPClient())
	assert.Equal(t, mockHTTP, runtime.GetHTTPClient())
}
