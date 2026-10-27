package domain

import (
	"context"
	"errors"
	"time"

	"github.com/looplab/fsm"
)

const (
	EventSubmit        = "submit"
	EventForward       = "forward"        // admin approve + bind success
	EventForwardFailed = "forward_failed" // admin approve + bind failure
	EventReject        = "reject"
	EventCancel        = "cancel"
	EventResetDraft    = "reset_to_draft"
)

const (
	StateDraft         = "draft"
	StateSubmitted     = "submitted"
	StateForwarded     = "forwarded"
	StateForwardFailed = "forward_failed"
	StateRejected      = "rejected"
	StateCancelled     = "cancelled"
)

var certStateLabelZh = map[string]string{
	StateDraft:         "草稿",
	StateSubmitted:     "待審核",
	StateForwarded:     "已送出至金流商",
	StateForwardFailed: "金流商轉送失敗",
	StateRejected:      "已退回",
	StateCancelled:     "已取消",
}

func certStateLabel(state string) string {
	if label, ok := certStateLabelZh[state]; ok {
		return label
	}
	return state
}

var eventToDst = map[string]string{
	EventSubmit:        StateSubmitted,
	EventForward:       StateForwarded,
	EventForwardFailed: StateForwardFailed,
	EventReject:        StateRejected,
	EventCancel:        StateCancelled,
	EventResetDraft:    StateDraft,
}

func newCertificationFSM(app *CertificationApplication, initialState string) *fsm.FSM {
	return fsm.NewFSM(
		initialState,
		fsm.Events{
			{Name: EventSubmit, Src: []string{StateDraft}, Dst: StateSubmitted},
			{Name: EventForward, Src: []string{StateSubmitted, StateForwardFailed}, Dst: StateForwarded},
			{Name: EventForwardFailed, Src: []string{StateSubmitted, StateForwardFailed}, Dst: StateForwardFailed},
			{Name: EventReject, Src: []string{StateSubmitted, StateForwardFailed}, Dst: StateRejected},
			{Name: EventCancel, Src: []string{StateDraft, StateSubmitted, StateForwardFailed}, Dst: StateCancelled},
			{Name: EventResetDraft, Src: []string{StateRejected, StateForwarded}, Dst: StateDraft},
		},
		fsm.Callbacks{
			"after_event": func(_ context.Context, e *fsm.Event) {
				status, err := CertificationStatusString(e.Dst)
				if err != nil {
					return
				}
				app.Status = status
				app.UpdatedAt = app.pendingNow
			},
			"after_submit": func(_ context.Context, _ *fsm.Event) {
				now := app.pendingNow
				app.SubmittedAt = &now
			},
			"after_forward": func(_ context.Context, _ *fsm.Event) {
				now := app.pendingNow
				app.ForwardedAt = &now
			},
			"after_reject": func(_ context.Context, _ *fsm.Event) {
				now := app.pendingNow
				app.RejectedAt = &now
			},
		},
	)
}

func fsmFireEvent(f *fsm.FSM, event string, now time.Time, app *CertificationApplication) error {
	app.pendingNow = now
	err := f.Event(context.Background(), event)
	if err == nil {
		return nil
	}
	var noTransition fsm.NoTransitionError
	if errors.As(err, &noTransition) {
		return nil
	}
	dst := eventToDst[event]
	return &ErrInvalidTransition{From: f.Current(), To: dst}
}
