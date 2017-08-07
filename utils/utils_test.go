package utils

import (
	"io/ioutil"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWritePid(t *testing.T) {
	pidPath, err := ioutil.TempFile(os.TempDir(), "pid-")
	assert.NoError(t, err)

	WritePid(pidPath.Name())

	f, err := os.Open(pidPath.Name())
	assert.NoError(t, err)

	content, err := ioutil.ReadAll(f)
	assert.NoError(t, err)

	pid := strconv.Itoa(os.Getpid())
	assert.Equal(t, pid, string(content))

	os.Remove(pidPath.Name())
}

func TestGetAppInfo(t *testing.T) {
	containerName := "eru-stats_api_EAXPcM"
	name, entrypoint, ident, err := GetAppInfo(containerName)
	assert.NoError(t, err)

	assert.Equal(t, name, "eru-stats")
	assert.Equal(t, entrypoint, "api")
	assert.Equal(t, ident, "EAXPcM")

	containerName = "api_EAXPcM"
	_, _, _, err = GetAppInfo(containerName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container name is not eru pattern")
}
