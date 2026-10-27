# PAYUNi 金流 API 官方文檔（統整版）

> **整合來源**：PAYUNi 開發專區官方 API 文件
> **最後更新**：2026-03-10
> **涵蓋範圍**：整合式支付頁 UNiPaypage (UPP) + 虛擬帳號幕後 (ATM) + Apple Pay 幕後 + Google Pay 幕後
> **代理商專用 API**（撥款指示、多功能收付、撥款查詢、新增/修改合作商店）已收斂至《[PayUni 代理商專用 API 總覽](PayUni%20代理商專用%20API%20總覽.md)》

---

## 目錄

1. [通用規範](#1-通用規範)
2. [整合式支付頁 UNiPaypage (UPP)](#2-整合式支付頁-unipaypage-upp)
3. [虛擬帳號幕後 (ATM)](#3-虛擬帳號幕後-atm)
4. [Apple Pay 幕後](#4-apple-pay-幕後)
5. [Google Pay 幕後](#5-google-pay-幕後)
6. [代理商專用 API（已移至獨立文件）](#6-代理商專用-api已移至獨立文件)

---

## 1. 通用規範

### 1.1 環境

| 環境 | 平台域名 | API 域名 |
|------|--------|---------|
| 測試區 | `https://sandbox.payuni.com.tw` | `https://sandbox-api.payuni.com.tw` |
| 正式區 | `https://www.payuni.com.tw` | `https://api.payuni.com.tw` |

### 1.2 安全與加密

- 所有傳輸必須採用 **TLS v1.2** 以上協定
- 串接金鑰：登入 PAYUNi 平台 → 商店串接資訊 → 取得 **Hash Key** 及 **Hash IV**

### 1.3 統一請求外層參數（商店 → 系統）

所有 API 的外層參數結構一致：

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | 需與 EncryptInfo 中 MerID 參數值一致 |
| Version | Y | String | 版本 | 各 API 版本不同，詳見各節 |
| EncryptInfo | Y | String | AES 加密字串 | 請參考「資料加解密」文件 |
| HashInfo | Y | String | SHA256 加密字串 | 請參考「資料加解密」文件 |

### 1.4 統一返回外層參數（系統 → 商店）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=成功，`UNKNOWN`=等待授權結果逾期，`Unapproved`=訂單待確認，失敗請參考錯誤代碼 |
| MerID | String | 商店代號 | |
| Version | String | 版本 | |
| EncryptInfo | String | 加密字串 | 請參考「資料加解密」文件 |
| HashInfo | String | 加密 Hash | 請參考「資料加解密」文件 |

### 1.5 支付項目總覽

| 支付項目 | PaymentType | 支援與說明 |
|--------|-------------|----------|
| 信用卡 | 1 | Visa、MasterCard、JCB、銀聯。一次付清（含國內/外卡）、分期（3/6/9/12/18/24/30 期）、Apple Pay、Google Pay™、Samsung Pay |
| ATM 轉帳 | 2 | 支援單繳帳號（一次性虛擬帳號） |
| 超商代碼繳費 | 3 | 持超商代碼至全台統一超商，使用多媒體事務機列印繳費單臨櫃付款 |
| 貨到付款 | 5 | 7-ELEVEN 門市取貨付款，需開通物流 |
| 愛金卡 (ICash) | 6 | icash Pay 支付，需申請審核開通 |
| AFTEE 先享後付 | 7 | 先享後付，需申請審核開通 |
| LINE Pay | 9 | 需至 LINE Pay 平台申請取得 ID 與 KEY |
| 宅配貨到付款 | 10 | 黑貓宅配送貨到府付款，需開通物流 |
| 街口支付 (JKoPay) | 11 | 需申請審核開通 |
| 信用卡記憶卡號 | — | 付款時選擇記憶卡號，下次自動帶出 |
| 信用卡 Token | — | 首次交易約定後，後續可幕後授權。需申請審核開通並綁定 IP |

### 1.6 前置作業

1. 於 PAYUNi 平台註冊會員並建立收款商店，取得 **MerID**
2. 登入 PAYUNi 後台申請開通所需的支付工具
3. 取得 Hash Key 及 Hash IV

### 1.7 測試區信用卡測試卡號

| 用途 | 卡號 |
|------|------|
| 一次付清 | `4147631000000001`、`3560511000000001` |
| 一次付清（模擬 3D 交易 ECI 值不符主動取消授權） | `4147631000000002`、`3560511000000002` |
| 分期付款（不支援 9 期） | `4147632000000001`、`3560512000000001` |
| 銀聯卡 | `6200000000000001` |

- 卡片到期日及背面末三碼可任意填入
- Apple Pay、Google Pay、Samsung Pay 不限卡號於測試區皆為模擬成功
- LINE Pay：Channel ID 與 Channel Secret Key 可填隨意數字
- 測試區 ATM 轉帳、超商代碼繳費：登入測試區於交易動態明細點選「模擬繳費」按鈕

---

## 2. 整合式支付頁 UNiPaypage (UPP)

### 2.1 概述

PAYUNi 整合式支付頁（UPP）提供 RWD 設計的支付頁面，整合所有支付方式，消費者可在不同設備的瀏覽器上呈現最佳的瀏覽支付頁面。

### 2.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/upp` |
| 正式區 URL | `https://api.payuni.com.tw/api/upp` |
| 請求方式 | Form POST（使用 HTML `<form>` 元素提交數據） |
| 版本 | 固定 `2.0` |

### 2.3 請求參數（EncryptInfo）

#### 基本參數

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| MerTradeNo | Y | String | 商店訂單編號 | 限制長度 25，格式：`[A-Za-z0-9_-]` |
| TradeAmt | Y | Int | 訂單金額 | |
| Timestamp | Y | Int | 時間戳記 | 格式：`time()` |
| ReturnURL | C | String | 前景通知網址 | 付款完成返回指定網址（Form Post）。若空值，付款後呈現 PAYUNi 付款結果頁或取號完成頁面。**交易結果請以 NotifyURL 為主** |
| DeepLinkURL | C | String | Deep Link 網址 | 可打開特定的應用內容（APP、網站等）。**測試環境將略過此欄位視為空值處理** |
| NotifyURL | C | String | 背景通知網址 | 將交易資料通知指定網址 |
| BackURL | C | String | 返回商店按鈕網址 | PAYUNi 付款頁、付款結果頁、取號完成頁面顯示的返回商店按鈕，點擊後返回指定網址 |
| UsrMail | C | String | 消費者信箱 | |
| UsrMailFix | C | Int | 消費者信箱固定 | 若無帶入 UsrMail 參數，則信箱預設可修改 |
| Cardholder | C | Int | 持卡人英文名稱 | 啟用信用卡 3D 交易時需輸入，供發卡行驗證 |
| ProdDesc | Y | String | 商品說明 | |
| ExpireDate | C | String | 繳費有效日期 | 超商代碼或 ATM 轉帳。最少值為當日，超商代碼最大值為當日+7 天，ATM 轉帳最大值為當日+180 天。當設定大於當日+7 天時，支付頁不顯示超商代碼支付方式。若設定為當日，請注意訂單成立後至少需有 2 小時的繳費時間。如未帶此參數預設為當日+7 天 |
| AtmBankType | C | String | 指定 ATM 銀行 | |
| TradeLExpireSec | C | Int | 付款頁面交易截止秒數 | 若未帶此參數則預設為 600 秒 |
| Lang | C | String | 語系 | |

#### 支付工具參數

若未帶任何支付工具參數，則依據後台之開啟項目決定付款頁面啟用項目。

| 參數 | 必要 | 類型 | 說明 |
|------|------|------|------|
| Credit | C | Int | 信用卡一次付清 |
| CreditInst | C | String | 信用卡分期支付 |
| CreditUnionPay | C | Int | 信用卡（銀聯）支付 |
| ATM | C | Int | 虛擬帳號支付 |
| CVS | C | Int | 超商代碼/條碼支付 |
| ICash | C | Int | icash Pay 支付 |
| Aftee | C | Int | AFTEE 先享後付 |
| LinePay | C | Int | LINE Pay |
| JKoPay | C | Int | 街口支付 |
| ApplePay | C | Int | Apple Pay |
| GooglePay | C | Int | Google Pay™ |
| SamsungPay | C | Int | Samsung Pay |
| API3D | C | Int | 指定 3D |
| Union3D | C | Int | 指定銀聯 Unionpay |
| Ship | C | Int | 貨到付款 |
| TradeInvoice | C | Int | 電子發票 |

#### 首次信用卡 Token 交易參數

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| CreditToken | C | String | 信用卡 Token | 付款人綁定資料使用（例：會員編號、Email、手機等）。如使用 UseTokenType 參數，此為必填。長度限制 150，格式：`[A-Z a-z 0-9 @.#$%_-]` |
| UseTokenType | C | Int | Token 類型 | 1=約定信用卡（至付款頁面消費者可選擇），2=記憶卡號功能，3=強制約定信用卡（消費者無法取消） |
| CreditShowType | C | Int | 記憶卡號顯示類型 | 1=卡號，2=卡號+到期日（預設） |
| CreditTokenType | C | Int | Token 紀錄類型 | 1=會員（預設，旗下所有商店代號共用），2=商店（僅限首次交易商店代號） |
| CreditTokenExpired | C | String | Token 有效期間 | 格式：`MMYY`。若未帶此參數，則預設以該信用卡到期日為主 |

#### 首次買方 Token 交易參數

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| BuyerToken | C | String | 綁定買方 Token | 買方會員綁定資料使用（例：會員編號、Email、手機等），並由買方會員於前景頁面完成登入。如使用 BuyerToken 參數，此為必填 |
| BuyerTokenType | C | Int | 買方綁定類型 | 如使用此參數，BuyerToken 參數為必填。會員=旗下所有商店代號皆可使用，商店=僅限於首次交易的商店代號 |

#### 買方 Token 交易參數

需先經「首次買方 Token」交易，取得 BuyerHash 值，於後續交易可使用。

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| BuyerHash | C | String | 買方會員 Token Hash | 帶入買方 Hash，該筆交易可完成買方驗證並綁定交易於該會員身分上。BuyerHash 須經由 UPP API 首次交易帶入 BuyerToken，且經由 User 透過 UPP 支付頁登入並完成交易後即可取得 |

#### 優惠券服務參數

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| Coupon | C | Int | 是否啟用優惠券 | 1=啟用，2=停用 |
| CouponNotifyURL | C | String | 優惠券發券背景通知網址 | 將優惠券通知指定網址 |

#### 物流服務參數

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| ShipTag | C | Int | 是否啟用物流 | 1=啟用物流（預設含取貨不付款及取貨付款）。不啟用則不帶入該參數 |
| LgsType | C | String | 物流通路 | `B2C`=大宗寄倉，`C2C`=店到店，`HOME`=黑貓宅配。不啟用物流則不帶入 |
| ShipType | C | Int | 通路類別 | 1=7-ELEVEN，2=黑貓宅配。若物流通路為 B2C 或 C2C 請帶 1，為 HOME 請帶 2 |
| GoodsType | C | Int | 寄件型態 | 1=常溫，2=冷凍，3=冷藏。冷藏僅黑貓宅配支援 |
| Consignee | C | String | 取件人姓名 | 限制長度 10。最長 5 個中文字、最短至少 2 個中文字或 4 個英文字 |
| ConsigneeMobile | C | String | 取件人手機號碼 | 限填手機號碼 09 開頭，半形數字 |
| ConsigneeAddress | C | String | 取件人地址 | 不啟用黑貓物流則不帶入 |
| ConsigneeAddressFix | C | Int | 取件人地址固定 | 1=不可修改。若無帶此參數，則收件人地址預設可修改 |

#### 物流參數使用範例

| ShipTag=1 | Ship=1 | 指定支付工具參數 | 支付頁顯示結果 |
|-----------|--------|--------------|------------|
| V | | | 取貨付款 + 商店預設支付工具（搭配物流取貨不付款） |
| V | | V | 指定支付工具（搭配物流取貨不付款） |
| V | V | V | 取貨付款 + 指定支付工具（搭配物流取貨不付款） |
| | V | | 取貨付款 |
| | V | V | 取貨付款 + 指定支付工具（無物流） |

**備註**：

1. 當帶 ShipTag=1 時，表示啟用物流（預設含取貨不付款及取貨付款），若有帶支付工具參數時，則依參數設定有帶 Ship=1 時才有取貨付款
2. 當帶 Ship=1 且未帶 ShipTag=1 時，僅有取貨付款，無取貨不付款
3. 當有啟用物流時（Ship=1 或 ShipTag=1），則需傳遞 LgsType、ShipType、GoodsType、Consignee、ConsigneeMobile 參數
4. 若沒有帶（Ship=1 或 ShipTag=1）時，則視為一般交易，無物流服務，亦不產生物流單

### 2.4 返回參數（EncryptInfo）

#### 通用返回參數

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=成功，`UNKNOWN`=等待授權結果逾期，`UNAPPROVED`=訂單待確認（買家會員資格審查中），失敗請參考錯誤代碼 |
| Message | String | 狀態說明 | 授權成功=信用卡授權成功，(CVS)建立成功=超商代碼取號成功，(ATM)建立成功=ATM 轉帳取號成功。UNKNOWN=系統忙碌中，尚未確認交易結果（當 60 秒無收到銀行回應會先回覆 UNKNOWN） |
| MerID | String | 商店代號 | |
| MerTradeNo | String | 商店訂單編號 | 限制長度 25，格式：`[A-Za-z0-9_-]` |
| TradeNo | String | UNi 序號 | |
| TradeAmt | Int | 訂單金額 | |
| TradeStatus | Int | 訂單狀態 | 0=取號成功，1=已付款，2=付款失敗，3=付款取消，8=訂單待確認 |
| PaymentType | Int | 支付工具 | 1=信用卡，2=ATM 轉帳，3=代碼，5=貨到付款（超商取貨付款），6=愛金卡，7=後支付，9=LinePay，10=宅配到付，11=JKoPay |
| Gateway | Int | 交易標記 | 固定 2=整合式支付頁 UNiPaypage (UPP) |
| BuyerHash | String | 買方會員 Token Hash | Token 專用返回參數。需在初次交易帶入 BuyerToken，並由買方登入或註冊買方會員，完成交易後回傳 |

#### 信用卡（PaymentType=1）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Card6No | String | 卡號前六碼 | |
| Card4No | String | 卡號後四碼 | |
| CardInst | String | 分期數 | |
| FirstAmt | String | 首期金額 | |
| EachAmt | String | 每期金額 | |
| ResCode | String | 回應碼 | |
| ResCodeMsg | String | 回應碼敘述 | |
| AuthCode | String | 授權碼 | |
| AuthBank | String | 授權銀行（代碼） | |
| AuthBankName | String | 授權銀行（名稱） | |
| AuthType | Int | 授權類型 | 1=一次，2=分期，4=Apple Pay，5=Google Pay，6=Samsung Pay，7=銀聯 |
| AuthDay | String | 授權日期 | 格式：`YYYYMMDD` |
| AuthTime | String | 授權時間 | 格式：`HHIISS` |
| CreditHash | String | 信用卡 Token Hash | Token 專用返回參數，有 CreditToken 且授權成功才會壓碼 |
| CreditLife | String | Token 有效日期 | 格式：`MMYY` |
| CardBank | String | 發卡銀行（代碼） | 若為國內發卡行則為銀行代碼（3 碼），若非國內發卡行則為 `-` |
| CoBrandCode | String | 聯名卡代號 | 聯名卡交易識別代號（需事先設定） |

#### 虛擬帳號（PaymentType=2）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| BankType | String | 銀行（代碼） | 請參考銀行代碼（數字） |
| PayNo | String | 繳費虛擬帳號 | |
| PaySet | Int | 繳費設定 | 1=一次性 |
| ExpireDate | String | 繳費截止時間 | 格式：`YYYY-MM-DD HH:II:SS` |

#### 超商代碼（PaymentType=3）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Store | String | 超商（代碼） | 7-ELEVEN |
| PayNo | String | 繳費代碼 | |
| ExpireDate | String | 繳費截止時間 | 格式：`YYYY-MM-DD HH:II:SS` |

#### 純取貨（ShipTag=1）/ 貨到付款（PaymentType=5）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| PartnerId | String | 母代碼 | LgsType=B2C，長度限制 3 |
| ShipTradeNo | String | UNi 物流序號 | |
| GoodsType | Int | 寄件型態 | 1=常溫，2=冷凍 |
| LgsType | String | 物流型態 | `B2C`=大宗寄倉，`C2C`=店到店 |
| ShipType | Int | 通路類別 | 1=7-ELEVEN |
| ServiceType | Int | 取貨方式 | 1=取貨付款，3=取貨不付款 |
| ShipAmt | Int | 取貨付款金額 | |
| StoreID | String | 取件門市代碼 | |
| StoreName | String | 取件門市名稱 | |
| StoreAddr | String | 取件門市地址 | |
| Consignee | String | 收件人名稱 | 限制長度 10 |
| ConsigneeMobile | String | 收件人手機號碼 | |
| ConsigneeMail | String | 收件人電子信箱 | |

#### 愛金卡（PaymentType=6）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| PayNo | String | 愛金卡交易序號 | |
| PayTime | String | 付款日期時間 | 格式：`YYYY-MM-DD HH:II:SS` |

#### 後支付 AFTEE（PaymentType=7）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| PayNo | String | Aftee 交易序號 | |
| CreateDT | String | 交易建立日期時間 | |

#### LINE Pay（PaymentType=9）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| PayNo | String | LINEPay 交易號碼 | |
| PayTime | String | 付款日期時間 | 格式：`YYYY-MM-DD HH:II:SS` |

#### 宅配到付（PaymentType=10）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| TradeType | Int | 宅配類別 | 固定 1=正物流 |
| ShipTradeNo | String | 物流單號 | |
| GoodsType | Int | 寄件型態 | 1=常溫，2=冷凍，3=冷藏 |
| LgsType | String | 物流型態 | `HOME`=黑貓宅配 |
| ShipType | Int | 通路類別 | 2=黑貓宅配 |
| ServiceType | Int | 取貨方式 | 1=取貨付款，3=取貨不付款 |
| ShipAmt | Int | 取貨付款金額 | |
| Consignee | String | 收件人名稱 | 限制長度 10 |
| ConsigneeMobile | String | 收件人手機號碼 | |
| ConsigneeTel | String | 收件人聯絡電話 | 區碼+號碼。若有帶時回 `00-00000000`，若交易當下沒帶時回 `-` |
| ConsigneeAddress | String | 收件人地址 | |
| DeliveryTimeTag | String | 希望配達時段 | `01`=13 時前，`02`=14-18 時，`04`=不指定 |
| ProductTypeId | String | 商品類別代碼 | 0001~0015（見物流 API 總覽） |
| ProdDesc | String | 商品名稱 | |

#### 街口支付（PaymentType=11）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| JKoTradeNo | String | JKoPay 交易號碼 | |
| JKoStrCupAmt | String | 店家街口券折抵 | |
| JKoChannel | String | 支付工具 | `account`=儲值帳戶，`bank`=銀行帳戶，`creditcard`=信用卡 |
| PayTime | String | 付款日期時間 | 格式：`YYYY-MM-DD HH:II:SS` |

#### 優惠券核銷返回參數

當該交易有使用優惠券核銷時，將額外回傳：

| 參數 | 類型 | 說明 |
|------|------|------|
| PromoCode | String | 優惠碼 |
| DiscountAmt | Int | 折扣金額 |
| OrderAmt | Int | 原訂單金額 |
| CouponFee | Int | 核銷費 |
| CampaignName | String | 活動名稱 |
| CampaignNo | String | 活動序號 |
| CouponName | String | 優惠券名稱 |
| CouponNo | String | 優惠券序號 |

### 2.5 測試區 DeepLinkURL 注意事項

- 為避免混淆測試結果，測試環境將自動忽略 DeepLinkURL 欄位，視為空值處理
- 若於測試環境提供 DeepLinkURL 欄位，因該欄位將被略過，請務必同時提供 ReturnURL，系統將改由 ReturnURL 導回交易結果頁面

---

## 3. 虛擬帳號幕後 (ATM)

### 3.1 概述

PAYUNi 提供 ATM 轉帳單串機制，可滿足幕後取號交易。會員須向 PAYUNi 提出申請且需綁定幕後取號 IP，待審核開通後即可使用。

### 3.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/atm` |
| 正式區 URL | `https://api.payuni.com.tw/api/atm` |
| 請求方式 | HTTP POST（請於 header 加入 `user-agent`，建議內容為 `"payuni"`） |
| 版本 | 固定 `1.3` |

### 3.3 請求參數（EncryptInfo）

#### 基本參數

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| MerTradeNo | Y | String | 商店訂單編號 | 限制長度 25，格式：`[A-Za-z0-9_-]`，10 分鐘內不可重複 |
| TradeAmt | Y | Int | 訂單金額 | 請參考訂單金額限制說明 |
| Timestamp | Y | Int | 時間戳記 | 格式：`time()` |
| BankType | Y | String | 銀行（代碼） | 請參考銀行代碼（數字） |
| PaySet | C | Int | 繳費帳號類型 | 1=單繳帳號（一次性）（預設） |
| NotifyURL | C | String | 背景通知網址 | 將交易資料通知指定網址。格式：完整網址，僅限 80 與 443 port |
| UsrMail | Y/C | String | 消費者信箱 | 格式：信箱格式。若有開啟物流功能時此欄必填（視為物流收件人信箱）。若有開啟電子發票功能且 CarrierType=amego 時，此欄位必填 |
| ProdDesc | Y | String | 商品說明 | 長度限制 550，若超出則系統將自動截斷移除。可透過半形分號（`;`）帶入多個敘述 |
| ExpireDate | C | String | 繳費截止日期 | 格式：`YYYY-MM-DD`。PaySet=1 時，最少值為當日，最大值為當日+180 天。如未帶此參數預設為當日+7 天 |
| BuyerHash | C | String | 買方會員 Token Hash | 帶入買方 Token Hash 可完成買方驗證及交易綁定於此買方會員。若商店啟用開放交易金額上限，且交易金額大於訂單金額限制時，需帶入此參數。取得方式：請先使用整合式支付頁 UPP 帶入 BuyerToken 參數，消費者完成買方註冊或登入且交易完成後取得 |

#### 發票參數

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| CarrierType | Y/C | String | 發票載具類別 | 如需開立發票此參數必帶，無須開立則不用帶。`3J0002`=手機條碼，`CQ0001`=自然人憑證，`amego`=會員載具，`Donate`=捐贈碼，`Company`=公司發票 |
| CarrierInfo | Y/C | String | 載具內容 | 當 CarrierType 為 3J0002、CQ0001、Donate、Company 時必填。例：CarrierType=3J0002 時，CarrierInfo 需填手機條碼（含 `/`） |
| InvBuyerName | Y/C | String | 買方名稱或公司抬頭 | 當 CarrierType 有帶參數時，此欄位必填 |

#### 優惠券參數

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| PromoCode | Y | String | 優惠碼 | |
| DiscountAmt | Y | Int | 折扣金額 | |
| OrderAmt | Y | Int | 原訂單金額 | |
| CouponNotifyURL | C | String | 優惠券發券背景通知網址 | |

#### 物流參數（取貨不付款）

若有開啟物流功能時，使用虛擬帳號幕後 API 做純門市取貨/純送貨到宅（即取貨不付款），則需傳遞以下參數：

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| ServiceType | Y | String | 取件方式 | 固定為 3=取貨不付款 |
| Consignee | Y | String | 取件人姓名 | 限制長度 10 |
| ConsigneeMobile | Y | String | 取件人手機號碼 | 限填 09 開頭半形數字 |
| LgsType | Y | String | 物流型態 | `B2C`=大宗寄倉，`C2C`=店到店，`HOME`=黑貓宅配 |
| GoodsType | Y | Int | 寄件型態 | 1=常溫，2=冷凍，3=冷藏。僅黑貓宅配支援冷藏溫層 |
| ShipType | Y | Int | 通路類別 | 1=7-ELEVEN，2=黑貓宅配 |

**超商取貨不付款（ShipType=1）額外參數**：

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| StoreID | Y | String | 取件門市代碼 | 例如：`916712` |

**黑貓宅配取貨不付款（ShipType=2）額外參數**：

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| ConsigneeTelAreaCode | C | String | 收件人電話區碼 | 至少 2 碼、最多 3 碼數字 |
| ConsigneeTel | C | String | 收件人電話號碼 | 最多 8 碼數字 |
| ConsigneeAddress | Y | String | 收件人地址 | 最長 120 位。格式應為：縣市+鄉鎮市區+段弄巷街+號(+樓) |
| DeliveryTimeTag | Y | String | 希望配達時段 | `01`=13 時前，`02`=14-18 時，`04`=不指定 |

**備註**：有帶 ServiceType 時，當作取貨不付款，並檢查格式。

### 3.4 返回參數（EncryptInfo）

#### 基本返回

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=成功，失敗請參考錯誤代碼 |
| Message | String | 狀態說明 | `(ATM)建立成功`=ATM 轉帳取號成功 |
| MerID | String | 商店代號 | |
| MerTradeNo | String | 商店訂單編號 | 限制長度 25，格式：`[A-Za-z0-9_-]` |
| TradeNo | String | UNi 序號 | |
| TradeAmt | Int | 訂單金額 | |
| TradeStatus | Int | 訂單狀態 | 0=取號成功 |
| PaymentType | Int | 支付工具 | 2=ATM 轉帳 |
| BankType | String | 銀行（代碼） | 請參考銀行代碼（數字） |
| PayNo | String | 繳費虛擬帳號 | |
| PaySet | Int | 繳費帳號類型 | 1=單繳帳號（一次性） |
| ExpireDate | String | 繳費截止日期 | 格式：`YYYY-MM-DD HH:II:SS` |

#### 優惠券核銷返回

| 參數 | 類型 | 說明 |
|------|------|------|
| PromoCode | String | 優惠碼 |
| DiscountAmt | Int | 折扣金額 |
| OrderAmt | Int | 原訂單金額 |
| CouponFee | Int | 核銷費 |
| CampaignName | String | 活動名稱 |
| CampaignNo | String | 活動序號 |
| CouponName | String | 優惠券名稱 |
| CouponNo | String | 優惠券序號 |

#### 物流取貨不付款返回

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| ShipTradeNo | String | UNi 物流序號 | |
| ServiceType | Int | 取件方式 | 固定為 3=取貨不付款 |
| LgsType | String | 物流型態 | `B2C` / `C2C` / `HOME` |
| GoodsType | Int | 寄件型態 | 1=常溫，2=冷凍，3=冷藏 |
| ShipType | Int | 通路類別 | 1=7-ELEVEN，2=黑貓宅配 |
| ShipAmt | Int | 取貨付款金額 | 固定為 0 |
| Consignee | String | 取件人姓名 | |
| ConsigneeMobile | String | 取件人手機號碼 | |

**超商取貨不付款（ShipType=1）額外返回**：

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| PartnerId | String | 母代碼 | |
| StoreID | String | 門市代碼 | 例如：`916712` |
| StoreName | String | 門市名稱 | |
| StoreAddr | String | 門市地址 | |
| ConsigneeMail | String | 取件人電子信箱 | |

**黑貓宅配取貨不付款（ShipType=2）額外返回**：

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| TradeType | Int | 宅配類別 | 固定為 1=正物流 |
| ConsigneeTel | String | 收件人聯絡電話 | 區碼+號碼。若有帶時回 `00-00000000`，若交易當下沒帶時回 `-` |
| ConsigneeAddress | String | 收件人地址 | |
| DeliveryTimeTag | String | 希望配達時段 | `01`=13 時前，`02`=14-18 時，`04`=不指定 |
| ProductTypeId | String | 商品類別代碼 | 0001~0015 |
| ProdDesc | String | 商品名稱 | |

---

## 4. Apple Pay 幕後

### 4.1 概述

PAYUNi 提供 Apple Pay 單串機制，可滿足 Apple Pay 幕後授權交易。會員須向 PAYUNi 提出申請，審核開通並透過 PAYUNi 進行 Apple Pay 相關憑證申請作業，且綁定幕後授權 IP 即可使用。

### 4.2 前置作業

1. 於 PAYUNi 平台註冊會員，建立收款商店，取得 **MerID**，申請開通所需支付工具
2. 向 PAYUNi 提出 Apple Pay 幕後機制申請，並透過 PAYUNi 進行 Apple Pay 相關憑證申請作業（請參考 Apple Pay 憑證申請說明）
3. 需自行進行 Apple Pay 開發取得 **Payment token**（詳細開發說明請參閱 Apple 官網）

### 4.3 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/apple` |
| 正式區 URL | `https://api.payuni.com.tw/api/apple` |
| 請求方式 | HTTP POST（請於 header 加入 `user-agent`，建議內容為 `"payuni"`） |
| 版本 | 固定 `1.3` |

- 測試區為模擬交易成功，無須使用特定測試卡號

### 4.4 請求參數（EncryptInfo）

#### 基本參數

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| MerTradeNo | Y | String | 商店訂單編號 | 限制長度 25，格式：`[A-Za-z0-9_-]`，10 分鐘內不可重複 |
| TradeAmt | Y | Int | 訂單金額 | |
| Timestamp | Y | Int | 時間戳記 | 格式：`time()` |
| AppleObject | Y | String | Apple Pay Payment token | 請放入自行進行 Apple Pay 開發取得之 Payment token |
| AppleMode | C | String | Apple Pay 模式 | `1`=OnWeb（預設），`2`=InApp |
| NotifyURL | C | String | 背景通知網址 | 將交易資料通知指定網址，僅於平台點選補觸發時使用。格式：完整網址，僅限 80 與 443 port |
| UsrMail | Y/C | String | 消費者信箱 | 格式：信箱格式。若有開啟物流功能時此欄必填。若有開啟電子發票功能且 CarrierType=amego 時此欄必填 |
| ProdDesc | Y | String | 商品說明 | 長度限制 550，若超出則系統將自動截斷。可透過半形分號（`;`）帶入多個敘述 |
| UserIP | C | String | 消費者 IP | 若有帶入則會列入全平台風險管控機制，協助阻擋異常交易。格式：支援 IPv4 和 IPv6 格式 |

#### 發票參數

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| CarrierType | Y/C | String | 發票載具類別 | 如需開立發票此參數必帶。`3J0002`=手機條碼，`CQ0001`=自然人憑證，`amego`=會員載具，`Donate`=捐贈碼，`Company`=公司發票 |
| CarrierInfo | Y/C | String | 載具內容 | 當 CarrierType 為 3J0002、CQ0001、Donate、Company 時必填 |
| InvBuyerName | Y/C | String | 買方名稱或公司抬頭 | 當 CarrierType 有帶參數時，此欄必填 |
| ProdDesc | Y/C | String | 產品說明 | 當 CarrierType 有帶參數時此欄必填。長度限制 500 |

#### 優惠券參數

若有開啟優惠券功能時，使用優惠碼於幕後 API 則需傳遞以下參數：

| 參數 | 必要 | 類型 | 說明 |
|------|------|------|------|
| PromoCode | Y | String | 優惠碼 |
| DiscountAmt | Y | Int | 折扣金額 |
| OrderAmt | Y | Int | 原訂單金額 |
| CouponNotifyURL | C | String | 優惠券發券背景通知網址 |

#### 物流參數（取貨不付款）

若有開啟物流功能時，使用 Apple Pay 幕後 API 做純門市取貨/純送貨到宅（即取貨不付款），則需傳遞以下參數：

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| ServiceType | Y | String | 取件方式 | 固定為 3=取貨不付款 |
| Consignee | Y | String | 取件人姓名 | 限制長度 10。中文 5 個字，英文 10 個字 |
| ConsigneeMobile | Y | String | 取件人手機號碼 | 限填手機號碼，半形數字 |
| LgsType | Y | String | 物流型態 | `B2C`=大宗寄倉，`C2C`=店到店，`HOME`=黑貓宅配 |
| GoodsType | Y | Int | 寄件型態 | 1=常溫，2=冷凍，3=冷藏。僅黑貓宅配支援冷藏溫層 |
| ShipType | Y | Int | 通路類別 | 1=7-ELEVEN，2=黑貓宅配 |

**超商取貨不付款（ShipType=1）額外參數**：

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| StoreID | Y | String | 取件門市代碼 | 例如：`916712` |

**黑貓宅配取貨不付款（ShipType=2）額外參數**：

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| ConsigneeTelAreaCode | C | String | 收件人電話區碼 | 至少 2 碼、最多 3 碼數字 |
| ConsigneeTel | C | String | 收件人電話號碼 | 最多 8 碼數字 |
| ConsigneeAddress | Y | String | 收件人地址 | 最長 120 位。格式應為：縣市+鄉鎮市區+段弄巷街+號(+樓) |
| DeliveryTimeTag | Y | String | 希望配達時段 | `01`=13 時前，`02`=14-18 時，`04`=不指定 |

### 4.5 返回參數（EncryptInfo）

#### 基本返回

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=成功，失敗請參考錯誤代碼 |
| Message | String | 狀態說明 | 授權成功=信用卡授權成功，失敗請參考錯誤代碼 |
| MerID | String | 商店代號 | |
| MerTradeNo | String | 商店訂單編號 | 限制長度 25，格式：`[A-Za-z0-9_-]` |
| TradeNo | String | UNi 序號 | |
| TradeAmt | Int | 訂單金額 | |
| TradeStatus | Int | 訂單狀態 | 1=已付款，2=付款失敗，3=付款取消 |
| PaymentType | Int | 支付工具 | 1=信用卡 |
| Card6No | String | 卡號前六碼 | |
| Card4No | String | 卡號後四碼 | |
| CardInst | String | 分期數 | |
| FirstAmt | String | 首期金額 | |
| EachAmt | String | 每期金額 | |
| ResCode | String | 回應碼 | |
| ResCodeMsg | String | 回應碼敘述 | |
| AuthCode | String | 授權碼 | |
| AuthBank | String | 授權銀行（代碼） | |
| AuthBankName | String | 授權銀行（名稱） | |
| AuthType | Int | 授權類型 | 4=Apple Pay |
| AuthDay | String | 授權日期 | 格式：`YYYYMMDD` |
| AuthTime | String | 授權時間 | 格式：`HHIISS` |
| CardBank | String | 發卡銀行（代碼） | 若為國內發卡行則為銀行代碼（3 碼），若非國內發卡行則為 `-` |

#### 優惠券核銷返回

當該交易有使用優惠券核銷時，將額外回傳：

| 參數 | 類型 | 說明 |
|------|------|------|
| PromoCode | String | 優惠碼 |
| DiscountAmt | Int | 折扣金額 |
| OrderAmt | Int | 原訂單金額 |
| CouponFee | Int | 核銷費 |
| CampaignName | String | 活動名稱 |
| CampaignNo | String | 活動序號 |
| CouponName | String | 優惠券名稱 |
| CouponNo | String | 優惠券序號 |

#### 物流取貨不付款返回

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| ShipTradeNo | String | UNi 物流序號 | |
| ServiceType | Int | 取件方式 | 固定為 3=取貨不付款 |
| LgsType | String | 物流型態 | `B2C`=大宗寄倉，`C2C`=店到店，`HOME`=黑貓宅配 |
| GoodsType | Int | 寄件型態 | 1=常溫，2=冷凍，3=冷藏 |
| ShipType | Int | 通路類別 | 1=7-ELEVEN，2=黑貓宅配 |
| ShipAmt | Int | 取貨付款金額 | 固定為 0 |
| Consignee | String | 取件人姓名 | |
| ConsigneeMail | String | 取件人電子信箱 | |
| ConsigneeMobile | String | 取件人手機號碼 | |

**超商取貨不付款（ShipType=1）額外返回**：

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| StoreID | String | 取件門市代碼 | 例如：`916712` |
| StoreName | String | 取件門市名稱 | |
| StoreAddr | String | 取件門市地址 | |

**黑貓宅配取貨不付款（ShipType=2）額外返回**：

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| ConsigneeAddress | String | 收件人地址 | |
| DeliveryTimeTag | String | 希望配達時段 | `01`=13 時前，`02`=14-18 時，`04`=不指定 |

### 4.6 AppleObject 參數說明

> ⚠️ **此節以實證為準**：PayUni 官方文字敘述與範例之間存在矛盾，2026-06-06 經多輪 Sandbox 真實測試後，確定真實規格如下節「實證確認規格」。原始文字保留於後段作為對照。

#### 實證確認規格（OnWeb / InApp 通用）

```
AppleObject = Base64Encode( JSON.stringify( paymentToken.paymentData ) )
```

**重點**：

1. **AppleObject 只放 `paymentData` 一個物件**，不是整包 token，**不包**含 `paymentMethod` / `transactionIdentifier`
2. AppleObject 是**單層 Base64 字串**（無外層 wrapper、無 `amount`/`paymentTokenObject` JSON 包裹）
3. OnWeb / InApp 編碼方式**相同**，差別僅在 `AppleMode` 參數值（`1` vs `2`）

#### Apple Pay JS 原生回傳結構（Web）

```js
// Safari ApplePaySession 的 onpaymentauthorized 事件
event.payment.token = {
  paymentData: {                                  // ← AppleObject 只取這個物件
    data: "<Apple 加密過的 base64 密文>",
    signature: "<PKCS#7 簽章 base64>",
    header: {
      publicKeyHash: "<base64>",
      ephemeralPublicKey: "<base64>",
      transactionId: "<hex>"
    },
    version: "EC_v1"                              // 或 "RSA_v1"
  },
  paymentMethod: {                                // ← AppleObject 不取
    displayName: "MasterCard 4903",
    network: "MasterCard",
    type: "credit"
  },
  transactionIdentifier: "<hex>"                  // ← AppleObject 不取
}
```

#### 正確轉換流程（Web OnWeb）

```js
const paymentDataJson = JSON.stringify(event.payment.token.paymentData);
const appleObject = btoa(unescape(encodeURIComponent(paymentDataJson)));
// POST /api/apple with AppleObject=appleObject, AppleMode=1, ...
```

#### iOS PKPaymentToken 對應（InApp）

##### 原生 iOS（PKPaymentRequest）

`PKPaymentAuthorizationViewController` 的 delegate 回呼 `paymentAuthorizationController(_:didAuthorizePayment:handler:)` 取得 `PKPayment.token`，其結構：

| 屬性 | 對應 Apple Pay JS 的位置 | 是否放入 AppleObject |
|---|---|---|
| `paymentData` (Data) | `paymentToken.paymentData` 物件序列化後的 JSON bytes（含 `data` / `signature` / `header` / `version`） | ✓ |
| `paymentMethod` (PKPaymentMethod) | `paymentMethod` 物件 | ✗ |
| `transactionIdentifier` (String) | `transactionIdentifier` | ✗ |

iOS 平台已將 paymentData 直接序列化為 JSON `Data`，**caller 不需自己組裝 / 不需從 PKPaymentMethod 取資料**，把 `payment.token.paymentData` 轉 UTF-8 字串後 Base64Encode 即可。

##### Flutter `pay` 套件

`pay` 套件在 iOS 平台已替我們做完上述步驟：`showPaymentSelector` 回傳的 `result['token']` 直接就是 paymentData JSON 字串（**已抽出 paymentData，不含 paymentMethod / transactionIdentifier**），caller 只需 Base64Encode 即可：

```dart
final paymentDataJson = result['token'] as String;  // 已是 paymentData JSON
final appleObject = base64Encode(utf8.encode(paymentDataJson));
// POST /api/apple with AppleObject=appleObject, AppleMode=2, ...
```

##### InApp 文件文字與實證的差異

PayUni 官方文字：
> 若以 in app 方式串接 Apple Pay，取得以下格式的資訊，請使用 `Base64Encode` 後，放入 `AppleObject` 參數。

兩個未明確之處 + 實證補充：

| 文件未明確 | 實證結論 |
|---|---|
| 「以下格式的資訊」具體指什麼？ | = `paymentData` 物件（含 `data` / `signature` / `header` / `version`），**不包**含 `paymentMethod` / `transactionIdentifier` |
| 「Base64Encode」對什麼編碼？ | 對 `JSON.stringify(paymentData)` 結果的 UTF-8 bytes 做 base64 |
| 雙重 base64 / `paymentMethod` 是否一併送？ | 都會被 PayUni 拒（2026-06-04 ~ 06 buyer InApp 測試 6 筆失敗即為 caller 端多 base64 一次造成的雙重編碼 — 修正前 `pay_token_service.dart` `_extractBase64Token` 多做 `base64Encode`，後端再 base64 → AppleObject = base64(base64(JSON)) → 回 `APPLE01009 未提供此版本處理` / `送入參數 Decode 失敗`） |

InApp 與 OnWeb 殊途同歸：**最終 AppleObject 都是 single `Base64Encode(paymentData JSON)`**，差別僅在 `AppleMode` 標記值（`1` vs `2`）。

#### 實證紀錄

| 測試 | AppleObject 結構 | PayUni 回應 |
|---|---|---|
| 整包 token base64 | `Base64(JSON.stringify(token))` 含 paymentMethod / transactionIdentifier | `APPLE01009 未提供此版本處理` |
| 純 JSON 整包 token | `JSON.stringify(token)` 無 base64 | `APPLE01009 送入參數Decode失敗` |
| wrapper 結構 | `{"amount":"...","paymentTokenObject":"<base64>"}` | `APPLE01009 送入參數Decode失敗` |
| 整包 token base64（重試） | `Base64(JSON.stringify(token))` | `APPLE01009 未提供此版本處理` |
| **paymentData base64**（✓ 正解） | `Base64(JSON.stringify(token.paymentData))` | **通過解密** |

#### PayUni 官方文字（保留作對照，措辭與實證有差距）

> 若以 OnWeb 方式串接 Apple Pay，取得以下資訊後，僅需取 `paymentTokenObject` 之資訊，放入 `AppleObject` 參數。

> 若以 in app 方式串接 Apple Pay，取得以下格式的資訊，請使用 `Base64Encode` 後，放入 `AppleObject` 參數。

#### PayUni 官方範例（含命名與我方對齊）

```json
{
  "amount": "1",
  "paymentTokenObject": "<EncryptInfo 範例，原文略>"
}
```

**範例對照解讀**：
- 範例 `paymentTokenObject` 的 value 字串 `eyJk...` Base64 解碼後是 `{"data":"...","signature":"...","header":{...},"version":"EC_v1"}` — 對應 Apple Pay JS 的 `token.paymentData` 物件（不含 paymentMethod / transactionIdentifier）
- 範例外層 `{"amount":"1","paymentTokenObject":"..."}` JSON wrapper **並不是** AppleObject 的真實格式（實測會被 PayUni 拒）。**真實 AppleObject 只放範例裡 `paymentTokenObject` 那串 base64 字串**
- PayUni 文字「僅需取 paymentTokenObject 之資訊」實際意思 = 從 token 取出 paymentData → JSON 序列化 → Base64Encode

#### 我方實作位置

| 檔案 | 角色 |
|---|---|
| `services/java/payment-service/.../PaymentClient.java` `createApplePayPayment` | backend 端 base64（OnWeb / InApp 統一） |
| `hawkr-website/lib/payment/apple-pay.ts` `onpaymentauthorized` | web caller 抽取 `event.payment.token.paymentData` 並 `JSON.stringify` |
| `hawkr_buyer/lib/core/payment/pay_token_service.dart` `_extractRawToken` | iOS caller 取 `result['token']`（pay 套件已給 paymentData JSON） |

---

## 5. Google Pay 幕後

### 5.1 概述

PAYUNi 提供 Google Pay 單串機制，可滿足 Google Pay 幕後授權交易。會員須綁定幕後授權 IP 即可使用。

> **注意事項**：行動裝置僅支援瀏覽器 APP，若使用 APP 內建瀏覽器將無法使用 Google Pay 付款。

### 5.2 前置作業

1. 於 PAYUNi 平台註冊會員，建立收款商店，取得 **MerID**，申請開通所需支付工具
2. 向 PAYUNi 提出 Google Pay 幕後機制申請

### 5.3 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/google` |
| 正式區 URL | `https://api.payuni.com.tw/api/google` |
| 請求方式 | HTTP POST（請於 header 加入 `user-agent`，建議內容為 `"payuni"`） |
| 版本 | 固定 `1.3` |

- 測試區為模擬交易成功，無須使用特定測試卡號

### 5.4 請求參數（EncryptInfo）

#### 基本參數

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| MerTradeNo | Y | String | 商店訂單編號 | 限制長度 25，格式：`[A-Za-z0-9_-]`，10 分鐘內不可重複 |
| TradeAmt | Y | Int | 訂單金額 | |
| Timestamp | Y | Int | 時間戳記 | 格式：`time()` |
| GoogleObject | C | String | Google Pay Payment token | 請放入自行進行 Google Pay 開發取得之 Payment token |
| GoogleMode | C | String | Google Pay 模式 | `1`=OnWeb（預設），`2`=InApp |
| API3D | C | Int | 幕後強制 3D | `1`=強制 3D |
| NotifyURL | C | String | 背景通知網址 | 將交易資料通知指定網址，僅於平台點選補觸發時使用。格式：完整網址，僅限 80 與 443 port |
| UsrMail | Y/C | String | 消費者信箱 | 格式：信箱格式。若有開啟物流功能時此欄必填。若有開啟電子發票功能且 CarrierType=amego 時此欄必填 |
| ProdDesc | Y | String | 商品說明 | 長度限制 550，若超出則系統將自動截斷。可透過半形分號（`;`）帶入多個敘述 |
| UserIP | C | String | 消費者 IP | 若有帶入則會列入全平台風險管控機制，協助阻擋異常交易。格式：支援 IPv4 和 IPv6 格式 |

#### 發票參數

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| CarrierType | Y/C | String | 發票載具類別 | 如需開立發票此參數必帶。`3J0002`=手機條碼，`CQ0001`=自然人憑證，`amego`=會員載具，`Donate`=捐贈碼，`Company`=公司發票 |
| CarrierInfo | Y/C | String | 載具內容 | 當 CarrierType 為 3J0002、CQ0001、Donate、Company 時必填 |
| InvBuyerName | Y/C | String | 買方名稱或公司抬頭 | 當 CarrierType 有帶參數時，此欄必填 |
| ProdDesc | Y/C | String | 產品說明 | 當 CarrierType 有帶參數時此欄必填。長度限制 500 |
| UsrMail | Y/C | String | 消費者電子信箱 | 當 CarrierType 有帶參數時此欄必填。CarrierType=amego 時，UsrMail 為必填 |

#### 優惠券參數

若有開啟優惠券功能時，使用優惠碼於幕後 API 則需傳遞以下參數：

| 參數 | 必要 | 類型 | 說明 |
|------|------|------|------|
| PromoCode | Y | String | 優惠碼 |
| DiscountAmt | Y | Int | 折扣金額 |
| OrderAmt | Y | Int | 原訂單金額 |
| CouponNotifyURL | C | String | 優惠券發券背景通知網址 |

#### 物流參數（取貨不付款）

若有開啟物流功能時，使用 Google Pay 幕後 API 做純門市取貨/純送貨到宅（即取貨不付款），則需傳遞以下參數：

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| ServiceType | Y | String | 取件方式 | 固定為 3=取貨不付款 |
| Consignee | Y | String | 取件人姓名 | 限制長度 10。中文 5 個字，英文 10 個字 |
| ConsigneeMobile | Y | String | 取件人手機號碼 | 限填手機號碼 09 開頭，半形數字 |
| LgsType | Y | String | 物流型態 | `B2C`=大宗寄倉，`C2C`=店到店，`HOME`=黑貓宅配 |
| GoodsType | Y | Int | 寄件型態 | 1=常溫，2=冷凍，3=冷藏。僅黑貓宅配支援冷藏溫層 |
| ShipType | Y | Int | 通路類別 | 1=7-ELEVEN，2=黑貓宅配 |

**超商取貨不付款（ShipType=1）額外參數**：

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| StoreID | Y | String | 取件門市代碼 | 例如：`916712` |

**黑貓宅配取貨不付款（ShipType=2）額外參數**：

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| ConsigneeTelAreaCode | C | String | 收件人電話區碼 | 至少 2 碼、最多 3 碼數字 |
| ConsigneeTel | C | String | 收件人電話號碼 | 最多 8 碼數字 |
| ConsigneeAddress | Y | String | 收件人地址 | 最長 120 位。格式應為：縣市+鄉鎮市區+段弄巷街+號(+樓) |
| DeliveryTimeTag | Y | String | 希望配達時段 | `01`=13 時前，`02`=14-18 時，`04`=不指定 |

**備註**：有帶 ServiceType 時，當作取貨不付款，並檢查格式。

### 5.5 返回參數（EncryptInfo）

#### 基本返回

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=成功，`UNKNOWN`=等待授權結果逾期，失敗請參考錯誤代碼 |
| Message | String | 狀態說明 | 授權成功=信用卡授權成功。`UNKNOWN`=系統忙碌中，尚未確認交易結果（當 60 秒無收到銀行回應會先回覆 UNKNOWN，後續若有取得交易結果會 Notify 至 NotifyURL） |
| MerID | String | 商店代號 | |
| MerTradeNo | String | 商店訂單編號 | 限制長度 25，格式：`[A-Za-z0-9_-]` |
| TradeNo | String | UNi 序號 | |
| TradeAmt | Int | 訂單金額 | |
| TradeStatus | Int | 訂單狀態 | 1=已付款，2=付款失敗，3=付款取消 |
| PaymentType | Int | 支付工具 | 1=信用卡 |
| Card6No | String | 卡號前六碼 | |
| Card4No | String | 卡號後四碼 | |
| CardInst | String | 分期數 | |
| FirstAmt | String | 首期金額 | |
| EachAmt | String | 每期金額 | |
| ResCode | String | 回應碼 | |
| ResCodeMsg | String | 回應碼敘述 | |
| AuthCode | String | 授權碼 | |
| AuthBank | String | 授權銀行（代碼） | |
| AuthBankName | String | 授權銀行（名稱） | |
| AuthType | Int | 授權類型 | 1=一次，7=銀聯 |
| AuthDay | String | 授權日期 | 格式：`YYYYMMDD` |
| AuthTime | String | 授權時間 | 格式：`HHIISS` |
| CreditHash | String | 信用卡 Token Hash | Token 專用返回參數，有 CreditToken 且授權成功才會壓碼 |
| CreditLife | String | Token 有效日期 | 格式：`MMYY` |
| CardBank | String | 發卡銀行（代碼） | 若為國內發卡行則為銀行代碼（3 碼），若非國內發卡行則為 `-` |

#### 強制 3D（API3D=1）返回

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=建立幕後 3D 成功，失敗請參考錯誤代碼 |
| Message | String | 狀態說明 | 建立幕後 3D 成功，失敗請參考錯誤代碼 |
| URL | String | 強制 3D 導頁網址 | |

#### 優惠券核銷返回

當該交易有使用優惠券核銷時，將額外回傳：

| 參數 | 類型 | 說明 |
|------|------|------|
| PromoCode | String | 優惠碼 |
| DiscountAmt | Int | 折扣金額 |
| OrderAmt | Int | 原訂單金額 |
| CouponFee | Int | 核銷費 |
| CampaignName | String | 活動名稱 |
| CampaignNo | String | 活動序號 |
| CouponName | String | 優惠券名稱 |
| CouponNo | String | 優惠券序號 |

#### 物流取貨不付款返回

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| ShipTradeNo | String | UNi 物流序號 | |
| ServiceType | Int | 取件方式 | 固定為 3=取貨不付款 |
| LgsType | String | 物流型態 | `B2C`=大宗寄倉，`C2C`=店到店，`HOME`=黑貓宅配 |
| GoodsType | Int | 寄件型態 | 1=常溫，2=冷凍，3=冷藏 |
| ShipType | Int | 通路類別 | 1=7-ELEVEN，2=黑貓宅配 |
| ShipAmt | Int | 取貨付款金額 | 固定為 0 |
| Consignee | String | 取件人姓名 | |
| ConsigneeMail | String | 取件人電子信箱 | |
| ConsigneeMobile | String | 取件人手機號碼 | |

**超商取貨不付款（ShipType=1）額外返回**：

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| StoreID | String | 門市代碼 | 例如：`916712` |

**黑貓宅配取貨不付款（ShipType=2）額外返回**：

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| ConsigneeAddress | String | 收件人地址 | |
| DeliveryTimeTag | String | 希望配達時段 | `01`=13 時前，`02`=14-18 時，`04`=不指定 |

### 5.6 GoogleObject 參數說明

取得 Payment token 資訊後，請使用 **Base64Encode** 後，放入 `GoogleObject` 參數。

---

## 6. 代理商專用 API（已移至獨立文件）

代理商專用 API 已自本檔收斂至獨立文件，避免與一般收款商店 API 混雜：

- **撥款指示** `/api/trade/grant`
- **多功能收付** `/api/agent/agent_fee`、**多功能收付交易查詢** `/api/agent/agent_fee_query`、**批次上傳**（SFTP）
- **撥款查詢** `/api/trade/grant_query`
- **新增合作商店** `/api/agent/bind_merchant`、**修改合作商店** `/api/agent/update_merchant`

> 完整規格請見《[PayUni 代理商專用 API 總覽](PayUni%20代理商專用%20API%20總覽.md)》。

---

## 附錄：API Endpoint 速查表

| # | API 名稱 | Endpoint | 方式 | 版本 | 用途 |
|---|---------|----------|------|------|------|
| 1 | 整合式支付頁 UPP | `/api/upp` | Form POST | 2.0 | 前景支付頁面，整合所有支付方式 |
| 2 | 虛擬帳號幕後 ATM | `/api/atm` | HTTP POST | 1.3 | 幕後 ATM 轉帳取號（需綁定 IP） |
| 3 | Apple Pay 幕後 | `/api/apple` | HTTP POST | 1.3 | Apple Pay 幕後授權交易（需申請憑證+綁定 IP） |
| 4 | Google Pay 幕後 | `/api/google` | HTTP POST | 1.3 | Google Pay 幕後授權交易（需綁定 IP） |

> 代理商專用 API（撥款指示 `/api/trade/grant`、多功能收付、撥款查詢、新增/修改合作商店）見《[PayUni 代理商專用 API 總覽](PayUni%20代理商專用%20API%20總覽.md)》。
