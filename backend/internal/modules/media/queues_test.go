package media_test

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

func TestQueuesIncludesTheProcessQueue(t *testing.T) {
	if got, ok := media.Queues()[media.ProcessQueue]; !ok || got != media.ProcessQueueWorkers {
		t.Fatalf("Queues() = %v，預期含 %s=%d", media.Queues(), media.ProcessQueue, media.ProcessQueueWorkers)
	}
}
