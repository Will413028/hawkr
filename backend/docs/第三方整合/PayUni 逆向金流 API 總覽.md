# PAYUNi 逆向金流 API 官方文檔（統整版）

> **整合來源**：PAYUNi 開發專區官方 API 文件
> **最後更新**：2026-03-30
> **涵蓋範圍**：交易查詢、交易取消授權、信用卡請退款（退款/取消退款）、非信用卡退款轉匯（API 版 + 請求頁面版）、非信用卡退款轉匯取消

---

## 目錄

1. [通用規範](#1-通用規範)
2. [交易查詢 API](#2-交易查詢-api)
3. [交易取消授權 API（CREDIT）](#3-交易取消授權-apicredit)
4. [信用卡請退款 API（CREDIT）](#4-信用卡請退款-apicredit)
5. [非信用卡退款轉匯 API（API 版）](#5-非信用卡退款轉匯-apiapi-版)
6. [非信用卡退款轉匯 API（請求頁面版）](#6-非信用卡退款轉匯-api請求頁面版)
7. [非信用卡退款轉匯取消 API](#7-非信用卡退款轉匯取消-api)
8. [逆向金流操作對照表](#8-逆向金流操作對照表)

---

## 1. 通用規範

### 1.1 環境

| 環境 | 域名 |
|------|------|
| 測試區 | `https://sandbox-api.payuni.com.tw` |
| 正式區 | `https://api.payuni.com.tw` |

### 1.2 安全與加密

- 所有傳輸必須採用 **TLS v1.2** 以上協定
- 請於 header 加入 `user-agent`，建議內容為 `"payuni"`
- 串接金鑰：登入 PAYUNi 平台 → 商店串接資訊 → 取得 **Hash Key** 及 **Hash IV**

### 1.3 統一請求外層參數（商店 → 系統）

所有 API 的外層參數結構一致：

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| Version | Y | String | 版本 | `1.0`（交易查詢為 `2.0`） |
| EncryptInfo | Y | String | AES 加密字串 | 請參考「資料加解密」文件 |
| HashInfo | Y | String | SHA256 加密字串 | 請參考「資料加解密」文件 |

### 1.4 統一返回外層參數（系統 → 商店）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=成功，失敗請參考錯誤代碼 |
| MerID | String | 商店代號 | |
| Version | String | 版本 | `1.0`（交易查詢為 `2.0`） |
| EncryptInfo | String | 加密字串 | 請參考「資料加解密」文件 |
| HashInfo | String | 加密 Hash | 請參考「資料加解密」文件 |

### 1.5 前置作業

1. 於 PAYUNi 平台註冊會員並建立收款商店，取得 **MerID**
2. 申請開通所需支付工具
3. 登入 PAYUNi 平台取得 Hash Key 及 Hash IV

---

## 2. 交易查詢 API

### 2.1 概述

查詢交易狀態，支援所有支付方式：信用卡、ATM 轉帳、超商代碼交易、icash Pay、AFTEE 先享後付、LINE Pay、超商取貨付款、黑貓宅配貨到付款、街口支付交易。

### 2.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/trade/query` |
| 正式區 URL | `https://api.payuni.com.tw/api/trade/query` |
| 請求方式 | HTTP POST |
| 版本 | 固定 `2.0` |

### 2.3 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| MerTradeNo | C | String | 商店訂單編號 | 限制長度: 25，格式: `[A-Za-z0-9_-]`，與 TradeNo 擇一 |
| TradeNo | C | String | UNi 序號 | 與 MerTradeNo 擇一 |
| Timestamp | Y | Int | 時間戳記 | |

### 2.4 返回參數（EncryptInfo）

> **注意**：由於可能多筆紀錄，故回傳資料統一為 **Result 陣列**（由 0 開始）。當建立訂單時有使用物流服務，回傳參數中將包含 `ShipTradeNo`（UNi 物流序號）。

#### 共用欄位

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=查詢成功，失敗請參考錯誤代碼 |
| Message | String | 狀態說明 | 查詢成功，失敗請參考錯誤代碼 |
| MerTradeNo | String | 商店訂單編號 | 限制長度: 25，格式: `[A-Za-z0-9_-]` |
| TradeNo | String | UNi 序號 | |
| TradeAmt | String | 訂單金額 | |
| TradeFee | String | 交易手續費 | 統一金流收取的手續費金額（見下方說明） |
| TradeStatus | String | 訂單狀態 | `0`=取號成功, `9`=未付款, `1`=已付款, `2`=付款失敗, `3`=付款取消, `4`=交易逾期, `8`=訂單待確認 |
| PaymentType | String | 支付工具 | `1`=信用卡, `2`=ATM 轉帳, `3`=條碼/代碼, `5`=取貨付款(超商), `6`=愛金卡(ICash), `7`=後支付(Aftee), `8`=退貨代收(C2B退貨便), `9`=LINEPay, `10`=宅配到付, `11`=街口支付 |
| PaymentDay | String | 支付日期 | 格式: `YYYY-MM-DD HH:II:SS` |
| CreateDay | String | 建立日期 | 格式: `YYYY-MM-DD HH:II:SS` |
| Gateway | String | 閘道 | `1`=單串, `2`=整合式支付頁(UPP), `3`=一頁式支付頁(UOP) |
| DataSource | String | 查詢結果狀態 | `A`=完整資料, `B`=處理中未完整（建議 B 時 10 分鐘後再查） |

> **TradeFee 說明**：
> 1. 當 `DataSource=A` 且 `TradeStatus=1` 時，才有正確手續費資訊，否則為 `0`
> 2. `PaymentType=6`(ICash) 交易手續費非統一金流收取，故參數值為 `"-"`；`PaymentType=9`(LINEPay) 交易手續費為統一金流收取的交易處理費

#### 信用卡專用欄位（PaymentType=1）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Card6No | String | 卡號前六碼 | |
| Card4No | String | 卡號後四碼 | |
| CardExp | String | 卡片到期日 | 格式: `MMYY` |
| CardInst | String | 分期數 | |
| AuthCode | String | 授權碼 | |
| AuthType | String | 授權類型 | `1`=一次, `2`=分期, `3`=紅利, `4`=Apple Pay, `5`=Google Pay, `6`=Samsung Pay, `7`=銀聯（註：2025/09/01 起不支援紅利交易） |
| CardBank | String | 發卡銀行(代碼) | 國內發卡行為銀行代碼(3碼)，非國內發卡行為 `"-"` |
| CloseStatus | String | 請款狀態 | `1`=請款申請中, `2`=請款成功, `3`=請款取消, `7`=請款處理中, `9`=未申請 |
| CloseAmt | String | 請款金額 | |
| RefundType | String | 退款類型 | `2`=退款, `3`=預計退款 |
| RefundStatus | String | 退款狀態 | `1`=退款申請中, `2`=退款成功, `3`=退款取消, `8`=退款處理中 |
| RefundAmt | String | 退款金額 | |
| RefundDay | String | 退款發動日期 | 格式: `YYYY-MM-DD HH:II:SS` |
| RemainAmt | String | 剩餘可退款金額 | |
| CoBrandCode | String | 聯名卡代號 | 聯名卡交易識別代號（需事先設定） |

> **信用卡退款僅提供查詢最後一筆退款紀錄**。
>
> **預計退款說明**：當發動退款時，若同一筆(UNi序號)交易尚有請款或退款處理中，此次退款會先記錄為預計退款，待前一次請款或退款處理完成後才轉為退款申請中。

#### 線下交易專用欄位（PaymentType=2,3,5,8,10）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| OffChannel | String | 取號通路別 | 銀行代碼(數字)/超商代碼(英文)。`PaymentType=5,8` 顯示 `SEVEN`；`PaymentType=10` 顯示 `TCAT` |
| OffPayChannel | String | 實際繳費通路別 | 銀行代碼(數字)/超商代碼(英文)。`PaymentType=5,8` 顯示 `SEVEN`；`PaymentType=10` 顯示 `-` |
| OffPayNo | String | 繳費代碼 | `PaymentType=5` 顯示出貨單號；`PaymentType=8` 顯示退貨便編號；`PaymentType=10` 顯示 `-`；其餘顯示繳費代碼 |
| OffExpireTime | String | 繳費截止日期 | 格式: `YYYY-MM-DD HH:II:SS`。`PaymentType=5,8` 無繳費截止日期 |

#### 愛金卡專用欄位（PaymentType=6）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Channel | String | 選擇通路 | 固定回 `ICASH` |
| PayChannel | String | 付款通路 | 固定回 `ICASH` |
| PayNo | String | 愛金卡序號 | |

#### 後支付專用欄位（PaymentType=7）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Channel | String | 選擇通路 | 固定回 `AFTEE` |
| PayChannel | String | 付款通路 | 固定回 `AFTEE` |
| PayNo | String | 後支付(Aftee)序號 | |

#### LINE Pay 專用欄位（PaymentType=9）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Channel | String | 選擇通路 | 固定回 `LINE` |
| PayChannel | String | 付款通路 | 固定回 `LINE` |
| PayNo | String | LINEPay 交易號碼 | |

#### 街口支付專用欄位（PaymentType=11）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Channel | String | 選擇通路 | 固定回 `JKoPay` |
| PayChannel | String | 付款通路 | 固定回 `JKoPay` |
| PayNo | String | JKoPay 交易號碼 | |
| JKoStrCupAmt | String | 店家街口券折抵 | |
| JKoChannel | String | 支付工具 | `account`=儲值帳戶, `bank`=銀行帳戶, `creditcard`=信用卡 |
| RemainAmt | String | 剩餘可退款金額 | |
| RefundLastDT | String | 最後退款日期 | |

#### 優惠券核銷欄位（有使用優惠券時額外回傳）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| PromoCode | String | 優惠碼 | |
| DiscountAmt | String | 折扣金額 | |
| OrderAmt | String | 原訂單金額 | |
| CouponFee | String | 核銷費 | |
| CampaignName | String | 活動名稱 | |
| CampaignNo | String | 活動序號 | |
| CouponName | String | 優惠券名稱 | |
| CouponNo | String | 優惠券序號 | |

### 2.5 交易流程

```
收款商店 → 傳遞交易查詢參數請求 → 統一金流
  ↑                               |
  └── 回傳參數請求資訊 ←←←←←←←←←←←┘
```

---

## 3. 交易取消授權 API（CREDIT）

### 3.1 概述

信用卡已完成授權的交易，**尚未執行請款**時，可透過此 API 向銀行發動取消授權訊息。

支援類型：
- 信用卡一次付清
- 分期付款
- 國外卡

> **適用時機**：已授權之信用卡交易，但尚未請款，若有取消訂單發生可自行發動取消授權。

### 3.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/trade/cancel` |
| 正式區 URL | `https://api.payuni.com.tw/api/trade/cancel` |
| 請求方式 | HTTP POST |
| 版本 | 固定 `1.0` |

### 3.3 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| TradeNo | Y | String | UNi 序號 | |
| Timestamp | Y | Int | 時間戳記 | 格式：`time()` |

### 3.4 返回參數（EncryptInfo）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=成功，失敗請參考錯誤代碼 |
| Message | String | 狀態說明 | 取消授權成功，失敗請參考錯誤代碼 |
| MerID | String | 商店代號 | |
| TradeNo | String | UNi 序號 | |

### 3.5 交易流程

```
付款人 → 取消訂單
  ↓
收款商店 → 傳送信用卡取消授權參數 → 統一金流
  ↑                                ↓
  └── 回傳結果 ←←←←←←←←←←←←← 傳送取消授權資訊 → 收單銀行
                                     ← 回覆取消授權結果 ←
```

---

## 4. 信用卡請退款 API（CREDIT）

### 4.1 概述

信用卡已完成授權的交易，可透過此 API 向銀行發動**退款**訊息。

主要操作：
- **退款**（CloseType=2）：已請款交易的退款
- **取消退款**（CloseType=-2）：撤回尚未完成的退款

> **關於自動請款**：PAYUNi 平台預設為**自動請款**模式，信用卡交易授權成功後會由平台自動向銀行請款，商店無需手動操作。因此本節僅聚焦於退款相關操作（CloseType=2 / -2）。取消授權（已授權未請款）請參閱 [§3 交易取消授權 API](#3-交易取消授權-apicredit)。若未來需切換為手動請款模式（需自行發動 CloseType=1 請款、CloseType=-1 取消請款），請參閱 PAYUNi 官方開發專區文件。

支援卡別與限制：

| 卡別 | 全額請退款 | 部分請退款 |
|------|----------|----------|
| 信用卡一次付清 | 支援 | 支援 |
| 分期付款 | 支援 | 不支援 |
| 銀聯卡交易 | 支援 | 不支援 |
| 國外卡 | 支援 | 支援 |

### 4.2 天期限制

- **退款天期**：請款完成後需於 **180 天內**退款

### 4.3 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/trade/close` |
| 正式區 URL | `https://api.payuni.com.tw/api/trade/close` |
| 請求方式 | HTTP POST |
| 版本 | 固定 `1.0` |

### 4.4 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| TradeNo | Y | String | UNi 序號 | |
| Timestamp | Y | Int | 時間戳記 | 格式：`time()` |
| CloseType | Y | Int | 關帳類型 | `1`=請款, `2`=退款, `-1`=取消請款, `-2`=取消退款 |
| TradeAmt | C | Int | 請退款金額 | 請款或退款時為**必填**；部分請退款時填入部分金額 |

### 4.5 返回參數（EncryptInfo）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=成功，失敗請參考錯誤代碼 |
| Message | String | 狀態說明 | 成功時回「處理成功」 |
| MerID | String | 商店代號 | |
| TradeNo | String | UNi 序號 | |
| CloseType | Int | 關帳類型 | `1`=請款, `2`=退款, `-1`=取消請款, `-2`=取消退款 |

### 4.6 交易流程

```
付款人 → 付款完成或交易退貨
  ↓
收款商店 → 傳送信用卡請退款參數 → 統一金流
  ↓                                ↓
  ← 回傳結果 ←←←←←←←←←←←←←← 傳送請退款資訊 → 收單銀行
                                    ← 回覆請退款結果 ←
```

---

## 5. 非信用卡退款轉匯 API（API 版）

### 5.1 概述

PAYUNi 平台提供非信用卡支付方式的退款功能，透過銀行轉帳方式將退款金額匯入買方指定銀行帳號。

支援支付方式：
- ATM 轉帳（PaymentType=2）
- 超商代碼（PaymentType=3）

### 5.2 重要規則

- **退款天期**：付款完成後需於 **180 天內**退款
- **退款流程**：商店發動退款後，PAYUNi 於**隔日**進行商店 UNi 帳戶扣款及匯款至買方銀行帳號，約 **2~3 個工作天**到帳
- **身分驗證**：買方身分證號或統編需與銀行留存的帳號持有人相同
- **取消限制**：發動退款**當日**可執行取消退款，**隔日則不可**取消退款
- **自動撥款**：若正向交易尚未撥款入帳，則會於退款發動日隔日主動撥款
- **申請開通**：會員須向 PAYUNi 提出申請，審核開通且綁定幕後 IP 即可使用
- **手續費**：此服務需額外收取轉帳退款手續費，詳見官網費用一覽

### 5.3 測試區說明

測試區為方便測試，會於發動後**立即**執行 UNi 帳戶扣款及模擬匯款處理完成（正式環境為隔日處理）。

### 5.4 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/trade/offline/refund` |
| 正式區 URL | `https://api.payuni.com.tw/api/trade/offline/refund` |
| 請求方式 | HTTP POST |
| 版本 | 固定 `1.0` |

### 5.5 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| TradeNo | Y | String | UNi 序號 | |
| Timestamp | Y | Int | 時間戳記 | 格式：`time()` |
| PaymentType | Y | Int | 支付方式 | `2`=ATM 轉帳, `3`=超商代碼 |
| RefundAmt | Y | Int | 退款金額 | 須在訂單金額之內 |
| BankCode | Y | String | 轉入銀行代碼 | 3 碼銀行代碼 |
| BankSubCode | Y | String | 轉入銀行分行代碼 | 4 碼分行代碼 |
| BankAccountNo | Y | String | 轉入銀行帳戶號碼 | |
| UID | Y | String | 身分證字號或統編 | 需與銀行帳號持有人相同 |

### 5.6 返回參數（EncryptInfo）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=退款轉匯請求接收成功，失敗請參考錯誤代碼 |
| Message | String | 狀態說明 | 成功時回「退款轉匯請求接收成功」 |
| MerID | String | 商店代號 | |
| MerTradeNo | String | 商店訂單編號 | |
| TradeNo | String | UNi 序號 | |
| RefundAmt | Int | 退款金額 | |
| PaymentType | Int | 支付工具 | `2`=ATM 轉帳, `3`=超商代碼 |
| EstimatedRefundDate | String | UNi 帳戶扣款日期 | 格式：`YYYY-MM-DD`，失敗時固定回 `0000-00-00` |

### 5.7 退款轉匯結果 Notify（PAYUNi → 商店）

退款轉匯實際匯出結果，將 Notify 至原交易所指定之 NotifyURL。

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=退款轉匯處理成功, `FAIL`=退款轉匯失敗 |
| Message | String | 狀態說明 | 同 Status |
| MerID | String | 商店代號 | |
| MerTradeNo | String | 商店訂單編號 | |
| TradeNo | String | UNi 序號 | |
| RefundAmt | Int | 退款金額 | |
| PaymentType | Int | 支付工具 | `2`=ATM 轉帳, `3`=超商代碼 |
| RefundStatus | Int | 退款轉匯結果 | `1`=轉匯成功, `2`=轉匯失敗 |
| BankDate | String | 實際到帳日 | 格式：`YYYY-MM-DD`，失敗時為 `-` |

---

## 6. 非信用卡退款轉匯 API（請求頁面版）

> **與 API 版的差異**：請求頁面版由 PAYUNi 提供頁面收集買方銀行帳號資訊，商店無需自行傳入銀行帳號等敏感資料。適用於無法直接取得買方銀行資訊的場景。

### 6.1 概述

PAYUNi 平台提供非信用卡支付方式的退款功能，透過**請求頁面**方式讓買方自行填寫銀行帳號資訊，完成退款轉匯。

支援支付方式：
- ATM 轉帳（PaymentType=2）
- 超商代碼（PaymentType=3）

### 6.2 重要規則

- **退款天期**：付款完成後需於 **180 天內**退款
- **退款流程**：商店發動退款後，PAYUNi 於**隔日**進行商店 UNi 帳戶扣款及匯款至買方銀行帳號，約 **2~3 個工作天**到帳
- **身分驗證**：買方身分證號或統編需與銀行留存的帳號持有人相同
- **取消限制**：發動退款**當日**可執行取消退款，**隔日則不可**取消退款
- **自動撥款**：若正向交易尚未撥款入帳，則會於退款發動日隔日主動撥款
- **Notify**：執行請求非信用卡退款轉匯頁結果，將 Notify 至原交易所指定之 NotifyURL
- **申請開通**：會員須向 PAYUNi 提出申請，審核開通且綁定幕後 IP 即可使用
- **手續費**：此服務需額外收取轉帳退款手續費，詳見官網費用一覽

### 6.3 測試區說明

測試區為方便測試，會於發動後**立即**執行 UNi 帳戶扣款及模擬匯款處理完成（正式環境為隔日處理）。

### 6.4 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/offline_payment/refund` |
| 正式區 URL | `https://api.payuni.com.tw/api/offline_payment/refund` |
| 請求方式 | HTTP POST |
| 版本 | 固定 `1.0` |

> **代理商模式注意**：外層必須帶 `IsPlatForm=1`，否則 PayUni 會用商店自己的 Key/IV 解密，導致「資料解密失敗」。

### 6.5 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| TradeNo | Y | String | UNi 序號 | |
| Timestamp | Y | Int | 時間戳記 | 格式：`time()` |
| PaymentType | Y | Int | 支付方式 | `2`=ATM 轉帳, `3`=超商代碼 |
| RefundAmt | Y | Int | 退款金額 | 須在訂單金額之內 |
| ReturnURL | C | String | 完成後返回指定網址 | 若空值則呈現結果頁。格式：完整網址，僅限 80 與 443 port |
| BackURL | C | String | 返回商店按鈕網址 | 點擊後返回指定網址。格式：完整網址，僅限 80 與 443 port |

> **與 API 版差異**：不需傳入 `BankCode`、`BankSubCode`、`BankAccountNo`、`UID`，這些資訊由 PAYUNi 頁面向買方收集。

### 6.6 返回參數（EncryptInfo）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=退款轉匯請求接收成功，失敗請參考錯誤代碼 |
| Message | String | 狀態說明 | 成功時回「退款轉匯請求接收成功」 |
| MerID | String | 商店代號 | |
| MerTradeNo | String | 商店訂單編號 | |
| TradeNo | String | UNi 序號 | |
| RefundAmt | Int | 退款金額 | |
| PaymentType | Int | 支付工具 | `2`=ATM 轉帳, `3`=超商代碼 |
| EstimatedRefundDate | String | UNi 帳戶扣款日期 | 格式：`YYYY-MM-DD`，失敗時固定回 `0000-00-00` |

### 6.7 退款轉匯結果 Notify（PAYUNi → 商店）

退款轉匯實際匯出結果，將 Notify 至原交易所指定之 NotifyURL。

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=退款轉匯處理成功, `FAIL`=退款轉匯失敗 |
| Message | String | 狀態說明 | 同 Status |
| MerID | String | 商店代號 | |
| MerTradeNo | String | 商店訂單編號 | |
| TradeNo | String | UNi 序號 | |
| RefundAmt | Int | 退款金額 | |
| PaymentType | Int | 支付工具 | `2`=ATM 轉帳, `3`=超商代碼 |
| RefundStatus | Int | 退款轉匯結果 | `1`=轉匯成功, `2`=轉匯失敗 |
| BankDate | String | 實際到帳日 | 格式：`YYYY-MM-DD`，失敗時為 `-` |

---

## 7. 非信用卡退款轉匯取消 API

### 7.1 概述

若發動非信用卡退款轉匯後，因故須進行取消，可利用此 API 發動退款取消。

- 發動退款**當日**可執行取消退款
- **隔日則不可**取消退款

### 7.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/trade/offline/cancel_refund` |
| 正式區 URL | `https://api.payuni.com.tw/api/trade/offline/cancel_refund` |
| 請求方式 | HTTP POST |
| 版本 | 固定 `1.0` |

### 7.3 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| TradeNo | Y | String | UNi 序號 | 須為已請求退款且尚未處理轉匯的訂單 |
| Timestamp | Y | Int | 時間戳記 | 格式：`time()` |

### 7.4 返回參數（EncryptInfo）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=成功，失敗請參考錯誤代碼 |
| Message | String | 狀態說明 | 成功時回「退款轉匯取消處理成功」 |
| MerID | String | 商店代號 | |
| MerTradeNo | String | 商店訂單編號 | |
| TradeNo | String | UNi 序號 | |
| CancelAmt | Int | 取消退款的金額 | |
| PaymentType | Int | 支付工具 | `2`=ATM 轉帳, `3`=超商代碼 |

---

## 8. 逆向金流操作對照表

### 8.1 依支付方式選擇 API

| 支付方式 | 場景 | 使用 API | Endpoint |
|---------|------|---------|----------|
| 全部 | 查詢交易狀態 | 交易查詢 | `/api/trade/query` |
| 信用卡（已授權未請款） | 取消授權 | 交易取消授權 | `/api/trade/cancel` |
| 信用卡（已請款） | 退款 | 請退款（CloseType=2） | `/api/trade/close` |
| 信用卡（已發動退款） | 取消退款 | 請退款（CloseType=-2） | `/api/trade/close` |
| ATM 轉帳 | 退款（API 版，商店傳銀行帳號） | 非信用卡退款轉匯（API 版） | `/api/trade/offline/refund` |
| 超商代碼 | 退款（API 版，商店傳銀行帳號） | 非信用卡退款轉匯（API 版） | `/api/trade/offline/refund` |
| ATM 轉帳 | 退款（頁面版，PAYUNi 收集帳號） | 非信用卡退款轉匯（請求頁面版） | `/api/offline_payment/refund` |
| 超商代碼 | 退款（頁面版，PAYUNi 收集帳號） | 非信用卡退款轉匯（請求頁面版） | `/api/offline_payment/refund` |
| ATM 轉帳 / 超商代碼 | 取消退款（當日） | 非信用卡退款轉匯取消 | `/api/trade/offline/cancel_refund` |

### 8.2 信用卡交易生命週期與可用操作

```
授權成功 ─┬─ 取消授權（/api/trade/cancel）   → 授權取消（交易終止）
          └─ 自動請款 → 請款完成
                         ├─ 退款（CloseType=2）           → 退款中
                         │    ├─ 取消退款（CloseType=-2）  → 回到請款完成狀態
                         │    └─ 退款完成                  → 交易終止
                         └─（180 天內可退款）

※ 任何時點皆可透過交易查詢（/api/trade/query）查看最新狀態
```

### 8.3 非信用卡退款時間軸

```
付款完成（180 天內可退款）
  └─ 發動退款轉匯（/api/trade/offline/refund）
       ├─ 當日：可取消（/api/trade/offline/cancel_refund）
       └─ 隔日：PAYUNi 扣款 + 匯款 → 2~3 工作天到帳
            └─ Notify 結果至 NotifyURL（RefundStatus: 1=成功, 2=失敗）
```

---

## 附錄：API Endpoint 速查表

| # | API 名稱 | Endpoint | 方式 | 版本 | 用途 |
|---|---------|----------|------|------|------|
| 1 | 交易查詢 | `/api/trade/query` | POST | 2.0 | 查詢所有支付方式的交易狀態（信用卡/ATM/超商/LINE Pay 等） |
| 2 | 交易取消授權 | `/api/trade/cancel` | POST | 1.0 | 信用卡已授權未請款的取消授權 |
| 3 | 信用卡請退款 | `/api/trade/close` | POST | 1.0 | 退款 / 取消退款 |
| 4 | 非信用卡退款轉匯（API 版） | `/api/trade/offline/refund` | POST | 1.0 | ATM/超商代碼退款（商店傳入買方銀行帳號） |
| 5 | 非信用卡退款轉匯（請求頁面版） | `/api/offline_payment/refund` | POST | 1.0 | ATM/超商代碼退款（PAYUNi 頁面收集買方帳號） |
| 6 | 非信用卡退款轉匯取消 | `/api/trade/offline/cancel_refund` | POST | 1.0 | 取消當日發動的非信用卡退款 |

---

## 附錄：ATM 退款錯誤碼表

> 官方未於 API 文件列出，由客服回報整理。適用 `/api/trade/offline/refund`（API 版）與 `/api/offline_payment/refund`（頁面版）。

### REF01xxx — 基礎參數

| 錯誤碼 | 說明 |
|---|---|
| REF01001 | 已達連線處理上限，請稍後再試 |
| REF01002 | 未有商店代號 |
| REF01003 | 資料 HASH 比對不符合 |
| REF01004 | 資料解密失敗 |
| REF01005 | 解密資料不存在 |
| REF01006 | 查無符合商店資料 |
| REF01007 | 網路連線異常 |
| REF01017 | 未有代理商幕後相關設定 |
| REF01018 | 不提供此 IP 幕後交易 |
| REF01019 | 商店未有設定 AesType |
| REF01020 | 商店 AesType 不符合 |

### REF02xxx — 解密參數

| 錯誤碼 | 說明 |
|---|---|
| REF02001 | 未有商店代號 |
| REF02002 | 未有商店訂單編號 |
| REF02003 | 商店訂單編號超過長度限制 |
| REF02004 | 商店訂單編號格式錯誤 |
| REF02005 | 未有退款金額 |
| REF02006 | 退款金額僅可輸入整數 |
| REF02007 | 退款金額格式錯誤 |
| REF02008 | 未有時間戳記 |
| REF02009 | 時間戳記僅可輸入整數 |
| REF02010 | 時間戳記已過期 |
| REF02011 | 未有支付方式 |
| REF02012 | 無法辨識的支付方式 |
| REF02018 | 無銀行代碼（3 碼） |
| REF02019 | 銀行代碼格式錯誤 |
| REF02020 | 無銀行分行代碼 |
| REF02021 | 銀行分行代碼格式錯誤 |
| REF02022 | 無收款人金融帳戶號碼 |
| REF02023 | 收款人金融帳戶號碼格式錯誤 |
| REF02024 | 無收款人金融帳戶戶名 |
| REF02025 | 無收款人身分證字號或公司統編 |
| REF02026 | 未有 UNi 序號 |
| REF02028 | 收款人身分證字號或公司統編格式有誤 |
| REF02030 | UNi 序號超過長度限制 |
| REF02031 | UNi 序號格式有誤 |
| REF02032 | 返回商店網址格式有誤 |
| REF02033 | 前景導回網址格式錯誤 |
| REF02034 | 無法取得金融機構名單資料 |
| REF02035 | 查無該筆訂單資料 |
| REF02036 | 該訂單支付方式暫不支援退款轉匯相關功能 |
| REF02037 | 無足夠金額可退款 |
| REF02038 | 該筆退款訂單的支付工具類別有誤 |

### REF03xxx — 商店檢查

| 錯誤碼 | 說明 |
|---|---|
| REF03001 | 商店未啟用 |
| REF03002 | 商店未提供非信用卡退款轉匯幕後交易服務 |
| REF03003 | 商店未啟用非信用卡退款轉匯服務 |

### REF04xxx / REF06xxx — 異常

| 錯誤碼 | 說明 |
|---|---|
| REF04001 | 已存在相同商店訂單編號 |
| REF06001 | 未有 API 處理結果 |
| REF06002 | 回傳加密失敗 |

### REF05xxx — 業務狀態

| 錯誤碼 | 說明 |
|---|---|
| REF05001 | 查無該會員的詳細資料 |
| REF05002 | 查無該商店的虛擬資金資訊 |
| REF05003 | 該訂單不為付款完成狀態，無法退款 |
| REF05004 | 該訂單付款完成日期時間已超過可退款的期限 |
| REF05005 | 該帳戶的金融機構分行已不存在或合併，無法使用 |
| REF05006 | 已有退款處理中，請先取消退款再重新 |
| REF05007 | 更新支付金額資料失敗 |
| REF05008 | 更新預計撥款日期失敗 |
| REF05009 | 新增退款匯轉資料失敗 |
| REF05010 | 更新系統流水號末 6 碼失敗 |
| REF05011 | 金融機構代碼格式有誤 |
| REF05012 | 該金融機構分行已將業務移轉至其他分行，請更換其他金融帳戶 |
| REF05013 | 該筆退款資料已無法取消 |
| REF05014 | 已超過可取消退款操作的時間 |
| REF05015 | 更新該筆退款訂單狀態失敗 |
| REF05016 | 更新該筆訂單失敗 |
| REF05017 | 更新前筆退款轉匯紀錄狀態失敗 |
| REF05018 | 更新商店資金池失敗 |
| REF05019 | 新增帳務明細紀錄表失敗 |
| REF05020 | 還原預計撥款日期失敗 |
| REF05021 | 取消退款轉匯申請失敗 |
