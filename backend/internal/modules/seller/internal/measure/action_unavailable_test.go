package measure

import (
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 被拒絕的動作回 failed_precondition，LocalizedMessage（apperr 的 Message）等於原因清單第一項的文字，也就是 API 的 MeasureActionUnavailability.message
// 用的同一句；清單為空時退回通用文字。
func TestActionUnavailableErrorSaysTheFirstReasonText(t *testing.T) {
	read := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	type cell struct {
		status model.MeasureStatus
		kind   model.MeasureKind
		state  model.MeasureState
	}
	var cells []cell
	for _, kind := range model.MeasureKindValues() {
		cells = append(cells, cell{model.MeasureStatusRevoked, kind, model.StateRevoked})
		if kind == model.MeasureWarning {
			cells = append(cells, cell{model.MeasureStatusEnforced, kind, model.StateIssued})
			continue
		}
		for _, state := range []model.MeasureState{model.StateActive, model.StateEnded, model.StateScheduled} {
			cells = append(cells, cell{model.MeasureStatusEnforced, kind, state})
		}
	}
	requestSets := [][]model.MeasureRequest{
		nil,
		{{Kind: model.RequestKindAppeal, State: model.RequestPending}},
		{{Kind: model.RequestKindAppeal, State: model.RequestRejected}},
		{{Kind: model.RequestKindRemediation, State: model.RequestPending}},
	}
	checked, withReasons := 0, 0
	for _, c := range cells {
		for _, requests := range requestSets {
			m := model.Measure{Kind: c.kind, Status: c.status, State: c.state, Basis: model.BasisViolation, ReadAt: read, Requests: requests}
			for _, action := range model.MeasureActionValues() {
				reasons := m.ActionUnavailableReasons(action)
				err := errActionUnavailable(m, action)
				var ae *apperr.Error
				if apperr.KindOf(err) != apperr.KindFailedPrecondition || !errors.As(err, &ae) {
					t.Fatalf("%+v %s：%v", c, action, err)
				}
				checked++
				if len(reasons) == 0 {
					if ae.Message == "" {
						t.Errorf("%+v %s：清單為空時也要有通用文字", c, action)
					}
					continue
				}
				withReasons++
				want := model.ActionUnavailableText(reasons[0], action).Text
				if want == "" || ae.Message != want {
					t.Errorf("%+v %s：錯誤文字 %q，第一個原因 %q 的文字是 %q", c, action, ae.Message, reasons[0], want)
				}
			}
		}
	}
	if checked != len(cells)*len(requestSets)*len(model.MeasureActionValues()) || withReasons == 0 {
		t.Fatalf("checked=%d withReasons=%d", checked, withReasons)
	}
}
