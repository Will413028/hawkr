package usecase

import (
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sync/semaphore"
)

const (
	transformPeakBytesFactor = 6

	defaultInFlightBytesBudget int64 = 256 << 20

	envInFlightBytesBudget = "TRANSFORM_INFLIGHT_BYTES_BUDGET"
)

type InFlightLimitError struct {
	NeedBytes   int64
	BudgetBytes int64
}

func (e *InFlightLimitError) Error() string {
	if e.NeedBytes > e.BudgetBytes {
		return fmt.Sprintf("圖片過大：本次請求預估需要 %d bytes，超過單一請求可用的 %d bytes",
			e.NeedBytes, e.BudgetBytes)
	}
	return fmt.Sprintf("修圖服務忙碌中，請稍後再試（本次需要 %d bytes，目前可用額度不足，總額 %d bytes）",
		e.NeedBytes, e.BudgetBytes)
}

type transformAdmission struct {
	sem    *semaphore.Weighted
	budget int64
}

func newTransformAdmission() *transformAdmission {
	budget := inFlightBytesBudget()
	return &transformAdmission{sem: semaphore.NewWeighted(budget), budget: budget}
}

func inFlightBytesBudget() int64 {
	v := os.Getenv(envInFlightBytesBudget)
	if v == "" {
		return defaultInFlightBytesBudget
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return defaultInFlightBytesBudget
	}
	return n
}

func (a *transformAdmission) admit(imageBytes int) (release func(), err error) {
	need := int64(imageBytes) * transformPeakBytesFactor
	if need <= 0 {
		need = 1
	}
	if need > a.budget {

		return nil, &InFlightLimitError{NeedBytes: need, BudgetBytes: a.budget}
	}
	if !a.sem.TryAcquire(need) {
		return nil, &InFlightLimitError{NeedBytes: need, BudgetBytes: a.budget}
	}
	return func() { a.sem.Release(need) }, nil
}
