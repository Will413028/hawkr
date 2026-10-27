package account

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
)

type testClerk struct {
	banned  []string
	deleted []string
	reject  bool
	prefix  string
	noop    bool
	calls   int
}

func (*testClerk) Issuer() string            { return "test" }
func (c *testClerk) OwnsUser(id string) bool { return strings.HasPrefix(id, c.prefix) }
func (*testClerk) Verify(context.Context, string) (model.ClerkIdentity, error) {
	return model.ClerkIdentity{}, nil
}
func (c *testClerk) BanUser(_ context.Context, id string) error {
	c.calls++
	if c.reject {
		return model.ErrRejected
	}
	if !c.noop {
		c.banned = append(c.banned, id)
	}
	return nil
}
func (c *testClerk) DeleteUser(_ context.Context, id string) error {
	c.calls++
	if c.reject {
		return model.ErrRejected
	}
	if !c.noop {
		c.deleted = append(c.deleted, id)
	}
	return nil
}

func TestClerkUserWorker(t *testing.T) {
	c := &testClerk{prefix: "user_"}
	m := New(Deps{Customers: []model.Clerk{c}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	w := &clerkUserWorker{m: m}
	work := func(args clerkUserArgs) error {
		return w.Work(context.Background(), &river.Job[clerkUserArgs]{JobRow: &rivertype.JobRow{}, Args: args})
	}
	if err := work(clerkUserArgs{Action: clerkUserActionBan, ClerkUserID: "user_1"}); err != nil || len(c.banned) != 1 || c.banned[0] != "user_1" {
		t.Fatalf("ban = %v, %v", c.banned, err)
	}
	if err := work(clerkUserArgs{Action: clerkUserActionDelete, ClerkUserID: "user_1"}); err != nil || len(c.deleted) != 1 || c.deleted[0] != "user_1" {
		t.Fatalf("delete = %v, %v", c.deleted, err)
	}
	if err := work(clerkUserArgs{Action: clerkUserActionDelete, ClerkUserID: "unlinked:9"}); err != nil || len(c.deleted) != 1 {
		t.Fatalf("unlinked = %v, %v", c.deleted, err)
	}
	c.reject = true
	var cancel *river.JobCancelError
	if err := work(clerkUserArgs{Action: clerkUserActionBan, ClerkUserID: "user_1"}); !errors.As(err, &cancel) {
		t.Fatalf("rejected = %v", err)
	}
}

func TestClerkUserWorkerRoutesDevJobs(t *testing.T) {
	regular := &testClerk{prefix: "user_"}
	dev := &testClerk{prefix: "dev_", noop: true}
	m := New(Deps{Customers: []model.Clerk{regular, dev}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	w := &clerkUserWorker{m: m}
	for _, action := range []clerkUserAction{clerkUserActionBan, clerkUserActionDelete} {
		args := clerkUserArgs{Action: action, ClerkUserID: "dev_buyer"}
		if err := w.Work(context.Background(), &river.Job[clerkUserArgs]{JobRow: &rivertype.JobRow{}, Args: args}); err != nil {
			t.Fatal(err)
		}
	}
	if regular.calls != 0 || dev.calls != 2 || len(dev.banned) != 0 || len(dev.deleted) != 0 {
		t.Fatalf("regular calls = %d, dev calls = %d, dev writes = %v/%v", regular.calls, dev.calls, dev.banned, dev.deleted)
	}
}
