package collector

import (
	"bufio"
	"context"
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

func TestJournalRecordMapsStderrPriority(t *testing.T) {
	assert.Equal(t, "stderr", (&journalRecord{Priority: "3"}).entry().Stream)
	assert.Equal(t, "stdout", (&journalRecord{Priority: "6"}).entry().Stream)
	assert.Equal(t, "console", (&journalRecord{Priority: "3", EruStream: "console"}).entry().Stream)
}

func TestJournalArgsFollowFromTheSavedCursor(t *testing.T) {
	assert.Equal(t, []string{
		"--follow", "--output=json", "--no-pager", "--lines=0", "SYSLOG_IDENTIFIER=eru",
	}, args(""))

	assert.Equal(t, []string{
		"--follow", "--output=json", "--no-pager",
		"--after-cursor=s=aaa;i=7", "--lines=all", "SYSLOG_IDENTIFIER=eru",
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

func TestJournalReadReportsTheExitStatusOfTheReader(t *testing.T) {
	j := NewJournal(filepath.Join(t.TempDir(), "state"))
	j.binary = fakeReader(t, "echo 'no journal on this node' >&2\nexit 1\n")

	err := j.Read(t.Context(), func(*Entry) { t.Error("no entry is expected") })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exited")
	assert.Contains(t, err.Error(), "no journal on this node")
}

func TestJournalReadDoesNotBlameTheReaderWhenTheContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	j := NewJournal(filepath.Join(t.TempDir(), "state"))
	j.binary = fakeReader(t, "sleep 30\n")

	done := make(chan error, 1)
	go func() { done <- j.Read(ctx, func(*Entry) { t.Error("no entry is expected") }) }()
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the reader to stop")
	}
}

func TestJournalReadStopsWhenARecordExceedsTheLineLimit(t *testing.T) {
	j := NewJournal(filepath.Join(t.TempDir(), "state"))
	j.binary = fakeReader(t, "head -c 2097152 /dev/zero | tr '\\0' 'x'\necho\nsleep 30\n")

	done := make(chan error, 1)
	go func() { done <- j.Read(t.Context(), func(*Entry) { t.Error("no entry is expected") }) }()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the reader to stop")
	}
}

func TestRingKeepsTheLastBytes(t *testing.T) {
	r := &ring{limit: 4}

	_, err := r.Write([]byte("abcdefg"))
	require.NoError(t, err)
	assert.Equal(t, "defg", r.String())
}

func fakeReader(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-journalctl")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatalf("write fake reader: %v", err)
	}
	return path
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
