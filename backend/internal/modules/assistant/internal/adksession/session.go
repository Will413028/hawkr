package adksession

import (
	"iter"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"google.golang.org/adk/v2/session"
)

type liveSession struct {
	appName   string
	userID    string
	sessionID string

	mu        sync.RWMutex
	events    []*session.Event
	state     map[string]any
	updatedAt time.Time
}

func (s *liveSession) ID() string      { return s.sessionID }
func (s *liveSession) AppName() string { return s.appName }
func (s *liveSession) UserID() string  { return s.userID }

func (s *liveSession) State() session.State { return &liveState{mu: &s.mu, state: s.state} }

// Events 回傳快照，呼叫端迭代時不必持有 session 的鎖。
func (s *liveSession) Events() session.Events {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return eventList(slices.Clone(s.events))
}

func (s *liveSession) LastUpdateTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updatedAt
}

func (s *liveSession) applyAppended(delta map[string]any, stored *session.Event, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(delta) > 0 {
		if s.state == nil {
			s.state = make(map[string]any)
		}
		maps.Copy(s.state, delta)
	}
	s.events = append(s.events, stored)
	s.updatedAt = at
}

type eventList []*session.Event

func (e eventList) All() iter.Seq[*session.Event] {
	return func(yield func(*session.Event) bool) {
		for _, ev := range e {
			if !yield(ev) {
				return
			}
		}
	}
}

func (e eventList) Len() int { return len(e) }

func (e eventList) At(i int) *session.Event {
	if i >= 0 && i < len(e) {
		return e[i]
	}
	return nil
}

type liveState struct {
	mu    *sync.RWMutex
	state map[string]any
}

func (s *liveState) Get(key string) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.state[key]
	if !ok {
		return nil, session.ErrStateKeyNotExist
	}
	return v, nil
}

func (s *liveState) All() iter.Seq2[string, any] {
	s.mu.RLock()
	snapshot := maps.Clone(s.state)
	s.mu.RUnlock()
	return func(yield func(string, any) bool) {
		for k, v := range snapshot {
			if !yield(k, v) {
				return
			}
		}
	}
}

func (s *liveState) Set(key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil {
		s.state = make(map[string]any)
	}
	s.state[key] = value
	return nil
}

func splitDelta(delta map[string]any) (app, user, sess map[string]any) {
	app, user, sess = map[string]any{}, map[string]any{}, map[string]any{}
	for k, v := range delta {
		if rest, ok := strings.CutPrefix(k, session.KeyPrefixApp); ok {
			app[rest] = v
		} else if rest, ok := strings.CutPrefix(k, session.KeyPrefixUser); ok {
			user[rest] = v
		} else if !strings.HasPrefix(k, session.KeyPrefixTemp) {
			sess[k] = v
		}
	}
	return app, user, sess
}

func mergeStates(app, user, sess map[string]any) map[string]any {
	merged := make(map[string]any, len(app)+len(user)+len(sess))
	maps.Copy(merged, sess)
	for k, v := range app {
		merged[session.KeyPrefixApp+k] = v
	}
	for k, v := range user {
		merged[session.KeyPrefixUser+k] = v
	}
	return merged
}

func withoutTempKeys(ev *session.Event) *session.Event {
	if len(ev.Actions.StateDelta) == 0 {
		return ev
	}
	kept := make(map[string]any, len(ev.Actions.StateDelta))
	for k, v := range ev.Actions.StateDelta {
		if !strings.HasPrefix(k, session.KeyPrefixTemp) {
			kept[k] = v
		}
	}
	if len(kept) == len(ev.Actions.StateDelta) {
		return ev
	}
	cp := *ev
	cp.Actions.StateDelta = kept
	return &cp
}
