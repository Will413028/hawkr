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

// RateLimitTierNameForTest exposes classifyTier's verdict for one matched route
// under its metric label ("exempt", "default", "moderate", …).
//
// It exists for TestAIRoutesAreNotRateLimitExempt, which builds the REAL router
// (router.SetupRoutes) and therefore cannot live inside package middleware —
// router imports middleware. An external _test package may import router, so the
// assertion runs against the live route table rather than a hand-copied list
// that would quietly rot the moment someone mounts a new AI endpoint.
//
// Deliberately not a production export: the classification has exactly one
// caller in the binary (tieredRateLimiter), and an exported spelling would
// invite a second, divergent one.
func RateLimitTierNameForTest(fullPath string) string { return tierLabel(classifyTier(fullPath)) }
