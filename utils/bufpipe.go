package utils

import (
	"bytes"
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

// A PipeReader is the read half of a pipe.
type PipeReader struct {
	*pipe
}

// A PipeWriter is the write half of a pipe.
type PipeWriter struct {
	*pipe
}

// NewBufPipe creates a synchronous pipe with capacity
func NewBufPipe(bufCap int64) (*PipeReader, *PipeWriter) {
	p := &pipe{
		buf:    bytes.NewBuffer(nil),
		cond:   sync.NewCond(new(sync.Mutex)),
		cap:    bufCap,
		length: 0,
		rerr:   nil,
		werr:   nil,
	}
	return &PipeReader{
			pipe: p,
		}, &PipeWriter{
			pipe: p,
		}
}

// Read implements the standard Read interface
func (r *PipeReader) Read(data []byte) (int, error) {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

RETRY:
	n, err := r.buf.Read(data)
	// If not closed and no read, wait for writing.
	if err == io.EOF && r.rerr == nil && n == 0 {
		r.cond.Wait()
		goto RETRY
	}
	// io.Reader requires to always handle n > 0
	if n > 0 {
		r.length -= int64(n)
	}
	if err == io.EOF {
		return n, r.rerr
	}
	return n, err
}

// Close closes the reader
func (r *PipeReader) Close() error {
	return r.CloseWithError(nil)
}

// CloseWithError closes the reader; subsequent writes to the write half of the
// pipe will return the error err.
func (r *PipeReader) CloseWithError(err error) error {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	if err == nil {
		err = io.ErrClosedPipe
	}
	r.werr = err
	return nil
}

// Write implements the standard Write interface: discard if current length exceeds capacity
func (w *PipeWriter) Write(data []byte) (int, error) {
	w.cond.L.Lock()
	defer w.cond.L.Unlock()

	if w.werr != nil {
		return 0, w.werr
	}

	// discard
	if w.length > w.cap {
		return len(data), nil
	}
	n, err := w.buf.Write(data)
	// io.Writer stipulates n < 0 if err != nil
	if n > 0 {
		w.length += int64(n)
	}
	w.cond.Signal()
	return n, err
}

// Close closes the writer
func (w *PipeWriter) Close() error {
	return w.CloseWithError(nil)
}

// CloseWithError closes the writer
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
