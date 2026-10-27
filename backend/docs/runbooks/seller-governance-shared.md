# Seller Governance — 共用判斷標準

> 本文是 [`seller-violation-management-runbook.md`](./seller-violation-management-runbook.md)（一般檢舉）與 [`food-safety-incident-runbook.md`](./food-safety-incident-runbook.md)（食安通報）共用的判斷依據。
>
> **避免重複內容** — 兩份 runbook 都 link 過來，本文是 single source of truth。

---

## 1. 措施種類與依據選用

措施（`CreateMeasure`）依輸入的種類分三支：`warning`、`suspension`、`termination`（設定哪一支就是哪一種）；警告與停權另有依據（`violation` 或 `risk`），終止有事由（ground）與依據（契約條款或認定），見 runbook 的終止合作一節。

| 種類 | 何時選 | 影響 |
|---|---|---|
| `WARNING` | 輕微 / 首次 / 標示誤差 | 沒有限制，不停權；賣家看得到理由，可申訴 |
| `SUSPENSION` | 重大食安 / 連續違規 / 假認證，或需要先限制賣家的風險 | 帳號停權：能力降為白名單，連動下架商品；賣家看得到理由，可申訴 |
| `TERMINATION` | 終止合作（條款第 21 條：預告終止、重大違約、停業、解散、破產等） | 到期或立即生效；生效後停權，店家關閉；不是升級用的停權，事由與流程見 [`seller-violation-management-runbook.md`](./seller-violation-management-runbook.md) §4.1 |

| 依據 | 內容 | 結束時間 | 複審 |
|---|---|---|---|
| `VIOLATION` | 認定（類別、嚴重度；新建或引用同帳號既有、未撤銷的認定） | 停權必填；警告不可設 | 無 |
| `RISK` | 風險類型 `RiskGround`（條款 17.1 / 17.5：交易、食安、詐欺、個資、侵權、資安、履約、外部要求），一定是停權 | 選填，通常不設 | 必填 `review_by`，建立後 30 天內（預設 7 天） |

其他限制種類（罰金、部分品類停權、永久停權、強制訓練）目前沒有，要用時須先補上配套。

### 判斷指引

**選 `WARNING`（VIOLATION 依據）**：
- 商品標示錯誤但無實質傷害（如圖片誤植、規格欄位筆誤）
- 誇大宣稱（無「療效」「醫療」字眼，僅行銷話術）
- 首次違規 + 賣家配合改正

**選 `SUSPENSION`（VIOLATION 依據）**：
- 食安事件 verified（檢出污染、添加禁用物質、過期重貼標）
- 刷單、假評價（影響平台信任）
- 假認證（有機 / 產地 / 檢驗報告造假）
- 危害消費者重大利益（虛假廣告致買家受傷或受騙）

**選 `SUSPENSION`（RISK 依據）**：
- 風險存在但尚未形成可認定的違規，需要先限制（例如不服勸導、等待外部機關回覆）；風險排除就 `EndMeasure`，沒有排除就在複審期限前 `ScheduleMeasureReview` 並寫下理由

**不確定 → 升級給主管**：
- 邊界案件、首次出現的違規類型
- 賣家高銷量 / 高曝光（停權對平台流量有衝擊）
- 涉及法律解釋（需法務 review）

> VIOLATION 依據的停權，`end_time` 預設 30 天；極重大（食安致傷）可設 90 天，再長需主管批准。`end_time` 必須晚於現在。

---

## 2. 給賣家的理由（`statement`）撰寫指引

`statement` 是賣家在措施頁與停權公告看到的理由（DSA Art.17），**不公開**給買家或其他人（公開違規紀錄已刪除）。仍要避免污衊用語，因為賣家可能對外引用，也是申訴與爭議時的證據。

### 2.1 用詞對照

| 不可寫 | 應寫 |
|---|---|
| 「黑心賣家」「故意欺騙」「詐欺」 | 「商品標示與實際不符」「未達食品安全標準」 |
| 賣家私人資訊（姓名、住址、電話） | 不提及賣家個人，僅提 store_name |
| 通報者個資（即使去識別） | 完全不提及通報者（連「有買家通報」都避免） |
| 主觀字眼（「惡劣」「卑劣」「離譜」） | 客觀描述事實 |
| 推測動機（「為了省成本」「故意」） | 只描述事實，不推測動機 |

### 2.2 範例

**高嚴重（食安 verified）**：
> 「商品『XXX 有機高麗菜（500g）』經檢驗檢出農藥殘留超標（標示為有機，實際與標示不符），店家停權 30 天，期間商品下架。」

**中嚴重（標示誇大）**：
> 「商品標示誇大效果（未提供佐證資料），已要求賣家修正並給予警告。」

