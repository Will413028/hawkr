# 68　7-ELEVEN 大宗寄倉（B2C Bulk Warehouse）詳細設計書

> 共用規約（identity 鍵／金額口徑／時間口徑／命名／API envelope／錯誤碼號段／CodeMeta 流程／
> MQ 兩套 SSOT／排程器判準／新服務登記／媒體 objectKey）**只在 65 §3.9 定義一次，本書只引用**。


**覆蓋需求書**：**§21 全章（21.1～21.22）** ＋ **65 附錄 E 的「補遺」節** 第 7 條（需求書 §21.19 的**關鍵路徑**＝技術依賴順序 → 本書 §12.6）
（21.16 的畫面細規交 65A，本書 §附錄 D 只做對照表，（跨本協調事項見 **65 附錄 F**）

**承接決議**：**D-11**（batch `DC_RECEIVED` 由聚合推導 ＋ 單向禁令保留 ＋ `DC_RECEIVED → RECONCILED` **補直達邊** ＋ `RECONCILING` 由差異 case 關閉驅動）、
**D-12**（§21.11 的 23 值 → 65 normalized 的**三欄映射表**；batch **不進 order read model**；取消矩陣是計算 `allowedActions[]` 的規則不是第二套機制）、
**D-15**（**7-11 B2C 用 `3004`**——依需求書 §21 的 provisional 碼；`3002` 明令不得重用；商家自送另配 `3005`）、
**D-19 群 E**（**17 個 S0**，佔該軸 29 個狀態的 **58.6%**（亦即全部 30 個 S0 的 56.7%））、**D-13**（fail-closed ＋ 五端文案）、**D-4**（`CANCELLED` 雙 L；provider 原生值照抄）、
**D-5**（各軸獨立 enum）、**D-7**（帳務軸：`commerce_model = SELLER_DIRECT`、account 屬賣家 ⇒ **不進平台 payout**；**不得複製 Uber 模式推論**）、
**D-16**（金額字串 ＋ **HTTP 409／412 改 200＋業務碼**）、**D-17**（告警四問 ＋ 補缺口，**六條**）、
**D-20**（maker-checker 沿用 58；Admin 子角色新建；`/admin` 路由要明講掛哪個 group）、**D-1**（reconcile 四義：五層每日對帳＝義②、差異 case＝義③）、
**D-2／U-6**（正式契約與 code mapping 未到齊前 fail-closed）

**引用 65 的共用規約**：§一 identity／§二 金額／§三 時間／§四 命名／§五 API／§六 錯誤碼／
§七 CodeMeta／§八 MQ／§九 排程器／**§十一 媒體（🔴 標籤 PDF 是 explicit 例外請求，本書必須正面裁決）**
（**不引用 §十 新服務**——本書落在既有 `shipping-service` ＋ 獨立 `cvs-711-adapter` 邊界，見 §4.3）

---

## 0. 關鍵參數速查

> 標「repo 實測」者可 grep 覆核；標「需求書 §21」者來自任務書、**repo 內無第二份**，
> 施工前以正式契約覆核（U-6）。🔴 **門檻不是 SSOT，`entered_at` 才是**。

### 0.1 provider 硬規格（不可協商，超出即 fail-closed）

| # | 規格 | 值 | 來源 | 落在哪裡守 |
|---|---|---|---|---|
| 1 | 單件重量上限 | **10 kg** | 需求書 §21.3.9 | `shipping_method_configs(3004,*).max_weight_grams = 10000`（§5.1.9） |
| 2 | 最長邊 | **45 cm** | 需求書 §21.3.9 | `max_single_side_cm = 45` |
| 3 | 長寬高合計 | **105 cm** | 需求書 §21.3.9 | `max_dimension_sum_cm = 105` |
| 4 | 商品價值／理賠上限 | **NT$20,000** | 需求書 §21.3.9；`/trade` 的 `TradeAmt` 註記「上限 2 萬」（repo 實測《PayUni 物流官方 API 總覽》§4.3） | 建單前置檢核；文案一律「**依條款審核，最高上限**」 |
| 5 | 單次列印筆數 | **≤ 50 筆**，逗號分隔 | 同上 §7.3 `ShipTradeNo` 欄備註 | 標籤分組器（§4.2） |
| 6 | **B2C `ShipDate` 不得為當日** | `YYYYMMDD`，> 今日 | 同上 §7.3 | 列印端點前置驗證；**當日出貨在 B2C 不成立** |
| 7 | 溫層 | **常溫／冷凍，不含冷藏** | 同上 §2.4 `GoodsType` 只有 `1=常溫 / 2=冷凍` | `chk_b2c_temp`（§5.4）只允許 `3201`／`3203` |
| 8 | B2C 標籤取得 | **PAYUNi 直接顯示出貨單**（C2C 是跳轉超商、商店不得自行連線） | 同上 §7.4 | §5.1.13 的裁決建立在這一條上 |
| 9 | 配編編號 | **B2C ＝ `PartnerId`(3)+`Odno`(8) ＝ 11 碼**（C2C 是 12 碼） | 同上 §7.6 | `provider_shipments.provider_label_no` |
| 10 | 建店資料**不得修改** | 無更新／重新送審端點 | 同上 §2.6 ① | §3.4 |
| 11 | B2C 常溫**同時開通 C2B 退貨便** | 副作用非選項 | 同上 §2.6 ④、§2.2 | §3.3；**逆向不是我方開的**，不得當 feature 宣傳 |
| 12 | **僅限商業會員** | 個人會員不可 | 同上 §2.2 | gate ② |

### 0.2 Activation gate 七項（**AND，缺一即不可建單**）

> 🔴 **這七項分屬兩個服務、兩個資料庫**（單體 seller 模組的 `seller.payuni_registrations` ／ shipping-service
> `seller_shipment_settings`）⇒ **不可能在一個交易內求值**。判定點固定在 shipping-service，
> 跨服務那兩項走既有 gRPC（§4.5.11），取回值存成快照並帶 `checked_at`，不得當成即時真相。

| # | 項目 | 求值來源（repo 實測欄位） | 通過條件 | 漏了會怎樣 |
|---|---|---|---|---|
| ① | **商務資格：母子廠商代號** | `seller_shipment_settings.partner_id` / `.eship_id`（V1 既有欄） | 兩欄皆非空 | 建單被 provider 退，錯誤碼落在 `AGENT02001`（查無符合商店物流資料） |
| ② | **PayUni 會員層** | 單體 seller 模組 `seller.payuni_registrations.member_status` | `= '1'` | 商業會員資格未成立，B2C 本來就不開放 |
| ③ | **PayUni 商店層** | 單體 seller 模組 `seller.payuni_registrations.mer_status` | `= '1'`（`91`＝待補件、`92`＝退件，repo 實測《代理商 API 總覽》§12 對應邏輯） | 同上 |
| ④ | **服務產品層（溫層各自）** | `seller_shipment_settings.provider_status`，**該溫層那一列** | 常溫列＝`SPB2C` 已開通、冷凍列＝`SPB2CFrz` 已開通 | 🔴 **兩者完全分開**：常溫過了不代表冷凍過了（§3.3） |
| ⑤ | **建店層** | `seller_shipment_settings.onboarding_status` | `= '3602' ACTIVE`（repo 實測 CodeMeta：3601 待審核／3602 已啟用／3603 審核不通過／3604 已關閉／3605 永久關閉） | `findActiveSetting` 回 null ⇒ 建單拋 `SHIPMENT_SELLER_NOT_ONBOARDED(5004)` |
| ⑥ | **賣家本地開關** | `seller_shipment_settings.enabled` | `= true` | 賣家關掉了仍能建單＝違反 06 號 §2.3.1 的寫路徑語意 |
| ⑦ | **`BackSetShipping` 已收到** | 🔴 **本書新增欄**（`seller_shipment_settings.back_set_shipping_at`，§5.1.14） | 非空 | 見 §3.2.5：這個值**只能從 Notify 取得、無查詢端點** ⇒ 漏收一次，該賣家永久卡住且系統無法自癒 |

⚠️ **gate 輸出三態**：`ELIGIBLE`／`BLOCKED(reasonCode)`／**`UNKNOWN`**。`UNKNOWN`（跨服務讀取失敗、
快照過期）**當 `BLOCKED` 處理但 reasonCode 不同**——塌縮成 `BLOCKED` 會讓「真的沒開通」
與「我們讀不到」在營運端長得一樣。

### 0.3 前端正規化狀態 9 值 ＋ 前端態 `LOADING`

| 前端態 | 對應 normalized（65 §7.1.2 值域，**本書只映射不新增**） |
|---|---|
| `NOT_APPLIED` | 無 `seller_shipment_settings` 列 |
| `SUBMITTING` | `payuni_submit_status = 4302 REQUESTED`（V1 既有欄） |
| `PENDING_REVIEW` | `onboarding_status = 3601` ／ provider `SetStatus ∈ {9, 8, 7}` |
| `REJECTED` | `onboarding_status = 3603` ／ `SetStatus = 2` |
| `PARTIALLY_ACTIVE` | 🔴 常溫與冷凍**只有一邊**通過 gate（B2C 專屬態，C2C 不會出現） |
| `ACTIVE` | 七項全過 |
| `DISABLED_BY_SELLER` | gate ①～⑤⑦ 全過但 `enabled = false` |
| `CLOSED` | `onboarding_status = 3604` ／ `SetStatus ∈ {3}` |
| `PERMANENTLY_CLOSED` | `onboarding_status = 3605` ／ `SetStatus = 4` |
| **`LOADING`** | 🔴 **純前端態，不在後端值域**——後端**不得**回傳這個字串 |

⚠️ **provider `SetStatus` 只有 7 個值不是 9 個**（repo 實測同上 §2.5：`9 未處理／1 正常／2 審查失敗／
3 關閉／4 永久關閉／7 重新送審／8 已上傳審核中`，**跳號無 `5`／`6`**）。骨架寫「9 值」與實測不符，
**以實測為準**；`5`／`6` 若日後出現，`fromValue` 必須**拋錯不得靜默**。

### 0.4 租約與 aging 門檻（本書初值）

| 對象 | 值 | 上下界推導（65 §3.9 九） |
|---|---|---|
| 取號 lease（逐筆 `/trade`） | **120 s** | 下界 > 單筆最壞耗時（deadline 30 s × retry 3）；上界 < 批次 `OPEN` 壽命（以小時計） |
| 標籤列印 lease（每組 ≤50 筆） | **300 s** | 下界 > `/print_label` deadline 60 s ＋ PDF 下載上傳；50 筆是**同一次外部呼叫**，不可逐筆取租約 |
| `CREATE_UNKNOWN(991)` 回查窗 | **180 分**（取 65 §0.1.2 不等式 ⑥ 的上界） | 🔴 **SSOT 在 65 §0.1.2，本書只引用不自行取值**（引用點：§7.0 K68-3／§7.3.2 X1／§8.2.3）。🔴 **建單兜底的候選述詞不得枚舉狀態值，尤其不得排除已有 `991`／`990` 殼的訂單**：`FL-1` 階段的 `CREATE_FAILED(990)` **失敗殼不發任何 lifecycle 事件** ⇒ `orders.shipping_status` 仍為 `NULL` ⇒ **那一筆必須被兜底收進候選並重試**（03 §8.11.1 **Case A**）。排除它 ＝ 從 fail-closed 退化成 fail-silent。🔴 **recovery 邊界依 provider-neutral 的 Backstop Ownership Contract**（03 §8.11.1）：**Order backstop 只負責「Shipping 還沒接手」的訂單；Shipping 一旦以正式 shipment lifecycle event 把 `shipping_status` 投影回 Order，generic Order backstop 必須退出。** |
| batch `DISPATCHED`／`RECONCILING` aging | **48 h**／**72 h** | 65 §7.1.1.2 I 已定，沿用不另訂 |
| `PENDING_LABEL` aging | **24 h** | > 一個工作日；< `ShipDate` 不得為當日所隱含的一日提前量 |
| gate 快照有效期 | **15 min** | 超過即 `UNKNOWN`；小於賣家「後台開通完成 → 回來建單」的典型間隔 |

### 0.5 費率與費用參數

| 項目 | 值／規則 | 註 |
|---|---|---|
| `shipping_method_configs(3004, 3201/3203)` | 兩列皆 `105 / 45 / 10000`；`estimated_days`／`settlement_days`／`commission_rate` **待 rate card**（U-6） | repo 實測 V1 seed：`3002` 常溫是 `105/45/5000`、冷凍 `105/45/10000` ⇒ B2C 常溫的重量上限與 C2C 不同 |
| `shipping_fee_tiers(3004, *)` | 🔴 **不 seed 真實金額**，`enabled = false` | 既有 `3002` 是常溫 67／冷凍 165（V1 seed）；B2C 費率屬正式 rate card，**猜一個數字上線＝對賣家收錯錢** |
| **退貨費 38 元／件** | 🔴 **是 rate policy 不是常數** | 存 `shipping_fee_ledger` 的 `rate_snapshot`，**不得寫成程式常數**；改版時舊單必須沿用舊值 |
| 理賠 | 「**依條款審核，最高上限 NT$20,000**」 | 文案用語，五端一致（§3.3） |
| 代收 | 🔴 **PAYUNi 與 7-ELEVEN 絕不同時收**（§3.5.4） | `ServiceType` 既有 `3 = 取貨不付款`（repo 實測 `ServiceType.NO_CASH_ON_DELIVERY`），本書預設值 |

### 0.6 本書新增的碼值與錯誤碼

| 類 | 值 | 狀態 |
|---|---|---|
| `ShippingMethod` | **`3004 SEVEN_ELEVEN_BULK_WAREHOUSE`** | 本書新增（附錄 C.2）。⚠️ **`3005` 已由 69 號 `MERCHANT_DELIVERY` 佔用** |
| `ShippingType` | **`3653 CVS_B2C`** | 🔴 **值域定案在 65 附錄 C.1.1／C.2，本書只引用不得自行裁決**。它是 §5.1.14 撞號問題的唯一正解。🔴 **65 這次擴充新增的是兩個值不是一個**：`3653 CVS_B2C`（本書）與 `3654 MERCHANT_DELIVERY`（69）——**不是「兩本共用同一次擴充的同一個值」**，兩者的 `buildSlotKey` 第一段必須不同 |
| `ShippingStatus` | 用 65 §7.1.2 的 31 值，**本書不新增任何值** | `930`／`931`／`932`／`991`／`992`／**`993`** 用得到（`993 CLAIM_PENDING` 見 §7.2 X11、§6.4.1 第 16 列）；`933` 是 Uber 專用 |
| batch 狀態 | `OPEN／CLOSED／DISPATCHED／DC_RECEIVED／RECONCILING／RECONCILED` | 🔴 值域由 65 §5.2.2 `chk_batch_status` 定死，**不得另立第二套** |
| 錯誤碼 | 本書需 **≈35 個**業務碼 | 🔴 **號碼由 65 §6.7 統一指派，本書不得自己撿號**（65 附錄 C.5.1 明訂）。號段 `9500~9699`（200 個）：65 佔 `9501~9575`（實配 **50** 個），**66／68／69 自 `9576` 起連號追加** |

---

## 1. 概述與目標

### 1.1a 先讀基本設計書

> **本書的基本設計書：[68 7-ELEVEN 大宗寄倉基本設計書](../基本設計書/68-7-ELEVEN%20大宗寄倉（B2C）基本設計書.md)**
> ——三層（包裹／列印組／批次）、與店到店的差異與排他性、開通七項檢核、交倉八步、
> 買家可見六階段、**錢與發票怎麼走**（收款兩種模式、物流費由 PayUni 自賣家帳戶扣、平台不在資金線上）、
> 每日分層對帳、冷凍逆向的降級路徑。**本書不重述那些內容，只把它們落成契約與資料結構。**

共用骨架另讀 [65 物流履約平台基本設計書](../基本設計書/65-物流履約平台（Fulfillment%20Platform）基本設計書.md)
（六個正交維度、跨配送方式共通規則、營運工作台、憑證保存）。大宗寄倉的流程說明在本書的基本設計書，不在 65 基設。

⚠️ **基本設計書不含任何可施工的細節**（無 DDL、無端點、無述詞）⇒ **不得拿它當契約來源**。

### 1.2 設計目標

| # | 目標 | 可驗收的形式 |
|---|---|---|
| G1 | 一批訂單編成批次、一次交倉 | 批次端點（§6.1.3）＋ `shipment_batches` 狀態機（65 §5.2.2） |
| G2 | **逐筆**取號，取不到的留在批次外 | 🔴 不是整批失敗：`shipment_batch_items` 逐列有自己的狀態（§5.1.5） |
| G3 | 標籤可靠取得且可稽核 | ≤50 筆分組、SHA-256、授權下載、audit 留痕（§5.1.13） |
| G4 | 驗收結果回到訂單 | `931`／`932` → 個件；batch 只由下往上聚合（65 D-11） |
| G5 | 開通每一格看得見、卡住有人知道 | gate 七項各有 reasonCode ＋ aging（§0.2／§0.4） |
| G6 | 契約未到齊**整條 fail-closed** | U-6：`shipping_method_configs(3004,*).enabled = false`（§3.2.6） |

### 1.3 非目標

| # | 不做 | 理由 |
|---|---|---|
| N1 | **不建 SKU 庫存倉儲模型**（`inventory_storage_mode = NONE`） | 大宗寄倉是「一次交一批包裹」不是寄倉存貨；建它＝商品域多一個沒有寫入者的狀態 |
| N2 | 不爬網站、不把 CSV 當真相 | 對帳權威是 provider API 與帳單，不是可被人工編輯的檔案 |
| N3 | **不 fallback 到 C2C** | §2.6：互為備援會造成「同一筆訂單兩張物流單」 |
| N4 | 不支援冷藏（`3202`） | §0.1 ⑦ |
| N5 | MVP 維持「一訂單＝一包裹＝一寄件編號」 | 多件拆包需 provider 多單號模型，需求書 §21.1 明訂延後 |
| N6 | 不做「當日交倉」 | §0.1 ⑥ |

### 1.4 與其他設計書／服務的關係

| 對象 | 關係 | 方向 |
|---|---|---|
| **65 骨幹** | normalized 值域、帳本、撥款、事件 envelope、排程器判準、65 §3.9 十一節 | **只引用**，不得各寫一份 |
| **06 Shipping** | 本書是它的擴充（`shipments`／`seller_shipment_settings`／`OnboardingService`） | 只 expand，不改既有語意 |
| **10 Webhook／11 Client** | 貨態 Notify 落點；`payuni-java-client` 是唯一送出點 | 🔴 **不新增 HTTP 出口** |
| **23／09／58** | 門市選擇（`/ship_map`）／逆向／憑證 | 沿用與擴充，不另起 |
| **65A** | 五端畫面細規 | 本書只給契約 |
| **69 商家自送** | 同形（同落 shipping-service、同要擴 `ShippingType`） | 🔴 **`3005` 是它的** |

### 1.5 已定案決策一覽

| 決策 | 內容 |
|---|---|
| D-15 | ✅ **`3004 SEVEN_ELEVEN_BULK_WAREHOUSE`**；`3002` 不得重用；`3005` 屬 69 |
| 🆕 本書 | ✅ **採用 `ShippingType.CVS_B2C(3653)`**（🔴 **值域定案在 65 附錄 C.1.1／C.2，本書只引用**；65 同批指派給 69 的是另一個值 `3654 MERCHANT_DELIVERY`）—— §5.1.14 撞號與 `mapShippingMethodToType` 回 null 的**唯一正解**；✅ **標籤 PDF 走 media-service ＋ 受控存取 ＋ TTL**（§5.1.13） |
| D-11／D-12 | batch `DC_RECEIVED` **由聚合推導**、單向禁令、補直達邊；normalized 值域由 65 定，本書**只映射**；batch 不進 order read model |
| D-7／D-16 | `SELLER_DIRECT`、發票賣家開給買家、**不進平台 payout**；金額字串；**409／412 一律改 200 ＋ 業務碼** |
| D-4／D-5／U-6 | `CANCELLED` 雙 L、provider 原生值照抄、各軸獨立 enum；契約未到齊**整條 fail-closed** |
| D-1 | 需求書的 "reconcile" 是**四個不同的東西**，拆成四義各走各的路 |
| D-2 | 受託代銷 vs 58 號 `Q-B1` ⇒ 自行收斂，fail-closed 擋在**結帳**不是開票 |
| D-13 | 需求書要 fail-closed，而 repo 現況正好相反（`config == null` 靜默跳過） |
| D-17 | 告警覆蓋不到「狀態卡住」，且需求書從未討論**告警何時會靜默** |
| D-19 | 稽核出的 **30 個 S0 狀態**（沒有出口）的處置 |
| D-20 | Admin／maker-checker／RBAC ⇒ 沿用既有，不另建；step-up 本書不做 |

---

## 2. 範圍與情境

### 2.1 系統情境圖

```
Buyer /app/*  Seller /app/*  Admin·CS /admin/*
      └──────────┴──────────────┘
                 ▼
              Gateway ──gRPC──▶ shipping-service
                                 ├ cvs-711-adapter（內部 package §4.2）
                                 ├ 批次編排器／標籤分組器
                                 └ event inbox／每日對帳五層
                                     │ gRPC          │ HTTP（payuni-java-client）
                       單體 seller（gate ②③）       ▼
                       media-service（標籤資產）    PAYUNi
                                                      │ 商務資格核准（非 API 連線）
                                                      ▼
                          司機交接 ──▶ 7-ELEVEN 物流中心 ──▶ 門市 ──▶ 買家取貨
```

🔴 **我方與 7-ELEVEN 之間沒有任何技術連線**；圖上那條線是商務關係。

### 2.2 In-scope

開通（七項 gate）／checkout 選項與承載檢查／逐筆取號／標籤列印與分組／掃描綁定／批次生命週期／
司機交接／驗收與驗退／到店取貨／未取退回／理賠受理／費用帳與每日對帳／五端可觀測性。

#### 2.2.1 v1.4A ／ v1.4B 的降級切分點

| 切分 | 內容 | 切點成立的條件 | 🔴 解鎖條件 |
|---|---|---|---|
| **v1.4A（開通與狀態）** | 七項 gate、`/setting` 送件、兩類 Notify、狀態同步、Admin 可觀測性、`3004` 碼與 config 列（**`enabled=false`**） | 🔴 **A 交付後買家端零變化**——config disabled ⇒ checkout 不出現該選項 | RFI-1（憑證模式）書面回覆（§11.2.1） |
| **v1.4B（出貨與批次）** | 取號、標籤、批次、交接、驗收、對帳、費用帳 | 依賴 A 的 gate 與狀態閉環 | A 的 gate 與狀態閉環 ＋ RFI-9（費率）＋ §11.2.2 第 2／3／4 項 |

⚠️ 切分的價值在**人力不足時 A 仍有獨立價值**（賣家看得到開通進度）。
反過來**不成立**：只做 B ⇒ 沒有 gate，等於讓未開通的賣家去建單。

### 2.3 Out-of-scope

見 §1.3 N1～N6。另：**不建第二套 object storage**（§5.1.13）、**不建常駐漂移掃描器**
（《後端服務責任邊界規範》禁令；對帳走 §7.0 K68-8 的 **lease** 排程——65 §九 level-triggered 三題已在 §7.0／§12.7 逐條評估，**單一寫者不成立**）。

### 2.4 角色與身分模型

沿 65 §3.9 一：賣家鍵 **`store_id`**、買家鍵 **`buyer_id`**、登入 principal `account_id`
（三者不可互換）、`merId = HAWK<store_id>`。

🔴 **司機只取得交接所需的最低資訊**：批次號、件數、封箱數、manifest checksum、賣家聯絡窗口。
**不得**出現買家姓名、電話、地址或訂單金額——那份文件會離開我方系統。

### 2.5 名詞邊界：買家看到的 vs 賣家／Admin 看到的

| 概念 | Buyer 顯示 | Seller／Admin 顯示 |
|---|---|---|
| 配送方式 | **「7-ELEVEN 門市取貨」** | 7-11 大宗寄倉（B2C） |
| 交倉 | 「賣家已出貨」 | 交倉／進倉批次 |
| 物流中心驗收 | 「配送中」 | DC 驗收通過／驗退 |
| 廠商代號 | **不顯示** | 母子廠商代號（`PartnerId`／`EshipId`） |

🔴 **禁止對買家顯示的詞：「大宗寄倉」「母子代號」「物流中心」「B2C」「批次」。**
判準不是「會不會洩漏」，是**買家對這些詞沒有可執行的動作**——顯示它只製造客服工單。
落地：買家向 DTO 逐欄白名單（同 65 §3.9 五），並加契約測試斷言「回應欄位集合 ＝ 白名單」
——**只斷言「禁詞不在回應」擋不住上游長胖時新流出的欄位**。

### 2.6 與 C2C（`3002`）的關係

| 面向 | 規則 |
|---|---|
| 後端路徑 | **完全分開**：不同 `ShippingMethod`／`ShippingType`／`LgsType`／slot |
| 買家選擇 | 買家**看不到**差別，只看到「7-ELEVEN 門市取貨」；由賣家開通狀態決定走哪條 |
| fallback | 🔴 **不得互為備援**。B2C 中斷時保留選項並顯示可信原因，不得自動改走 C2C |
| **排他性** | 🔴 開通 B2C 後同一帳號**不得再用門市交寄** |

⚠️ 排他性落在**資料層不是 UI**：同一 `store_id` 不得同時存在 `3651` 與 `3653` 的 ACTIVE 列
（§5.4 部分唯一索引）。只寫在文件裡＝下一個人加個新入口就繞過了。

---

## 3. 約束與前提

### 3.1 PAYUNi 契約約束（逐支寫「證明什麼／不證明什麼」）

| 端點（repo 實測常數） | 本書用途 | 它**證明**什麼 | 它**不證明**什麼 |
|---|---|---|---|
| `/setting`（`PATH_CVS_ONBOARD` v1.0） | 送建店資料 | `Status=SUCCESS` ＝**收件成功**；`SetStatus` 是**當下**審核態 | 🔴 **不證明開通完成**。B2C 是「每週一 9:00 彙整上週」的人工審核（官方 65A §2.7），冷凍另需物流中心測標 |
| `/trade`（`PATH_CVS_SHIPMENT` v1.3） | 逐筆取號 | `ShipTradeNo` ＝ 該包裹的 provider 主鍵 | 不證明標籤可印；`MerTradeNo` **10 分鐘不可重複** |
| `/print_label`（`PATH_PRINT_CVS_LABEL` v1.0） | 標籤 | Form POST 回應本身就是可列印的出貨單 | 不證明賣家印了貼了；`Odno` 由 **Notify** 回來，不在同步回應裡 |
| `/ship_map`（`PATH_STORE_MAP` v1.1） | 買家選門市 | 門市當下可用 | 不證明取貨時仍可用（關轉 → `81`） |
| `/query`（`PATH_QUERY_SHIPMENT` v1.1） | `991` 回查 | 該 `ShipTradeNo` 當下貨態 | 🔴 **查無 ≠ 沒建成功**——延遲期間會回查無，不得據此重建（INV-3） |
| 貨態 Notify | 狀態推進 | provider **曾經**處於該狀態 | 不保證順序與單次 ⇒ inbox ＋ 單調前進守門 |
| Agent Notify（`MerStatus` 等） | 開通狀態 | 商店層狀態變更 | 🔴 **不含 `BackSetShipping`**（§3.2.5） |

⚠️ **`/api/logistics/update`（限未列印）在 repo 的 `PayUniShippingEndpoint` 內查無對應常數**
（對照組：同檔的 `PATH_CVS_SHIPMENT`／`PATH_PRINT_CVS_LABEL`／`PATH_QUERY_SHIPMENT` 都查得到 ⇒ 查詢有效）。
⇒ **本書不得假設它存在**；需要修改未列印物流單時，走「取消後重建」，並在 §6.1.4 定義該端點。

### 3.2 🔴 憑證分歧與兩條實作分支（本書最高風險）

#### 3.2.1 BLOCKER 本體
官方 `/setting` 文件（repo 實測 §2.4）的必填欄位表**只有 `MerID`**，沒有 `AgentID`／`IsPlatForm`
⇒ 代理商能不能**代旗下商店**送 `/setting`，文件沒有答案。

#### 3.2.2 分支 A：Agent Key ＋ `IsPlatForm=1`，站內表單、後端代送
賣家在站內填 14 欄 → 後端用**代理商憑證**代送。UX 最好，但**建立在一個未經書面確認的參數上**。

#### 3.2.3 分支 B：商店自有 Key／IV，賣家在 PAYUNi 後台完成，我方只同步狀態
🔴 **這一支有實測支持**：`SPB2C`／`SPB2CFrz` 被官方明列為「**不可透過 API 帶入**、須由商店
自行至 PayUni 後台申請開通」（repo 實測《PayUni 代理商專用 API 總覽》§9.1）
⇒ **至少「服務產品開通」一定在後台**，分支 A 就算成立也只覆蓋建店資料送件。

#### 3.2.4 切換機制定案
🔴 **不是 env、不是全域 policy version，而是 per-provider-account 欄位**：
`seller_shipment_settings.credential_mode ∈ {AGENT_PROXY, STORE_OWNED}`（§5.1.14）。
理由：① 兩種模式的賣家操作步驟不同，同一環境必然同時存在兩種賣家（canary）
② env 切換會讓「已用 A 開通的賣家」的狀態解讀方式整個改變，而歷史列不會跟著變
③ 它同時是稽核欄位——出事時要答得出「這個賣家當初是誰送的件」。
**預設 `STORE_OWNED`**（fail-closed：不假設我們有代送權）。

#### 3.2.5 🔴 分支 B 的狀態同步結構性缺口
`search_merchant_status` 的回應**不含 `BackSetShipping`**（repo 對照：`MerStatus`／`MemberStatus`
在單體 seller 模組的 `seller.payuni_registrations` 有對應欄位，而 `BackSetShipping` 全 repo **零命中**
⇒ 查詢有效，它真的不存在）。
⇒ **賣家在後台申請完成後，我方唯一得知的途徑是那一次 Notify。漏收＝該賣家永久卡住且無 API 可自我修復。**

**補償設計（三層，缺一不可）**：① inbox 對 Agent Notify **先落庫再處理**（§5.1.7），不因解析失敗而丟；
② `back_set_shipping_at` 為空且 `onboarding_status = 3602` 超過 aging ⇒ 開差異 case，不是靜默等待；
③ Admin **人工標記入口**（§6.1.4），留痕在 `seller_shipment_onboarding_logs`（V1 既有表）。
⚠️ ③ 是人工出口不是自動修復——依 65 §3.11 第②問，答案是「人工」時那個端點必須真的存在。

#### 3.2.6 憑證前置未到位時的 no-go 封鎖態
在 PAYUNi **書面**確認憑證模式、且取得 sandbox golden payload 之前：
`/apply` flag 全關、正式 onboarding 不開、`3004` 建單路徑不開、`shipping_method_configs(3004,*).enabled = false`。
🔴 **「sandbox 打得通」不能解除這一條**——它證明的是我們的 client 寫對了，不是我們有權這樣用。

### 3.3 7-ELEVEN 公開營運基線

| # | 基線 | 影響 |
|---|---|---|
| 1 | **僅限商業會員** | gate ② |
| 2 | **常溫／冷凍各自開通**（`SPB2C`／`SPB2CFrz`） | gate ④；產生 `PARTIALLY_ACTIVE` 前端態 |
| 3 | **B2C 常溫同時開通 C2B 退貨便**（僅常溫） | 逆向能力常溫有、冷凍沒有 |
| 4 | **冷凍退貨僅週二／週五兩退**，模式只有自行取件或統昶配合貨運 | 冷凍逆向 SLA 與常溫不同，**不可共用文案** |
| 5 | `ReturnCycle`／`ReturnCycleType` 合法組合受溫層限制（常溫 2/3；冷凍固定 4） | §4.5.1 欄位驗證 |
| 6 | 建檔預設**冷凍上收**，要改需通知窗口 | 計費口徑，列入 §3.5 open item |

🔴 **v1.4 預設所有 SKU 不可用**：`3004` 是 opt-in，SKU 必須逐一通過承載檢查（重量／尺寸／溫層）
才進入該選項的候選集合。反過來（預設可用、超規才擋）會讓上線第一天就出現大量建單失敗。

### 3.4 建店資料不可變性

provider 無更新／重新送審端點（§0.1 ⑩）⇒ ① 送件前**二次確認**（前端 diff，65A）；
② 逾時或回應未知 ⇒ **保留 `payuni_submit_status=4302`／`provider_status=UNKNOWN`，不得盲目重送**
（重送＝製造第二筆不可撤回的建店申請）；③ 要改資料只有 `CLOSED` 後重新申請一條路，
**且 `3605 PERMANENTLY_CLOSED` 不可逆**。

### 3.5 帳務軸約束（D-7）

| # | 約束 |
|---|---|
| 3.5.1–2 | `commerce_model = SELLER_DIRECT`；**發票由賣家開給買家**（擴 58 號既有 `issuer → recipient`，不另立）；`collection_model ∈ {PROVIDER_COLLECTED, EXTERNAL_PREPAID}` |
| 3.5.1–2 補註 🆕 | 🔴 **`collection_model` 的值域名以 65 附錄 C.2 為準**（`PROVIDER_COLLECTED`／`EXTERNAL_PREPAID`）。寫入其他字面值會被 65 §5.1.3 的 `chk_orders_collection_model` 以 **23514** 拒絕。🔴 值域是**通用維度**（款項實際經過誰的手），不含 provider 名——provider 由 `provider` 維度（`CVS_711`）承載 |
| 3.5.3 | 🔴 **章內矛盾的裁決**：需求書 §21.3.2 預設 `EXTERNAL_PREPAID`，需求書 §21.12 說它是合約 blocker。**取前者**——`ServiceType = 3（取貨不付款）` 是 repo 既有且已在 C2C 上線運行的預設值（`ServiceType.NO_CASH_ON_DELIVERY`，多處呼叫點）；把它當 blocker 等於把已在跑的模式重標成未定。**`PROVIDER_COLLECTED` 才是需要合約的那個，v1.4 不開** |
| 3.5.4 | 🔴 **代收互斥**：PAYUNi 與 7-ELEVEN **絕不同時收**。落地：`ServiceType = 3` 時 `TradeAmt` 是**商品價值**（供理賠認定）不是應收金額——這兩者共用一個欄位，是本書最容易寫錯的一格 |
| 3.5.5 | 撥款主體：B2C 走 `SELLER_DIRECT`，帳戶屬**賣家** ⇒ **不進平台 payout**、不建 subledger、不需 maker-checker／reserve。🔴 `PLATFORM_CONSIGNMENT` **目前無任何履約方式採用**（65 §0.6.1／§3.8b C-1）——Uber 同樣是 `SELLER_DIRECT`，不得拿它推論出一條平台代收軸 |

⚠️ **§3.5.1～3.5.5 的編號即上表第一欄**，不另立子節。

**本節的施工落點**（上表只說規則，這裡說規則落在哪個欄位）：

| 約束 | 落在哪 |
|---|---|
| `commerce_model` ／ `collection_model` | `provider_shipments` 的參數快照 14 項之一（§5.1.3）；🔴 **不落 `shipments`**——那是可變的當前狀態 |
| 發票 | 擴 58 號既有 `issuer → recipient`，**不另立**（§1.4）。本書零 DDL |
| 代收互斥 | `service_type` 快照欄 ＋ §5.4 的 `chk_method_lgs_type`；`ServiceType = 3` 時 `trade_amt` 的語意是**商品價值**（§4.7 陷阱 6） |
| 撥款主體 | 🔴 **本書全篇不建立** 平台 payout、subledger、maker-checker、reserve／chargeback。那三樣屬 `PLATFORM_CONSIGNMENT`（65 §0.6），B2C 走 `SELLER_DIRECT` ⇒ **不適用**（不是延後）。🔴 `PLATFORM_CONSIGNMENT` **無任何履約方式採用**（65 §3.8b C-1）⇒ 那三樣對**每一本**都不適用 |

### 3.6 工程約束
→ 65 §3.9 一～九、十一。**不引用 65 §3.9 十（新服務）**——本書不新增服務（§4.3）。

### 3.7 假設

| # | 假設 | 不成立會怎樣 |
|---|---|---|
| A1 | 一訂單＝一包裹＝一寄件編號（MVP） | 多包裹要改 `provider_shipments` 唯一鍵＝破壞性變更 |
| A2 | 批次不跨賣家／溫層／provider account／`ShipDate` | 分組鍵少一個就會把不能同批的東西併進同一次 `/print_label` |
| A3 | 驗收結果會逐件回來 | 🔴 **目前無證據**，見 §5.6 U-6a |

### 3.8 由本書定版的決策
`3004` 碼與 `ShippingType.CVS_B2C(3653)`／批次狀態機沿用 65 §5.2.2 單一值域／標籤走方案 (a)／
`credential_mode` 為 per-account 欄位／`handoffOptionId` 的承載物是 `handoff_options` 列／
`/trade` 的時點＝**賣家把訂單納入批次的那一刻**（不是下單當下——那時 `ShipDate` 還不確定，
而 B2C 的 `ShipDate` 不得為當日）。

---

## 4. 架構

### 4.1 解法總綱

🔴 **「大宗批次」由我方內部編排；provider 沒有 batch／manifest／交倉 API。**
repo 實測：`PayUniShippingEndpoint` 的路徑常數裡沒有任何一個是批次或交接語意
（對照組：`PATH_PRINT_CVS_LABEL` 查得到 ⇒ 查詢有效）；`/print_label` 的「最多 50 筆」
是**唯一**帶批次味道的東西，而它是**列印分組**不是交倉批次。

⇒ 三層要分清楚，混在一起是本書最貴的錯誤：

| 層 | 是什麼 | 誰知道它 |
|---|---|---|
| **包裹**（`provider_shipments`） | 一筆訂單一個 `ShipTradeNo` | provider **知道** |
| **列印組**（≤50 筆，四參數一致） | 一次 `/print_label` 的參數分組 | provider 知道那一次呼叫，不知道「組」這個概念 |
| **批次**（`shipment_batches`） | 賣家一次交倉的單位 | 🔴 **只有我方知道**，provider 端無對應物 |

⚠️ 因此 batch 的狀態**永遠不可能由 provider 事件直接推進**——它只能由
①我方操作（開／關／交接登記）②個件狀態的**聚合**（65 D-11，單向）驅動。

### 4.2 元件視圖

```
shipping-service（既有 Java 服務，不新增服務）
├── grpc/            既有 ShippingGrpcServiceImpl（本書新增 B2C 相關 RPC）
├── service/
│   ├── OnboardingService     〔既有〕擴 SLOT_DEFINITIONS ＋ mapShippingMethodToType
│   ├── ShippingService       〔既有〕建單入口，含 gate（§0.2）
│   ├── ShipmentLabelService  〔既有〕已有 triggerBatchCvsPrint／printAndUploadCvsLabel
│   └── b2c/  🆕 B2cBatchOrchestrator（開／關／交接／聚合）｜B2cLabelGrouper（≤50 筆＋四參數分組）
│             ｜B2cActivationGate（七項 AND、三態）｜B2cReconciler（每日五層，🔴 **lease** 非 level-triggered，§7.0 K68-8）
├── adapter/cvs711/  🆕 cvs-711-adapter（**內部 package**）
│   └── 唯一組裝 B2C 參數與解析 B2C 回應的地方
├── mq/              〔既有〕ShippingEventPublisher；🆕 provider event inbox consumer
└── client/          〔既有〕SellerGrpcClient／MediaGrpcClient／OrderGrpcClient／BuyerGrpcClient
```

🔴 **`cvs-711-adapter` 不是新的對外 HTTP 出口。**
實際送出請求的仍是既有的 `payuni-java-client`（`PayUniHttpClient` → `ShippingClient`），
B2C 與 C2C **共用同一支端點與同一把憑證**。這個 adapter 的職責是把
「B2C 的參數組法、狀態映射與批次語意」收斂在一處。
**命名裡的 `711` 指 PayUni 的服務通路，不是統一超商的 API。**

**邊界規則**：`Order`／`Payment`／`Seller`／`Aftersales` **不得直呼** adapter；
adapter **不得寫任何業務表**（只組參數、解回應、丟 domain 物件）。

### 4.3 部署拓撲：不新增服務

| 判準 | 答案 |
|---|---|
| 獨立資料所有權？ | ❌ `shipments`／`seller_shipment_settings`／`shipping_method_configs`／`shipment_batches` 全是 shipping-service 的 |
| 獨立擴縮需求？ | ❌ 批次由賣家操作驅動，量級遠低於既有貨態 webhook |
| 會產生跨服務交易？ | 🔴 **拆出去就會**——批次聚合要同交易讀寫 `shipments` 與 `shipment_batches`（65 §7.3 K-11 明訂「同交易」） |
| 新服務固定成本 | Port／NetworkPolicy／deploy values／content-addressed tag 雙向 gate／MQ 登記 |

⇒ **落在既有 `shipping-service`（HTTP 9930 ／ gRPC 9931，TSID node base 600，repo 實測 65 §7.2 表）**，
不新增 port、不新增 release、不動 `k8s/values/`。

⚠️ **唯一的跨服務新呼叫是 gate ②③ 讀單體 seller 模組（legacy gRPC）**，依 65 §6.1.5 三件同批：
① NetworkPolicy 放行 shipping→單體 ② values 的 `SELLER_SERVICE_HOST`／`_GRPC_PORT` 指向單體的 legacy gRPC
③ 檢查對方 policy 註解有沒有寫「只有 X 會呼叫」。
🔴 **必須 fail-closed**（讀不到 ⇒ gate 回 `UNKNOWN` ⇒ 擋建單）。fail-open 會產生
65 §6.1.5 的完全無聲失效：測試綠、CI 綠、部署成功、smoke 綠，而 gate 從未執行過。

### 4.4 命名與契約規約

→ 65 §3.9 四。本書補兩條：

#### 4.4.1 🔴 `PartnerId` vs `PartnerID` 的 casing 陷阱
官方 wire key 是 **`PartnerId`／`EshipId`**（repo 實測《PayUni 物流官方 API 總覽》§2.5、§7.5）；
repo 既有欄名 `partner_id`／`eship_id`（snake，正確）。
⚠️ 識別字若寫成 `PartnerID`／`EshipID`（大寫 D）會對不上 wire key，而 Jackson 預設**靜默忽略未知欄位**
⇒ 讀回來是 null、不報錯。⇒ adapter 內兩個 key **以常數集中定義**，並加契約測試：
把 sandbox golden payload 反序列化後斷言兩欄非空。

#### 4.4.2 四層名詞的關係

| 層 | 值 | 誰定義 | 例 |
|---|---|---|---|
| `shipping_method_code` | `3004` | 我方 CodeMeta | 購物車看到的選項 |
| `shipping_family`（`ShippingType`） | `3653 CVS_B2C` 🆕 | 我方 CodeMeta | onboarding 分流、slot key |
| `provider` | PayUni | — | **真正的 API 通道**（B2C 與 C2C 同一個） |
| `service_product` | `SPB2C`／`SPB2CFrz` | PayUni | 開通粒度（常溫／冷凍各一） |

🔴 四層**不是同義詞**。`provider` 只有 PayUni 與 Uber 兩個；「7-ELEVEN」不在這一欄。

### 4.5 PAYUNi 整合要點

#### 4.5.0 🔴 capability 對應（65 §4.4.1／§4.4.1a／§4.4.1b）

> **本節是本書與骨幹 adapter 模型的接縫。**
> 🔴 **本書的 `capability` 一詞有兩個既有含義**（§6.1.4 表頭的 `CapOrderFulfill`／`CapProductManage` ＝ 46 號賣家授權；
> `shipping_capability_policies` ＝ 哪些 SKU／門市／溫層可用 3004）⇒ **provider capability 一律加前綴寫成「provider capability」**，
> 不得只寫 capability。撞名是本節最容易被誤讀的地方。

**本 provider（`3004 CVS_711` B2C）的 provider capability 集合**（引自 65 §4.4.1b 那一欄，**不得在本書另寫一份**）：
`create` ✅／`label` ✅**（批次型）**／`dispatch` ❌／`cancel` ❌／`track` ✅／`returns` ❌／`pod` ❌／`onboard` ✅／`inbound` ✅／`storeLocator` ✅。

🔴 **本書是「四個 `empty()`」最多的一本，而那四個各自都有我方端點。**
這正是 65 §4.4.1a 要分成兩層的原因：把 `Optional.empty()` 直接接到 `allowedActions[].enabled = false`，
會讓 **B6 交倉／C2 取消／E8 領退結案** 三支確實存在、賣家與 ADM 每天在按的動作**整條消失**。
⇒ 下表每一列都必須讀 **`requiresProviderCall`** 那一欄，不能只看 supported。

| # | 操作 | provider capability | supported | adapter method | domain action | `requiresProviderCall` | `allowedActions[]` | 對外端點 | 失敗行為（權威節） |
|---|---|---|:---:|---|---|:---:|---|---|---|
| §6.5 **#6**（provider `E2`） | `/api/logistics/trade` 逐筆取號 | `create()` | ✅ | `create().submit(ctx)` | `dispatch`（取號，正常履約）／`resubmit`（`990` 失敗後 recovery）🔴 **兩者共用同一支 provider 呼叫，但是兩個不同的 Domain Action**——共用的 helper 必須收必填的 `actionContext`（值域 `DISPATCH` ／ `RESUBMIT`），否則會退化成一支 unrestricted `create()` 讓狀態守門失效（§6.1.4a C） | **true** | `dispatch`／`resubmit` | 🔴 **`dispatch`（取號）＝ `C4 POST /app/shipments/:order_no/dispatch`（＝ 65 `S-10`）**；`resubmit` ＝ `C3 .../resubmit`（僅 `990`）。<br/>🔴 **K68-1 由 `C4` 觸發，不由 `B1` 觸發**：§7.6 逐字「批次建立端點對『尚未取號』的訂單**一律回 `9520`，不代為取號**」，且「取號必須早於成批」⇒ 把取號按鈕接到 B1，按下去必失敗（§7.0） | 冪等鍵 `order_no+package_id+service_product+confirm_snapshot_hash`；bounded 3 次；30s；未知 ⇒ `CREATE_UNKNOWN(991)` → `/query` 回查，🔴 **不得直接重建**；回查窗 ≤180 min（§6.1.3／§6.5） |
| §6.5 **#8**（provider `E3`） | `/api/logistics/print_label` | `label()` 🔴 **批次型** | ✅ | `label().printBatch(ctx, items)` | `print_label`／`reprint_label` | **true** | `print_label`（批次 `OPEN` 以外不可用）／`reprint_label` | `B5 POST /app/shipment_batches/:batch_no/labels`／`C1 .../b2c_label` | 冪等 `(shipTradeNo, shipDate)`；bounded 3 次；60s（租約 300s）；整組失敗 ⇒ 整組不改 `label_print_status`；🔴 **不得整批重印**（會產生與包裹上不同的第二份標籤）（§7.1.3） |
| — | **批次約束由 capability 攜帶** | `label()` 的 constraint | ✅ | `label().constraints()` → `maxBatchSize=50`／`groupingKeys=[LgsType, ShipType, GoodsType, ShipDate]` | — | — | — | — | 🔴 **65 §4.4.1a 判準**：出現在業務層的 provider 數字（50、180 min、30s）都是漏出 adapter 的約束。`C-GRP B2cLabelGrouper` 只讀 `constraints()`，**不得寫死 50** |
| §6.5 **#9**（provider `E4`） | `/api/logistics/query` | `track()` | ✅ | `track().fetch(ctx)` | `requery` | **true** | `requery`（ADM 側） | `E4 POST /admin/shipments/:id/requery` | 天然冪等；bounded；10s；🔴 **查無 ≠ 不存在（INV-3）**，不得據此重建；套用必過 `isValidTransition`，不得倒退覆蓋 `LOCAL_PROCESSING_DONE`（§6.1.3） |
| §6.5 **#5**（provider `E1`）＋ **#4** | `/api/logistics/setting` 建店 ＋ `search_merchant_status` 查開通 | `onboard()` | ✅ | `onboard().apply(ctx)`／`onboard().status(ctx)` | —（建店不走 `allowedActions`） | **true** | 不出現 | `A2 POST /app/shipping_onboardings/cvs_b2c/apply`（端點刻意保留 provider 名，65 §6.2.1 硬約束 1 的例外） | P5 **不自動重試**（無更新端點，重送＝開出第二個子商店）；30s；未知 ⇒ 保留 `REQUESTED`／`UNKNOWN` ＋ 人工查證佇列。⚠️ **P4 的擁有者是單體 seller 模組（Go），不是 shipping-service** ⇒ `onboard().status()` 是**跨服務**讀取，adapter 只翻譯不持有（§6.1.2） |
| §6.3.1／§6.3.2／§6.3.4／§6.3.5 | 貨態 Notify ／`ApiType=Print` 回填 ／ Agent Notify（`BackSetShipping`）／ 檔案交換 importer | `inbound()` 🔴 **三個來源** | ✅ | `inbound().parse(raw)`／`inbound().toNormalized(...)` | — | — | 不出現 | `POST /webhook/...`（落 payuni-webhook-service）／`E3 POST /admin/shipment_batches/reimport`／`E7 .../back_set_shipping` | 驗簽 raw bytes、`HashInfo` `equalsIgnoreCase`、`Timestamp` >15 min 拒收；🔴 **驗簽失敗回 200 不回 5xx**；🔴 **表外的貨態碼一律拋錯不得靜默**（§4.5.7-8）；importer 完整性三件缺一即整檔不處理、**序號跳號 FAIL 不是 SKIP**（§6.3.5） |
| §6.5 **#7**（provider `E5`） | `/api/logistics/ship_map` | `storeLocator()` | ✅ | `storeLocator().buildParams(ctx)`／`decryptResult(...)` | `reselect_store` | **true** | `reselect_store` | 買家 checkout（**零新端點**） | 天然冪等（前景 Form POST）；由使用者重試，**不設後端 retry**（§6.1.3） |
| B6 | **交倉回報** | `dispatch()` | ❌ **empty** | —（無 adapter 呼叫） | `handover` | 🔴 **false** | `handover` ✅ **照常 enabled** | `B6 POST /app/shipment_batches/:batch_no/handoff`（骨幹入口 65 `S-3`） | 🔴 **provider 端無交倉 API**（§4.1 逐字：provider 沒有 batch／manifest／交倉 API）⇒ 純本地寫 `930 HANDED_OVER` ＋ 同交易寫 outbox。**`dispatch()` 為 `empty()` 完全不影響這顆按鈕** |
| C2 | **取消申請** | `cancel()` | ❌ **empty** | —（無 adapter 呼叫） | `cancel` | 🔴 **false** | `cancel` ✅ **照常 enabled** | **`S-11 POST /app/shipments/:order_no/cancel`**（🔴 **原 `/cancel_request` 改名**，見下方） | 🔴 **十支 provider 端點全集無 cancel** ⇒ 寫 `CANCEL_PENDING(992)`，出邊主要靠貨態 Notify、兜底 K68-4；`9521` 已有 pending／`9522` 不可取消。逾時門檻**待 RFI-7**，未取得前只出 series 不設門檻（§7.4） |
| E8 | **領退／驗退結案** | `returns()` | ❌ **empty** | —（無 adapter 呼叫） | `admin_close_case`／`seller_pickup`／`accept_return` | 🔴 **false** | 三者皆 ✅ enabled（`accept_return` 🔴 冷凍 `3203` 一律移除） | `E8 POST /admin/shipments/:id/return_outcome`／`E6 .../reconciliation_cases/:id/close` | 🔴 **逆物流不是我方建單**：C2B 退貨便是 B2C 常溫開通的 provider 副作用、冷凍沒有。`53 RETURN_AT_DC` **shipment 軸無出邊**，aging 必須錨 `shipment_returns.outcome IS NULL`，**不得錨 `shipments.status`**（§7.3／§7.4） |
| — | **送達證明** | `pod()` | ❌ **empty** | — | — | — | 不出現 | 無 | 🔴 全書零 POD 概念且**這是正確的**：7-11 B2C 由買家在門市取貨，`PICKED_UP(11)` 貨態即代替簽收。**不要因為 66／69 有 POD 就在本書補一個** |
| §6.5 **#1／#2／#3** | `bind_merchant`／`link_merchant`／`update_merchant` | —（**不納入**） | ❌ 本書不用 | — | — | — | 不出現 | 無 | 明示不用（§6.5）。**不是缺口** |
| §6.5 **#10**（provider `E6`） | `/api/logistics/update` | `create()` 的修改面 | ⚠️ **契約未到齊 ⇒ 整支不啟用** | `create().amend(ctx)`（**本書只留簽章，不接線**） | `update` | true | `update` 🔴 **恆 disabled**，`disabledReasonCode` 指向 RFI-3 | 🔴 無（不開放入口） | repo 內 `PayUniShippingEndpoint` 查無此常數 ⇒ **不得假設它存在**；需要修改未列印物流單時走「**取消後重建**」；契約測試只斷言**拒絕**（§3.1／§13.3） |

**明文不納入 provider capability 的三類**（65 §4.4.1a 的排除清單在本書的具體所指）：

| 不納入 | 本書的具體對象 | 理由 |
|---|---|---|
| 批次編排 | `B1` 建批／`B4` 關批／`B7` 掃描綁定／`B8` 交倉時段 | provider 端無對應物 ⇒ 是**我方 domain model**。放進 adapter 會讓一個純本地概念被 provider 邊界切開 |
| provider 帳單／對帳 | `shipping_fee_ledger`、`E2 reconciliation_cases` | 唯一寫入者是 65 `A-19` 人工匯入；§22 明令不做自動比對排程 |
| 賣家可用性 gate | `GetB2cEligibility`、`shipping_capability_policies` | 那是**我方**的政策資料，不是 provider 能力。⚠️ 名字裡有 capability，**與本節無關** |

🔴 **本節的編號用法（本書有兩套 `E` 號，不加限定詞會指到錯的端點）**：

| 前綴 | 指的是 | 定義在 | 例 |
|---|---|---|---|
| **`§6.5 #N`** | provider 對外呼叫（10 支總表的列號） | §6.5 | `#10` ＝ `/api/logistics/update` |
| **provider `E1`～`E7`** | 同一批 provider 端點的另一個既有代號 | §6.1.1／§6.1.2 | provider `E6` ＝ `/api/logistics/update` |
| **`A*`／`B*`／`C*`／`D*`／`E*`** | **我方**對外端點（🔴 **25 支**：`A1–A3`／`B1–B8`／`C1–C4`／`D1–D2`／`E1–E8`） | §6.1.4 | `E6` ＝ `POST /admin/reconciliation_cases/:id/close` |

🔴 **`E6` 在本書有兩個意思**（provider 的 `/update` vs 我方的「關案」），`E4`／`E8` 同理
⇒ **跨本引用一律寫「provider `E6`」或「§6.5 #10」，不得只寫 `E6`**。
⚠️ 65 §4.4.1c 引用的「68 的 `E6`」指的是 **provider `E6`（`/api/logistics/update`，＝ §6.5 #10）**。
🔴 **判準（本書 §7.3 對 `X` 系列訂過同一條，此處補套用）**：**引用識別字必須來自被引用的那一節，不得自創或憑印象**（例：`N1`～`N6` 是 §1.3 的非目標編號，不得在別處重新指派）。

🔴 **端點命名（65 §6.2.1 硬約束 1）**：`C2` ＝ `POST /app/shipments/:order_no/cancel`（＝骨幹 `S-11`）。
同一個領域動作在同一個資源上**只能有一個 URL**——`allowedActions[].href` 由後端給、前端不會發現有第二個。
⚠️ 連帶：`FulfillmentAction` 值域**沒有 `cancel_request` 這個值**（65 §4.4.1c 把它併入 `cancel`）
⇒ §7.2 映射表（M2／M3 兩列）與全書一律寫 `cancel`。

| 節 | 端點 | 本書關鍵 |
|---|---|---|
| 4.5.1 | `/setting` | **14 個 B2C 必填欄**：`MerID`／`Timestamp`／`GoodsType`／`LgsType`／`ShipType`／`LabelCompanyName`／`ServiceTel`／`ContactName`／`ContactTel`／`ContactMail`／`MerURL`／`ReturnCycle`／`ReturnCycleType`／`ReturnAddress`。🔴 **`StoreID` 是 C2C 專用，B2C 不得傳**；`ReturnCycleType=6` 時 `ReturnCarName` 條件必填（6 中文字／長度 12）；`LabelCompanyName` 有四條互斥格式規則 |
| 4.5.2–3 | `/trade`／`/ship_map` | 帶 `LgsType=B2C`；`MerTradeNo` ≤25 且 10 分鐘不可重複；`ServiceType=3`；`TradeAmt` ＝**商品價值**（§3.5.4）。門市選擇與 C2C 同流程 |
| 4.5.4–5 | `/print_label`／`/query` | ≤50 筆／四參數一致／`ShipDate` 非當日／`LabelMode` 1=A4 2=直立；**B2C 由 PAYUNi 直接顯示**。`/query` 是 `991` 回查唯一工具，**查無 ≠ 不存在** |
| 4.5.6 | `/update` | ⚠️ **repo 內查無此端點常數**，本書不假設它存在（§3.1 註） |
| 4.5.7–8 | 兩類 Notify | `MerStatus`（`1` 正常／`91` 待補件／`92` 退件），🔴 不含 `BackSetShipping`；貨態值域是官方 21 碼，🔴 **表外的碼一律拋錯不得靜默** |
| 4.5.9 | 錯誤映射 **6 類** | `retryable`／`business_reject`／`auth_config`／`rate_limit`／`provider_outage`／`manual_review`。🔴 **不得把所有非 200 當同一種失敗**——`AGENT02002`（B2C 未開通）要停下來，網路逾時要重試；混在一起＝對已被明確拒絕的請求重試到耗盡 |
| 4.5.10 | client 必備 | timeout／bounded retry／breaker／rate limit／correlation id／**redacted audit**（`payuni_audit_logs`，憑證欄遮罩、**二進位 PDF 不落內容**）／sandbox 與 production 憑證隔離 |
| 4.5.11 | 單體 seller 模組 gRPC（legacy） | 🆕 `SellerShippingProfile` 擴 **2 欄**（`mer_status = 10`／`member_status = 11`，皆 `string`）——expand-only、沿用既有 `GetSellerShippingProfile` RPC，**不新開 RPC**。⚠️ 依 §3 Proto 規範：只改 `api/proto/seller/seller.proto`，生成後盤點所有 Go/Java caller |

⚠️ **§4.5.1～4.5.11 的編號即上表第一欄**，骨架的五個空子節已併回表格（理由同 §3.5）。
**逐支的完整契約（必填欄逐字、回應、冪等、重試、逾時處置）在 §6.1.3，本節不重複**；
錯誤碼六類的分類判準與處置在 §6.5；client 必備七件的驗收形式在 §13.3。

### 4.6 現況差距（施工前必須先關掉的五條）

| # | 差距 | repo 實測證據 |
|---|---|---|
| 1 | **憑證層**無 B2C 模式概念 | `seller_shipment_settings` 無 `credential_mode` 欄 |
| 2 | **`mapShippingMethodToType` 只認 `3001`／`3002`**，對 `3004` 回 `null` | 該方法兩個 if 之後 `return null` |
| 3 | **strategy 固定 `LgsType = C2C`** | `CvsShipmentStrategy.LGS_TYPE_C2C = LgsType.C2C.code()` 是 `static final` |
| 4 | **CHECK 擋住寫入**：`chk_shipping_method` 只認 `3001/3002/3003`；`chk_shipping_type` 只認 `3651/3652` | `V1__init_shipping_tables.sql` |
| 5 | **log 記完整 params** | `payuni_audit_logs.request_params` 註解逐字「收件人 PII 明文保留」⇒ 上線前確認遮罩涵蓋 `ReturnAddress`／`ContactTel`／`ContactMail` |

### 4.7 靜默陷阱表（每一條都是「不會編譯錯、不會測試紅」）

| # | 陷阱 | 症狀 | 防線 |
|---|---|---|---|
| 1 | **slot key 撞號** | `buildSlotKey(shippingType, goodsType)` 對 B2C 兩槽產生與 C2C **相同**的 key，`Collectors.toMap` 合併函式是 `(a,b) -> a`（原碼註解「重複取第一筆」）⇒ **靜默取到 C2C 的設定** | 新增 `ShippingType.CVS_B2C(3653)`——key 第一段不同就不會撞 |
| 2 | `mapShippingMethodToType` 回 null | `findActiveSetting` 回 null ⇒ 建單拋 `5004`，錯誤訊息說「尚未開通」而實際上賣家已開通 | §5.1.14 第 2 列 |
| 3 | 費率列缺席 | 65 D-13：`config == null` 直接 `continue` ⇒ 選項**完全靜默消失** | config 列與 CHECK **同批**（§5.4） |
| 4 | topology 漏登記 | 服務端 passive declare ⇒ 查不到 queue **不 crash、不進 DLQ**，該訂閱靜默失效 | §6.4.4 對帳 pin 增減量 |
| 5 | fail-open 的跨服務 gate | 見 §4.3 | gate 回 `UNKNOWN` ⇒ 擋 |
| 6 | `TradeAmt` 語意混用 | 不代收時它是商品價值；當成應收會讓理賠金額錯 | §3.5.4 ＋ 契約測試 |

---

## 5. 資料設計

> 🔴 **Flyway 版本號於施工時現讀決定，本書不凍結數字**：取
> `ls services/java/shipping-service/src/main/resources/db/migration/ | sort -t_ -k1.2 -n | tail -1`
> 的**下一號**。🔴 **`ls` 是字典序，必須帶 `sort -t_ -k1.2 -n`**，否則 `V10` 以上會被排到 `V2` 前面而漏看，
> 取到一個已被佔用的號。
> ⚠️ 撞號的後果不是 merge conflict：同一服務出現兩個同版號的 `*.sql`，**Flyway 啟動即失敗、
> 整個 shipping-service 部署被擋住**。
> ⚠️ **本書不得 `DROP/ADD chk_shipping_method`** —— 該 CHECK 的擁有者是 65
> （`V11` 已把它擴成含 `3004`／`3005` 的 5 值）；再 DROP 一次會**把它的成果刷掉**（後跑的贏）。
> 本書只保留 `chk_shipping_type` 的擴充與自己的新表。
>
> **本章讀法**：所有表都屬 **shipping-service（Java／Flyway）**，migration 落在
> `db/migration/`（版本號取法見上方紅字）。🔴 每張表都答
> **65 §5.0 的三問**（誰寫入／誰讀取／誰把它推走）**＋ 65 §3.11 的第 ④ 問**（卡住誰知道）。
> ⚠️ 這是兩套判準的合用，不是一套四問——**表**的三問問資料的權威與生命週期，
> **狀態**的四問問轉換邊與卡住的偵測；混成一句會讓人以為表也要答 allowed-from。**`shipment_batches` 的 DDL 在 65 §5.2.2 已定版，本書不重寫**，
> 只補缺的分組鍵（§5.1.5）。

### 5.1 資料模型

#### 5.1.1 `shipping_provider_accounts`：🔴 **不新建，用既有 `seller_shipment_settings`**
需求書 §21.5 說「每個出貨據點一個帳號」，需求書 §21.14 的主鍵卻是 `seller + provider + service product`。
**衝突時取後者**——repo 既有 `seller_shipment_settings` 的唯一索引正是
`(store_id, shipping_type, goods_type)`（V1 `idx_settings_seller_type`），且已承載
`mer_id`／`partner_id`／`eship_id`／`provider_status`／`onboarding_status`。
「出貨據點」在本 repo **無任何實體**（對照組：`store_id` 大量命中 ⇒ 查詢有效）⇒ 新建一張表
＝ 造一個沒有寫入者的維度。**多據點需求出現時再 expand。**

| 四問 | 答 |
|---|---|
| 誰寫入／讀取 | `OnboardingService`、Agent Notify consumer ／ `B2cActivationGate`、建單、`ShipmentLabelService` |
| 誰推走 | `onboarding_status` 狀態機（3601→3602／3603／3604／3605） |
| 卡住誰知道 | `3601` 停留 > 14 天（每週一彙整的人工審核，兩個週期沒動＝異常）⇒ aging |

#### 5.1.2 `shipping_capability_policies`（🆕）
承載「哪些 SKU／門市／地區／溫層可用 3004」的 server-side 判準，含 **policy hash 與生效區間**。
🔴 **hash 必須進 checkout preview 回應與 `provider_shipments` 快照**——沒有它，
「賣家看到的報價」與「後端當下的規則」無從對帳（`eligibilityHash`）。

#### 5.1.3 `provider_shipments`（🆕，immutable confirm snapshot）
一筆訂單一列，記下**送出去的那一刻**的全部事實。**建立後不得 UPDATE 業務欄**（只允許附加狀態欄）。

| 群 | 欄 |
|---|---|
| 鍵 | `id`(TSID)、`shipment_id`、`store_id`、`order_no` |
| provider | `mer_id`、`ship_trade_no`、`provider_label_no`（＝`PartnerId`+`Odno`）、`partner_id`、`odno` |
| 參數快照 **14 項** | `lgs_type`／`ship_type`／`goods_type`／`service_type`／`trade_amt`／`store_code`／`consignee_name`／`consignee_mobile`／`consignee_mail`／`prod_desc`／`ship_date`／`label_mode`／`policy_hash`／`rate_card_version` |
| 稽核 | `idempotency_key`、`request_at`、`confirmed_at`、`raw_response_audit_id`（→ `payuni_audit_logs.id`） |

⚠️ **為什麼不直接用 `shipments`**：它是**可變的當前狀態**（status 被 webhook 一直改）。
snapshot 與可變狀態同列 ⇒ 沒有東西能證明「我們當初送出去的是什麼」，而理賠與對帳要的就是那一份。

| 四問 | 答 |
|---|---|
| 誰寫入／讀取 | adapter 在 `/trade` 回 `SUCCESS` 的**同一交易**內 ／ 對帳五層、理賠、control tower、費用帳 |
| 誰推走 | 🔴 **沒有人**——append-only 稽核事實不是狀態 |
| 卡住誰知道 | 不適用；但「有 `shipments` 無對應 `provider_shipments`」是對帳第一層的偵測項 |

#### 5.1.4 `shipment_packages`（🆕）
`shipment_id`、`weight_grams`、`length_cm`／`width_cm`／`height_cm`、`declared_value`、
`label_sha256`、`label_asset_id`、`barcode`、`packed_at`、`scanned_at`、`scanned_by`。
MVP 一對一（A1），但**表允許一對多**——多包裹落地時不必改鍵。

#### 5.1.5 `shipment_batch_items`（🆕）＋ `shipment_batches` 的兩個補欄

`shipment_batch_items`：`batch_id`、`shipment_id`、`item_status`、`entered_at`、`excluded_reason_code`。
🔴 **`item_status` 是本表自己的軸**（`PENDING_TRADE_NO` → `LABEL_READY` → `SCANNED` → `HANDED_OVER`
→ `DC_ACCEPTED`／`DC_REJECTED` → `EXCLUDED`），**不是 `shipments.status` 的副本**：
取不到號的那幾筆停在 `PENDING_TRADE_NO` 或轉 `EXCLUDED`，**批次不因此失敗**（G2）。

**`shipment_batches` 補兩欄**（65 §5.2.2 只有 `store_id`／`shipping_method`／`delivery_temp` 三個分組鍵，
需求書 §21.10 要六個）：`provider_account_setting_id`（→ `seller_shipment_settings.id`）與 `ship_date`。
⚠️ **少了它們，`/print_label` 四參數一致性就沒有結構保證**——批次裡混進不同 `ShipDate` 的件，
會在列印時才發現，而那時包裹已經包好了。

| 四問 | 答 |
|---|---|
| 誰寫入／讀取 | `B2cBatchOrchestrator`、取號與列印流程 ／ 標籤分組器、交接文件、batch 聚合函式 |
| 誰推走 | `EXCLUDED` 由賣家或逾時排程；其餘由個件事件推 |
| 卡住誰知道 | `PENDING_TRADE_NO` 停留 > 24 h（§0.4）⇒ aging |

#### 5.1.6 `batch_handoffs`（🆕，需求書無表）
`batch_id`、`vehicle_no`／`carrier_reference`、`window_start`／`window_end`、`departed_at`、
`sealed_carton_count`、`manifest_checksum`、`seller_signature_ref`、`driver_signature_ref`。
🔴 **分表不加欄**：交接可能重來（司機沒來、車滿了），加欄會讓「重來一次」覆蓋掉第一次的紀錄。

#### 5.1.7 `provider_event_inbox`（🆕）
`id`、`source`（`AGENT_NOTIFY`／`SHIPPING_NOTIFY`）、`event_key`、`payload_raw`、`received_at`、
`processed_at`、`process_lease_until`、`process_attempts`、`last_error`。

🔴 **unique key 在 processed 之前就成立**：`UNIQUE (source, event_key)` 在**落庫當下**建立，
不是處理成功後才建；反過來做 ⇒ 重送時落兩列，去重變成應用層競態。
⚠️ 落庫用 **`ON CONFLICT DO NOTHING`**（不指名 conflict target），**不得**靠 catch
`DuplicateKeyException`——PostgreSQL 會讓整個交易 abort（`25P02`），後續 SELECT 全部 500。

| 四問 | 答 |
|---|---|
| 誰寫入／讀取 | webhook edge（落庫先於解析）／ inbox consumer（lease 搶佔，65 §九） |
| 誰推走 | 處理成功寫 `processed_at`；失敗留給重試 |
| 卡住誰知道 | `processed_at IS NULL AND received_at < now()-1h` 的列數指標＋告警 |

#### 5.1.8 `shipping_fee_ledger` ＋ `provider_settlement_lines`（🆕）
13 個 component（運費／溫層加價／偏遠／退貨費／驗退費／理賠／調整…）＋
🔴 **補第 14 個 `BUYER_REFUND`**——需求書 13 個 component **沒有商品退款**，
而「退貨與退款是分開的兩件事」正需要兩者在同一本帳上對得起來。
每列帶 `rate_snapshot`（§0.5 的 38 元／件是**當時的 policy 值**）。

🔴 **policy versioning 契約（provider 條款／費率／規格更新時的固定動作）**：

```
provider 更新 → sandbox 驗證 → shadow 比對（舊 policy vs 新 policy）→ 升 policy_version
                                                                        └─ 🔴 舊單沿用舊值
```

⚠️ **`rate_snapshot` 與 `policy_hash` 存在就是為了這件事**（§0.5、§5.1.2）：
改版時舊單必須沿用舊值。**沒有快照就沒有演進**——每次改費率都會把歷史帳一起改掉。

#### 5.1.9 `shipment_returns` ＋ `shipment_claims`（🆕）
退貨：`return_type`（驗退／未取退回／買家退貨）、`custody`、`fee_bearer`、
🔴 **`refund_independence`**（退貨**不自動觸發退款**，旗標明寫）、`evidence_asset_ids`、`deadline_at`、`outcome`。
理賠：`claim_reason`、`declared_value`、`claimed_amount`、`approved_amount`、`outcome`、`closed_at`。
⚠️ 金額文案一律「依條款審核，最高上限」（§0.5）。

#### 5.1.10 `reconciliation_cases`（🆕）
需求書只有一個 reference 欄，**沒有 case 表本身**。本表是 65 D-1 義③的載體：
`domain`、`case_type`、`status`、`sla_due_at`、`impact_amount`、`assignee`、`blocked_by`、
`subject_ref`。🔴 **不建常駐掃描器**——case 由對帳批次（K68-8，每日一輪、走 **lease**，§7.0）或人工建立，
符合《後端服務責任邊界規範》。

#### 5.1.11 `handoff_options`（🆕）
`handoffOptionId` 在需求書裡是「後端核發的 opaque id」，卻**無端點核發、無表存它**。
本書定案：它是本表一列（`id`(opaque)、`store_id`、`window_start`／`window_end`、`carrier_hint`、
`expires_at`、`consumed_by_batch_id`），由 §6.1.3 核發、**一次性消費**。
🔴 **不得讓前端自由傳入時間窗**——那等於讓前端決定 provider 的收件時段。

#### 5.1.12 孤兒欄位歸位

| 欄 | 歸宿 |
|---|---|
| `inventory_storage_mode` | 🔴 **不落表**——值恆為 `NONE`（N1）。落一個恆定值的欄位＝造一個沒有寫入者的維度 |
| `first_mile_mode`（`SELLER_ARRANGED`／`PROVIDER_PICKUP`） | `shipment_batches` 的一欄（**批次級**不是包裹級） |
| `parcel custody events` | `shipment_returns.custody` ＋ `shipment_batch_items.item_status`，**不另立事件表** |

#### 5.1.13 🔴 標籤檔儲存：對 65 §3.9 十一（《多媒體與圖片 URL 規範》的 objectKey 規則）的 explicit 例外裁決

需求書明確**排除**「一般 object storage 長期保存」，要求：加密保存、**短期** reference、SHA-256、授權下載、audit。

| 方案 | 判定 | 理由 |
|---|---|---|
| **(a) media-service ＋ 受控存取 ＋ TTL** | ✅ **採用** | 見下 |
| (b) 另立受控 encrypted store | ❌ | 它是**第二套 storage**（第二份憑證、第二條備份路徑、第二個 owner），而 repo 已有 `MediaGrpcClient.serverUploadAndFinalize`／`linkImages`／`unlinkImages` 的完整生命週期。為一種檔案另開一套＝第二個真相 |
| (c) 只存 hash ＋ 即時重取 | ❌ **技術上不成立** | `/print_label` 是**列印動作不是查詢**：吃 `ShipDate`（B2C 不得當日）與 ≤50 筆分組，重取＝**重新列印**，無法保證與已貼在包裹上的那張位元相同。且 repo 查無「以 `ShipTradeNo` 取回既有標籤」的端點常數（對照組 `PATH_PRINT_CVS_LABEL` 查得到 ⇒ 查詢有效） |

**(a) 的具體形式（五件缺一不可）**：
1. **儲存**：`MediaGrpcClient.serverUploadAndFinalize(bytes, filename, "application/pdf", AssetReferenceType.SHIPPING(7104))` ——與既有 C2C 標籤同一條路（`ShipmentLabelService.printAndUploadCvsLabel`，repo 實測）
2. **只存 objectKey**：`shipments.label_object_key`／`label_asset_id`（V1 既有欄）。🔴 **禁止回寫完整 URL**——`shipments.label_url` 已於 **V7** DROP，理由逐字「DB 一律只存 objectKey」
3. **短期 reference**：`shipments.label_expire_at`（V1 既有欄）＝交倉完成＋保存期；到期由排程 `unlinkImages` 解除引用（`ShipmentLabelService.unlinkLabel` 已存在）
4. **SHA-256**：`shipment_packages.label_sha256`（§5.1.4）。**上傳前算、下載後驗**——只存不驗＝沒有完整性保證
5. **授權下載＋audit**：一律經 shipping-service 的授權端點（§6.1.3），驗 `store_id` 歸屬後才組 URL，每次下載寫一列 audit。⚠️ `payuni_audit_logs` 既有規則是「二進位檔（PDF）**不落內容**」（V5 註解逐字）⇒ 只記 metadata（who／when／assetId／sha256）

⚠️ **偏離要寫出來**：《多媒體與圖片 URL 規範》沒有「TTL 後解除引用」，本書為它新增。偏離的是**保存期限**
不是**只存 objectKey**——後者完全遵守。

#### 5.1.14 既有表擴充（🔴 **②～⑤ 四件必須同批，缺一即靜默失效**）

**① `chk_shipping_method`：🔴 本書不碰。**
該 CHECK 的擁有者是 65（`V11` 已把它擴成含 `3004`／`3005` 的 5 值）；
再 `DROP/ADD` 一次會**把它的成果刷掉**（後跑的贏）。本書只做 `chk_shipping_type` 擴充：
```sql
-- V{N}__extend_shipping_type_for_b2c.sql
--   🔴 {N} 於施工時現讀 db/migration/ 決定：ls | sort -t_ -k1.2 -n | tail -1 的下一號。

-- 🔴 這裡刻意沒有 chk_shipping_method 的 DROP/ADD：該 CHECK 的擁有者是 65
--    （V11 已擴成含 '3004'／'3005' 的 5 值）。本檔再 DROP 一次會把它刷掉（後跑的贏）。

-- 🔴 四值不是三值：chk_shipping_type 目前仍是 V1 建立的兩值 ('3651','3652')，
--    而 DROP/ADD 之後只留下本檔寫的那組值 ⇒ 必須寫成含 69 '3654' 的超集，
--    否則 69 的 MERCHANT_DELIVERY（碼值由 65 附錄 C.1.1 指派）會被本檔刷掉。
ALTER TABLE seller_shipment_settings DROP CONSTRAINT chk_shipping_type;
ALTER TABLE seller_shipment_settings ADD  CONSTRAINT chk_shipping_type
    CHECK (shipping_type IN ('3651', '3652', '3653', '3654'));
    -- 3653 = CVS_B2C（本書）；3654 = MERCHANT_DELIVERY（69，見 69 §5.1.9 M6-2）

ALTER TABLE seller_shipment_settings
    ADD COLUMN credential_mode        VARCHAR(20) NOT NULL DEFAULT 'STORE_OWNED',
    ADD COLUMN back_set_shipping_at   TIMESTAMPTZ,
    ADD COLUMN gate_snapshot_at       TIMESTAMPTZ,
    ADD COLUMN gate_mer_status        VARCHAR(10),
    ADD COLUMN gate_member_status     VARCHAR(10);
ALTER TABLE seller_shipment_settings ADD CONSTRAINT chk_credential_mode
    CHECK (credential_mode IN ('AGENT_PROXY', 'STORE_OWNED'));
```
🔴 **`chk_shipping_type` 必須含 `'3654'`，這是本節最容易漏的一格**：
`'3654'` 是 69 的 `ShippingType.MERCHANT_DELIVERY`（65 附錄 C.1.1／C.2 定案）；
**`DROP/ADD` 之後只留下本檔寫的那組值** ⇒ 寫三值就等於把 `'3654'` 從值域移除。
症狀是 69 §5.1.9 (a) 的 `seller_shipment_settings` 那一列從本書上線那天起再也寫不進去（`23514`），
而**沒有任何編譯錯或測試紅**，且 68 自己的功能完全正常 ⇒ 只有 69 的賣家會建不出單。
⚠️ 這與上面 `chk_shipping_method` 的處置**方向相反但同一個道理**：
同一個 CHECK 的擁有者只能有一個，`chk_shipping_method` 的擁有者是 65（本書完全不碰），
`chk_shipping_type` 的**最後寫入者**是本書 ⇒ 本書必須寫**兩本值的聯集**。
**這一格要與 69 §5.1.9 M6-2 對齊，兩書一起改，不得只改一邊。**

**② `SLOT_DEFINITIONS` 新增兩槽（常溫／冷凍同批）**：
```java
new SlotDef(ShippingMethod.SEVEN_ELEVEN_BULK_WAREHOUSE.value(), ShippingTemperature.AMBIENT.value(),
        "7-11大宗寄倉（常溫）", "批次交寄至物流中心", true, SHIPPING_TYPE_CVS_B2C),
new SlotDef(ShippingMethod.SEVEN_ELEVEN_BULK_WAREHOUSE.value(), ShippingTemperature.FROZEN.value(),
        "7-11大宗寄倉（冷凍）", "批次交寄至物流中心", true, SHIPPING_TYPE_CVS_B2C),
```
其中 `private static final String SHIPPING_TYPE_CVS_B2C = ShippingType.CVS_B2C.value();`。
🔴 **第六個參數必須是 `SHIPPING_TYPE_CVS_B2C` 不是 `SHIPPING_TYPE_CVS`**——
填後者會與既有 7-11 C2C 兩槽產生**完全相同的 `buildSlotKey`**，
而 `Collectors.toMap` 的合併函式是 `(a, b) -> a`（原碼註解逐字「重複取第一筆」）
⇒ **B2C 靜默取到 C2C 的設定，不報錯、不轉紅**。
⚠️ B2C **要兩槽不是一槽**：`SPB2C` 與 `SPB2CFrz` 各自開通（§3.3 第 2 條）。

**③ `mapShippingMethodToType` 加一格**（**這是「B2C 的 onboarding 解析」那一格的答案**）：
```java
if (ShippingMethod.SEVEN_ELEVEN_BULK_WAREHOUSE.value().equals(shippingMethod)) {
    return SHIPPING_TYPE_CVS_B2C;
}
```
🔴 **漏了會怎樣（實測推導）**：`mapShippingMethodToType("3004")` 回 `null` → `findActiveSetting`
`return null` → 建單前置 `if (!UBER_INSTANT.equals(method) && onboardingSetting == null)` 成立
→ 拋 **`SHIPMENT_SELLER_NOT_ONBOARDED(5004)`**，訊息是「賣家尚未開通『7-11大宗寄倉』物流服務」
——**對一個已開通的賣家說他沒開通**。`printCvsLabel` 有同一道防線 ⇒ 兩處都會擋。
⇒ 這是**上線順序的第 0 步**。

**④ `shipping_method_configs` 兩列 ＋ ⑤ `shipping_fee_tiers`（`enabled = false`）**：
```sql
INSERT INTO shipping_method_configs
  (shipping_method, delivery_temp, max_dimension_sum_cm, max_single_side_cm, max_weight_grams,
   estimated_days, settlement_days, commission_rate, commission_min, enabled) VALUES
('3004', '3201', 105, 45, 10000, 3, 7, 0.0000, 0, false),
('3004', '3203', 105, 45, 10000, 3, 7, 0.0000, 0, false);
```
🔴 **上線前置：`enabled=false` 這個 fail-closed 形式今天不成立，必須同批改一行**：
`ShippingFeeService` 載入設定快取時**帶了 `.eq(ShippingMethodConfigEntity::getEnabled, true)` 的 filter**
⇒ **`enabled=false` 的列在快取裡與「這一列不存在」完全一樣**，`CheckoutValidator` 的 `config == null` 分支分不出來
⇒ 本書全部依賴 `enabled=false` 的地方（§0.2 `DISABLED_BY_SELLER`、§3.6 G6、§11.2.2、§12.0 封鎖態、§12.4 L1 停 checkout）
**會靜默退化成 B 類「設定不存在」**，而 U-6 要的是 A 類「查得到規則、規則說不可用」。
⇒ **修法在 65 §5.7**：快取改成載入全部列、由述詞層判 `enabled`，回 `DISABLED` 而不是 `CONFIG_MISSING`。
⚠️ 65 §5.7 那一行沒改之前，本書的 `enabled=false` 只是**看起來**有 fail-closed。
🔴 **這是全書唯一「我方依賴一個尚不成立的既有行為」的地方**；66 §3.4 與 69 §5.1.9 有同一個前置。

`commission_rate` 留 0 是**因為 rate card 未定**
不是不收費——開 flag 前必須先 UPDATE 成正式費率（§12.0 第 1／2 格）。
⚠️ 漏了這兩列＝65 D-13：`config == null` 直接 `continue`，選項**完全靜默消失**。

**⑥ CodeMeta 值域**：`ShippingMethod.SEVEN_ELEVEN_BULK_WAREHOUSE = 3004` 與
`ShippingType.CVS_B2C = 3653` 是本書使用的碼值（值域定案在 65 附錄 C.1.1／C.2，**本書只引用不自行裁決**）。
🔴 碼值的 SSOT 是 `docs/代碼管理（CodeMeta）/業務類型碼.md`，任何增修一律**先改 `.md` → `./generate.sh --all`
→ 同 PR 同步四份前端拷貝**，不得手改生成檔、不得只跑單一 target。

### 5.2 索引與查詢模式

| 述詞 | 索引 |
|---|---|
| 待成批清單 | `shipments (store_id, shipping_method, status)` WHERE `shipping_method='3004'` |
| batch 內的件／aging | `shipment_batch_items (batch_id, item_status)`；`(item_status, entered_at)` |
| inbox 未處理／對帳／gate 過期 | `provider_event_inbox (processed_at, received_at)` WHERE `processed_at IS NULL`；`provider_shipments (store_id, confirmed_at)`；`seller_shipment_settings (shipping_type, gate_snapshot_at)` |

### 5.3 序號與冪等鍵

**shipment idempotency key ＝ `order_no` ＋ `package_id` ＋ `service_product` ＋ `confirm_snapshot_hash`**
🔴 **不含重試次數、隨機值、時間戳**——含了等於每次重試都是新鍵，冪等直接失效。
批次號沿 65 §5.10.1 走 SEQUENCE（`shipment_batch_no_seq`）。

### 5.4 Schema 演進與 DB CHECK（expand-only）

| CHECK | 內容 |
|---|---|
| `chk_b2c_temp` | `shipping_method='3004'` ⇒ `delivery_temp IN ('3201','3203')`（**排除冷藏 3202**） |
| 排他性 | 同一 `store_id` **不得同時**有 `3651` 與 `3653` 的 `onboarding_status='3602'` 列 ⇒ 部分唯一索引 |
| 快照非空 | `provider_shipments` 的 `service_product`／`policy_hash`／`goods_type`／`ship_date` NOT NULL；`shipping_fee_ledger.rate_snapshot` NOT NULL |

🔴 **`3002` 只搭 C2C、`3004` 只搭 B2C**：新增 `chk_method_lgs_type`（`3002` ⇒ `lgs_type='C2C'`；
`3004` ⇒ `'B2C'`）。它擋的是 §4.6 第 3 列那個寫死常數被複製到 B2C 路徑上——**程式錯了 DB 會擋**。

### 5.5 不變式清單

| # | 不變式 |
|---|---|
| INV-1 | 🔴 **重播 100 次、亂序、跨日、多 replica，仍只得到一個 provider shipment、一筆費用、一個 normalized transition** |
| INV-2 | 一個 `shipment` 任一時刻最多屬於一個**未結案**批次 |
| INV-3 | `991` 期間**不得**建第二筆 `provider_shipments`；`/query` 回查無**不構成**重建理由。🔴 **兩個約束面（同一條不變式的兩半）**：① **對外部 recovery 者**——order-service 的 `ShipmentCreationBackstopScheduler` 在 `orders.shipping_status` 非 `NULL` 時必須退出（03 §8.11.1 **Backstop Ownership Contract**），否則「Shipping 正在 reconcile `991`」＋「Order 同時重新 create」兩條 recovery chain 會相撞；② **對我方端點**——C3 `/resubmit` 的 allowed-from **不含 `991`**（§6.1.4 C3），`991` 的合法動作只有 `requery`／`admin_resolve`（§7.2 X1） |
| INV-4 | `accepted+rejected ≤ expected`（65 已落 CHECK）；`DC_RECEIVED` 要求三者相等 |
| INV-5 | 🔴 **batch 狀態不得反推個件**（65 INV-A1，AST gate） |
| INV-6 | 同一次 `/print_label` 的所有件四參數完全相同且 ≤50 筆 |
| INV-7 | 退貨 ≠ 退款：建 `shipment_returns` **不得**直接寫任何退款狀態 |
| INV-8 | 未過 gate 的 `store_id` **不得**存在任何 `3004` 的 `provider_shipments` 列 |

### 5.6 本章的未決前提

| # | 未決 | 不定它會怎樣 |
|---|---|---|
| **U-6a** | 🔴 **`931`／`932` 的 provider 事件來源不明**：官方貨態碼表（repo 實測 §12）**只有 21 個碼，沒有一個是「驗收通過／驗退」**；最接近的 `22 物流驗收` 被 65 §7.1.2 明訂為 **C2C 語意、不重用於 B2C**，`98` 只是「待傳送至物流中心」。對照組：`98`／`22` 在該表查得到 ⇒ 查詢有效 | §3.7 A3 不成立 ⇒ **G4 沒有推進者**，batch 永遠停在 `DISPATCHED`。取得正式契約時第一個要問的就是「驗收結果怎麼回來」；在那之前 `931`／`932` 只有 Admin 人工入邊 |
| **U-6b** | `BackSetShipping` 的 wire key 大小寫與所在 Notify 型別 | §3.2.5 的補償設計無法寫測試 |
| **U-6c／d** | 正式 rate card 與退貨費 policy／母子廠商代號與指定物流中心 | 費率不能 seed、`enabled` 不能開；gate ① 永遠不過 |
| **U-6e** | 冷凍退貨流程（公開退貨作業只支援常溫） | 冷凍逆向走客服，**流程尚未定義**（基本設計書 §5.4 逐字） |
| **U-6f** | `credential_mode = AGENT_PROXY` 是否合法（§3.2.1） | 只能走 `STORE_OWNED`，賣家操作多一段 |

## 6. API 與 provider 整合

**覆蓋**：需求書 §21.3.1、需求書 §21.3.6、需求書 §21.8、§21.16

> 🔴 **先讀 §3 前的宣告**：B2C 與既有 C2C **走同一組 PayUni 端點、同一把代理商 Key／IV**，
> 差別只在請求參數 `LgsType`（`B2C` vs `C2C`）與服務產品代碼（`SPB2C`／`SPB2CFrz`）。
> 本章不新增任何「第二個 provider 整合層」，只新增**參數組法、批次語意與狀態映射**。

### 6.1 PayUni provider 端點：版控實證盤點

**實證錨**：`services/java/shared/payuni-java-client/.../shipping/PayUniShippingEndpoint.java`（路徑與版本常數）、
同 package 的 `ShippingClient`（方法）、`request/`／`response/` 下的 record。

#### 6.1.1 已存在的端點（B2C 直接沿用，零新增整合層）

| # | 端點 | 版本 | client 方法 | B2C 用途 | 本書是否改動 |
|---|---|---|---|---|---|
| E1 | `/api/logistics/setting` | 1.0 | `ShippingClient.onboardCvsShipping` | 建店（開通 `SPB2C`／`SPB2CFrz`） | 🔴 **要改**：`OnboardCvsRequest.lgsType` 目前由 `OnboardingService.CVS_LGS_TYPE` 寫死 `LgsType.C2C.code()` |
| E2 | `/api/logistics/trade` | 1.3 | `createCvsShipment` | 取號（產生 `ShipTradeNo`） | 🔴 **要改**：`CvsShipmentStrategy.LGS_TYPE_C2C` 寫死 |
| E3 | `/api/logistics/print_label` | 1.0 | `printCvsLabel`／`triggerCvsPrint` | 標籤列印 | 要改：分組規則（§6.1.3） |
| E4 | `/api/logistics/query` | 1.1 | `queryShipment` | 單筆狀態回查 | 只需傳 `lgsType="B2C"`（`QueryShipmentRequest.lgsType` 已是參數） |
| E5 | `/api/logistics/ship_map` | 1.1 | （`StoreMapService`） | 買家選門市 | 不改（B2C 仍是門市取貨） |

⚠️ **`/api/home_delivery/*` 五支端點與 B2C 完全無關**，不得誤入 B2C 路徑。

#### 6.1.2 需求書提及、**版控內不存在**的端點（本書必須新增或標為 RFI）

| # | 端點 | 證據 | 處置 |
|---|---|---|---|
| E6 | `/api/logistics/update`（限未列印前修改） | `PayUniShippingEndpoint` 無此常數 | **本書新增** client 方法；正式路徑與版本號屬 RFI-3，未取得前 **fail-closed**（不開放「已取號後修改」入口） |
| E7 | `search_merchant_status`（查子商店開通狀態） | 🔴 **它不在 `PayUniShippingEndpoint`（Java），而在單體 Go**：`internal/platform/payuni/client.go` 的 `MerchantStatus`，由 seller 模組的 `payuni.go` 呼叫並寫入 `seller.payuni_registrations`／`seller.payuni_observations` | 🔴 **不是「本書新增」，是「本書要擴的既有能力」，且擁有者是單體 seller 模組不是 shipping-service**（逐條處置見 §6.5）；⚠️ §3.2.5 已載明它**不回 `BackSetShipping`** ⇒ 它救不了 §6A.6 的單點風險 |
| E8–E10 | 需求書 §21.3.1 宣稱「已文件化 10 支」，其餘 3 支未在任何版控文件逐字轉錄 | — | 🔴 **open item**：P5 收尾前回需求書補齊，未補齊者不得進 §13.3 契約測試清單 |

#### 6.1.3 逐支契約（必填／回應／錯誤／冪等／重試）

**E1 `/api/logistics/setting`（建店）**

| 項目 | 內容 |
|---|---|
| 現有必填（`OnboardCvsRequest` 9 欄逐字） | `goodsType`／`lgsType`／`shipType`／`labelCompanyName`／`contactName`／`contactTel`／`contactMail`／`storeId`／`notifyUrl` |
| B2C 差異 | `lgsType="B2C"`；`shipType="1"`（`ShipType.SEVEN`）；`goodsType ∈ {"1" 常溫, "2" 冷凍}`——🔴 **`"3"` 冷藏不得傳**（`GoodsType.REFRIGERATED` 存在但 7-11 B2C 不支援，§0.1） |
| 🔴 `storeId`（退貨門市代碼） | **C2C 專用；B2C 不得傳**。B2C 的退貨去向由母子廠商代號與指定物流中心決定（需求書 §21.4 商務前置） |
| 回應（`OnboardShippingResponse` 逐字） | `PartnerID`／`EshipID`／`SetStatus`（⚠️ 三個 key 的 casing 以 `OnboardShippingResponse.from` 的 `data.getOrDefault` 為準，見 §4.4.1） |
| 冪等 | **provider 側無冪等鍵**。我方以 `(store_id, provider, service_product)` 唯一鍵擋重送；已存在 `REQUESTED`／`APPROVED` 者直接回既有列，**不再打 provider** |
| 重試 | ❌ **不自動重試**（§3.4：無更新／重新送審端點，重送＝可能開出第二個子商店）。逾時或未知 ⇒ 保留 `REQUESTED`／`UNKNOWN` ＋ 進 §7.6.1 人工查證佇列 |

**E2 `/api/logistics/trade`（取號）**

| 項目 | 內容 |
|---|---|
| 現有必填（`CreateCvsShipmentRequest` 13 欄逐字） | `merTradeNo`／`shipType`／`lgsType`／`goodsType`／`serviceType`／`senderName`／`senderPhone`／`receiverName`／`receiverPhone`／`storeId`／`tradeAmt`／`goodsName`／`refundStoreId` |
| B2C 差異 | `lgsType="B2C"`；`serviceType`：已付款訂單走「取貨不付款」（§3.5.3 定案 `EXTERNAL_PREPAID`）；`refundStoreId` **不傳**（C2C 專用，同 E1） |
| 回應（`CreateCvsShipmentResponse`） | `ShipTradeNo`／`ShipStatus` |
| 🔴 冪等鍵 | `order id + package id + service product + confirm snapshot hash`（§5.3），**不含重試次數**。同 key 重送 ⇒ 直接回既有 `ShipTradeNo`，**不打 provider** |
| 逾時處置 | ❌ **不得直接重建**。寫 `CREATE_UNKNOWN(991)` ⇒ 先 `/query`（E4）回查 ⇒ **成功轉 `92`**、失敗轉 `990`、超出回查窗轉 `41`＋人工。<br/>🔴 **回查成功轉 `92` 不是 `91`**：§7.3.2 **X1** 逐字「K68-3 回查：**成功→`92`**」（`/trade` 成功 ＝ `PROVIDER_CREATED` ＝ `92`，§7.2 **M2**）。65 §7.1.2.1 的 `991` 列是 provider-aware 的（`3001`／`3002`／`3003` → `91`；**`3004` → `92`**），本欄與它對齊 |
| ⚠️ 與建單兜底的關係 | ✅ **依 provider-neutral 的 Backstop Ownership Contract**（03 §8.11.1）：**Shipping 一旦把 `shipping_status` 投影回 Order，generic Order backstop 即退出**，recovery 的所有權歸 Shipping（K68-3 回查）／賣家（C3）／Admin。<br/>🔴 **本書的 `991` 有兩道退出點，兩道都成立**：① 依 §7.6 定案，`/trade` 取號由**賣家**發動（K68-1），而 `shipments` 列在**付款後**由既有建單鏈就建出（`91`，§7.3.1 第 1 列）並發 `shipment.created` ⇒ 進入 `991` 之前 `orders.shipping_status` **早已是 `'91'`**，本來就不在候選集裡；② `shipment.status.changed(991)` 的投影會把它寫成 `'991'`，同樣非 `NULL`。<br/>⚠️ **原措辭「排除已有 `991`／`990` 殼的訂單」已作廢**（見 §0.4）：`FL-1` 階段的 `990` 失敗殼不發事件、`shipping_status` 仍為 `NULL`，**那一筆兜底必須管**（03 §8.11.1 Case A），排除它是錯的。<br/>🔴 **`/trade` 逾時後的 `991` 一律不得由 Order 端重建**（`INV-3`）；跨服務測試見 §13.4 **I5** |
| 🔴 收件人姓名 | PayUni 對 `recipientName` 限中英數（見既有事故 `SHIP02080`）⇒ 送出前必過既有 `InputValidator` |

**E3 `/api/logistics/print_label`（標籤）**

| 項目 | 內容 |
|---|---|
| 現有必填（`PrintCvsLabelRequest` 5 欄逐字） | `shipTradeNo`／`goodsType`／`lgsType`／`shipType`／`shipDate` |
| 🔴 分組規則（硬上限，§0.1） | **每次最多 50 筆**，且同一次呼叫的 `goodsType`＋`lgsType`＋`shipType` **必須完全相同** |
| 🔴 B2C `ShipDate` | **不得為當日**（`ShipDate ≥ 今日+1`，且必須是可出貨工作天 ⇒ 走既有 `ShippingBusinessDaysPath` 的節假日判定） |
| 一 batch 500 件 | 後端自動切 10 組送出；UI 顯示「10 份 PDF」而非失敗（§7.1.3） |
| 回應 | `PrintCvsLabelDirectResponse.imageUrl`（HTML 解析後的圖片 URL）；🔴 **不得落庫**（§8.2） |
| 冪等 | 以 `(shipTradeNo, shipDate)` 為鍵；重送回同一份 label hash。**已列印的 shipment 禁止再送 E6 `/update`** |
| 重試 | 可 bounded retry（3 次、指數退避）。整組失敗 ⇒ 整組不改 `label_print_status`，逐筆失敗 ⇒ 該筆寫 `4104 FAILED`（沿用既有 `LabelPrintStatus` 四值機） |

**E4 `/api/logistics/query`（回查）**

| 項目 | 內容 |
|---|---|
| 必填（`QueryShipmentRequest` 逐字） | `shipTradeNo`（B2C 必填）、`lgsType`（必填）；`merTradeNo` 選填 |
| 回應（`QueryShipmentResponse`） | `ShipTradeNo`／`ShipStatus`／`ShipStatusTime`／`MerTradeNo` |
| 冪等 | 天然冪等（唯讀）。**降級模式下唯一允許的 provider 呼叫**（§8.3 query-only） |
| 🔴 不得覆蓋本地進度 | 回查結果套用時必須經 `ShipmentStatusMapper.isValidTransition`——`LOCAL_PROCESSING_DONE` 不得被 provider 回的出貨前狀態倒退覆蓋。🔴 **該集合的成員要從 `210`／`920`／`921` 擴成 ＋`930` ＋`98`，且倒退來源清單相應變成 `91`／`92`／`21`／`22`**（`98` 改為受保護方，理由與三條顯式邊見 §7.5「`98` 的歸屬」）；`931` 進 `IN_DELIVERY` |

**E5 `/api/logistics/ship_map`**：B2C 不改。買家仍選 7-ELEVEN 取貨門市，前端顯示文案固定為「7-ELEVEN 門市取貨」（§2.5 禁詞）。

**E6 `/api/logistics/update`（本書新增，fail-closed）**：僅允許 `label_print_status = 4101 NONE` 的 shipment；已列印一律回業務碼拒絕（§6.4）。正式契約未到齊前**整支不啟用**。

**E7 `search_merchant_status`（🔴 既有能力，本書只擴欄位）**：每日一次的 onboarding 對帳來源（§7.8 **L0**）。
🔴 **呼叫方是單體 seller 模組（Go）不是 shipping-service**，本書要做的是「解析回應時擴出 `SPB2C`／`SPB2CFrz`」而非新增一支 client——三件逐條見 §6.5 末段。
⚠️ 它不回 `BackSetShipping` ⇒ **不能**當成 §6A.6 的補償手段。

#### 6.1.4 我方對外端點

**規約**（→ 65 §五；本書不重述 envelope）：
- URL path／query 一律 **snake_case**；JSON body／response **camelCase**。
  🔴 §6.1.3 骨架稿寫的 `b2c-batches` **是錯的**（kebab-case 不合規），本書定為 `b2c_batches`。
- 🔴 **路由落點裁決**：批次資源**不掛 `/app/shipments` 之下**，另開 `/app/shipment_batches`。
  理由：`/app/shipments` 現有第一層 path param 是 `:order_no`（`gateway/internal/router/routes_shipping.go`
  的 `"/:order_no"+ShipmentsPrintCvsLabel` 等 5 條），**同層再引入第二個參數名會讓 gin 啟動即 panic**；
  雖然靜態 sibling 合法（`ShipmentsAnomaliesPath` 已是先例），但批次是獨立聚合根，另開 group 語意也更正確。
- Onboarding 沿用既有 group `/app/shipping_onboardings`，**新增靜態子路徑**（先例：`/cvs`、`/home_delivery`）。
  🔴 該 group 既有第一層參數名是 `:onboarding_id`，任何新增的參數式路由**必須沿用此名**。

| # | Method | Path | group | capability | 說明 |
|---|---|---|---|---|---|
| A1 | GET | `/app/shipping_onboardings/cvs_b2c` | `onboardings` | `CapProductManage`〔既有〕 | 開通狀態（11 欄，含安全化 `userMessage`） |
| A2 | POST | `/app/shipping_onboardings/cvs_b2c/apply` | 同上 | `CapProductManage` | 送件；需 `Idempotency-Key`。🔴 HTTP 200 **只代表已受理**，不代表已啟用 |
| A3 | POST | `/app/shipping_onboardings/:onboarding_id/toggle`〔既有〕 | 同上 | `CapProductManage`〔既有〕 | **本書不改端點**，只改後端對 `3004` 列的處理 |
| B1 | POST | `/app/shipment_batches` | 新 group | `CapOrderFulfill` | 建立批次（body：`orderNos[]` ＋ opaque `handoffOptionId`） |
| B2 | GET | `/app/shipment_batches` | 同上 | `CapOrderFulfill` | 列表（分頁走 `response.Page`，key 為 `records`） |
| B3 | GET | `/app/shipment_batches/:batch_no` | 同上 | `CapOrderFulfill` | 明細 |
| B4 | POST | `/app/shipment_batches/:batch_no/close` | 同上 | `CapOrderFulfill` | 🔴 §21.11 有 `OPEN→CLOSED` 邊卻無端點 |
| B5 | POST | `/app/shipment_batches/:batch_no/labels` | 同上 | `CapOrderFulfill` | 批次標籤（後端自動切 50 筆分組） |
| B6 | POST | `/app/shipment_batches/:batch_no/handoff` | 同上 | `CapOrderFulfill` | 🔴 **司機交接確認 ＝ `930 HANDED_OVER` 的唯一推進者** |
| B7 | POST | `/app/shipment_batches/:batch_no/items/scan` | 同上 | `CapOrderFulfill` | 掃描綁定包裹（body 帶 barcode，**不進 URL**，§8.1） |
| B8 | GET | `/app/shipment_batches/handoff_options` | 同上 | `CapOrderFulfill` | 🔴 `handoffOptionId` 的**核發端點**（需求書說由後端核發卻無端點無表） |
| C1 | POST | `/app/shipments/:order_no/b2c_label` | `shipments`〔既有 group〕 | `CapOrderFulfill`〔同層既有〕 | 單張列印。**參數名沿用 `:order_no`** |
| C2 ＝ **65 `S-11`** | POST | `/app/shipments/:order_no/cancel` | 同上 | `CapOrderFulfill` | 取消申請 ⇒ 寫 `992`（§7.4）。🔴 **路徑是 `/cancel` 不是 `/cancel_request`**（§4.5.0 末段：與 66 的 neutral 化一致，同一領域動作不得有兩個 URL） |
| C3 | POST | `/app/shipments/:order_no/resubmit` | 同上 | `CapOrderFulfill` | 🔴 **`CREATE_FAILED(990)` 的賣家自助重建（recovery action）。allowed-from ＝ `990` only。** `91` 與 `991` **一律拒絕**（`991` 回 `9520`，非 `990` 回 `9525 SHIPMENT_RECREATE_NOT_ALLOWED`）。<br/>🔴 **`resubmit` 與 `dispatch` 是兩個不同的 Domain Action，兩支端點都保留，不得 alias、不得合併成一支 URL**。<br/>**完整契約（actor／allowed-from／request／response／冪等／錯誤碼／副作用邊界）見 **§6.1.4a**。**<br/>⚠️ **`991` 不得進入本端點**：`INV-3` 逐字「`991` 期間**不得**建第二筆 `provider_shipments`；`/query` 回查無**不構成**重建理由」，§6.1.3 E2 亦逐字「❌ **不得直接重建**」。`991` 的合法動作只有 `requery`／`admin_resolve`（§7.2 **X1**），落點 **§6.1.4 E4 `POST /admin/shipments/:id/requery`**；賣家側對 `991` 只有唯讀「確認中」（§7.1.1 逐字）。⚠️ 動作名一律取自 `FulfillmentAction` 值域（CodeMeta 27 值，fail-closed ＋ 前端可見）——**該值域沒有 `reconcile`**。<br/>（§21.16 `canResubmit` ⇒ 只在 `990` 為 true） |
| C4 ＝ **65 `S-10`** 🆕 | POST | `/app/shipments/:order_no/dispatch` | 同上 | `CapOrderFulfill` | 🔴 **賣家宣告「貨已備妥」並發動逐筆取號（`/trade`，觸發 K68-1）＝ 正常 fulfillment action。** 本書不另立端點，直接沿用 **65 §6.3.4 A** 的 provider-neutral `S-10`（🔴 **必須帶 `65` 前綴**——本書自己也有一個 §6.3.4「`BackSetShipping` 補償設計」，裸寫會指到本書的錯誤節）；契約＝`DispatchShipmentRequest` → `ShipmentActionResp`，錯誤碼 `9520`／`9525`。<br/>🔴 **本書對 `S-10` 的 allowed-from 行使「逐 provider 收窄」（65 §6.3.4 A 明文允許）** ⇒ **只含 `91 READY_TO_CREATE`（正常 pre-`FL-2` 態）；不含 `990`（歸 C3 `resubmit`）、不含 `991`（歸 E4 `requery`）。** ⚠️ 收窄的是 `990` 這個**失敗態**，**不是**下界中的正常態，故不違反 65「不得移除下界中的正常態」那條硬規則。<br/>🔴 **這一列是 `91` 那顆「取號」按鈕的唯一落點**（§7.2 M1 與 §7.1.2 的 11 步流第 2 步都要求賣家在 `91` 按它）——`B1` 被 §7.6 明令禁止代為取號，接錯就等於取號做不到 |
| D1 | GET | `/app/shipments/b2c_worklists/:worklist` | 同上（靜態 sibling） | `CapOrderFulfill` | Seller 工作台 9 清單合一（`:worklist` 枚舉 9 值，見下） |
| D2 | GET | `/app/shipments/:order_no/fees` | 同上 | `CapOrderFulfill` | 費用四段（預估／provider 已入帳／待對帳／已對帳） |
| E1 | GET | `/admin/shipment_batches` | `admin`〔既有，已含 `JWTAuth + requireAdmin`〕 | Admin | control tower |
| E2 | POST | `/admin/shipment_batches/:batch_no/reconciliation_cases` | 同上 | Admin ＋ maker-checker | 建立差異 case |
| E3 | POST | `/admin/shipment_batches/reimport` | 同上 | Admin ＋ maker-checker | 重跑 importer |
| E4 | POST | `/admin/shipments/:id/requery`〔`:id` 沿用 `AdminShipmentDetail` 既有參數名〕 | 同上 | Admin | 重新 `/query` |
| E5 | POST | `/admin/shipments/:id/dc_result` | 同上 | Admin ＋ maker-checker | 🔴 **登錄驗收結果**：`931`／`932` 在 v1.4 的**過渡期唯一入邊**（§6A.3 (c)、§7.3.1 第 8 列、規格 §7.6.1） |
| E6 | POST | `/admin/reconciliation_cases/:case_id/close` | 同上 | Admin ＋ maker-checker | 🔴 **關案（含處置結果）**：batch `RECONCILING` 與驗退／理賠 case 的**唯一出邊執行者**（§7.6.1；**不是 Admin 直接改 state**） |
| E7 | POST | `/admin/shipping_onboardings/:onboarding_id/back_set_shipping` | `onboardings`〔既有 group〕 | Admin ＋ maker-checker | 🔴 **`BackSetShipping` 人工登錄**：漏一則 Agent Notify ＝ 該賣家永久無法建單且**無 API 可自我修復**（§6A.6 補償三件之②）。參數名沿用 `:onboarding_id` |
| E8 | POST | `/admin/shipments/:id/return_outcome` | `admin` | Admin ＋ maker-checker | 🔴 **退貨結案登錄**：寫 `shipment_returns.outcome ∈ {COLLECTED, ABANDONED, COMPENSATED}` ＋ `custody` ＋ `closed_at`。`RETURN_COLLECTED`（§7.3.2 X8）在 shipment 軸沒有值 ⇒ **沒有它，退貨軸永遠結不掉**，而 §8.2.1 #11 的 aging 正是錨在 `outcome IS NULL` |

🔴 **`:worklist` 的 9 值**（9 個清單收斂成一個端點，避免 9 條路由）：
`pending_trade`／`pending_label`／`pending_pack`／`pending_batch`／`batch_diff`／`dc_rejected`／`unclaimed_return`／`pending_seller_pickup`／`reconcile_abnormal`。

🔴 **E5–E8 都是「人直接寫一個 provider 沒說過的事實」** ⇒ 一律 maker-checker ＋ audit 六欄（§8.4），
它們是本書權限最高的四個動作。⚠️ **E7 掛在既有的 `shipping_onboardings` group**（不是 `shipment_batches`），
因為它改的是開通軸的欄位；其餘三支掛 `admin`。

⚠️ **代號撞名，引用時一定要帶節號**：本節的 `E1`–`E8` 是**我方 admin 端點**，
而 §6.1.2／§6.1.3／§6.5 的 `E1`–`E10` 是 **provider 端點**（`/api/logistics/*`、`/api/agent/*`）。
兩套在 `E5`–`E7` 完全重疊 ⇒ 全書一律寫「**§6.1.4 E5**」或「**§6.1.3 E5**」，**不得只寫 `E5`**。

🔴 **Admin group 歸屬（D-20）**：E1–E8 **八支全部掛 `admin`**（**不是 `adminNoAuth`**）。
理由：八支都會讀到賣家與買家 PII 或改變狀態，`adminNoAuth` 的存在理由是「登入前的公開 admin 資源」，不適用。

⚠️ **不要把這一條與上面的「路由前綴 group」搞混——那是兩個不同的軸**：

| 軸 | 值 | E7 的答案 |
|---|---|---|
| **認證 group**（D-20 問的） | `admin` ／ `adminNoAuth` | **`admin`**（八支都是） |
| **路由前綴 group**（掛在哪一組路由下） | `shipment_batches` ／ `shipping_onboardings`〔既有〕 | **`shipping_onboardings`**，因為它改的是開通軸的欄位 |

⇒ E7 **同時**是「掛 `admin` 認證」與「掛 `shipping_onboardings` 前綴」，兩者不衝突。
🔴 **本節上下文曾讀起來像互相矛盾**（一處說 E7 掛 `onboardings`、一處說 E1–E8 全掛 `admin`），
照結尾那句施工會把 E7 的路由前綴掛錯，也牴觸 §6A.6 對 `:onboarding_id` 的沿用要求。

#### 6.1.4a 🔴 `C3 /resubmit` 契約

> 本節照 65 §6.3.4（`S-10`／`S-11`）的同一個 pattern 寫出 `C3` 的 request／response schema。
> 它承載的是既有 action（`FulfillmentAction.RESUBMIT`，CodeMeta 逐字「**只有 990 建單失敗的配送單可重建**」），
> **不新增任何 Product 行為**。

##### A. 🔴 `dispatch` 與 `resubmit` 是兩個不同的 Domain Action

| 面向 | `dispatch`（正常履約） | `resubmit`（失敗後 recovery） |
|---|---|---|
| **端點** | `POST /app/shipments/:order_no/dispatch`（＝ 65 `S-10`，本書 `C4`） | `POST /app/shipments/:order_no/resubmit`（本書 `C3`） |
| **business intent** | 「**貨物已準備完成，開始正常履約。**」 | 「**前一次建立已確定失敗，重新送出。**」 |
| **allowed-from** | 正常 pre-`FL-2` 態；本書 ＝ **`91 READY_TO_CREATE` only** | **`990 CREATE_FAILED` only** |
| **前置事實** | 尚未對 provider 發出過履約承諾 | 前一次 provider create **已確定失敗**（不是未知） |
| **UI wording** | 「取號」／「開始出貨」 | 「重新送出」 |
| **audit intent** | 正常流程推進 | 異常恢復 |
| **observability** | 計入正常履約漏斗（`hawkr_b2c_trade_total{result}`，§8.2.1 第 2 列） | 計入 recovery 指標；**推走 `990` 的就是它**（`hawkr_b2c_create_failed_aging`，§8.2.2 ⑥） |
| **失敗處置** | 失敗 ⇒ 落 `990`（進入 recovery 軸） | 失敗 ⇒ **留在 `990`**，可再次 `resubmit`；逾時／未知 ⇒ `991`（轉回查軸） |

🔴 **兩支都保留。不得 alias。不得合併成一支 URL。不得因為實作可能共用 helper 就把 Domain Action 合併。**

⚠️ **明令禁止的三條邊**：`91 → resubmit`、`991 → resubmit`、`991 → dispatch`。
`991` 的合法動作**只有** `requery`／`admin_resolve`（§7.2 **X1**、`INV-3`）。

##### B. `C3 POST /app/shipments/:order_no/resubmit` 契約

| 面向 | 定版內容 |
|---|---|
| **語意** | 賣家對一筆**已確定建單失敗**的配送單重新送出建單請求（本書＝重打 provider `E2 /api/logistics/trade`） |
| **actor** | 🔴 **`SELLER` only。** capability ＝ `CapOrderFulfill`（與 `C1`／`C2`／`C4` 同層）。買家、系統排程一律不得發動；**Admin 的重建走 65 `A-6 POST /admin/shipments/:id/recreations`**（治理動作、admin namespace），不是本端點 |
| **ownership 驗證** | 🔴 **必過**：`:order_no` 對應的 shipment 其 `store_id` 必須等於 gateway 解析出的當前賣家 `store_id`（§6 身分分離：`account_id` 是登入 principal，營運鍵是 `store_id`）。不符 ⇒ 與「查無」同一回應，**不得**洩漏該單存在 |
| **allowed-from** | 🔴 **`990 CREATE_FAILED` only。** 其餘一律拒絕：`991` ⇒ `9520`；其他非 `990` ⇒ `9525 SHIPMENT_RECREATE_NOT_ALLOWED`（CodeMeta 逐字「只有建單失敗的配送單可以重新建單」） |
| **request** | 🔴 **無 body。** 完整輸入 ＝ 路徑參數 `:order_no` ＋ gateway 注入的賣家身分。<br/>⚠️ **不得為了形式硬加欄位**：重送的是「同一筆、同一份快照」，任何可由客戶端帶入的業務欄位都會讓它變成「改單＋重送」，那是另一個動作。<br/>⚠️ **也不得 reuse `DispatchShipmentRequest`**——65 §6.1.1 只給了它的名字、**未定義欄位集合**，照抄一個內容未知的 DTO 等於把未定義的欄位帶進 recovery 路徑 |
| **response** | `ShipmentActionResp`（🔴 **與 `S-10`／`S-11` 統一**，envelope 走 `{code, message, data, traceId}`）：回該筆的 normalized 狀態、`allowedActions[]`、以及本次是否真的打了 provider |
| **冪等鍵** | 🔴 **沿用 §5.3 的 shipment idempotency key，不變**（`order_no`＋`package_id`＋`service_product`＋`confirm_snapshot_hash`，**不含重試次數**）。⚠️ **重送不是新的一筆**——換鍵等於允許同一筆訂單長出第二個 `ShipTradeNo`（§7.9 **R1**） |
| **claim-before-side-effect** | 🔴 **本端點不在請求執行緒內呼叫 provider**，而是與 `C4` 走同一條路：**觸發 `K68-1 B2cTradeNoWorker`**，由它持 **lease（`120 s`，§0.4／§7.0）** 執行 `/trade`（30 s deadline、bounded 3 次）。<br/>🔴 **兩段式，且每一條邊都必須是既有合法邊**：<br/>**① 交易 A（搶佔）＝ CAS `990 → 91 UNPROCESSED`，COMMIT。** 這一步就是 claim——`status` 一旦離開 `990`，第二次 `C3` 立刻不滿足 allowed-from ⇒ 回 `9525`。語意上它就是 **`RESUBMIT`＝「重新建單」**（CodeMeta 逐字），與 65 `A-6`（Admin 重建→`91`）**對稱**。<br/>**② 交易外 ＝ 觸發 `K68-1`（持 lease `120 s`，§0.4／§7.0）執行 `/trade`**，再由它寫回：成功→`92`（§7.2 **M2**）／確定失敗→`990`（可再次 `resubmit`）／逾時或未知→`991`（轉回查軸，`INV-3`）。<br/>🔴 **為什麼不是「維持 `990` 直接打 provider」**：`ShipmentStatusMapper` 對 `from == CREATE_FAILED` 是**列舉分支**，出邊**只有 `{91, 41}`**（實碼大意：`to` 必須是 `UNPROCESSED` **或** `CANCELLED`，二者之外一律 false）⇒ **`990 → 92` 與 `990 → 991` 都是非法邊**，照那樣寫會 rows=0、DB 不動、只留一行 `unknownTransition` warn。⚠️ **本欄草稿曾寫成那樣，已更正**——而它與 65 §7.1.2.1 `990` 列的既有設計（出口只有「Admin 重建→`91`／Admin 取消→`41`」）也不一致。<br/>✅ **改成兩段式之後四條邊全部合法**：`990 → 91`（`CREATE_FAILED` 分支）／`91 → 92`、`91 → 990`、`91 → 991`（`91 ∈ PRE_SHIPMENT` ⇒ blanket allow）。<br/>⚠️ **本書不新增任何狀態值**（**§0.6** 逐字「用 65 §7.1.2 的 31 值，**本書不新增任何值**」；§7.2 亦逐字「本表只做映射，不新增任何值」）——**先前草稿曾寫「CAS `990 → 建單進行中`」，那個中間態不在 65 的 31 值內，已撤銷**。<br/>**端點自身冪等（兩道）**：① **CAS rows=0**（`status` 已不是 `990`）⇒ 回 `9525` ＋ 現況（`preserveData`）；② 該筆已有有效的 `K68-1` lease ⇒ 回 200 ＋ `9520` ＋ 現況，**不重複 enqueue、不第二次打 provider**。<br/>✅ **65 §6.3.4 B 的三條硬規則全數滿足**：① 外部呼叫**不在交易內**（在 worker 內、交易外）② claim 在副作用**之前**（lease 先取得）③ 逾時／5xx **一律寫「結果不明」不重送**（→`991`）。⚠️ **但承載物與 65 §6.3.4 B 不同、不要照抄它的字面**：65 那段的「以我方唯一鍵落一列 pre-commit」是給**「還沒有任何列」的首次建單**用的；`990` 已經有一張失敗殼，本端點沿用既有的 `K68-1` lease 當搶佔。<br/>🔴 **既有 `990` 殼一律重用同一列，不得新增第二筆 `shipments`／`provider_shipments`**（`INV-1`／§7.9 **R1**）。`provider_shipments` 只在 `/trade` 回 `SUCCESS` 的同一交易內建立（§5.1.3）⇒ 失敗時本來就沒有列，**冪等鍵不換也不會撞唯一鍵** |
| **🔴 前置順序（缺了就是端點回 200 而什麼都沒發生）** | 承 **65 §6.4.3** 對 `A-6` 的同一條警告——逐字：`ShippingService.createShipmentFromOrder` 第 1 步 `getShipmentByOrderNo` **命中就 `return existing`**，而失敗時 `persistFailedShipmentShell` 會落一張 `CREATE_FAILED(990)` 殼 ⇒ **重試被自己擋死**。<br/>🔴 **`C3` 是賣家側的同一件事，順序同樣固定**：① 先改寫 `createShipmentFromOrder` 讓 **`990` 殼不算「已存在」** → ② 才接上本端點。**只做 ② ＝ 端點回 200 而什麼都沒發生。**<br/>⚠️ 65 §6.4.3 只替 **Admin 側的 `A-6`** 寫了這條；本列是把它回灌到賣家側 |
| **逾時／未知** | 🔴 **由 `K68-1` 從 `91` 寫 `991`**（`91 → 991` 合法：`91 ∈ PRE_SHIPMENT`），**不得就地重送**（`INV-3`）⇒ 之後由 `K68-3` `/query` 回查收斂。**本端點自此不可再進入該筆**（它已不是 `990`） |
| **錯誤碼（一律既有碼值，不新增）** | `9520 SHIPMENT_STATE_TRANSITION_REJECTED`（`991`／已有進行中的重送）／`9525 SHIPMENT_RECREATE_NOT_ALLOWED`（非 `990`）／`9528 SHIPMENT_PROVIDER_MISMATCH`（該單不是 `3004`）。🔴 **不得臨時發明新碼**；需要新碼一律走 CodeMeta 流程 |
| **allowedActions 對齊** | §7.2 **Y1**（`990`）＝ `resubmit`／`admin_resolve`；§7.2 **M1**（`91`）＝ `dispatch`／`cancel`。**兩者不重疊**，與本契約一致 |

##### C. 🔴 實作可以共用 helper，但 Domain Action 不合併

```
C4 dispatch  ─┐
              ├─→ shared provider-create execution helper（本書＝ E2 /trade 的呼叫與落庫）
C3 resubmit  ─┘
```

🔴 **helper 必須收到 `actionContext = DISPATCH | RESUBMIT`（或等價資訊），且該值必須是必填參數。**

| 為什麼必須帶 | 不帶的後果 |
|---|---|
| **allowed-from 守門** | helper 退化成一支 unrestricted `create()`，`991` 也進得去 ⇒ `INV-3` 失效、建出第二筆 |
| **audit intent** | 稽核軌跡分不出「正常出貨」與「異常恢復」，事後無法回答「這筆重送過幾次」 |
| **metrics** | 正常履約漏斗與 recovery 指標混在同一個計數器 ⇒ **`990` aging 告警（§8.2.2 ⑥）失去意義**——它要量的是「失敗後多久沒人重送」，混進正常建單就永遠看起來很健康 |
| **error handling** | 兩者的失敗處置不同（見 A 表最後一列），同一個 catch 會把 recovery 的失敗當成正常建單失敗 |

⚠️ **可機械檢查**：helper 的簽章若沒有這個必填參數，或存在任何不帶它的呼叫路徑，即為違規。
🔴 **最終不得變成一支 unrestricted `create()`，讓狀態守門失效。**

#### 6.1.5 preview／confirm 與錯誤契約

- `previewId`／`expiresAt`／`eligibilityHash`／`rateCardVersion` 四欄由 checkout preview 回；confirm 時比對 `eligibilityHash`。
- 🔴 **需求書寫的 409／412，依 D-16 一律改為 HTTP 200 ＋ 業務碼**（envelope 不變、`data` 帶差異明細）。
- 🔴 **錯誤訊息紅線**：`userMessage` 不得含 raw payload／`MerID`／`PartnerID`／`EshipID`／簽章／合約識別／完整電話 Email 地址。
  違反即 §8.6 的 PII 事故，不是 UX 問題。

#### 6.1.6 業務碼（沿用 65 §六號段，本書不自造號段）

| 碼 | 名稱 | 觸發 |
|---|---|---|
| `9520`〔65 提案〕 | `SHIPMENT_STATE_TRANSITION_REJECTED` | allowed-from 不含目前狀態 |
| `9521`〔65 提案〕 | `SHIPMENT_CANCEL_ALREADY_PENDING` | 已有 `992` |
| `9522`〔65 提案〕 | `SHIPMENT_CANCEL_NOT_CANCELLABLE` | 已終態或 provider 不支援取消 |
| 本書新增 | `B2C_ONBOARDING_NOT_ACTIVE` | Activation gate 七項未全滿足 |
| 本書新增 | `B2C_BATCH_NOT_OPEN` | 對非 `OPEN` 批次加件 |
| 本書新增 | `B2C_LABEL_SHIP_DATE_INVALID` | `ShipDate` 為當日或非工作天 |
| 本書新增 | `B2C_LABEL_GROUP_MIXED` | 同一次 `/print_label` 混了不同 `GoodsType`／`ShipType` |
| 本書新增 | `B2C_UPDATE_AFTER_PRINT` | 已列印仍呼叫 E6 |

⚠️ 上述「本書新增」者的碼值依 65 附錄 C.6 落 CodeMeta（`.md` → `generate.sh --all` → 四份前端拷貝同 PR），**不得**手改生成檔。

#### 6.1.7 `disabledReasonCode` 12 值（前端顯示可信理由，禁止前端自行 fallback）

`NOT_ONBOARDED`／`ONBOARDING_PENDING`／`ONBOARDING_REJECTED`／`BACK_SET_SHIPPING_MISSING`／
`SKU_NOT_ELIGIBLE`／`PACKAGE_OVER_SPEC`／`TEMPERATURE_NOT_SUPPORTED`／`REGION_NOT_SUPPORTED`／
`PROVIDER_OUTAGE`／`SELLER_SUSPENDED`／`RETURN_BACKLOG_EXCEEDED`／**`PROVIDER_STATUS_UNKNOWN`**。

🔴 **第 12 個 `PROVIDER_STATUS_UNKNOWN` 不是可選的**（附錄 C.4 ④ 逐字，理由在 §14.3）：
§0.2 的 gate 三態要求 `UNKNOWN` 的 reasonCode 與 `BLOCKED` **不同**，而前 11 個沒有任何一個表達「我方讀不到」。
沒有它，三態在實作上塌縮成兩態，「真的沒開通」「provider 掛了」「我們讀不到」在營運端長得一樣。

#### 6.1.8 錯誤訊息紅線（🔴 不得顯示 raw payload／`MerID`／`PartnerId`／簽章／合約識別／完整電話 Email 地址）

**`userMessage` 是唯一可以進到五端畫面的字串，它只有一個產生點**：`B2cUserMessageResolver`
（`shipping-service`，🆕）。輸入是 `disabledReasonCode`（§6.1.7）或業務碼（§6.1.6），
輸出是**查表得到的固定文案**。

| 規則 | 內容 |
|---|---|
| 產生方式 | 🔴 **查表，禁止字串串接**。provider 回應、Java 例外 message、SQL 錯誤一律不得成為 `userMessage` 的任何一部分 |
| 禁止出現 | raw payload（`EncryptInfo`／`HashInfo` 原文）、`MerID`／`PartnerId`／`EshipId`／`AgentID`、簽章、合約識別、完整電話／Email／地址、完整 barcode（§8.1） |
| 允許出現 | 固定文案、`disabledReasonCode` 本身、`correlationId`（給客服對單用） |
| provider 原因給誰看 | 存 `payuni_audit_logs` 與 `shipment_anomaly_acknowledgments`，由 ADM（§6.1.4 E4）憑 role＋purpose 取用；**不經 `userMessage` 外流** |

🔴 **這是安全問題不是 UX 問題**：`userMessage` 同時出現在 Buyer App、Seller App、Web、
Sentry breadcrumb 與客服工單。provider 的 message 一旦被串進去，同一個字串會在五個地方各留一份，
而 §8.6 的遮罩只管 log、管不到已經回給前端的 response body。

**機械保證（依 65 §3.10 四問逐格作答）**：

| 問 | 答 |
|---|---|
| ① 觀測窗 | 契約測試對**每一個** `disabledReasonCode`（**12 值**，含 `PROVIDER_STATUS_UNKNOWN`）與**每一個**本書業務碼各造一次回應，斷言 `userMessage` 落在白名單集合內 |
| ② 窗外輸出 | 🔴 白名單裡沒有的碼 ⇒ **測試 FAIL**（不是跳過）。新增碼卻忘了寫文案，會在加碼那一刻轉紅 |
| ③ 誰能繞過 | (a) handler 直接 `c.JSON` ⇒ ruleguard `banDirectJSON` 擋得住；(b) 手寫 `response.Biz` 的 message 參數 ⇒ ⚠️ **擋不住**；(c) adapter 把 provider message 塞進 `data` 的其他欄位 ⇒ ⚠️ **擋不住**；(d) 例外訊息被 gateway 的通用 error mapper 透出 ⇒ ⚠️ **擋不住** |
| ④ 目標不存在 | 🔴 `disabledReasonCode` enum 被改名或搬走時要 **FAIL 不是 SKIP**：斷言「白名單長度 == enum 成員數」，兩邊對不上就紅 |

⚠️ **③ 的三個「擋不住」要寫進那支測試自己的檔頭**（65 §3.10 誠實條款）：
一道誠實標明邊界的 gate 是資產，一道被當成牆的 gate 是負債。

### 6.2 gRPC 介面

**不新增 service**，只在既有 `ShippingGrpcServiceImpl` 增 RPC；proto 只改 `api/proto/`。

| RPC（本書新增） | caller | 用途 |
|---|---|---|
| `GetB2cEligibility` | checkout（order／cart 鏈） | eligibility 四檢查點的收斂查詢 |
| `CreateB2cShipmentBatch` | — | ❌ **不建**：批次是賣家操作，只走 HTTP |
| `ListB2cBatchesForAdmin` | gateway admin | control tower |

🔴 **§5.3（API 同步責任）**：任何新增 RPC 必須同批補 Gateway handler，禁止孤兒 RPC。
🔴 **數值型別**：response 側一律 `string`（`make proto-lint-numeric` 的走訪起點是 RPC 的 OUTPUT type，
服務間 RPC 的 response 寫 `int64` **照樣擋紅**，`scripts/proto/numeric-allowlist.txt` ＋ `make proto-lint-numeric` 例外 3）。

---

### 6.3 PAYUNi Notify 接收（兩類）
#### 6.3.1 驗簽與 `HashInfo` casing 相容

**兩類 Notify 共用同一套驗簽**——它們走同一把代理商 Hash Key／IV（§6 章首宣告），不是兩套憑證。

| 步驟 | 規則 |
|---|---|
| 1. 取 raw body | 🔴 **必須用 raw bytes**，不得反序列化後再重新序列化——欄位順序或空白一改，`HashInfo` 必不相同 |
| 2. 解 `EncryptInfo` | 沿用 `PayUniHttpClient` 既有解密路徑，**不另寫第二份**（第二份＝第二個會漂移的憑證處理） |
| 3. 比對 `HashInfo` | 🔴 **大小寫不敏感比對**（`equalsIgnoreCase`）。官方文件未載明十六進位字串的大小寫；寫成 `equals` 時 provider 換一次大小寫＝全部 Notify 被拒，而**沒有任何錯誤訊息是我方發出的** |
| 4. 時間窗 | `Timestamp` 與現在相差 > 15 min ⇒ 拒收，記 `hawkr_b2c_notify_rejected_total{reason="stale"}` |
| 5. 落 inbox | 驗簽通過**才**落 `provider_event_inbox`（§6.3.2）；驗簽失敗的內容不落庫，只留 metadata |

🔴 **casing 陷阱有兩個，不是一個**：

1. **`HashInfo` 值本身的大小寫** —— 上表第 3 列。
2. **wire key 的大小寫**：`PartnerId` vs `Partnerid` vs `PartnerID`（RFI-4 逐字問的就是這題，§11.2.1）。
   ⚠️ Jackson 預設**靜默忽略未知欄位** ⇒ 猜錯的後果是讀回 `null`、不報錯、不轉紅。
   ⇒ 反序列化一律 `@JsonAlias({"PartnerId", "Partnerid", "PartnerID"})` 三種寫法同時吃，
   並在 §13.3 對三種寫法各斷言一次。**只吃一種＝把一個 RFI 的答案寫死成猜測。**

⚠️ **驗簽失敗不得回 5xx**：一律回 HTTP 200 並在本地記 `rejected`。回 5xx 會讓 provider
不斷重送一則**永遠不會通過**的訊息，把重送預算耗在一則壞掉的事件上，並拖慢後面正常的事件。

#### 6.3.2 inbox 與 exactly-once

**寫入路徑（順序不可換）**：

```
驗簽 → INSERT ... ON CONFLICT DO NOTHING 落 provider_event_inbox → 回 200 → consumer 憑租約取件處理
```

🔴 **不是**「處理成功才回 200」——那會讓一次下游 DB 慢查詢變成 provider 端的重送風暴。

| 項目 | 值 | 為什麼 |
|---|---|---|
| unique key | `(source, event_key)`，**落庫當下就成立** | §5.1.7：processed 之後才建 ⇒ 重送會落兩列，去重退化成應用層競態 |
| `event_key` 組法 | `lgsType : merId : shipTradeNo : shipStatus : shipStatusTime` | 三個修正見 §6A.5。缺 `lgsType` ⇒ B2C 與 C2C 互相吞；缺 `shipStatusTime` ⇒ 真實的貨態回退被當重播丟掉 |
| 衝突處置 | `ON CONFLICT DO NOTHING`（**不指名 conflict target**） | catch `DuplicateKeyException` 在 PostgreSQL 上必定失敗：交易 aborted（`25P02`）後所有後續 SELECT 一律 500（《PostgreSQL 設計規範與最佳實踐》） |
| 首見判定 | 以 **affected rows** 判 | `SELECT COUNT` 再 `INSERT` 不是原子（§6A.5 缺陷 1） |
| 取件 | lease（`process_lease_until`），走 65 §3.9 九「批次讀候選 → 逐筆副作用」 | 租約 **120 s**：下界＝一次本地 tx ＋ 一次 MQ publish（秒級）；上界＝ inbox 積壓告警窗 1 h |
| 失敗處置 | 寫 `last_error` ＋ `process_attempts`，**不刪列**；達上限進 §5.1.10 case | 刪列＝provider 曾經說過的話消失，而它無法重查 |

🔴 **exactly-once 的界線要說清楚，否則會被當成萬用保證**：

| 保證 | 由誰擋 | 不保證什麼 |
|---|---|---|
| 同一則 Notify 只被**處理**一次 | inbox 唯一鍵 | 不保證同一個狀態只被寫一次——provider 可用兩個不同 `ShipStatusTime` 推同一個狀態 |
| 同一個狀態只被**寫**一次 | `ShipmentStatusMapper.isValidTransition` 的第一個分支（`from == to ⇒ false`，實測） | 不保證事件不重複抵達 |

**兩道防線各擋一半，不可互相取代。** 只有 inbox ⇒ 同狀態異時間會重寫；
只有 transition 守門 ⇒ 業務副作用（發事件、記費用）已經做過一次。

#### 6.3.3 亂序／重播／延遲／跨日收斂

| 情境 | 規則 | 落點 |
|---|---|---|
| **亂序**（`32` 早於 `31` 抵達） | normalized 只單調前進：`31 → 32` 合法、`32 → 31` 不合法（`IN_DELIVERY → PRE_SHIPMENT` 被禁）⇒ 晚到的 `31` 被拒，狀態停在 `32` | `ShipmentStatusMapper.isValidTransition` |
| **重播** | inbox 唯一鍵擋第一層；`from == to ⇒ false` 擋第二層 | §6.3.2 |
| **延遲** | 🔴 **不得因為 `ShipStatusTime` 較舊就拒收**——單一事件上「延遲」與「亂序」分不出來。判準一律是 **normalized 轉換的合法性**，不是時間先後 | 同上 |
| **跨日** | `ShipStatusTime` 只存不判；業務日切換不影響狀態機。🔴 只有 `ShipDate`（列印用）走節假日判定（`ShippingBusinessDaysPath`），兩者是不同的東西不可混用 | §6.1.3 E3 |
| **合法回退** | provider 明確推一個較早的狀態（門市關轉 `81` 之後回 `31`）⇒ 這是 **reversal 不是倒退**：`isAbnormal → IN_DELIVERY` 現況已允許 | 同上 |

🔴 **「收斂」要寫成可斷言的形式，否則它只是一個形容詞**：
對同一個 `ship_trade_no`，把它收到的**全部** Notify 以**任意順序**重放**任意次數**，
最終 `shipments.status` 必須相同。這是 §13.4 的 I2／I3，也是 §5.5 INV-1 的一半。

⚠️ **`from == to ⇒ false` 有一個代價**：它讓「provider 重推同狀態」與「我方收到重播」
在 log 上長得一模一樣。⇒ 兩者必須用**不同的 counter**：

| 事件 | counter |
|---|---|
| inbox 命中唯一鍵（同一則再來一次） | `hawkr_b2c_notify_replayed_total{source}` |
| transition 被 `from == to` 拒（不同則、同狀態） | `hawkr_b2c_transition_noop_total{status}` |
| transition 被單調前進規則拒（真的倒退） | `hawkr_b2c_transition_rejected_total{from,to}` |

合成一支 counter ⇒ provider 真的開始亂推時，指標上看不出來（65 §3.10 ②：看不到就放行）。

#### 6.3.4 🔴 **`BackSetShipping` 只能從 push 取得且無法查詢**的補償設計（漏一個 Notify ⇒ 該賣家永久無法建單且無 API 自我修復）

**補償設計三件全文在 §6A.6，本節不重複裁決。** 這裡補 §6A.6 沒有寫的另一半——
**Notify 要先進得來**，而它的入口是在建店送件那一刻凍結的：

| 項目 | 值 | 漏了會怎樣 |
|---|---|---|
| `NotifyURL` 誰決定 | 🔴 **後端**，`/setting` 送件時帶入（§6.1.3 E1 必填欄含 `notifyUrl`）。前端不得傳、不得顯示 | 前端可傳＝賣家能把自己的 Notify 導去別的地方 |
| 落點 | `payuni-webhook-service`（既有 Notify 入口，與貨態 Notify 同一個服務、同一套驗簽） | 另開入口＝第二套驗簽 ＋ 第二份 inbox ＋ 第二個會漂移的憑證處理 |
| 環境隔離 | sandbox 與 production 的 `NotifyURL` 必須不同，且**由 config 給、不得寫死** | 寫死＝sandbox 的 Notify 打進 production inbox，而它會驗簽通過 |
| 🔴 不可變性 | `NotifyURL` 也在「建店資料不可修改」之列（§3.4） ⇒ **網域一旦送出就對該賣家永久固定** | 見下 |

🔴 **不可變性乘上 per-seller 送件，產生一個結構性後果**：
Notify 網域是**每個賣家送件當下凍結的值**，不是一個全域設定。
⇒ 日後若更換對外網域，**已開通的賣家不會跟著換**，只能逐一 `CLOSED` 後重新申請（§3.4 ③），
而 `3605 PERMANENTLY_CLOSED` 不可逆。
⇒ 這一條列入 §11.1 技術債，**現在就要寫下來**，不是等到換網域那天才發現。

⚠️ **送件前的機械檢查**：`/setting` 的 `notifyUrl` 必須等於 config 值且為 HTTPS，
兩者不符即拒絕送件（fail-closed）。這是**送出去就撤不回**的參數，
它不屬於「上線後再修」的那一類。

#### 6.3.5 檔案交換備援（若 provider 只提供 polling／檔案）

**這不是「以防萬一」的備案，它是 §6A.3 定案的落地前提**：`931`／`932` 在 v1.4 由
**(b) importer** 或 **(c) ADM 人工登錄** 寫入 ⇒ (b) 沒有規格，§6A.3 的定案就落不了地。

| 項目 | 規約 |
|---|---|
| 觸發方向 | 🔴 **拉取不是推送**：排程器每日一次去取。走 65 §3.9 九的 **lease**（批次讀候選 → 逐筆副作用），排程器名冊見 §7.0 |
| 落點 | 逐列進 `provider_event_inbox`（`source = FILE_IMPORT`），**與 Notify 共用同一張 inbox 與同一個 consumer**——兩套 inbox＝兩套去重＝兩個會分岔的真相 |
| 冪等鍵 | `event_key = FILE : file_serial : line_no`；同檔重跑逐列 `ON CONFLICT DO NOTHING` |
| 完整性三件 | 檔案序號連續 ＋ checksum 相符 ＋ 列數與 header 宣告相符。🔴 **缺任一件即整檔不處理**，不得逐列盡力而為 |
| 🔴 序號跳號 | **FAIL 不是 SKIP**：`file_serial` 不連續 ⇒ 建 `reconciliation_cases`（§5.1.10）並停止推進。跳過缺號繼續匯入＝那一天的驗收結果永久遺失，且沒有任何東西會說 |
| 可重跑 | 對同一個檔重跑必須得到相同結果（由冪等鍵保證）。重跑入口＝§6.1.4 的 `E3 POST /admin/shipment_batches/reimport`（maker-checker） |
| 退場 | 取得 provider 原生驗收貨態（§6A.3 (a)）之後，本路徑轉為**對帳用**（兩來源比對），不再是唯一推進者 |

⚠️ **目前沒有檔案格式**——7-ELEVEN 商務側是否提供對帳檔屬 §11.2.2 的商務前置，
未取得前 (b) 不成立，`931`／`932` 只剩 (c) 人工登錄一條路。

🔴 **只有 (c) 時的後果必須寫出來**：`DISPATCHED → DC_RECEIVED` 的推進速度＝人工作業速度
⇒ 65 已定的 **48 h aging 會常態性告警**。開 canary 前必須二選一：
① 拿到對帳檔讓 (b) 成立；② 把該門檻改成反映人工節奏的值**並在 §8.2.3 寫下理由與新的四問答案**。
🔴 **不得直接把那條告警關掉**——那是把「沒有人推走它」換成「沒有人知道」（65 §3.10 的第 6 種外殼）。

### 6.4 非同步事件（MQ）
#### 6.4.1 事件清單（需求書 16 個）

需求書 §21.15 列了 16 個事件名。**它們不是 16 條 routing key**——多數是同一條
`shipment.status.changed` 的不同 payload 狀態值。逐一歸位（🔴 答不出 producer 的一律標「不發」＋理由）：

| # | 需求書事件名 | 本書落點 | routing key |
|---|---|---|---|
| 1 | `ProviderShipmentRequested` | 🔴 **不發**：取號請求送出是一次外部呼叫、不是業務狀態；訂閱者為 0（65 §6A.1.4 的判準） | — |
| 2 | `ProviderShipmentCreated` | `shipments.status` → `PROCESSING(92)`／`CVS_CODE_RECEIVED(920)` | 既有 `shipment.status.changed` |
| 3 | `LabelIssued` | `label_print_status` → `4103`；狀態值 `920`（M3） | 既有（同上） |
| 4 | `PackageScanned` | `shipment_batch_items.item_status` → `SCANNED` | 🔴 **不發**：批次內部進度，訂閱者為 0；賣家看的是 §6.1.4 B3 的明細查詢 |
| 5 | `BatchClosed` | batch → `CLOSED` | 🆕 `shipment.batch.closed` |
| 6 | `BatchDispatched` | batch → `DISPATCHED`（＋個件 `930`） | 🆕 `shipment.batch.dispatched` ＋ 既有 `shipment.status.changed` |
| 7 | `WarehouseAccepted` | 個件 → `DC_ACCEPTED(931)` | 既有 `shipment.status.changed` |
| 8 | `WarehouseRejected` | 個件 → `DC_REJECTED(932)` | 既有（同上） |
| 9 | `ArrivedAtStore` | 個件 → `WAITING_PICKUP(32)` | 既有（同上，C2C 已在用） |
| 10 | `PickedUp` | 個件 → `PICKED_UP(11)` | 既有（同上） |
| 11 | `UnclaimedReturnStarted` | 個件 → `BUYER_NOT_PICKED(52)` | 既有（同上） |
| 12 | `ReturnArrivedAtWarehouse` | 個件 → `RETURN_TO_WAREHOUSE(53)` | 既有（同上） |
| 13 | `ReturnCollected` | 🔴 **不落 shipment 軸**：寫 `shipment_returns.outcome = COLLECTED`（§7.3.2 X8）。**不得映 `55`**——`55 SELLER_NOT_PICKED` 的 CodeMeta 語意是「商店**未**於期限內領取」，與「已領回」正好相反 | 🔴 **不發**：退貨軸的終態，訂閱者為 0；賣家看的是 D1 工作台的「待領退貨」清單 |
| 14 | `ProviderFeePosted` | `shipping_fee_ledger` 新列 | 🔴 **不發**：帳本是拉取模型（每日對帳 §7.8 讀它），沒有需要被推播的下游 |
| 15 | `ProviderSettlementPosted` | 同上 | 🔴 **不發**：同理；且 B2C 是 `SELLER_DIRECT` ⇒ **不進平台 payout**（§3.5），沒有 settlement 消費者 |
| 16 | `ShipmentClaimUpdated` | 個件 → `CLAIM_PENDING(993)`／`COMPENSATION(43)` | 既有 `shipment.status.changed` |

**歸位結果**：既有 key 承接 **9 個**、新增 batch key 承接 **2 個**、明確不發 **5 個**（9 ＋ 2 ＋ 5 ＝ 16）。
§6A.4 另外新增 4 條 batch key（`opened`／`dc_received`／`reconciling`／`reconciled`）
——那 4 條是**需求書漏掉的 batch 生命週期事件**（§6.4.2），不在這 16 個裡面。

🔴 **「不發」的四個必須寫理由，不能只是沒列**（65 §6A.1.4）：留白會讓下一個人以為漏了，
於是補一條沒有 consumer 的 key，而那條 key 上線後永遠不會有人發現它壞掉。

#### 6.4.2 🔴 **補齊**：`BatchOpened`、batch 層 `DC_RECEIVED`／`RECONCILING`／`RECONCILED` 的事件、
以及 7 個零事件狀態（`CREATE_UNKNOWN`／`CANCEL_PENDING`／`STORE_REDIRECT_REQUIRED`／`CUSTOMER_RETURN`／`LOST`／`DAMAGED`／`CLOSED_WITH_ADJUSTMENT`）

**兩件事，答案不同。**

**① batch 軸的 4 條事件落點在 §6A.4**（`opened`／`dc_received`／`reconciling`／`reconciled`；
連同 §6.4.1 已計入的 `closed`／`dispatched` 共 6 條）。本節不重複那張表。
🔴 需求書 §21.15 的 16 個事件名裡沒有它們，**不是因為不需要，是因為需求書只列了個件軸**——
而 batch 的 `DC_RECEIVED`／`RECONCILING`／`RECONCILED` 三格若無事件，
賣家與 ADM 只能靠輪詢畫面才知道進倉與對帳結果。

**② 7 個「零事件」狀態：它們不需要新事件，需要的是被正確地認出來。**

| 狀態 | 需求書為什麼看起來沒事件 | 本書的落點 |
|---|---|---|
| `CREATE_UNKNOWN`(`991`)／`CANCEL_PENDING`(`992`)／`STORE_REDIRECT_REQUIRED`(`81`)／`CUSTOMER_RETURN`(`51`)／`LOST`(`44`)／`DAMAGED`(`33`)／`CLOSED_WITH_ADJUSTMENT`(`43`) | 需求書 §21.15 的 16 個名字是**業務里程碑**，不是狀態機的全集 ⇒ 這 7 個沒被取名字 | 🔴 **它們全都是 shipment 軸的狀態值** ⇒ 一律走**既有** `shipment.status.changed`（§6A.2：貨態新增 routing key ＝ 0）。**不為它們造任何新 key** |

🔴 **「不造新 key」不等於「不用做事」**——每一個都要有 §6.4.1 同一套的四格才算完成：
① 誰推進（§7.3.2 逐列已答）② 誰推走（同上）③ `isValidTransition` 的 allowed-from 含不含它（`991`／`992`／`993` **必補**，§7.5）
④ 卡住誰知道（aging 指標名，§8.2.2 已登記）。
⚠️ **最容易漏的是 ③**：狀態值進得去、事件發得出，但 `isValidTransition` 讓那次寫入 `return false`
⇒ **一行 `log.warn`、沒有任何東西轉紅**，而事件根本不會被發（狀態沒變）。

#### 6.4.3 envelope 遵循聲明（§21 全章唯一的跨章引用）

**事件 envelope 的定義在 65 §4.6，本書零修改、零擴充、零例外。**
需求書 §21.15 對 envelope 只寫了一句「沿用第 6 章」——那是 §21 全章唯一一次跨章引用，
本書把它落實成一句可稽核的聲明：

> 本書新增的 6 條 batch routing key，payload 一律使用 65 §4.6 的 envelope
> （event id／aggregate id＋version／tenant＋seller／occurred time＋received time／
> correlation＋causation／schema version／producer），**不新增 envelope 欄位、不省略任何一欄**。

| 欄 | 本書的取值 |
|---|---|
| `aggregateId` | `shipment_batches.batch_no`（batch 軸）；個件事件仍是 `order_no`（既有） |
| `tenant` ／ `seller` | 🔴 賣家鍵一律 `store_id`（65 §3.9 一），**不得用 `account_id`** |
| `occurredAt` | 業務事實發生時間（例：`batch_handoffs.departed_at`），**不是** publish 時間 |
| `schemaVersion` | 新 key 一律從 `1` 起 |
| `producer` | `shipping-service` |

⚠️ **`occurredAt` 與 `receivedAt` 不可互相冒充**：交接離場時間是賣家登記的業務時間，
可以比 publish 早好幾分鐘；用 publish 時間充當它，`DISPATCHED` 的 48 h aging 就會從錯的起點算。

#### 6.4.4 `topology.yaml` 條目 ＋ 數量 pin 對帳

**結論：本書對 `infra/mq/topology/topology.yaml` 的增減量是 0，`test_expand.py` 的資源數量 pin 不變。**
推導在 §6A.2（貨態沿用既有 queue）與 §6A.4（batch 事件由 `notification.shipping` 的 `'#'` binding 涵蓋），本節只做對帳。

🔴 **pin 的現值於施工時現讀 `infra/mq/tools/test_expand.py`（`grep -n 'len(out\['`），本書不凍結數字**：
那份 pin 是**單一份**共用檔，65／66／69 的增減量都先落在它身上 ⇒ 抄任何一個快照值都會漂。

**對帳算式（逐項驗算，不得只抄結論）**：
1. 🔴 **總和是四項相加不是三項**——`sum(len(v) for v in out.values())` 把 `queues_json_args` 也算進去。
   ⚠️ 只加 exchanges／queues／bindings 會少掉 `queues_json_args` 那一項；
   **這一格漏掉就會把一個對的 pin 改成錯的**。
2. 一個帶 `dlq:` 的新 queue ＝ queues +2、bindings +2、exchanges +0、`queues_json_args` +0（65 §3.9 八）。
3. 本書新增 queue 數為 0（推導在 §6A.2 與 §6A.4）⇒ 四項皆 +0 ⇒ **一個數字都不用改**。

🔴 **本書的增減量是 0 ⇒ 不得去動 `test_expand.py` 的 pin**（改了就是替別人記帳）。
施工時先對帳增減量再決定要不要改數字，**不得為了讓測試變綠而改**（65 附錄 C.6 第 5 項明令禁止）。

🔴 **「+0」不是「不用做事」，它反而更危險**——因為沒有任何 gate 會被觸發：

| 會不會轉紅 | 項目 |
|---|---|
| ❌ 不會 | `mq-consumer-registration-check.py`（只掃 Go，本書是 Java） |
| ❌ 不會 | `test_expand.py` 的 pin（數量沒變） |
| ❌ 不會 | `validate.py`（topology 沒改） |
| ✅ 會 | `CodeMeta drift diff`（EventType 落 CodeMeta 時，§6.4.5） |

⇒ **本節的三件必做全部無 CI 保護，只能靠 review**（§6A.4 已列，此處只補「誰會抓到」這一欄）。

⚠️ **一個具體的反向陷阱**：若施工時判斷「B2C 要獨立 queue 以免拖累 C2C」（§6A.2 已列出這個代價），
那就**不再是 +0**：一個帶 `retry:`／`dlq:` 的新 queue ⇒ queues +2、bindings +2 ⇒
`test_expand.py` 的四個 pin 全部要改，**而且要先對帳增減量再改數字**，不得為了讓測試變綠而改。

#### 6.4.5 CodeMeta EventType

🔴 **物流是唯一沒有 EventType CodeMeta enum 的大領域**（65 §3.9 八，36 號 §3.3 自述）
⇒ 需要**建立**這個 enum，不是往既有的加值。建立者是 65（骨幹），本書只登記自己的 6 個成員。

| EventType 成員 | routing key | 觸發 |
|---|---|---|
| `SHIPMENT_BATCH_OPENED` | `shipment.batch.opened` | B1 建立批次 |
| `SHIPMENT_BATCH_CLOSED` | `shipment.batch.closed` | B4 關閉批次 |
| `SHIPMENT_BATCH_DISPATCHED` | `shipment.batch.dispatched` | B6 交接確認（同交易 outbox） |
| `SHIPMENT_BATCH_DC_RECEIVED` | `shipment.batch.dc_received` | 個件聚合推導成立 |
| `SHIPMENT_BATCH_RECONCILING` | `shipment.batch.reconciling` | 件數不符 |
| `SHIPMENT_BATCH_RECONCILED` | `shipment.batch.reconciled` | 差異 case 關閉 |

**流程**：`docs/代碼管理（CodeMeta）/內部技術碼.md` 加 6 條 → `./generate.sh --all`
→ **四份前端拷貝同 PR**（pre-commit 的 `codemeta-copies` 會擋）。

🔴 **一律 `--all`，不要自己判斷「這個 enum 前端看不看得到」**——CI 的 `CodeMeta drift diff`
跑的就是 `--all` 並逐位元對拍，判錯會紅在一個與邏輯無關的地方。
⚠️ 比對嚴格度不同：website／ADM 逐位元、兩個 Flutter 只比 `(code, value, label)`
⇒ 只改 description 時 Flutter 的 gate 會過而拷貝其實已不同步。

🔴 **只改 `.md` 不重新生成，會擋掉每一個 PR**（不只本書的）——`static-gates.yml` 會重跑
`generate.sh --all` 並逐位元對拍。新碼值的清單見附錄 C.4。

#### 6.4.6 36 號落點

**36 號《MQ 與事件總覽》是事件契約的兩套 SSOT 之一**（另一套是 `topology.yaml`，§6.4.4）
⇒ **兩邊都要改**；只改一邊的失效模式完全不同：

| 只改了 | 後果 |
|---|---|
| 只改 36 號 | 文件對、broker 沒資源 ⇒ 訂閱**靜默失效**（passive declare 不 crash、不進 DLQ） |
| 只改 topology | 資源有了、catalog 沒有 ⇒ 下一個人看 36 號會以為這條 key 不存在，於是新建一條同語意的 |
| 本書 | topology **增減量 0**（§6.4.4）⇒ 🔴 **只剩 36 號這一邊要改，而它沒有任何 CI gate** |

**逐條要寫進 36 號的欄位**（沿該文件既有欄位，不新增欄）：

| 欄 | 值 |
|---|---|
| exchange | `hawkr.shipping`（既有） |
| routing key | §6.4.5 的 6 條 |
| producer | `shipping-service`／`B2cBatchOrchestrator` |
| consumer | `notification-service`（queue `notification.shipping`，binding key `'#'`） |
| payload | 65 §4.6 envelope ＋ batch 欄（`batchNo`／`storeId`／`plannedCount`／`acceptedCount`／`rejectedCount`／`deliveryTemp`／`shipDate`） |
| 冪等 | consumer 端以 `eventId` 去重 |
| 失敗處置 | retry → DLQ（`notification.shipping` 既有 policy） |

⚠️ **`notification.shipping` 的 binding key 是 `'#'`** ⇒ 新 key 一上線就有 consumer，
**但「有 consumer」不等於「有投遞」**：`notification_templates.payload_filter`
對不上就是零投遞（§6A.4 第 4 件）。這是 65 §3.9 第 6 種外殼——機制存在但不做事——
所以 36 號那一列的「consumer」欄要**同時**寫 queue 與 template 名稱，只寫 queue 會讓人以為已經接上。

#### 6.4.7 部署順序

事件面的部署順序**只有一條硬約束：consumer 必須先於 producer 就緒**。
其餘順序由 §12.1 的部署／遷移順序決定，本節不重排。

```
① CodeMeta EventType（6 成員）＋ 四份前端拷貝        ← 同一個 PR，否則 pre-commit 擋
② notification_templates 的 payload_filter 與模板     ← consumer 先就緒
③ MqConstants 6 個常數 ＋ 36 號 catalog 6 條          ← 常數與文件
④ shipping-service 的 publisher（B2cBatchOrchestrator）← producer 最後
```

| 步驟 | 提前做會怎樣 | 延後做會怎樣 |
|---|---|---|
| ① | — | `generate.sh --all` 沒跑 ⇒ CI 逐位元對拍紅，且擋住**所有人**的 PR |
| ② | — | 🔴 producer 已在發、template 對不上 ⇒ **零投遞且零錯誤**；賣家收不到通知而系統一切正常 |
| ③ | — | 字面值散落（違反紅線 8）；36 號漏登記無 CI 會抓 |
| ④ | 事件發出去沒人收（`'#'` binding 下訊息進 queue 後被 consumer 當未知型別處理） | — |

🔴 **本書沒有 broker 資源變更**（§6.4.4）⇒ **沒有 `MQ Topology` workflow 要跑、沒有 destructive apply**。
⚠️ 反過來說：**部署後不會有任何東西告訴你 ② 有沒有做對**。
⇒ 驗收方式是**實跑一次**：dev 上手動 close 一個測試批次，斷言 notification-service
真的產生一則收件匣訊息（不是「queue 深度有動」——那只證明訊息進了 queue）。

### 6.5 provider adapter 對外呼叫契約總表（10 支 × 冪等／重試／逾時處置）

> **「10 支」的組成**（§6.1.2 E8–E10 那三支的身分在此收斂）：
> **代理商控制面 4 支**（`bind_merchant`／`link_merchant`／`update_merchant`／`search_merchant_status`）
> ＋ **物流 6 支**（`setting`／`trade`／`ship_map`／`print_label`／`query`／`update`）
> ＝ 需求書 §21.3.2 逐條列出的 10 支。兩類 Notify 是**入站**，不在本表（見 §6.3）。

| # | 端點 | 本書用不用 | 冪等 | 重試 | 逾時處置（timeout） |
|---|---|---|---|---|---|
| 1 | `/api/agent/bind_merchant` | ❌ 不用 | — | — | — |
| 2 | `/api/agent/link_merchant` | ❌ 不用 | — | — | — |
| 3 | `/api/agent/update_merchant` | ❌ 不用 | — | — | — |
| 4 | `/api/agent/search_merchant_status` | ✅ **gate ②③ 與 §7.8 L0** | 天然（唯讀） | bounded 3 次、指數退避 | 10 s；失敗 ⇒ gate 回 **`UNKNOWN`**（≠ `BLOCKED`，§0.2） |
| 5 | `/api/logistics/setting`（E1） | ✅ 建店 | 我方以 `(store_id, provider, service_product)` 唯一鍵擋重送 | 🔴 **不自動重試**（§3.4） | 30 s；未知 ⇒ 保留 `REQUESTED`／`UNKNOWN` ＋ 進 §7.6.1 人工查證佇列 |
| 6 | `/api/logistics/trade`（E2） | ✅ 取號 | `order_no + package_id + service_product + confirm_snapshot_hash`（§5.3） | bounded 3 次 | 30 s；未知 ⇒ 寫 `CREATE_UNKNOWN(991)` → `/query` 回查，🔴 **不得直接重建** |
| 7 | `/api/logistics/ship_map`（E5） | ✅ 買家選門市 | 天然（前景 Form POST，無我方副作用） | 由使用者重試 | 前景操作，不設後端 retry |
| 8 | `/api/logistics/print_label`（E3） | ✅ 標籤 | `(shipTradeNo, shipDate)` ⇒ 回同一份 label hash | bounded 3 次、指數退避 | 60 s（＋PDF 下載上傳，租約 300 s，§0.4）；整組失敗 ⇒ 整組不改 `label_print_status` |
| 9 | `/api/logistics/query`（E4） | ✅ `991` 回查 ＋ 降級模式唯一允許的呼叫 | 天然（唯讀） | bounded | 10 s；🔴 **查無 ≠ 不存在**（INV-3） |
| 10 | `/api/logistics/update`（E6） | ⚠️ **fail-closed 不啟用** | `(shipTradeNo, 修改欄位 hash)` | — | 契約未到齊 ⇒ 整支不啟用；契約測試只斷言**拒絕**（§13.3） |

🔴 **重試分類必須先過 §4.5.9 的六類判準，才輪到上表的「重試」欄**：
`AGENT02002`（B2C 未開通）屬 `business_reject` ⇒ **重試 0 次**；網路逾時屬 `retryable` ⇒ 才走 bounded retry。
把兩者混成「非 200 就重試」＝對一個已被明確拒絕的請求重試到耗盡，並且把 rate limit 燒在上面。

**client 七件（§4.5.10）逐件的驗收形式**：

| 件 | 驗收 |
|---|---|
| timeout | 每支都有**顯式** deadline（上表最後一欄）。🔴 無 deadline ⇒ 最壞耗時無上界 ⇒ 租約下界無從建立（65 §3.9 九） |
| bounded retry | 次數與退避是常數且可測；`business_reject` 類必須為 0 |
| circuit breaker | 沿用 `pkg/sms` 的 gobreaker 參數形狀；跳開時 §8.3 的降級模式生效（只留 E4） |
| rate limit | 值屬 RFI-9（§11.2.1）⇒ 未取得前以**保守值**設定並記 `hawkr_b2c_provider_throttled_total` |
| correlation id | 每次呼叫帶入並回寫 `payuni_audit_logs`，與 §6.1.5 的 `userMessage` 對得起來 |
| redacted audit | 🔴 見 §8.3：現況 `PayUniAuditMapper` 記完整 params，**B2C 上線前必補 PII 遮罩** |
| sandbox／production 憑證隔離 | 兩套 secret、兩個 `NotifyURL`（§6.3.4）；🔴 混用時 sandbox 的 Notify 會驗簽通過並打進 production inbox |

🔴 **實測修正一條 §6.1.2 的推論（對照組已驗）**：E7 `search_merchant_status`
在 `PayUniShippingEndpoint`（Java）確實零命中，**但它在單體 seller 模組（Go）已經實作並在跑**
（`internal/platform/payuni/client.go` 的 `MerchantStatus`，seller 模組 `payuni.go` 呼叫；
`seller.payuni_registrations` 已存 `mer_status`／`member_status`／`credit_status` 等六欄，
另有 `seller.payuni_observations` 保存每次轉換）。
⇒ **它不是「本書新增」，而是「本書要擴的既有能力」**，且**擁有者是單體 seller 模組不是 shipping-service**。

| 事實 | 證據 |
|---|---|
| 呼叫已存在 | `internal/platform/payuni/client.go`（`MerchantStatus` 呼叫 `/api/agent/search_merchant_status`） |
| 🔴 `SPB2C`／`SPB2CFrz`／`BackSetShipping` **全 repo 零命中** | 同一支 grep、同一個範圍；對照組 `LgsType` 在 `payuni-java-client` 命中 11 檔 ⇒ 查詢有效 |
| `seller.payuni_registrations` 無服務產品欄 | `internal/modules/seller/db/schema.sql` 的欄位清單無 `sp_b2c`／`back_set_shipping` |

⇒ **本書真正要做的是三件，不是「新增一支 client」**：
① 單體 `internal/platform/payuni` 的 `search_merchant_status` 回應解析**擴出** `SPB2C`／`SPB2CFrz` 兩欄（Go／Atlas）；
② 這兩欄要不要落 `seller.payuni_registrations`，取決於 gate ④ 的判定點——本書 §0.2 已定
**判定點在 shipping-service 的 `seller_shipment_settings.provider_status`**
⇒ 單體 seller 模組只做**傳遞**不做真相（否則同一個事實有兩個表，而它們會分岔）；
③ `BackSetShipping` **API 根本不回** ⇒ 只能走 §6A.6 的補償三件，不在本表任何一列裡。

⚠️ 這一格同時修正 §4.3 的一句話：shipping-service 讀單體 seller 模組走 legacy gRPC，`SELLER_SERVICE_HOST`／`SELLER_SERVICE_GRPC_PORT` 指向單體的 legacy gRPC 位址。

## 6A. 事件（provider 貨態的接收與去重）

### 6A.1 🔴 跨服務不能同交易——本節的前提

| 事實 | 錨 |
|---|---|
| provider Notify 落在 **payuni-webhook-service**（DB `hawkr_webhook`） | `services/java/payuni-webhook-service/` |
| shipment 表落在 **shipping-service**（DB `hawkr_shipping`） | `services/java/shipping-service/src/main/resources/db/migration/V1__init_shipping_tables.sql` 的 `shipments` |
| 兩者是**兩個獨立資料庫** ⇒ 只能走事件 | — |
| batch 聚合（`DC_RECEIVED`）與 shipment 個件**同一個 DB** ⇒ **可以同交易**（65 K-11 逐字「同交易」） | 同上 |

### 6A.2 貨態接收：**完全沿用既有路徑，新增 routing key ＝ 0**

**既有鏈路（逐段實證）**：

```
PayUni 貨態 Notify
  → payuni-webhook-service
      PayUniEventRouting.topic(SHIPPING_STATUS) = MqConstants.ROUTING_KEY_PAYUNI_SHIPPING_STATUS ("payuni.shipping.status")
      exchange = MqConstants.EXCHANGE_PAYUNI_WEBHOOK ("payuni.events")
  → queue  shipping.payuni.webhook   （topology.yaml 已有；binding key = payuni.shipping.status）
  → shipping-service  ShippingWebhookConsumer
      ApiType=Print → ShippingService.processWebhookPrint(shipTradeNo, partnerId, odno, validationNo)
      其他          → ShippingService.processWebhookStatusUpdate(shipTradeNo, payUniStatus, ...)
```

🔴 **B2C 為什麼不需要新的 key**：`ShippingStatusEvent` 本身就帶 `lgsType`（record 第 6 欄，javadoc 逐字
「通路類型（B2C, C2C, HOME, C2B）」），`ShippingPrintEvent` 同樣帶 `lgsType`。
⇒ B2C 與 C2C 在**同一條 queue** 上以 payload 欄位區分，consumer 端分流即可。

| 項目 | 值 |
|---|---|
| 新增 exchange | **0** |
| 新增 queue | **0** |
| 新增 binding | **0** |
| `infra/mq/topology/topology.yaml` 條目 | **0**（`shipping.payuni.webhook` 已存在，行 877–883） |
| `tools/test_expand.py` 資源數量 pin | **不變**（本書增減量 0；四項 pin 的現值現讀該檔，見 §6.4.4） |

⚠️ **這個「0」是有代價的，必須寫出來**：C2C 與 B2C 共用一條 queue ⇒ **B2C 的 poison message 會拖累 C2C**。
緩解＝consumer 內對 `lgsType` 分流後各自 try/catch，B2C 分支的例外**不得**讓整筆 nack 進共用 DLQ；
並在 §8.2 加一條「`shipping.payuni.webhook` DLQ 深度按 `lgsType` 拆維度」的指標。

### 6A.3 🔴 `DC_ACCEPTED(931)`／`DC_REJECTED(932)` 的推進者——誠實裁決

65 §7.1.2.1 把 `931`／`932` 的推進者寫成「68 的 provider 事件」。**本書必須指名它到底是什麼**，
否則就是只定義出邊不定義入邊（S0）。三個候選：

| 候選 | 成立條件 | 判定 |
|---|---|---|
| (a) PayUni 貨態 Notify 的某個 `ShipStatus` 值 | 需要 PayUni 書面給出 B2C 專屬貨態碼表 | 🔴 **RFI-5，未取得**。既有 `ShippingStatus` 24 值中**沒有**對應「物流中心驗收通過／驗退」的碼（`22 LOGISTICS_CHECK` 是 C2C 語意且屬 `PRE_SHIPMENT`，**明令不重用**） |
| (b) 檔案交換 importer（provider 每日給對帳檔） | 需要 7-ELEVEN 商務側同意 | §4.2 已列為備援；**本書取為 v1.4 的實際來源** |
| (c) 我方人工登錄 | 有 ADM 端點即可 | 取為**過渡期唯一來源**，且必留 audit |

**定案（fail-closed）**：
1. **v1.4 期間**：`931`／`932` 由 **(b) importer** 或 **(c) ADM 人工登錄**寫入，兩者都在 shipping-service 內、
   與 batch 聚合**同交易**（同一個 DB ⇒ 65 K-11 的「同交易」成立）。
2. **(a) 取得後**：改由既有 `ShippingWebhookConsumer` 分流寫入，**仍然不新增 routing key**（同 6A.2）。
3. 🔴 **在三者皆未就緒前，`931`／`932` 不得出現在任何 shipment 上**，且 `DISPATCHED` 的 48h aging
   （65 §7.1.1.2 I 節）必須先上線——否則批次會在 `DISPATCHED` 靜默堆積而沒有任何東西會響。

### 6A.4 本書**新增**的事件（batch 軸）與 topology 增減量

`930 HANDED_OVER` 是 shipment 軸的值，走**既有** `shipment.status.changed`（`MqConstants.ROUTING_KEY_SHIPMENT_STATUS_CHANGED`
→ queue `order.shipping.status`）⇒ **shipment 軸新增 routing key ＝ 0**。

batch 軸則完全沒有既有事件。新增如下（exchange 沿用 `hawkr.shipping`）：

| routing key（本書新增） | producer | consumer | 用途 |
|---|---|---|---|
| `shipment.batch.opened` | shipping-service `B2cBatchService` | notification-service | 賣家看板 |
| `shipment.batch.closed` | 同上 | notification-service | 封箱提醒 |
| `shipment.batch.dispatched` | 同上（B6 handoff 端點，同交易 outbox） | notification-service | 出車通知 |
| `shipment.batch.dc_received` | 同上（聚合推導成立時） | notification-service | 進倉完成 |
| `shipment.batch.reconciling` | 同上（件數不符） | notification-service ＋ ADM 佇列 | 差異告警 |
| `shipment.batch.reconciled` | 同上（case 關閉） | notification-service | 對帳完成 |

🔴 **consumer 為什麼不用新增 queue**：`notification.shipping` 的 binding key 是 `'#'`
（`infra/mq/topology/topology.yaml` 行 490–497 逐字），**綁定整個 `hawkr.shipping` exchange**
⇒ 上列 6 個新 key 一上線就有 consumer。

| 項目 | 增減量 |
|---|---|
| 新增 exchange | **0** |
| 新增 queue | **0** |
| 新增 binding | **0**（`'#'` 已涵蓋） |
| `topology.yaml` 條目 | **0** |
| `test_expand.py` pin | **不變** |

⚠️ **但 `topology.yaml` 為 0 不代表什麼都不用做**——三件仍必做，缺一即靜默失效：
1. `MqConstants` 加 6 個常數（**禁止字面值散落**，紅線 8）；
2. `docs/詳細設計書/36-MQ 與事件總覽（MQ & Event Catalog）.md` 加 6 條（SSOT 之一）；
3. CodeMeta `內部技術碼.md` 加 EventType → `generate.sh --all`（🔴 **一律 `--all`**，不要自己判斷前端看不看得到）。
4. notification-service 的 `notification_templates.payload_filter` 要有對應 template，
   否則「有 consumer」但**零投遞**——那正是 65 §3.9 第 6 種外殼（機制存在但不做事）。

### 6A.5 去重與 exactly-once：既有機制的三個實測缺陷

**既有**：`ShippingWebhookConsumer` 以 `eventKey = merId + ":" + shipTradeNo + ":" + shipStatus`
（Print 分支為 `merId + ":" + shipTradeNo + ":Print"`）查 `shipping_webhook_dedup`（`uk_dedup_event_key` UNIQUE）。

| # | 缺陷 | 後果 | 本書處置 |
|---|---|---|---|
| 1 | **check-then-insert 不是原子**（`selectCount` 後 `insert`） | 兩個 replica 同時收到同一則 ⇒ 兩次業務處理，第二次 insert 才撞唯一鍵。而 PostgreSQL 交易內任一句失敗 ⇒ **整個交易 aborted（25P02）**，之後所有指令被拒 | 🔴 改為 `INSERT ... ON CONFLICT DO NOTHING` ＋ 以 affected rows 判定是否首見（**唯一安全寫法**，《PostgreSQL 設計規範與最佳實踐》） |
| 2 | eventKey **不含狀態時間** | 同一狀態在不同時間重複推送會被當重播吞掉——對 `992`→回退的場景會丟失真實貨態 | 對 `992` 分支的 key 加 `ShipStatusTime` |
| 3 | eventKey **不含 `lgsType`** | 本書共用 queue 後，B2C 與 C2C 若曾出現同 `ShipTradeNo`（provider 未保證跨通路唯一）會互相吞 | key 前綴加 `lgsType` |

🔴 **不變式（§5.5 逐字）**：重播 100 次、亂序、跨日、多 replica ⇒ **只能得到一個 provider shipment、
一筆費用、一個 normalized transition**。缺陷 1 未修時這條**不成立**，且沒有任何東西會轉紅。

### 6A.6 `BackSetShipping` 單點風險（§3.2.5 的補償設計）

`BackSetShipping` **只能從 Agent Notify push 取得、無查詢端點**（E7 `search_merchant_status` 不回它）。
⇒ 漏一則 Notify ＝ **該賣家永久無法建單，且無 API 可自我修復**。

**補償三件**（缺一即留下一個沒人推得走的狀態）：
1. **aging 指標**：`hawkr_b2c_backsetshipping_missing_sellers`（開通已 `APPROVED` 但 `BackSetShipping` 未收到、逾 24h；登記在 §8.2.2 缺口⑤）
   ——§8.2.2 逐字要求的缺口之一（該節**六條**），需求書零指標。
2. **ADM 人工登錄端點**：`POST /admin/shipping_onboardings/:onboarding_id/back_set_shipping`
   （沿用既有參數名 `:onboarding_id`），maker-checker ＋ audit 六欄。
3. **對帳**：§7.8 **L0** 每日以 E7 拉一次子商店狀態，與本地 `provider_account` 對拍。

---

## 7. 狀態機與映射表（本書核心）

### 7.0 元件代號與排程器名冊（每支標明走 65 §九 哪一條 ＋ 租約）

**元件代號**（§4.2 的元件在本章一律用這些代號稱呼，避免同一個東西三種叫法）：

| 代號 | 元件 | 職責一句話 | 不做什麼 |
|---|---|---|---|
| **C-GATE** | `B2cActivationGate` | 七項 AND、三態輸出（§0.2） | 不寫任何業務表 |
| **C-ADPT** | `adapter/cvs711` | 組 B2C 參數、解 B2C 回應 | 🔴 不寫業務表、不是新的 HTTP 出口（§4.2） |
| **C-BATCH** | `B2cBatchOrchestrator` | 批次開／關／交接／聚合 | 🔴 不得反推個件狀態（INV-A1） |
| **C-GRP** | `B2cLabelGrouper` | ≤50 筆 ＋ 四參數一致分組 | 不決定 `ShipDate`（那是批次的欄位） |
| **C-RECON** | `B2cReconciler` | 每日五層對帳（§7.8） | 🔴 不改值，只建 case（§5.1.10） |
| **C-INBOX** | provider event inbox consumer | 取件、去重、推 normalized | 不驗簽（驗簽在 webhook edge，§6.3.1） |
| **C-MSG** | `B2cUserMessageResolver` | 碼 → 固定文案（§6.1.8） | 不接受任何 provider 字串 |

**排程器名冊**——每支標明走 65 §3.9 九的哪一條、租約推導與指標：

| # | 名稱 | 65 §九 走哪條 | cadence | 租約 | 上下界推導 |
|---|---|---|---|---|---|
| K68-1 | `B2cTradeNoWorker`（逐筆 `/trade`） | **lease** | 🔴 **由 `C4`（＝ 65 `S-10 dispatch`）觸發**，`fixedDelay 30s` 掃殘留。🔴 **不由 B1 觸發**——§7.6「B1 不代為取號」（§6.5 #6） | **120 s** | 下界＝ `/trade` deadline 30 s × retry 3；上界＝批次 `OPEN` 壽命（小時級，§0.4） |
| K68-2 | `B2cLabelPrintWorker`（每組 ≤50 筆） | **lease** | 由 B5 端點觸發 | **300 s** | 下界＝ `/print_label` deadline 60 s ＋ PDF 下載上傳；🔴 **50 筆是同一次外部呼叫，不可逐筆取租約** |
| K68-3 | `ShipmentCreateUnknownRecheckScheduler`（`991` 回查） | **lease**（65 表 A #15，本書沿用不另訂） | `fixedDelay 600s` | **10 min** | 窗口＝ `CREATE_UNKNOWN` 回查窗 **≤180 min**（§0.4，🔴 **不得用 24 h 草案值**）⇒ 窗口內約 18 次 |
| K68-4 | `CancelPendingTimeoutScheduler`（`992` 逾時） | **lease**（65 表 A #16） | `fixedDelay 600s` | **10 min** | 窗口 ∞（provider 可能永不回覆）；升級門檻另計（值待 RFI-7，§11.2.1） |
| K68-5 | `B2cBatchCutoffScheduler`（`OPEN` 截止自動關閉） | **lease** | `fixedDelay 600s` | **60 s** | 下界＝一次本地 tx ＋ 一次 outbox 寫入（秒級）；上界＝ `OPEN` aging 24 h |
| K68-6 | `B2cOnboardingStatusSyncScheduler`（E7 每日拉狀態） | 🔴 **lease，不是 level-triggered** | `cron 每日一次` | **10 min** | 下界＝一次 `search_merchant_status`（deadline 10 s）× 賣家數分批；上界＝一日 |
| K68-7 | `B2cDcResultImportScheduler`（驗收結果 importer，§6.3.5） | **lease** | `cron 每日一次` | **15 min** | 下界＝一次檔案取得 ＋ 逐列落 inbox；上界＝一日 |
| K68-8 | `B2cReconcileScheduler`（每日五層，§7.8） | **lease** | `cron 每日一次` | **30 min** | 下界＝五層查詢全跑一輪；上界＝一日 |
| K68-9 | C-INBOX 取件 | **lease**（65 §九 第一列的形狀） | `fixedDelay 10s` | **120 s** | 下界＝一次本地 tx ＋ 一次 MQ publish；上界＝ inbox 積壓告警窗 1 h（§5.1.7） |

🔴 **K68-6 為什麼不是 level-triggered**：它形狀上確實是「狀態同步（絕對值）」，
但 65 §3.9 九對 level-triggered 的第一條硬要求是**單一寫者＝部署形狀**（`replicas: 1`
**且** `strategy.type: Recreate`）。實測 `k8s/values/shipping-service.yaml` 沒有覆寫
deployment strategy ⇒ 沿用 chart 預設 `maxSurge: 1` ⇒ **每次部署都有兩個 pod 並存**
⇒ 單一寫者不成立。⚠️ 「今天 `replicas: 1`」**不是**理由——那正是 65 §7.5 開頭警告的
「擴容那一刻才引爆」。⇒ 走 lease，成本只有一個租約欄。

🔴 **aging 指標不要寫成排程器**（65 §7.5.0 逐字）：`hawkr_b2c_*_aging` 系列由
`monitoring/sql-exporter/sql_exporter.yml` 的 SQL 產生（§8.2.2）。
為它寫一支排程器＝多出一個需要搶佔、需要租約、而且掛掉之後沒人知道的東西，
去做一件 exporter 已經在做的事。

**每支都要有的兩支指標**（沿 65 §7.5.1，不是樣板文字）：
`fulfillment_scheduler_claim_total{scheduler, result}`（counter）與
`fulfillment_scheduler_last_success_timestamp_seconds{scheduler}`（gauge）。
告警一律 **`absent()` ∨ `age > 3×cadence`**——🔴 `absent()` 那一半不是裝飾：
排程器**從來沒跑過**（DI 沒 wire、`@Scheduled` 打錯）時，`age > 3T` 永遠不會響，
因為根本沒有 series 可以比較。

### 7.1 §21.11 的 29 個值：兩軸清單

> **來源**：65 §D-12 逐字「29 個相異狀態名，分屬兩條互不相交的軸：shipment 軸 23 個（主線 11 ＋ 例外 12）
> 與 batch 軸 6 個」。
> 🔴 **第 12 個例外值是 `RETURN_COLLECTED`**（需求書 §21.15 的 `ReturnCollected`，§6.4.1 第 13 列）。
> normalized 值域裡沒有它的位置 ⇒ **不落 shipment 軸、落退貨軸**
> （`shipment_returns.outcome = COLLECTED`）。編號一律用 §7.3.2 那一套（它是 X8）。

**主線 11 值**（來源：65 §D-12 的 normalized 對照表逐列）：
`READY_TO_CREATE`／`PROVIDER_CREATED`／`LABEL_READY`／`PACKED`／`ASSIGNED_TO_BATCH`／
`HANDED_TO_CARRIER`／`ARRIVED_AT_DC`／`DC_ACCEPTED`／`IN_TRANSIT_TO_STORE`／`AT_STORE`／`PICKED_UP`

**例外分支 12 值**（來源：本書 §6.4.1 與 §7.3.2；編號 X1–X12 以 §7.3.2 為準）：
`CREATE_UNKNOWN`／`CANCEL_PENDING`／`DC_REJECTED`／`STORE_REDIRECT_REQUIRED`／`UNCLAIMED_RETURN`／
`CUSTOMER_RETURN`／`RETURN_AT_DC`／**`RETURN_COLLECTED`**（X8）／`LOST`／`DAMAGED`／`CLAIM_PENDING`／`CLOSED_WITH_ADJUSTMENT`

**batch 6 值**：`OPEN`／`CLOSED`／`DISPATCHED`／`DC_RECEIVED`／`RECONCILING`／`RECONCILED`

#### 7.1.1 🔴 **`/trade` 的時點裁決**（需求書 §21.10 說「成批**前**逐筆取號」、§21.16 說「建立批次**時**逐筆 `/trade` 可部分成功」⇒ **兩節相反**）

**裁決全文在 §7.6（採 需求書 §21.10：成批前逐筆取號），本節不重複裁決。** 這裡只補兩件 §7.6 沒寫的：

**① §21.16 那句話有一半仍然成立，不要整句丟掉。** 「逐筆可部分成功」講的是**取號這個動作**
本身可以部分成功，這一點本書完全保留——只是它不發生在批次建立端點裡：

| §21.16 的語意 | 本書落在哪 | 差別 |
|---|---|---|
| 逐筆呼叫、可部分成功 | K68-1 `B2cTradeNoWorker`（§7.0） | ✅ 保留 |
| 成功項保留、失敗項分開顯示 | D1 工作台 `pending_trade`／`batch_diff`（§6.1.4） | ✅ 保留 |
| 結果 `UNKNOWN` 只顯示「確認中」、不得立即重送 | `CREATE_UNKNOWN(991)` ＋ K68-3 回查（§7.4 S0-4） | ✅ 保留且更嚴格 |
| **綁在「建立批次」這一個端點裡** | 🔴 **不採用** | 見 §7.6 三條理由 |

**② 這一格決定了 `ASSIGNED_TO_BATCH` 的前置條件，而那是一個 DB 層可守的不變式。**
批次建立端點（B1）對「尚未取號」的訂單一律回 `9520`、**不代為取號**（§7.6 末段）
⇒ `shipment_batch_items` 的插入路徑必須驗 `shipments.ship_trade_no IS NOT NULL`。
🔴 **只寫在 service 層不夠**：加一條 CHECK 或 FK 語意的守衛，否則下一個人新增一個
「快速成批」入口就繞過了（同 §2.6 排他性落在資料層不是 UI 的理由）。

#### 7.1.2 🔴 「訂單 release」的定義、觸發者與端點（需求書未定義）

**定案：「訂單 release」不是新狀態、不是事件、不是端點，而是一個述詞。**

**理由三條**：
1. 需求書全章只在 需求書 §21.10 的作業流第一步出現這個詞，**沒有第二處定義它**、action matrix 沒有它、
   需求書 §21.15 的 16 個事件沒有它 ⇒ 它是**流程敘述**不是領域概念。
2. 若造成狀態，65 §3.11 四問的 ①②③ 全部答不出來（誰推進？誰推走？allowed-from？）
   ⇒ 依判準「①或②答不出即刪」。
3. 它要表達的事實**已經有承載物**：`shipments` 列存在且 `status = UNPROCESSED(91)`。
   再造一個＝同一個事實兩個真相，而它們會分岔。

**述詞（＝ D1 工作台 `pending_trade` 的候選條件）**：

```sql
shipments.shipping_method = '3004'
  AND shipments.status         = '91'      -- UNPROCESSED：已建 shipment、尚未取號
  AND shipments.ship_trade_no IS NULL
  AND shipments.store_id       = :storeId
  AND shipments.is_return      = FALSE
  AND NOT EXISTS (該筆有未結案的取消申請 992)
  -- 以下不在 SQL 內，由 C-GATE 逐筆求值（跨服務、跨 DB，§0.2）
  AND B2cActivationGate(store_id, goods_type) = ELIGIBLE
```

🔴 **gate 要求值兩次不是一次**：清單載入時一次、**取號當下再一次**。
清單是快照；賣家開著頁面十分鐘後才按下去，gate 可能已經變了（快照有效期 15 min，§0.4）。
只驗清單＝把一個副作用的授權建立在一份過期的讀取上。

**誰讓述詞成立**（＝真正的「release」觸發者，全部是既有機制，本書零新增）：

| 步 | 誰 | 落點 |
|---|---|---|
| 訂單付款完成 | order-service | 既有建單鏈 |
| 建 `shipments` 列 | shipping-service 消費建單事件 | 既有 |
| 兜底 | `ShipmentCreationBackstopScheduler`（order-service） | 既有。🔴 **它只負責「Shipping 還沒接手」的那一段**：述詞 `orders.shipping_status IS NULL`。**`shipments` 列建出並發 `shipment.created`（`shipping_status='91'`）的那一瞬間，兜底就退出**，之後 `91`／`92`／`98`／`991`／`990` 全程與它無關（03 §8.11.1 **Backstop Ownership Contract**）。與 `991` 的完整推導見 §6.1.3 E2 |

---

**交倉作業流 11 步**（需求書 §21.10 逐步；🔴 標「人工」者系統只留痕不驅動）：

```
 1. 訂單 release ........... 系統（述詞成立，見上）        → shipments 91
 2. 逐筆 /trade ............ 賣家按「取號」→ C4 → K68-1   → 92 / 920，失敗留 991
 3. 取得標籤 .............. 賣家按「列印」→ K68-2         → label_print_status 4103
 4. 揀貨／包裝 ............ 🔴 人工                        → （無系統狀態，刻意不造，§7.4 S0-1）
 5. 重量尺寸複核 .......... 🔴 人工 ＋ B7 掃描時回填       → shipment_packages 尺寸重量欄
 6. 掃描綁定包裹 .......... 賣家 B7 端點                   → item_status SCANNED
 7. 加入內部進倉批次 ...... 賣家 B1 端點                   → item_status ＋ batch OPEN
 8. 關閉內部批次 .......... 賣家 B4 端點／K68-5 截止排程   → batch CLOSED
 9. 司機交接 .............. 賣家 B6 端點（🔴 930 的唯一推進者）→ batch DISPATCHED ＋ 個件 930
10. 物流中心驗收 .......... 🔴 **我方無法驅動**：importer（§6.3.5）或 ADM 人工登錄 → 931 / 932
11. 差異回報 .............. C-RECON 或人工 → reconciliation_cases → batch RECONCILING
```

🔴 **第 4、5 步刻意沒有系統狀態**：需求書的 `PACKED` 在 §7.4 S0-1 已判定「無推進者 ⇒ 不獨立造，
併入 `920`」。把人工動作造成狀態，會得到一個永遠靠人去點的格子，而沒有人會去點。
🔴 **第 10 步是整條流程唯一我方推不動的一步**，它是 §6A.3 與 U-6a 的來源，
也是 `DISPATCHED` 48 h aging 會不會常態告警的決定因素（§6.3.5 末段）。

#### 7.1.3 批次件數上限（需求書無上限；一 batch 500 件要分 10 次 `/print_label` ⇒ UI 與後端行為定案）

**定案：軟上限 500 件／批（超過建議拆批，不硬拒）；`/print_label` 的 50 筆是硬上限，由後端自動分組。**

| 層 | 值 | 性質 | 超過時 |
|---|---|---|---|
| `/print_label` 單次 | **50 筆** | 🔴 **provider 硬上限**（§0.1 ⑤） | 後端自動切組，前端不感知 |
| 一個批次 | **500 件** | 我方**軟**上限 | 前端顯示建議拆批的提示，**仍可送出** |
| 一次 B1 請求的 `orderNos[]` | **200 筆** | 我方**硬**上限（請求體大小與交易時長） | 回業務碼拒絕，訊息說明「請分次加入」 |

**500 從哪裡來（推導寫出來，不要背）**：50 筆／組 × 10 組 ≈ `/print_label` p95 15 s × 10 ≈ 2.5 分鐘
（§9 的 NFR 表）。這是「賣家按一次列印、可接受的等待上限」，**不是 provider 的限制**
⇒ 它是可調參數，值改動時只要重估上面那個乘法，不需要跟 provider 談。

**後端行為（B5 端點）**：

```
B5 收到 batch_no
  → C-GRP 依 (goodsType, lgsType, shipType, shipDate) 四參數分群
      ⚠️ 批次的分組鍵已保證四參數一致（§5.1.5 補的兩欄）⇒ 正常情況只會有一群
      🔴 若真的分出 >1 群 ⇒ 該批次的分組鍵被破壞了，回 B2C_LABEL_GROUP_MIXED 並建 case
  → 群內每 50 筆切一組 → 逐組送 /print_label（K68-2，每組一個租約）
  → 逐組結果：整組失敗 ⇒ 整組不改 label_print_status；逐筆失敗 ⇒ 該筆寫 4104 FAILED
  → 回應：非同步 job id，前端輪詢
```

**UI 行為（細規交 65A，本書只定契約）**：

| 情境 | 回應 | 🔴 不得做的事 |
|---|---|---|
| 500 件 → 10 組 | 顯示「10 份標籤，處理中 3/10」 | 不得顯示「失敗」——分組是正常行為不是錯誤 |
| 第 7 組失敗 | 前 6 組可下載、第 7 組可單獨重試 | 🔴 **不得整批重印**——已列印的不能再送 E6 `/update`，重印會產生第二份與包裹上不同的標籤 |
| 全部完成 | 10 份 PDF 各自授權下載（§5.1.13） | 不得合併成一份大 PDF（合併後 SHA-256 對不上單筆的 `label_sha256`） |

⚠️ **這一節與 §9 的 NFR 表是同一組數字的兩種表述**：那裡寫 p95，這裡寫它推導出來的操作上限。
改任一邊都要回頭改另一邊——**兩個地方各寫一個數字而不寫關係，就是下一次漂移的起點**。

### 7.2 🔴 三欄映射表：§21.11 shipment 軸 → 65 normalized → `allowedActions[]`

> **判準（65 §7.1.2.3 逐字推廣）**：兩個 provider 原生值，只要落在 `ShippingStatusRules` 的
> **不同**述詞集合裡，就**不得**映到同一個 normalized 值。本表逐列已套用。
> **值域來源唯一**：65 §7.1.2.1 的 31 值（既有 24 ＋ 新增 7）。**本表只做映射，不新增任何值。**

| # | §21.11 值 | → normalized（65） | 述詞集合 → 訂單軸 | `allowedActions[]`（fail-closed 版） |
|---|---|---|---|---|
| M1 | `READY_TO_CREATE` | `UNPROCESSED(91)` | `isPendingShipment` → 20 | 🔴 **`dispatch`（取號，端點＝`C4 POST /app/shipments/:order_no/dispatch` ＝ 65 `S-10`）／`cancel`（端點＝`C2` ＝ 65 `S-11`）**。<br/>⚠️ **原寫 `cancel`／`resubmit`，`resubmit` 在 `91` 是錯的**，且有兩個獨立依據：① **CodeMeta SSOT** `業務類型碼.md` 的 `RESUBMIT` description 逐字「**只有 990 建單失敗的配送單可重建**」；② §6.1.4 **C3** 的 allowed-from 已收斂為 `990` only ⇒ 照原表施工，**每一筆 `91` 都會長出一顆「重新送出」按鈕，而按下去必回 `9520`**。<br/>🔴 `91` 這一格賣家真正要做的是**取號**（`dispatch`），不是重送 |
| M2 | `PROVIDER_CREATED` | `PROCESSING(92)` | `isPendingShipment` → 20 | `cancel`／`update`〔E6 到位前禁用〕 |
| M3 | `LABEL_READY` | `CVS_CODE_RECEIVED(920)` | `isPendingShipment` → 20 | `print_label`／`cancel`。🔴 **它就是 `LABEL_READY`**，且已是防倒退掛載點（`LOCAL_PROCESSING_DONE`） |
| M4 | `PACKED` | `CVS_CODE_RECEIVED(920)`（**不另造值**） | 同 M3 | `scan_bind`／`assign_batch` |
| M5 | `ASSIGNED_TO_BATCH` | `PROCESSING_ACCEPTED(98)` | **不屬任一述詞 → 不推進訂單** | `remove_from_batch`（僅批次 `OPEN` 時） |
| M6 | `HANDED_TO_CARRIER` | 🆕 `HANDED_OVER(930)` | `isConfirmedShipment` → 30 | `view_only`（交接後賣家不得再改） |
| M7 | `ARRIVED_AT_DC` | **無對應值**（65 明示「到倉待驗不造」） | — | 🔴 **本書刪除此狀態**（§7.4 **S0-2**；S0-1 是 `PACKED`） |
| M8 | `DC_ACCEPTED` | 🆕 `DC_ACCEPTED(931)` | `isConfirmedShipment` → 30 | `view_only` |
| M9 | `IN_TRANSIT_TO_STORE` | `IN_TRANSIT(31)` | `isConfirmedShipment` 之外，C1 → 30 | `view_only` |
| M10 | `AT_STORE` | `WAITING_PICKUP(32)` | C1 → `OrderStatus.WAITING_PICKUP(32)` | `view_only`；逾期 → M-X5 |
| M11 | `PICKED_UP` | `PICKED_UP(11)` | `isTerminal` → 40 | **終態 T1** |
| X1 | `CREATE_UNKNOWN` | 🆕 `CREATE_UNKNOWN(991)` | **不推進**（65 核心不變式） | `requery`（自動）／`admin_resolve` |
| X2 | `CANCEL_PENDING` | 🆕 `CANCEL_PENDING(992)` | **不推進** | `view_only`；逾時 → 兜底端點 |
| X3 | `DC_REJECTED` | 🆕 `DC_REJECTED(932)` | `isAbnormal` → 35 | `open_case`／`arrange_return`／`claim` |
| X4 | `STORE_REDIRECT_REQUIRED` | `STORE_TRANSFER(81)` | `isAbnormal` → 35 | `reselect_store`（期限內） |
| X5 | `UNCLAIMED_RETURN` | `BUYER_NOT_PICKED(52)` | `isAbnormal` → 35 | `view_only` → 自動轉 `53` |
| X6 | `CUSTOMER_RETURN` | `RETURN_GENERAL(51)` | `isAbnormal` → 35 | `accept_return` |
| X7 | `RETURN_AT_DC` | `RETURN_TO_WAREHOUSE(53)` | `isAbnormal` → 35 | `seller_pickup` → 退貨軸 `outcome = COLLECTED`（X8）；逾期未領 → 退貨軸 `outcome = ABANDONED`。🔴 **不推 `55`／`82`**，理由見 §7.3.2 末段 |
| X8 | `RETURN_COLLECTED` | 🔴 **不落 shipment 軸**（§7.3.2 已逐一否決 `55`／`11`／`56`／`82`）⇒ 承載欄是 `shipment_returns.outcome = COLLECTED` | shipment 軸停在 `53` 不動 ⇒ **不推進** | `view_only`（它是退貨軸的終態） |
| X9 | `LOST` | `PACKAGE_LOST(44)` | `isAbnormal` → 35 | `claim` → `993` |
| X10 | `DAMAGED` | `ABNORMAL(33)`〔非 `44`〕 | `isAbnormal` → 35 | `claim` → `993`；🔴 **不映 `44`**（`44` 是遺失事實，毀損是另一件事） |
| X11 | `CLAIM_PENDING` | 🆕 `CLAIM_PENDING(993)` | `isAbnormal` → 35 | `admin_close_case` → `43` 成立／`33` 不成立 |
| X12 | `CLOSED_WITH_ADJUSTMENT` | `COMPENSATION(43)` | `isAbnormal` → 35 | **業務終態 T2** |

🔴 **三個必須明說的「不」**：
1. **不重用 `LOGISTICS_CHECK(22)`**——它適用欄是 `3002`、語意是 C2C 的物流驗收、且屬 `PRE_SHIPMENT`
   （`ShipmentStatusMapper.PRE_SHIPMENT` 逐字含它）⇒ B2C 若映到它，訂單軸**不推進**，
   而 B2C 的驗收正是要推進訂單的關鍵事件。
2. **不合併 `PROCESSING(92)` 與 `TCAT_CALLED(210)`**（65 F.7.1；本表 B2C 根本不觸及 `210`）。
3. **不新增任何值**——M4／M7／X4–X7／X9／X10／X12 全部以既有值承接（X8 不落 shipment 軸）。31 值就是全部。

🔴 **`CREATE_FAILED(990)` 的 `allowedActions`**

`990` **不是** §21.11 的 23 值之一 ⇒ 上表沒有它，而 `allowedActions[]` 是 **fail-closed**（§7.3.5 逐字：「表上沒列的 action 就是不可用，前端**不得**自行 fallback」）
⇒ **一筆 `990` 回給前端的 `allowedActions[]` 會是空的，賣家看不到任何 recovery 入口**，
而 §6.1.4 **C3** 與附錄 D 都設計了「重新送出」按鈕 ⇒ **設計互相矛盾，且是靜默的**（前端不會報錯，只是按鈕不出現）。

| # | 值（65 §7.1.2.1 值域，適用欄＝`全`） | normalized | 述詞集合 → 訂單軸 | `allowedActions[]`（fail-closed 版） |
|---|---|---|---|---|
| **Y1** 🆕 | `CREATE_FAILED(990)` | `CREATE_FAILED(990)`（即自身，不另映） | **不屬任一述詞 → 不推進訂單軸**（06 §2.1 逐字） | `resubmit`（賣家，C3）／`admin_resolve`（Admin 重建→`91`／Admin 取消→`41`＋退款鏈，§7.4） |

⚠️ **Y1 與 §7.2 X1（`991`）的差別是本書最容易被抄錯的一格**：
`990` ＝ **確定失敗** ⇒ 重建是合法的；`991` ＝ **結果未知**（65 §7.1.2.1 語意欄逐字「**不是失敗**」）⇒ **重建被 `INV-3` 明令禁止**，只能 `requery`。
🔴 **兩者的 action 不得互抄**。

🔴 **這一列不新增任何狀態值**——`990` 是 65 §7.1.2.1 的既有值（本書 §7.5 本來就要為它補 `isValidTransition` 分組與出口），本節只是補上它缺的 `allowedActions` 那一格。

🔴 **X 編號以本表與 §7.3.2 為同一套**（X8 ＝ `RETURN_COLLECTED`、X12 ＝ `CLOSED_WITH_ADJUSTMENT`）。
全書其他章節引用 `X8` 的地方（§7.3.7 退貨軸列、§7.10 第 7 條、§11.1 A4、附錄 C.4 ①、§8.2.2 指標登記表等）
都讀這一套；🔴 **改動任一張表的編號時，用 `X8`／`X12` 全文 grep 一次再改**，不要憑印象數有幾處。

### 7.3 batch 軸：不進 normalized delivery 值域

**D-12 決議②逐字**：batch 是**聚合層**，不是訂單的一軸 ⇒ **不映射到 normalized**，也**不進 order read model**。

| batch 值 | 入邊（誰推進） | 出邊（誰推走） | allowed-from 檢查 | 卡住誰知道 |
|---|---|---|---|---|
| `OPEN` | B1 建立批次端點 | B4 關閉端點／截止排程 | `UPDATE ... WHERE status='OPEN'` | aging > 24h |
| `CLOSED` | B4 | B6 handoff 端點 | `WHERE status='CLOSED'` | 🔴 **aging > 12h**（§8.2.2 缺口①：關了沒出車） |
| `DISPATCHED` | B6（同交易寫 `930`） | **聚合推導** | `WHERE status='DISPATCHED'` | 🔴 aging > **48h**（65 已定） |
| `DC_RECEIVED` | 聚合：全部個件皆 `931` 或明確 `932` | `RECONCILED`（相符，**直達邊**）／`RECONCILING`（不符） | 🔴 **直達邊必須在 allowed-from 內**（需求書箭頭圖沒有這條，65 §7.1.1.2 I 已補） | aging > 24h |
| `RECONCILING` | 件數不符 | **差異 case 關閉**（不是 Admin 直接改 state） | `WHERE status='RECONCILING'` | aging > 72h ＋ ADM 佇列 |
| `RECONCILED` | 上兩者 | — | — | 終態 |

🔴 **INV-A1（單向禁令，65 逐字）**：`DC_RECEIVED` **不得回頭推論任何個件狀態**。
守法＝AST gate（batch 聚合函式內不得出現對 shipment 狀態的 UPDATE）。
⚠️ 依 65 §3.10：該 gate 的觀測窗只到函式邊界，**抽 helper 就看不到** ⇒ 這句話要寫進 gate 自己的檔頭。

#### 7.3.1 Shipment 主線 11 值（逐值四問；處置需求書點名的三個狀態：`PACKED`／`ARRIVED_AT_DC` 🔴 **真的無推進者**、`IN_TRANSIT_TO_STORE` ⚠️ **推進者既有**（PayUni 貨態 `31`，§7.4 S0-3）**不屬不可達**）

> 值 → normalized 的映射在 §7.2（三欄映射表），本節不重複；
> 三個不可達狀態的處置在 §7.4（S0-1／S0-2／S0-3），本節不重複裁決。
> **本節只做一件 §7.2 與 §7.4 都沒做的事：把主線的轉換邊逐條寫出來，並逐值答 65 §3.11 四問。**

**Happy path 一行串完**（純線性，不畫圖）：
`91 → 92 → 920 → 98 → 930 → 931 → 31 → 32 → 11`（9 個格子，因為 `PACKED` 併入 `920`、`ARRIVED_AT_DC` 已刪）

**逐值四問**：

| # | §21.11 值 | normalized | ① 誰推進（`檔案:符號名` 或端點） | ② 誰推走 | ③ allowed-from 含不含它 | ④ 卡住誰知道（aging） |
|---|---|---|---|---|---|---|
| 1 | `READY_TO_CREATE` | `91` | 既有建單鏈（訂單付款完成 → 建 `shipments` 列）＋ `ShipmentCreationBackstopScheduler` 兜底（🔴 **兜底只在 `orders.shipping_status IS NULL` 期間有效；`91` 一旦投影回 Order 它即退出**，見 §7.1.2 與 03 §8.11.1） | K68-1 `B2cTradeNoWorker` 取號成功 | ✅ `PRE_SHIPMENT` 含 `91` ⇒ 往任何狀態皆合法 | `hawkr_b2c_pending_trade_aging`（> 24 h） |
| 2 | `PROVIDER_CREATED` | `92` | C-ADPT 在 `/trade` 回 `SUCCESS` 的**同一交易**內（同時寫 `provider_shipments`，§5.1.3） | Print Notify 回填 `Odno` → `920` | ✅ `PRE_SHIPMENT` 含 `92` | `hawkr_b2c_pending_label_aging`（> 24 h，§0.4） |
| 3 | `LABEL_READY` | `920` | `ShippingService.processWebhookPrint`（既有） | B1 加入批次 → `98` | ✅ `LOCAL_PROCESSING_DONE` 含 `920`（既有）⇒ 🔴 已是防倒退掛載點。⚠️ **但 `920 → 98` 現況恆 false**，§7.5「`98` 的歸屬」必補 | 同上 |
| 4 | `PACKED` | `920`（**不獨立造**） | 🔴 **無推進者** ⇒ §7.4 S0-1 判定併入 `920` | — | — | — |
| 5 | `ASSIGNED_TO_BATCH` | `98` | B1 `POST /app/shipment_batches`（同交易寫 `shipment_batch_items`） | B6 交接 → `930` | 🔴 **必改**：`98` 移出 `PRE_SHIPMENT`、進 `LOCAL_PROCESSING_DONE`，並顯式加三條邊（§7.5「`98` 的歸屬」）——留在 `PRE_SHIPMENT` ＝ 已入批的單可被任何 provider 回值覆蓋 | `hawkr_b2c_batch_open_aging_seconds`（batch `OPEN` > 24 h，§8.2.1 #6） |
| 6 | `HANDED_TO_CARRIER` | 🆕 `930` | 🔴 B6 `POST /app/shipment_batches/:batch_no/handoff` ——**唯一推進者**，同交易寫 `batch_handoffs` ＋ batch `DISPATCHED` ＋ outbox | 驗收結果（§6A.3 (b)/(c)）→ `931`／`932` | 🔴 **必補**：`LOCAL_PROCESSING_DONE` 加 `930`（§7.5） | `hawkr_b2c_dispatched_aging`（> 48 h，65 已定） |
| 7 | `ARRIVED_AT_DC` | **無對應值** | 🔴 **無事件無端點** ⇒ §7.4 S0-2 判定**刪除** | — | — | — |
| 8 | `DC_ACCEPTED` | 🆕 `931` | K68-7 importer 或 ADM 人工登錄（§6A.3）；(a) 取得後改由 `ShippingWebhookConsumer` | 貨態 `31` | 🔴 **必補**：`IN_DELIVERY` 加 `931`（§7.5） | `hawkr_b2c_dc_accepted_aging`（> 72 h 未進 `31`） |
| 9 | `IN_TRANSIT_TO_STORE` | `31` | PayUni 貨態 Notify `31`（既有） | 貨態 `32` | ✅ `IN_DELIVERY` 已含 `31` | 既有（`ShipmentStatusReconcileScheduler`） |
| 10 | `AT_STORE` | `32` | PayUni 貨態 Notify `32`（既有） | 貨態 `11`；逾期 → `52`（X5） | ✅ `IN_DELIVERY` 已含 `32` | 既有 ＋ `52` 轉換的到期判定 |
| 11 | `PICKED_UP` | `11` | PayUni 貨態 Notify `11`（既有） | 🔴 **終態，無出邊**（例外見 §7.3.7） | ✅ `TERMINAL` 已含 `11` | 不適用（終態） |

🔴 **第 6 列是本章最重要的一格**：`930` 的推進者是**我方的一個 HTTP 端點**，不是 provider 事件。
⇒ 賣家不按那顆按鈕，批次永遠停在 `CLOSED`，而 provider 端**什麼都不知道**（provider 沒有 batch 概念，§4.1）。
這就是 §8.2.2 缺口①（`CLOSED` 卡住 12 h）存在的理由——沒有那條告警，一批已封箱的包裹可以無聲地放到過期。

⚠️ **第 8 列的 ① 在 v1.4 期間是人或檔案，不是系統**：依 65 §3.11 第②問的判準
——答案是「掃描器／ADM／人工」時**必須去查那個東西存不存在**。
本書的答案：importer 規格在 §6.3.5（🔴 檔案格式未取得）、
ADM 端點 ＝ **`POST /admin/shipments/:id/dc_result`**（規格在 §7.6.1：maker-checker ＋ audit 六欄；
§6.1.4 端點表列為 **E5**）。
**兩者都不到位之前，第 8 列的 ① 是空的，而 §6A.3 第 3 點已明訂：那時 `931`／`932` 不得出現在任何 shipment 上。**

#### 7.3.2 Shipment 例外分支 12 值（逐值入邊／出邊／allowed-from／aging）

##### 🔴 第 12 個值：`RETURN_COLLECTED`

需求書 §21.11「例外分支」那一句的逗號清單逐字為 12 項，第 8 項是 **`RETURN_COLLECTED`**；
需求書 §21.15 的 16 個事件裡對應的是 `ReturnCollected`（§6.4.1 第 13 列）。
🔴 **它的編號是 `X8` 不是 `X12`**（`X12` 是 `CLOSED_WITH_ADJUSTMENT`），§7.2 與本節共用同一套編號。

🔴 **但它關掉一個 open item、打開一個更難的問題**：**normalized 值域裡沒有它的位置。**

| 候選 | 為什麼不行 |
|---|---|
| `55 SELLER_NOT_PICKED` | 🔴 語意**相反**：CodeMeta 逐字「買家未取包裹配達退貨門市後，**商店未於期限內領取**（僅限超商 C2C）」。`RETURN_COLLECTED` 是**已領取** |
| `11 PICKED_UP` | 🔴 語意逐字「**消費者**已取貨」。用它會讓買家端的訂單顯示成「已送達」，而實際上包裹退回了 |
| `56` ／ `82` | CodeMeta 逐字皆標「僅限超商 **C2C**」 |
| 新增一個值 | 🔴 **本書不得新增**（§7.2 逐字「本表只做映射，不新增任何值」；值域 SSOT 在 65 §7.1.2.1） |

**定案（依 65 D-5「各軸獨立 enum」）：`RETURN_COLLECTED` 不落 shipment 軸，落退貨軸。**
承載欄是 §5.1.9 的 `shipment_returns.outcome = COLLECTED` ＋ `custody = SELLER` ＋ `closed_at`。

🔴 **代價要寫出來，不能只寫結論**：shipment 軸因此**停在 `53 RETURN_TO_WAREHOUSE` 不動**
⇒ 65 §3.11 第④問的 aging **不能錨在 `shipments.status`**（那會對每一筆已領回的退貨永久告警），
必須錨在 `shipment_returns.outcome IS NULL`。
⇒ 這一格要回報給 65：值域擁有者是它，若日後決定補一個 normalized 值，
本書的映射表（§7.2 **X8** 列）與這裡要同批改。

##### 🔴 `55`／`82` 不是 `3004` 的出邊（三方衝突，本節裁決）

上表用「僅限超商 C2C」否決了 `56`／`82`，**而同樣的理由對 `55` 一字不差地成立**
——CodeMeta 逐字「賣家未取…（僅限超商 **C2C**）」。三方的說法目前是：

| 來源 | 對 `53` 在 `3004` 上的出邊怎麼寫 |
|---|---|
| 65 §7.1.2.1 `53` 列（適用 `3001／3002／3004`） | `→ 55 逾期未領／82／11 賣家領回` |
| 65 §7.1.2.1 `55`／`82` 列 | **適用欄各自只有 `3002`** |
| CodeMeta `ShippingStatus` 的 `55`／`56`／`82` | 描述逐字「僅限超商 C2C」 |

⇒ **65 自己的兩列互相矛盾**，而 65 §7.1.2 的紅字明訂
「**SSOT 是 §7.1.2.1 的『適用』欄**」⇒ 依它裁決：**`3004` 上不得推 `55`／`82`**。
`11` 同樣不行——它的語意是「**消費者**已取貨」（§7.3.2 開頭的候選表已否決）。

**本書定案**：`3004` 的 `53` 在 shipment 軸上**沒有出邊**，
領回與逾期未領都由退貨軸的 `shipment_returns.outcome`（`COLLECTED`／`ABANDONED`）承載。
🔴 **代價**：`53` 會永久留在 `shipments.status` ⇒ 任何錨在 `shipments.status='53'` 的 aging
都會對每一筆已結案的退貨永久告警 ⇒ 指標一律錨 `shipment_returns.outcome IS NULL`（§8.2.1 #11）。

🔴 **要回報 65 的一格**：`53` 列的「誰把它推走」欄對 `3004` 是空的，
而 65 §7.1.2.1 對每一列都要求答得出第②問 ⇒ 65 需補一句「`3004` 走退貨軸」，
或補一個 normalized 值。🔴 **定案（65 §7.1.2.0b `M-2`）**：`3004` 上不得推 `55`／`82`；
`53` 列的出邊欄**依 provider 分列**（`3002` 才有 `55`／`82`）；`RETURN_COLLECTED` 落
`shipment_returns.outcome = COLLECTED`，**不新增第 32 個值**。
⚠️ `M-2` **會改動既有 C2C 路徑的共用出邊表** ⇒ `3002` 的 `53 → 55`／`53 → 82` 必須在同一個 PR 內有回歸測試坐實**沒有被這次分列改掉**。

##### 例外分支逐值：入邊、出邊、四問

| # | 值 | normalized | ① 誰推進（入邊） | ② 誰推走（出邊） | ③ allowed-from | ④ aging |
|---|---|---|---|---|---|---|
| X1 | `CREATE_UNKNOWN` | 🆕 `991` | C-ADPT 的 `/trade` 逾時／未知回應 | K68-3 回查：成功→`92`／確定失敗→`990`／超窗→`41`＋人工 | `991` 屬 `PRE_SHIPMENT` ⇒ blanket allow（回查查到答案後要寫得回 `92`／`990`）。<br/>🔴 **不得為 `991` 另加「顯式列舉分支」**：一旦加，它必須排在 `PRE_SHIPMENT` blanket 之前**且列舉完整**，漏一個值就會讓 `3004` 的回查成功被拒。⚠️ **66 §7.7 四問④** 與 **CodeMeta `業務狀態碼.md` 的 `CREATE_UNKNOWN` 描述**都帶 Uber-scoped 的列舉——**對 `3003` 正確，不得當成 provider-neutral 全集照抄**；本書 §7.6.1 case 表同理。⇒ **加分支前先對 65 §7.1.2.1 的 `991` 列** | `hawkr_b2c_create_unknown_aging`（> 180 min，§0.4） |
| X2 | `CANCEL_PENDING` | 🆕 `992` | C2 `POST /app/shipments/:order_no/cancel` | 主要＝貨態 Notify；兜底＝K68-4 | 🔴 **必補**（同上） | `hawkr_b2c_cancel_pending_aging`（門檻待 RFI-7） |
| X3 | `DC_REJECTED` | 🆕 `932` | §6A.3 的 (b) importer／(c) ADM | 驗退 case 關閉 → `53`／`993` | 🔴 **必補**；`932` 進 `isAbnormal` | `hawkr_b2c_dc_rejected_open_cases` |
| X4 | `STORE_REDIRECT_REQUIRED` | `81` | 貨態 Notify `81`（既有） | 重選門市 → `31`／`32`；逾期 → `52`／`53` | ✅ `isAbnormal` 已含 `81` | 既有；🔴 重選期限由 provider 決定，我方只顯示不自己算 |
| X5 | `UNCLAIMED_RETURN` | `52` | 貨態 Notify `52`（既有） | → `53` | ✅ | 既有 |
| X6 | `CUSTOMER_RETURN` | `51` | 售後（09 號）核准退貨 → `51` | → `53`；🔴 冷凍走 §7.7.1 | ✅ | 既有 |
| X7 | `RETURN_AT_DC` | `53` | 貨態 Notify／驗退 case | 領回 → **退貨軸** `outcome = COLLECTED`（X8）；逾期未領 → **退貨軸** `outcome = ABANDONED`。🔴 **不推 `55`／`82`／`11`**（見下方「`55`／`82` 不是 `3004` 的出邊」） | ✅ | 🔴 錨在 `shipment_returns.outcome IS NULL`，**不錨在 `shipments.status`** |
| X8 | **`RETURN_COLLECTED`** | 🔴 **不落 shipment 軸** | 賣家至物流中心領回 ⇒ ADM／賣家登錄 → `shipment_returns.outcome = COLLECTED` | 🔴 **它就是退貨軸的終態** | 退貨軸自己的 allowed-from（`outcome IS NULL` ⇒ `COLLECTED`） | 同 X7 |
| X9 | `LOST` | `44` | 貨態 `44`；或 `32` 卡 N 天偵測 | → `993` → `43` | ✅ | 既有 |
| X10 | `DAMAGED` | `33`〔非 `44`〕 | 驗退／貨態 → `33` | Admin 處置 → `31`／`41`／`993` | ✅（`shipment_anomaly_acknowledgments` 是現成載體） | 既有 |
| X11 | `CLAIM_PENDING` | 🆕 `993` | `44`／`932`／`33` 轉入；ADM 建案 | claim case 關閉 → `43`（成立）／`33`（不成立） | 🔴 **必補** | `hawkr_b2c_claim_pending_aging` |
| X12 | `CLOSED_WITH_ADJUSTMENT` | `43` | claim 裁定成立 | 🔴 **業務終態，無出邊**（§7.3.7 T2） | ✅ | 不適用 |

🔴 **「必補」的四格（X1／X2／X3／X11）＋ 主線的 `930`／`931`／`98` 是同一件事的七個面**：
`ShipmentStatusMapper` 的四個集合若不安排它們，`isValidTransition` 會讓它們落到最後的
`log.warn + return false` ⇒ **一次寫不進去、一行 warn、沒有任何東西轉紅**（§7.5 逐字）。
既有的 `990 CREATE_FAILED` 就是這樣變成死狀態的（65 D-19 群 A 實測）。
⇒ **照抄 `990` 的形狀即繼承它的死法**，本書必須同批補 `990` 的出口（§7.4 末段）。

⚠️ **X4 的重選期限不要自己算**：門市關轉的重選窗是 provider 的規則，
我方只顯示 provider 給的期限；自己算一個看起來合理的天數，會在 provider 改規則那天靜默錯開。

#### 7.3.3 🔴 **Batch 狀態機的收斂**（本書必答）
| 來源 | 值 |
|---|---|
| §21.11（權威狀態機） | `OPEN`／`CLOSED`／`DISPATCHED`／`DC_RECEIVED`／`RECONCILING`／`RECONCILED`（6 個英文值） |
| §21.16 行 2191（內部批次頁） | 草稿／檢核中／建立中／部分完成／待交倉／已交倉／失效／對帳異常（**8 個中文值**） |
⇒ 兩套並存且無映射。**本節裁決**：唯一值域 ＝ §21.11 那 6 個英文值（**領域態**，DB CHECK 是 65 §5.2.2 `chk_batch_status`）；
§21.16 那 8 個中文值是 **UI 態**，後端**不得回傳**（同 `LOADING`，§0.3）。
🔴 **兩套的逐值對照表在 §14.3「batch 的兩套」**，本節不重複（重複必然漂移）。
**分層一句話**：領域態是狀態機的節點、有 allowed-from 與告警；UI 態只是前端把幾個領域態＋本地表單進度合併顯示，
**它沒有後端落點，也不得有**——一旦後端開始回傳「檢核中」，就會有人拿它當 allowed-from。
#### 7.3.4 17 個 S0 的逐一處置：判準與分群

✅ **處置表本體在 §7.4**（S0-1～S0-17 逐列），本節只留**判準**，不重複那張表。

**表頭固定五欄**：`狀態｜①誰推進｜②誰推走｜③推走那句的 allowed-from 含不含它｜④卡住誰會知道（aging 指標名）`。
**處置只有三種**：**補齊執行者**／**補齊轉換邊**／**從狀態機刪除**（①或②答不出即刪，不得留著）。
🔴 **第③欄是本書最容易被跳過的一格**：①②都答得出、③沒查，結果就是「有人推、有人接、但那句 SQL 或 `isValidTransition` 走不到」
——rows=0 或 `return false`，而呼叫端多半把它讀成「別人先做過了」。

**分群（17 個的來源）**：
| 群 | 狀態 |
|---|---|
| 主線（3） | `PACKED`／`ARRIVED_AT_DC` 🔴 **真的無推進者**；`IN_TRANSIT_TO_STORE` ⚠️ **推進者既有**（PayUni 貨態 `31`）**不屬不可達**，列在這裡只因需求書把它一起點名了（§7.4 S0-3） |
| 例外零邊（10） | `CREATE_UNKNOWN`／`CANCEL_PENDING`／`DC_REJECTED`／`STORE_REDIRECT_REQUIRED`／`UNCLAIMED_RETURN`／`CUSTOMER_RETURN`／`RETURN_AT_DC`／`LOST`／`DAMAGED`／`CLAIM_PENDING` |
| batch 壞 4 | `DISPATCHED`／`DC_RECEIVED`／`RECONCILING`／`RECONCILED` ⇒ **批次永遠結不掉** |
- `CREATE_UNKNOWN` 🔴 **與既有生產中的 `CREATE_FAILED(990)` 同形**（P1-A 四條證據證明那個完全沒有出口）⇒ **照抄即繼承死狀態**
- batch 四個依 **D-11** 收斂：聚合推導 ＋ 單向禁令保留 ＋ **補 `DC_RECEIVED → RECONCILED` 直達邊** ＋ `RECONCILING` 由差異 case 關閉驅動（**不是 Admin 直接改 state**）
#### 7.3.5 三欄映射表：§21.11 值 → 65 normalized 值 → `allowedActions[]`

三欄映射表見 **§7.2**（M1–M11 ＋ X1–X12 逐列）。本節只記兩件 §7.2 沒說的：
① 「normalized 缺 `AT_STORE`／逾期未取」不成立——65 §7.1.2.1 的 `WAITING_PICKUP(32)` 逐字就是 `AT_STORE`、
`BUYER_NOT_PICKED(52)` 逐字就是「逾期未取」（§7.2 M10／X5）⇒ **不再依賴 65 骨幹擴充**。
② `allowedActions[]` 一律 **fail-closed**：表上沒列的 action 就是不可用，前端**不得**自行 fallback 成 `view_only` 以外的東西。
#### 7.3.6 買家可見 6 階段 ↔ 主線 11 值的映射

六階段逐列對照見 **§7.7**。本節只保留那條**不得違反的**：
🔴 **不得把 seller 出車（`930 HANDED_OVER`）顯示為「物流中心已收件」**——
`930` 是**我方端點**寫的（B6，§7.3.1 第 6 列），provider 那一刻什麼都還不知道；
「物流中心驗收」對應的是 `931 DC_ACCEPTED`，而它的入邊在 v1.4 期間可能根本還沒到位（§6A.3 第 3 點）。
把兩者混成一格 ⇒ 買家看到「已進倉」而包裹其實還在賣家的車上，**這是對買家說了一件沒發生的事**。
禁詞清單見 §14.1。
#### 7.3.7 終態清單（逐軸明列；`PICKED_UP` 是不是終態需求書沒寫，且 `PICKED_UP → CUSTOMER_RETURN` 邊存在但未明寫）

**三軸各自的終態（65 D-5：各軸獨立 enum，不共用值域，終態也不共用）**：

| 軸 | 終態 | 依據 | 出邊 |
|---|---|---|---|
| **shipment**（normalized） | `11 PICKED_UP`、`41 CANCELLED` | 🔴 **repo 實測**：`ShipmentStatusMapper.TERMINAL = Set.of(PICKED_UP, CANCELLED)`，且 `isValidTransition` 第二個分支 `if (TERMINAL.contains(from)) return false` | 無 |
| shipment（業務終態，非 `TERMINAL` 集合成員） | `43 COMPENSATION`（X12） | 理賠裁定成立後不再變動 | ⚠️ **技術上仍可轉出**（`isAbnormal → isAbnormal` 合法）⇒ 只有業務約定，沒有機制 |
| **batch** | `RECONCILED` | 65 §5.2.2 `chk_batch_status`；§7.3 表最後一列 | 無 |
| **退貨** | `shipment_returns.outcome ∈ {COLLECTED, ABANDONED, COMPENSATED}` | §5.1.9 ＋ §7.3.2 X8 | 無 |
| **理賠** | `shipment_claims.outcome ∈ {APPROVED, REJECTED, WITHDRAWN}` ＋ `closed_at` | §5.1.9 | 無 |

##### 🔴 `PICKED_UP` 是終態，而 `PICKED_UP → CUSTOMER_RETURN` 這條邊**在 shipment 軸上不存在**

需求書把 `CUSTOMER_RETURN` 列在例外分支裡，讀起來像是 `PICKED_UP` 之後還能轉出去。**不是。**

| 事實 | 證據 |
|---|---|
| `11` 在 `TERMINAL` 集合內 | `ShipmentStatusMapper.TERMINAL`（實測） |
| 終態不可再轉移 | `isValidTransition`：`if (TERMINAL.contains(from)) return false`（實測） |
| ⇒ `11 → 51` 一律回 false | 白盒推理，由上兩條推得 |

**這不是缺陷，是正確的建模**：買家取貨後才申請退貨，**那是一筆新的逆向物流**，
不是原本那張物流單改狀態。落地形式：

```
原 shipment（11 PICKED_UP，終態，不動）
      │
      └─ 售後（09 號）核准退貨
             └─▶ 新建一筆 shipments 列，is_return = TRUE（欄位既有）
                     └─▶ 該列自己的狀態機從 51 RETURN_GENERAL 起走
```

🔴 **兩個必須寫出來的後果**：
1. **`is_return = TRUE` 的那一筆不走 B2C 大宗寄倉**——7-11 B2C 常溫的逆向是
   「同時開通的 C2B 退貨便」（§0.1 ⑪），**不是我方開的、也不是同一個服務產品**
   ⇒ 它的 `shipping_method` 不是 `3004`。把它建成 `3004` 會讓 §5.4 的
   `chk_method_lgs_type` 與批次分組鍵一起錯亂。
2. **冷凍沒有這條路**（§0.1 ⑪ 逐字「僅常溫」）⇒ 見 §7.7.1。

⚠️ **終態的定義要看機制不看語感**：`43 COMPENSATION` 讀起來像結案，但它**不在** `TERMINAL` 集合裡
⇒ 技術上還能轉到別的 `isAbnormal` 值。這一格是「業務約定 ≠ 機制保證」的實例，
要嘛接受它（並在 §8.2 加一條「`43` 之後仍發生轉換」的告警），
要嘛把它加進 `TERMINAL`——🔴 **而那是改既有 C2C 行為，屬 65 的值域決定，不是本書能單方面做的**。

#### 7.3.8 🔴 孤兒狀態歸位：`PROVIDER_REVIEW_REQUIRED`、`FULFILLMENT_SETUP_FAILED`

**兩個名字在需求書裡各出現一次、不屬任何狀態機，且全書其他章節零引用。**
依 §7.3.4 的處置三選一（補執行者／補轉換邊／刪除），逐一裁決：

| 名字 | 需求書出處 | 它想表達的事實**已經有**落點嗎 | 裁決 |
|---|---|---|---|
| `PROVIDER_REVIEW_REQUIRED` | 需求書 §21.6（onboarding） | ✅ 有：`onboarding_status = 3601 待審核`（§0.3 前端態 `PENDING_REVIEW`）＋ provider `SetStatus=8 已上傳審核中`（§14.3） | 🔴 **不造這個值**。它是 onboarding 軸既有狀態的別名，造出來就是同一個事實有兩個表 |
| `FULFILLMENT_SETUP_FAILED` | 需求書 §21.8（訂單層） | ⚠️ **部分**：訂單層目前沒有「履約設定失敗」的值 | 🔴 **不造 shipment 值，也不造訂單值**：這個事實由 `disabledReasonCode`（§6.1.7 12 值）在 **checkout preview** 就攔下 ⇒ **它根本不該長到訂單層**。真的漏到訂單層 ⇒ 那是 `991`／`990` 的殼（§7.4 S0-4），不是第三個值 |

🔴 **與 `CREATE_UNKNOWN(991)` 的先後與併存（需求書要求本節定義）**：

```
checkout preview ──▶ C-GATE 三態（§0.2）
                      ├─ BLOCKED(reasonCode) ─▶ 該配送方式不出現 ⇒ 訂單根本建不出來
                      │                          （＝ FULFILLMENT_SETUP_FAILED 的正解）
                      └─ ELIGIBLE ─▶ 建單 ─▶ /trade
                                              ├─ SUCCESS ─▶ 92
                                              └─ 逾時／未知 ─▶ 991 CREATE_UNKNOWN
```

⇒ **兩者互斥且有嚴格先後**：gate 在**建單之前**、`991` 在**建單之後**，
沒有任何時點兩者會同時成立。**併存規則因此是「不得併存」**——
若實作上出現一筆 `991` 的 shipment 而它的賣家 gate 是 `BLOCKED`，那是 gate 被繞過（§0.2 收斂點），
屬 §8.2.1 G1／G2 等級的事故，不是一個要建模的狀態。
### 7.4 🔴 17 個 S0 的逐一處置

**表頭固定五欄**；處置只有三種：**補執行者**／**補轉換邊**／**刪除**（①或②答不出即刪）。

| # | 狀態 | ① 誰推進 | ② 誰推走 | ③ allowed-from 含不含它 | ④ 卡住誰知道（aging 指標） |
|---|---|---|---|---|---|
| S0-1 | `PACKED` | 🔴 無 ⇒ **不獨立造**，併入 `920`（M4） | — | — | — |
| S0-2 | `ARRIVED_AT_DC` | 🔴 無事件無端點（65 實測） ⇒ **刪除** | — | — | — |
| S0-3 | `IN_TRANSIT_TO_STORE` | PayUni 貨態 `31` | 貨態 `32` | `IN_DELIVERY` 集合已含 `31`／`32` | 既有 |
| S0-4 | `CREATE_UNKNOWN(991)` | E2 逾時（**補**） | 回查排程 `ShipmentCreateUnknownRecheckScheduler`（65 K-08，**本書新增**） | `991` 屬 `PRE_SHIPMENT` ⇒ blanket allow；🔴 不得另加顯式列舉分支，理由見 §7.3.2 **X1** | 🔴 `hawkr_b2c_create_unknown_aging`（§8.2.2 缺口④） |
| S0-5 | `CANCEL_PENDING(992)` | C2 端點（**補**） | 主要＝貨態 Notify；**兜底＝`CancelPendingTimeoutScheduler`**（65 K-09） | 同上必補 | 逾時門檻＝provider 回覆時間 ×2（🔴 **值待 RFI-7**，65 P-4） |
| S0-6 | `DC_REJECTED(932)` | §6A.3 (b)/(c) | 驗退 case 關閉 → `53`／`993`（🔴 **不含 `55`**，§7.3.2 末段） | 必補 | `hawkr_b2c_dc_rejected_open_cases` |
| S0-7 | `STORE_REDIRECT_REQUIRED` | 貨態 → `81` | 重選門市 → `31`／`32`；逾期 → `52`／`53` | `isAbnormal` 已含 `81` ✅ | 既有 |
| S0-8 | `UNCLAIMED_RETURN` | 貨態 → `52` | → `53` | ✅ | 既有 |
| S0-9 | `CUSTOMER_RETURN` | 售後（09 號）→ `51` | → `53`／`11` | ✅ | 既有 |
| S0-10 | `RETURN_AT_DC` | 貨態 → `53` | 🔴 **shipment 軸無出邊**（停在 `53`）；領回／逾期未領一律走退貨軸 `shipment_returns.outcome`（§7.3.2 X7／X8）。**不推 `55`／`82`／`11`** | ✅（`isAbnormal` 含 `53`） | 🔴 錨 `shipment_returns.outcome IS NULL`，**不錨 `shipments.status`**（§8.2.1 #11） |
| S0-11 | `LOST` | 貨態 → `44`；或 `32` 卡 N 天偵測 | → `993` → `43` | ✅ | 既有 |
| S0-12 | `DAMAGED` | 驗退／貨態 → `33` | Admin 處置 → `31`／`41`／`993` | ✅（`shipment_anomaly_acknowledgments` 是現成載體） | 既有 |
| S0-13 | `CLAIM_PENDING(993)` | `44`／`932`／`33` 轉入；ADM 建案 | claim case 關閉 → `43`／`33` | 🔴 必補 | 🔴 `hawkr_b2c_claim_pending_aging` |
| S0-14 | batch `DISPATCHED` | B6 端點（**補**） | 聚合推導 | `WHERE status='DISPATCHED'` | 🔴 48h（缺口②） |
| S0-15 | batch `DC_RECEIVED` | 聚合 | 直達邊／`RECONCILING` | 🔴 **直達邊補進 allowed-from** | 24h |
| S0-16 | batch `RECONCILING` | 件數不符 | 差異 case 關閉 | ✅（補後） | 🔴 72h（缺口③） |
| S0-17 | batch `RECONCILED` | 上兩者 | — | — | 終態 |

🔴 **`CREATE_UNKNOWN` 與既有 `CREATE_FAILED(990)` 同形**：`990` 在 `ShipmentStatusMapper.isValidTransition`
裡**落在四個集合之外** ⇒ 走到最後的 `log.warn` ＋ `return false` ⇒ **完全沒有出口**（65 D-19 群 A 實測）。
**照抄即繼承死狀態**。⇒ 本書必須同批補 `990` 的出口（Admin 重建→`91`／Admin 取消→`41`＋退款鏈），
否則 `991` 回查判定為「確定失敗」後會落進一個一樣死的格子。

### 7.5 🔴 `ShipmentStatusMapper` 的集合歸屬（必改，漏了就是靜默失效）

**四集合的歸屬**（`services/java/shipping-service/.../service/ShipmentStatusMapper.java`）：

| 集合 | 🔴 B2C 相關的成員與不變式 |
|---|---|
| `PRE_SHIPMENT` | `91`／`92`／`21`／`22`／`991`。🔴 **`98` 不屬本集合**（見下方「`98` 的歸屬」） |
| `LOCAL_PROCESSING_DONE` | `210`／`920`／`921`／`98`／`933`。🔴 不變式：**本地進度不得被 provider 回的 `91`／`92`／`21`／`22` 倒退覆蓋** |
| `IN_DELIVERY` | `31`／`32`／`930`／`931`。🔴 交接（`930`）與驗收通過（`931`）後仍在配送鏈上，且要能往 `31`／`32` 前進；同樣不得被出貨前狀態倒退覆蓋 |
| `TERMINAL` | `11`／`41` |
| 四者皆不屬 | `990`；🔴 `932`／`991`／`992`／`993` 若不安排，**四個一起變死狀態** |

**處置**：`932`／`993` 進 `ShippingStatusRules.isAbnormal`（65 §7.1.2.3 表已列）；
`991`／`992` 刻意**不進任何述詞集合**（不推進訂單），但 `isValidTransition` 必須為它們**顯式**加分支，
不能讓它們落到 `log.warn + return false`。

🔴 **`98` 的歸屬：本節唯一沒有現成答案的一格（必答，不得沿用現況）**

| 事實 | 證據 |
|---|---|
| 主線 happy path 有 `920 → 98` 這條邊 | §7.3.1 第 3 列②（B1 加入批次 → `98`）與第 5 列① |
| 🔴 現行 `isValidTransition(920, 98)` **恆 false** | `ShipmentStatusMapper.isValidTransition`：`from ∈ LOCAL_PROCESSING_DONE` 時只回 `TERMINAL(to) ∨ isAbnormal(to) ∨ IN_DELIVERY(to)`，而 `98 PROCESSING_ACCEPTED` 三者皆非（`ShippingStatusRules.isAbnormal` 的 11 個成員不含它） |
| 本書自己在兩處把 `98` 列為「不得倒退覆蓋」的來源 | §6.1.3 E4「不得覆蓋本地進度」列、本節 `LOCAL_PROCESSING_DONE` 列 |
| 在 B2C，`98` 的寫者是**我方端點** | §7.3.1 第 5 列① ＝ B1；而 65 §7.1.2.1 的 `98` 列寫的是 PayUni webhook ⇒ **同一個值兩個寫者** |
| 移動 `98` **不影響 C2C** | 65 §7.1.2.1 的 `98` 列「適用」欄只有 `3004`；CodeMeta 描述逐字「僅限超商 B2C」 |

🔴 **定案（65 §7.1.2.0b `M-1`）**：`98` 屬 `LOCAL_PROCESSING_DONE`（不屬 `PRE_SHIPMENT`），
並**顯式**加三條邊：`920 → 98`、`98 → 930`、`98 → 920`（僅批次 `OPEN` 時）。
🔴 前兩條是集合**內部**轉移，不顯式列出就會落到 blanket `return` 而被拒。
⇒ **集合歸屬與三條邊兩書一致，下表不再是「本書單方裁決」，而是 65 已定案的施工清單。**
⚠️ 65 對 `M-1` 附了同一個條件：這是**共用 mapper 集合**的改動，**C2C 路徑要一併回歸驗證**——與本節末「必須在同一個 PR 內附一支測試坐實 `3002` 從不經過 `98`」是同一項要求，不是兩項。

**定案（三條一起做，缺一即靜默失效）**：

| # | 動作 | 沒做的後果 |
|---|---|---|
| ① | `98` 從 `PRE_SHIPMENT` **移到** `LOCAL_PROCESSING_DONE` | `PRE_SHIPMENT.contains(from) ⇒ return true` ⇒ 一則遲到的 `92` 就能把**已入批**的 shipment 推回批次外，而 batch 側依 INV-A1 不得反推個件 ⇒ **沒有任何東西會知道** |
| ② | 顯式加前進邊 `920 → 98`、`98 → 930` | 四集合模型**表達不了集合內的順序**：`920`／`98`／`930` 三個都在 `LOCAL_PROCESSING_DONE`，而該分支不允許轉到同集合成員 ⇒ 主線第 3→5 格與第 5→6 格**兩條都是 false** |
| ③ | 顯式加回退邊 `98 → 920`，**且只在批次 `OPEN` 時**允許 | §7.2 M5 的 `allowedActions[]` 有 `remove_from_batch`（僅批次 `OPEN`）。不加 ⇒ 那顆按鈕按下去一律失敗；不加條件 ⇒ 已交接的單可以被移出批次 |

⚠️ **② 是本節真正的發現，不是 ① 的附屬**：只做 ① 會讓 `98 → 930` **從 true 變成 false**
（現況它靠 `98 ∈ PRE_SHIPMENT ⇒ return true` 才成立）——**修一半比不修更糟**，
因為主線會在更後面一格才斷，而那時已經有一批包裹交接出去了。

🔴 **副作用要寫出來**：做完 ① 之後 `98 → 920` 以外的所有「回到出貨前」都變成 false
（含 65 §7.1.2.1 `98` 列寫的 `98 → 22`）。`22 LOGISTICS_CHECK` 是 C2C 語意、
本書明令不重用（§7.2 三個「不」之一）⇒ 對 `3004` 沒有損失；
**但這是改既有 mapper 的共用行為，必須在同一個 PR 內附一支測試坐實 `3002` 從不經過 `98`**，
坐實不了就退回「在 mapper 內以 `shipping_method` 分流」，**不得默默照改**。

🔴 **鑑別力驗證（§13.6）**：造一筆走到 `920` 的 `3004` shipment，呼叫 B1 加入批次 ⇒ 斷言狀態變 `98`；
再呼叫 B6 ⇒ 斷言變 `930`。**改 mapper 之前這兩支都必須是紅的**——
不紅就代表 B1／B6 根本沒經過 `isValidTransition`，那是另一個問題
（§7.9 併發表第 3 列要求「單列 CAS ＋ `isValidTransition`」），要先回頭確認再往下做。

🔴 **鑑別力驗證（§13.7）**：拿掉 `930` 的 `LOCAL_PROCESSING_DONE` 成員 ⇒ 必須有一支測試轉紅。
沒轉紅就三選一回報（測試弱／變異無鑑別力／預期錯），不准只寫「沒轉紅」。

### 7.6 🔴 `/trade` 取號時點：本書定案

需求書兩節相反：需求書 §21.10 說「成批**前**逐筆取號」、§21.16 說「建立批次**時**逐筆 `/trade` 可部分成功」。

**定案：成批前逐筆取號（採 需求書 §21.10）**，理由三條：
1. `/print_label` 需要 `ShipTradeNo`，而標籤要在**包裝前**貼 ⇒ 取號必須早於成批。
2. 「建立批次時取號可部分成功」會讓 `POST /app/shipment_batches` 變成一個**同時做兩件事且可部分失敗**的端點
   ——部分失敗的那幾筆會停在一個沒有人推走的中間態（正是 65 §3.11 第②問答不出來的形狀）。
3. 取號失敗要能**單筆重試**（C3 `resubmit`），綁在批次建立裡就只能整批重來。

⇒ **`ASSIGNED_TO_BATCH`(M5) 的前置條件是 `PROVIDER_CREATED`(M2) 或更後面**；
批次建立端點對「尚未取號」的訂單一律回 `9520`，**不代為取號**。

#### 7.6.1 🔴 「人工／查證佇列」的承載物（需求書只有名字：無表、無端點、無事件 ⇒ 本書補）

**定案：不建新表、不建新佇列基礎設施。承載物就是 §5.1.10 的 `reconciliation_cases`。**

理由：需求書 §21.9 只寫了「必須進人工／查證佇列」六個字，沒有表、沒有端點、沒有事件。
造一個獨立的 queue 表 ⇒ 又一個要問「誰推進／誰推走／誰會知道」的東西，
而它要做的事（一筆待人處理的差異、有 SLA、有負責人、有 blocked_by）
`reconciliation_cases` 逐欄都已經有了。
🔴 **並且它不得是常駐掃描器**——case 由對帳批次（K68-8，每日一輪、走 **lease**，🔴 **不是 level-triggered**：§12.7 實測部署窗口有兩個 pod ⇒ 單一寫者不成立）
或人工建立，符合《後端服務責任邊界規範》（§5.1.10 逐字）。

**進佇列的四個來源（本書全部的「答案是人工」的格子，一次列完）**：

| 來源 | `case_type` | 誰建 | 誰關 | SLA（`sla_due_at`） |
|---|---|---|---|---|
| `/setting` 逾時或結果未知（§6.1.3 E1、§3.4） | `ONBOARD_UNKNOWN` | C-ADPT 在逾時當下 | Admin 憑 E7 拉到的狀態關閉 | 1 個工作日（🔴 建店不可重送，拖越久賣家越不知道自己在等什麼） |
| `991` 超出回查窗（§7.4 S0-4） | `CREATE_UNKNOWN_EXPIRED` | K68-3 | 🔴 **有端點承接的出口只有「取消（→`41`＋退款鏈）」。** <br/>⚠️ **「Admin 重建（→`91`）」沒有任何端點接得住**：65 `A-6` 的 allowed-from 是 `990`（非 `990` 回 `9525`，65 §6.4.3 檢核表逐字）；65 `S-10 dispatch` **明文不得含 `991`**（65 §6.3.4 A）；68 `C3 resubmit` 對 `991` 回 `9520`（§6.1.4a B）。<br/>🔴 **若要保留該選項，合法形狀是兩跳**：先由 Admin 處置把 `991` 收斂成 `990`（`991 → 990` 合法：`991 ∈ PRE_SHIPMENT`），再走 `A-6`（`990 → 91`）。**但「誰把 `991` 寫成 `990`」在 65 的 allowed-from 中無對應邊** ⇒ **本書不新增端點，該缺口回 65 補**。<br/>⚠️ 本列也是「`991` 出邊列舉不含 `92`」的一處 | 4 h |
| `BackSetShipping` 逾 aging 未收到（§6A.6） | `BACKSET_MISSING` | K68-6 | **E7** ADM 人工登錄端點（§6.1.4） | 1 個工作日 |
| 對帳各層任一差異（§7.8） | `RECON_L0`～`RECON_L4`（L0 ＝ 子商店開通狀態，§7.8 逐字） | K68-8 | 差異消除後由 **E6 關案端點**驅動（🔴 **不是 Admin 直接改 state**） | 依層級，L4（款項）最短 |

**端點**（§6.1.4 已有兩支，本節指出缺的那一支）：

| 動作 | 端點 | 狀態 |
|---|---|---|
| 建案 | `POST /admin/shipment_batches/:batch_no/reconciliation_cases`（E2） | ✅ 已列 |
| 重跑 importer | `POST /admin/shipment_batches/reimport`（E3） | ✅ 已列 |
| 重新 `/query` | `POST /admin/shipments/:id/requery`（E4） | ✅ 已列 |
| **關案（含處置結果）** | `POST /admin/reconciliation_cases/:case_id/close` | **§6.1.4 E6**：沒有它，`RECONCILING` 的出邊（§7.3 表）就沒有執行者 |
| **登錄驗收結果** | `POST /admin/shipments/:id/dc_result` | **§6.1.4 E5**：§6A.3 (c) 的落地端點，`931`／`932` 的過渡期唯一入邊（§7.3.1 第 8 列） |
| **`BackSetShipping` 人工登錄** | `POST /admin/shipping_onboardings/:onboarding_id/back_set_shipping` | **§6.1.4 E7**（§6A.6 補償三件之②）：`BACKSET_MISSING` case 的唯一關閉者 |
| **退貨結案登錄** | `POST /admin/shipments/:id/return_outcome` | **§6.1.4 E8**：`RETURN_COLLECTED`（§7.3.2 X8）在 shipment 軸沒有值 ⇒ 沒有它退貨軸永遠結不掉 |

🔴 **四支都要 maker-checker ＋ audit 六欄**（§8.4）：它們是「人直接寫一個 provider 沒說過的事實」，
是本書權限最高的四個動作。

⚠️ **依 65 §3.11 第②問的判準——答案是「人工」時必須去查那個端點存不存在。**
上表用「✅ 已列 ／ 🔴 缺」兩態把這件事寫死，**留白會被讀成已存在**。

### 7.7 買家可見 6 階段 ↔ 主線映射

| 買家看到（§2.5 允許用詞） | normalized 值 |
|---|---|
| 賣家備貨中 | `91`／`92`／`920`／`98` |
| 前往物流中心 | `930` |
| 物流中心處理中 | `931` |
| 配送至門市 | `31` |
| 已到店，可取貨 | `32` |
| 已取貨／已退回 | `11`／`52`／`53`／`51` |

🔴 **禁止**：把 `930`（賣家出車）顯示為「物流中心已收件」——交接完成 ≠ 物流中心已驗收。
🔴 **禁止**對買家出現「大宗寄倉／母子代號／物流中心驗退／進倉批次」等詞（§2.5）。
🔴 **交接完成 ≠ 整批已驗收**：整批已驗收只能由**每件聚合**得出，且**單向不可回推**（INV-A1）。

---

#### 7.7.1 冷凍商品售後（🔴 公開指南僅支援常溫退貨 ⇒ 走客服／商家責任流程，而**那條流程需求書完全沒有描述**）

**事實基線**（§0.1 ⑪、§3.3 第 3／4 條）：

| 面向 | 常溫 | 冷凍 |
|---|---|---|
| B2C 開通時是否同時開通 C2B 退貨便 | ✅ 是（副作用非選項） | ❌ **否** |
| 買家主動退貨可用門市退貨代碼 | ✅ | 🔴 **不可用** |
| 驗退／未取退回的回收 | 依約定週期至物流中心領回 | **僅週二／週五兩退**，模式只有自行取件或統昶配合貨運 |

**定案：冷凍逆向不是「另一條流程」，是「沒有物流的流程」——本書把它落成一條可執行的降級路徑，
而不是留白。**

```
冷凍 shipment 進入需要逆向的狀態（51 / 52 / 53 / 932 / 33 / 44）
  │
  ├─ 系統做的三件（全部可機械執行）
  │    ① disabledReasonCode 不變、shipment 狀態照常推進（逆向物流缺席不影響貨態記錄）
  │    ② 🔴 buyer 端不得顯示任何「門市退貨」入口與退貨代碼
  │         判準＝ delivery_temp = '3203' ⇒ allowedActions[] 移除 accept_return
  │         （§7.2 X6 的 accept_return 在冷凍上不出現）
  │    ③ 自動建 reconciliation_cases（case_type = FROZEN_REVERSE_MANUAL），
  │         assignee 預設客服佇列，sla_due_at = 下一個週二或週五
  │
  └─ 系統不做的（🔴 明寫，避免下一個人以為漏了）
       ✗ 不安排取件、不產生逆向物流單、不通知司機
       ✗ 不自動退款（INV-7：退貨 ≠ 退款）
       ✗ 不猜測領回日期給買家看
```

**🔴 未定義的部分要標成未定義，不得編一個流程**：

| 未定 | 卡在哪 | fail-closed 形式 |
|---|---|---|
| 冷凍退貨的**責任歸屬與費用** | 屬 §11.2.2 商務前置（rate card ＋ 退貨條款）；公開資料只寫常溫的 38 元／件 | `shipping_fee_ledger` 對冷凍逆向**不寫費用列**，留給人工調整單；🔴 **不得沿用 38 元**——那是常溫的 policy 值 |
| 「統昶配合貨運」的接洽方式 | 需求書 §21.4 逐字「文件版本與費率可能不同，未取得書面報價前不得當成預設到府收件」 | 案件的 assignee 是**人**，不是一個排程器 |
| 買家看得到什麼文案 | 品味與法遵判斷，非工程可裁決 | 🔴 交產品／法務；在那之前 buyer 端顯示**通用**的「請聯繫客服」，**不得**顯示任何日期或費用承諾 |

⚠️ **這一節與 §7.7.2 的關係**：冷凍逆向的人工節奏（一週兩次）直接決定 §7.7.2 的門檻不能用天數，
否則冷凍賣家會在正常營運下被停權。

#### 7.7.2 「退貨未領停止新 checkout」的門檻（幾件／幾天，需求書未定義）

需求書 §21.5 逐字：「帳號停權、條款版本失效、對帳逾期、**退貨未領**或 provider 維護時，
立即停止新 checkout」——**沒有給任何門檻**。

**定案：門檻用「件數 ＋ 逾期輪次」兩個條件的 AND，不用天數。**

| 參數 | 值 | 推導 |
|---|---|---|
| 件數門檻 | **待定（§12.0 第 8 格解除前不上線）**；🔴 **不得先填一個數字** | 它是 provider 對物流中心堆置量的規定，屬 §11.2.2「退貨週期與物流中心異常通知方式」；猜一個數字＝拿賣家的營業權去賭 |
| 逾期輪次 | **2 輪**（連續兩個約定回收週期未領） | 常溫＝賣家自訂週期（`ReturnCycle`／`ReturnCycleType`，§6.1.3 E1）；冷凍＝週二／週五 ⇒ 兩輪對兩種溫層都是「明顯不是忘記，是不打算領」 |
| 判定粒度 | `(store_id, delivery_temp)` | 🔴 常溫與冷凍分開判：冷凍一週只有兩次機會，用同一個計數會讓冷凍賣家先被停 |

🔴 **為什麼不用天數**：「7 天未領」對常溫（可天天領）與冷凍（一週兩次）是完全不同的嚴厲程度。
用天數＝把 provider 的營運節奏差異變成我方的懲罰差異，而那不是任何人決定過的事。

**fail-closed 形式（值未定時怎麼上線）**：

| 狀態 | 行為 |
|---|---|
| 件數門檻未取得（**現況**） | 🔴 **本規則整條不啟用**（不停任何人），但**指標與案件照建**：`hawkr_b2c_unclaimed_return_backlog{store_id, temp}` ＋ 逾兩輪自動建 case 給客服 |
| 取得之後 | 啟用 `disabledReasonCode = RETURN_BACKLOG_EXCEEDED`（§6.1.7 已列）⇒ checkout 不出現 `3004` 選項 |

⚠️ **「不啟用」為什麼不是留白**：留白＝上線後沒有人知道有多少賣家在堆退貨；
先出指標，門檻拿到那天就有真實分布可以對照，而不是拿一個從沒量過的數字去停權。

🔴 **停權是收縮型操作，不停既有 shipment**：規則生效時只擋**新的 checkout**，
已成立的 shipment 一律讓它跑完（同 §12.3 Rollback 的原則）。
擋掉已成立的單＝製造一批沒有出口的包裹，那比堆積更糟。

**與賣家能力層的關係**：`RETURN_BACKLOG_EXCEEDED` 是 **capability 層的 `disabledReasonCode`**，
不是賣家停權（`SELLER_SUSPENDED` 是另一個值）。兩者不可互相冒充——
前者只關掉一個配送方式，後者關掉整個賣家。

### 7.8 每日 reconciliation 五層（🔴 **L1–L4 是需求書的四層，L0 是本書為 §6A.6 補償設計新增的第五層**；shipment ids 與貨態／batch planned-vs-accepted／fees returns claims／settlement-vs-bank；
🔴 **任一差異建立 case，不以人工改值消音**）

**執行者＝K68-8 `B2cReconcileScheduler`**（`cron` 每日一次、**lease**、租約 30 min，§7.0）。
它是 C-RECON，「不做什麼」欄逐字：**只建 case、不改值**。

🔴 **共同紅線**：任一層的差異一律建 `reconciliation_cases`（§5.1.10），
**不得由人工直接改值消音**——改值會讓下一輪對帳查不到差異，而差異還在。

| 層 | 對象 | 左值（我方） | 右值（provider／外部） | 差異判準 | `case_type` |
|---|---|---|---|---|---|
| **L0** 🆕 | 子商店開通狀態 | `seller_shipment_settings` 的 `onboarding_status`／`provider_status` | E7 `search_merchant_status` 的 `MerStatus`／`MemberStatus`／`SPB2C`／`SPB2CFrz` | 任一欄不等 | `RECON_L0` |
| **L1** | shipment ids 與貨態 | `provider_shipments.ship_trade_no` ＋ `shipments.status` | E4 `/query` 的 `ShipTradeNo`／`ShipStatus` | id 只存在於一邊；或狀態不等**且我方較舊**（見下） | `RECON_L1` |
| **L2** | batch planned-vs-accepted | `shipment_batch_items` 的件數 | 驗收結果（K68-7 importer／ADM `dc_result` 登錄） | 件數不等 ⇒ batch 轉 `RECONCILING`（§7.3） | `RECON_L2` |
| **L3** | 費用／退貨／理賠 | `shipping_fee_ledger`／`shipment_returns`／`shipment_claims` | provider 帳單與驗退明細 | 金額或件數不等 | `RECON_L3` |
| **L4** | settlement-vs-bank | `provider_settlement_lines` | 銀行入帳 | 金額不等 | `RECON_L4` |

🔴 **L0 不是需求書的四層之一**（需求書只寫 L1–L4），是本書為 §6A.6 補償設計新增的。
⚠️ **它救不了 `BackSetShipping` 本身**：E7 不回那一欄（§3.2.5）
⇒ L0 只證明「開通狀態對得起來」，`BackSetShipping` 仍靠 §6A.6 的 aging 指標 ＋ ADM 人工登錄端點。

🔴 **L1 的判準必須是「我方較舊」而不是「不相等」**：provider 是貨態的權威，
但我方有 provider 不知道的本地進度（`LOCAL_PROCESSING_DONE`，§7.5）
⇒ 只有 `isValidTransition(我方, provider) == true` 的不等才是差異；
反向的不等是**正常的**（provider 還沒收到我方剛做完的事）。
把兩者一起當差異 ⇒ **每一筆剛成批的 shipment 每天都會建一張 case**，
而那正是「告警被當狼來了」的起點。

🔴 **L4 對 B2C 是 `SELLER_DIRECT`**（§3.5.5：無平台撥款）
⇒ 右值是**賣家自己的**銀行入帳，我方拿不到 ⇒ **L4 在 v1.4 只跑到「我方帳本自洽」為止**，
`provider_settlement_lines` 與銀行的比對標為 **N/A 不是 0**（同 §8.2.1 G4 的寫法）。
把拿不到的那一半量成 0，會讓 Dashboard 顯示「零差異」而其實是零觀測。

⚠️ **每一層都要能回答「這一層今天跑過了嗎」**：靠 §7.0 的
`fulfillment_scheduler_last_success_timestamp_seconds{scheduler="B2cReconcileScheduler"}`，
告警 `absent() ∨ age > 3×cadence`。**`absent()` 那一半在這裡特別重要**——
對帳排程從未跑過時，五層的差異指標全都是 0，看起來像「完全沒有差異」。

### 7.9 並發與順序

**三種併發來源，處置各不相同**（沿 65 §7.6.1 的分類，本節填 B2C 的具體格子）：

| # | 來源 | 本書的具體場景 | 處置 | 🔴 錯誤的處置 |
|---|---|---|---|---|
| 1 | **多 pod 同時跑同一支排程** | K68-1～K68-9 全部 | lease（§7.0，逐支已推導租約） | 用永久旗標（`SET claimed = true` 且無時間欄）⇒ pod 中途崩潰即永久卡死且無告警 |
| 2 | **同一使用者的重複請求** | 賣家連點「建立批次」／「列印」；網路重送 | `Idempotency-Key`（§6.1.4 A2、B1）＋ §5.3 的冪等鍵 | 只靠前端 disable 按鈕——重新整理與網路重送擋不到 |
| 3 | **不同來源同時改同一列** | provider Notify 與 ADM 人工登錄同時推 `931` | 單列 CAS（`UPDATE ... WHERE status = :expected`）＋ `isValidTransition` | 先 `SELECT` 再 `UPDATE`（check-then-act），兩者之間別人已經改了 |

#### 四個具體重複風險

| # | 風險 | 為什麼會發生 | 防線 |
|---|---|---|---|
| R1 | **同一筆訂單兩個 `ShipTradeNo`** | `/trade` 逾時後重試 | §5.3 冪等鍵（🔴 不含重試次數）＋ `991` 一律先 `/query` 不重建（INV-3） |
| R2 | **同一筆 shipment 進兩個批次** | 兩個分頁各建一個批次 | INV-2 落 DB：`shipment_batch_items` 對「未結案批次」的部分唯一索引 |
| R3 | **同一則 Notify 處理兩次** | 多 replica ＋ check-then-insert 不是原子 | `ON CONFLICT DO NOTHING` ＋ affected rows 判首見（§6A.5 缺陷 1） |
| R4 | **batch 聚合與個件更新交錯** | 交接端點寫 `930` 的同時，importer 正在寫 `931` | 🔴 **同一個 DB ⇒ 可以同交易**（65 K-11 逐字）：聚合函式與個件更新放同一個 tx，並以 batch 列的 `SELECT ... FOR UPDATE` 序列化 |

🔴 **R4 是本書唯一「可以用交易解決」的併發**，其餘三個都跨交易或跨服務。
不要把 R4 的解法（同交易）誤推廣到 R1～R3——那正是 §6A.1 前提要先講清楚的原因。

#### 順序

🔴 **順序不能靠 MQ 保證**（65 §7.6.2）。本書所有的順序保證都來自**狀態機**，不來自訊息到達順序：

| 需要的順序 | 靠什麼保證 | 不靠什麼 |
|---|---|---|
| 取號 → 列印 | `/print_label` 必填 `shipTradeNo` ⇒ 沒號就送不出去 | 不靠「先按取號再按列印」的 UI 順序 |
| 列印 → 成批 | §7.1.1 ②：B1 驗 `ship_trade_no IS NOT NULL` | 不靠賣家照流程操作 |
| 交接 → 驗收 | `931` 的 allowed-from 只含 `930` | 🔴 **不靠事件時間戳比大小** |
| 個件 `931` → batch `DC_RECEIVED` | 聚合推導（全部個件皆 `931` 或明確 `932`） | 🔴 不靠 batch 事件先到 |
| provider 貨態彼此之間 | `isValidTransition` 的單調前進（§6.3.3） | 不靠 `ShipStatusTime` 先後 |

⚠️ **「用時間戳比大小來決定要不要套用」是本節最想擋的反模式**：
provider 的 `ShipStatusTime` 是**它的**時鐘，我方無法校時；
延遲與亂序在單一事件上分不出來（§6.3.3）。
判準永遠是「這個轉換在狀態機上合不合法」，不是「這個事件夠不夠新」。

### 7.10 靜默陷阱表

> §4.7 是**整合層**的靜默陷阱（slot key、config 缺席、topology）。
> 本節是**狀態機與批次層**的，兩張表不重疊。共同特徵：**不會編譯錯、不會測試紅**。

| # | 陷阱 | 症狀（為什麼安靜） | 防線 | 誰會抓到 |
|---|---|---|---|---|
| 1 | `ShipmentStatusMapper` 沒加 `930`／`931`／`932`／`991`／`992`／`993` | 落到最後的 `log.warn + return false` ⇒ **一行 warn、狀態寫不進去、HTTP 照樣 200** | §7.5 的集合歸屬 | 🔴 只有 §13.4 的變異測試；沒有 gate |
| 2 | 照抄 `990 CREATE_FAILED` 的形狀 | `990` 在四個集合外 ⇒ **完全沒有出口**（65 D-19 群 A 實測）；`991` 回查判「確定失敗」後落進一個一樣死的格子 | 同批補 `990` 的出口（§7.4 末段） | 無 gate ⇒ review |
| 3 | batch 聚合函式反推個件狀態 | 個件被 batch 覆蓋 ⇒ 單件的驗退結果消失，而總數看起來對 | INV-A1 ＋ AST gate（§13.4 I6） | ✅ AST gate；⚠️ 觀測窗只到函式邊界，**抽 helper 就看不到**（要寫進 gate 檔頭） |
| 4 | 批次分組鍵少一個 | `/print_label` 四參數不一致，**在列印時才發現，而包裹已經包好了** | §5.1.5 補的 `provider_account_setting_id` ＋ `ship_date` 兩欄 | ✅ 結構保證（分組鍵在表上） |
| 5 | `931`／`932` 沒有推進者就上線 | batch 永遠停在 `DISPATCHED`；48 h aging **會響**，但如果那條告警也沒上，就完全無聲 | §6A.3 第 3 點：三者皆未就緒前 `931`／`932` 不得出現 | 🔴 靠 §12.0 exit gate |
| 6 | aging 告警錨在狀態集合而非 `entered_at` | 狀態值一改名，告警靜默轉綠（查不到那個值＝沒有積壓） | §8.2.3：述詞錨在時間 | 🔴 見 §8.2.3 的四問 |
| 7 | `RETURN_COLLECTED` 的 aging 錨在 `shipments.status` | 每一筆**已領回**的退貨都會永久告警 ⇒ 告警被當狼來了忽略 ⇒ 真的沒領的那筆也沒人看 | §7.3.2 X7／X8：錨在 `shipment_returns.outcome IS NULL` | 無 gate ⇒ review |
| 8 | `from == to ⇒ false` 讓重播與重推長得一樣 | provider 開始亂推時指標上看不出來 | §6.3.3 的三支獨立 counter | 無 gate ⇒ review |
| 9 | 冷凍沿用常溫的退貨費 38 元 | 金額錯但流程全綠；賣家對帳時才發現 | §7.7.1：冷凍逆向**不寫費用列** | 無 gate ⇒ review |
| 10 | 「退貨未領」門檻先填一個猜的數字 | 停到不該停的賣家，而系統行為完全正確 | §7.7.2：值未取得前**整條規則不啟用**、只出指標 | 🔴 靠 §12.0 exit gate |

🔴 **這張表有一半的「誰會抓到」是「無 gate ⇒ review」，那不是失誤而是誠實標註**
（65 §3.10 誠實條款）。把它們寫成「已被機制守住」才是問題——
**假的保證會排擠掉真的警覺**。

⚠️ **第 1 列是所有列裡最貴的**：它同時是 §7.3.1、§7.3.2、§7.4、§7.5 四節的共同結論。
四節各講一次不是重複，是因為施工者可能只讀其中一節——但**修法只有一個地方**
（`ShipmentStatusMapper` 的四個集合），改的時候四節要一起回灌。

## 8. 跨切面關注點（需求書 §21.17、需求書 §21.18）

### 8.1 🔴 條碼：業務鍵同時是敏感值

**衝突事實**：`條碼`（provider barcode）被 需求書 §21.17 明列「不得進 log」，
而它正是 B7 掃描綁定的**業務鍵**——查一筆包裹、對一次帳、debug 一次綁定失敗，全部要靠它。

**遮罩策略（三條，缺一即無法運維或即洩漏）**：

| 場景 | 規則 |
|---|---|
| **傳輸** | 🔴 barcode 一律走 **request body**，**禁止**出現在 URL path／query。理由：gateway `middleware/logger.go` 把 `RawQuery` 併進 `path` 欄位（gateway `middleware/logger.go` 的 `sensitiveKeys`／`redactBody`／`redactQuery` 的實測教訓：`guestId` 就是這樣漏的），而 `redactQuery` 只擋清單內的 key |
| **log** | 一律遮成 `前 3 碼 + **** + 後 4 碼`；完整值只寫進 `provider_shipments` 與 audit 表，不進 log。同時把 barcode 的 key 名加入 gateway `sensitiveKeys`，**body 與 query 兩條路徑都要加**（同一個值的每條進入路徑都要遮） |
| **查詢** | 運維要用完整 barcode 定位時，走 ADM 端點（role＋purpose＋audit 六欄），**不是**去翻 Loki |

⚠️ **憑證 vs PII 的判準**（gateway `middleware/logger.go` 的 `sensitiveKeys`／`redactBody`／`redactQuery`）：barcode 問的是「拿到它能做什麼」。
若 barcode 單獨持有即可操作（掃描綁定不需第二因子）⇒ **它是憑證，不是 PII**，
處置等級比姓名電話更高。⇒ B7 端點必須另外驗 `store_id` 歸屬，**不能只認 barcode**。

### 8.2 標籤與 manifest

- 🔴 **標籤 PDF／圖片不得走一般 object storage 長期保存**（需求書 §21.17 明確排除，是對 65 §3.9 十一／《多媒體與圖片 URL 規範》的
  **explicit 例外**）。⚠️ 沉默帶過的後果是下一個人照《多媒體與圖片 URL 規範》存進一般 storage。
- 🔴 **標籤儲存的裁決在 §5.1.13，本節不重複裁決**（原稿在此做了第二次裁決且結論相反，已刪）。
  §5.1.13 定案 **方案 (a)：media-service ＋ 受控存取 ＋ TTL**，並把方案 (c)「只存 hash ＋ 即時重取」
  判為**技術上不成立**——`/print_label` 是**列印動作不是查詢**，重取＝重新列印，
  無法保證與已貼在包裹上的那張位元相同。
  ⚠️ 原稿此處主張的「(c) 為常態、E3 冪等回同一份」正是 §5.1.13 否決的那件事，
  且引用了一個不存在的欄位（`provider_shipments.label_hash`；本書的 hash 欄是
  §5.1.4 的 `shipment_packages.label_sha256`）。
  **本節只保留存取控制規約**：下載 URL 短效、一次性、需授權檢查；manifest 的 PII 規則同下。

- 下載 URL **短效、一次性、需授權檢查**；manifest 含收件人 PII，同等處理。

#### 8.2.1 需求書 13 指標 ＋ 6 條零容忍 guard ＋ 6 區 Dashboard

**命名沿 repo 既有 sql-exporter 慣例**（實測 `monitoring/sql-exporter/sql_exporter.yml`：
`hawkr_atm_refund_stuck_total`／`hawkr_holiday_coverage_days`／
`hawkr_payment_rescue_manual_pending`）⇒ 本書一律 `hawkr_b2c_*`。
🔴 **空表回 `-1` 不回 0**（`holiday` collector 的既有做法）——0 與「查詢看不到那一類東西」
在 PromQL 上無法分辨，而後者恰恰是最該告警的狀態（65 §3.10 ④）。

##### 13 個指標（需求書 §21.18 逐條 → 指標名 → 來源）

| # | 需求書講的 | 指標名 | 型別 | 來源 |
|---|---|---|---|---|
| 1 | capability 可用率 | `hawkr_b2c_capability_available_ratio{store_id}` | gauge | C-GATE 求值結果 |
| 2 | 寄件編號成功率 | `hawkr_b2c_trade_total{result}` | counter | K68-1 |
| 3 | 寄件編號延遲 | `hawkr_b2c_trade_duration_seconds` | histogram | C-ADPT |
| 4 | 重複寄件編號 | `hawkr_b2c_duplicate_ship_trade_no_total` | counter | 🔴 **零容忍 G3**，穩態必須恆 0 |
| 5 | 標籤失敗 | `hawkr_b2c_label_total{result}` | counter | K68-2 |
| 6 | batch 未關閉 aging | `hawkr_b2c_batch_open_aging_seconds` | gauge | sql-exporter |
| 7 | planned-vs-accepted mismatch | `hawkr_b2c_batch_count_mismatch` | gauge | sql-exporter（§7.8 L2） |
| 8 | 驗退率與原因 | `hawkr_b2c_dc_rejected_total{reason}` | counter | §6A.3 的入邊 |
| 9 | 到店時間 | `hawkr_b2c_dc_to_store_duration_seconds` | histogram | `930`→`32` 的時間差 |
| 10 | 未取率 | `hawkr_b2c_unclaimed_ratio{temp}` | gauge | `52` ÷ 已到店 |
| 11 | 退貨 aging | `hawkr_b2c_return_open_aging_seconds` | gauge | 🔴 錨 `shipment_returns.outcome IS NULL`（§7.3.2 X7） |
| 12 | event lag／replay | `hawkr_b2c_inbox_lag_seconds`、`hawkr_b2c_notify_replayed_total` | gauge／counter | §6.3.2、§6.3.3 |
| 13 | fee／settlement mismatch ＋ claim rate | `hawkr_b2c_fee_mismatch`、`hawkr_b2c_claim_open_total` | gauge／gauge | §7.8 L3 |

##### 6 條零容忍 guard（需求書 §21.18）

🔴 **零容忍的意思是「穩態值必須是 0，出現任何非 0 就是事故」**，
不是「數字大的時候才看」。⇒ 告警一律 `> 0 for 5m`，severity `critical`。

| # | guard | 怎麼量 | 🔴 量不到的話 |
|---|---|---|---|
| G1 | 跨 tenant 存取 | 端點層：`store_id` 歸屬檢查失敗次數 `hawkr_b2c_cross_tenant_denied_total` | 那不是「沒有發生」，是**沒有在看**（65 §3.10 ②） |
| G2 | 未知 code fallback | `hawkr_b2c_unknown_code_total{field}`；🔴 `ShippingType.fromValue` 遇 `5`／`6` 必須**拋錯不得靜默**（§0.3） | 靜默 fallback 是本書最大的單點：它讓 B2C 走到 C2C 的設定上 |
| G3 | 重複 provider shipment | 指標 #4 ＋ `provider_shipments` 的唯一鍵 | 唯一鍵是結構保證，指標是它的逃逸偵測 |
| G4 | 重複費用／退款／撥款 | `shipping_fee_ledger` 對 `(shipment_id, component, rate_snapshot)` 唯一 | B2C 無平台撥款（§3.5.5）⇒ 撥款那一半 **N/A 不是 0** |
| G5 | 未授權狀態跳轉 | `hawkr_b2c_transition_rejected_total{from,to}`（§6.3.3） | ⚠️ 它與 G2 是不同的東西：這裡是**合法值走了非法邊**，那裡是**值本身不認得** |
| G6 | PII 洩漏 | 見 §8.6：barcode 與 `userMessage` 兩條路徑各有斷言 | 🔴 **沒有一支指標量得到 PII 洩漏**——它由契約測試擋，不由 runtime 指標擋。誠實寫出來 |

##### 6 區 Dashboard

| 區 | 內容 | drill down 到 |
|---|---|---|
| provider health | #2 #3 #5 ＋ breaker 狀態 ＋ `hawkr_b2c_provider_throttled_total` | `correlationId` |
| warehouse operations | #6 #7 #8 ＋ batch 各狀態列數 | `batch_no` |
| seller exceptions | 9 個工作台清單的列數（§6.1.4 D1）＋ `991`／`992`／`993` aging | `store_id` |
| buyer delivery | #9 #10 ＋ 買家 6 階段分布（§7.7） | `order_no` |
| finance reconciliation | #13 ＋ 五層對帳各層差異列數（§7.8） | `case_id` |
| security audit | G1～G6 ＋ ADM 高風險 action 次數（§8.4 的 8 個觸發時機） | audit 六欄 |

🔴 **每一張圖預設遮罩 PII**（需求書 §21.18 逐字）：drill down 欄位一律是**業務單號**
（`batch_no`／`order_no`／`case_id`／`correlationId`），不是姓名、電話或 barcode。
⚠️ barcode 特別要注意——它是業務鍵**也是**憑證（§8.1），
放進 Dashboard 的 label 等於放進每個有 Grafana 讀權限的人手上。

#### 8.2.2 🔴 **六條缺口**：`CLOSED` 卡住（關了沒出車）／`DISPATCHED` 卡住／`RECONCILING` aging／`CREATE_UNKNOWN` aging／`CREATE_FAILED` aging／
**`BackSetShipping` 從未收到的賣家數**（需求書 §21.3.5 的單點風險，需求書零指標）

**§8.2.1 的 13 個指標來自需求書 §21.18；下表是本書自己補的。**
🔴 **本節同時是全書 aging 指標名的登記處**——§7.3.1／§7.3.2／§7.4 逐列引用的名字都在這裡，
沒有登記的名字視為不存在（§8.2.1 的 13 指標表不含它們，那不是漏列，是分工）。

| # | 缺口 | 指標名 | 型別 | 門檻 | 來源述詞（§8.2.3 的四問已逐條作答） |
|---|---|---|---|---|---|
| ① | batch `CLOSED` 卡住（關了沒出車） | `hawkr_b2c_batch_closed_aging_seconds` | gauge | **12 h** | `shipment_batches` 的 `status='CLOSED'` 且 `closed_at` 逾門檻 |
| ② | batch `DISPATCHED` 卡住 | `hawkr_b2c_dispatched_aging` | gauge | **48 h**（65 已定） | 同表 `status='DISPATCHED'` |
| ③ | batch `RECONCILING` aging | `hawkr_b2c_reconciling_aging_seconds` | gauge | **72 h** | 同表 `status='RECONCILING'` ＋ ADM 佇列列數 |
| ④ | `CREATE_UNKNOWN` aging | `hawkr_b2c_create_unknown_aging` | gauge | **180 min**（§0.4） | `shipments.status='991'` |
| ⑤ | `BackSetShipping` 從未收到 | `hawkr_b2c_backsetshipping_missing_sellers` | gauge | **24 h** | `seller_shipment_settings` 的 `onboarding_status='3602' AND back_set_shipping_at IS NULL` |
| ⑥ | **`CREATE_FAILED` aging** | `hawkr_b2c_create_failed_aging` | gauge | 🔴 **待定** | `shipments.status='990'` 且最後一次狀態變更逾門檻。<br/>🔴 **為什麼非補不可**：`991`／`992`／`993` 各有 aging，**只有 `990` 沒有**；而 R6 剛把 `990` 訂成 `C3 resubmit` **唯一**能作用的狀態（§7.2 **Y1**）⇒ 依 `06-no-small-tails.md` §3.13 第 ④ 問「停在這裡超過 N 小時，誰會知道？」，補之前的答案是**沒有人**。<br/>⚠️ **門檻不自行填值**：它量的是**賣家自助動作的 SLA**（不是系統收斂速度），與 `991` 的 180 分（機器回查窗）不同類 ⇒ 取值需功能擁有者裁決，比照 `992` 的門檻掛 **RFI-7** 的作法登記。🔴 **不得為了讓表看起來完整而填一個看似合理的數值** |

**主線與例外分支逐列引用的 aging 指標（§7.3.1／§7.3.2／§7.4 的第④欄）**：

| 指標名 | 對象 | 門檻 | 引用處 |
|---|---|---|---|
| `hawkr_b2c_pending_trade_aging` | `91` 未取號 | 24 h | §7.3.1 第 1 列 |
| `hawkr_b2c_pending_label_aging` | `92` 未回 `Odno` | 24 h（§0.4） | §7.3.1 第 2、3 列 |
| `hawkr_b2c_batch_open_aging_seconds` | batch `OPEN` | 24 h | §8.2.1 #6、§7.3.1 第 5 列 |
| `hawkr_b2c_dc_accepted_aging` | `931` 未進 `31` | 72 h | §7.3.1 第 8 列 |
| `hawkr_b2c_cancel_pending_aging` | `992` | 🔴 **待 RFI-7**（§11.2.1）⇒ 未取得前**只出 series 不設門檻**，不得憑空填一個數字 | §7.3.2 X2 |
| `hawkr_b2c_dc_rejected_open_cases` | `932` 未結案件數 | > 0 即列入 ADM 佇列 | §7.3.2 X3、§7.4 S0-6 |
| `hawkr_b2c_claim_pending_aging` | `993` | 依理賠受理期（§11.2.2 商務前置） | §7.3.2 X11、§7.4 S0-13 |
| `hawkr_b2c_return_open_aging_seconds` | 退貨未結 | §7.7.2 門檻未定前**不設門檻** | §8.2.1 #11、§7.3.2 X7／X8 |

🔴 **命名規則（全書一律遵守，違反即視為未登記）**：
① 前綴一律 `hawkr_b2c_`——**不得寫成 `b2c_*`**（sql-exporter 的 series 名沒有前綴就與別的 collector 撞名）；
② 量「一段時間」的一律以 `_seconds` 結尾，量「列數」的一律以 `_total`／`_cases`／`_sellers` 結尾。
⚠️ 上表兩張裡有 **8 個** `_aging` 沒有 `_seconds` 尾綴（`create_failed`／`pending_trade`／`pending_label`／`dispatched`／
`dc_accepted`／`cancel_pending`／`create_unknown`／`claim_pending`），**與 ② 不一致**。
🔴 **這批名字沒有外部擁有者，全部是本書自訂**（實測：65 全文零 `hawkr_` 命中，它用的是 `fulfillment_*`；
`monitoring/sql-exporter/sql_exporter.yml` 零 `b2c` 命中而 `hawkr_` 命中 16 ⇒ 查詢有效、確實一支都還不存在）
⇒ **沒有「改名要別人同意」的問題，施工時一次補齊尾綴即可**；
🔴 但要**整批改**並同批改本表——留一半是最壞的結果（兩種寫法各自看起來都對，而 PromQL 查不到的那半沒有任何提示）。
65 那邊只定了門檻值（48 h／72 h，65 §7.1.1.2 的 I 軸），**沒有定任何指標名**。

🔴 **空表回 `-1` 不回 0**（§8.2.1 開頭逐字）：本表每一個 gauge 都適用。

#### 8.2.3 每條告警回答四問（述詞錨在時間不錨在狀態集合）

**四問就是 65 §3.10 的那四問**（本書不另訂第二套）。逐條告警作答：

| 告警 | ① 觀測窗 | ② 窗外輸出 | ③ 誰能移到窗外 | ④ 目標不存在時 |
|---|---|---|---|---|
| `batch CLOSED > 12h` | `shipment_batches` 中 `status='CLOSED'` 的列 | 🔴 **必須 fail-closed**：查詢異常回 `-1` 而非 0 | 改狀態值名稱／把批次搬到別的表／手寫 SQL 改狀態 | 🔴 **表空時回 `-1` 觸發 critical**，不是回 0 靜默轉綠 |
| `batch DISPATCHED > 48h` | 同上，`status='DISPATCHED'` | 同上 | 同上 ＋ 聚合推導提前把它推走 | 同上 |
| `batch RECONCILING > 72h` | 同上 | 同上 | 差異 case 被人工關掉但差異還在 | 同上 |
| `991 aging > 180min` | `shipments` 中 `status='991'` | 同上 | K68-3 回查把它推走（**正常**）／人工改成別的值（**異常**） | 同上 |
| `BackSetShipping 未收到 > 24h` | `seller_shipment_settings` 中 `onboarding_status='3602' AND back_set_shipping_at IS NULL` | 同上 | ADM 人工登錄（**正常**） | 同上 |

##### 🔴 述詞為什麼要錨在時間，不錨在狀態集合

**錯的寫法**（看起來完全正常）：

```sql
SELECT count(*) FROM shipment_batches WHERE status IN ('DISPATCHED');
```

**它的失效方式**：狀態值改名、值域搬到別的 enum、或那一列被手寫 SQL 推到一個
應用層走不到的組合 ⇒ **查詢回 0 ⇒ 告警靜默轉綠**，而積壓還在。
這正是 65 §3.10 要防的推理錯誤：拿「沒有觀察到危險」推論「沒有危險」。

**對的寫法**（錨在 `entered_at`，門檻是參數不是述詞的一部分）：

```sql
SELECT COALESCE(
         EXTRACT(EPOCH FROM (now() - MIN(status_entered_at))), -1
       )::float8 AS oldest_age_seconds
FROM shipment_batches
WHERE status = 'DISPATCHED';
```

| 差別 | 錯的 | 對的 |
|---|---|---|
| 回傳 | 列數（0 有兩種意思） | **最舊那一列的年齡**（`-1` 專門表示「一列都沒有」） |
| 門檻在哪 | 寫死在 SQL | 在告警規則裡（`> 48*3600`）⇒ 改門檻不用改 exporter |
| 值改名時 | 靜默轉綠 | 仍然轉綠 ⚠️ **但**配一條「該狀態列數 ＋ 其他狀態列數 ＝ 總列數」的守恆斷言就抓得到 |

🔴 **§0 的紅字在這裡兌現：「門檻不是 SSOT，`entered_at` 才是」。**
⇒ `shipment_batches` 與 `shipments` 都必須有 **`status_entered_at`** 欄，
且**每次狀態轉換同交易更新它**。沒有這一欄，上面每一條告警都只能退回錯的寫法。

⚠️ **`status_entered_at` 不能用 `updated_at` 代替**：`updated_at` 會被任何一次無關的欄位更新
（回填 `odno`、寫 `label_sha256`）推掉 ⇒ 一筆卡了三天的批次會顯示成「剛剛才進這個狀態」，
而那正是最需要告警的那一筆。

##### 每支排程器另有兩條（沿 65 §7.5.1，本書不重述推導）

```promql
absent(fulfillment_scheduler_last_success_timestamp_seconds{scheduler="X"})
  or (time() - fulfillment_scheduler_last_success_timestamp_seconds{scheduler="X"}) > 3 * <cadence>
```

🔴 `absent()` 那一半對應四問的 ④：排程器**從來沒跑過**時沒有 series 可比較，
`age > 3T` 永遠不會響。

### 8.3 現況缺口：PayUni HTTP client 記錄完整 params

`PayUniAuditRecord` 的 javadoc 逐字寫「業務參數（**憑證欄位已遮罩**）」——
🔴 **遮的是憑證，不是 PII**。B2C 的 `/trade` params 含 `receiverName`／`receiverPhone`／`senderPhone`，
它們會整包進 `requestParams`（`PayUniAuditMapper` 的 `row.setRequestParams(record.requestParams())`）。

⇒ **必補**：`PayUniAuditMapper` 增一層 PII 遮罩（姓名保留首字、電話保留末 3 碼），
並在 §8.4 的 retention 到期時刪 raw params、只留不可逆 hash。

### 8.4 稽核與 retention

- audit **必記 6 欄**：actor／tenant／reason／before-after／policy version／correlation id。
- 8 個觸發時機：建店送件、開通狀態變更、批次關閉、交接確認、驗退登錄、差異 case 開關、
  理賠裁定、人工登錄 `BackSetShipping`。
- 🔴 **刪除 provider event、ledger 或 claim evidence 永不允許**（只能到期刪 raw payload／標籤／影像，
  保留不可逆 hash）。

---

### 8.5 設定與機密（secret manager；sandbox／production 隔離；輪替撤銷最小權限；正式 IP allowlist）

🔴 **B2C 與 C2C 共用同一把代理商 Key／IV**（§6 章首宣告）⇒ **本書不新增任何 provider 憑證**。
本節要處理的不是「新增機密」，是「既有機密多了一個使用者」帶來的四件事。

| 項目 | 規約 | 漏了會怎樣 |
|---|---|---|
| **key 名與值來源進 git，值不進** | 沿 65 §3.9 十／附錄 G.10。`k8s/values/shipping-service.yaml` 的 `secretRef` 名 ＋ `k8s/scripts/check-secret-declarations.sh` 的靜態宣告 | 🔴 環境不可重建、功能靜默失能（實例：`PAYUNI_API_URL` 讓 prod 金流全打 sandbox，#1798） |
| **sandbox／production 隔離** | 兩套 secret ＋ **兩個 `NotifyURL`**（§6.3.4） | 🔴 sandbox 的 Notify 會**驗簽通過**並打進 production inbox——驗簽擋不住它，因為 key 是對的 |
| **輪替與撤銷** | 沿既有 PayUni 憑證輪替流程；🔴 輪替時 **`NotifyURL` 不變**（網域凍結在建店那一刻，§6.3.4） | 把兩者綁在一起輪替＝所有已開通賣家的 Notify 一起失效，且**無法回復**（建店資料不可修改） |
| **最小權限** | C-ADPT 只讀憑證不寫；憑證不進 `payuni_audit_logs`（既有規則已遮憑證欄，§8.3） | — |

#### 正式 IP allowlist

**它是 Release gate 的一項（§13.8），且是本書唯一一個「我方做完仍不算數」的設定。**

| 方向 | 誰的 allowlist | 現況 | 落點 |
|---|---|---|---|
| **出站**（我方 → PAYUNi） | PAYUNi 側白名單我方來源 IP | 🔴 **屬 RFI-9**（§11.2.1）；值未取得 | 若需要固定出口 IP ⇒ 走既有 `egress-proxy`（《固定 IP 出口與第三方轉發規範（Egress Proxy）》），🔴 **不得為此另開新 proxy／新 VPC／新固定 IP** |
| **入站**（PAYUNi → 我方 Notify） | 我方側白名單 provider 來源 IP | 🔴 **provider 來源 IP 清單未取得**（需求書 §21.3.8 第 7 題） | ⚠️ **未取得前不得只靠 IP 白名單**——驗簽（§6.3.1）才是唯一到位的防線；IP 是加分不是替代 |

🔴 **兩個方向常被講成同一件事，它們不是**：出站白名單沒設好 ⇒ 我方呼叫被拒（**吵、看得見**）；
入站白名單沒設好 ⇒ 任何人都可以送一則 Notify 進來（**安靜、看不見**），而它會被驗簽擋下來
——所以入站的真正防線是驗簽，IP 白名單只是縮小攻擊面。

⚠️ **`egress-proxy` 這條路要先問「現在需不需要」**：現況 C2C 已在 production 呼叫同一組端點且沒有固定 IP 需求
⇒ 若 PAYUNi 回覆說 B2C 需要 IP allowlist，那是一個**新的**基礎設施依賴，
要走 《固定 IP 出口與第三方轉發規範（Egress Proxy）》 的接入流程（路徑前綴白名單 ＋ 斷路器 ＋ 呼叫端韌性），不是改一行 config。

### 8.6 PII 與 log 遮罩
#### 8.6.1 🔴 **業務鍵同時是敏感值**：`條碼`（provider barcode）被明列不得進 log，而它正是掃描綁定的業務鍵 ⇒ 遮罩策略必答

**遮罩策略三條全文在 §8.1（傳輸／log／查詢），本節不重複裁決。**
這裡補 §8.1 講了規則卻沒有列清單的那一格——**要加進 gateway `sensitiveKeys` 的 key 名**：

| key 名 | 進入路徑 | 為什麼要列 |
|---|---|---|
| `barcode` | body（B7）；🔴 **不得出現在 query／path** | §8.1：它是憑證不是 PII |
| `shipTradeNo` ／ `shipTradeNos` | body、query | 它是 provider 主鍵；單獨持有可查貨態 |
| `labelSha256` | body、回應 | 洩漏 hash 不致命，但它與 barcode 一起就構成標籤指紋 |
| `manifestChecksum` | body | 同上 |
| `consigneeMobile` ／ `consigneeName` ／ `consigneeMail` | body、回應 | 一般 PII |
| `returnAddress` ／ `contactTel` ／ `contactMail` ／ `serviceTel` | body（`/setting` 的 14 欄） | §4.6 第 5 列：現況 audit 記完整 params |

🔴 **同一個值的每條進入路徑都要遮**（gateway `middleware/logger.go` 的 `sensitiveKeys`／`redactBody`／`redactQuery` 的實測教訓：`guestId` 在 body 有遮、
以 `?guest_id=` 進來時整段落進 `path` 欄位）⇒ 上表每一個 key **`redactBody` 與 `redactQuery` 兩邊都要加**。
⚠️ 加完要確認 key 名經 `normalizeKey` 之後對得上——大小寫與底線寫法不同時，加了等於沒加。

**機械保證**：契約測試對每一個 key 各送一次含該 key 的請求，斷言 log 輸出不含原值。
🔴 對「key 名被改掉」要 **FAIL 不是 SKIP**：斷言「上表 key 集合 ⊆ `sensitiveKeys`」，
少一個就紅。

#### 8.6.2 標籤與 manifest 含 PII ⇒ 下載 URL 短效、一次性或授權檢查

**存取控制規約全文在 §8.2、儲存裁決在 §5.1.13，本節不重複。**
補三件那兩節沒有寫死的參數與一個容易漏的對象：

| 參數 | 值 | 推導 |
|---|---|---|
| 下載 URL 有效期 | **5 min** | 下界＝賣家從點擊到瀏覽器完成下載；上界＝短到「轉貼出去也已失效」 |
| 一次性 | ✅ 用過即失效 | 🔴 標籤 PDF 含收件人姓名電話與門市 ⇒ 可重用的 URL 等於一份可轉發的 PII 檔 |
| 授權檢查 | 每次組 URL 前驗 `store_id` 歸屬（§5.1.13 第 5 件） | 🔴 **不得只驗「知道 assetId」**——那等於把 objectKey 當憑證 |

🔴 **manifest 是最容易漏的那一份**：它不是標籤、不走 media-service、而是**交接時要交給司機的紙**。
§2.4 已定「司機只取得最低資訊」⇒ manifest 上**不得**出現買家姓名、電話、地址或訂單金額。
⇒ 落地形式：manifest 產生器與標籤產生器是**兩支不同的東西**，
manifest 只吃 `batch_no`／件數／封箱數／`manifest_checksum`／賣家窗口。

⚠️ **兩者的 checksum 目的不同，不要混用**：`label_sha256` 是**完整性**
（下載後驗，證明這張標籤沒被換過，§5.1.13 第 4 件）；
`manifest_checksum` 是**對帳**（交接時雙方各持一份，證明件數一致，§5.1.6）。
用同一個欄位承載兩件事，就會在其中一邊需要重算時把另一邊弄壞。

#### 8.6.3 現況缺口：PayUni HTTP client **會記錄完整 params**（需求書 §21.3.7 第 5 條）

**缺口本體與修法在 §8.3，本節不重複。** 補一件 §8.3 沒有寫的：**這個缺口今天就存在，不是 B2C 帶來的**。

| 事實 | 狀態 |
|---|---|
| `PayUniAuditRecord` 的 javadoc 逐字「業務參數（**憑證欄位已遮罩**）」 | 遮的是憑證，不是 PII |
| C2C 的 `/trade` 已經在送 `receiverName`／`receiverPhone` | 🔴 **所以 prod 現在就在記買家 PII** |
| B2C 讓它變嚴重的是 `/setting` 的 14 欄 | 多了 `returnAddress`／`contactMail`／`serviceTel`（賣家的營業資料） |

🔴 **這改變了修法的優先序**：它不是「B2C 上線前要做的事」，是**現在就該修的事**，
而 B2C 只是讓它從「買家電話」擴大到「賣家地址」。
⇒ 本書把它列進 §11.1 技術債（既有），並在 §12.1 步 3（adapter ＋ 狀態映射）標成**前置**而非**並行**
——因為 B2C 的第一筆 sandbox `/setting` 就會把 14 欄寫進去。

⚠️ **遮罩要在 mapper 層不是在 log 層**：`payuni_audit_logs` 是**資料表**不是 log，
gateway 的 `sensitiveKeys`（§8.6.1）完全管不到它。
⇒ 修改點是 `PayUniAuditMapper` 的 `row.setRequestParams(...)`，
**不是**加一條 Loki 的遮罩規則。這兩個地方看起來都叫「遮罩」，但只有前者擋得住。

### 8.7 稽核（audit 必記 6 欄：actor／tenant／reason／before-after／policy version／correlation id；
8 個觸發時機；🔴 **刪除 provider event、ledger 或 claim evidence 永不允許**）
### 8.8 retention 分層（交易／稅務／理賠／provider contract；到期刪 raw payload、標籤與影像，保留不可逆 hash）

**四層分層（需求書 §21.17 逐條 → 本書的欄位與年限）**：

| 層 | 保存對象 | 年限 | 到期動作 | 依據 |
|---|---|---|---|---|
| **交易** | `provider_shipments` 的參數快照、`shipments`、`shipment_batches`、`shipment_batch_items` | **7 年** | 🔴 **不刪**（沿既有 `payuni-audit-retention-days=2555` ＝ 7 年，實測 shipping-service 設定） | 稅務與商業紀錄 |
| **稅務** | `shipping_fee_ledger`、`provider_settlement_lines` | **7 年** | 不刪 | 同上 |
| **理賠** | `shipment_claims`、`shipment_returns` 的 `outcome`／金額欄 | **7 年** | 不刪 | 爭議追溯 |
| **provider contract** | `payuni_audit_logs.request_params`／`response_body`、標籤 PDF、理賠影像、`provider_event_inbox.payload_raw` | 🔴 **短**（見下表） | **刪內容、留 hash** | 需求書 §21.17 逐字「到期刪除 raw payload、標籤與影像，保留不可逆 hash」 |

#### 第四層逐項的年限與到期動作

| 對象 | 年限 | 到期做什麼 | 留下什麼 |
|---|---|---|---|
| `payuni_audit_logs.request_params` | **90 天** | `UPDATE ... SET request_params = NULL` | `params_sha256` ＋ metadata（who／when／endpoint／correlationId） |
| `provider_event_inbox.payload_raw` | **90 天**（且必須 `processed_at IS NOT NULL`） | 同上 | `payload_sha256` ＋ `event_key` |
| 標籤 PDF（media asset） | 🔴 **交倉完成 ＋ 保存期**（`shipments.label_expire_at`，V1 既有欄） | `ShipmentLabelService.unlinkLabel` 解除引用（既有方法） | `shipment_packages.label_sha256` |
| 理賠影像 | **理賠結案 ＋ 1 年** | 解除引用 | `evidence_asset_ids` 的 hash |

🔴 **兩條永不允許（§8.4 逐字）**：刪除 **provider event 本身**（列）、**ledger 列**、**claim evidence 的紀錄**。
到期刪的是**內容**（`payload_raw`、PDF、影像），不是那一列。
⇒ 到期之後仍答得出「這件事發生過、我方當時收到什麼的指紋是什麼」。

⚠️ **hash 要在寫入當下算，不能在刪除當下算**：刪除排程跑的時候若才算 hash，
中間任何一次資料修復都會讓 hash 對應到一份被改過的內容，而沒有人分辨得出來。
⇒ `params_sha256`／`payload_sha256`／`label_sha256` 全部**與內容同一交易寫入**。

⚠️ **90 天不是隨便挑的**：它必須 > 對帳與爭議的最長回溯窗（§7.8 五層 ＋ 理賠受理期）。
🔴 理賠受理期屬 §11.2.2 商務前置**未取得** ⇒ 90 天是**暫定值**，
拿到條款那天要回頭核對；若條款的爭議窗 > 90 天，這一格必須加長，**不得反過來縮短爭議窗**。

### 8.9 靜默陷阱表

> §4.7 是整合層、§7.10 是狀態機與批次層，本節是**跨切面層**（安全／稽核／retention／設定）。
> 三張表不重疊。共同特徵仍是：**不會編譯錯、不會測試紅**。

| # | 陷阱 | 症狀（為什麼安靜） | 防線 | 誰會抓到 |
|---|---|---|---|---|
| 1 | barcode 只在 `redactBody` 遮、忘了 `redactQuery` | 以 `?barcode=` 進來時整段落進 log 的 `path` 欄位 ⇒ **憑證明文落 Loki**，而 API 行為完全正常 | §8.6.1：兩條路徑都加 ＋ 契約測試 | ✅ 契約測試（若有寫） |
| 2 | key 名經 `normalizeKey` 後對不上 | 清單裡有這個 key，但實際比對不到 ⇒ **加了等於沒加**，且看起來已經處理過 | §8.6.1 末段 | 🔴 只有真的送一次請求並讀 log 才知道 |
| 3 | 遮罩加在 Loki 而非 `PayUniAuditMapper` | `payuni_audit_logs` 是**資料表**不是 log ⇒ Loki 規則完全管不到它 | §8.6.3 末段 | 無 gate ⇒ review |
| 4 | hash 在刪除當下算 | 中間任何一次資料修復都讓 hash 對應到被改過的內容，且無人分辨得出 | §8.8 末段：同交易寫入 | 無 gate ⇒ review |
| 5 | 到期刪整列而不是刪內容 | provider 曾經說過的話消失，**而它無法重查**（§5.1.7） | §8.8：刪內容留列 | 無 gate ⇒ review |
| 6 | sandbox 與 production 共用 `NotifyURL` | sandbox 的 Notify **驗簽會通過**（key 相同）⇒ 打進 production inbox 並推動真實貨態 | §8.5：兩個 `NotifyURL` | 🔴 無 gate；只有 §12.0 exit gate |
| 7 | 憑證輪替時一併換 `NotifyURL` | 網域凍結在建店那一刻 ⇒ **所有已開通賣家的 Notify 一起失效且無法回復** | §8.5、§6.3.4 | 🔴 無 gate ⇒ Runbook（§12.4） |
| 8 | 入站只靠 IP allowlist、不驗簽 | provider 換出口 IP ⇒ 全部 Notify 被拒，**而錯誤在我方、沒有任何訊號送給 provider** | §8.5：驗簽是唯一防線，IP 只縮小攻擊面 | 無 gate ⇒ review |
| 9 | manifest 沿用標籤產生器 | 買家姓名電話跟著司機離開我方系統（§2.4） | §8.6.2：兩支不同的產生器 | 無 gate ⇒ review |
| 10 | 下載 URL 可重用 | 一次轉貼＝一份可無限取用的 PII 檔，且 audit 只記到第一次 | §8.6.2：5 min ＋ 一次性 | 無 gate ⇒ review |
| 11 | `userMessage` 串了 provider message | 同一段 provider 原文同時出現在五端 ＋ Sentry ＋ 客服工單，而 §8.6 的遮罩只管 log | §6.1.8 | ✅ 契約測試（白名單斷言） |

🔴 **十一列裡有七列是「無 gate ⇒ review」**。這不是失誤，是本節的誠實標註（65 §3.10 誠實條款）：
把它們寫成「已被機制守住」會讓下一個人不再檢查，而**假的保證會排擠掉真的警覺**。

⚠️ **第 2 列是本表唯一「連測試都容易假綠」的一列**：測試如果自己組一個
「我以為 normalize 之後長這樣」的 key，就會與被測物一起錯。
⇒ 該測試必須**送真實請求、讀真實 log 輸出**，不得斷言 `sensitiveKeys` 的內容。

## 9. 品質需求與 NFR

| 項 | 值 | 根據 |
|---|---|---|
| `/trade` p95 | ≤ 3s（deadline 10s） | 🔴 deadline 必須 < 租約下界（《多副本排程搶佔規範》：租約 > 單筆最壞耗時） |
| `/print_label` 50 筆 p95 | ≤ 15s | 一 batch 500 件 ⇒ 10 組 ≈ 2.5 分鐘，UI 須非同步 |
| 貨態 Notify 端到端 | p95 ≤ 30s（收到 → normalized transition 落地） | 既有 queue 無 backlog 時 |
| batch 件數 | 軟上限 **500 件／批**（超過建議拆批，非硬拒） | 需求書無上限；取自 `/print_label` 50 筆 × 10 組的可操作性 |
| 硬上限依賴 | PayUni rate limit（值屬 RFI-9）、物流中心可驗收時段 | 🔴 《硬上限依賴接入治理規範（配額與容量）》：接上生產路徑前**必須先有**「與上限對齊的指標 ＋ 告警 ＋ 預算數字」三件 |

---

### 9.1 SLO／SLI

🔴 **SLI（量得到的東西）與 SLO（承諾的目標）要分開寫**：上面那張表混寫了兩者，
本節把它拆開，並明標哪些**今天量不到**。

| # | SLI（怎麼量） | SLO（目標） | 資料來源 | 現況 |
|---|---|---|---|---|
| S1 | `/trade` 成功率 ＝ `hawkr_b2c_trade_total{result="ok"}` ÷ 全部 | ≥ **99.5%**（滾動 7 日） | K68-1 | 🔴 **無基線**——B2C 從未跑過；C2C 的數字不可直接沿用（不同 `LgsType`、不同審核路徑） |
| S2 | `/trade` p95 延遲 | ≤ **3 s**（deadline 10 s） | C-ADPT histogram | 同上 |
| S3 | `/print_label` 50 筆 p95 | ≤ **15 s** | K68-2 | 同上 |
| S4 | 貨態 Notify 端到端（收到 → normalized 落地）p95 | ≤ **30 s** | `hawkr_b2c_inbox_lag_seconds` | ⚠️ C2C 同一條 queue 上量得到，可當**下界參考**（B2C 只會更慢不會更快） |
| S5 | capability 判定成功率（gate 不回 `UNKNOWN` 的比例） | ≥ **99.9%** | C-GATE | 🔴 依賴單體 seller 模組 gRPC 可用性，**不是本服務單獨決定** |
| S6 | 交倉到驗收（`930` → `931`）| 🔴 **不設 SLO** | §7.3.1 第 8 列 | 🔴 **推進者是人或檔案**（§6A.3）⇒ 設一個我方無法影響的目標，只會製造一條永遠紅的告警 |
| S7 | 驗收到到店（`931` → `32`） | 🔴 **不設 SLO** | 同上 | provider 的配送時間；我方只量分布不承諾 |

🔴 **S6／S7 明寫「不設 SLO」是本節最重要的一格**：
需求書 §21.18 把「到店時間」列進主要指標，但**指標不等於 SLO**。
把一個我方完全無法影響的外部時長寫成承諾，會在第一週就把告警訓練成雜訊
——之後真正該看的那幾條也一起被忽略。⇒ 它們**只出分布圖，不出告警**。

⚠️ **S1～S3 沒有基線這件事要寫進 §12.0 的 exit gate**：
canary 前先跑 shadow（§13.5 的 14 天）取得真實分布，**再回頭把上表的目標值訂死**。
現在表上的數字是**設計初值**，不是量測結果——兩者混寫會讓下一個人以為它們被驗證過。

### 9.2 容量與規模（批次件數、`/print_label` 50 筆分組、Notify 量）

| 維度 | 值 | 推導 | 超過時 |
|---|---|---|---|
| 批次件數 | 軟上限 **500 件／批** | §7.1.3：50 筆／組 × 10 組 × p95 15 s ≈ 2.5 分鐘 | 提示拆批，**不硬拒** |
| 單次 B1 請求 `orderNos[]` | 硬上限 **200 筆** | 請求體大小 ＋ 單一交易時長 | 回業務碼拒絕 |
| `/print_label` 單次 | 硬上限 **50 筆** | 🔴 provider 規格（§0.1 ⑤） | 後端自動切組 |
| 標籤 PDF 單檔 | ≈ 50 筆／檔 | 一組一份 | 🔴 **不合併**（合併後 SHA-256 對不上單筆，§7.1.3） |
| 貨態 Notify 量 | 每筆 shipment 約 **5～8 則**（`92`／`920`／`931`／`31`／`32`／`11` ＋ 重播） | §7.3.1 主線 9 格，扣掉我方自己推的 | 與 C2C **共用 queue**（§6A.2）⇒ 量疊加 |
| inbox 列數成長 | ≈ Notify 量 × 賣家數 × 訂單數 | — | 90 天後 `payload_raw` 轉 NULL（§8.8）⇒ 列在、內容不在 |

🔴 **與 C2C 共用 queue 的容量後果要寫出來**（§6A.2 已寫代價，這裡寫數字面）：
B2C 上線＝那條 queue 的流量疊加，而它**沒有獨立的積壓指標**
⇒ §6A.2 末段要求的「DLQ 深度按 `lgsType` 拆維度」在這裡兌現為第二件事：
**consumer lag 也要按 `lgsType` 拆**，否則 B2C 塞住時看到的是一條混合曲線。

⚠️ **本節所有數字都是設計初值**：真實件量屬 §11.2.2 商務前置
（7-ELEVEN 申請表要填「預估件量」）。拿到之後回頭核對 500 與 200 這兩個值——
🔴 特別是「一個賣家一天出 2000 件」的情境下，500 件／批會變成一天 4 個批次，
而每個批次都要一次司機交接（§7.1.2 第 9 步）⇒ 那是營運節奏問題不是技術問題，要一起看。

### 9.3 硬上限依賴治理（PAYUNi rate limit、物流中心可驗收時段）

**判準（《硬上限依賴接入治理規範（配額與容量）》）**：這個依賴用滿了會「回錯誤／拒絕服務／丟資料」⇒ 適用；答案是「變慢」⇒ 不適用。

| 依賴 | 適用？ | 上限值 | 三件前置（指標／告警／預算數字） |
|---|---|---|---|
| **PAYUNi rate limit** | ✅ 適用（用滿會被拒） | 🔴 **屬 RFI-9，未取得**（§11.2.1） | ① `hawkr_b2c_provider_throttled_total`（用量面）＋ 🔴 上限那一個 series **拿不到值** ② 告警：throttled > 0 for 5m ③ 預算：**寫不出來** |
| **物流中心可驗收時段** | ✅ 適用（過了時段就是拒收，包裹白跑一趟） | 🔴 屬 §11.2.2 商務前置，未取得 | ① 交接時段 vs 可驗收時段的偏差 ② 交接登記落在時段外即**擋下並提示** ③ 預算：時段是離散的，不是配額 |
| 標籤 PDF 儲存（media-service） | ❌ 不適用 | 無硬上限 | — |
| inbox 表成長 | ❌ 不適用（DB 容量是「變慢」不是「拒絕」） | — | 🔴 但 §8.8 的到期清理仍要做，否則會變成適用 |

🔴 **這一節目前不合格，而且要寫出來**：《硬上限依賴接入治理規範（配額與容量）》 明訂
「接入前置三件缺一不得接上**任何環境**的生產路徑」，而 PAYUNi rate limit 的
**上限值拿不到** ⇒ 三件裡的「與上限對齊的指標」與「預算數字」都做不出來。

**fail-closed 形式（值未取得時怎麼上線）**：

| 措施 | 內容 |
|---|---|
| 保守限流 | C-ADPT 自己端一個**我方側**的 rate limiter，值取保守（例：批次列印每分鐘 ≤ N 組），**寫進 config 可調** |
| 只量用量不算佔比 | 出 `hawkr_b2c_provider_call_total{endpoint}`；🔴 **不得憑空生一個「上限」series** 去算佔比——那是編造的分母 |
| throttled 即告警 | 被 provider 限流是**唯一**能證明上限存在的訊號 ⇒ `> 0` 就當事故看，並用它回推真實上限 |
| exit gate | §12.0：RFI-9 未回覆前不得開 Buyer production 流量（§13.8 已列） |

⚠️ 🔴 **不得用「加大額度／升付費方案」當修法**，也不得「先接上去之後再補監控」
（《硬上限依賴接入治理規範（配額與容量）》 逐條否決）。本書的處置是**先接上我方側的限流**，那不是繞過，
它是「我們自己決定用多少」——與「對方允許我們用多少」是兩件事，兩件都要有。

### 9.4 效能預算

**預算的意思是「這一段時間分給誰」，不是「希望它多快」。** 逐條把 §9 表頭的 p95 拆開：

| 動作 | 總預算 | 拆給誰 | 剩餘餘裕 |
|---|---|---|---|
| `/trade` 一筆 | **3 s** (p95) | C-GATE 求值 200 ms（含單體 seller 模組 gRPC，deadline 10 s 但正常 < 100 ms）／參數組裝 < 10 ms／provider HTTP **2.5 s**／寫 `provider_shipments` ＋ `shipments` 同交易 200 ms | ≈ 90 ms |
| `/print_label` 一組 50 筆 | **15 s** (p95) | provider HTTP **10 s**／PDF 下載 2 s／media 上傳 2 s／算 SHA-256 ＋ 寫欄 500 ms | ≈ 500 ms |
| 貨態 Notify 端到端 | **30 s** (p95) | 驗簽 ＋ 落 inbox < 200 ms（回 200 給 provider）／C-INBOX tick 等待 **≤ 10 s**（`fixedDelay 10s`，K68-9）／處理 ＋ 狀態轉換 ＋ outbox 300 ms／MQ 投遞 < 1 s | ≈ 18 s |
| B1 建立批次（200 筆） | **5 s** | 訂單重新載入 ＋ 歸屬驗證 2 s／逐筆 gate 求值（批次化，一次 gRPC）500 ms／寫 `shipment_batch_items` 1 s | ≈ 1.5 s |

🔴 **deadline 必須小於租約下界**（65 §3.9 九）——這是本節與 §7.0 的接縫，逐支對照：

| 呼叫 | deadline | 對應租約 | 不等式成立？ |
|---|---|---|---|
| `/trade` | 30 s × retry 3 ＝ 90 s 最壞 | K68-1 **120 s** | ✅ 120 > 90 |
| `/print_label` | 60 s ＋ 下載上傳 | K68-2 **300 s** | ✅ |
| `search_merchant_status` | 10 s × 分批 | K68-6 **10 min** | ✅ |
| `/query`（回查） | 10 s | K68-3 **10 min** | ✅ |

⚠️ **「p95 3 s」與「deadline 30 s」不是同一件事，不要用前者當後者**：
p95 是常態表現，deadline 是**最壞情況的上界**，而租約要用後者推。
用 p95 推租約 ⇒ 尾端那 5% 的請求會在還在跑的時候被別的 pod 合法搶走
（《多副本排程搶佔規範》的 C1 實例：30 s 租約 < 100 s deadline ⇒ 重複退款）。

⚠️ **貨態 Notify 的 18 s 餘裕不是浪費**：它幾乎全部是 C-INBOX 的 tick 等待。
把 `fixedDelay` 縮短可以換到更低的 p95，但也會提高空轉查詢的頻率
⇒ 這一格是可調的旋鈕，**調之前要先有 §9.1 的實測分布**。

### 9.5 資料保存與清理（同 §8.8，此處寫可執行的清理排程）

**年限與到期動作在 §8.8，本節只寫「誰去做、什麼時候做、做不完誰知道」。**

| 排程 | 對象 | cadence | 形狀（65 §3.9 九） | 沿用還是新增 |
|---|---|---|---|---|
| `PayUniAuditCleanupScheduler` | `payuni_audit_logs` | `cron 0 30 3 * * ?` | **非 claim 形狀（GC）** | 🔴 **既有**（`services/java/shipping-service/.../scheduler/PayUniAuditCleanupScheduler.java`）⇒ 本書只改它的動作：從**刪列**改成**清 `request_params` 留 hash**（§8.8） |
| `DedupCleanupScheduler` | `shipping_webhook_dedup` | `cron 0 0 3 * * ?` | 非 claim（GC） | 既有，本書不改 |
| 🆕 `B2cInboxPayloadCleanupScheduler` | `provider_event_inbox.payload_raw` | `cron 每日一次` | 非 claim（GC） | **新增**；🔴 條件必含 `processed_at IS NOT NULL`——未處理就清內容＝那則事件永久無法重放 |
| 🆕 `B2cLabelExpiryScheduler` | 逾 `label_expire_at` 的標籤 asset | `cron 每日一次` | **lease**（要打 media-service，是逐筆副作用） | **新增**；呼叫既有 `ShipmentLabelService.unlinkLabel` |

🔴 **GC 形狀與 lease 形狀的差別不是風格**：前兩支只刪自己 DB 的列，重跑一次沒有副作用
⇒ 不需要搶佔；`B2cLabelExpiryScheduler` 要打 media-service（外部呼叫）
⇒ 多 pod 同時跑會對同一個 asset 解除兩次引用 ⇒ **必須 lease**。

**做不完誰知道**：

| 訊號 | 指標 |
|---|---|
| 排程沒跑 | `absent(fulfillment_scheduler_last_success_timestamp_seconds{scheduler="..."})`（§8.2.3） |
| 跑了但清不完（積壓） | `hawkr_b2c_retention_overdue_rows{table}` ＝ 已逾期但內容仍非空的列數；🔴 **空表回 `-1`** |
| media 解除引用失敗 | `hawkr_b2c_label_unlink_failed_total` |

⚠️ **`hawkr_b2c_retention_overdue_rows` 不是可有可無的**：清理排程掛掉時，
上面第一條（`absent`）只在**完全沒跑**時響；「跑了但每輪只清得動一小部分」
是無聲的——那正是 retention 承諾實際失效的方式。

🔴 **一次性補清與常態排程要分開**（《後端服務責任邊界規範》）：既有資料的第一次大量清理走
**維運 SQL**（人執行、有計畫、有授權），不寫進 application code。
本節四支排程只負責**常態**，不負責追平歷史。

## 10. 決策落點（本書不另開決策章）
> 決策寫在 **§1.5 已定案決策一覽**（承接 65 的 D 系列）與 **§3.7 由本書定版的決策**兩處；跨本決策（文件切割、責任歸屬、金額語意等）**集中在 [`docs/adr/`](../adr/)**，本書只引用該處檔名。

## 11. 風險、技術債與未決問題
**覆蓋**：需求書 §21.3.8、需求書 §21.4、需求書 §21.22

### 11.1 已知風險與技術債

🔴 **`LgsType.java` 的 javadoc 與實際用法相反**

`services/java/shared/payuni-java-client/.../shipping/LgsType.java` 的註解逐字：

```
/** 超商取貨（店到店） */   B2C("B2C", "超商取貨"),
/** 超商寄件（消費者到店） */ C2C("C2C", "超商寄件"),
```

而**實際用法相反**：既有的 7-11 店到店（`3002`）三處呼叫點全用 `C2C`
——`CvsShipmentStrategy.LGS_TYPE_C2C`／`OnboardingService.CVS_LGS_TYPE`／`StoreMapService.FIXED_LGS_TYPE`，
且 `LgsType.B2C` 在 `shipping-service` **零使用**（全 `services/java/` grep `LgsType.` 只命中 `HOME`／`C2C`）。
官方總覽 [PayUni 物流官方 API 總覽](../第三方整合/PayUni%20物流官方%20API%20總覽.md) 也寫 `B2C`＝**大宗寄倉**、`C2C`＝店到店。

🔴 **照那份 javadoc 施工會把本模式的 `LgsType` 送錯值，而送錯 PayUni 直接退。**
🔴 **本書一律以「`B2C`＝大宗寄倉、`C2C`＝店到店」為準**（與官方總覽及三處實際呼叫點一致）；
該 javadoc 屬 production source，修正它是獨立的一次改動。


> 分兩類：**(A) 本設計固有的 trade-off**、**(B) 既有缺陷，B2C 會第一個踩到**。
> 🔴 (B) 特別重要——它們今天就存在，寫成「B2C 上線前要做」會讓人以為現在沒事。

#### (A) 本設計固有的 trade-off

| # | 項目 | 風險 | 償還條件 |
|---|---|---|---|
| A1 | **`NotifyURL` 逐賣家凍結** | 建店資料不可修改（§3.4）⇒ 換對外網域時，已開通賣家**不會跟著換**，只能逐一 `CLOSED` 重申請，而 `3605` 不可逆（§6.3.4） | 🔴 無解，只能**記著**。換網域前先盤點已開通賣家數並排人工遷移 |
| A2 | **B2C 與 C2C 共用一條 queue** | B2C 的 poison message 會拖累 C2C（§6A.2 逐字） | 分維度指標（§9.2）先上；真的發生再拆 queue（拆＝ topology +2/+2，§6.4.4） |
| A3 | **`931`／`932` 靠人工或檔案** | `DISPATCHED` 48 h aging 會常態告警（§6.3.5） | RFI-5 取得 provider 貨態碼，或取得對帳檔 |
| A4 | **`RETURN_COLLECTED` 沒有 normalized 值** | shipment 軸停在 `53` 不動 ⇒ aging 必須錨在退貨軸（§7.3.2 X8） | 回報 65；若補值，§7.2 映射表與 §7.3.2 同批改 |
| A5 | **6 條 batch routing key 無 CI 保護** | topology 增減量 0 ⇒ 沒有任何 gate 會被觸發（§6.4.4） | 🔴 無 gate，只能 review ＋ §6.4.7 的實跑驗收 |
| A6 | **`ShipmentStatusMapper` 的六個新值靠變異測試守** | 漏一個就是 `log.warn + return false`，狀態寫不進去而 HTTP 200（§7.10 第 1 列） | §13.4 的變異測試是唯一防線 |

#### (B) 既有缺陷，B2C 會第一個踩到

| # | 項目 | 🔴 今天的狀態 | B2C 如何放大 | 處置 |
|---|---|---|---|---|
| B1 | **`payuni_audit_logs` 記完整 params** | 🔴 **prod 現在就在記買家姓名與電話**（C2C 的 `/trade` 已在送）；javadoc 逐字「憑證欄位已遮罩」——遮的是憑證不是 PII | `/setting` 的 14 欄再加上賣家地址／Email／服務電話 | §8.6.3：修 `PayUniAuditMapper`，列為 §12.1 步 3 的**前置**不是並行 |
| B2 | **`990 CREATE_FAILED` 是死狀態** | 四個集合皆不含它 ⇒ `isValidTransition` 走到 `log.warn + return false`，**完全沒有出口**（65 D-19 群 A 實測） | `991` 回查判「確定失敗」後會落進同一個死格子 | §7.4 末段：**同批**補 `990` 的出口 |
| B3 | **`shipping_webhook_dedup` 是 check-then-insert** | 非原子；多 replica 下第二次 insert 撞唯一鍵 ⇒ 交易 aborted（`25P02`）⇒ 後續 SELECT 全 500 | B2C 讓同一條 queue 的量疊加，撞的機率變高 | §6A.5 缺陷 1：改 `ON CONFLICT DO NOTHING` |
| B4 | **`mapShippingMethodToType` 對未知值回 `null`** | 靜默；`findActiveSetting` 回 null ⇒ 建單拋 `5004`「賣家尚未開通」 | `3004` 是第一個會踩到它的新值 | §5.1.14 ③；🔴 順帶把 `return null` 改成拋錯（§8.2.1 G2） |
| B5 | **`shipping_method_configs` 缺列時 `continue`** | 選項**完全靜默消失**（65 D-13） | 兩個新溫層列漏一個就少一個溫層 | §5.1.14 ④⑤：config 列與 CHECK **同批** |

⚠️ **B1／B2／B3／B4／B5 五條都不是 B2C 的問題，是 B2C 會第一個踩到的問題。**
把它們寫在「B2C 的技術債」欄下有風險——下一個人可能以為不做 B2C 就不用管。
🔴 **B1 尤其**：它是**現在進行式的 PII 紀錄**，與 B2C 上不上線無關。

#### 明確不列為技術債的兩件（避免被當成漏寫）

| 項目 | 為什麼不是債 |
|---|---|
| 沒有平台 payout／subledger／maker-checker／reserve | 🔴 **不適用**（不是延後）：B2C 走 `SELLER_DIRECT`，那三樣屬 `PLATFORM_CONSIGNMENT`（§3.5.5）——而**該值無任何履約方式採用**（65 §3.8b C-1） |
| 沒有 SKU 庫存倉儲模型 | N1：大宗寄倉是「一次交一批包裹」不是寄倉存貨；建它＝多一個沒有寫入者的狀態 |

### 11.2 未決前提與外部窗口
#### 11.2.1 PAYUNi 契約未到齊時的 fail-closed 規格

> 12 題 RFI 的問題本體、窗口與追蹤狀態在 **issue #3498**（純外部依賴，工程無法自助）。
> 本節只定義**未取得書面回覆前系統必須處於什麼狀態**——每一格都是可機械檢查的旗標或欄位值，
> 不是「我們會小心」。

| 約束（RFI 編號沿用全書引用） | 🔴 fail-closed 規格 |
|---|---|
| **RFI-1** 代子商店申請憑證模式 | `credential_mode` 預設 `STORE_OWNED`；`/apply` flag 全關（§3.2.6） |
| **RFI-2** Agent Key 的適用端點範圍 | 沿用 C2C 現行的 transport 形狀，**不改**；有疑義的那一支不啟用 |
| **RFI-3** `/api/logistics/update` 的正式路徑 | E6 **整支不啟用**；要改就走取消後重建（§3.1 註） |
| **RFI-4** 貨態 Notify 的 wire key casing | `@JsonAlias` 三寫法同時吃 ＋ `HashInfo` 一律 `equalsIgnoreCase`（§6.3.1） |
| **RFI-5** 驗收通過／驗退對應的 `ShipStatus` | 走 (b) importer 或 (c) ADM 人工；🔴 **三者皆未就緒前 `931`／`932` 不得出現在任何 shipment 上**（§6A.3） |
| **RFI-6** 同 `MerID` 的服務並存 | 依 §11.2.3：資料層排他（部分唯一索引），並存與否**由 DB 擋住而非由假設決定** |
| **RFI-7** Notify 重送機制與取消回覆時間上界 | 逾時門檻**不填數字**，`992` 只在 24 h 後建 ADM 案件；入站只靠驗簽不靠 IP（§8.5） |
| **RFI-8** `BackSetShipping` 漏收時的重送 | 補償三件（inbox 先落庫／aging 建 case／ADM 人工登錄）——🔴 **人工出口是補償不是修復**（§6A.6） |
| **RFI-9** 正式費率／rate limit／IP allowlist | `shipping_fee_tiers(3004,*).enabled = false`；我方側自訂保守限流；🔴 **不得憑空生一個「上限」series 去算佔比**（§9.3） |
| **RFI-10** 官方申請入口 URL | 🔴 **不虛構「前往申請」URL**；物流頁只顯示指引與狀態（§3.2.3） |
| **RFI-11** 建店資料變更流程 | 唯一路徑＝`CLOSED` 後重新申請；`SetStatus=7` 只讀不寫（我方不主動觸發）（§3.4） |
| **RFI-12** 官方 batch／對帳檔 API | 批次與交接一律**我方內部編排**（§4.1）；對帳檔格式未取得前 §6.3.5 的 (b) 不成立 |

🔴 **每一格都必須是「可機械檢查的狀態」**——一個具體的 flag、欄位值或「整支不啟用」。
這是 §12.0 exit gate 能不能被機械檢查的前提。
🔴 **「sandbox 打得通」不解除任何一格**（§3.2.6 逐字）：它證明的是我們的 client 寫對了，不是我們有權這樣用。

#### 11.2.2 7-ELEVEN 商務前置未完成時的 fail-closed 規格

> 8 項前置的內容與追蹤在 **issue #3498**（窗口是 **7-ELEVEN／統一數網／物流中心**，
> 與 §11.2.1 的 PAYUNi 是**兩批不同的人**；我方與它們之間沒有任何技術連線）。
> 本節只定義前置未完成時系統必須處於什麼狀態。

| # | 前置 | 缺了它擋住哪一格 | 🔴 fail-closed 規格 |
|---|---|---|---|
| 1 | 商家資格核准 | gate ①②③（§0.2） | 🔴 **官方建議的日均件數是適用性建議，不得寫成 provider 的拒絕門檻**（§3.3）——我方不做這個判斷 |
| 2 | 母子廠商代號與退貨代碼 | gate ①（兩欄皆非空才過） | `seller_shipment_settings.partner_id`／`.eship_id` 為空 ⇒ gate 回 `BLOCKED`，建單會被 provider 退成 `AGENT02001` |
| 3 | 指定物流中心 | 批次分組鍵（§5.1.5）、交接地點 | ⚠️ **分組鍵沒有 warehouse 欄** ⇒ 現行假設「一個賣家一個物流中心」；多物流中心時要 expand |
| 4 | 進退貨時段 | §9.3 的硬上限依賴、交接登記的合法性 | 交接時段落在時段外即擋下並提示；**值未取得前不擋，只記指標** |
| 5 | 正式 rate card | 🔴 `shipping_fee_tiers(3004,*)` 不能 seed（§0.5） | `enabled = false`；🔴 **猜一個數字上線＝對賣家收錯錢** |
| 6 | 代收與撥款主體 | §3.5.3～3.5.5 的裁決 | v1.4 只開 `EXTERNAL_PREPAID`（`ServiceType = 3`）；`PROVIDER_COLLECTED` **不開** |
| 7 | 物流費／退貨費的發票開立主體與稅別 | §3.5.1：發票沿用 58 號 `issuer → recipient` | 🔴 **不得由 provider 名稱推論平台是受票人** |
| 8 | 理賠與個資條款 | §8.8 的 retention 年限（🔴 90 天是暫定值）、理賠證據 retention | 條款到手後回頭核對 §8.8；🔴 若爭議窗 > 90 天，**加長保存，不得縮短爭議窗** |

##### 兩件容易被略過的

| 項目 | 為什麼容易略過 | 後果 |
|---|---|---|
| **第一哩方案**（`SELLER_ARRANGED` ／ `PROVIDER_PICKUP`） | 需求書把它寫在 §21.4 的第 4 條，讀起來像選配 | 🔴 冷凍另有統昶「初次運輸（上收）」的最低件數／溫控／設備條件，**未取得書面報價前不得當成預設到府收件**。`first_mile_mode` 是 `shipment_batches` 的**批次級**欄（§5.1.12） |
| **建檔預設「冷凍上收」** | §3.3 第 6 條一句帶過 | 它是**計費口徑**：預設值錯了，冷凍賣家的每一批都會依錯的模式計費，而系統行為完全正常 |

🔴 **上述任一項未完成時，Production seller allowlist 為空**（需求書 §21.4 逐字）。
⚠️ 「本地 fixture 或 sandbox」是唯一允許的活動範圍——這與 §3.2.6 的憑證前置 no-go 是兩條**獨立**的封鎖線
（一條卡 PAYUNi 憑證、一條卡 7-ELEVEN 商務），**解除其中一條不會解除另一條**。

#### 11.2.3 🔴 B2C 與 C2C 的排他性（兩層分開，且都 fail-closed）

排他性有**兩個不同層次**，不得混為一談：

| 層次 | 誰的規則 | 落點 |
|---|---|---|
| **7-ELEVEN 的營運規則**（服務排他性：開通大宗寄倉後不得再用同一服務帳號走門市交寄） | 7-ELEVEN 服務條款 | 需求書 §21.3.9 第 2 條 |
| **PAYUNi 的資料模型**（同一 `MerID` 能不能掛兩個 service product） | PAYUNi API 契約（RFI-6，#3498） | 需求書 §21.5 |

⇒ **「服務不能同時用」與「系統能不能同時存在兩筆設定」是兩件事。**

##### 定案（兩層各自處置）

| 層 | 定案 | 機制 |
|---|---|---|
| 營運排他 | 🔴 **落資料層不落 UI**：同一 `store_id` 不得同時存在 `3651`（C2C）與 `3653`（B2C）的 `onboarding_status = '3602'` 列 | §5.4 的**部分唯一索引** |
| PAYUNi 資料模型 | 🔴 **不假設**：`seller_shipment_settings` 的唯一索引已是 `(store_id, shipping_type, goods_type)`（V1 既有）⇒ 常溫與冷凍**本來就分兩列**，不需要知道 provider 端怎麼存 | 既有索引 |

🔴 **為什麼落資料層**：只寫在文件裡＝下一個人加一個新入口就繞過了（§2.6 逐字）。
⚠️ 而部分唯一索引擋的是**同時 ACTIVE 的兩列**，不擋已轉 `3604 CLOSED` 的舊列——
賣家從 C2C 轉 B2C 是合法路徑（先把 C2C 那列轉 `3604 CLOSED`），索引不會擋住它。

🔴 **RFI-6 的答案不改變我方 schema**，唯一的例外是一種回覆：
若「`PartnerId`／`EshipId` **不**依 `GoodsType` 分開」，則同 `store_id` 同 `shipping_type` 的兩列會存到同一組代號
⇒ 要在 §5.4 加一條「兩列代號必須一致」的 CHECK，否則兩列分岔時沒有人會發現。
⚠️ **其餘回覆都不改設計** ⇒ 不得把「等 RFI-6」寫成整節的 blocker 而讓索引被無限期擱置。

## 12. 部署與維運（需求書 §21.19，只取技術依賴順序）

### 12.0 Production Enablement Gate：商務與規格封口（exit gate：**未被官方文件或書面回覆證明的能力，不得成為 Production default**）

> **本閘門沒有任何程式碼交付物。** 它的產出是**書面回覆**與**一組可機械檢查的旗標狀態**。
> 🔴 這一節存在的理由：把「我們還在等」寫成一個**看得出來的狀態**，
> 而不是散在十幾個章節裡的紅字。

#### 進入條件

無。**封鎖是預設值不是動作**——下表每一格的「封鎖態」就是 repo 的現況，
不需要有人去關，也沒有「什麼時候開始生效」這個問題。

#### 封鎖態（＝可機械檢查的 exit gate；每一格都是一句可執行的查詢）

| # | 檢查 | 封鎖時的值 | 解除條件 |
|---|---|---|---|
| 1 | `shipping_method_configs` 中 `shipping_method='3004'` 的列 | `enabled = false`（兩列皆是） | RFI-9 費率到位 ＋ §11.2.2 第 5 項 |
| 2 | `shipping_fee_tiers` 中 `3004` 的列 | `enabled = false`；🔴 **不 seed 真實金額** | 同上 |
| 3 | `seller_shipment_settings.credential_mode` 的預設 | `'STORE_OWNED'`（DB DEFAULT，§5.1.14） | RFI-1 書面確認支援 `AGENT_PROXY` |
| 4 | `/apply` 端點（A2）的 feature flag | 關 | RFI-1 |
| 5 | Production seller allowlist | **空** | §11.2.2 八項全數完成 |
| 6 | E6 `/api/logistics/update` | 整支不啟用；契約測試只斷言**拒絕** | RFI-3 |
| 7 | `931`／`932` 是否出現在任何 shipment 上 | 🔴 **零列**（§6A.3 第 3 點） | RFI-5，或 importer／ADM 端點就緒 |
| 8 | 「退貨未領停止 checkout」規則 | 整條不啟用（只出指標，§7.7.2） | §11.2.2 第 4 項（退貨週期）到位 |

🔴 **這八格要能被一支腳本查出來**，不是靠人記得。
建議形式：一支 `scripts/b2c-enablement-gate-status.sh`，逐格輸出 `BLOCKED`／`OPEN` ＋ 依據，
在 §12.1 的每一個步驟開始前跑一次。
⚠️ **它是 status 不是 gate**——它不擋 CI，因為封鎖態本來就是預設值；
它要回答的是「我們現在到底解鎖了幾格」，而那件事今天沒有任何地方看得到。

#### exit gate 的判準（逐字，且要能反駁）

> **未被官方文件或書面回覆證明的能力，不得成為 Production default。**

**這句話擋得住什麼、擋不住什麼**（65 §3.10 誠實條款）：

| 情境 | 這句話擋得住嗎 |
|---|---|
| 「sandbox 打得通所以應該可以」 | ✅ 擋得住——§3.2.6 逐字：它證明的是我們的 client 寫對了，不是我們有權這樣用 |
| 「C2C 是這樣所以 B2C 也是」 | ✅ 擋得住——不同 `LgsType`、不同審核路徑、不同服務產品 |
| 「官方頁面上寫著…」 | ⚠️ **看情況**：7-ELEVEN 公開頁面只證明一般營運規則（需求書 §21.22 逐字），不證明我方已取得資格 |
| 「窗口口頭說可以」 | ❌ **擋不住**——「書面」兩個字要當真。⇒ 口頭回覆一律回信確認並存檔，存檔位置寫進 issue |
| 「我們自己量出來的數字」 | ❌ 擋不住，也不該擋——量測是事實。但**量測不能取代授權**（rate limit 量得出來，不代表我們有權用到那個量） |

⚠️ **最後兩列是這句話的邊界，要寫出來**：一道誠實標明邊界的 gate 是資產，被當成牆的是負債。

### 12.1 部署／遷移順序（Deployment / Migration Order，expand-only）

🔴 **前置（不是步驟）**：§12.0 的商務與規格封口（RFI 10 題、7-ELEVEN 商務前置），
exit gate ＝「**未被官方文件或書面回覆證明的能力，不得成為 Production default**」。
它是下表每一步的共同前提，本身沒有程式碼交付物 ⇒ 不佔步號。

🔴 **下表的編號是技術依賴序**：後一步依賴前一步的產出
（碼值 → migration → adapter ＋ 映射 → 事件 → 端點 → canary），顛倒任一格都會壞掉。

| 步 | 內容 | exit gate |
|---|---|---|
| **1** | CodeMeta：`3004` ＋ 7 個 shipment 狀態值 ＋ EventType | `generate.sh --all` 後**四份前端拷貝同 PR**（pre-commit `codemeta-copies` 會擋） |
| **2** | DB migration `V{N}__extend_shipping_type_for_b2c.sql`（🔴 **`{N}` 施工時現讀 `db/migration/` 取下一號，查法見 §5 章首；`ls` 必須帶 `sort -t_ -k1.2 -n`**；檔名與 §5.1.14 一致） | ① 🔴 **不動 `chk_shipping_method`**（65 的 V11 已擴成 5 值，重複 DROP 會刷掉）；② 新表 expand-only；③ **同一個 migration 內完成回填**（新增可由既有欄推導的欄位時，《DB Migration 相容性規範（expand-contract）》） |
| **3** | adapter ＋ 狀態映射 ＋ `ShipmentStatusMapper` 集合（§7.5） | 變異驗證：拿掉 `930` 的防倒退成員 ⇒ 測試轉紅 |
| **4** | MQ：6 個 batch routing key ＋ 36 號 catalog ＋ notification template | 🔴 template 的 `payload_filter` 對得上，否則零投遞 |
| **5** | 端點（A1–E4）＋ Swagger 註解 ＋ 契約 baseline | `gateway-json-contract` Check B 綠；`cd gateway && make swagger` 可生成 |
| **6** | seller allowlist canary | §13.8 release gate 全部同時成立 |

🔴 **關鍵路徑（65 附錄 E 補遺第 7 條逐字 7 節）**：
`PAYUNi 書面確認憑證 → sandbox 建店 → 商店／工具／BackSetShipping 狀態閉環 → 一筆 B2C E2E → 標籤與實體交倉 SOP → 安全／對帳驗收 → seller allowlist canary`

### 12.2 部署順序的結構性風險

🔴 **`3004` 是跨 repo 契約**（與《多媒體與圖片 URL 規範》的 image preset 同一個形狀）：後端送出碼值、前端持有白名單。
**順序固定：後端先上 → 前端才 promote**。反過來 ⇒ 前端拿到未知 `shippingMethod` ⇒ 畫面破。

🔴 **NetworkPolicy／env 三問**（65 §6.1.5）：本書不新增跨服務呼叫 ⇒ 三問皆 N/A；
但若 §12.1 步 3 決定讓 order-service 直呼 shipping 新 RPC，三問必須全部回答，
且**驗證方式不能是「跑一次看有沒有壞」**（fail-open 路徑跑起來一切正常）。

### 12.3 Rollback

關閉新 capability 與 checkout 入口；**不取消、不改路由既有 shipment**；adapter 轉 **query-only**（只允許 E4）。
🔴 已成立的 provider shipment 一律讓它跑完，回滾不得產生第二張單。

### 12.4 Runbook（固定 7 步，不得換序）

停止新 checkout → 保護已成立 shipment → `/query` 對帳／暫停重試 → 對帳批次與款項 →
通知受影響 seller／buyer → 修復後 shadow 驗證 → 分批恢復。

---

### 12.5 Rollback（關閉新 capability／checkout，**不取消或改路由既有 shipment**；adapter 轉 query-only）

**Rollback 原則在 §12.3，本節不重複。** 補 §12.3 沒有寫的三件：**怎麼關、關到什麼程度、關不掉的是什麼**。

#### 三段可分別觸發的關閉（🔴 不是一個開關）

| 段 | 關掉什麼 | 觸發方式 | 影響 |
|---|---|---|---|
| **L1 停新 checkout** | 買家看不到 `3004` 選項 | `shipping_method_configs(3004,*).enabled = false` | 🔴 **最快、影響最小**，且是 §12.0 的封鎖態（第 1 格）⇒ 回到已知狀態不是進入新狀態 |
| **L2 停新建單** | 已下單但尚未取號的走不下去 | C-GATE 強制回 `BLOCKED(PROVIDER_OUTAGE)` | 停在 `91`，可等恢復後續行 |
| **L3 adapter query-only** | 只允許 E4 `/query` | breaker 強制開啟 ／ config flag | 貨態仍收得到（Notify 是入站，不受影響） |

⚠️ **L1 不會讓已成立的 shipment 消失**：§4.7 陷阱 3 講的是 config 缺席會讓選項靜默消失
——那是**上線時**的陷阱；在 rollback 情境下那正是我們要的效果。同一個機制，兩種評價。

#### 關不掉的三樣（🔴 明寫，避免以為 rollback 是全有全無）

| 關不掉 | 為什麼 | 怎麼收斂 |
|---|---|---|
| **已成立的 provider shipment** | 它在 provider 端存在，我方關閉不影響它 | 讓它跑完；🔴 回滾**不得產生第二張單**（§12.3 逐字） |
| **已交倉的實體包裹** | 它在物流中心 | 靠 §12.4 Runbook 的第 2 步「保護已成立 shipment」 |
| **已送出的 `/setting`** | 建店資料不可修改、無撤回端點（§3.4） | 🔴 **無解**——這是 §3.4 ② 要求「逾時或未知不得盲目重送」的真正理由 |

#### 回滾後的驗收（不是「部署成功」）

| 宣稱 | 證據 |
|---|---|
| 「新 checkout 已停」 | 實際打一次 checkout preview，回應中**沒有** `3004` 選項 |
| 「既有 shipment 未受影響」 | 選一筆進行中的，斷言其 `status` 與 `provider_shipments` 列**未變** |
| 「貨態仍在收」 | `hawkr_b2c_inbox_lag_seconds` 仍有 series 且未上升 |

🔴 **「部署成功」與「config 已改」都不能單獨證明回滾完成**——
前者只證明 pod 起來了，後者只證明我方的表變了。

### 12.6 🔴 關鍵路徑（**65 附錄 E 的「補遺」節** 第 7 條，逐字７節）：
`PAYUNi 書面確認憑證 → sandbox 建店 → 商店／工具／BackSetShipping 狀態閉環 → 一筆 B2C E2E → 標籤與實體交倉 SOP → 安全／對帳驗收 → seller allowlist canary`
### 12.7 健康檢查與優雅關閉

🔴 **本書不新增服務**（§4.3）⇒ **健康檢查與關閉行為全部沿用 `shipping-service` 既有配置，本書零修改。**
本節只做兩件事：把既有值寫下來（讓施工者不必去猜），以及指出 B2C 讓哪一格變得重要。

#### 既有配置（實測 `k8s/values/shipping-service.yaml`）

| probe | 路徑 | 語意 | 參數 |
|---|---|---|---|
| readiness | `/actuator/health/readiness` | 任一 indicator（db／redis／rabbit／componentReadiness）DOWN 即失敗 | `initialDelay 30s`／`period 10s`／`timeout 5s`／`failureThreshold 3` |
| liveness | `/actuator/health/liveness` | **僅** `livenessState`，不檢查外部依賴 | `initialDelay 45s`／`period 20s`／`failureThreshold 3` |
| startup | `/actuator/health/readiness` | 冷啟動保護 | `initialDelay 15s`／`period 10s`／`failureThreshold 12`（＝最多 2 分鐘） |

⚠️ **liveness 刻意不看外部依賴**——PAYUNi 掛掉時我方不該重啟自己。
🔴 **B2C 不得改動這一格**：把 provider 可用性接進 liveness ＝ provider 一抖動全部 pod 進 restart loop。
provider 健康屬**指標與 breaker**（§8.2.1 provider health 區），不屬 probe。

#### 優雅關閉：真正要注意的一格

🔴 **不要在 shutdown 時主動釋放租約**（65 §3.10 硬規則 1 逐字）。

| 做法 | 結果 |
|---|---|
| shutdown 時無條件清空 `process_lease_until`／`*_next_at` | 🔴 **把 fail-closed 做成 fail-open**：正在跑一半的 `/trade` 被別的 pod 立刻接手 ⇒ 同一筆訂單兩次取號 |
| 讓租約**自然到期** | ✅ 正解。最壞多等一個租約（K68-1 是 120 s，§7.0） |

⇒ K68-1～K68-9 九支**全部**適用這一條。

#### 部署窗口的兩個 pod

實測 `k8s/values/shipping-service.yaml` **未覆寫** deployment strategy ⇒ 沿用 chart 預設
（`maxSurge: 1`）⇒ 🔴 **每次部署都有兩個 pod 並存**，即使 `replicas: 1`。

| 後果 | 處置 |
|---|---|
| 所有排程器在部署窗口內都是多副本 | 這正是 K68-1～K68-9 一律用 lease 的理由（§7.0） |
| level-triggered 的「單一寫者」不成立 | §7.0 已據此把 K68-6 判成 lease |
| 🔴 「今天 `replicas: 1` 所以沒問題」是錯的 | 65 §7.5 開頭逐字警告：那是擴容那一刻才引爆的地雷 |

### 12.8 Runbook（🔴 **固定 7 步順序**：停止新 checkout → 保護已成立 shipment → query provider／暫停重試 → 對帳批次與款項 → 通知受影響 seller／buyer → 修復後 shadow 驗證 → 分批恢復）

**七步的順序與內容在 §12.4，本節不重複。** 補 §12.4 沒有寫的：**每一步的「做完了」怎麼認**。

🔴 **順序不得換**的理由要寫在這裡，否則第 3 步（暫停重試）會被人提前——
而在停止新 checkout 之前暫停重試，等於讓新單一直進來卻沒有人處理。

| 步 | 動作 | 「做完了」的證據 | 🔴 提前做會怎樣 |
|---|---|---|---|
| 1 | 停止新 checkout | checkout preview 實打一次，回應無 `3004`（§12.5） | — |
| 2 | 保護已成立 shipment | 列出**所有**非終態的 `3004` shipment（一次查詢），並確認沒有任何自動流程會改它們 | 在 1 之前做＝清單一直在長，永遠列不完 |
| 3 | `/query` 對帳／暫停重試 | breaker 強制開 ＋ K68-1／K68-2 停用；`hawkr_b2c_trade_total` 停止成長 | 在 2 之前做＝有些單正在半路，沒人知道它們的狀態 |
| 4 | 對帳批次與款項 | §7.8 五層各跑一次；差異全部落 `reconciliation_cases`（🔴 **不以人工改值消音**） | 在 3 之前做＝一邊對帳一邊還在產生新資料 |
| 5 | 通知受影響 seller／buyer | 受影響清單來自第 2 步的查詢；通知走既有 notification 鏈 | 在 4 之前做＝通知內容會被後續對帳推翻 |
| 6 | 修復後 shadow 驗證 | 連續 mismatch = 0（§13.5 的判準，天數依事故規模另定） | 在 5 之前做＝使用者還不知道發生什麼事就看到狀態變動 |
| 7 | 分批恢復 | 從 seller allowlist 的最小子集開始；每批恢復後重跑第 4 步 | 🔴 **一次全開＝把同一個事故重演一遍** |

#### 觸發這份 Runbook 的條件（不寫的話它永遠不會被啟動）

| 觸發 | 訊號 |
|---|---|
| provider 大範圍失敗 | `hawkr_b2c_trade_total{result!="ok"}` 佔比 > 20% for 10m ／ breaker 開啟 |
| 重複寄件編號 | `hawkr_b2c_duplicate_ship_trade_no_total > 0`（零容忍 G3） |
| PII 疑似外洩 | 任一 §8.6 的契約測試在 prod 對應行為上被推翻 |
| 對帳連續兩日差異未收斂 | `hawkr_b2c_fee_mismatch` 連兩日非 0 |

⚠️ **第 4 列的「連續兩日」是本書初值**——它必須 > 對帳的自然收斂時間（一日一輪），
否則會把正常的一日延遲當成事故。

🔴 **這份 Runbook 要真的演練過才算數**（§13.8 的 release gate 逐字含「rollback 演練」）。
演練的判準是**每一步都產出上表第三欄的證據**，不是「照著唸一遍」。

### 12.9 一次性資料處理的分界

**判準（《後端服務責任邊界規範》 逐字）**：目的是**業務正確性** ⇒ 可考慮寫進 application code；
目的是**資料完整性監測** ⇒ 一律維運 SQL。

| 類別 | 本書的實例 | 處置 |
|---|---|---|
| ✅ **屬 application code** | 批次聚合（`DC_RECEIVED` 由個件推導）；狀態轉換的 allowed-from；退貨與費用的同交易寫入 | 寫進 codebase，走原寫入路徑同交易 |
| ✅ **屬 application code（例外，有理由）** | 交接端點的同交易 outbox（65 §3.9 九 例外 A） | 保 MQ 必發，是規範列出的例外 |
| 🔴 **禁止寫進 application code** | `*ReconciliationJob`／`*BackfillCLI`／drift scanner／`@Scheduled` 的漂移偵測 | 🔴 codebase 拒絕；§5.1.10 已明訂「不建常駐掃描器」 |
| ✅ **維運 SQL（人執行、有計畫、有授權）** | retention 的**第一次**大量清理（§9.5 末段）；`3004` 上線前的既有資料盤點；事故後的一次性補值 | 戴維運帽子主動做，不寫 codebase |
| ✅ **migration SQL** | 新增可由既有欄推導的欄位時的**同檔回填**（《DB Migration 相容性規範（expand-contract）》） | 🔴 與 schema 變更**同一個 migration 檔**內完成 |

#### 本書明確會用到維運 SQL 的三個時機

| 時機 | 內容 | 🔴 為什麼不寫進 code |
|---|---|---|
| retention 第一次清理 | 既有 `payuni_audit_logs` 逾 90 天的 `request_params` 清空並補 hash | 它是**追平歷史**，不是常態；常態由 §9.5 的排程負責 |
| 啟用 `3004` 之前的排他性盤點 | 有多少 `store_id` 已有 `3651` ACTIVE 列（§11.2.3 的排他性影響面） | 一次性查詢，沒有第二次 |
| 事故後補值 | 例：某批 `931` 因 importer 序號跳號而缺（§6.3.5） | 🔴 **必須同交易補一列狀態歷程**（65 §3.11 末段），裸 `UPDATE` 會讓軌跡變成假的 |

#### 維運 SQL 的四個必答（65 §3.11 對「任何會改狀態的維運 SQL」的要求）

| 問 | 為什麼在這裡特別容易錯 |
|---|---|
| ① 我要寫的這個值，有沒有執行者？ | 🔴 把 row 推回非終態 ＝ 手動造一個沒有出口的狀態 |
| ② 推走它的那一句，allowed-from 含不含我造出來的組合？ | 本書有六個新值的 allowed-from 是**本書才補**的（§7.5）——補之前手寫 SQL 造出來的組合，程式裡走不到 |
| ③ 同一交易要不要補一列狀態歷程？ | 不補 ⇒ 下一個人看到的軌跡與當前 status 直接矛盾 |
| ④ 這會不會讓錢或憑證出錯？ | `shipping_fee_ledger` 的 `rate_snapshot` 是不可變的歷史；改它＝改已經對過的帳 |

⚠️ **大量或跨 DB 的操作需要明確計畫與授權**（《後端服務責任邊界規範》）：
本書的 `provider_event_inbox` 與 `payuni_audit_logs` 都可能是百萬列級，
一句沒有 `LIMIT` 的 `UPDATE` 會鎖住整張表，而它同時是 Notify 的落庫目標。

## 13. 測試策略（需求書 §21.20）

### 13.1 分層與落點

| 層 | 落點 | 內容 |
|---|---|---|
| unit | `shipping-service` | 狀態映射表（§7.2）逐列、`isValidTransition` 新集合 |
| contract | `shipping-service` ＋ `payuni-java-client` | 8 支（§13.3） |
| integration | `shipping-service` | 🔴 帶 `-tags integration` 的檔**不會被 `go test ./...` 看到**（Java 側對應：Maven reactor 要跑到） |
| E2E | dev | 一筆完整 B2C（下單→取號→標籤→成批→交接→驗收→到店→取貨） |

### 13.2 🔴 fixture 來源（禁止手寫）

- **有機器可檢的唯一定義者**（proto／gateway DTO＋swagger 註解／CodeMeta）⇒ fixture **從那份定義生**。
- **MQ payload 兩端各自手寫 struct** ⇒ **producer 原始碼就是 de facto 契約**：
  batch 事件的欄名與型別從 `shipping-service` 的 publisher 呼叫點讀出來，
  consumer 側（notification-service template）**不是**契約來源。
- **禁止**自己手寫一份「我覺得對方會發成這樣」的 fixture；禁止照抄註解或設計書範例。
- 🔴 **期望值一律硬寫字面值**，不得呼叫被測函式或它引用的常數（否則改了被測物、測試跟著變，驗不到任何東西）。
- provider 側 fixture＝**sandbox golden payload**；🔴 `/setting` 的 Agent Key 代送在書面確認前
  **只能走核准的 sandbox 路徑**。

### 13.3 契約測試 8 支

`/setting`（E1）／`/trade`（E2）／`/ship_map`（E5）／`/print_label`（E3）／`/query`（E4）／
`/update`（E6，🔴 契約未到齊 ⇒ 該支只斷言 **fail-closed 拒絕**）／Agent Notify／貨態 Notify。

#### 13.3.1 🔴 **fixture 來源**：sandbox golden payload；`/setting` 的 Agent Key 代送未書面確認前**只能走核准的 sandbox 路徑**

**fixture 來源的規則在 §13.2，本節不重複。** 補三件 §13.2 沒有寫的：**golden payload 從哪來、存在哪、什麼時候會過期**。

| 問題 | 答 |
|---|---|
| **從哪來** | 🔴 **實際打一次 sandbox 並錄下 raw bytes**，不是從官方文件的範例抄。文件範例是排版過的，欄位順序與空白都可能不同——而 `HashInfo` 驗簽用的是 raw bytes（§6.3.1） |
| **存在哪** | `services/java/shipping-service/src/test/resources/fixtures/payuni/b2c/`；🔴 **必須先過 PII 遮罩**：sandbox 也可能含真實格式的姓名電話，而 fixture 是永久留在版控裡的 |
| **什麼時候過期** | provider 改版時（`VERSION_*` 常數變動）；⚠️ 沒有東西會通知我們 ⇒ 契約測試要**同時**斷言版本號，版本一變測試就紅 |
| **哪一支不能有 golden payload** | E6 `/update` —— 契約未到齊（RFI-3）⇒ 該支只斷言 **fail-closed 拒絕**（§13.3 逐字），不得為它捏一份「我覺得會長這樣」的回應 |

##### `/setting` 的特殊限制

🔴 **RFI-1 書面確認前，`/setting` 只能走「核准的 sandbox 路徑」**——這句話的具體意思是：

| 允許 | 不允許 |
|---|---|
| 用 sandbox 憑證、對 sandbox 端點、送測試商店資料 | 🔴 用**代理商憑證代任何真實子商店**送件（那正是 RFI-1 在問的權限） |
| 錄下回應當 golden payload | 🔴 拿 production 的一次成功送件當 fixture（它同時是不可撤回的建店申請，§3.4） |

⚠️ **這一格與別支不同的原因**：其他端點打錯了最多是一次失敗的呼叫；
`/setting` 打出去就**產生一筆不可修改、無法撤回的建店申請**（§3.4）。
⇒ 它是本書唯一一支「測試本身就有不可逆副作用」的端點。

##### fixture 的鑑別力自檢（§13.2 的判準代進本節）

| 問 | 本節的答 |
|---|---|
| 這份 fixture 的形狀是誰決定的？ | provider（sandbox 實際回應）⇒ ✅ 合格 |
| 期望值會不會跟著被測物一起變？ | 🔴 **不會**：`HashInfo` 的期望值硬寫字面值，**不得**呼叫我方的簽章函式重算——那樣改壞簽章邏輯測試也會跟著錯 |
| fake 拒絕什麼？ | 驗簽 fake 只接受那一組 golden 的 `HashInfo`，**其餘一律回 reject**；接受任何值的 fake 在這裡零鑑別力 |

### 13.4 冪等與亂序（本章的招牌）

| # | 測試 | 斷言 |
|---|---|---|
| I1 | **相同 idempotency key 重送 100 次** | 🔴 只有 **1** 個 provider shipment、**1** 筆費用、**1** 個 normalized transition。⚠️ 樣本要跨 replica（單 replica 跑 100 次證不到缺陷 6A.5-1） |
| I2 | 貨態 Notify 亂序（`32` 先於 `31` 抵達） | 只產生一個 normalized 結果，且**不倒退** |
| I3 | 重播＋延遲＋跨日 | 同上 |
| I4 | `HashInfo` **大小寫差異** | 同一則被視為重複，不重複處理 |
| I5 | `991`／`990` 的跨服務測試（🔴 **雙向，缺一不算通過**） | **① 退出向**：造一筆已投影 `shipping_status='991'`（或 `'990'`、或 `'91'`）的訂單 ⇒ 斷言 **order-service 的建單兜底不對它動作**（候選 NO）。**② 不退出向**：造一筆 `FL-1` 階段的 `CREATE_FAILED(990)` **失敗殼**（不發任何 lifecycle 事件 ⇒ `orders.shipping_status` 仍為 `NULL`）⇒ 斷言兜底**仍然**把它收進候選並重試（候選 YES，03 §8.11.1 **Case A**）。<br/>🔴 **只驗 ① 會讓「排除 `990` 殼」這個已作廢的錯法照樣全綠**（65 §0.1.6 不等式 ⑥ 的收斂） |
| I6 | batch 聚合單向 | AST gate：聚合函式內出現對 shipment 狀態的 UPDATE ⇒ **轉紅** |

### 13.5 shadow 與實體 dry run

- **shadow**：連續 **14 天** shipment／batch／金額 mismatch 全 0 才進 canary。
- **實體 dry run**：標籤雷射列印**不可縮放**、掃描回讀、重複 barcode、包裹規格（≤10kg／最長邊 ≤45cm／
  長寬高合計 ≤105cm）、派車、進廠、驗收差異、驗退與領退。

### 13.6 鑑別力驗證（每道新 gate 都要做）

1. 動手前 `cp <file> <file>.mut-backup`；2. 注入變異 → 跑 gate → 記紅／綠；
3. **`cp` 還原**（🔴 禁 `git checkout <file>`——它還原到 HEAD，會抹掉未提交的修復）；4. 再跑確認轉綠；
5. 🔴 沒轉紅時三選一：測試弱（缺哪個斷言）／變異無鑑別力（哪條冗餘防線接住，`檔案:符號名`）／
   派工單預期錯（實際由誰守住）。**不准只寫「沒轉紅」**。

### 13.7 測試資料與環境防線

- fixture deterministic、可重跑、**不含真實個資或 secret**。
- dev smoke 產生的資料用可識別前綴，驗證後清理。
- 🔴 動到 product／order／cart／payment／shipping 任一層 ⇒ push 前 **與 merge＋deploy 後各跑一次**
  `infra/smoke-checkout.sh --env=dev`（push 前那次只證明基線沒壞）。

### 13.8 🔴 Release gate

只有在 PAYUNi 書面確認憑證模式、`MemberStatus=1`、`MerStatus=1`、`SPB2C`／`SPB2CFrz=1`、
`BackSetShipping=1`、正式 IP allowlist、sandbox contract tests、物流中心 dry run、五端 UAT、
seller allowlist、PII 遮罩、客服／財務／退貨 SOP 與 rollback 演練**同時存在**時，
才可開啟 Buyer Production 流量。
**PR 合併、部署成功、`SetStatus=1` 或 provider 後台可登入，都不能單獨證明本章完成。**

## 14. 術語表
### 14.1 Buyer 顯示 vs Seller／Admin 顯示（同一件事的兩套說法，含**禁止對買家使用的詞**）

**四個核心對照在 §2.5，本節不重複。** 本節把它擴成完整詞表，並把禁詞寫成可檢查的清單。

| 概念 | Buyer 顯示 | Seller／Admin 顯示 | 後端識別字 |
|---|---|---|---|
| 配送方式 | **「7-ELEVEN 門市取貨」** | 7-11 大宗寄倉（B2C） | `shipping_method = 3004` |
| 交倉 | 「賣家已出貨」 | 交倉／進倉批次 | batch `DISPATCHED`、shipment `930` |
| 物流中心驗收 | 「配送中」 | DC 驗收通過／驗退 | `931` ／ `932` |
| 廠商代號 | **不顯示** | 母子廠商代號 | `partner_id` ／ `eship_id` |
| 取號 | **不顯示** | 取得寄件編號 | `ship_trade_no` |
| 標籤 | **不顯示** | 出貨單／標籤 | `label_object_key`、`label_sha256` |
| 批次 | **不顯示** | 進倉批次 | `batch_no` |
| 未取退回 | 「已退回」 | 刷退／未取退回 | `52` → `53` |
| 驗退 | 「配送異常，客服將與您聯繫」 | 驗退（DC_REJECTED） | `932` |
| 理賠 | 「處理中」＋「**依條款審核，最高上限**」 | 理賠案件 | `993` → `43` |

#### 🔴 禁止對買家使用的詞（可機械檢查的清單）

```
大宗寄倉  母子代號  母子廠商代號  物流中心  B2C  批次  進倉批次
驗退      刷退      寄件編號      配編編號  PartnerId  EshipId  MerID
```

**判準不是「會不會洩漏」，是「買家對這些詞沒有可執行的動作」**（§2.5 逐字）——
顯示它只製造客服工單。

**落地兩件（缺一不可，§2.5 已定，此處寫成檢查形式）**：

| 件 | 內容 | 🔴 為什麼單獨一件不夠 |
|---|---|---|
| ① 逐欄白名單 | 買家向 DTO 逐欄明列（同 65 §3.9 五） | — |
| ② 契約測試斷言**欄位集合 ＝ 白名單** | 不是只斷言「禁詞不在回應」 | 只擋禁詞 ⇒ 上游 DTO 長胖時**新欄位靜默流出**，而測試全綠 |

⚠️ **禁詞清單本身也要被測**：把上面那個清單寫成常數，測試對每一個買家端回應做子字串比對。
🔴 對「清單被清空」要 **FAIL 不是 SKIP**——斷言清單長度 > 0，否則有人把它清空之後測試會全綠。

### 14.2 provider 專有名詞（大宗寄倉／進倉批次／驗退／刷退未取退回／母子廠商代號／物流中心）

| 詞 | 定義 | 🔴 常見誤解 |
|---|---|---|
| **大宗寄倉** | B2C 商家把**已成立訂單的包裹**集中送至指定 7-ELEVEN 物流中心，驗收後配送至買家指定門市 | 🔴 **不是 3PL 倉儲**：不是把 SKU 庫存長期寄放在 7-ELEVEN（需求書 §21.1【已驗證】逐字）⇒ `inventory_storage_mode = NONE`（N1） |
| **進倉批次** | 同一賣家、出貨據點、物流中心、溫層與預定交貨窗口下的一組包裹 | 🔴 **provider 端沒有這個東西**（§4.1）：它 100% 是我方內部編排。它不是訂單批次、不是付款批次、不是庫存批次 |
| **驗退** | 物流中心因規格、標籤、數量或時限不符而**拒收** | 🔴 驗退 ≠ 買家退貨；也 ≠ 退款（INV-7）。它發生在包裹**還沒送到買家手上**之前 |
| **刷退／未取退回** | 包裹到店後買家逾期未取，由物流體系退回物流中心 | 🔴 與「驗退」的差別是**時點**：驗退在進倉、刷退在到店之後 |
| **母子廠商代號** | `PartnerId`（母，3 碼）＋ `EshipId`（子）；B2C 的配編編號＝`PartnerId`(3)+`Odno`(8)＝**11 碼** | 🔴 C2C 是 **12 碼**（§0.1 ⑨）⇒ 長度判斷不可共用 |
| **物流中心（DC）** | 7-ELEVEN 指定的收貨與驗收據點 | 🔴 **我方與它之間沒有任何技術連線**（§2.1）；它是商務關係 |
| **服務產品（service product）** | `SPB2C`（常溫）／`SPB2CFrz`（冷凍），PayUni 的開通粒度 | 🔴 **兩者完全分開**：常溫過了不代表冷凍過了（§3.3 第 2 條）⇒ 產生 `PARTIALLY_ACTIVE` 前端態 |
| **`BackSetShipping`** | 商店端物流設定完成的旗標；`1` 開啟、`2` 關閉 | 🔴 **只能從 Agent Notify（`NotifyType=BackSetting`）取得，`search_merchant_status` 不回它**（§3.2.5，全 repo 零命中已驗） |
| **C2B 退貨便** | B2C 常溫開通時**同時**開通的逆向能力 | 🔴 **副作用非選項**（§0.1 ⑪）：不是我方開的，不得當 feature 宣傳；**冷凍沒有**（§7.7.1） |

⚠️ **「大宗」兩個字只發生在批次進倉那一段**（需求書 §21.1 逐字）：
一張訂單仍然對應**一件**包裹與**一個** provider 寄件編號（A1）。
把「大宗」讀成「多件合併成一單」是本書最容易的一個誤解，而它會直接導致 `provider_shipments` 的唯一鍵設計錯誤。

### 14.3 同一件事的多個名字與值域（`SetStatus` 7 值 ／ 前端 9 值 ／ `SPB2C` 7 值 ／ shipment 31 值 ／ batch 兩套）

#### 五套值域各自的權威與大小

| 值域 | 值數 | 權威 | 我方能不能改 |
|---|---|---|---|
| provider `SetStatus` | **7**（`1` 正常／`2` 審查失敗／`3` 關閉／`4` 永久關閉／`7` 重新送審／`8` 已上傳審核中／`9` 未處理，**跳號無 `5`／`6`**） | PayUni | ❌ 照抄不改（65 §3.9 四） |
| provider `SPB2C`／`SPB2CFrz` | **7**（`1` 開啟／`2` 未開啟／`3` 未申請／`4` 強制關閉／`9` 審核中／`92` 審核未過／`99` 其他關閉） | PayUni | ❌ |
| 我方 `onboarding_status` | **5**（`3601` 待審核／`3602` 已啟用／`3603` 審核不通過／`3604` 已關閉／`3605` 永久關閉） | 我方 CodeMeta | ✅ |
| 前端正規化態 | **9 ＋ `LOADING`** | 我方（§0.3） | ✅ |
| normalized shipment | **31**（既有 24 ＋ 65 新增 7） | 🔴 **65** | ❌ 本書只映射（§7.2） |
| batch | **6** | 65 §5.2.2 `chk_batch_status` | ❌ |

#### 🔴 前端 9 值有**兩套**，而它們不是同一套

需求書 §21.3.5／§21.5 給了一套 9 值，§0.3 給了另一套 9 值。**逐值對照**：

| 需求書的值 | §0.3 的值 | 是否一對一 |
|---|---|---|
| `NOT_ELIGIBLE` | 🔴 **無對應** | ❌ §0.3 把「資格」拆到 `disabledReasonCode` 維度（`SELLER_SUSPENDED` 等），不佔一個狀態 |
| `NOT_APPLIED` | `NOT_APPLIED` | ✅ |
| — | `SUBMITTING` | 🔴 §0.3 **多**一個（`payuni_submit_status = 4302 REQUESTED`） |
| `PENDING_REVIEW` | `PENDING_REVIEW` | ✅ |
| `ACTION_REQUIRED` | 🔴 **無對應** | ❌ §0.3 用 `REJECTED` ＋ `canResubmit = true` 表達 |
| `REJECTED` | `REJECTED` | ✅ |
| — | `PARTIALLY_ACTIVE` | 🔴 §0.3 **多**一個（B2C 專屬：常溫與冷凍只有一邊過，C2C 不會出現） |
| `ACTIVE_ENABLED` | `ACTIVE` | ✅ |
| `ACTIVE_DISABLED` | `DISABLED_BY_SELLER` | ✅（改名） |
| `SUSPENDED` | `CLOSED` ／ `PERMANENTLY_CLOSED` | ⚠️ 一對二（`3604` 可逆、`3605` **不可逆**，§3.4） |
| `PROVIDER_UNKNOWN` | 🔴 **無對應** | ❌ **見下，這是一個缺口不是設計選擇** |

**定案：以 §0.3 那一套為準**（它把「狀態」與「不可用原因」拆成兩個正交維度，
而需求書那套把兩者壓成一個列舉 ⇒ 後者在「已啟用但本次不可用」的情境下表達不出來）。

#### 🔴 `PROVIDER_UNKNOWN` 的落點

| 事實 | 出處 |
|---|---|
| gate 有三態：`ELIGIBLE`／`BLOCKED(reasonCode)`／**`UNKNOWN`** | §0.2 |
| §0.2 逐字要求：`UNKNOWN`「當 `BLOCKED` 處理但 **reasonCode 不同**」 | 同上 |
| 🔴 `PROVIDER_OUTAGE` 是「provider 掛了」，**不是**「我方讀取失敗」 | §6.1.7 |
| 需求書 §21.16 要求 `PROVIDER_UNKNOWN` 有專屬 UI（只顯示重試，不開放申請／切換／交倉） | §21.16 逐字 |

⇒ **`disabledReasonCode` 共 12 值**，第 12 個 `PROVIDER_STATUS_UNKNOWN` 承接 gate 的 `UNKNOWN` 三態
（§6.1.7／§6.1.8 ①／附錄 C.4 ④ 三處同為 12 值）。
🔴 缺它則 §0.2 的「reasonCode 不同」在實作上無法成立，而塌縮成 `PROVIDER_OUTAGE`
會讓「真的沒開通」「provider 掛了」「我們讀不到」三件事在營運端長得一樣。

#### batch 的兩套（§7.3.3 已收斂，此處只列對照）

| §21.11（領域態，**權威**） | §21.16（UI 態，8 個中文值） | 說明 |
|---|---|---|
| — | 草稿／檢核中／建立中 | 🔴 **純前端態**，後端不得回傳（同 `LOADING`） |
| `OPEN` | 部分完成 | 逐筆取號可部分成功（§7.1.1 ①） |
| `CLOSED` | 待交倉 | — |
| `DISPATCHED` | 已交倉 | — |
| `DC_RECEIVED` ／ `RECONCILED` | （無對應） | UI 那套沒有區分驗收與對帳完成 |
| `RECONCILING` | 對帳異常 | — |
| — | 失效 | 🔴 領域態沒有這個值；若指「批次被作廢」，那是缺一條轉換邊，不是缺一個狀態 |

## 附錄 C：CodeMeta 與碼表新增清單
### C.1 生成流程（→ 65 §七）

**流程本體在 65 §3.9 七，本書不重述。** 本節只寫 68 的**執行順序**與**本書的邊界**。

🔴 **`.md` 與生成檔必須同批**：`static-gates.yml` 會重跑 `generate.sh --all` 並逐位元對拍
⇒ **只改 `.md` 不重新生成，會擋掉每一個 PR**（不只本書的）。本附錄 C.2～C.5 是本書用到的碼值清單。

**CodeMeta 變更的執行順序（不得跳步）**：

```
① 改 docs/代碼管理（CodeMeta）/*.md
      業務類型碼.md  → ShippingMethod 3004、ShippingType 3653（值域已在 SSOT，只在需要增修時動）
      內部技術碼.md  → EventType 6 個成員（§6.4.5）
      錯誤碼.md      → 本書業務碼（🔴 號碼由 65 §6.7 統一指派，見 C.4）
      ↓
② cd docs/代碼管理（CodeMeta）/scripts && ./generate.sh --all
      🔴 一律 --all，不要自己判斷「這個 enum 前端看不看得到」
      ↓
③ 同一個 PR 內同步四份前端拷貝（pre-commit 的 codemeta-copies 會擋）
      ↓
④ 編譯驗證（Java reactor ＋ Go 全模組）
```

⚠️ **嚴禁**直接改生成檔（`ShippingMethod.java`、各 CodeMeta Enum、`pkg/errors/bizcode/bizcode.go`）
——下次 generate 會覆蓋，曾造成執行期 crash。

⚠️ **四份拷貝的比對嚴格度不同**（65 §3.9 七）：website／ADM **逐位元**、
兩個 Flutter **只比對 `(code, value, label)`** ⇒ 只改 description 時 Flutter 的 gate 會過而拷貝其實已不同步。
🔴 **不要拿 Flutter 的綠燈推論 website／ADM 也同步了。**

⚠️ **ADM 的拷貝走 `--admin`，enum 由 `generate.sh` 的 `ADMIN_ENUMS` 陣列決定**；
65 §3.9 七實測「現況缺 `ShippingType`」⇒ 本書要用 `ShippingType.CVS_B2C(3653)` 就得**先擴那個陣列**。
這一格漏了不會編譯錯，只會讓 ADM 拿不到新值。

### C.2 🔴 `3004 SEVEN_ELEVEN_BULK_WAREHOUSE`
（🔴 **命名以 65 附錄 C.1 為準**：同一個 `ShippingMethod` enum 內既有 `3002` 是
`SEVEN_ELEVEN_STORE_PICKUP`，命名要一致。**不得**用 `CVS_711` 前綴——它會強化「7-ELEVEN 是獨立
provider」的誤讀，而本書 §3.1 前的宣告正是要消除它）
（🔴 **`3002` 明令不得重用**；**`3005` 不可用**——已由 69 號《商家自有出貨》的 `MERCHANT_DELIVERY`
佔用（同一個 `ShippingMethod` enum））
### C.3 `service_product`／`shipping_family`／`collection_model`／`commerce_model`／`first_mile_mode` 值域

> 🔴 **五個裡面只有兩個要進 CodeMeta**，其餘三個各有理由不進。逐項寫出來，
> 否則下一個人會把五個都建成 enum，而其中三個沒有第二個值或沒有寫入者。

| # | 名稱 | 值域 | 進 CodeMeta？ | 落點與理由 |
|---|---|---|---|---|
| 1 | **`service_product`** | `SPB2C`（常溫）／`SPB2CFrz`（冷凍） | ❌ **不進** | 🔴 它是 **provider 原生值**（§C.5）。落點＝`seller_shipment_settings.provider_status` 那一列所代表的溫層，由 `(shipping_type, goods_type)` 唯一索引承載 ⇒ **不需要第三個欄位** |
| 2 | **`shipping_family`** | ＝ `ShippingType`：`3651 CVS_C2C`／`3652 HOME_DELIVERY`／**`3653 CVS_B2C` 🆕**／**`3654 MERCHANT_DELIVERY` 🆕（69 的，非本書）** | ✅ **進**（本書用 `3653`；🔴 **值域由 65 附錄 C.1.1／C.2 定案，本書只引用**） | `seller_shipment_settings.shipping_type`；🔴 它是 §5.1.14 撞號問題的唯一正解（`buildSlotKey` 第一段不同就不會撞） |
| 3 | **`collection_model`** | `EXTERNAL_PREPAID`（v1.4 唯一開放）／`PROVIDER_COLLECTED`（**不開**） | ⚠️ **不進** | 🔴 v1.4 只有**一個**可用值 ⇒ 建一個單值 enum 等於造一個沒有分支的維度。落點＝`provider_shipments` 的 `service_type` 快照（`3 = 取貨不付款`，既有 `ServiceType`）。開第二個值時再建 |
| 4 | **`commerce_model`** | `SELLER_DIRECT`（B2C 固定） | ❌ **不進（本書）** | 🔴 它是 **65 的值域**（65 §0.6／§0.6.2），不是 68 的。本書只**引用**不定義；`PLATFORM_CONSIGNMENT` 的東西一件都不做（§3.5.5） |
| 5 | **`first_mile_mode`** | `SELLER_ARRANGED`／`PROVIDER_PICKUP` | ⚠️ **待商務前置** | 落點＝`shipment_batches` 的一欄（**批次級**不是包裹級，§5.1.12）。🔴 值域取決於 §11.2.2 第 4 條的「第一哩方案」書面確認；未取得前只寫 `SELLER_ARRANGED` 且**不建 enum** |

🔴 **「不建 enum」不等於「用裸字串」**：第 3、5 項以 **DB CHECK 約束**限制值域
（`chk_first_mile_mode` 等），程式側用常數。CHECK 是 DB 層的白名單，
比一個只有一個成員的 CodeMeta enum 更誠實——它擋得住寫入，而不假裝那裡有一個值域決策。

⚠️ **第 1 項最容易被誤建**：`service_product` 讀起來很像一個維度，
但本 repo 的唯一索引 `(store_id, shipping_type, goods_type)` 已經表達它了
（`3653` × `3201` ＝ `SPB2C`；`3653` × `3203` ＝ `SPB2CFrz`）
⇒ 再建一欄就是**同一個事實兩個真相**，而它們會分岔。

### C.4 shipment 與 batch 狀態碼、事件 EventType、`disabledReasonCode` 12 值、錯誤碼佔用清單

#### ① shipment 狀態碼

🔴 **本書不新增任何 `ShippingStatus` 值**（§7.2 逐字）。用到的 7 個新值（`930`／`931`／`932`／`991`／`992`／`993` ＋ `933` 不用）
由 **65 §7.1.2.1** 定義並落 CodeMeta；本書只做映射。

**本書要求 65 確認的一格**（§7.3.2 X8）：`RETURN_COLLECTED`（賣家已領回退貨）
**在 31 值裡沒有對應者**，本書處置為「不落 shipment 軸、落 `shipment_returns.outcome`」。
若 65 決定補一個 normalized 值，§7.2 的映射表與 §7.3.2 要同批改。

#### ② batch 狀態碼

`OPEN`／`CLOSED`／`DISPATCHED`／`DC_RECEIVED`／`RECONCILING`／`RECONCILED`
🔴 值域由 **65 §5.2.2 `chk_batch_status`** 定死，本書不新增、不改名。
§21.16 的 8 個中文值是 **UI 態**，不進值域（§14.3 末表）。

#### ③ EventType（`內部技術碼.md`）

6 個成員，逐條在 §6.4.5。🔴 **物流是唯一沒有 EventType enum 的大領域**（65 §3.9 八）
⇒ enum 本身由 65 建立，本書只登記成員。

#### ④ `disabledReasonCode`

**§6.1.7 與本表同為 12 值**（第 12 個是 `PROVIDER_STATUS_UNKNOWN`，語意見 §14.3）。

| # | 值 | 語意 |
|---|---|---|
| 1 | `NOT_ONBOARDED` | 沒有 `seller_shipment_settings` 列 |
| 2 | `ONBOARDING_PENDING` | `3601` 待審核 |
| 3 | `ONBOARDING_REJECTED` | `3603` 審核不通過 |
| 4 | `BACK_SET_SHIPPING_MISSING` | gate ⑦ 未滿足（§3.2.5） |
| 5 | `SKU_NOT_ELIGIBLE` | 商品未通過承載檢查 |
| 6 | `PACKAGE_OVER_SPEC` | 重量／尺寸超規（§0.1 ①②③） |
| 7 | `TEMPERATURE_NOT_SUPPORTED` | 冷藏（`3202`）或該溫層未開通 |
| 8 | `REGION_NOT_SUPPORTED` | 門市／地區不支援 |
| 9 | `PROVIDER_OUTAGE` | provider 服務中斷（breaker 開啟） |
| 10 | `SELLER_SUSPENDED` | 賣家停權（《賣家能力權限層規範》） |
| 11 | `RETURN_BACKLOG_EXCEEDED` | 退貨未領超過門檻（§7.7.2，值未定前不啟用） |
| **12** 🆕 | **`PROVIDER_STATUS_UNKNOWN`** | 🔴 **本節新增**：gate 回 `UNKNOWN`（跨服務讀取失敗／快照過期）。§0.2 逐字要求它的 reasonCode 與 `BLOCKED` **不同**，而 1～11 沒有任何一個表達「我們讀不到」 |

🔴 **第 12 個不是可選的**：沒有它，§0.2 的三態設計在實作上會塌縮成兩態，
而「真的沒開通」「provider 掛了」「我們讀不到」在營運端長得一樣——那正是 §0.2 要避免的事。

#### ⑤ 錯誤碼佔用

🔴 **本書不得自己撿號**（65 附錄 C.5.1 明訂）：號碼由 **65 §6.7 統一指派**。
本書只登記**需要幾個**與**每個的觸發條件**。

| 狀態 | 內容 |
|---|---|
| 需要量 | **≈ 35 個**（§0.6） |
| 已知沿用（65 提案） | `9520` `SHIPMENT_STATE_TRANSITION_REJECTED`／`9521` `SHIPMENT_CANCEL_ALREADY_PENDING`／`9522` `SHIPMENT_CANCEL_NOT_CANCELLABLE` |
| 本書新增（名稱已定、號碼未定） | `B2C_ONBOARDING_NOT_ACTIVE`／`B2C_BATCH_NOT_OPEN`／`B2C_LABEL_SHIP_DATE_INVALID`／`B2C_LABEL_GROUP_MIXED`／`B2C_UPDATE_AFTER_PRINT`（§6.1.6）＋ 其餘依施工時的端點盤點補齊 |
| 號段 | 🔴 **65 §6.7.0 已定版 `9500~9699`（200 個）**；65 佔 `9501~9575`，本書自 `9576` 起由 65 連號追加。**取號是 65 的動作不是本書的** |

⚠️ **需要 `bizHTTPStatus` 登錄的碼要標出來**（65 §6.7.2）：未登錄的一律塌縮成 HTTP 409
⇒ 前端拿到 409 只能去解析 message 裡的 `[NNNN]`。本書的碼**全部**走 HTTP 200 ＋ 業務碼（D-16），
⇒ 每一個都要登錄，漏一個就會變成 409。

### C.5 provider 原生值（`SetStatus`／`SPB2C`／`BackSetShipping`）**刻意不進 CodeMeta** 的理由

**判準一句話**：CodeMeta 是**我方共享代碼值**的 SSOT；provider 原生值不是我方的碼，
把它放進去等於宣稱我方對它有定義權——而我方沒有。

| provider 原生值 | 值域 | 為什麼不進 CodeMeta | 落在哪 |
|---|---|---|---|
| `SetStatus` | 7 值（跳號無 `5`／`6`，§14.3） | 🔴 **我方無法保證窮舉**：跳號本身就是證據——`5`／`6` 可能存在只是沒文件化。CodeMeta enum 的 `fromValue` 對未知值會拋錯，而**那正是我們要的行為**；但把它做成 enum 就等於宣稱這 7 個是全部 | `seller_shipment_settings.provider_status`（原字串照存）＋ §0.3 的映射表 |
| `SPB2C`／`SPB2CFrz` | 7 值 | 同上；且它是**服務產品的開通狀態**，我方的對應概念是 `onboarding_status`（5 值，我方定義） | 同上 |
| `BackSetShipping` | 2 值（`1` 開啟／`2` 關閉） | 🔴 只有兩個值且**只能從 push 取得**（§3.2.5）⇒ 我方要的不是「值」而是「**有沒有收到過**」 | `seller_shipment_settings.back_set_shipping_at`（**時間戳不是布林**） |
| PayUni 貨態 `ShipStatus` | 官方 21 碼 | ⚠️ **例外**：它**已經**在 CodeMeta 裡（`ShippingStatus` 的 value 直接對應 PayUni 狀態碼，實測 `ShipmentStatusMapper` 的 javadoc） | 既有，本書不改 |

#### 🔴 `back_set_shipping_at` 為什麼是時間戳不是布林

| 存法 | 能回答的問題 |
|---|---|
| `back_set_shipping BOOLEAN` | 「現在是不是開的」 |
| **`back_set_shipping_at TIMESTAMPTZ`** | 「現在是不是開的」（非 NULL 即是）**＋「什麼時候收到的」＋「收到多久了」** |

而 §6A.6 的補償設計要的正是第三個問題——
**「開通已 `APPROVED` 但 `BackSetShipping` 未收到、逾 24 h」的 aging**，
布林欄答不出來。這與 §8.2.3「述詞錨在時間不錨在狀態集合」是同一條紀律。

#### 最後一列的例外要說清楚，否則它看起來像自相矛盾

`ShippingStatus` 的值**就是** PayUni 的狀態碼，這是既有設計、本書不改。
但它與上面三個的差別是：

| 面向 | `ShippingStatus`（已在 CodeMeta） | `SetStatus` 等（不進） |
|---|---|---|
| 我方有沒有加值 | ✅ 有：加了 `210`／`920`／`921`／`990` 等**我方自己的**狀態，並定義了轉換規則 | ❌ 沒有：純粹是讀回來顯示 |
| 未知值怎麼辦 | 🔴 `fromValue` 拋錯（§4.5 逐字「表外的碼一律拋錯不得靜默」） | 原字串照存 ＋ 映射表查不到就走 `UNKNOWN` 分支 |
| 它是誰的軸 | **我方的** normalized 軸（65 §7.1.2） | provider 的軸 |

⇒ 判準不是「是不是 provider 給的」，而是「**我方有沒有在這個值域上做決定**」。
有 ⇒ 它是我方的碼，進 CodeMeta；沒有 ⇒ 照存不定義。

## 附錄 D：前端契約與交付對照（五端 ↔ 端點 ↔ 路由 ↔ 檔案；畫面細規 → 65A）

> **本附錄只做對照，不做畫面設計**（畫面細規 → 65A §2.7／§4.7／65A §6.8，見 65 附錄 E 第 2167 條）。
> 目的是回答一個問題：**「一個真實使用者，從哪個畫面、按哪顆按鈕，會用到這個端點？」**
> 🔴 答不出具體畫面＋按鈕的端點，就是還沒交付完（後端加了欄位＋CI 綠＋部署成功都不算）。

### 五端與 repo（實測 `ls`，非背誦）

| 端 | repo | 交付路徑 | 把關者 |
|---|---|---|---|
| Buyer Web | `hawkr-website` | 直接 push `dev`（禁開分支） | CI |
| Buyer App | `hawkr_buyer`（Flutter） | 直接 push `dev` | 本地驗證 ＋ Xcode Cloud |
| Seller Web | `hawkr-website`（同一個 repo，`app/(seller)/seller-center/*`） | 同上 | CI |
| Seller App | `hawkr_seller`（Flutter） | 直接 push `dev` | 本地驗證 ＋ Xcode Cloud |
| Admin | `hawkr-admin`（`src/app/admin/*`） | 直接 push `dev` | CI |

### 端點 ↔ 畫面 ↔ 路由 ↔ 檔案

| 端點（§6.1.4） | 端 | 畫面／按鈕 | 路由 | 檔案（實測現況） |
|---|---|---|---|---|
| A1 `GET /app/shipping_onboardings/cvs_b2c` | Seller Web | 物流設定頁的「大宗寄倉」卡片載入 | `/seller-center/me/shipping` | ✅ **已存在**：`app/(seller)/seller-center/me/shipping/page.tsx`（現況只有 C2C）⇒ **擴充不新建** |
| A2 `POST .../cvs_b2c/apply` | Seller Web | 同頁「申請開通大宗寄倉」CTA（送出前二次確認不可變資料） | 同上 | 同上 |
| A3 `POST .../:onboarding_id/toggle`〔既有〕 | Seller Web | 同頁本地開關 | 同上 | 同上；🔴 **本書不改端點**，只改後端對 `3004` 列的處理 |
| B1 `POST /app/shipment_batches` | Seller Web | 批次頁「建立批次」 | `/seller-center/shipments/batches` | 🔴 **不存在**：`app/(seller)/seller-center/shipments/` 目前只有 `anomalies` ⇒ **新建** |
| B2 `GET /app/shipment_batches` | Seller Web | 批次列表 | 同上 | 同上 |
| B3 `GET /app/shipment_batches/:batch_no` | Seller Web | 批次明細 | `/seller-center/shipments/batches/[batchNo]` | 同上 |
| B4 `POST .../close` | Seller Web | 明細頁「關閉批次」 | 同上 | 同上 |
| B5 `POST .../labels` | Seller Web | 明細頁「列印標籤」（🔴 顯示 N 份而非失敗，§7.1.3） | 同上 | 同上 |
| B6 `POST .../handoff` | Seller Web ＋ **Seller App** | 「司機交接確認」（🔴 `930` 的唯一推進者） | 同上 ／ App 批次頁 | Web 同上；App **不存在**：`lib/features/` 現況無 `shipment_batch`（只有 `shipment_anomaly`，實測） |
| B7 `POST .../items/scan` | **Seller App**（掃碼） | 掃描綁定包裹 | App 批次頁 | 🔴 **App 才有實際價值**（要用相機）；Web 端可用手動輸入備援 |
| B8 `GET .../handoff_options` | Seller Web ＋ App | 建立批次時的時段選單 | 同 B1 | 同上 |
| C1 `POST /app/shipments/:order_no/b2c_label` | Seller Web | 訂單明細「列印標籤」 | `/seller-center/orders/[orderNo]` | ✅ 路由已存在（`seller-center/orders`，實測）⇒ 加按鈕 |
| C2 `POST .../cancel` | Seller Web | 訂單明細「申請取消」 | 同上 | 同上 |
| C3 `POST .../resubmit` | Seller Web | 工作台「重新送出」（🔴 **只在 `990`**；`991` **不得**出現這顆按鈕，改顯示唯讀「確認中」，見 §6.1.4 C3 與 §7.2 X1） | 同 D1 | 隨 D1 新建 |
| C4 `POST .../dispatch`（＝ 65 `S-10`） 🆕 | Seller Web | 工作台／訂單明細「取號」（`91` 唯一的推進動作） | 同 D1 | 隨 D1 新建 |
| D1 `GET /app/shipments/b2c_worklists/:worklist` | Seller Web ＋ App | **9 個工作台**（待取號／待列印／待包裝／待成批／批次差異／驗退／未取退回／待領退貨／對帳異常） | `/seller-center/shipments/worklists/[worklist]` | 🔴 **不存在** ⇒ **新建** |
| D2 `GET /app/shipments/:order_no/fees` | Seller Web | 訂單明細「費用」分頁（預估／已入帳／待對帳／已對帳四段） | 同 C1 | 加分頁 |
| E1～E4 `/admin/*` | Admin | control tower ／ 建 case ／ 重跑 importer ／ 重新 query | `/admin/shipments/batches` 等 | 🔴 **不存在**：`hawkr-admin/src/app/admin/` 無任何 shipping 頁 ⇒ **新建** |
| E5 `POST /admin/shipments/:id/dc_result` | Admin | 配送單明細的「登錄驗收結果」（通過／驗退＋原因） | `/admin/shipments/[id]` | 🔴 **端點與畫面都不存在** ⇒ **一起新建**。**沒有它 `931`／`932` 在 v1.4 沒有入邊**（§6A.3 第 3 點） |
| E6 `POST /admin/reconciliation_cases/:case_id/close` | Admin | 差異 case 的處置與關閉 | `/admin/shipments/reconciliation_cases/[caseId]` | 同上；🔴 **它是 batch `RECONCILING` 的唯一出邊執行者**（§7.3） |
| E7 `POST /admin/shipping_onboardings/:onboarding_id/back_set_shipping` | Admin | 開通明細的「人工登錄 BackSetShipping」 | `/admin/shipping_onboardings/[onboardingId]` | 同上；🔴 漏一則 Agent Notify ⇒ 該賣家永久建不了單且無 API 可自救（§6A.6） |
| E8 `POST /admin/shipments/:id/return_outcome` | Admin | 配送單明細的「退貨結案」（領回／棄置／已賠償） | `/admin/shipments/[id]` | 同上；🔴 沒有它退貨軸永遠結不掉，`hawkr_b2c_return_open_aging_seconds` 會對每一筆已領回的退貨永久告警 |

### Buyer 端：**零新端點、零新畫面**

| 事實 | 說明 |
|---|---|
| 買家仍只有**一張** 7-ELEVEN 門市取貨卡 | 需求書 §21.1【產品決策】逐字「不增加第五張卡」 |
| 門市選擇器沿用既有 `/ship_map` 流程 | §6.1.3 E5：B2C 不改 |
| 訂單追蹤顯示 6 階段 | §7.7 的映射表；🔴 **禁詞清單見 §14.1** |
| ⇒ Buyer Web／App 的工作 | **只有文案與階段映射**，沒有新端點 |

🔴 **這一格是本附錄最重要的結論**：v1.4A 交付後**買家端零變化**（§2.2.1 逐字）
——因為 `shipping_method_configs(3004,*).enabled = false` ⇒ checkout 不出現該選項。
⇒ 驗收 v1.4A 時**不要去買家端找變化**，那裡本來就不該有。

### 交付前的機械檢查（65 §6.0 與附錄 D 的兩條，套用到本附錄）

| 檢查 | 做法 |
|---|---|
| 「誰消費它」 | 每個新端點都要給出 caller 的 `檔案:行號`，或明說「本書不接，缺口是 X」。🔴 拿路由字串去**所有**前端 repo grep，**零命中＝未交付** |
| 「哪個畫面、哪顆按鈕」 | 上表第三欄。⚠️ 最難察覺的形狀是**畫面有、按鈕沒接上** ⇒ 問的是「按哪顆按鈕」不是「有沒有這個元件」 |

⚠️ **跨 repo 部署順序**（§12.2）：`3004` 是後端送出的碼值、前端持有白名單
⇒ **後端先上 → 前端才 promote**。反過來 ⇒ 前端拿到未知 `shippingMethod` ⇒ 畫面破。
