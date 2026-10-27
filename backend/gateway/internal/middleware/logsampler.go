package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// emitLog is the shape of pkglogger's level functions (Warn, Error, ...). It is
// taken as a parameter so a truncation notice is emitted at the SAME level as
// the line it accounts for: reporting a sampled ERROR's true scale at WARN would
// hide it from whatever alerts on the ERROR.
type emitLog func(context.Context, string, ...zap.Field)

// logSampler admits one log line per key per window, with a hard ceiling on
// lines per window. Keys are forgotten by clearing the whole set when the window
// rolls rather than by LRU eviction, so key cardinality never causes
// re-admission of a key that was already logged.
//
// It exists because a per-request log line on a failure path is an availability
// hazard of its own: on 2026-07-29 a single 4.8 req/s client would have written
// ~410k lines/day from ONE line, flooding the pipeline exactly when it had to
// stay readable. Two mechanisms that look like fixes are not:
//
//   - a lower log LEVEL emits the identical number of lines;
//   - LRU eviction bounds memory, not output — past its capacity every repeat
//     looks like a fresh key and gets its own line, so a distributed flood
//     degenerates to one line per request (audited at 50k sources × 2 blocks →
//     100k lines).
//
// Every call site holds its OWN sampler: a shared budget would let the noisiest
// site spend the whole window and silence the others, which is the failure mode
// sampling exists to prevent.
//
// It holds no Redis client, issues no I/O and takes no allocation per admitted
// key beyond the set itself: a flood must never generate load of its own.
type logSampler struct {
	mu           sync.Mutex
	seen         map[string]struct{}
	windowStart  time.Time
	suppressed   int
	window       time.Duration
	maxPerWindow int
	now          func() time.Time // injectable for tests; defaults to time.Now
}

func newLogSampler(window time.Duration, maxPerWindow int) *logSampler {
	return &logSampler{
		seen:         make(map[string]struct{}),
		window:       window,
		maxPerWindow: maxPerWindow,
		now:          time.Now,
	}
}

// admit reports whether key earns a line in the current window. The second
// return is how many occurrences the just-ended window suppressed — both repeats
// of an already-logged key and keys turned away by the cap — and is non-zero
// only on the first call after a window rolls. Without it a sampled incident
// reads as a single event and its severity is under-read by orders of magnitude.
// A flood that stops outright never reports its tail: this is a volume guard,
// not an accounting record.
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

// admitWithNotice is admit plus the reporting of the previous window's
// suppressed count through emit, so no caller can sample without also accounting
// for what it hid.
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

// routeKey identifies a request by its MATCHED route rather than its raw URL.
// A raw URL carries ids and query strings, so a caller could mint a fresh
// sampler key — and therefore a fresh line — on every request, which is exactly
// the property sampling has to deny it.
func routeKey(c *gin.Context) string {
	if p := c.FullPath(); p != "" {
		return p
	}
	return "unmatched"
}
