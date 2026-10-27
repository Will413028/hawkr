package usecase

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sync/semaphore"

	"github.com/Will413028/hawkr/ai-service/internal/config"
)

func newWeighted(n int64) *semaphore.Weighted { return semaphore.NewWeighted(n) }

func asInFlight(err error, target **InFlightLimitError) bool { return errors.As(err, target) }

func TestTransformAdmission_RejectsWhenBudgetExhausted(t *testing.T) {
	a := &transformAdmission{sem: newWeighted(600), budget: 600}

	r1, err := a.admit(50)
	if err != nil {
		t.Fatalf("第一個請求應該進得來: %v", err)
	}
	r2, err := a.admit(50)
	if err != nil {
		t.Fatalf("第二個請求應該進得來: %v", err)
	}
	if _, err := a.admit(50); err == nil {
		t.Fatal("預算用完時第三個請求必須被拒，不能排隊")
	}

	r1()
	if r3, err := a.admit(50); err != nil {
		t.Fatalf("釋放後應該又進得來: %v", err)
	} else {
		r3()
	}
	r2()
}

func TestTransformAdmission_RejectsOversizedSingleRequest(t *testing.T) {
	a := &transformAdmission{sem: newWeighted(100), budget: 100}
	_, err := a.admit(1000)
	if err == nil {
		t.Fatal("單一請求超過總預算時必須立即回錯，不能永遠卡住")
	}
	var ifl *InFlightLimitError
	if !asInFlight(err, &ifl) {
		t.Fatalf("應回 *InFlightLimitError，實得 %T", err)
	}
	if ifl.NeedBytes <= ifl.BudgetBytes {
		t.Errorf("NeedBytes(%d) 應大於 BudgetBytes(%d)", ifl.NeedBytes, ifl.BudgetBytes)
	}
}

func TestTransformAdmission_ZeroSizedRequestStillCostsSomething(t *testing.T) {
	a := &transformAdmission{sem: newWeighted(2), budget: 2}
	r1, err := a.admit(0)
	if err != nil {
		t.Fatalf("空圖第一次應該進得來: %v", err)
	}
	r2, err := a.admit(0)
	if err != nil {
		t.Fatalf("空圖第二次應該進得來: %v", err)
	}
	if _, err := a.admit(0); err == nil {
		t.Fatal("零權重會讓無限多個空請求同時進場，必須至少佔一格")
	}
	r1()
	r2()
}

func TestTransformAdmission_WeightIsProportionalToImageSize(t *testing.T) {
	budget := int64(transformPeakBytesFactor) * 100
	a := &transformAdmission{sem: newWeighted(budget), budget: budget}
	if _, err := a.admit(100); err != nil {
		t.Fatalf("剛好用滿預算的請求應該進得來: %v", err)
	}
	if _, err := a.admit(1); err == nil {
		t.Fatal("預算用滿後再小的請求也該被拒")
	}
}

func TestInFlightBytesBudget_EnvOverride(t *testing.T) {
	cases := map[string]int64{
		"":         defaultInFlightBytesBudget,
		"0":        defaultInFlightBytesBudget,
		"-1":       defaultInFlightBytesBudget,
		"abc":      defaultInFlightBytesBudget,
		"33554432": 33554432,
	}
	for raw, want := range cases {
		t.Setenv(envInFlightBytesBudget, raw)
		if raw == "" {
			_ = os.Unsetenv(envInFlightBytesBudget)
		}
		if got := inFlightBytesBudget(); got != want {
			t.Errorf("%s=%q → %d, want %d", envInFlightBytesBudget, raw, got, want)
		}
	}
}

func TestTransformUseCase_StructLiteral_HasWorkingAdmission(t *testing.T) {
	uc := &TransformUseCase{}

	a := uc.admitter()
	if a == nil {
		t.Fatal("結構字面量建出來的 usecase 也必須拿到閘門——nil 會在 Transform 主路徑 SIGSEGV")
	}
	if a.budget != defaultInFlightBytesBudget {
		t.Errorf("預設預算 = %d, want %d", a.budget, defaultInFlightBytesBudget)
	}

	if uc.admitter() != a {
		t.Error("admitter() 每次都建新的 ⇒ semaphore 形同虛設，限不了併發")
	}

	release, err := a.admit(1)
	if err != nil {
		t.Fatalf("預設預算下小請求應該進得來: %v", err)
	}
	release()
}

func TestTransformUseCase_ConstructorAndLazyPathAgree(t *testing.T) {
	built := NewTransformUseCase(nil, &config.Config{})
	if got := built.admitter().budget; got != defaultInFlightBytesBudget {
		t.Errorf("建構子路徑的預算 = %d, want %d", got, defaultInFlightBytesBudget)
	}
}
