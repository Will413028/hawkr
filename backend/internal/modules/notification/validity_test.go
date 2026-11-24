package notification

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
)

func TestNewPanicsWithoutValidityWhenATypeDeclaresStillValid(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		msg, _ := r.(string)
		if !strings.Contains(msg, "voucher_expiring") || !strings.Contains(msg, "Validity") {
			t.Errorf("預期建構時 panic 並指出型別與 Validity，得到 %v", r)
		}
	}()
	newModule(Deps{}, registry.Registry{})
}
