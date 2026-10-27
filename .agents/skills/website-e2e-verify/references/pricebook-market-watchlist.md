# Pricebook 全市場與追蹤清單的 browser e2e

主檔 `tests/e2e/insights-optional-market.spec.ts`。這是 mock 的 browser e2e，不證明 DB transaction、事件或真的登入；HTTP 契約與實際回應形狀用後端驗證另外確認。

```bash
API_BASE_URL=http://127.0.0.1:9/api/v1 PLAYWRIGHT_PORT=3187 pnpm exec playwright test tests/e2e/insights-optional-market.spec.ts
```

不加 `--project` 時跑 config 的 desktop 與 mobile；要分開跑用 `--project=chromium`、`--project=mobile`，先用 `--list` 確認目前的名稱。

## Fixture 契約

- overview 的 query 用 snake_case，POST body 用 camelCase。斷言要看實際 request 的 key、scope 與有無，不能讓 mock 接受任意 query。
- overview 的數值與 ID 是字串；不帶市場與指定市場是不同的追蹤目標；DELETE 的 data 是 `{}`。
- `latestActual`／`previousActual`／change／points／marketOptions／snapshot metadata 以契約的唯一來源與實際回應校對。沒有價格就保持缺值；比較失敗時明確給 `p50: null`，並保留市場的身分與狀態。
- 查「零元是假價格」前先確認 fixture 真的是缺值：合法的 0 元、前值 0 都不能當成缺值；前值為 0 時不合成百分比。
- unavailable 的 fixture 用正式的空 reason code，省略 `sourceDataAsOf`／actual／change；snapshot 的 `sourceDataAsOf` 只從實際參與的 item 計算，不自造 reason code。
- 特定回應的時序用 deferred route 控制，同步點用「已觀察到 request」或 UI 狀態。

## 覆蓋矩陣

| 群組 | 情境 |
|---|---|
| 全市場 | 精確作物預設不帶市場；單一市場切換與清除；換作物不殘留市場；七日預測與觀測各自的錯誤與重試；部分比較失敗時不出現假的 P50 |
| 雷達 | 首頁 all／watchlist 兩個 scope 的空態；空搜尋與清空；UI 視窗 7／30 天（14 天由 API 層驗）；query 與市場只作用在 all；一個 scope 錯誤與重試不拖垮另一個 |
| 追蹤 | POST 成功後兩個 scope 同步；optimistic POST／DELETE 失敗後兩個 scope 都還原且可重試；unavailable 的目標仍可刪除；pending 時防重複送出 |
| 非同步 | 舊搜尋晚回不能覆蓋新搜尋；切換視窗或市場後不被前一次回應還原；慢的 POST 只送一次，完成後才更新狀態 |
| 資料狀態 | 缺值不補 0；stale 狀態清楚；空的 points 不畫假的 sparkline；前值 0 的百分比是缺值；同作物兩個市場的身分不互換 |
| 版面 | desktop／mobile 操作；961px 邊界無重疊；mobile 無水平捲動；空態 CTA 可用鍵盤操作 |

## 判準

- 新增的測試要說得出它能抓到哪一種實際的 regression。
- POST／DELETE 失敗後先暫停 overview 的 refetch 再檢查 cache 是否還原，否則 refetch 的自癒會掩蓋缺陷。
- 對錯誤保留、晚回防護、optimistic rollback 做突變：拿掉 production 的那段防護，新測試必須紅。
- 共用的 WebView 幾何 helper 判斷「已穩定」的條件是 overlay 已開且自身與祖先的 animation 已完成，不等後代無限循環的 Lottie；不要放寬 24px 的門檻。這是測試同步問題，與價格契約分開處理。
