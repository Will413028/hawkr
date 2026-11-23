//go:build integration

package run_test

import (
	"context"
	"testing"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func TestResumeStoresTheOptionAndReplaysOnlyTheSameAnswer(t *testing.T) { // A9-D30 默認：agent_run_inputs.option_id
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	_, irq, _ := waitingParent(ctx, t, d, s, 1)
	in := run.ResumeInput{BuyerID: 1, InterruptID: irq, ClientRequestID: uuid.NewV7(), OptionID: "category-21", Locale: uitext.ZhTW}
	answer, err := s.Resume(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	var option *string
	var message string
	if err := d.Pool().QueryRow(ctx, `SELECT option_id, buyer_message FROM procurement.agent_run_inputs WHERE run_id = $1`, answer).Scan(&option, &message); err != nil {
		t.Fatal(err)
	}
	if option == nil || *option != "category-21" || message != "" {
		t.Fatalf("option_id = %v、buyer_message = %q，要選項與空訊息", option, message)
	}
	if again, err := s.Resume(ctx, in); err != nil || again != answer {
		t.Fatalf("同一個回答重送要回同一個 run：%v %v", again, err)
	}
	other := in
	other.OptionID = "category-22"
	if _, err := s.Resume(ctx, other); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("同一個 client_request_id 換選項：%v，要 INVALID_ARGUMENT", err)
	}
	typed := in
	typed.OptionID, typed.Message = "", "蘋果乾"
	if _, err := s.Resume(ctx, typed); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("同一個 client_request_id 換成文字：%v，要 INVALID_ARGUMENT", err)
	}
}

func TestRunInputHoldsExactlyOneOfMessageAndOption(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	_, irq, _ := waitingParent(ctx, t, d, s, 1)
	answer, err := s.Resume(ctx, resumeInput(irq, "文字"))
	if err != nil {
		t.Fatal(err)
	}
	for name, sql := range map[string]string{
		"both":    `UPDATE procurement.agent_run_inputs SET option_id = 'category-1' WHERE run_id = $1`,
		"neither": `UPDATE procurement.agent_run_inputs SET buyer_message = '' WHERE run_id = $1`,
	} {
		if _, err := d.Pool().Exec(ctx, sql, answer); err == nil {
			t.Errorf("%s：CHECK 要擋下訊息與選項不是恰一個", name)
		}
	}
}
