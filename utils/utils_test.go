package utils

import (
	"io"
	"io/ioutil"
	"math"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

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
	assert.Contains(t, err.Error(), "invalid container name")
}

func TestPipeWriter_NoBlocking(t *testing.T) {
	r, w := NewBufPipe(10000)
	io.WriteString(w, "abc")
	io.WriteString(w, "def")
	w.Close()

	b, err := ioutil.ReadAll(r)
	assert.NoError(t, err)
	assert.Equal(t, b, []byte("abcdef"))
}

func TestMultiBlocking(t *testing.T) {
	results := make(chan []byte)
	block := func(r io.Reader) {
		b := make([]byte, 3)
		n, err := r.Read(b)
		assert.NoError(t, err)
		results <- b[:n]
	}

	r, w := NewBufPipe(10000)
	go block(r)
	go block(r)
	go block(r)

	time.Sleep(time.Millisecond) // Ensure blocking.

	data := []string{"abc", "def", "ghi"}
	for _, s := range data {
		n, err := w.Write([]byte(s))
		assert.NoError(t, err)
		assert.Equal(t, n, 3)
	}

	var ss []string
	for i := 0; i < 3; i++ {
		ss = append(ss, string(<-results))
	}
	sort.Strings(ss)
	assert.Equal(t, ss, data)
}

func BenchmarkReadOnly(b *testing.B) {
	length := 2
	r, w := NewBufPipe(int64(b.N))
	w.Close()
	data := make([]byte, length)
	io.WriteString(w, string(make([]byte, b.N, b.N)))
	b.ResetTimer()
	for {
		_, err := io.ReadFull(r, data)
		if err != nil {
			if math.Mod(float64(b.N), float64(length)) == 0 {
				assert.EqualError(b, err, "EOF")
			} else {
				assert.EqualError(b, err, "unexpected EOF")
			}
			break
		}
	}
}
