package main

import (
	"fmt"

	"github.com/riverqueue/river"
)

func workerQueues(defaultWorkers int, modules ...map[string]int) (map[string]int, error) {
	queues := map[string]int{}
	add := func(name string, n int) error {
		if n <= 0 {
			return fmt.Errorf("worker queue %q：併發 %d 必須為正", name, n)
		}
		if _, dup := queues[name]; dup {
			return fmt.Errorf("worker queue %q 重複宣告", name)
		}
		queues[name] = n
		return nil
	}
	if err := add(river.QueueDefault, defaultWorkers); err != nil {
		return nil, err
	}
	for _, m := range modules {
		for name, n := range m {
			if err := add(name, n); err != nil {
				return nil, err
			}
		}
	}
	return queues, nil
}
