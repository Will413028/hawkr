package interceptor

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	testRegisteredMethod   = "/hawkr.account.AccountService/GetWebSession"
	testUnregisteredMethod = "/hawkr.account.AccountService/GetAccount"
)

// runLogging drives the unary interceptor once and returns the single log entry
// it emitted for the call's outcome (the ".start" entry is filtered out).
func runLogging(t *testing.T, fullMethod string, handlerErr error, opts ...LoggingOption) (observer.LoggedEntry, error) {
	t.Helper()

	core, logs := observer.New(zapcore.DebugLevel)
	intercept := LoggingInterceptor(zap.New(core), opts...)

	_, err := intercept(
		context.Background(),
		struct{}{},
		&grpc.UnaryServerInfo{FullMethod: fullMethod},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			if handlerErr != nil {
				return nil, handlerErr
			}
			return struct{}{}, nil
		},
	)

	var outcome []observer.LoggedEntry
	for _, e := range logs.All() {
		if ev, ok := e.ContextMap()["event"].(string); ok && len(ev) > 6 && ev[len(ev)-6:] == ".start" {
			continue
		}
		outcome = append(outcome, e)
	}
	if len(outcome) != 1 {
		t.Fatalf("expected exactly 1 outcome log entry, got %d", len(outcome))
	}
	return outcome[0], err
}

// A registered (method, code) pair is a business answer: info level, ".expected"
// event, and the error is still handed back to the caller unchanged.
func TestLoggingInterceptor_ExpectedCodeLogsAtInfo(t *testing.T) {
	want := status.Error(codes.NotFound, "web session not found")

	entry, err := runLogging(t, testRegisteredMethod, want,
		WithExpectedCodes(map[string][]codes.Code{
			testRegisteredMethod: {codes.NotFound},
		}))

	if entry.Level != zapcore.InfoLevel {
		t.Errorf("expected info level for a declared-expected NotFound, got %v", entry.Level)
	}
	if got := entry.ContextMap()["event"]; got != "AccountService.GetWebSession.expected" {
		t.Errorf("event = %v, want AccountService.GetWebSession.expected", got)
	}
	if got := entry.ContextMap()["code"]; got != "NotFound" {
		t.Errorf("code field = %v, want NotFound (still greppable)", got)
	}
	if status.Code(err) != codes.NotFound {
		t.Errorf("interceptor must return the error untouched, got %v", err)
	}
}

// The load-bearing scoping guarantee #1: the SAME code on a method that did not
// opt in still logs at error level. Guards against the downgrade being widened
// into "NotFound is never an error".
func TestLoggingInterceptor_SameCodeOnUnregisteredMethodStaysError(t *testing.T) {
	entry, _ := runLogging(t, testUnregisteredMethod, status.Error(codes.NotFound, "account not found"),
		WithExpectedCodes(map[string][]codes.Code{
			testRegisteredMethod: {codes.NotFound},
		}))

	if entry.Level != zapcore.ErrorLevel {
		t.Errorf("NotFound on an unregistered method must stay error level, got %v", entry.Level)
	}
	if got := entry.ContextMap()["event"]; got != "AccountService.GetAccount.error" {
		t.Errorf("event = %v, want AccountService.GetAccount.error", got)
	}
}

// The load-bearing scoping guarantee #2: registering a method excuses only the
// codes listed for it — a real fault on that same method still alerts.
func TestLoggingInterceptor_UnlistedCodeOnRegisteredMethodStaysError(t *testing.T) {
	entry, _ := runLogging(t, testRegisteredMethod, status.Error(codes.Internal, "db exploded"),
		WithExpectedCodes(map[string][]codes.Code{
			testRegisteredMethod: {codes.NotFound},
		}))

	if entry.Level != zapcore.ErrorLevel {
		t.Errorf("Internal on a registered method must stay error level, got %v", entry.Level)
	}
	if got := entry.ContextMap()["event"]; got != "AccountService.GetWebSession.error" {
		t.Errorf("event = %v, want AccountService.GetWebSession.error", got)
	}
}

// No options = historical behaviour, so the other nine services that call
// LoggingInterceptor(logger) are provably unaffected by this change.
func TestLoggingInterceptor_NoOptionsKeepsErrorLevel(t *testing.T) {
	entry, _ := runLogging(t, testRegisteredMethod, status.Error(codes.NotFound, "web session not found"))

	if entry.Level != zapcore.ErrorLevel {
		t.Errorf("without WithExpectedCodes, NotFound must log at error level, got %v", entry.Level)
	}
	if got := entry.ContextMap()["event"]; got != "AccountService.GetWebSession.error" {
		t.Errorf("event = %v, want AccountService.GetWebSession.error", got)
	}
}

func TestLoggingInterceptor_SuccessLogsEndAtInfo(t *testing.T) {
	entry, err := runLogging(t, testRegisteredMethod, nil,
		WithExpectedCodes(map[string][]codes.Code{
			testRegisteredMethod: {codes.NotFound},
		}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Level != zapcore.InfoLevel {
		t.Errorf("success must log at info level, got %v", entry.Level)
	}
	if got := entry.ContextMap()["event"]; got != "AccountService.GetWebSession.end" {
		t.Errorf("event = %v, want AccountService.GetWebSession.end", got)
	}
}

// A bare method name must not match: the map is keyed by the full method that
// grpc.UnaryServerInfo actually reports.
func TestLoggingInterceptor_BareMethodNameDoesNotMatch(t *testing.T) {
	entry, _ := runLogging(t, testRegisteredMethod, status.Error(codes.NotFound, "web session not found"),
		WithExpectedCodes(map[string][]codes.Code{
			"GetWebSession": {codes.NotFound},
		}))

	if entry.Level != zapcore.ErrorLevel {
		t.Errorf("a bare method key must not match a full method, got %v", entry.Level)
	}
}
