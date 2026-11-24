package main

import (
	"testing"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

func TestWorkerQueuesIncludesDefaultAndTheMediaProcessQueue(t *testing.T) {
	q, err := workerQueues(workerConcurrency, media.Queues())
	if err != nil {
		t.Fatal(err)
	}
	if q[river.QueueDefault] != workerConcurrency || q[media.ProcessQueue] != media.ProcessQueueWorkers || len(q) != 2 {
		t.Fatalf("queues = %v", q)
	}
}

func TestWorkerQueuesRejectsADuplicateKey(t *testing.T) {
	for name, mods := range map[string][]map[string]int{
		"between modules": {{"a": 1}, {"a": 2}},
		"with default":    {{river.QueueDefault: 3}},
	} {
		if q, err := workerQueues(workerConcurrency, mods...); err == nil {
			t.Fatalf("%s：預期重複 key 回錯誤，得到 %v", name, q)
		}
	}
}

func TestWorkerQueuesRejectsNonPositiveConcurrency(t *testing.T) {
	if _, err := workerQueues(workerConcurrency, map[string]int{"a": 0}); err == nil {
		t.Fatal("併發 0 該回錯誤")
	}
	if _, err := workerQueues(0); err == nil {
		t.Fatal("default 併發 0 該回錯誤")
	}
}
