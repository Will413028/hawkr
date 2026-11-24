package intake

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
)

func TestFanoutChunkDefaultsAndClampsToThePageLimit(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct{ set, want int }{
		"沒設":   {0, DefaultFanoutChunk},
		"負數":   {-4, DefaultFanoutChunk},
		"合理":   {50, 50},
		"剛好上限": {registry.MaxFollowerPage, registry.MaxFollowerPage},
		"超過上限": {registry.MaxFollowerPage + 1, registry.MaxFollowerPage},
	} {
		s := &Service{d: Deps{FanoutChunk: c.set}}
		if got := s.fanoutChunk(); got != c.want {
			t.Errorf("%s：fanoutChunk(%d) = %d，預期 %d", name, c.set, got, c.want)
		}
	}
}
