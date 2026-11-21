package jobs_test

import (
	"slices"
	"testing"
	"time"

	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

func TestUnfinishedStatesCoverRunningAndExcludeCompleted(t *testing.T) {
	states := jobs.UnfinishedStates()
	if !slices.Contains(states, rivertype.JobStateRunning) {
		t.Fatal("running must be in the unique states (River requires it)")
	}
	if slices.Contains(states, rivertype.JobStateCompleted) {
		t.Fatal("completed must not be in the unique states")
	}
}

func TestUniqueByArgsOptions(t *testing.T) {
	o := jobs.UniqueByArgs(7)
	if o.MaxAttempts != 7 || !o.UniqueOpts.ByArgs || o.UniqueOpts.ByPeriod != 0 {
		t.Fatalf("unexpected opts: %+v", o)
	}
	if !slices.Equal(o.UniqueOpts.ByState, jobs.UnfinishedStates()) {
		t.Fatalf("states: %v", o.UniqueOpts.ByState)
	}
}

func TestSweepOptsOptions(t *testing.T) {
	o := jobs.SweepOpts(15 * time.Minute)
	if o.UniqueOpts.ByPeriod != 15*time.Minute || o.UniqueOpts.ByArgs {
		t.Fatalf("unexpected opts: %+v", o)
	}
}
