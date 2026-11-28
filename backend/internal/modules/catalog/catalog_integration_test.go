//go:build integration

package catalog

import (
	"context"
	"sync"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type recordingNotifier struct {
	mu                    sync.Mutex
	notified              []RestockedSKU
	purged                [][]uuid.UUID
	onNotify              func()
	failNotify, failPurge error
}

func (n *recordingNotifier) NotifyRestocked(_ context.Context, sku RestockedSKU) error {
	n.mu.Lock()
	n.notified = append(n.notified, sku)
	hook := n.onNotify
	n.mu.Unlock()
	if hook != nil {
		hook()
	}
	return n.failNotify
}

func (n *recordingNotifier) PurgeSKUSubscriptions(_ context.Context, ids []uuid.UUID) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.purged = append(n.purged, ids)
	return n.failPurge
}

func (n *recordingNotifier) skus() []uuid.UUID {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []uuid.UUID
	for _, s := range n.notified {
		out = append(out, s.SKUID)
	}
	return out
}
