package usecase

import "errors"

// errBoom 是 usecase package 測試共用的 sentinel error，代表「底層依賴回傳了某個
// 非 nil error，細節不重要」。定義在這裡（Task 5）供本檔與後續 Task 7、12 重用，
// 避免每個測試檔各自宣告同名變數造成 redeclare。
//
// 本檔原本的其餘內容（fakeNarrativeReader、marshalPayload、
// TestBuildRecentNarratives_* 四個測試）測的是已刪除的 buildRecentNarratives／
// RecentNarrativeReader／SurfaceContext（procurement graph cutover，
// 2026-08-31，SurfaceRealizer 整層刪除）——那批行為已隨之刪除，只留下這個仍被
// package 內大量測試檔重用的 sentinel。
var errBoom = errors.New("boom")
