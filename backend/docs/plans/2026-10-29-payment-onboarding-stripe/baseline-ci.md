# 驗收基線：commit `1c0b246` 的 CI 失敗

產生：`gh run list --commit $(git rev-parse 1c0b246)`，每個失敗 run 跑 `gh run view <id> --log-failed`，以下面的 regex 抽出錯誤行（去掉時間戳記，相同的只留一筆）。2026-11-02 抽取。本階段的驗收：改到的 job 與這裡逐行比對，新增為 0。

## backend migration inventory（run 40744254471）

- 失敗的 job：舊服務的去向是否齊全
- 失敗的 step：check-migration-inventory
- 抽取 regex：`^✘`，16 行

```
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:458：pricebook-service 已不在，但「PricebookService / GetMarketOverview」還沒有 disposition
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:459：pricebook-service 已不在，但「PricebookService / CreateMarketWatchTarget」還沒有 disposition
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:460：pricebook-service 已不在，但「PricebookService / DeleteMarketWatchTarget」還沒有 disposition
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:461：pricebook-service 已不在，但「PricebookService / GetObservedPrice」還沒有 disposition
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:462：pricebook-service 已不在，但「PricebookService / GetEstimatedItemPrice」還沒有 disposition
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:463：pricebook-service 已不在，但「PricebookService / GetForecastCurve」還沒有 disposition
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:464：pricebook-service 已不在，但「PricebookService / GetTrendSummary」還沒有 disposition
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:465：pricebook-service 已不在，但「PricebookService / SearchCropCatalog」還沒有 disposition
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:466：pricebook-service 已不在，但「PricebookService / SearchPriceableEntities」還沒有 disposition
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:467：pricebook-service 已不在，但「PricebookService / ListGroupCropOptions」還沒有 disposition
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:468：pricebook-service 已不在，但「PricebookService / GetSpatialSurface」還沒有 disposition
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:469：pricebook-service 已不在，但「PricebookService / GetRegionalComparison」還沒有 disposition
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:470：pricebook-service 已不在，但「PricebookService / ResolveCategoryMapping」還沒有 disposition
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:471：pricebook-service 已不在，但「PricebookService / UpsertCategoryMapping」還沒有 disposition
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:472：pricebook-service 已不在，但「PricebookService / ListCategoryMappingWorklist」還沒有 disposition
舊服務的去向是否齊全 :: ✘ rpc-servers.csv:473：pricebook-service 已不在，但「PricebookService / GetCategoryPriceRollup」還沒有 disposition
```

## backend monolith（run 40744254488）

- 失敗的 job：golangci-lint（一般／integration）
- 失敗的 step：golangci-lint、golangci-lint（integration tag）
- 抽取 regex：`\.go:\d+:\d+: |issues`，42 行

