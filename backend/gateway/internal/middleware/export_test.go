package middleware

// SetDiagnosticBodyLoggingConfigPathForTest overrides the diagnostic config
// path for tests in the middleware_test (external) package. Only compiled
// as part of `go test` — this file's _test.go suffix means it never becomes
// part of the production binary, unlike a normal exported function would.
func SetDiagnosticBodyLoggingConfigPathForTest(t interface{ Cleanup(func()) }, path string) {
	orig := diagnosticBodyLoggingConfigPath
	diagnosticBodyLoggingConfigPath = path
	t.Cleanup(func() { diagnosticBodyLoggingConfigPath = orig })
}
