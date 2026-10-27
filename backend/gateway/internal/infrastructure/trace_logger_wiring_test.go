package infrastructure

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestTraceLoggerWiringUsesSharedMiddleware(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	sourceDir := filepath.Dir(testFile)
	diSource, err := os.ReadFile(filepath.Join(sourceDir, "di.go"))
	if err != nil {
		t.Fatalf("read infrastructure wiring: %v", err)
	}

	if !bytes.Contains(diSource, []byte(`github.com/Will413028/hawkr/go-pkg/middleware/gin`)) {
		t.Fatal("gateway must import the shared gin middleware package")
	}
	if !bytes.Contains(diSource, []byte(`ginmw.TraceLogger(l)`)) {
		t.Fatal("gateway must register the shared TraceLogger implementation")
	}

	tracingIndex := bytes.Index(diSource, []byte(`r.Use(middleware.TracingMiddleware())`))
	traceLoggerIndex := bytes.Index(diSource, []byte(`r.Use(ginmw.TraceLogger(l))`))
	if tracingIndex < 0 || traceLoggerIndex < 0 || tracingIndex > traceLoggerIndex {
		t.Fatal("TracingMiddleware must be registered before TraceLogger")
	}

	localImplementation := filepath.Join(sourceDir, "..", "middleware", "trace_logger.go")
	if _, err := os.Stat(localImplementation); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("gateway-local TraceLogger must be removed, stat error: %v", err)
	}
}
