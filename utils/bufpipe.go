package utils

import (
	"bytes"
	"errors"
	"io"
	"sync"
)

type pipe struct {
	cond       *sync.Cond
	buf        *bytes.Buffer
	cap        int64
	length     int64
	rerr, werr error
}

type PipeReader struct {
	*pipe
}

func (r *PipeReader) Read(data []byte) (int, error) {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

RETRY:
	n, err := r.buf.Read(data)
	if errors.Is(err, io.EOF) && r.rerr == nil && n == 0 {
		r.cond.Wait()
		goto RETRY
	}
	// the io.Reader contract requires n > 0 to be handled even with an error
	if n > 0 {
		r.length -= int64(n)
	}
	if errors.Is(err, io.EOF) {
		return n, r.rerr
	}
	return n, err
}

func (r *PipeReader) Close() error {
	return r.CloseWithError(nil)
}

// CloseWithError closes the reader, later writes on the pipe return err.
func (r *PipeReader) CloseWithError(err error) error {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	if err == nil {
		err = io.ErrClosedPipe
	}
	r.werr = err
	return nil
}

type PipeWriter struct {
	*pipe
}

// Write appends data to the buffer, discarding it once the buffer is over capacity.
func (w *PipeWriter) Write(data []byte) (int, error) {
	w.cond.L.Lock()
	defer w.cond.L.Unlock()

	if w.werr != nil {
		return 0, w.werr
	}

	if w.length > w.cap {
		return len(data), nil
	}
	n, err := w.buf.Write(data)
	if n > 0 {
		w.length += int64(n)
	}
	w.cond.Signal()
	return n, err
}

func (w *PipeWriter) Close() error {
	return w.CloseWithError(nil)
}

func (w *PipeWriter) CloseWithError(err error) error {
	w.cond.L.Lock()
	defer w.cond.L.Unlock()

	if err == nil {
		err = io.EOF
	}
	w.rerr = err
	w.cond.Signal()
	return nil
}

func NewBufPipe(bufCap int64) (*PipeReader, *PipeWriter) {
	p := &pipe{
		buf:    bytes.NewBuffer(nil),
		cond:   sync.NewCond(new(sync.Mutex)),
		cap:    bufCap,
		length: 0,
		rerr:   nil,
		werr:   nil,
	}
	return &PipeReader{pipe: p}, &PipeWriter{pipe: p}
}
