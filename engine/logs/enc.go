package logs

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/coreos/go-systemd/journal"
	"github.com/projecteru2/agent/types"
)

// Encoder .
type Encoder interface {
	Encode(*types.Log) error
	Close() error
}

// StreamEncoder .
type StreamEncoder struct {
	*json.Encoder
	wt io.WriteCloser
}

// NewStreamEncoder .
func NewStreamEncoder(wt io.WriteCloser) *StreamEncoder {
	return &StreamEncoder{
		Encoder: json.NewEncoder(wt),
		wt:      wt,
	}
}

// Encode .
func (e *StreamEncoder) Encode(logline *types.Log) error {
	return e.Encoder.Encode(logline)
}

// Close .
func (e *StreamEncoder) Close() error {
	return e.wt.Close()
}

var errJournalDisabled = fmt.Errorf("journal disabled")

// JournalEncoder .
type JournalEncoder struct {
	sync.Mutex
}

// CreateJournalEncoder .
func CreateJournalEncoder() (*JournalEncoder, error) {
	if !journal.Enabled() {
		return nil, errJournalDisabled
	}
	return &JournalEncoder{}, nil
}

// Encode .
func (c *JournalEncoder) Encode(logline *types.Log) error {
	extra, err := json.Marshal(logline.Extra)
	if err != nil {
		return err
	}

	vars := map[string]string{
		"SYSLOG_IDENTIFIER": logline.Name,
		"ID":                logline.ID,
		"TYPE":              logline.Type,
		"ENTRY_POINT":       logline.EntryPoint,
		"IDENT":             logline.Ident,
		"DATE_TIME":         logline.Datetime,
		"EXTRA":             string(extra),
	}

	c.Lock()
	defer c.Unlock()

	p := fmt.Sprintf("message %s", logline.Data)
	return journal.Send(p, journal.PriInfo, vars)
}

// Close .
func (c *JournalEncoder) Close() (err error) {
	return
}
