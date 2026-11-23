package snapshot

import (
	"slices"
	"testing"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
)

const withCandidates = `"candidates":[{"categoryId":"21","label":"蘋果","path":"水果 > 蘋果"},{"categoryId":"22","label":"梨","path":"水果 > 梨"}]`

func TestConfirmationOfBuildsDisambiguationOnlyFromTheItemDisambiguationCard(t *testing.T) {
	cases := map[string]struct {
		payload string
		want    []string
	}{
		"item card":  {`{"stage":"ITEM",` + withCandidates + `,"question":{"slotKey":"SLOT_KEY_ITEM_DISAMBIGUATION","prompt":"哪一個？"}}`, []string{"蘋果", "梨"}},
		"other slot": {`{"stage":"ITEM",` + withCandidates + `,"question":{"slotKey":"SLOT_KEY_QUANTITY","prompt":"幾件？"}}`, nil},
		"no slot":    {`{"stage":"ITEM",` + withCandidates + `,"question":{"prompt":"?"}}`, nil},
	}
	for name, c := range cases {
		conf, labels, err := confirmationOf(run.OpenInterrupt{ID: uuid.NewV7(), Payload: []byte(c.payload)})
		if err != nil || conf == nil {
			t.Fatalf("%s：%v %v", name, conf, err)
		}
		if !slices.Equal(labels, c.want) {
			t.Errorf("%s：labels = %v，要 %v", name, labels, c.want)
		}
	}
}
