package procurementrelay

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
)

type recordingJobs struct {
	args []river.JobArgs
	fail error
}

func (q *recordingJobs) Enqueue(_ context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	if q.fail != nil {
		return q.fail
	}
	q.args = append(q.args, args)
	return nil
}

type fakeInvites struct{ sent, failed []uuid.UUID }

func (f *fakeInvites) MarkInviteSent(_ context.Context, id uuid.UUID) error {
	f.sent = append(f.sent, id)
	return nil
}

func (f *fakeInvites) MarkInviteFailed(_ context.Context, id uuid.UUID) error {
	f.failed = append(f.failed, id)
	return nil
}

type fakeSender struct {
	sent []rabbitmq.PublishArgs
	err  error
}

func (f *fakeSender) Send(_ context.Context, a rabbitmq.PublishArgs) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, a)
	return nil
}

var (
	publicationID = uuid.MustParse("0192a000-0000-7000-8000-000000000001")
	inviteA       = uuid.MustParse("0192a000-0000-7000-8000-0000000000a1")
	inviteB       = uuid.MustParse("0192a000-0000-7000-8000-0000000000b2")
)

func args() InviteCreatedArgs {
	return InviteCreatedArgs{InviteID: inviteA, PublicationID: publicationID, StoreID: 7496483081637855232}
}

func TestOnRfqPublishedEnqueuesOneJobPerInvite(t *testing.T) {
	q := &recordingJobs{}
	err := OnRfqPublished(q)(t.Context(), procurement.RfqPublished{
		PublicationID: publicationID,
		Invites:       []procurement.RfqInvite{{InviteID: inviteA, StoreID: 11}, {InviteID: inviteB, StoreID: 22}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []river.JobArgs{
		InviteCreatedArgs{InviteID: inviteA, PublicationID: publicationID, StoreID: 11},
		InviteCreatedArgs{InviteID: inviteB, PublicationID: publicationID, StoreID: 22},
	}
	if len(q.args) != 2 || q.args[0] != want[0] || q.args[1] != want[1] {
		t.Fatalf("jobs = %v，預期 %v", q.args, want)
	}
}

func TestOnRfqPublishedReturnsTheEnqueueError(t *testing.T) {
	boom := errors.New("boom")
	err := OnRfqPublished(&recordingJobs{fail: boom})(t.Context(), procurement.RfqPublished{Invites: []procurement.RfqInvite{{InviteID: inviteA, StoreID: 1}}})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v，排入失敗要讓發布回滾", err)
	}
}

func TestSuccessfulSendMarksTheInviteSent(t *testing.T) {
	invites, out := &fakeInvites{}, &fakeSender{}
	if err := NewWorker(invites, out).inviteCreated(t.Context(), args(), false); err != nil {
		t.Fatal(err)
	}
	if len(out.sent) != 1 || out.sent[0].Exchange != "procurement.events" || out.sent[0].RoutingKey != "procurement.rfq_publication_invite.created" || out.sent[0].EventID != inviteA.String() {
		t.Fatalf("sent = %+v", out.sent)
	}
	if len(invites.sent) != 1 || invites.sent[0] != inviteA || len(invites.failed) != 0 {
		t.Fatalf("sent=%v failed=%v", invites.sent, invites.failed)
	}
}

func TestFailedSendBeforeTheLastAttemptMarksNothing(t *testing.T) {
	invites, out := &fakeInvites{}, &fakeSender{err: errors.New("broker down")}
	if err := NewWorker(invites, out).inviteCreated(t.Context(), args(), false); err == nil {
		t.Fatal("送出失敗要回錯誤讓 River 重試")
	}
	if len(invites.sent)+len(invites.failed) != 0 {
		t.Fatalf("sent=%v failed=%v", invites.sent, invites.failed)
	}
}

func TestFailedSendOnTheLastAttemptMarksTheInviteFailed(t *testing.T) {
	invites, out := &fakeInvites{}, &fakeSender{err: errors.New("broker down")}
	if err := NewWorker(invites, out).inviteCreated(t.Context(), args(), true); err == nil {
		t.Fatal("用盡仍要回送出的錯誤")
	}
	if len(invites.failed) != 1 || invites.failed[0] != inviteA || len(invites.sent) != 0 {
		t.Fatalf("sent=%v failed=%v", invites.sent, invites.failed)
	}
}

func TestPayloadIsExactlyThreeStringKeys(t *testing.T) {
	body, err := payload(args())
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"inviteId": inviteA.String(), "publicationId": publicationID.String(), "storeId": "7496483081637855232"}
	if len(got) != len(want) {
		t.Fatalf("payload = %s", body)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("payload[%s] = %v，預期 %v", k, got[k], v)
		}
	}
}
