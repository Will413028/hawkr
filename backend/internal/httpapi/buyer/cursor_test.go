package buyer

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/buyer"
)

func TestBuyerCursorCodec(t *testing.T) {
	at := time.Date(2026, 9, 30, 1, 2, 3, 0, time.UTC)
	address := &model.AddressCursor{Default: true, Time: at, ID: 7}
	decodedAddress, err := addressCursor("address", encodeAddressCursor("address", address))
	if err != nil || decodedAddress == nil || *decodedAddress != *address {
		t.Fatalf("address=%+v err=%v", decodedAddress, err)
	}
	item := &model.Cursor{Time: at, ID: 9}
	decodedItem, err := rpc.ValidCursor[model.Cursor]("items", encodeCursor("items", item))
	if err != nil || decodedItem == nil || *decodedItem != *item {
		t.Fatalf("item=%+v err=%v", decodedItem, err)
	}
	member := merchantCursor{Spent: 100, StoreID: 3}
	decodedMember, err := rpc.PageCursor[merchantCursor]("member", rpc.TokenFor("member", member))
	if err != nil || decodedMember == nil || *decodedMember != member {
		t.Fatalf("member=%+v err=%v", decodedMember, err)
	}
	if _, err := rpc.ValidCursor[model.Cursor]("other", encodeCursor("items", item)); err == nil {
		t.Fatal("cross-scope cursor accepted")
	}
}

func TestValidCursorRejectsContentThatCannotBeACursor(t *testing.T) {
	for name, c := range map[string]model.Cursor{"zero id": {Time: time.Unix(1, 0).UTC()}, "zero time": {ID: 3}} {
		if _, err := rpc.ValidCursor[model.Cursor]("items", rpc.TokenFor("items", c)); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}
