# 成交溫度計 v1 的 outcome 定義與色帶依據

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-18（決策作成於 2026-09-09） |
| 影響範圍 | ai-service 的 `procurement_price_previews`、買家端溫度顯示、CodeMeta reason code |

規格本身見
`docs/詳細設計書/64-Hawkr Agent 商務線（RFQ Publication、Seller Quote、Buyer Acceptance）詳細設計書.md` §2-5。

## 決策一：outcome 定義換成「涵蓋率」，不是「會不會收到報價」

產品稿定義的 outcome 是「截止前至少收到一筆合規賣家報價」
（`AT_LEAST_ONE_COMPLIANT_IMMUTABLE_SELLER_QUOTE_BY_DEADLINE`）。

**v1 刻意不採用**：該定義需要歷史標籤才能校準，而標籤來源（賣家回覆版本）當時尚未實作。
⇒ 改為現在就算得出的
`BUYER_RANGE_VS_PRICEBOOK_FORECAST_COVERAGE`（`outcomeDefinitionVersion = "1"`）。

`dealTemperature` 這個契約名稱保留，`outcomeDefinitionVersion = "2"` 保留給未來的
真回覆機率版本——換版不換名，消費端不必改。

## 決策二：色帶依**寬鬆界**而非嚴格界

最初的色帶（`BANDS_V1`）依嚴格界（市場價完全落在買家區間內的機率）決定。
**那對買家是反指標**：

> 出價 70–90、行情 52–74 時，嚴格界只有 12%（判 COLD），
> 而那正是買家最有利的情況——行情整段低於他的上限。

寬鬆界（市場價不超過買家上限）對上限**單調**，嚴格界不是。
⇒ `BANDS_V2` 改依 `probabilityUpperBps`，亦即「買家上限落在行情分布的第幾百分位」。

門檻也一併從 20/50 改成百分位的自然四分位切法（<25% COLD、25–75% WARM、≥75% HOT）：
寬鬆界的值域是 `[0.10, 0.90]`，且買家上限 ≥ 市場中位時必然 ≥ 50%，
沿用舊門檻會讓「熱」涵蓋一半以上的情形。

## 決策三：reason code 採 `MAPPING_MISSING`，不改名

同一個概念在三處有三個名字：產品稿的 `NO_RELIABLE_PRICEBOOK_MAPPING`、
票上 allowlist 的 `MAPPING_UNRELIABLE`、線上程式碼的 `MAPPING_MISSING`。

採用 `MAPPING_MISSING`——它已在 CodeMeta 登記，且該字面值已經在 prod 的 wire 上。
改名是零收益的破壞性契約變更。

## 後果

- 買家畫面**只顯示 `probabilityUpperBps` 一個數字**；`probabilityLowerBps` 留在 wire 上
  供 `outcomeDefinitionVersion = 2` 使用，但不呈現。
- 重新評估條件：賣家回覆版本累積出足夠的歷史標籤之後，`outcomeDefinitionVersion = 2`
  可以換成真回覆機率；屆時色帶依據要重新推導（上述單調性論證只對涵蓋率成立）。
