package utils

import (
	"bufio"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"
)

func TestBufPipe(t *testing.T) {
	size, _ := units.RAMInBytes("10M")
	r, w := NewBufPipe(size)
	w.Write([]byte("test"))
	go func() {
		time.Sleep(time.Second)
		w.Close()
		r.Close()
	}()

	reader := bufio.NewReader(r)

	for {
		_, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println(err)
			assert.Equal(t, err, io.EOF)
			return
		}
	}
}
