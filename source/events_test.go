package source

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/common"
)

func TestPipeEventsDetachesReporterBeforeClosing(t *testing.T) {
	reporter := NewReporter()
	events, errs := PipeEvents(t.Context(), reporter, func(context.Context) error {
		return errors.New("watch failed")
	})

	require.Error(t, <-errs)
	for range events {
	}

	for i := range 1000 {
		action := common.StatusStart
		if i%2 == 1 {
			action = common.StatusDie
		}
		reporter.Report(oneWorkload, action)
	}
}
