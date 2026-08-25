package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/projecteru2/core/log"
)

const (
	journalBinary = "journalctl"
	cursorFile    = "journal.cursor"
	unitMatch     = "eru-*"
	identifier    = "eru"

	defaultStream = "stdout"

	cursorFlushInterval = 5 * time.Second
	scanBufferSize      = 64 << 10
	scanLineMax         = 1 << 20
)

// Entry is one journal record, addressed by the workload id the log shim wrote or by its unit.
type Entry struct {
	WorkloadID string
	Unit       string
	Stream     string
	Data       string
	Time       time.Time
}

type journalRecord struct {
	Cursor    string          `json:"__CURSOR"`
	Realtime  string          `json:"__REALTIME_TIMESTAMP"`
	Message   json.RawMessage `json:"MESSAGE"`
	Unit      string          `json:"_SYSTEMD_UNIT"`
	EruID     string          `json:"ERU_ID"`
	EruStream string          `json:"ERU_STREAM"`
}

func (r *journalRecord) entry() *Entry {
	stream := r.EruStream
	if stream == "" {
		stream = defaultStream
	}
	return &Entry{
		WorkloadID: r.EruID,
		Unit:       r.Unit,
		Stream:     stream,
		Data:       message(r.Message),
		Time:       realtime(r.Realtime),
	}
}

// Journal follows the node's journal and hands every eru workload line to one reader.
type Journal struct {
	cursorPath string
}

func NewJournal(stateDir string) *Journal {
	return &Journal{cursorPath: filepath.Join(stateDir, cursorFile)}
}

// Read follows the journal until ctx is done, calling handle for every eru workload line.
func (j *Journal) Read(ctx context.Context, handle func(*Entry)) error {
	logger := log.WithFunc("collector.Read")
	// journald speaks a binary format only libsystemd reads, so its own tool is the reader
	logger.Debugf(ctx, "forwarding workload logs needs %s on this node", journalBinary)

	cursor := j.loadCursor(ctx)
	cmd := exec.CommandContext(ctx, journalBinary, args(cursor)...) //nolint:gosec // the arguments are the agent's own match list and its saved cursor
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() { _ = cmd.Wait() }()
	defer func() { j.saveCursor(ctx, cursor) }()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, scanBufferSize), scanLineMax)
	nextFlush := time.Now().Add(cursorFlushInterval)

	for scanner.Scan() {
		record := &journalRecord{}
		if err := json.Unmarshal(scanner.Bytes(), record); err != nil {
			logger.Errorf(ctx, err, "failed to decode a journal record")
			continue
		}
		handle(record.entry())

		cursor = record.Cursor
		if time.Now().After(nextFlush) {
			j.saveCursor(ctx, cursor)
			nextFlush = time.Now().Add(cursorFlushInterval)
		}
	}
	return scanner.Err()
}

func (j *Journal) loadCursor(ctx context.Context) string {
	data, err := os.ReadFile(j.cursorPath) //nolint:gosec // the path is the agent's own state dir
	if err != nil {
		log.WithFunc("collector.loadCursor").Debugf(ctx, "no saved cursor at %s, following from now on", j.cursorPath)
		return ""
	}
	return string(data)
}

// saveCursor renames the cursor into place, so a restart never reads a half-written one.
func (j *Journal) saveCursor(ctx context.Context, cursor string) {
	if cursor == "" {
		return
	}
	logger := log.WithFunc("collector.saveCursor")

	if err := os.MkdirAll(filepath.Dir(j.cursorPath), 0o750); err != nil {
		logger.Errorf(ctx, err, "failed to create the state dir for %s", j.cursorPath)
		return
	}
	tmp := j.cursorPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(cursor), 0o600); err != nil {
		logger.Errorf(ctx, err, "failed to write %s", tmp)
		return
	}
	if err := os.Rename(tmp, j.cursorPath); err != nil {
		logger.Errorf(ctx, err, "failed to rename %s into place", tmp)
	}
}

func args(cursor string) []string {
	args := []string{"--follow", "--output=json", "--no-pager"}
	if cursor == "" {
		args = append(args, "--lines=0")
	} else {
		args = append(args, "--after-cursor="+cursor)
	}
	// -u builds its own disjunction, so the shim's identifier ors with the unit glob
	return append(args, "--unit="+unitMatch, "+", "SYSLOG_IDENTIFIER="+identifier)
}

// message decodes MESSAGE, which journalctl renders as a byte array when the line is not utf8.
func message(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var bytes []byte
	if err := json.Unmarshal(raw, &bytes); err == nil {
		return string(bytes)
	}
	return ""
}

func realtime(micros string) time.Time {
	usec, err := strconv.ParseInt(micros, 10, 64)
	if err != nil {
		return time.Now()
	}
	return time.UnixMicro(usec)
}
