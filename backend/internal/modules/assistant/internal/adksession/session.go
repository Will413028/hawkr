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

// liveSession 是 Create、Get、List 回給呼叫端的 session，也是 AppendEvent 就地更新的對象：
// runner 在同一輪 invocation 內一邊 AppendEvent、一邊從同一個 session 讀 events 與 state 組 prompt，
// 所以 AppendEvent 成功後要讓這個物件反映新 event（行為同 ADK 的記憶體與資料庫實作）。
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

// applyAppended 在 AppendEvent 的 transaction 提交之後呼叫：state 套完整的 delta（含 temp: key，
// temp 只活在這一輪 invocation 的記憶體裡），events 只放已剝除 temp: key 的版本。
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

// splitDelta 把一份 state delta 依前綴分成 app、user、session 三層（前綴已去掉，app、user 層）；
// temp: 前綴的 key 在這裡被丟掉，不會進任何一層。
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

// mergeStates 把三層 state 合成呼叫端看到的一份：app、user 層的 key 加回前綴。
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

// withoutTempKeys 回傳 StateDelta 不含 temp: key 的 event。沒有 temp: key 時回傳原物件；
// 有的話複製一份，不改呼叫端手上的 event（agent 的後續 callback 在同一輪還會用到 temp: key）。
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
