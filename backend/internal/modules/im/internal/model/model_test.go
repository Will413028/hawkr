package model

import (
	"reflect"
	"testing"
	"time"
)

// 在線狀態不提供最後上線時間：型別裡沒有任何時間欄位，欄位不存在比留空更不會被誤用。
func TestPresenceStateHasNoTimeField(t *testing.T) {
	t.Parallel()
	rt := reflect.TypeFor[PresenceState]()
	for i := range rt.NumField() {
		switch ft := rt.Field(i).Type; ft {
		case reflect.TypeFor[time.Time](), reflect.TypeFor[*time.Time](), reflect.TypeFor[time.Duration]():
			t.Errorf("PresenceState.%s 是時間型別 %v", rt.Field(i).Name, ft)
		}
	}
	if rt.NumField() != 2 {
		t.Errorf("PresenceState 欄位數 %d，預期只有 ConversationID 與 Online", rt.NumField())
	}
}
