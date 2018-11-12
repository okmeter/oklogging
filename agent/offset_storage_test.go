package agent

import (
	"testing"
	"io/ioutil"
	"os"
	"github.com/stretchr/testify/require"
	"path"
	"github.com/stretchr/testify/assert"
)

func TestOffsetStorage(t *testing.T) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "offsets")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	baseDir := path.Join(tmpDir, "baseDir")
	storage, err := NewOffsetStorage(baseDir)
	require.NoError(t, err)

	assert.NoError(t, storage.Save("/var/log/123.log", 100))
	offset, err := storage.Get("/var/log/123.log")
	assert.NoError(t, err)
	assert.Equal(t, int64(100), offset)

	storage.GC([]string{})

	_, err = storage.Get("/var/log/123.log")
	assert.Error(t, err)
}