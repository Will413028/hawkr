package run

import (
	"errors"
	"fmt"
	"slices"
	"sort"
)

// Run 的狀態（與 schema 的 CHECK 相同）。
const (
	StatusQueued          = "QUEUED"
	StatusRunning         = "RUNNING"
	StatusWaitingForInput = "WAITING_FOR_INPUT"
	StatusCancelRequested = "CANCEL_REQUESTED"
	StatusSucceeded       = "SUCCEEDED"
	StatusFailed          = "FAILED"
	StatusCancelled       = "CANCELLED"
)

// edges 是 run 狀態機允許的邊（from → 可到的 to），也是唯一的來源：沒有出邊的狀態是終態，終態吸收。
var edges = map[string][]string{
	StatusQueued:          {StatusRunning, StatusCancelRequested, StatusFailed},
	StatusRunning:         {StatusWaitingForInput, StatusSucceeded, StatusFailed, StatusCancelRequested},
	StatusCancelRequested: {StatusCancelled, StatusFailed},
	StatusWaitingForInput: {StatusSucceeded, StatusCancelled},
	StatusSucceeded:       nil,
	StatusFailed:          nil,
	StatusCancelled:       nil,
}

// IsTerminal 回報狀態是否是終態（已知狀態而且沒有出邊）。
func IsTerminal(status string) bool {
	out, known := edges[status]
	return known && len(out) == 0
}

// CanTransition 回報 from → to 是否是允許的邊。
func CanTransition(from, to string) bool {
	return slices.Contains(edges[from], to)
}

// allowedFrom 回傳可以轉到 to 的所有來源狀態（排序後，輸出穩定）。
func allowedFrom(to string) []string {
	var out []string
	for from, tos := range edges {
		if slices.Contains(tos, to) {
			out = append(out, from)
		}
	}
	sort.Strings(out)
	return out
}

// publicFrom 是公開的 Transition 允許的來源：WAITING_FOR_INPUT 轉 SUCCEEDED 只在回答的提交點（同時解決中斷），公開入口不含這條邊。
func publicFrom(to string) []string {
	from := allowedFrom(to)
	if to == StatusSucceeded {
		from = slices.DeleteFunc(from, func(s string) bool { return s == StatusWaitingForInput })
	}
	return from
}

// activeStatuses 回傳還沒到終態的狀態：事件只能在這些狀態下寫入。
func activeStatuses() []string {
	var out []string
	for status, tos := range edges {
		if len(tos) > 0 {
			out = append(out, status)
		}
	}
	sort.Strings(out)
	return out
}

// ErrRunTerminal 表示 run 已經在終態：轉換與事件寫入都被拒絕。executor 收到它就該停止（run 已被取消、失敗或對帳收斂）。
var ErrRunTerminal = errors.New("run：已在終態")

// InvalidTransitionError 表示 run 目前的狀態不允許轉到 To（非終態、但沒有這條邊）。
type InvalidTransitionError struct {
	From, To string
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("run：不允許從 %s 轉到 %s", e.From, e.To)
}
