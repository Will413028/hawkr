# Uber 額外成本的責任歸屬：**依責任成因判定，不依費用名稱**

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-11 |
| 影響範圍 | payment-service 責任判定與成本歸屬 |

- **最終規則（一句話）**：
  > 🔴 **Uber 額外成本不是依「取消費／等待費／退回費」的名稱決定負擔方，
  > 而是依「造成該成本的責任成因」決定。**
- **為什麼不依 fee type 一刀切**：同一個「取消費」可能來自賣家沒備貨、買家不在場、
  我方系統重複建單、或 Uber 自己派不出人——**四種成因的正解相反**，
  而帳單上它們都叫 cancellation fee。依名稱切會把四種情形壓成一個答案，
  那正是先前「平台吸收」這個單一答案站不住的原因。

## 決定：Responsibility-Based Cost Attribution

🔴 **本表的「負擔方」欄講的是 settlement counterparty（財務結算對象），不是 fault（責任方）。**
三軸分離的完整規約在 **§5.5.3**——`Fault` ≠ `Settlement Counterparty` ≠ `KPI Attribution`，
本表只填中間那一軸。⚠️ **把本表讀成「責任方對照表」是本 ADR 最容易被誤讀的地方**：
`BUYER_CAUSED` 那一列的負擔方是 SELLER，**而賣家在那一格不被記 fault、不被記 KPI failure**。

| 成因（Fault，`ResponsibilityCause`） | 典型情形 | **Settlement counterparty（`Bearer`）** | 硬規則 |
|---|---|---|---|
| **賣家原因** | 賣家主動取消／商品未備妥造成等待／pickup 資訊錯誤／賣家地址或聯絡資訊錯誤／賣家要求改派／賣家要求退回／賣家操作造成額外配送 | 🔴 **賣家負擔實際發生的 Uber provider cost** | **平台不吸收** |
| **買家原因** | 買家主動取消／無法聯絡／不在場造成等待／要求改址／拒收／買家原因造成退回 | 🔴 **該 provider cost 仍先歸屬「賣家物流成本」** | 🔴 **平台不得直接向買家建立第二次追加付款** |
| **平台／系統原因** | 平台 bug 重複建 Uber delivery／自動化錯誤造成取消／系統錯誤導致重新派送／平台提供錯誤資訊造成 provider 額外費用 | 🔴 **平台負擔** | 🔴 **不得向賣家收取** |
| **物流商／provider 原因** | Uber 系統問題／外送員問題／provider 自身錯誤／provider 不合理 adjustment | 🔴 **不得自動向賣家收取**；先進 **provider dispute／reconciliation** | provider 最終撤銷 ⇒ **平台與賣家都不應被收**；provider 最終確認費用成立且調查後責任其實屬賣家或買家 ⇒ **再依本表歸屬** |

## 🔴 買家原因那一格的推導（最容易被做錯的一格）

**為什麼買家造成的成本先歸賣家，而不是向買家追收？**

平台提供的是**賣家使用的物流服務**——線 B 是 `平台 ↔ 賣家` 的企業對企業關係，
買家不是這條線的當事人。平台若直接向買家二次扣款，等於**繞過賣家**在
`賣家 ↔ 買家` 的契約之外對買家收錢，而那個契約不是平台的。

⇒ **分層**：
```
平台 ↔ 賣家（線 B）：買家原因造成的 Uber 成本 ⇒ 歸賣家物流成本
賣家 ↔ 買家（另一層）：賣家依其 Buyer Terms 自行決定
                        ① 向買家收回 ② 自行吸收 ③ 以退款規則處理
```
🔴 **第二層不是平台的規則，平台不得代行**。

## 🔴 系統至少要能表達**五**個成因

```
ResponsibilityCause：SELLER_CAUSED ／ BUYER_CAUSED ／ PLATFORM_CAUSED ／ PROVIDER_CAUSED
FulfillmentExceptionCategory（案件分類，另一個碼表）：… ／ RESPONSIBILITY_UNRESOLVED
```

