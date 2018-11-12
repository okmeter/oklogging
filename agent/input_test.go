package agent

import (
	"testing"
	"io/ioutil"
	"os"
	"github.com/stretchr/testify/require"
	"path"
	"github.com/stretchr/testify/assert"
)

func TestFileInput(t *testing.T) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "logs")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	offsetStgorage, err := NewOffsetStorage(tmpDir)
	require.NoError(t, err)

	logPath := path.Join(tmpDir, "1.log")
	l, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE, 0644)
	require.NoError(t, err)

	l.WriteString("line1\n")

	input, err := NewFileInput(logPath, offsetStgorage)
	require.NoError(t, err)

	line, err := input.ReadLine()
	require.NoError(t, err)
	assert.Equal(t, "line1", line)

	require.NoError(t, input.SaveOffset())
	offset, err := offsetStgorage.Get(logPath)
	require.NoError(t, err)
	assert.Equal(t, int64(6), offset)
	input.Close()

	l.WriteString("line2\n")
	input, err = NewFileInput(logPath, offsetStgorage)
	require.NoError(t, err)

	line, err = input.ReadLine()
	require.NoError(t, err)
	assert.Equal(t, "line2", line)
	input.Close()

	require.NoError(t, offsetStgorage.Save(logPath, 100500))
	input, err = NewFileInput(logPath, offsetStgorage)
	require.NoError(t, err)

	line, err = input.ReadLine()
	require.NoError(t, err)
	assert.Equal(t, "line1", line)
}