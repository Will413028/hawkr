# Hawkr 幫找的 LLM 抽取：從「不重試」改為「有條件重試」，逾時上限三次調整

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-18（決策過程 2026-08-15 ～ 2026-09-16） |
| 影響範圍 | ai-service `internal/procurement/usecase/extractor_provider.go`、`extract.go` |
| 相關 issue | #2412 |

規格本身見 `docs/詳細設計書/55-Hawkr 幫找採購助手（Procurement Assistant）詳細設計書.md` §7.3。

## 脈絡：原本的「不重試」是沒有數據的判斷

最初的決策是「LLM 逾時不重試」，理由是「逾時多半是上游壅塞，重試會延長使用者等待且成功率不高；
降級路徑本身已是可用結果」。

問題在於當時**零指標、零 log**——這個判斷本身沒有任何數據支撐。

## 量測

補上指標後，用同一版程式碼對 dev 量測近 7 天 `procurement_answer_patches`
（`source = 'LLM_EXTRACT'`）的降級率：**18.4%（114 輪中 21 輪降級）**，
是 SLO（< 2%）的 9 倍。判定不可接受。

同日的 followup 又推翻了第二個假設：**8s 上限本身就設得太緊，不是 LLM 偶爾出問題。**
用 dev-token 連續取樣發現，「成功」的樣本本身就貼著舊的 8s 上限跑（7.3–9.5s），
且降級樣本的重試（當時 4s 上限）3/3 全部失敗——8s 從一開始就切在真實延遲分佈的
**中位數附近**，不是尾端。

## 決策

1. **改為有條件重試**：只對 `aidomain.ProviderUnavailableError`（逾時／連線失敗／5xx／429／
   回應無法解析）重試，**最多一次**；4xx 等「請求本身有問題」不重試。
2. **逾時上限三次調整**：8s →（量測後暫時放寬）120s →（再量測後收斂）**30s**。
   最終值的依據是 dev 7 天分佈：p50 0.85s／p99 5.7s／最大 < 10s。

## 為什麼最後收斂回 30s 而不是留在 120s

120s 是刻意的寬鬆值，用來先把降級率壓下去並取得乾淨的分佈。
拿到分佈之後，`ExtractTimeout` ＋ `RetryTimeout` 各 30s（合計 60s）
才能維持在 gateway `streamTurnDeadline`（120s）的一半以內——
留在 120s 的話最壞情況 240s 會超過 gateway 的上限，那條風險因此關不掉。

## 後果

- 重試由 `retryingExtractor` 承擔，沿用與 `callGeminiWithRetry` 同一條錯誤分類判準。
- 上限與 gateway deadline 綁在一起：**改動 `ExtractTimeout` / `RetryTimeout` 前，
  必須先確認 `2 × timeout ≤ gateway streamTurnDeadline / 2`**，否則會把已關閉的風險重新打開。
- 重新評估條件：模型或 provider 換代、或 p99 明顯偏移時，重跑同一組量測再定值。