**低嚴重（規格誤差）**：
> 「商品實際重量與標示有差異（誤差超過 5%），已要求賣家校正。」

### 2.3 字數限制

建議 **50 - 150 字**，上限 2000 字。賣家看的理由只寫事實與依據；詳細證據鏈、批次號、檢驗報告連結寫在**內部註記**（`CreateMeasureNote`，只新增、賣家看不到）。

---

## 3. 申訴決議標準

收到申訴（`measure_requests` 的 `PENDING`）時的判斷依據。

### 3.1 建議 `accept` 申訴

- 賣家提供**新證據**（批次紀錄、進貨單、製程紀錄）證明檢舉不成立
- 原措施或其依據的認定**有誤**（admin 後續發現證據不足、誤判 batch、誤判商品）
- 該批次商品**已下架且無新流出**，且賣家提供下架證明
- 通報者撤回 / 通報內容不實（極少見，但可能）

### 3.2 建議 `reject` 申訴

- 賣家提供的證據與既有證據**矛盾**（如賣家說「沒進這批貨」但有出貨單）
- 證據**不足**（單純口頭說明無書面、無第三方佐證）
- 屬於**賣家應自行管控的範疇**（不能歸咎於上游、代工廠、物流）
- 賣家承認事實但主張「無心」/「初犯」/「無傷害」— 主觀理由不足以撤銷已 verified 的措施

### 3.3 `ACCEPTED`、`REJECTED` 都必填 `reason`

`DecideAppeal` 的 `reason`（存為 `decision_reason`）**會發給賣家**，所以必須：
1. 寫清楚為什麼不接受申訴（針對賣家提的證據逐點回應）
2. 給賣家循其他管道的線索（客服信箱、法務聯絡方式、訴願管道）
3. **避免人身攻擊**，僅描述事實與決策依據

範例：
> 「您提供的進貨單日期為 2026-04-15，但檢舉商品出貨記錄為 2026-04-10（早於進貨單），無法佐證檢舉不成立。如有其他批次紀錄請補充至客服信箱（service@example.com），可重新申訴。」

### 3.4 模糊案例

升級給**主管 + 法務 review**，不要 admin 個人判斷：
- 賣家銷量大、停權影響平台流量
- 涉及法律解釋（如「療效」字眼是否違法）
- 賣家提的證據看似合理但無法立即驗證
- 賣家威脅訴訟 / 媒體曝光

> Review 完後再 `DecideAppeal`，並在 `reason` 註明「經主管 / 法務確認」。

---

## 4. 升級規則

### 4.1 規則

近 90 天內，若該**帳號**（所有 stint 合計，關店重開不歸零）有 **≥ 2 筆未撤銷、原本是 medium 的認定**，admin 建立措施時新增的**這一筆 medium 認定**，系統**自動升級為 high**（即第 3 次 medium）。升級的認定 `escalated_from = MEDIUM`。

### 4.2 如何看到

`CreateMeasure` 回傳的措施，其認定帶 `escalated_from`；`ListViolations` / `GetViolation` 的認定可查 `severity` 與 `escalated_from`。

### 4.3 Admin 注意事項

- **給賣家的理由與內部註記應寫明事由**（避免賣家覺得突然升級無依據）。範例：
  > 「本次原評估為 medium，因該帳號近 90 天累計 2 筆 medium 認定（IDs: ...），依平台政策自動升 high。」
- 計數與新增在帳號 stint 的鎖內串行，並發建立不會繞過計數。
- 被撤銷的認定（含申訴成立後 `revoke_findings` 撤銷的）**不計入**累計。
- 升級後的措施可被申訴；申訴決議邏輯與一般措施相同（見 §3）。

### 4.4 自查

建立前可用 `ListViolations(account_id, status=ENFORCED)` 檢視帳號近 90 天的認定。

---

## 5. 共用聯絡方式

> 上線前由 owner 填入；**請勿** 寫明文檔於 git，僅放 Notion / 內部 Wiki。本文僅放 placeholder。

- **客服主管**：[填入]
- **治理 admin 主管**：[填入]
- **法務**：[填入]
- **ops**（Discord channel）：`#governance-admin`（admin UI 異常 / SQL 查不到 / DB 鎖定等）
- **B-169 owner**：[填入]
- **後端 on-call**：[填入]（食安通報走 24/7，一般檢舉僅工作時間）

---

## 6. Footer

- **Last updated**：2026-10-13
- **相關文件**：
  - [`seller-violation-management-runbook.md`](./seller-violation-management-runbook.md) — 一般檢舉處理
  - [`food-safety-incident-runbook.md`](./food-safety-incident-runbook.md) — 食安通報處理
  - [`postmortems/food-safety/_template.md`](../postmortems/food-safety/_template.md) — 食安 postmortem 模板
