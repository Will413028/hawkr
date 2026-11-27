package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type emitLog func(context.Context, string, ...zap.Field)

type logSampler struct {
	mu           sync.Mutex
	seen         map[string]struct{}
	windowStart  time.Time
	suppressed   int
	window       time.Duration
	maxPerWindow int
	now          func() time.Time
}

func newLogSampler(window time.Duration, maxPerWindow int) *logSampler {
	return &logSampler{
		seen:         make(map[string]struct{}),
		window:       window,
		maxPerWindow: maxPerWindow,
		now:          time.Now,
	}
}

func (s *logSampler) admit(key string) (bool, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	dropped := 0
	if s.windowStart.IsZero() || now.Sub(s.windowStart) >= s.window {
		dropped = s.suppressed
		s.seen = make(map[string]struct{})
		s.suppressed = 0
		s.windowStart = now
	}
	if _, ok := s.seen[key]; ok || len(s.seen) >= s.maxPerWindow {
		s.suppressed++
		return false, dropped
	}
	s.seen[key] = struct{}{}
	return true, dropped
}

func (s *logSampler) admitWithNotice(ctx context.Context, emit emitLog, key, noticeMsg string) bool {
	admitted, dropped := s.admit(key)
	if dropped > 0 {
		emit(ctx, noticeMsg,
			zap.Int("dropped", dropped),
			zap.Duration("window", s.window),
		)
	}
	return admitted
}

func routeKey(c *gin.Context) string {
	if p := c.FullPath(); p != "" {
		return p
	}
	return "unmatched"
}
