package adksession

import (
	"bytes"
	"reflect"
	"testing"

	"google.golang.org/adk/v2/session"
)

func TestSplitDeltaRoutesByPrefixAndDropsTemp(t *testing.T) {
	app, user, sess := splitDelta(map[string]any{"app:a": 1, "user:u": 2, "temp:t": 3, "k": 4})
	if !reflect.DeepEqual(app, map[string]any{"a": 1}) || !reflect.DeepEqual(user, map[string]any{"u": 2}) || !reflect.DeepEqual(sess, map[string]any{"k": 4}) {
		t.Fatalf("app=%v user=%v sess=%v", app, user, sess)
	}
}

func TestMergeStatesRestoresPrefixes(t *testing.T) {
	got := mergeStates(map[string]any{"a": 1}, map[string]any{"u": 2}, map[string]any{"k": 3})
	want := map[string]any{"app:a": 1, "user:u": 2, "k": 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestWithoutTempKeysCopiesInsteadOfMutating(t *testing.T) {
	ev := &session.Event{Actions: session.EventActions{StateDelta: map[string]any{"temp:t": 1, "k": 2}}}
	got := withoutTempKeys(ev)
	if _, ok := got.Actions.StateDelta["temp:t"]; ok || got.Actions.StateDelta["k"] != 2 {
		t.Fatalf("stored delta = %v", got.Actions.StateDelta)
	}
	if _, ok := ev.Actions.StateDelta["temp:t"]; !ok {
		t.Fatal("caller's event was mutated")
	}
	clean := &session.Event{Actions: session.EventActions{StateDelta: map[string]any{"k": 2}}}
	if withoutTempKeys(clean) != clean {
		t.Fatal("event without temp keys must be returned as is")
	}
}

// 寫進資料庫的 event JSON 不得含 temp: key，delta 也只能分流到對應的層（本機可跑，補 RunServiceTests 在 CI 才跑的缺口）。
func TestEncodeAppendStripsTempKeysFromPersistedBytes(t *testing.T) {
	ev := &session.Event{ID: "e", Actions: session.EventActions{StateDelta: map[string]any{"temp:secret": "x", "k": "v", "app:a": 1, "user:u": 2}}}
	enc, err := encodeAppend(ev)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(enc.eventJSON, []byte("temp:")) {
		t.Fatalf("persisted event JSON contains a temp: key: %s", enc.eventJSON)
	}
	if !bytes.Contains(enc.eventJSON, []byte(`"k":"v"`)) {
		t.Fatalf("non-temp key missing from persisted event JSON: %s", enc.eventJSON)
	}
	if bytes.Contains(enc.sessJSON, []byte("temp:")) || string(enc.sessJSON) != `{"k":"v"}` {
		t.Fatalf("session delta = %s", enc.sessJSON)
	}
	if !enc.hasApp || string(enc.appJSON) != `{"a":1}` || !enc.hasUser || string(enc.userJSON) != `{"u":2}` {
		t.Fatalf("app=%s user=%s", enc.appJSON, enc.userJSON)
	}
}
