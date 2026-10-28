# Uber Direct 的商流由「平台受託代銷」改為「賣家直營 ＋ 平台向賣家收物流服務費」

| 項目 | 值 |
|---|---|
| 狀態 | accepted；Java 的 Uber 程式刪除時（主計畫 B3）標 superseded by `2026-10-27-general-ecommerce-scope-cut.md`（D4）；在那之前 Java 仍依本檔運作 |
| 日期 | 2026-09-11 |
| 影響範圍 | order-service／payment-service／shipping-service 商流與帳務軸 |

- **脈絡**：需求書把 Uber Direct 指派給 `commerce_model = PLATFORM_CONSIGNMENT`
  ——平台以代理商戶代收、平台對買家開票、賣家對平台開貨款發票、平台撥款給賣家並收 1% 撥款指示費。
  整個 65 的帳本（§5.3）／撥款（§5.4）／1% 撥款指示費（§5A.2）／F 軸／G 軸都是為這條軸建的，
  而 Uber 是它**唯一**的消費者。

## 選項

> 三案逐一評估。**每案都要答「使用者看到的東西會不會變少」**——
> 答案是「會」的才需要產品裁決，答案是「不會」的就只是工作。
> ⚠️ **本 ADR 是第三種情形**：三案都不會讓使用者看到的東西變少，但**錢的歸屬與開票主體不同**
> ⇒ 它不是「那就是工作」，而是**商務與稅務決策**，只有客戶能裁。

## A：維持受託代銷（買家 → 平台代理商戶 → 平台撥款給賣家）
- **代價**：需要書面受託代銷附約與會計簽核（原 U-1／U-3，整條擋在結帳）；
  平台就全額對買家開票 ⇒ 平台的銷項與進項同時放大，稅務與帳務複雜度最高；
  且**與本平台在 PayUni 的實際帳務相反**——每個賣家都是以自己名義註冊的獨立合作商店，
  買家付款帶的是賣家自己的商店代號（58 詳設開票論證 **M1** 逐字：「本平台不是平台代收，
  在 PayUni 帳上這筆錢從一開始就掛在賣家名下」）。
- **使用者看到的東西會不會變少**：不會（買家仍只付一次款）
- ❌ **客戶否決**

## B：賣家直營 ＋ 平台向賣家收 Uber 物流服務費
- **內容**：買家一次付款（商品＋Uber 配送費）直接進賣家子商店；賣家對買家開票；
  平台另建一條「平台 → 賣家」的 Uber 物流服務費應收（PayUni `agent_fee`，FeeType=4 物流），
  **並同時對賣家開立服務費發票**（58 號**方向二 · 平台服務費**，issuer=平台／recipient=賣家／無 `order_no`）；
  平台與 Uber 之間是獨立的供應商月結與對帳。
- **代價**：新增一條需求書完全沒有的關係（線 B），它的收費時點、取消情境政策、
  1% 手續費負擔方三格需另外裁決（`2026-09-11-uber-logistics-fee-charge-trigger.md`／004／005）。
- **使用者看到的東西會不會變少**：不會
- ✅ **採用（客戶 2026-09-11 裁決）**

## C：Uber 配送費不向賣家收，平台全額吸收
- **代價**：平台為每一筆 Uber 訂單吸收全部配送成本，而配送費已由買家付給賣家
  ⇒ 賣家收了配送費、成本由平台出，經濟上不成立。
- **使用者看到的東西會不會變少**：不會
- ❌ 否決（未送客戶，經濟上自相矛盾）

## 決定：**B**

| 維度 | 舊（A，已否決） | 新（B，現行 Target） |
|---|---|---|
| `commerce_model` | `PLATFORM_CONSIGNMENT` | 🔴 `SELLER_DIRECT` |
| `collection_model` | `PLATFORM_AGENT_MERCHANT` | 🔴 `SELLER_MERCHANT` |
| `invoice_model` | `PLATFORM_TO_BUYER` | 🔴 `SELLER_TO_BUYER` |
| `consignment_agreement_version` | 必填 | 🔴 `NULL`（不適用） |
| 賣家→平台貨款發票（F 軸） | 有 | 🔴 無 |
| 平台撥款 ＋ 1% 撥款指示費（G 軸） | 有 | 🔴 無 |
| 平台→賣家物流服務費 | 無 | 🔴 **有**（`agent_fee` FeeType=4 ＋ 58 方向二發票） |
| 平台↔Uber | 供應商月結 | 供應商月結（不變） |

> ⚠️ **2026-09-20 Amendment**：表中 `consignment_agreement_version` 一列已失效——該欄與 `chk_orders_consignment_agreement` 隨 order-service `V16` 退場（受託代銷附約的簽署機制不會建立，該欄永遠無來源）。本 ADR 的決策（Uber 路徑走 `SELLER_DIRECT`）**不受影響**，其餘各列原樣成立。

## 理由

1. **商品銷售主體始終是「賣家 → 買家」**，換配送方式不改變商品買賣主體。
   一筆訂單因為換了配送方式就整張變成「平台賣給買家」，在商務與稅務上都站不住。
2. **與 repo 既有事實一致而非例外**：58 詳設 §14.4.4 的 **M1** 已經逐字記載
   PayUni 帳上款項從一開始就掛在賣家名下。原模型是與實況相反的那一個。
3. **五種履約方式收斂到同一條帳務軸** ⇒ 讀者不必再問「這一筆走哪條軸」。

## 後果

1. 🔴 **`PLATFORM_CONSIGNMENT`／`PLATFORM_AGENT_MERCHANT`／`PLATFORM_TO_BUYER` 三值變成
   值域內、零寫入路徑的保留值。** **不刪 enum、不改 DB CHECK**；
   ⚠️ **`settlement-service` 已刪除**（網域併入 payment-service），F／G 軸九張表亦未搬
   （`V9` 檔頭逐字列出）。**三個 enum 值本身仍保留。**
2. 🔴 **帳本／撥款／1% 撥款指示費／F 軸／G 軸失去唯一消費者，成為死碼。**
   **這是技術債不是安全缺陷**：撥款資格的 G0 軸守門就是 `commerce_model = PLATFORM_CONSIGNMENT`，
   現行的不可達性由**上游**保證：`OrderPolicyResolver` 一律回 `SELLER_DIRECT`
   ⇒ 沒有訂單填那個值 ⇒ **整條鏈自然惰性、不會誤撥任何一筆**。
   **「不得改接到 `SELLER_DIRECT`」已被 `2026-09-11-ledger-carrier-model-compatibility.md` 取代**。
3. **U-1（受託代銷附約）對 Uber 不再適用**；U-3 的開票時點確認改為對賣家開票那一側仍需會計確認。
4. **新增三格待裁決**：`2026-09-11-uber-logistics-fee-charge-trigger.md`（收費時點）／`2026-09-11-responsibility-based-cost-attribution.md`（取消與退款政策）／`2026-09-11-payuni-channel-fee-bearer.md`（1% 負擔方）。
5. **`2026-09-06-fulfillment-docs-split-into-six.md` 的第 4／5 列理由要回灌**（見該 ADR 的「什麼情況要重新檢視」）。

## 什麼情況要重新檢視

- 若客戶未來要求平台就全額對買家開票（受託代銷回歸）⇒ 本 ADR 翻回 A，
  而那三個保留 enum 值與整組帳本／撥款機制**正是為此保留的**（第 1／2 點的理由）。