```
golangci-lint（一般／integration） :: only-new-issues: false
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/httpapi/buyer/app.go:167:2: missing cases in switch of type buyerv1alpha1.Gender: buyerv1alpha1.Gender_GENDER_UNSPECIFIED (exhaustive)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/legacy/gatewayproxy/proxy.go:106:2: missing cases in switch of type sellerreason.Class: sellerreason.None (exhaustive)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/delegation/internal/model/delegation_rules.go:231:2: missing cases in switch of type model.DelegationDenial: model.DenialStoreNotOperable, model.DenialStoreClosed (exhaustive)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/internal/measure/update.go:146:3: missing cases in switch of type measure.endsChange: measure.endsUnchanged (exhaustive)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/internal/model/kyc.go:196:2: missing cases in switch of type model.KYCIssueCode: model.IssueMissing, model.IssueOther (exhaustive)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/internal/model/measure.go:766:3: missing cases in switch of type model.MeasureAction: model.MeasureActionShorten, model.MeasureActionRevoke, model.MeasureActionEnd (exhaustive)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/internal/model/store_action.go:170:2: missing cases in switch of type model.StoreAction: model.StoreActionCreateMeasure, model.StoreActionDelegate, model.StoreActionRebindPayUni (exhaustive)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/internal/model/store_action_test.go:116:5: missing cases in switch of type model.Requirement: model.RequirementKYCApproved, model.RequirementStoreName, model.RequirementLogo, model.RequirementTownCode (exhaustive)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/arch/sqlc.go:204:6: weakCond: suspicious `object != nil && strings.EqualFold(object[1], "TYPE")`; nil check may not be enough, check for len (gocritic)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/agreement_queries_test.go:40:7: weakCond: suspicious `set == nil || !regexp.MustCompile(`(?is)^\s*client_ip\s*=\s*NULL\s*,\s*user_agent\s*=\s*NULL\s*,\s*pii_purged_at\s*=\s*sqlc\.arg\(now\)\s*$`).MatchString(set[1])`; nil check may not be enough, check for len (gocritic)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/internal/measure/revoke.go:209:9: appendAssign: append result not assigned to the same slice (gocritic)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/internal/model/agreement_test.go:86:72: mapKey: suspicious whitespace in " 203.0.113.9 " key (gocritic)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/identity/internal/applerevoke/apple.go:268:15: G117: Marshaled struct field "RefreshToken" (JSON key "refresh_token") matches secret pattern (gosec)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/identity/internal/applerevoke/apple_test.go:240:12: G117: Marshaled struct field "RefreshToken" (JSON key "refresh_token") matches secret pattern (gosec)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/identity/internal/applerevoke/apple_test.go:286:23: G101: Potential hardcoded credentials (gosec)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/internal/measure/measure.go:123:37: G602: slice index out of range (gosec)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/internal/measure/appeal.go:289:4: ineffectual assignment to after (ineffassign)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/internal/model/measure_test.go:598:48: QF1001: could apply De Morgan's law (staticcheck)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/internal/model/payuni_state_test.go:94:53: QF1001: could apply De Morgan's law (staticcheck)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/httpapi/social/characterization/rest_test.go:25:6: func newRESTDriver is unused (unused)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/httpapi/social/characterization/rest_test.go:33:6: func restBusinessKind is unused (unused)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/internal/profile/profile.go:187:6: func storeIDsOf is unused (unused)
golangci-lint（一般／integration） :: 22 issues:
golangci-lint（一般／integration） :: ##[error]issues found
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/measure_integration_test.go:243:19: Error return value of `tx.Rollback` is not checked (errcheck)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/measure_integration_test.go:282:19: Error return value of `tx.Rollback` is not checked (errcheck)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/httpapi/seller/admin_integration_test.go:334:3: missing cases in switch of type sellerv1alpha1.TerminationGround: sellerv1alpha1.TerminationGround_TERMINATION_GROUND_UNSPECIFIED, sellerv1alpha1.TerminationGround_TERMINATION_GROUND_CEASED_BUSINESS, sellerv1alpha1.TerminationGround_TERMINATION_GROUND_DISSOLUTION, sellerv1alpha1.TerminationGround_TERMINATION_GROUND_BANKRUPTCY, sellerv1alpha1.TerminationGround_TERMINATION_GROUND_LICENSE_REVOKED, sellerv1alpha1.TerminationGround_TERMINATION_GROUND_FOOD_SAFETY_INCIDENT, sellerv1alpha1.TerminationGround_TERMINATION_GROUND_FRAUD, sellerv1alpha1.TerminationGround_TERMINATION_GROUND_PLATFORM_SECURITY, sellerv1alpha1.TerminationGround_TERMINATION_GROUND_OTHER_SERIOUS, sellerv1alpha1.TerminationGround_TERMINATION_GROUND_LINKED_ACCOUNT (exhaustive)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/social/concurrency_integration_test.go:91:2: missing cases in switch of type apperr.Kind: apperr.KindInternal, apperr.KindInvalidInput, apperr.KindVersionConflict, apperr.KindAlreadyExists, apperr.KindUnauthenticated, apperr.KindPermissionDenied, apperr.KindUnavailable, apperr.KindResourceExhausted (exhaustive)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/social/counts_integration_test.go:208:2: missing cases in switch of type apperr.Kind: apperr.KindInternal, apperr.KindVersionConflict, apperr.KindAlreadyExists, apperr.KindUnauthenticated, apperr.KindUnavailable, apperr.KindResourceExhausted (exhaustive)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/social/counts_integration_test.go:83:9: G404: Use of weak random number generator (math/rand or math/rand/v2 instead of crypto/rand) (gosec)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/httpapi/seller/kyc_integration_test.go:28:2: ST1019: package "github.com/Will413028/hawkr/backend/internal/modules/seller" is being imported more than once (staticcheck)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/httpapi/seller/kyc_integration_test.go:29:2: ST1019(related information): other import of "github.com/Will413028/hawkr/backend/internal/modules/seller" (staticcheck)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/exposure_integration_test.go:340:3: QF1002: could use tagged switch on st.Capability (staticcheck)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/inquiries_integration_test.go:81:5: QF1001: could apply De Morgan's law (staticcheck)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/inquiries_integration_test.go:783:6: QF1001: could apply De Morgan's law (staticcheck)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/social/writes_integration_test.go:470:14: ST1023: should omit type int64 from declaration; it will be inferred from the right-hand side (staticcheck)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/httpapi/content/content_integration_test.go:623:6: func ptr is unused (unused)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/content/content_integration_test.go:112:6: func ptr is unused (unused)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/seller_integration_test.go:1146:19: func (*fixture).kycApprovedStore is unused (unused)
golangci-lint（一般／integration） :: ##[error]/home/runner/work/hawkr/hawkr/backend/internal/modules/seller/seller_integration_test.go:1152:19: func (*fixture).kycApprovedStoreWithAccount is unused (unused)
golangci-lint（一般／integration） :: 37 issues:
```