🔴 **「未判定」這個狀態是必要的，不是保險絲——但它不住在成因 enum 裡。** 上列四種成因的前提是「已經知道是誰造成的」，
而真實的 provider 終局（`undeliverable`／`canceled`）**常常不足以判定**（§5.5.2 C 的 evidence 只有五類，
其中三類可能全缺）。**沒有一個正式的「未判定」狀態時**，實作者只有兩條路：**猜一個**，或**把它塞進 `PROVIDER_CAUSED`**
——後者正是既有 CodeMeta description 寫的（「判定原料不足時一律落 `PROVIDER_CAUSED`（先進爭議）」），
而那等於**做了一個沒有證據的判定**並讓下游（退款、receivable、KPI）照著它全部動起來。
⇒ **evidence 不足就是 `RESPONSIBILITY_UNRESOLVED` 本身**（🔴 它是 `FulfillmentExceptionCategory` 的一格，**不是**第五個成因），它的載體、出口與 aging 在 **§5.5.2 D**。

**enum 已定版**：成因 ＝ `ResponsibilityCause`（**四值**）；未判定狀態 ＝ `FulfillmentExceptionCategory.RESPONSIBILITY_UNRESOLVED`（§5.5.2 A）。
✅ **CodeMeta 側已完成**（2026-09-17）：description 的舊 fail-closed 已廢除，未判定狀態已在案件分類碼表落地。
⚠️ `RESPONSIBILITY_UNRESOLVED` **不屬** `ResponsibilityCause`；替它新增第 5 值會撞破 `V12`／`V13`／`V15` 三道 CHECK。

## 🔴 `refund_to_buyer` 是另一件事，不由本表決定

本 ADR 回答的是「**這筆 provider cost 算誰的**」；
「**要不要退錢給買家、退多少**」由 **§5.5.2 F 的二元規則**回答，兩者**不得互相推導**。
🔴 **`BUYER_CAUSED` 這一格兩件事同時成立**：`refund_to_buyer = 0`（買家拿不回錢）
**且** provider cost 的結算對象是 SELLER。它們不矛盾——前者是**買賣契約**的結果，後者是**平台↔賣家物流服務**的結算路徑。

## 🔴 Responsibility Adjustment ≠ Platform Subsidy（兩者不得混記）

| | Platform Subsidy | Platform-Caused Cost |
|---|---|---|
| 性質 | **商業促銷** | **平台／系統錯誤造成的成本責任** |
| 誰決定 | 促銷政策 | 事故歸因 |
| 會計語意 | 行銷費用 | 損失／賠付 |
| 營運分析 | 活動成效 | 事故成本 |

🔴 **合成同一個數字之後，財務歸因與營運分析都會失真**——
「這個月平台花了 5 萬」分不出是辦活動還是系統出錯。兩者各自獨立記錄。

## 既有結論的承接（不變的那一半）

✅ **未送達 ⇒ 不向賣家收「配送服務費」**仍然成立。
⚠️ **它與本 ADR 不衝突，因為它們講的是兩種錢**：
「配送服務費」是**服務本身的價金**（沒送到就沒有服務）；
本 ADR 講的是**額外成本**（取消費等 provider 實際支出，它已經發生了）。
⇒ 未送達的訂單：**不收配送服務費，但額外成本依本表歸屬**。

## 後果

1. **需要一個責任成因的判定與記錄機制**（判定可人工、可規則，但**必須留痕**）。
2. 🔴 **所有額外成本一律逐筆記錄並進對帳**——本 ADR 消滅了「責任方未定」這個不記帳的藉口。
   ⚠️ **與 `RESPONSIBILITY_UNRESOLVED` 相容且互補**（§5.5.2）：「未判定」指的是**責任分類尚未 resolved**，
   **不是**「這筆成本不記」。成本照記、對帳照跑；未判定期間只是**不建立 Seller receivable、不決定 settlement counterparty**。
   🔴 **把「未判定」讀成「先不記帳」會讓 provider 帳單對不起來**，而那是最難補的一種缺口。
3. **provider 原因那一格需要 dispute 流程**（先爭議、後歸屬），
   它的技術設計屬 Scoped Review。
4. 🔴 **不得依費用名稱寫死任何負擔方**——包含不得寫「取消費一律由平台吸收」。

## 什麼情況要重新檢視

- 若 Uber 帳單的 adjustment 欄位**無法分辨成因**（Scoped Review 要查證的事項之一）
  ⇒ 判定只能靠我方的事件軌跡推斷，那會提高「provider 原因」那一格的比例，
  但**不改變本表的歸屬規則**。
