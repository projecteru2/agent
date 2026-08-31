package logs

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"

	"github.com/coreos/go-systemd/v22/journal"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
)

type Encoder interface {
	Encode(*types.Log) error
	Close() error
}

type StreamEncoder struct {
	wt  io.WriteCloser
	buf bytes.Buffer
	enc *json.Encoder
}

func NewStreamEncoder(wt io.WriteCloser) *StreamEncoder {
	e := &StreamEncoder{wt: wt}
	e.enc = json.NewEncoder(&e.buf)
	return e
}

// Encode runs under Writer's lock, which is what lets one buffer serve every line.
func (e *StreamEncoder) Encode(logline *types.Log) error {
	e.buf.Reset()
	if err := e.enc.Encode(logline); err != nil {
		return err
	}
	_, err := e.wt.Write(e.buf.Bytes())
	return err
}

func (e *StreamEncoder) Close() error {
	return e.wt.Close()
}

type JournalEncoder struct {
	mu sync.Mutex
}

func CreateJournalEncoder() (*JournalEncoder, error) {
	if !journal.Enabled() {
		return nil, common.ErrJournalDisabled
	}
	return &JournalEncoder{}, nil
}

func (c *JournalEncoder) Encode(logline *types.Log) error {
	extra, err := json.Marshal(logline.Extra)
	if err != nil {
		return err
	}

	vars := map[string]string{
		common.FieldIdentifier: logline.Name,
		"ID":                   logline.ID,
		"TYPE":                 logline.Type,
		"ENTRY_POINT":          logline.EntryPoint,
		"IDENT":                logline.Ident,
		"DATE_TIME":            logline.Datetime,
		"EXTRA":                string(extra),
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return journal.Send(logline.Data, journal.PriInfo, vars)
}

func (c *JournalEncoder) Close() error {
	return nil
}
