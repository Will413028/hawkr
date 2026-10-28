# PAYUNi 代理商專用 API 官方文檔（統整版）

> **整合來源**：PAYUNi 開發專區「代理商專區」官方 API 文件 + Hawkr 實作對照
> **最後更新**：2026-07-08
> **涵蓋範圍**：多功能收付 (agent_fee) + 多功能收付交易查詢 (agent_fee_query) + 多功能收付批次上傳 (SFTP) + 撥款指示 (grant) + 撥款查詢 (grant_query) + 新增合作商店 (bind_merchant) + 修改合作商店 (update_merchant)
> **本檔定位**：收斂原分散於《代理商綁定商店 API 整合指南》《新增修改 商店 API》與《金流官方 API 總覽》§6 的代理商專用內容；一般收款商店自用的金流／逆向金流／物流 API 仍見各對應總覽檔。

---

## 目錄

1. [通用規範](#1-通用規範)
2. [多功能收付 (agent_fee)](#2-多功能收付-agent_fee)
3. [多功能收付交易查詢 (agent_fee_query)](#3-多功能收付交易查詢-agent_fee_query)
4. [多功能收付批次上傳 (SFTP)](#4-多功能收付批次上傳-sftp)
5. [撥款指示 (grant)](#5-撥款指示-grant)
6. [撥款查詢 (grant_query)](#6-撥款查詢-grant_query)
7. [新增合作商店 (bind_merchant)](#7-新增合作商店-bind_merchant)
8. [修改合作商店 (update_merchant)](#8-修改合作商店-update_merchant)
9. [合作商店支付方式與手續費代碼](#9-合作商店支付方式與手續費代碼)
10. [代理商貨態通知](#10-代理商貨態通知)
11. [合作商店 API 錯誤代碼對照表](#11-合作商店-api-錯誤代碼對照表agent)
12. [Hawkr 實作對照](#12-hawkr-實作對照)
13. [附錄：API Endpoint 速查表](#附錄api-endpoint-速查表)

---

## 1. 通用規範

### 1.1 環境

| 環境 | 平台域名 | API 域名 |
|------|--------|---------|
| 測試區 | `https://sandbox.payuni.com.tw` | `https://sandbox-api.payuni.com.tw` |
| 正式區 | `https://www.payuni.com.tw` | `https://api.payuni.com.tw` |

### 1.2 安全與加密

- 所有傳輸必須採用 **TLS v1.2** 以上協定
- 請求方式一律 **HTTP POST**（`application/x-www-form-urlencoded`）；請於 header 加入 `user-agent`，建議內容為 `"payuni"`
- 加解密演算法：AES + SHA256（EncryptInfo / HashInfo）。Hawkr 端實作細節見 §12.1

### 1.3 串接金鑰

代理商 API 使用**代理商 Hash Key / Hash IV**（與一般收款商店的金鑰不同）：

- 登入 PAYUNi 平台 → **代理商專區 → 合約及基本資料** → 取得 **Hash Key**（32 字元）與 **Hash IV**（16 字元）
- 部分 API（批次上傳、ZIP 壓縮密碼）僅使用 **Hash Key 前四碼**

### 1.4 統一請求外層參數

代理商 API 外層有**兩種識別方式**，依 API 而異：

**（A）代理商前綴外層**（`agent_fee`／`agent_fee_query`／`grant`／`bind_merchant`／`update_merchant` 使用）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| AgentID | Y | String | 代理商前綴字 | 4 個英文字母全大寫（Hawkr：`HAWK`） |
| Version | Y | String | 版本 | 固定 `1.0` |
| EncryptInfo | Y | String | AES 加密字串 | 請參考「資料加解密」文件 |
| HashInfo | Y | String | SHA256 壓碼字串 | 請參考「資料加解密」文件 |

**（B）商店代號外層 + 代理商模式旗標**（`grant_query` 撥款查詢使用）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| Version | Y | String | 版本 | 固定 `1.0` |
| EncryptInfo | Y | String | AES 加密字串 | 請參考「資料加解密」文件 |
| HashInfo | Y | String | SHA256 加密字串 | 請參考「資料加解密」文件 |
| IsPlatForm | C | Int | 代理商金鑰旗標 | 若使用**代理商金鑰**串接需帶 `1`，且與 `MerID`／`Version`／`EncryptInfo`／`HashInfo` **同層** |

> **注意**：一般收款商店的逆向金流／退款轉匯 API（見《逆向金流 API 總覽》）在代理商模式下呼叫時，外層同樣以 `IsPlatForm=1` 標記，其餘外層維持 MerID 版。

### 1.5 統一返回外層參數

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=成功，失敗請參考錯誤代碼 |
| Message | String | 狀態說明 | 依狀態代碼顯示回應訊息 |
| MerID / AgentID | String | 商店代號 / 代理商前綴 | 依 §1.4 外層方式回傳對應識別 |
| Version | String | 版本 | 固定 `1.0` |
| EncryptInfo | String | 加密字串 | 請參考「資料加解密」文件（查詢類 API 才回傳） |
| HashInfo | String | 加密 Hash | 請參考「資料加解密」文件（查詢類 API 才回傳） |

### 1.6 前置作業

1. 申請代理商合作資格，並與統一金流平台簽訂相關代理商合作業務契約
2. 於 PAYUNi 平台註冊會員，申請開通所需的相關機制（多功能收付／撥款指示 API 需個別申請開通）
3. 登入 PAYUNi 平台 → **代理商專區 → 合約及基本資料** → 設定功能 **API 允許 IP**
4. 批次上傳另需向 PAYUNi 申請 **SFTP 獨立帳號**（申請書請洽業務或客服中心索取）

### 1.7 身分識別概念釐清

| 識別 | 說明 | 範例 |
|------|------|------|
| **AgentID** | 代理商前綴字，4 個英文字母全大寫。代理商 API 外層識別（方式 A）。 | `HAWK` |
| **MerID** | 合作商店的商店代號，格式為代理商前綴 + 數字，長度 ≤ 20。 | `HAWK2612345678` |
| **AgentMerID** | 代理商商店代號。於代理商啟用商店提領時使用，可指定代理商出入帳之商店；未帶則依平台預設。 | 長度 20 |

> Hawkr 的 MerID = `HAWK` + `store_code`（見 §12.2）。`merId` 是金流商家編號，僅限 PayUni 付款／退款／Webhook 情境使用，不得當作業務身分鍵，見[Java 微服務開發規範 §5.2](../開發規範/Java%20微服務開發規範.md)。

### 1.8 共用代碼（多功能收付）

**FeeType 費用類別**（長度 1，`agent_fee`／`agent_fee_query`／批次上傳共用）

| 代碼 | 說明 |
|------|------|
| 1 | 平台交易手續費 |
| 2 | 佣金 |
| 3 | 退款 |
| 4 | 物流 |
| 5 | 平台使用費 |
| 6 | 補貼 |
| 7 | 押金 |
| 9 | 其他 |

**BalType 正負向**（長度 1）

| 代碼 | 說明 |
|------|------|
| 1 | 正向（扣合作商店款項，扣 1% 手續費後撥款給代理商） |
| 2 | 負向（扣代理商款項含該金額 1% 手續費，足額撥款給合作商店） |

---

## 2. 多功能收付 (agent_fee)

### 2.1 概述

代理商如有向旗下合作商店**收款或付款**的需求，可透過此 API 向合作商店執行扣款或付款。

- 發動多功能收付 API 後，實際交易**於次日執行**，合作商店也能從 PAYUNi 查詢相關交易
- 此功能收取交易金額之 **1% 手續費**
- **正向交易**：扣除合作商店款項（扣除 1% 手續費後）撥款給予代理商
- **負向交易**：扣除代理商款項（包含該金額 1% 手續費），足額撥款給予合作商店

### 2.2 前置作業

1. 申請代理商合作資格並與統一金流平台簽訂相關代理商合作業務契約
2. 於 PAYUNi 平台註冊會員，申請開通所需的相關機制
3. 此 API 需經申請開通才可使用
4. 登入 PAYUNi 平台 → 代理商專區 → 合約及基本資料 設定功能 API 允許 IP

### 2.3 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/agent/agent_fee` |
| 正式區 URL | `https://api.payuni.com.tw/api/agent/agent_fee` |
| 請求方式 | HTTP POST（header 加入 `user-agent`，建議 `"payuni"`） |
| 版本 | 固定 `1.0` |
| 外層識別 | AgentID（§1.4 方式 A） |
| 串接金鑰 | 代理商 Hash Key / Hash IV |

### 2.4 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | 長度 20，合作商店商店代號 |
| TradeNo | Y | String | 代理商自訂單號 | 長度 20，可重複 |
| Amt | Y | Int | 金額 | 長度 7，1~9999999 |
| FeeType | Y | Int | 費用類別 | 長度 1，見 §1.8 |
| BalType | Y | Int | 正負向 | 長度 1，1=正向、2=負向 |
| AgentMerID | C | String | 代理商商店代號 | 長度 20。於代理商啟用商店提領時使用，可指定代理商出入帳之商店；未帶則依平台預設 |

### 2.5 返回參數

| 參數 | 說明 | 備註 |
|------|------|------|
| Status | 狀態代碼 | `SUCCESS`=成功，失敗請參考錯誤代碼 |
| Message | 回應訊息 | 依狀態代碼顯示回應訊息 |

---

## 3. 多功能收付交易查詢 (agent_fee_query)

### 3.1 概述

代理商如有發動多功能收付 API 交易，可透過此查詢確認交易結果狀態（含撥款狀態）。

### 3.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/agent/agent_fee_query` |
| 正式區 URL | `https://api.payuni.com.tw/api/agent/agent_fee_query` |
| 請求方式 | HTTP POST（header 加入 `user-agent`，建議 `"payuni"`） |
| 版本 | 固定 `1.0` |
| 外層識別 | AgentID（§1.4 方式 A） |
| 串接金鑰 | 代理商 Hash Key / Hash IV |

### 3.3 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | 長度 20 |
| TradeNo | Y | String | 代理商自訂單號 | 長度 20，可重複 |
| Amt | Y | Int | 金額 | 長度 7，1~9999999 |

### 3.4 返回參數

| 參數 | 說明 | 備註 |
|------|------|------|
| MerID | 商店代號 | 長度 20 |
| TradeNo | 代理商自訂單號 | 長度 20 |
| Amt | 金額 | 長度 7，1~9999999 |
| FeeType | 費用類別 | 長度 1，見 §1.8 |
| BalType | 正負向 | 長度 1，1=正向、2=負向 |
| TradeDate | 建立日期 | |
| FundDate | 撥款日 | 若為 `0000-00-00` 表示尚未撥款 |
| Fee | 多功能收付手續費 | |
| AgentMerID | 代理商商店代號 | 長度 20；若無此參數則回空值 |

---

## 4. 多功能收付批次上傳 (SFTP)

### 4.1 概述

代理商可將各種預計多功能收付建檔，以上傳 SFTP 方式達到**批次**多功能收付，向旗下合作商店執行扣款或付款。

- 發動後實際交易**於次日執行**，合作商店可從 PAYUNi 查詢相關交易
- 收取交易金額之 **1% 手續費**；正向／負向定義同 §1.8
- 合作商店資格：使用 B2C 大宗寄倉相關費用需符合商業會員資格

### 4.2 前置作業

1. 申請代理商合作資格並與統一金流平台簽訂相關代理商合作業務契約
2. 登入 PAYUNi 平台檢視代理商串接資訊取得 **Hash Key 前四碼**（作為 ZIP 壓縮密碼）
3. 此功能需先開通**多功能收付 API** 才可使用
4. 向 PAYUNi 申請 **SFTP 獨立帳號**（申請書請洽業務或客服中心索取）

### 4.3 上傳檔案格式

#### 一、檔名格式

```
代理商前綴字(四碼)_上傳當天日期_序號.txt
```

- 每個 `.txt` 需**壓縮為 ZIP 檔**，並加帶密碼為**預設代理商串接 Hash Key 前四碼**
- 範例：
  - `HAWK_20230901_01.txt`（壓為 ZIP，密碼 = Hash Key 前四碼）
  - `HAWK_20230901_02.txt`（壓為 ZIP，密碼 = Hash Key 前四碼）

#### 二、檔案格式（統一以 `|` 為分隔符號）

```
檔頭：FH|代理商前綴|總筆數|背景通知網址
檔身：FD|商店代號|代理商自訂單號|金額|費用類別|正負向|代理商商店代號
```

**檔頭欄位**

| 欄位 | 必填 | 備註 |
|------|------|------|
| FH | Y | 固定放 `FH` |
| 代理商前綴字 | Y | |
| 總筆數 | Y | |
| 背景通知網址 | N | 執行完會 notify（`['message' => "檔案已處理，請查看處理結果。"]`）；僅限 80 與 443 port |

**檔身欄位**

| 欄位 | 必填 | 備註 |
|------|------|------|
| FD | Y | 固定放 `FD` |
| 商店代號 | Y | 長度 20，合作商店商店代號 |
| 代理商自訂單號 | Y | 長度 20，可重複 |
| 金額 | Y | 長度 7，1~9999999 |
| 費用類別 | Y | 長度 1，見 §1.8 |
| 正負向 | Y | 長度 1，1=正向、2=負向 |
| 代理商商店代號 | C | 長度 20；於代理商啟用商店提領時使用，可指定代理商出入帳之商店，未帶則依平台預設 |

**範例**：

```
FH|BOXX|2|
FD|BOXX1669886568|nts091201|25|3|1|
FD|BOXX1669886568|nts091202|150|2|1|
```

### 4.4 回傳檔案格式

#### 一、檔名格式

- 成功執行：`上傳檔名_原檔案日期_result.txt`（回覆壓縮 ZIP 檔，密碼 = 預設代理商串接 Hash Key 前四碼）
  - 例：`HAWK_20230901_01_result.txt`
- 錯誤：`上傳檔名_原檔案日期_error.txt`（回覆壓縮 ZIP 檔，密碼同上）
  - 例：`HAWK_20230901_01_error.txt`
  - 備註：錯誤發生情境為檔案格式問題（例：檔名與檔頭名稱不同、筆數不符等）

#### 二、檔案格式

```
檔身：狀態代碼|原檔案商店代號|原檔案代理商自訂單號|原檔案金額|原檔案費用類別|原檔案正負向|原檔案代理商商店代號|處理結果訊息內容
檔尾：總筆數|成功筆數|失敗筆數
```

| 回傳欄位 | 說明 |
|----------|------|
| 狀態代碼 | `SUCCESS`=成功；若失敗請參考錯誤代碼 |
| 商店代號 | 原檔案值 |
| 代理商自訂單號 | 原檔案值 |
| 金額 | 原檔案值 |
| 費用類別 | 原檔案值 |
| 正負向 | 原檔案值 |
| 代理商商店代號 | 原檔案值 |
| 處理結果訊息內容 | |
| 總筆數 / 成功筆數 / 失敗筆數 | 本次執行的資料筆數統計（檔尾） |

**範例**：

```
SUCCESS|BOXX1669886568|nts091201|25|3|1||代理商費用資料建立成功
SUCCESS|BOXX1669886568|nts091202|150|2|1||代理商費用資料建立成功
2|2|0
```

---

## 5. 撥款指示 (grant)

### 5.1 概述

本 API 提供代理商控制旗下合作商店**每筆交易的撥款**。

- 代理商可透過此 API 發動旗下合作商店交易撥款指示，PAYUNi 於**隔日**將此筆交易撥款至旗下合作商店之會員 UNi 帳戶
- **僅支援正向交易**撥款指示，且須於該筆交易建立時帶入撥款方式參數為撥款指示才可使用
- 退款則固定依平台政策（依商店撥款天期，或非信用卡退款轉匯則固定為隔日）
- 當有退款時（含部分退款），若該筆交易尚未發動撥款指示，則 PAYUNi 會主動撥款：信用卡交易撥款入帳日同退款的撥款入帳日，非信用卡退款轉匯交易撥款入帳日為退款發動隔日

### 5.2 前置作業

1. 申請代理商合作資格並與統一金流平台簽訂相關代理商合作業務契約
2. 於 PAYUNi 平台註冊會員，申請開通所需的相關機制
3. 此撥款指示 API 需經申請開通才可使用
4. 登入 PAYUNi 平台 → 代理商專區 → 合約及基本資料設定功能 API 允許 IP

### 5.3 撥款指示執行流程

1. 於交易建立 API 增加 EncryptInfo 帶入參數 `GrantExport=1`，表示指定此筆交易撥款方式為撥款指示（流程同各交易建立 API）
2. 執行撥款指示 API
3. 發動撥款指示 API 後，實際於**次日**執行撥款入帳

### 5.4 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/trade/grant` |
| 正式區 URL | `https://api.payuni.com.tw/api/trade/grant` |
| 請求方式 | HTTP POST（header 加入 `user-agent`，建議 `"payuni"`） |
| 版本 | 固定 `1.0` |
| 外層識別 | AgentID（§1.4 方式 A） |
| 串接金鑰 | 代理商 Hash Key / Hash IV |

### 5.5 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | 長度 20 |
| TradeNo | Y | String | UNi 序號 | 長度 20 |
| Amt | Y | Int | 金額 | 長度 7，1~9999999。該筆訂單之最終付款金額 |

### 5.6 返回參數

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=成功，失敗請參考錯誤代碼 |
| Message | String | 回應訊息 | 依狀態代碼顯示回應訊息 |

---

## 6. 撥款查詢 (grant_query)

### 6.1 概述

撥款查詢可供代理商查詢撥款當日所有撥款訂單的明細。

### 6.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/trade/grant_query` |
| 正式區 URL | `https://api.payuni.com.tw/api/trade/grant_query` |
| 請求方式 | HTTP POST（header 加入 `user-agent`，建議 `"payuni"`） |
| 版本 | 固定 `1.0` |
| 外層識別 | MerID + `IsPlatForm=1`（§1.4 方式 B） |
| 串接金鑰 | 代理商 Hash Key / Hash IV |

> **注意**：本 API 外層使用 `MerID`（非 AgentID）；若以**代理商金鑰**串接，需增加請求參數 `IsPlatForm=1`，且與 `MerID`／`Version`／`EncryptInfo`／`HashInfo` 同層。

### 6.3 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| QueryDate | Y | String | 日期 | 格式 `YYYY-MM-DD`；僅限查詢 30 天內資料 |
| Page | C | Integer | 頁碼 | 每頁固定回傳 500 筆，預設為第 1 頁 |

### 6.4 返回參數

**外層返回**

| 參數 | 說明 | 備註 |
|------|------|------|
| Status | 狀態代碼 | `SUCCESS`=查詢成功，失敗請參考錯誤代碼 |
| MerID | 商店代號 | |
| Version | 版本 | 固定 `1.0` |
| EncryptInfo | 加密字串 | 請參考「資料加解密」文件 |
| HashInfo | 加密 Hash | 請參考「資料加解密」文件 |

**Result 陣列**（EncryptInfo 內；可能多筆，由 0 開始）

| 參數 | 說明 | 備註 |
|------|------|------|
| Status | 狀態代碼 | `SUCCESS`=查詢成功，失敗請參考錯誤代碼 |
| Message | 狀態說明 | 若失敗請參考錯誤代碼 |
| TradeDate | 入帳日期 | |
| MerTradeNo | 商店訂單編號 | |
| TradeNo | UNi 序號 | |
| MerID | 商店代號 | |
| PaymentType | 支付方式 | |
| BankChannel | 收款通路 | |
| PaymentDate | 付款（退款）日期 | |
| PaymentInfo | 付款資訊 | |
| ExecuteAmt | 執行金額 | |
| Fee | 手續費 | |
| Action | 執行方式 | |
| FundsAmt | 入帳金額 | |
| Type | 類別 | 入帳或出帳 |

**分頁資訊（PageInfo）**

| 參數 | 說明 |
|------|------|
| Current | 目前分頁 |
| Total | 總頁數 |

---

## 7. 新增合作商店 (bind_merchant)

### 7.1 概述

代理商透過此 API 將旗下商戶註冊為 PAYUNi 合作商店。PAYUNi 平台會**自動判斷**該用戶是否已註冊會員：

- 若已是會員 → 僅建立商店
- 若尚未註冊 → 自動建立會員 + 商店

**流程**：代理商傳送參數 → PayUni 自動判斷是否為既有會員 → 建立會員（如需要）+ 建立商店。

### 7.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/agent/bind_merchant` |
| 正式區 URL | `https://api.payuni.com.tw/api/agent/bind_merchant` |
| 請求方式 | HTTP POST（`application/x-www-form-urlencoded`，header 加入 `user-agent`，建議 `"payuni"`） |
| 版本 | 固定 `1.0` |
| 外層識別 | AgentID（§1.4 方式 A） |
| 串接金鑰 | 代理商 Hash Key / Hash IV |

### 7.3 請求參數（EncryptInfo）

#### 7.3.1 會員資訊

| 參數 | 必要 | 類型 | 長度 | 說明 |
|------|------|------|------|------|
| Name | Y | String | 50 | 會員姓名或企業名稱，需與登記證明文件名稱相同（例：身分證姓名、公司登記名稱） |
| UBN | Y | String | 10 | 會員統編或身分證字號 |
| Email | Y | String | 50 | 電子郵件，必須符合信箱格式 |

#### 7.3.2 企業會員額外參數（個人會員不帶）

| 參數 | 必要 | 類型 | 長度 | 說明 |
|------|------|------|------|------|
| MBCName | C | String | 20 | 主聯絡人姓名。非必填；未帶由合作商店登入後台時補提供 |
| MBCTel | C | String | 12 | 主聯絡人電話。同上 |
| MBCMail | C | String | 50 | 主聯絡人電子郵件（信箱格式）。同上 |
| MCName | C | String | 20 | 負責人姓名。同上 |
| MCID | C | String | 10 | 負責人身分證字號。同上 |

#### 7.3.3 商店聯絡資訊

| 參數 | 必要 | 類型 | 長度 | 說明 |
|------|------|------|------|------|
| ContactName | Y | String | 20 | 商店聯絡人姓名 |
| ContactTel | C¹ | String | 20 | 商店聯絡人電話。格式 `【區碼-電話號碼】`，例 `02-9888888`；如需分機加 `#`，例 `02-9888888#123` |
| ContactMobile | C¹ | String | 20 | 商店聯絡人手機 |

> ¹ `ContactTel` 與 `ContactMobile` **擇一必填**。

#### 7.3.4 商店基本資訊

| 參數 | 必要 | 類型 | 長度 | 說明 |
|------|------|------|------|------|
| MerID | Y | String | 20 | 商店代號。格式：代理商前綴字 + 數字（例：`HAWK2612345678`） |
| MerEmail | Y | String | 50 | 商店客服信箱（信箱格式） |
| MerName | Y | String | 20 | 商店中文名稱，需符合 **BIG5** 編碼規則 |
| MerEngName | Y | String | 50 | 商店英文名稱。僅容許大小寫英文、數字、半形空白、特定符號：`` ~!@#$%^&*()_-+={[}]\:;"<,>.?/' `` |
| MerURL | Y | String | 200 | 商店網址，`https://` 或 `http://` 開頭之有效網址，**禁止 IP 或中文網址** |
| MerAddr | Y | String | 50 | 商店聯絡地址 |
| MerAddrPost | Y | String | 3 | 商店地址郵遞區號（**三碼**） |
| MerTel | C | String | 20 | 商店聯絡電話（格式同 `ContactTel`） |
| MerDesc | Y | String | 500 | 商店簡介，可用半形分號 `;` 分隔多段敘述 |

#### 7.3.5 商店營業設定

| 參數 | 必要 | 類型 | 說明 | Hawkr 建議值 |
|------|------|------|------|-------------------|
| MerType | Y | Int | 販賣類型：`1`=服務 / `2`=實體商品 / `3`=非實體商品 / `4`=實體+非實體 | `2` |
| MerMCC | Y | Int | 行業別代碼（3 碼，參考 PayUni 商品分類表） | `3`（食品） |
| GoodsRate | Y | String | 商品交付類型比例，15 字長度，逗號分隔：`非預付%,預付%,儲值遞延%,票券%`，總和須為 100 | `100,0,0,0` |
| Payment | Y | String | 支付方式代碼，逗號分隔（見 §9.1） | `CREDIT` |
| DeliverTime | Y | Int | 商品交付完成時間（月份，0~99） | `7` |
| EscrowType | Y | Int | 履約保證類型：`0`=無 / `1`=履保 / `2`=信託 / `3`=保證金 / `4`=定存質押 | `1` |
| MerUnitPrice | Y | Int | 商品平均客單價 | `1000` |
| SellType | Y | Int | 商店交易類型：`1`=網路商店 / `2`=實體商店 | `1` |
| Fee | C | String | 手續費率（見 §9.2）。未帶則依平台預設 | — |
| CloseApplySwitch | C | String | 關閉「後台申請開通」按鈕的支付工具清單，逗號分隔。未帶則後台顯示申請按鈕 | — |

### 7.4 回應參數

| 參數 | 說明 |
|------|------|
| Status | `SUCCESS`=成功；失敗參考 §11 錯誤碼 |
| Message | 回應訊息 |
| Version | 固定 `1.0`（失敗時才會出現） |

### 7.5 備註

- `bind_merchant` API 若使用 B2C **大宗寄倉**相關 `Fee` 參數，會員身分需符合**商業會員**資格，否則 `Status` 會回傳失敗。

---

## 8. 修改合作商店 (update_merchant)

### 8.1 概述

修改既有合作商店的聯絡方式、地址、支付設定。

> **重要限制**：`update_merchant` 只能修改商店的**聯絡方式、地址、支付設定**，**不能修改核心身份欄位**（`Name`、`UBN`、`MerName`、`MerEngName`、`MerURL`、`MerDesc`、`MerType`、`MerMCC`、`MCName`、`MCID` 等）。核心身份欄位一旦由 `bind_merchant` 建立就不可變更。

### 8.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/agent/update_merchant` |
| 正式區 URL | `https://api.payuni.com.tw/api/agent/update_merchant` |
| 請求方式 | HTTP POST（`application/x-www-form-urlencoded`，header 加入 `user-agent`，建議 `"payuni"`） |
| 版本 | 固定 `1.0` |
| 外層識別 | AgentID（§1.4 方式 A） |
| 串接金鑰 | 代理商 Hash Key / Hash IV |

### 8.3 請求參數（EncryptInfo）

> 欲修改商店資訊**僅需帶入要修改的參數**，無需修改的欄位不用帶。

| 參數 | 必要 | 類型 | 長度 | 說明 |
|------|------|------|------|------|
| MerID | Y | String | — | 商店代號（必填） |
| ContactName | C | String | — | 商店聯絡人姓名 |
| ContactTel | C¹ | String | 20 | 商店聯絡人電話（格式同 §7.3.3） |
| ContactMobile | C¹ | String | 20 | 商店聯絡人手機 |
| MerEmail | C | String | — | 商店客服信箱（信箱格式） |
| MerAddr | C | String | — | 商店聯絡地址 |
| MerAddrPost | C | String | 3 | 商店地址郵遞區號（三碼） |
| MerTel | C | String | 20 | 商店聯絡電話 |
| Fee | C | String | — | 手續費率（見 §9.2） |
| Payment | C | String | — | 支付方式**啟用**清單，逗號分隔 |
| PaymentOff | C | String | — | 支付方式**關閉**清單，逗號分隔 |
| CloseApplySwitch | C | String | — | 開通按鈕**不顯示**清單 |
| OpenApplySwitch | C | String | — | 開通按鈕**顯示**清單 |

> ¹ `ContactTel` / `ContactMobile` **擇一必填**（若有要修改聯絡方式時）。

### 8.4 衝突與注意事項

- `Payment`（啟用）與 `PaymentOff`（關閉）**不可同時帶入相同參數值**
- `CloseApplySwitch`（不顯示）與 `OpenApplySwitch`（顯示）**不可同時帶入相同參數值**

### 8.5 回應參數

| 參數 | 說明 |
|------|------|
| Status | `SUCCESS`=成功；失敗參考 §11 錯誤碼 |
| Message | 回應訊息 |

---

## 9. 合作商店支付方式與手續費代碼

適用於 `bind_merchant`（§7）與 `update_merchant`（§8）的 `Payment` / `Fee` 參數。

### 9.1 Payment 支付方式代碼

可直接透過 `bind_merchant` / `update_merchant` 啟用的代碼（逗號分隔）：

```
CREDIT, ForeignCard, Inst3, Inst9, Inst12, Inst30, UnionPay, ATM, CVS
```

> **不可透過 API 帶入**的支付方式（須由商店自行至 PayUni 後台申請開通）：
> `JKoPay`、`ICASH`、`LinePay`、`SPB2C`、`SPB2CFrz`、`SPC2C`、`SPC2CFrz`

### 9.2 Fee 手續費率格式

逗號分隔的 `代碼=費率` 列表：

```
CREDIT=0.03,ForeignCard=0.04,Inst3=0.03,Inst9=0.05,Inst12=0.07,Inst30=0.13,
UnionPay=0.031,ATM=0.01,CVS=0.01,ATMLimit=15,CVSLimit=15,
SPB2CCharge=0.005,SPB2CFee=50
```

**計費規則**：

| 情境 | 解讀 |
|------|------|
| `CREDIT=0.03` | 信用卡費率 3%（Apple Pay / Google Pay / Samsung Pay 同一次付清，無須設定） |
| `ATM=15` 或 `CVS=15`（≥1） | 每筆固定 15 元 |
| `ATM=0.01` 搭配 `ATMLimit=15`（<1） | 1% 費率，單筆上限 15 元 |
| `CVS=0.01` 搭配 `CVSLimit=15` | 同上 |

> 未帶 `Fee` 時依平台預設值。

### 9.3 開通按鈕控制參數

| 參數 | 用途 | 支援的支付工具 |
|------|------|--------------|
| CloseApplySwitch | 後台**不顯示**申請開通按鈕 | `JKoPay`、`ICASH`、`LinePay`、`SPB2C`、`SPB2CFrz`、`SPC2C`、`SPC2CFrz`、`HOME` |
| OpenApplySwitch | 後台**顯示**申請開通按鈕 | 同上 |

---

## 10. 代理商貨態通知

代理商建立合作商店後，可透過物流貨態通知機制接收旗下商店的物流貨態資訊。於 PAYUNi 平台 → 代理商專區 → 合約及基本資料取得串接金鑰並輸入貨態 Notify 網址。

> 此機制與一般物流貨態通知共用同一套狀態碼與 Notify 格式，完整規格見《[PayUni 物流官方 API 總覽](PayUni%20物流官方%20API%20總覽.md)》§11.4「代理商專用貨態通知」與 §12「物流貨態狀態碼對照表」。

---

## 11. 合作商店 API 錯誤代碼對照表（AGENT*）

適用於 `bind_merchant`（§7）與 `update_merchant`（§8）。

### 11.1 基本錯誤（AGENT01*）

| 錯誤碼 | 說明 |
|--------|------|
| `AGENT01001` | 請填入代理商代號 |
| `AGENT01002` | 查無符合代理商資料 |
| `AGENT01003` | 資料 HASH 比對不符合 |
| `AGENT01004` | 資料解密失敗 |
| `AGENT01005` | 解密資料不存在 |

### 11.2 物流相關（AGENT02*）

| 錯誤碼 | 說明 |
|--------|------|
| `AGENT02001` | 查無符合商店物流資料 |
| `AGENT02002` | 商店 B2C 物流服務尚未開通 |
| `AGENT02003` | 商店 C2C 物流服務尚未開通 |
| `AGENT02004` | 商店未啟用 B2C 物流服務 |
| `AGENT02005` | 商店未啟用 C2C 物流服務 |

### 11.3 會員相關（AGENT03*）

| 錯誤碼 | 說明 |
|--------|------|
| `AGENT03001` | 請填入會員名稱 |
| `AGENT03002` | 請填入統編或身分證字號 |
| `AGENT03003` | 統編或身分證字號長度過長 |
| `AGENT03004` | 統編或身分證字號格式有誤 |
| `AGENT03005` | 請填入電話 |
| `AGENT03006` | 電話長度過長 |
| `AGENT03007` | 電話格式不符 |
| `AGENT03008` | 請填入會員電子信箱 |
| `AGENT03009` | 會員電子信箱格式錯誤 |
| `AGENT03010` | 請填入會員地址 |
| `AGENT03011` | 請填入會員帳號 |
| `AGENT03012` | 會員登入帳號長度過長 |
| `AGENT03013` | 請填入會員銀行帳戶總行與分行代碼 |
| `AGENT03014` | 會員銀行帳戶總行與分行代碼長度過長 |
| `AGENT03015` | 請填入會員銀行帳戶 |
| `AGENT03016` | 會員銀行帳戶長度過長 |
| `AGENT03017` | 請填入會員銀行帳戶戶名 |
| `AGENT03019` | 會員帳號或手機或 Email 資料重複；或會員資料建立失敗 |
| `AGENT03020` | 會員建立銀行帳號失敗（可能是銀行代號不存在） |
| `AGENT03021` | 會員更新資料失敗 |
| `AGENT03022` | 會員銀行資料取得失敗 |
| `AGENT03023` | 會員銀行資料更新失敗 |
| `AGENT03024` | 缺少會員編號 |
| `AGENT03025` | 取得會員資料失敗 |
| `AGENT03026` | 修改會員資料失敗 |
| `AGENT03027` | 會員名稱格式錯誤 |
| `AGENT03028` | 個人會員名稱長度不得超過 25 字元 |
| `AGENT03029` | 企業會員名稱長度不得超過 200 字元 |
| `AGENT03031` | 此會員已停權，不可建立商店 |

### 11.4 商店基本資料（AGENT04*）

| 錯誤碼 | 說明 |
|--------|------|
| `AGENT04000` | 請填入商店代號 |
| `AGENT04001` | 請填入商店電子信箱 |
| `AGENT04002` | 商店電子信箱格式錯誤 |
| `AGENT04003` | 請填入商店名稱 |
| `AGENT04004` | 請填入商店英文名稱 |
| `AGENT04005` | 請填入商店網址 |
| `AGENT04006` | 商店網址格式錯誤 |
| `AGENT04007` | 請填入商店地址 |
| `AGENT04008` | 請填入商店電話 |
| `AGENT04009` | 商店電話長度過長 |
| `AGENT04010` | 商店電話格式錯誤，區碼須為 0 開頭且格式需為：`【區碼-電話號碼】`，例 `02-9888888` |
| `AGENT04011` | 請填入商店簡介 |
| `AGENT04012` | 請填入販賣類型 |
| `AGENT04013` | 販賣類型需為數字型態 |
| `AGENT04014` | 販賣類型長度過長 |
| `AGENT04015` | 請填入行業別 |
| `AGENT04016` | 行業別長度過長 |
| **`AGENT04017`** | **商店代號已存在** ⚠️ MerID 衝突，處理邏輯見 §12.8 |
| `AGENT04018` | 商品交付類型須為數字型態 |
| `AGENT04019` | 商品交付類型代碼長度過長 |
| `AGENT04020` | 請填入商品交付類型 |
| `AGENT04021` | 商店建立失敗 |
| `AGENT04022` | 請填入商店地址郵遞區號 |
| `AGENT04023` | 商店地址郵遞區號不存在 |
| `AGENT04024` | 請填入商店編號 |
| `AGENT04025` | 商店代號長度不可超過 20 位 |
| `AGENT04026` | 行業別資料必須為數字 |
| `AGENT04027` | 商店簡介字數過長 |
| `AGENT04028` | 國外卡英文帳單名稱僅容許大小寫英文、數字、半形空白、符號：`` ~!@#$%^&*()_-+={[}]\:;"<,>.?/' `` |
| `AGENT04029` | 國外卡英文帳單名稱僅容許 50 字元 |

### 11.5 代理商（AGENT05*）

| 錯誤碼 | 說明 |
|--------|------|
| `AGENT05001` | 代理商未啟用 |
| `AGENT05002` | 商店代號前贅字錯誤 |
| `AGENT05003` | 請填入代理商編號 |
| `AGENT05004` | 請填入代理商前綴字詞 |
| `AGENT05005` | 代理商前綴字詞超過長度 |
| `AGENT05006` | 商店代號在前贅字後只能放數字 |
| `AGENT05007` | ReturnURL 網址格式錯誤 |
| `AGENT05008` | 帶入合作商店方式錯誤：無連結建店 |

### 11.6 聯絡人（AGENT06*）

| 錯誤碼 | 說明 |
|--------|------|
| `AGENT06001` | 請填入聯絡人姓名 |
| `AGENT06002` | 請填入聯絡人英文姓名 |
| `AGENT06003` | 請填入聯絡人電話 |
| `AGENT06004` | 聯絡人電話長度過長 |
| `AGENT06005` | 聯絡人電話格式錯誤，區碼須為 0 開頭且格式需為：`【區碼-電話號碼】`，例 `02-9888888` |
| `AGENT06006` | 請填入聯絡人行動電話 |
| `AGENT06007` | 聯絡人行動電話長度過長 |
| `AGENT06008` | 聯絡人行動電話格式不符 |
| `AGENT06009` | `聯絡人行動電話` 與 `聯絡人電話` 請至少擇一填寫 |

### 11.7 商店支付設定（AGENT07*）

| 錯誤碼 | 說明 |
|--------|------|
| `AGENT07001` | 請填入至少一種支付方式 |
| `AGENT07002` | 支付方式設定錯誤 |
| `AGENT07003` | 系統錯誤，請稍後再試（商店編號未正確儲存） |
| `AGENT07004` | 手續費設定錯誤，可能是低於預設值或參數值錯誤 |
| `AGENT07005` | 撥款天期設定錯誤，可能是低於預設值或參數值錯誤 |
| `AGENT07006` | 系統錯誤，請稍後再試（更新商店手續費失敗） |
| `AGENT07007` | 系統錯誤，請稍後再試（回傳參數空白） |
| `AGENT07008` | 系統錯誤，請稍後再試（綁定商店資料異常） |
| `AGENT07009` | 不接受此會員類型 |
| `AGENT07010` | 來源 IP 錯誤 |
| `AGENT07011` | 商品交付完成時間不可空白 |
| `AGENT07012` | 商品交付時間需為數字型態 |
| `AGENT07013` | 商品交付時間超過長度 |
| `AGENT07014` | 商品交付類型比例不可空白 |
| `AGENT07015` | 履約保證類型不可空白 |
| `AGENT07016` | 商品平均客單價不可空白 |
| `AGENT07017` | 商品平均客單價需為數字型態 |
| `AGENT07018` | 請填入至少一種商店交易類型 |
| `AGENT07019` | 商店交易類型設定錯誤 |
| `AGENT07020` | 請填入至少一種手續費支付設定 |
| `AGENT07021` | 手續費率設定錯誤 |
| `AGENT07022` | 商品交付類型比例設定錯誤 |
| `AGENT07023` | 查無商店代號 |
| `AGENT07024` | 設定免費提領次數異常 |
| `AGENT07025` | 支付關閉方式設定錯誤或同時帶入支付啟用 & 關閉 |
| `AGENT07026` | 商店網址或商店敘述請擇一填入 |
| `AGENT07027` | 請勿查詢非屬於您的商店資料 |
| `AGENT07028` | 商品平均客單價須大於 0 |
| `AGENT07030` | `CloseApplySwitch` 與 `OpenApplySwitch` 不可有相同參數值 |
| `AGENT07031` | 變更開通按鈕顯示設定錯誤，以下支付工具已申請開通無法變更 |

### 11.8 負責人（AGENT08*）

| 錯誤碼 | 說明 |
|--------|------|
| `AGENT08001` | 請輸入負責人姓名 |
| `AGENT08002` | 請輸入負責人身份證號碼 |
| `AGENT08004` | `負責人身分證字號` 格式錯誤，僅允許 A 至 Z 與數字 |
| `AGENT08005` | `主聯絡人電子郵件` 格式錯誤 |

### 11.9 主聯絡人（AGENT09*）

| 錯誤碼 | 說明 |
|--------|------|
| `AGENT09001` | 請輸入主聯絡人姓名 |
| `AGENT09002` | 請輸入主聯絡人電話 |
| `AGENT09003` | 請輸入主聯絡人 E-Mail |

### 11.10 其他

- **信用卡錯誤碼**：參考 PayUni 信用卡 API 文件
- **虛擬帳號錯誤碼**：參考 PayUni 虛擬帳號 API 文件
- **超商代碼錯誤碼**：參考 PayUni 超商代碼 API 文件
- **多功能收付／撥款失敗代碼**：`agent_fee`、`grant` 等回傳非 `SUCCESS` 時，Message 依 PAYUNi 錯誤代碼顯示

---

## 12. Hawkr 實作對照

> 以下為 Hawkr（單體 seller 模組）串接代理商合作商店 API（`bind_merchant` / `update_merchant`）的實作對照。多功能收付／撥款相關 API 目前由 payment 側視需求串接，非本節範圍。

### 12.1 加解密機制（AES-256-GCM）

**串接金鑰**：PAYUNi 平台 → 代理商專區 → 合約及基本資料 取得 Hash Key（32 字元）+ Hash IV（16 字元）。

**AES-256-GCM 加密流程**：

```
1. 將請求參數序列化為 URL-encoded query string（key 按字母排序）
2. 使用 AES-256-GCM 加密（nonce = 16 bytes，非標準 12 bytes）
3. 分離 ciphertext 和 tag（tag = 最後 16 bytes）
4. 輸出格式: hex( base64(ciphertext) + ":::" + base64(tag) )
```

**SHA256 雜湊**：

```
HashInfo = SHA256( HashKey + EncryptInfo + HashIV ).toUpperCase()
```

### 12.2 MerID 格式

MerID = `HAWK` + `store_code`（例：`HAWK2612345678`）。
由 `model.FormatMerID(storeCode int64) string`（`internal/modules/seller/internal/model/payuni.go`）產生，確定性計算。`merId` 僅限 PayUni 付款／退款／Webhook 情境使用，不得當作業務身分鍵，見[Java 微服務開發規範 §5.2](../開發規範/Java%20微服務開發規範.md)。

### 12.3 PAYUNi 註冊流程

```
KYC 審核核准（seller 模組，同一個 transaction）
  → 建立 seller.payuni_registrations（bind_status = PENDING，MerID = HAWK{store_code}）
  → 排入 River job seller.payuni_sync（以 store_id 為唯一鍵）

seller.payuni_sync（internal/modules/seller/internal/payuni，Sync）
  1. transaction：鎖註冊列、記下 intent_seq
  2. transaction 外：讀 KYC（解密）與店家資料，呼叫 PayUni（有 deadline，不存請求本體）
       PENDING → bind_merchant；BOUND 而且有資料沒送出 → update_merchant
  3. transaction：寫結果與事件（鎖順序：店家列、註冊列）
```

- 暫時性失敗（傳輸錯誤、PayUni 的暫時性錯誤碼）交給 River 退避重試；最後一次仍失敗，綁定記為 `FAILED`（`PAYUNI_UNAVAILABLE`）。永久失敗（驗證類錯誤碼、資料不完整、沒有設定）直接記錄；`FAILED` 由管理員重新綁定。
- 店家改聯絡資料或地址、核准、重新綁定都讓 `intent_seq` 加一；`synced_seq` 是 job 上次送出時讀到的值，兩者不等表示有東西還沒送。

### 12.4 審核狀態同步

- `seller.payuni_poll`：向 PayUni 查一次審核狀態（來源是輪詢或手動刷新），以 `store_id` 為唯一鍵。
- `seller.payuni_observe`：套用一則 PayUni 通知（`AGENT_NOTIFY`），以 `event_key` 為唯一鍵；MerID 形狀不合回 InvalidInput，格式合法但沒有這個店家則取消、不重試。
- `seller.payuni_sweep`：啟動即執行、每 15 分鐘的掃描，補排遺失的 sync 並排輪詢。
- PayUni 回報的原碼（`mer_status`、`member_status`）與推導的 `outcome` 由同一個條件式 UPDATE 寫入，只有 `executed_at` 比 `status_observed_at` 新的觀測會更新。

### 12.5 欄位對應（KYC 申請與店家 → PAYUNi）

| 來源 | PayUni 參數 | 備註 |
|------|------|------|
| KYC `RealName` | `Name`（個人）/ `MCName`（企業）/ `MBCName` | |
| KYC `IDNumber` | `UBN`（個人）/ `MCID`（企業） | |
| KYC `CompanyName` | `Name`（企業） | |
| KYC `TaxID` | `UBN`（企業） | |
| KYC 公司電話 | `ContactTel` / `MBCTel` | |
| 店家 `SupportEmail` | `Email` / `MerEmail` / `MBCMail` | |
| 店家 `ContactName` | `ContactName` | |
| 店家 `ContactMobile` | `ContactMobile` | |
| 店家 `ContactPhone` | `MerTel` | |
| 店家 `TownCode` + `AddressDetail` | `MerAddr`、`MerAddrPost`（twaddr 查表，3 碼郵遞區號） | |
| KYC `EnglishName` | `MerEngName` | 需符合 §7.3.4 字元集 |
| `HAWK + store_code` | `MerID` | `model.FormatMerID` |
| 店家 `StoreName` | `MerName` | |
| 店家 `Description`，空則 KYC `ProductType`，再空則店名 | `MerDesc` | |
| KYC 官網，空則店面頁網址 | `MerURL` | 兩者都沒有則缺項，不補預設值 |
| 固定 `2` | `MerType` | Hawkr = 實體商品 |
| 固定 `3` | `MerMCC` | 食品 |
| 固定 `100,0,0,0` | `GoodsRate` | 全非預付 |
| 固定 `CREDIT,ApplePay,GooglePay,ATM` | `Payment` | |
| 固定 `7` | `DeliverTime` | 7 個月 |
| 固定 `1` | `EscrowType` | 履保 |
| 固定 `1000` | `MerUnitPrice` | |
| 固定 `1` | `SellType` | 網路商店 |

`bind_merchant` 建立後，核心身分欄位（`Name`、`UBN`、`MerName`、`MerEngName`、`MerURL`、`MerDesc`、`MerType`、`MerMCC`、`MCName`、`MCID` 等）不可變更；`update_merchant` 只送聯絡方式與地址。

### 12.6 錯誤碼處置

PayUni 錯誤碼由 `ClassifyPayUniCode`（`internal/modules/seller/internal/model/payuni.go`）分類為使用者該採取的動作（`PayUniRemedy`）：`PayUniRetryable` 的暫時性碼由 job 重試，`AGENT04017` 走 §12.8 認領，其餘永久失敗記錄碼。

### 12.7 相關程式碼位置

| 角色 | 檔案路徑 |
|------|---------|
| PayUni HTTP Client | `internal/platform/payuni/client.go` |
| 加解密 | `internal/platform/payuni/crypto.go` |
| 綁定、同步、輪詢、通知 | `internal/modules/seller/internal/payuni/` |
| PayUni Domain（MerID、錯誤碼分類、審核結果推導） | `internal/modules/seller/internal/model/payuni.go` |
| DB Schema | `internal/modules/seller/db/schema.sql`（`seller.payuni_registrations`） |

### 12.8 MerID 衝突認領（AGENT04017）

當 `bind_merchant` 回傳 `AGENT04017`（商店代號已存在）時，改呼叫 `search_merchant_status` 查詢商店狀態並認領（`claimExisting`），所以結果不明（逾時）之後重送一律安全。

---

## 附錄：API Endpoint 速查表

| # | API 名稱 | Endpoint | 方式 | 版本 | 外層識別 | 用途 |
|---|---------|----------|------|------|---------|------|
| 1 | 多功能收付 | `/api/agent/agent_fee` | HTTP POST | 1.0 | AgentID | 代理商向旗下商店單筆收/付款（次日執行，1% 手續費） |
| 2 | 多功能收付交易查詢 | `/api/agent/agent_fee_query` | HTTP POST | 1.0 | AgentID | 查多功能收付交易結果與撥款狀態 |
| 3 | 多功能收付批次上傳 | SFTP（批次檔） | 檔案上傳 | — | 代理商前綴 | 批次多功能收付（次日執行） |
| 4 | 撥款指示 | `/api/trade/grant` | HTTP POST | 1.0 | AgentID | 代理商控制旗下商店單筆交易撥款（次日撥款，需 `GrantExport=1`） |
| 5 | 撥款查詢 | `/api/trade/grant_query` | HTTP POST | 1.0 | MerID + `IsPlatForm=1` | 查詢撥款當日所有撥款訂單明細（限 30 天內，每頁 500 筆） |
| 6 | 新增合作商店 | `/api/agent/bind_merchant` | HTTP POST | 1.0 | AgentID | 代理商將旗下商戶註冊為合作商店（自動建會員 + 建店） |
| 7 | 修改合作商店 | `/api/agent/update_merchant` | HTTP POST | 1.0 | AgentID | 修改既有商店聯絡方式/地址/支付設定 |
