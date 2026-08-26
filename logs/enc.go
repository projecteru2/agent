package logs

import (
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
	*json.Encoder
	wt io.WriteCloser
}

func NewStreamEncoder(wt io.WriteCloser) *StreamEncoder {
	return &StreamEncoder{
		Encoder: json.NewEncoder(wt),
		wt:      wt,
	}
}

func (e *StreamEncoder) Encode(logline *types.Log) error {
	return e.Encoder.Encode(logline)
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
		"ID":                logline.ID,
		"TYPE":              logline.Type,
		"ENTRY_POINT":       logline.EntryPoint,
		"IDENT":             logline.Ident,
		"DATE_TIME":         logline.Datetime,
		"EXTRA":             string(extra),
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return journal.Send(logline.Data, journal.PriInfo, vars)
}

func (c *JournalEncoder) Close() error {
	return nil
}
