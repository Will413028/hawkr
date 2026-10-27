# 結帳：免運判定基準與運費服務降級策略

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-18（決策落地日期見各節） |
| 影響範圍 | order-service `CheckoutService`、shipping-service、買家結帳前端 |
| 規格 | [20 號 結帳模組詳細設計書](../詳細設計書/20-結帳模組（Checkout%20Module）詳細設計書.md) §4.7、§4.9、§4.12 |

## 決策一：門檻免運以「優惠券分攤後」的實付金額判定

**脈絡**：`feePayer=SELLER` 且 `freeThreshold > 0` 時，免運門檻要用優惠券**前**還是**後**的
小計來判定。兩種做法在業界都有先例，而結果對賣家毛利與買家 UX 的影響方向相反。

**考慮過的方案**

| 方案 | 先例 | 代價 | 判定 |
|---|---|---|---|
| **折扣後判定**（採用） | Shopify：native discounts validate the free shipping minimum requirement on the **remaining total after product/order discounts have been applied**；博客來以「實際付款金額」判斷，明列不含 E-Coupon、單品折價券與結帳再折扣 | 買家使用大額優惠券時可能失去免運資格 | **採用** |
| 折扣前判定 | Adobe Commerce / Magento、WooCommerce（保護商家毛利） | 與「實付金額達標即免運」的買家直覺不符 | 否決 |

**決策**：折扣後判定。本系統定位偏「平台型 C2C 電商」（多賣家、買家可在多賣家間結帳），
與 Shopify 模式更貼近；且「實際成交金額決定免運資格」更符合買家對「實付達標即免運」的認知。

**接受的權衡**

- 賣家毛利風險：買家用大額優惠券時，折後未達門檻 ⇒ 仍收運費（不會雙重夾擊賣家）。
- 買家 UX 風險：使用優惠券可能失去免運資格 ⇒ 由兩段判定（tentative ↔ final）讓前端在
  套用優惠券前提示。

⚠️ 博客來那段客服文字亦可作為前端 UX 文案的參考底本，例如「使用此券後將失去免運資格」。

## 決策二：`feePayer=SELLER` 未設門檻 ＝ 無條件免運

**脈絡**：賣家上架勾「我負擔運費」但未設免運門檻時（App 預設送 `freeThreshold='0'`），
要不要求他一定得設一個門檻才生效。

**決策**：視為**無條件負擔該 SKU 的運費**，買家運費固定為 0。

**理由**：產品定義的「免運」就是賣家勾「我負擔運費」，平台沒有「免運券」概念。要求賣家
「勾了我負擔運費卻還要設門檻才生效」與該產品認知直接矛盾。（產品負責人裁定。）

## 決策三：運費快取用 lazy ＋ circuit breaker，不做啟動預熱

**考慮過**：以 `@PostConstruct` 預熱 ＋ `@Scheduled` daemon 定期 refresh。

**否決理由**：order-service 啟動快於 shipping-service，rolling update 期間 warm-up 會失敗，
而 daemon 的重試窗口內買家結帳會拿到**誤導性**的 `ALL_SHIPPING_EXCEEDED`——dev 上觀測到約
21 秒的冷啟動拒絕視窗。

**決策**：lazy load ＋ `@CircuitBreaker`。首個請求承擔一次 gRPC RTT（毫秒級），後續走 5 分鐘
TTL 快取；shipping-service 短暫不可用時走 stale cache fallback，冷啟動視窗因此不存在。

🔴 **不得改回啟動預熱**——它會讓服務啟動順序變成結帳正確性的前提。

**快取為什麼仍然存在**：避免每次結帳都打 gRPC（運費規格變動頻率極低，分鐘級 staleness
可接受），並讓 circuit breaker 在 shipping-service 出問題時有東西可退。

## 決策四：運費查詢降級 fail-open ＋ stale

**考慮過**：CB open 時直接讓結帳失敗（fail-closed）。

**決策**：fail-open ＋ stale cache。運費規格變動頻率極低，30 秒的 stale 對買家體驗的影響
遠小於「整個結帳鏈路返錯」。

⚠️ 這條**只適用於運費查詢**。結帳鏈上決定「能不能買」的判定（商品可購買狀態、賣家接單
能力、庫存）一律 fail-closed，兩者不可互相類比。
