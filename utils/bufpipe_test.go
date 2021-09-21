package utils

import (
	"bufio"
	"fmt"
	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
	"time"
)

func TestBufPipe(t *testing.T)  {
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