## seller-app CI（run 40744254599）

- 失敗的 job：quality / Unit and widget tests
- 失敗的 step：Enforce coverage ratchet
- 抽取 regex：`Coverage ratchet failed|Coverage fell below`，3 行

```
quality / Unit and widget tests :: Coverage ratchet failed:
quality / Unit and widget tests :: - Coverage fell below checked-in baseline: 47.77% (21376/44745) < 48.15% (22026/45745).
quality / Unit and widget tests :: - Coverage fell below base-branch floor: 47.77% (21376/44745) < 48.15% (22026/45745).
```

## website e2e（run 40744254588）

- 失敗的 job：Storybook 互動測試；Playwright e2e (1/8) (2/8) (3/8) (5/8)
- 失敗的 step：Run pnpm run storybook:test；e2e（產生基準）
- 抽取 regex：`✘|^\s*\d+\) |Error: |product-certification|failed$|\d+ failed`，43 行

```
Storybook 互動測試 :: 6:22:20 PM [vite] (client) Pre-transform error: Failed to resolve import "../styles/components/product-certification.css" from ".storybook/preview.tsx". Does the file exist?
Storybook 互動測試 :: 7  |  import "../styles/components/product-certification.css";
Storybook 互動測試 :: 6:22:20 PM [vite] Internal server error: Failed to resolve import "../styles/components/product-certification.css" from ".storybook/preview.tsx". Does the file exist?
Storybook 互動測試 :: 6:22:21 PM [vite] Internal server error: Failed to resolve import "../styles/components/product-certification.css" from ".storybook/preview.tsx". Does the file exist?
Storybook 互動測試 :: 6:22:22 PM [vite] Internal server error: Failed to resolve import "../styles/components/product-certification.css" from ".storybook/preview.tsx". Does the file exist?
Storybook 互動測試 :: 6:22:23 PM [vite] Internal server error: Failed to resolve import "../styles/components/product-certification.css" from ".storybook/preview.tsx". Does the file exist?
Storybook 互動測試 :: 6:22:24 PM [vite] Internal server error: Failed to resolve import "../styles/components/product-certification.css" from ".storybook/preview.tsx". Does the file exist?
Storybook 互動測試 :: 6:22:25 PM [vite] Internal server error: Failed to resolve import "../styles/components/product-certification.css" from ".storybook/preview.tsx". Does the file exist?
Storybook 互動測試 ::  Test Files  38 failed (38)
Playwright e2e (1/8) :: Error: ENVIRONMENT_FALLBACK
Playwright e2e (1/8) :: [prerenderSafe] soft-fail during build: [TypeError: fetch failed] {
Playwright e2e (1/8) :: [cause]: Error: bad port
Playwright e2e (1/8) :: [WebServer] Error: ENVIRONMENT_FALLBACK
Playwright e2e (1/8) :: ✘  48 [chromium] › tests/e2e/procurement-assistant-rfq-draft.spec.ts:17:2 › procurement assistant — RFQ confirmation › confirms a guided server projection without publishing a legacy QuoteBatch @ui (19.9s)
Playwright e2e (1/8) :: 1) [chromium] › tests/e2e/procurement-assistant-rfq-draft.spec.ts:17:2 › procurement assistant — RFQ confirmation › confirms a guided server projection without publishing a legacy QuoteBatch @ui
Playwright e2e (1/8) :: Error: expect(locator).toBeDisabled() failed
Playwright e2e (1/8) :: Error: element(s) not found
Playwright e2e (1/8) :: 1 failed
Playwright e2e (3/8) :: Error: ENVIRONMENT_FALLBACK
Playwright e2e (3/8) :: [prerenderSafe] soft-fail during build: [TypeError: fetch failed] {
Playwright e2e (3/8) :: [cause]: Error: bad port
Playwright e2e (3/8) :: [WebServer] Error: ENVIRONMENT_FALLBACK
Playwright e2e (3/8) :: ✘  27 [chromium] › tests/e2e/public-static-routes.spec.ts:345:2 › canonical public static route browser health › public static inventory stays explicit and complete @ui (3ms)
Playwright e2e (3/8) :: 1) [chromium] › tests/e2e/public-static-routes.spec.ts:345:2 › canonical public static route browser health › public static inventory stays explicit and complete @ui
Playwright e2e (3/8) :: Error: expect(received).toHaveLength(expected)
Playwright e2e (3/8) :: 1 failed
Playwright e2e (5/8) :: Error: ENVIRONMENT_FALLBACK
Playwright e2e (5/8) :: [prerenderSafe] soft-fail during build: [TypeError: fetch failed] {
Playwright e2e (5/8) :: [cause]: Error: bad port
Playwright e2e (5/8) :: [WebServer] Error: ENVIRONMENT_FALLBACK
Playwright e2e (5/8) :: ✘  47 [mobile] › tests/e2e/procurement-assistant-rfq-draft.spec.ts:17:2 › procurement assistant — RFQ confirmation › confirms a guided server projection without publishing a legacy QuoteBatch @ui (18.8s)
Playwright e2e (5/8) :: 1) [mobile] › tests/e2e/procurement-assistant-rfq-draft.spec.ts:17:2 › procurement assistant — RFQ confirmation › confirms a guided server projection without publishing a legacy QuoteBatch @ui
Playwright e2e (5/8) :: Error: expect(locator).toBeDisabled() failed
Playwright e2e (5/8) :: Error: element(s) not found
Playwright e2e (5/8) :: 1 failed
Playwright e2e (2/8) :: Error: ENVIRONMENT_FALLBACK
Playwright e2e (2/8) :: [prerenderSafe] soft-fail during build: [TypeError: fetch failed] {
Playwright e2e (2/8) :: [cause]: Error: bad port
Playwright e2e (2/8) :: [WebServer] Error: ENVIRONMENT_FALLBACK
Playwright e2e (2/8) :: ✘  36 [chromium] › tests/e2e/protected-static-routes.spec.ts:327:2 › authenticated protected static route smoke › protected static inventory stays complete @ui (4ms)
Playwright e2e (2/8) :: 1) [chromium] › tests/e2e/protected-static-routes.spec.ts:327:2 › authenticated protected static route smoke › protected static inventory stays complete @ui
Playwright e2e (2/8) :: Error: expect(received).toHaveLength(expected)
Playwright e2e (2/8) :: 1 failed
```
