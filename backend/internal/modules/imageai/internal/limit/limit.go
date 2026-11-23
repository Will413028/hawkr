// Package limit 是每個 pod 同時進行的修圖數上限：計數 semaphore，跨 pod 不協調。
package limit

// Limiter 限制同時進行的數量。
type Limiter struct{ slots chan struct{} }

// New 建立上限為 n 的 Limiter；n 小於 1 時取 1。
func New(n int) *Limiter { return &Limiter{slots: make(chan struct{}, max(n, 1))} }

// TryAcquire 不等待地佔一個名額；滿了回 ok=false。release 只能呼叫一次。
func (l *Limiter) TryAcquire() (release func(), ok bool) {
	select {
	case l.slots <- struct{}{}:
		return func() { <-l.slots }, true
	default:
		return nil, false
	}
}
