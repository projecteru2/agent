package collector

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJournalRecordsMapToWorkloads(t *testing.T) {
	entries := readFixture(t)
	require.Len(t, entries, 4)

	assert.Equal(t, &Entry{
		Unit:   "eru-abc123.service",
		Stream: "stdout",
		Data:   "process pod line",
		Time:   time.UnixMicro(1627530367000000),
	}, entries[0])

	assert.Equal(t, &Entry{
		WorkloadID: "xyz789",
		Unit:       "containerd.service",
		Stream:     "stderr",
		Data:       "container line",
		Time:       time.UnixMicro(1627530368000000),
	}, entries[1])
}

func TestJournalRecordDecodesANonUtf8Message(t *testing.T) {
	entries := readFixture(t)
	assert.Equal(t, "hi\xff", entries[2].Data)
}

func TestJournalRecordWithoutATimestampIsStampedOnRead(t *testing.T) {
	before := time.Now()
	entries := readFixture(t)

	assert.Equal(t, "no timestamp", entries[3].Data)
	assert.False(t, entries[3].Time.Before(before))
}

func TestJournalArgsFollowFromTheSavedCursor(t *testing.T) {
	assert.Equal(t, []string{
		"--follow", "--output=json", "--no-pager", "--lines=0",
		"--unit=eru-*", "+", "SYSLOG_IDENTIFIER=eru",
	}, args(""))

	assert.Equal(t, []string{
		"--follow", "--output=json", "--no-pager", "--after-cursor=s=aaa;i=7",
		"--unit=eru-*", "+", "SYSLOG_IDENTIFIER=eru",
	}, args("s=aaa;i=7"))
}

func TestJournalCursorSurvivesARestart(t *testing.T) {
	ctx := t.Context()
	j := NewJournal(filepath.Join(t.TempDir(), "state"))
	assert.Empty(t, j.loadCursor(ctx))

	j.saveCursor(ctx, "s=aaa;i=9")
	assert.Equal(t, "s=aaa;i=9", j.loadCursor(ctx))

	_, err := os.Stat(j.cursorPath + ".tmp")
	assert.Error(t, err)
}

func TestJournalDoesNotSaveAnEmptyCursor(t *testing.T) {
	ctx := t.Context()
	j := NewJournal(filepath.Join(t.TempDir(), "state"))

	j.saveCursor(ctx, "")
	_, err := os.Stat(j.cursorPath)
	assert.Error(t, err)
}

func readFixture(t *testing.T) []*Entry {
	t.Helper()
	f, err := os.Open("testdata/journal.jsonl")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	var entries []*Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		record := &journalRecord{}
		require.NoError(t, json.Unmarshal(scanner.Bytes(), record))
		entries = append(entries, record.entry())
	}
	require.NoError(t, scanner.Err())
	return entries
}
