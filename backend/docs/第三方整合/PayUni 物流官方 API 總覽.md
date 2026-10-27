# PAYUNi 物流 API 官方文檔（統整版）

> **整合來源**：PAYUNi 開發專區官方 API 文件
> **最後更新**：2026-03-10
> **涵蓋範圍**：7-ELEVEN 超商物流 + 黑貓宅配物流（含建店、建單、退貨、列印、查詢、貨態通知）

---

## 目錄

1. [通用規範](#1-通用規範)
2. [超商物流建店 API（7-ELEVEN）](#2-超商物流建店-api7-eleven)
3. [宅配物流建店 API（黑貓）](#3-宅配物流建店-api黑貓)
4. [建立超商物流單 API](#4-建立超商物流單-api)
5. [建立宅配單 API（黑貓）](#5-建立宅配單-api黑貓)
6. [超商門市地圖 API](#6-超商門市地圖-api)
7. [超商出貨單列印 API](#7-超商出貨單列印-api)
8. [產宅配編號並下載託運單 PDF API](#8-產宅配編號並下載託運單-pdf-api)
9. [呼叫黑貓 API](#9-呼叫黑貓-api)
10. [物流單查詢 API](#10-物流單查詢-api)
11. [物流貨態通知 Webhook](#11-物流貨態通知-webhook)
12. [物流貨態狀態碼對照表](#12-物流貨態狀態碼對照表)
13. [建立宅配退貨單 API（黑貓・背景）](#13-建立宅配退貨單-api黑貓背景)

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
| Version | Y | String | 版本 | 各 API 版本不同，詳見各節 |
| EncryptInfo | Y | String | AES 加密字串 | 請參考「資料加解密」文件 |
| HashInfo | Y | String | SHA256 加密字串 | 請參考「資料加解密」文件 |

### 1.4 統一返回外層參數（系統 → 商店）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態代碼 | `SUCCESS`=成功，失敗請參考錯誤代碼 |
| MerID | String | 商店代號 | |
| Version | String | 版本 | |
| EncryptInfo | String | 加密字串 | 請參考「資料加解密」文件 |
| HashInfo | String | 加密 Hash | 請參考「資料加解密」文件 |

### 1.5 前置作業

1. 於 PAYUNi 平台註冊會員並建立收款商店，取得 **MerID**
2. 登入 PAYUNi 後台開通物流服務功能
3. 向 PAYUNi 提出申請開啟「物流幕後 API 服務」及設定 IP
4. 等候審核通過後即可使用

---

## 2. 超商物流建店 API（7-ELEVEN）

### 2.1 概述

開通商店的 7-ELEVEN 超商物流功能，支援 B2C 大宗寄倉與 C2C 店到店。

### 2.2 物流項目

| 物流項目 | 支援與說明 |
|--------|----------|
| **B2C 大宗寄倉** | 適合出貨量大的賣家，將包裹寄到超商指定物流中心。僅限商業會員。提供冷凍及常溫。 |
| **C2B 退貨便** | 開通 B2C 常溫即同時開通。取得退貨便編號供消費者至門市使用。僅常溫。僅限商業會員。 |
| **C2C 店到店** | 超商門市寄貨服務。商業及個人會員皆可使用。提供冷凍及常溫。 |

- 數網查件網站：`https://tracking.shopmore.com.tw/`

### 2.3 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/logistics/setting` |
| 正式區 URL | `https://api.payuni.com.tw/api/logistics/setting` |
| 請求方式 | HTTP POST |
| 版本 | 固定 `1.0` |

### 2.4 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| Timestamp | Y | Int | 時間戳記 | 格式：`time()` |
| GoodsType | Y | Int | 寄件型態 | 1=常溫, 2=冷凍 |
| LgsType | Y | String | 物流型態 | `B2C`=大宗寄倉, `C2C`=店到店 |
| ShipType | Y | Int | 通路類別 | `1`=7-ELEVEN |
| LabelCompanyName | Y | String | 廠商名稱 | 顯示於標籤紙上（見格式限制） |
| ServiceTel | Y(B2C) | String | 客服電話 | 限 B2C 必填，限制長度 10，顯示於標籤紙 |
| ContactName | Y | String | 聯絡人 | 限制長度 10 |
| ContactTel | Y | String | 聯絡人電話 | 限制長度 10，手機號碼格式 |
| ContactMail | Y | String | 聯絡人信箱 | 限制長度 50，需符合 Email 格式。C2C 退貨時發通知信 |
| MerURL | Y(B2C) | String | 商店網址 | 限 B2C 必填，格式完整網址，顯示於標籤紙 |
| ReturnCycle | Y(B2C) | Int | 退貨週期 | 常溫：2=日退(隔日退), 3=週退。冷凍：4=週退。C2C 不需要 |
| ReturnCycleType | Y(B2C) | Int | 退貨模式 | 常溫：1=自行取件(週三), 2=黑貓(週四), 3=大嘴鳥(週五), 4=新竹貨運(週二), 5=大榮貨運(週二), 6=自行派車。冷凍：1=自行取件, 10=統昶物流中心配合貨運 |
| ReturnCarName | C | String | 自訂派車名稱 | 限 B2C，ReturnCycleType=6 時必填，限 6 個中文字不得有符號，長度 12 |
| ReturnAddress | Y(B2C) | String | 退貨地址 | 限 B2C 必填 |
| NotifyURL | C | String | 貨態通知 URL | 回傳資訊參閱貨態通知，僅限 80 與 443 port |
| StoreID | Y(C2C) | String | 退貨門市代碼 | 限 C2C 必填，例如：`916712` |

#### LabelCompanyName 格式限制

1. 全中文：共 5 個中文字（總共 10 字串）
2. 英數字 + 指定符號（`_ . ' @ $ & #` 及空白）：共 10 個字
3. 混合（中文 + 數字或英文）：僅接受前五碼
4. 若 LgsType=C2C，則僅能填寫 10 個中文字或英文（限制長度 20 字元）

### 2.5 返回參數（EncryptInfo）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態 | `SUCCESS`=成功，失敗請參考錯誤代碼 |
| SetStatus | String | 開通狀態 | 9=未處理(待審查), 1=正常, 2=審查失敗, 3=關閉, 4=永久關閉, 7=重新送審, 8=已上傳(審核中) |
| Message | String | 狀態說明 | |
| MerID | String | 商店代號 | |
| PartnerId | String | 母代碼 | |
| EshipId | String | 子代碼 | |
| GoodsType | Int | 寄件型態 | 1=常溫, 2=冷凍 |
| LgsType | String | 物流型態 | B2C / C2C |
| ShipType | Int | 通路類別 | 1=7-ELEVEN |

### 2.6 重要備註

1. **建立資料後不得修改**
2. 退貨模式 ReturnCycleType=1 自行取件，需自行至物流中心取退貨件
3. 所有退貨件需等收到退貨通知才可派車或自行領取
4. **B2C 常溫**（GoodsType=1, LgsType=B2C）會同時申請開通 C2B 退貨便功能
5. 建檔時商店寄件方式預設為冷凍上收（列為冷凍寄件費用），若需改為自行寄送需通知窗口
6. 開通 B2C 常溫時，若 ReturnCycle=2（日退），則 ReturnCycleType 固定=1
7. B2C 冷凍退貨週期固定為一週二退（週二與週五），退貨模式僅提供自行取件或統昶物流中心配合貨運

### 2.7 開通流程

**B2C 大宗寄倉**（僅商業會員）：
1. 物流設定頁 → 點選申請 → 填寫申請資料
2. 狀態：待審核 → 每週一 9:00 彙整上週資料
3. 審核中 → 建立子廠商代號 → 通過/未通過
4. 冷凍另需「物流中心測標」通過

**C2C 店到店**（商業及個人會員）：
1. 物流設定頁 → 填寫開通資料 → 系統自動開通

---

## 3. 宅配物流建店 API（黑貓）

### 3.1 概述

開通商店的黑貓宅配物流功能，支援常溫與低溫（冷凍/冷藏）的正逆物流。商業及個人會員皆可使用。

### 3.2 物流項目

| 物流項目 | 支援與說明 |
|--------|----------|
| **黑貓宅配 — 常溫與低溫** | 提供到府收貨與配送到府服務。透過 PAYUNi 後台取得出貨單編號列印，並呼叫黑貓司機到府收件。提供常溫及低溫（冷凍/冷藏）寄件。 |
| **黑貓宅配 — 退貨** | 透過 PAYUNi 後台取得退貨編號，黑貓司機於建單時設定的取件日至指定地址收取退貨。 |

- 黑貓包裹查詢：`https://www.t-cat.com.tw/inquire/trace.aspx`（最多 10 筆）

### 3.3 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/home_delivery/setting` |
| 正式區 URL | `https://api.payuni.com.tw/api/home_delivery/setting` |
| 請求方式 | HTTP POST |
| 版本 | 固定 `1.0` |

### 3.4 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| Timestamp | Y | Int | 時間戳記 | 格式：`time()` |
| LgsType | Y | String | 物流型態 | 固定為 `HOME` |
| ShipType | Y | Int | 通路類別 | `2`=黑貓宅配 |
| SenderName | Y | String | 寄件人姓名 | 最長 30 位。中英文數字均可。英文字母算一位，中文字/全形算兩位 |
| SenderTelAreaCode | C | String | 寄件人電話區碼 | 至少 2 碼、最長 3 碼數字 |
| SenderTel | C | String | 寄件人電話 | 最長 8 碼數字 |
| SenderMobile | Y | String | 寄件人手機號碼 | 限填 09 開頭半形數字 |
| SenderAddress | Y | String | 寄件人地址 | 最長 120 位。**務必填寫正確地址，輸入不存在地址會無法啟用** |
| ProductTypeId | Y | String | 商品類別 | 見商品類別對照表 |
| NotifyURL | C | String | 貨態通知 URL | 僅限 80 與 443 port |

#### 商品類別對照表（ProductTypeId）

| 代碼 | 類別 | 代碼 | 類別 |
|------|------|------|------|
| 0001 | 一般食品 | 0009 | 生活用品 |
| 0002 | 名特產/甜產 | 0010 | 美容彩妝 |
| 0003 | 酒/油/醋/醬 | 0011 | 保健食品 |
| 0004 | 穀物蔬果 | 0012 | 醫療相關用品 |
| 0005 | 水產/肉品 | 0013 | 寵物用品飼料 |
| 0006 | 3C | 0014 | 印刷品 |
| 0007 | 家電 | 0015 | 其他 |
| 0008 | 服飾配件 | | |

### 3.5 返回參數（EncryptInfo）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態 | `SUCCESS`=成功 |
| Message | String | 狀態說明 | |
| MerID | String | 商店代號 | |
| PartnerId | String | 母代碼 | 固定=`CAT` |
| EshipId | String | 子代碼 | 固定=`001` |
| GoodsType | Int | 寄件型態 | 固定回 `-` 符號 |
| LgsType | String | 物流型態 | 固定=`HOME` |
| ShipType | Int | 通路類別 | `2`=黑貓 |
| SetStatus | Int | 開通狀態 | `1`=已開通 |

### 3.6 備註

- 啟用後可透過重打本 API 更新：寄件人姓名、電話、手機號碼、商品類別、貨態通知 URL
- 更新**寄件人地址**需聯繫 PAYUNi 客服（`service@example.com`）

### 3.7 開通與出貨流程

**開通流程**：
1. 收款商店同意黑貓宅配開通條款 → 點選申請 → 填寫開通資料
2. 系統自動開通 → PAYUNi 更新狀態為通過 → 寄發啟用通知信

**出貨流程**：
1. 消費者下單選擇黑貓宅配
2. 訂單成立，通知信送出
3. 商家備貨完成 → API 取宅配單號與下載託運單 PDF
4. 顯示託運單 PDF，更新宅配單號至物流明細
5. 出貨日使用「呼叫黑貓」功能 → 統一金流發送上收資訊至宅急便
6. 黑貓司機到府收件 → 配送至收件地址

**退貨流程**（消費者因未取而退貨）：
- 退貨原因：出貨後七日收件人不在家、未將商品送達、買家拒收、託運單內容有誤
- 聯絡原寄件人，商品退回原發貨地址
- 取貨不付款：不需另支付費用但原物流費不退
- 取貨付款：原物流費不退，且退回隔日自會員帳號扣除到付未取費 10 元

---

## 4. 建立超商物流單 API

### 4.1 概述

建立 7-ELEVEN 超商取貨物流單，支援 B2C 大宗寄倉與 C2C 店到店，含取貨付款與取貨不付款。

### 4.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/logistics/trade` |
| 正式區 URL | `https://api.payuni.com.tw/api/logistics/trade` |
| 請求方式 | HTTP POST |
| 版本 | 固定 `1.3` |

### 4.3 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| Timestamp | Y | Int | 時間戳記 | 格式：`time()` |
| MerTradeNo | Y | String | 商店訂單編號 | 長度限制 25，10 分鐘內不可重複 |
| GoodsType | Y | Int | 寄件型態 | 1=常溫, 2=冷凍 |
| LgsType | Y | String | 物流型態 | `B2C`=大宗寄倉, `C2C`=店到店 |
| ShipType | Y | Int | 通路類別 | `1`=7-ELEVEN |
| TradeAmt | Y | Int | 訂單金額 | 取貨付款金額（上限 2 萬），或商品價值 |
| ServiceType | Y | Int | 取件方式 | 1=取貨付款, 3=取貨不付款 |
| StoreID | Y | String | 取件門市代碼 | 限制長度 6 |
| Consignee | Y | String | 取件人姓名 | 限制長度 10 |
| ConsigneeMail | C | String | 取件人電子信箱 | 需符合 Email 格式 |
| ConsigneeMobile | Y | String | 取件人手機號碼 | 09 開頭半形數字 |
| RefundStoreID | C | String | 指定退貨門市 | 僅 C2C |
| SenderName | C | String | 指定退貨收件人 | 僅 C2C，限長 10 |
| SenderMobile | C | String | 指定退貨人手機 | 僅 C2C |
| NotifyURL | C | String | 取貨付款完成通知 | 僅限 80 與 443 port |
| CarrierType | Y/C | String | 發票載具類別 | `3J0002`, `CQ0001`, `amego`, `Donate`, `Company` |
| CarrierInfo | Y/C | String | 載具內容 | 根據類別填寫手機條碼、自然人憑證等 |
| InvBuyerName | Y/C | String | 買方名稱/抬頭 | 有帶發票參數時必填 |
| ProdDesc | Y/C | String | 產品說明 | 長度限制 500 |
| UsrMail | Y/C | String | 消費者電子信箱 | 有帶發票參數時必填 |
| PromoCode | Y(註) | String | 優惠碼 | 使用優惠券功能時必要 |
| DiscountAmt | Y(註) | Int | 折扣金額 | 使用優惠券功能時必要 |
| OrderAmt | Y(註) | Int | 原訂單金額 | 使用優惠券功能時必要 |
| CouponNotifyURL | C | String | 優惠券發券背景通知 | |

### 4.4 返回參數（EncryptInfo）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態 | `SUCCESS`=成功 |
| Message | String | 狀態說明 | |
| MerID | String | 商店代號 | |
| MerTradeNo | String | 自訂編號 | |
| TradeNo | String | UNi 序號 | |
| TradeAmt | Int | 訂單金額 | |
| TradeStatus | Int | 訂單付款狀態 | 固定=0 |
| PaymentType | Int | 支付方式 | 5=取貨付款, 0=取貨不付款 |
| PartnerId | String | 母代碼 | |
| GoodsType | Int | 寄件型態 | |
| LgsType | String | 物流型態 | |
| ShipType | Int | 通路類別 | |
| ShipTradeNo | String | UNi 物流序號 | |
| ShipAmt | Int | 取貨付款金額 | |
| StoreID | String | 取件門市代碼 | |
| StoreName | String | 取件門市名稱 | |
| StoreAddr | String | 取件門市地址 | |
| Consignee | String | 收件人名稱 | |
| ConsigneeMobile | String | 收件人手機號碼 | |
| ConsigneeMail | String | 收件人電子信箱 | |
| RefundStoreID | String | 指定退貨門市 | |
| RefundStoreName | String | 退貨門市名稱 | |
| RefundStoreAddr | String | 退貨門市地址 | |
| SenderName | String | 指定退貨收件人 | |
| SenderMobile | String | 指定退貨手機 | |

### 4.5 備註

1. 若為取貨不付款（ServiceType=3），則 TradeAmt 等同商品價值（遺失時依此金額認賠，由超商認定）
2. 針對 C2C 物流單，若未填寫 RefundStoreID，則自動帶入商店開通 C2C 物流時所填寫的退貨門市代碼
3. 針對 C2C 物流單，SenderName 與 SenderMobile 必須成對填寫或皆不填：
   - 有填 SenderName 則 SenderMobile 必填，反之亦然
   - 皆未填則自動帶入開通 C2C 時所填的退貨收件人與收件人資料

### 4.6 超商取貨付款取件通知（Notify PAYUNi→商店）

此為取貨付款時，消費者完成取件付款後 PAYUNi 發送的背景 Notify 通知。

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態 | `SUCCESS`=成功 |
| Message | String | 狀態說明 | |
| MerID | String | 商店代號 | |
| MerTradeNo | String | 自訂編號 | |
| TradeNo | String | UNi 序號 | |
| TradeAmt | Int | 訂單金額 | |
| TradeStatus | Int | 訂單付款狀態 | 固定=1 |
| PaymentType | Int | 支付方式 | 1=信用卡, 2=ATM, 3=超商代碼, 5=取貨付款, 6=愛金卡, 7=後支付, 8=退貨代收 |
| ShipAmt | Int | 取貨付款金額 | |
| StoreID | String | 取件門市代碼 | |
| StoreName | String | 取件門市名稱 | |
| StoreAddr | String | 取件門市地址 | |
| Consignee | String | 收件人名稱 | |
| ConsigneeMobile | String | 收件人手機號碼 | |
| ConsigneeMail | String | 收件人電子信箱 | |
| RefundStoreID | String | 指定退貨門市 | 僅請求時有填寫才回傳 |
| RefundStoreName | String | 退貨門市名稱 | 僅請求時有填寫才回傳 |
| RefundStoreAddr | String | 退貨門市地址 | 僅請求時有填寫才回傳 |
| SenderName | String | 指定退貨收件人 | 僅請求時有填寫才回傳 |
| SenderMobile | String | 退貨收件人手機 | 僅請求時有填寫才回傳 |
| ShipTradeNo | String | UNi 物流序號 | |
| Odno | String | 出貨單編號 | 8 碼 |
| GoodsType | Int | 寄件型態 | 1=常溫, 2=冷凍 |
| LgsType | String | 物流型態 | `B2C` / `C2C` |
| ShipType | Int | 通路類別 | 1=7-ELEVEN |
| ServiceType | Int | 取件方式 | 1=取貨付款, 3=取貨不付款 |
| ShipAmt | Int | 代收金額 | |
| TradeAmt | Int | 訂單金額 | |
| PayTime | Date | 取件日期 | 格式：`YYYY-MM-DD HH:II:SS` |

**優惠券核銷時額外回傳**：

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

---

## 5. 建立宅配單 API（黑貓）

### 5.1 概述

建立黑貓宅配物流單，支援常溫/冷凍/冷藏之正逆物流。

### 5.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/home_delivery/trade` |
| 正式區 URL | `https://api.payuni.com.tw/api/home_delivery/trade` |
| 請求方式 | HTTP POST |
| 版本 | 固定 `1.2` |

### 5.3 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| Timestamp | Y | Int | 時間戳記 | |
| MerTradeNo | Y | String | 商店訂單編號 | |
| GoodsType | Y | Int | 寄件型態 | 1=常溫, 2=冷凍, 3=冷藏 |
| LgsType | Y | String | 物流型態 | 固定為 `HOME` |
| ShipType | Y | Int | 通路類別 | `2`=黑貓 |
| TradeAmt | Y | Int | 訂單金額 | 30~20,000 元 |
| ServiceType | Y | Int | 取件方式 | 1=取貨付款, 3=取貨不付款 |
| DeliveryTimeTag | Y | String | 希望配達時段 | `01`=13 時前, `02`=14-18 時, `04`=不指定 |
| Consignee | Y | String | 收件人姓名 | 長度限制 30 |
| ConsigneeMobile | Y | String | 收件人手機號碼 | 09 開頭 |
| ConsigneeTelAreaCode | C | String | 電話區碼 | 2~3 碼 |
| ConsigneeTel | C | String | 電話號碼 | 最多 8 碼 |
| ConsigneeAddress | Y | String | 收件人地址 | 最長 120 位 |
| ProdDesc | Y | String | 商品名稱 | 最長 20 位 |
| NotifyURL | C | String | 到付完成通知 URL | |
| CarrierType | Y/C | String | 發票載具類別 | |
| CarrierInfo | Y/C | String | 載具內容 | |
| InvBuyerName | Y/C | String | 買方抬頭 | |
| ProdDesc (發票) | Y/C | String | 產品說明 | |
| UsrMail | Y/C | String | 消費者電子信箱 | |

### 5.4 返回參數（EncryptInfo）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態 | |
| Message | String | 狀態說明 | |
| MerID | String | 商店代號 | |
| MerTradeNo | String | 自訂編號 | |
| TradeNo | String | UNi 序號 | |
| TradeAmt | Int | 訂單金額 | |
| TradeStatus | Int | 訂單付款狀態 | 固定=0 |
| PaymentType | Int | 支付方式 | `10`=宅配到付 |
| Gateway | Int | 交易標記 | `1`=幕後 API |
| TradeType | Int | 宅配類別 | 固定 `1`=正物流 |
| ShipTradeNo | String | UNi 物流序號 | |
| GoodsType | Int | 寄件型態 | |
| LgsType | String | 物流型態 | `HOME` |
| ShipType | Int | 通路類別 | `2`=黑貓 |
| ServiceType | Int | 取件方式 | |
| ShipAmt | Int | 取貨付款金額 | |
| Consignee | String | 收件人姓名 | |
| ConsigneeMobile | String | 手機號碼 | |
| ConsigneeTel | String | 聯絡電話 | 區碼+號碼 |
| ConsigneeAddress | String | 收件人地址 | |
| DeliveryTimeTag | String | 配達時段 | |
| ProductTypeId | String | 商品類別代碼 | 0001~0015 |
| ProdDesc | String | 商品名稱 | |

### 5.5 備註

1. 若為取貨不付款（ServiceType=3），則 TradeAmt 等同商品價值
2. 若要了解宅配貨態變更時會發送的貨態 Notify 內容，請參考第 11 節「物流貨態通知 Webhook」

### 5.6 宅配到付取件完成通知（Notify PAYUNi→商店）

此為宅配到付單消費者已完成取件付款時，PAYUNi 發送的背景 Notify 通知。

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態 | `SUCCESS`=成功 |
| Message | String | 狀態說明 | |
| MerID | String | 商店代號 | |
| MerTradeNo | String | 自訂編號 | |
| TradeNo | String | UNi 序號 | |
| TradeAmt | Int | 訂單金額 | |
| TradeStatus | Int | 訂單付款狀態 | 固定=1 |
| PaymentType | Int | 支付方式 | 1=信用卡, 2=ATM, 3=超商代碼, 5=取貨付款, 6=愛金卡, 7=後支付, 8=退貨代收 |
| ShipTradeNo | String | UNi 物流序號 | |
| Odno | String | 出貨單編號 | 12 碼 |
| GoodsType | Int | 寄件型態 | 1=常溫, 2=冷凍, 3=冷藏 |
| LgsType | String | 物流型態 | 固定=`HOME` |
| ShipType | Int | 通路類別 | 2=黑貓 |
| ServiceType | Int | 取件方式 | 1=取貨付款 |
| ShipAmt | Int | 代收金額 | |
| TradeAmt | Int | 訂單金額 | |
| PayTime | Date | 取件日期 | 格式：`YYYY-MM-DD HH:II:SS` |

**優惠券核銷時額外回傳**：

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

---

## 6. 超商門市地圖 API

### 6.1 概述

消費者選擇 7-ELEVEN 取貨門市時使用。前景方式，支援 B2C/C2C、常溫/冷凍。

### 6.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/logistics/ship_map` |
| 正式區 URL | `https://api.payuni.com.tw/api/logistics/ship_map` |
| 請求方式 | Form POST |
| 版本 | 固定 `1.1` |

### 6.3 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| Timestamp | Y | Int | 時間戳記 | |
| MerKeyNo | Y | String | 自訂編號 | 限制長度 20。Tag=4 或 5 時請帶 UNi 物流序號 |
| GoodsType | Y | Int | 寄件型態 | 1=常溫, 2=冷凍 |
| LgsType | Y | String | 物流型態 | `B2C` / `C2C` |
| ShipType | Y | Int | 通路類別 | `1`=7-ELEVEN |
| MapType | Y | Int | 地圖涵蓋區域 | 1=僅限本島, 2=本島含離島。GoodsType=2 固定為 2 |
| MapReturnURL | C | String | 接收物流門市資訊 | 有值時 Tag=2,3,4,5 會以前景方式導回指定 URL |
| Tag | Y | Int | 標記 | 2=回傳門市, 3=更新商店退貨門市(C2C), 4=更新物流單取件門市, 5=更新指定物流單退貨門市(C2C) |
| MobileTag | C | String | 平台版本 | `Y`=手機版, `N`=PC 版。未帶則顯示 PC 版 |

### 6.4 返回參數（EncryptInfo）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態 | `SUCCESS`=成功 |
| Message | String | 狀態說明 | |
| MerID | String | 商店代號 | |
| MerKeyNo | String | 自訂編號 | |
| GoodsType | Int | 寄件型態 | |
| LgsType | String | 物流型態 | |
| ShipType | Int | 通路類別 | |
| MapJson | String | 物流門市資訊 | JSON 格式 |

#### MapJson 內容

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| StoreType | String | 門市類型 | `SEVEN`，限制長度 5 |
| StoreID | String | 門市代碼 | 限制長度 6，例如 `916712` |
| StoreName | String | 門市名稱 | 限制長度 12，例如「敦安門市」 |
| Address | String | 門市地址 | |
| InsularArea | String | 門市區域識別 | `I`=本島, `O`=離島 |

### 6.5 門市關轉處理

- 門市暫停服務（天災、調整、連假等）時，物流中心發送「門市關轉」貨態
- **B2C 更換期限**：收到通知日 (D)+2 天 23:59 前
- **C2C 更換期限**：收到通知日 (D)+6 天 23:59 前
- C2C 重選門市：Tag=3 為商店退貨門市重選；Tag=5 為指定物流單退貨門市重選；Tag=4 為取件門市重選
- 冷凍超商取貨門市一律 GoodsType=2, MapType=2

---

## 7. 超商出貨單列印 API

### 7.1 概述

列印 7-ELEVEN 超商出貨單並貼至包裹。支援 B2C 大宗寄倉與 C2C 店到店。

### 7.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/logistics/print_label` |
| 正式區 URL | `https://api.payuni.com.tw/api/logistics/print_label` |
| 請求方式 | Form POST |
| 版本 | 固定 `1.0` |

### 7.3 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| Timestamp | Y | Int | 時間戳記 | |
| ShipTradeNo | Y | String | UNi 物流序號 | 最多 50 筆，以半形逗號隔開 |
| GoodsType | Y | Int | 寄件型態 | 1=常溫, 2=冷凍 |
| LgsType | Y | String | 物流型態 | `B2C`=大宗寄倉, `C2C`=店到店 |
| ShipType | Y | Int | 通路類別 | `1`=7-ELEVEN |
| ShipDate | Y | String | 出貨日期 | 格式 `YYYYMMDD`，B2C 出貨不得為當日 |
| LabelMode | C | Int | 列印標籤格式 | 1=A4 版型（預設）, 2=直立式 |

### 7.4 備註

- 資料回傳亦採用 Form POST 方式
- **C2C 模式**：列印出貨單需由 PAYUNi 系統跳轉至超商進行列印，商店不得自行連線取得
- **B2C 模式**：由 PAYUNi 系統直接顯示出貨單供列印

### 7.5 Notify 回傳參數

若建立物流單時有填寫 NotifyURL，則列印成功時會收到通知：

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態 | `SUCCESS`=成功 |
| Message | Int | 狀態說明 | |
| MerID | String | 商店代號 | |
| ShipTradeNo | String | UNi 物流序號 | |
| GoodsType | Int | 寄件型態 | |
| LgsType | String | 物流類型 | |
| ShipType | Int | 通路類別 | |
| PartnerId | String | 母代碼 | 3 碼 |
| Odno | String | 出貨單編號 | 8 碼 |
| ValidationNo | String | 驗證碼 | 4 碼，店到店專用 |
| ApiType | String | API 類別 | 固定回傳 `Print` |

### 7.6 配編編號查詢

- **B2C**：`PartnerId`（3 碼）+ `Odno`（8 碼）= 11 碼
- **C2C**：`Odno`（8 碼）+ `ValidationNo`（4 碼）= 12 碼

---

## 8. 產宅配編號並下載託運單 PDF API

### 8.1 概述

黑貓宅配出貨時，取得宅配編號並產出託運單 PDF 供下載。

### 8.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/home_delivery/get_obt_number_pdf` |
| 正式區 URL | `https://api.payuni.com.tw/api/home_delivery/get_obt_number_pdf` |
| 請求方式 | Form POST |
| 版本 | 固定 `1.0` |

### 8.3 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| Timestamp | Y | Int | 時間戳記 | |
| PostType | C | Int | 傳遞方式 | `1`=前景 |
| PrintType | C | Int | 列印模式 | 固定=1 |
| ShipTradeNo | Y | String | UNi 物流序號 | 多筆時以半形逗號隔開 |
| GoodsType | Y | Int | 寄件型態 | 1=常溫, 2=冷凍, 3=冷藏 |
| LgsType | Y | String | 物流型態 | 固定為 `HOME` |
| ShipType | Y | Int | 通路類別 | `2`=黑貓 |
| ShipDate | Y | Date | 出貨日期 | `YYYYMMDD`，需大於今日 |
| DeliveryDate | Y | Date | 配達日期 | `YYYYMMDD`，需大於出貨日 |
| Spec | Y | Int | 規格代碼 | 1=60, 2=90, 3=120, 4=150（限常溫） |
| HideProdDesc | C | String | 隱藏商品名稱 | 預設 `N`=無須隱藏, `Y`=要隱藏 |
| Memo | C | String | 給黑貓的話 | 最長 100 位（英文一位，中文/全形兩位） |

### 8.4 備註

- 會產出託運單 PDF 檔供下載
- ShipDate 與 DeliveryDate 不得為**週日**及**國定假日**
- DeliveryDate 必須**大於** ShipDate
- 冷凍/冷藏不支援規格代碼 4（150cm）

---

## 9. 呼叫黑貓 API

### 9.1 概述

已建立宅配單並列印好託運單後，透過此 API 預約黑貓司機到府取件。

- 若於 **15:00 前**完成預約，黑貓司機將於**兩日內**（通常 15:00-18:00）至指定地址收件
- 若已為黑貓契約客戶並有約定收件時間，則無需使用本服務

### 9.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/home_delivery/call_cat` |
| 正式區 URL | `https://api.payuni.com.tw/api/home_delivery/call_cat` |
| 請求方式 | HTTP POST |
| 版本 | 固定 `1.0` |

### 9.3 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| Timestamp | Y | Int | 時間戳記 | 格式：`time()` |
| ContactName | Y | String | 聯絡人姓名 | 限制長度 30。中英文數字均可。英文字母算一位，中文字/全形算兩位 |
| ContactMobile | C | String | 聯絡人手機號碼 | 限填 09 開頭。市話跟手機號碼需二擇一填寫 |
| ContactTelAreaCode | C | String | 聯絡人電話區碼 | 至少 2 碼、最多 4 碼。市話跟手機號碼需二擇一填寫 |
| ContactTel | C | String | 聯絡人電話號碼 | 最多 8 碼。市話跟手機號碼需二擇一填寫 |
| ContactTelExt | C | String | 聯絡人電話分機 | 最多 8 碼。市話跟手機號碼需二擇一填寫 |
| ContactAddress | Y | String | 聯絡人地址 | 最長 120 位。英文字母算一位，中文字/全形算兩位 |
| NormalQuantity | Y | Int | 常溫包裹件數 | 填入 0 或正整數 |
| ColdQuantity | Y | Int | 冷藏包裹件數 | 填入 0 或正整數 |
| FreezeQuantity | Y | Int | 冷凍包裹件數 | 填入 0 或正整數 |
| IsContact | Y | String | 是否需要事先電聯 | `Y`=要, `N`=否 |
| IsTrolley | Y | String | 是否需要推車 | `Y`=要, `N`=否 |
| Memo | C | String | 給宅配人員的備註 | 最長 100 位 |

### 9.4 重要限制

1. 常溫 + 冷藏 + 冷凍包裹件數**總和需大於 0**
2. 因不同溫層需要不同運輸車與溫控，**每次呼叫僅可指定單一溫層 > 0**（混合多溫層將回傳失敗）

### 9.5 返回參數（EncryptInfo）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態 | `SUCCESS`=成功 |
| Message | String | 狀態說明 | |
| MerID | String | 商店代號 | |
| ErrorMsg | String | 呼叫失敗原因 | 成功時回 `-`，失敗回錯誤訊息 |
| CallTime | Date | 呼叫時間 | 格式 `YYYY-MM-DD HH:II:SS` |

---

## 10. 物流單查詢 API

### 10.1 概述

查詢 7-ELEVEN 超商及黑貓宅配之物流單狀態。

### 10.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/logistics/query` |
| 正式區 URL | `https://api.payuni.com.tw/api/logistics/query` |
| 請求方式 | HTTP POST |
| 版本 | 固定 `1.1` |

### 10.3 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| Timestamp | Y | Int | 時間戳記 | |
| LgsType | Y | String | 物流型態 | `B2C`, `C2C`, `HOME`, `C2B` |
| ShipTradeNo | C | String | UNi 物流序號 | B2C/C2C/HOME 時必填 |
| TradeType | C | Int | 宅配類別 | 僅宅配查詢：1=正物流, 2=逆物流 |
| ReturnOdno | C | String | 退貨便編號 | C2B 時必填，提供 12 碼 |

### 10.4 返回參數（EncryptInfo）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態 | |
| Message | String | 狀態說明 | |
| MerID | String | 商店代號 | |
| PartnerId | String | 母代碼 | 宅配固定為 `CAT` |
| MerTradeNo | String | 自訂編號 | |
| TradeNo | String | UNi 序號 | |
| ShipTradeNo | String | UNi 物流序號 | |
| Odno | String | 出貨單編號 | 超商 8 碼，宅配 12 碼 |
| GoodsType | Int | 寄件型態 | |
| LgsType | String | 物流型態 | |
| ShipType | Int | 通路類別 | 1=7-ELEVEN, 2=黑貓 |
| ServiceType | Int | 取件方式 | |
| ShipAmt | Int | 取貨付款金額 | |
| Consignee | String | 收件人 | 隱碼顯示 |
| ConsigneeMobile | String | 手機號碼 | 隱碼顯示 |
| ShipStatus | Int | 物流貨態代碼 | 詳閱狀態碼對照表 |
| PickupStoreType | Int | 取貨門市類型 | 81-門市關轉時：1=取件, 2=退件 |
| ShipStatusDesc | String | 貨態說明 | |
| ShipStatusTime | DateTime | 貨態日期 | `YYYY-MM-DD HH:II:SS` |
| StoreID | String | 門市代碼 | 僅超商物流回傳 |
| StoreName | String | 門市名稱 | 僅超商物流回傳 |
| ValidationNo | String | 驗證碼 | 僅 C2C 回傳，4 碼 |
| FileNo | String | 檔名序號 | 僅 HOME 回傳，重新下載 PDF 用 |
| ConsigneeAddress | String | 收件人地址 | 僅 HOME 回傳 |

---

## 11. 物流貨態通知 Webhook

### 11.1 概述

PAYUNi 系統主動推送物流貨態變更通知至商店設定的 NotifyURL。

### 11.2 B2C 大宗寄倉 / C2C 店到店 回傳參數（EncryptInfo）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態 | `SUCCESS`=成功 |
| Message | String | 狀態說明 | `貨態狀態處理成功({ShipStatus})` |
| MerID | String | 商店代號 | |
| PartnerId | String | 母代碼 | 3 碼 |
| ShipTradeNo | String | UNi 物流序號 | |
| LgsType | String | 物流型態 | `B2C` / `C2C` |
| GoodsType | Int | 寄件型態 | 1=常溫, 2=冷凍 |
| ShipType | Int | 通路類別 | 1=SEVEN |
| ShipStatus | Int | 物流貨態狀態碼 | 詳閱狀態碼對照表 |
| PickupStoreType | Int | 取貨門市類型 | 狀態碼 81（門市關轉）時：1=取件門市, 2=退件門市 |
| ShipStatusDesc | String | 貨態說明 | |
| ShipStatusTime | DateTime | 貨態日期 | `YYYY-MM-DD HH:II:SS` |
| ApiType | String | API 類別 | 固定回傳 `ShipStatus` |

### 11.3 C2B 退貨便 回傳參數（EncryptInfo）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態 | `SUCCESS`=成功 |
| Message | String | 狀態說明 | |
| MerID | String | 商店代號 | |
| PartnerId | String | 母代碼 | 3 碼 |
| RefundODNO | String | 退貨便編號 | 8 碼 |
| ValidationNo | String | 退貨便驗證碼 | 4 碼 |
| LgsType | String | 物流型態 | `C2B`=退貨便 |
| GoodsType | Int | 寄件型態 | 1=常溫 |
| ShipType | Int | 通路類別 | 1=SEVEN |
| ShipStatus | Int | 物流貨態狀態碼 | |
| ShipStatusDesc | String | 貨態說明 | |
| ShipStatusTime | DateTime | 貨態日期 | `YYYY-MM-DD HH:II:SS` |
| ApiType | String | API 類別 | 固定回傳 `ShipStatus` |

### 11.4 代理商專用貨態通知

代理商建立合作商店後，可透過此機制接收物流貨態資訊。於 PAYUNi 平台 > 代理商專區 > 合約及基本資料取得串接金鑰並輸入貨態 Notify 網址。

---

## 12. 物流貨態狀態碼對照表

| 代碼 | 中文名稱 | 說明 |
|------|----------|------|
| **91** | 未處理 | 尚未取得出貨單編號 |
| **92** | 處理中 | 出貨單編號傳送至上游廠商確認中（僅限超商） |
| **98** | 處理中(已接收) | 已配出貨單編號，待傳送至物流中心（僅限超商 B2C） |
| **21** | 待出貨 | 等待商店出貨 |
| **22** | 物流驗收 | 物流中心驗收中（僅限超商） |
| **31** | 配送中 | 安排出貨中；可能發生門市關轉需重選 |
| **32** | 待取貨 | 包裹配達門市，今日 23:59 後將退回物流中心 |
| **33** | 異常訂單 | 配送過程異常 |
| **11** | 已取貨 | 消費者已取貨 |
| **41** | 已取消 | 訂單已被商店取消 |
| **43** | 賠償訂單 | 物流中心/黑貓確認賠償訂單產生 |
| **44** | 包裹遺失 | 門市/物流中心遺失包裹，協尋 18 天未果 |
| **46** | 包裹拋棄 | 賣家未取包裹且逾期未提供宅配資料（僅限超商 C2C） |
| **51** | 一般退貨 | 退貨便/黑貓退貨 |
| **52** | 買家未取 | 買家超過取貨期限未取，產生退貨 |
| **53** | 廠退 | 包裹退回物流中心/黑貓集貨所 |
| **55** | 賣家未取 | 買家未取包裹配達退貨門市後，商店未於期限內領取（僅限超商 C2C） |
| **56** | 已轉宅配退回 | 賣家未取包裹已交付黑貓宅配退回指定地址（僅限超商 C2C） |
| **81** | 門市關轉 | 門市暫歇（裝修、改店號等），需在期限內重選門市 |
| **82** | 待轉宅配退回 | 包裹退回物流中心保管，將寄信提醒商店（僅限超商 C2C） |

### 12.1 物流狀態碼（ShippingStatus）

Shipping Service 內部物流狀態直接對應 PayUni 狀態碼，value 即為 PayUni 代碼。

| PayUni 代碼 | ShippingStatus code | 中文名稱 | 說明 |
|------------|-------------------|---------|------|
| 91 | UNPROCESSED | 未處理 | 尚未取得出貨單編號 |
| 92 | PROCESSING | 處理中 | 出貨單編號傳送至上游廠商確認中（僅限超商） |
| 98 | PROCESSING_ACCEPTED | 處理中(已接收) | 已配出貨單編號，待傳送至物流中心（僅限超商 B2C） |
| 21 | PENDING_SHIPMENT | 待出貨 | 等待商店出貨 |
| 22 | LOGISTICS_CHECK | 物流驗收 | 物流中心驗收中（僅限超商） |
| 31 | IN_TRANSIT | 配送中 | 安排出貨中；可能發生門市關轉需重選 |
| 32 | WAITING_PICKUP | 待取貨 | 包裹配達門市，今日 23:59 後將退回物流中心 |
| 33 | ABNORMAL | 異常訂單 | 配送過程異常 |
| 11 | PICKED_UP | 已取貨 | 消費者已取貨 |
| 41 | CANCELLED | 已取消 | 訂單已被商店取消 |
| 43 | COMPENSATION | 賠償訂單 | 物流中心/黑貓確認賠償訂單產生 |
| 44 | PACKAGE_LOST | 包裹遺失 | 門市/物流中心遺失包裹，協尋 18 天未果 |
| 46 | PACKAGE_ABANDONED | 包裹拋棄 | 賣家未取包裹且逾期未提供宅配資料（僅限超商 C2C） |
| 51 | RETURN_GENERAL | 一般退貨 | 退貨便/黑貓退貨 |
| 52 | BUYER_NOT_PICKED | 買家未取 | 買家超過取貨期限未取，產生退貨 |
| 53 | RETURN_TO_WAREHOUSE | 廠退 | 包裹退回物流中心/黑貓集貨所 |
| 55 | SELLER_NOT_PICKED | 賣家未取 | 買家未取包裹配達退貨門市後，商店未於期限內領取（僅限超商 C2C） |
| 56 | RETURN_BY_HOME_DELIVERY | 已轉宅配退回 | 賣家未取包裹已交付黑貓宅配退回指定地址（僅限超商 C2C） |
| 81 | STORE_TRANSFER | 門市關轉 | 門市暫歇（裝修、改店號等），需在期限內重選門市 |
| 82 | PENDING_HOME_RETURN | 待轉宅配退回 | 包裹退回物流中心保管，將寄信提醒商店（僅限超商 C2C） |

> 收到上表以外的 PayUni 狀態碼時，**拋出異常**，不做靜默處理。

---

## 13. 建立宅配退貨單 API（黑貓・背景）

### 13.1 概述

建立黑貓退貨單使用，將商店替消費者申請的退貨資訊提供給黑貓物流，黑貓司機將於退貨申請時設定的取件日，攜帶退貨單至退貨申請時指定的退貨地址收取退件。

- 開通黑貓宅配物流時即開通黑貓退貨，兩者均可支援常溫、冷凍與冷藏三種寄件型態
- 透過黑貓物流配送之退貨包裹可於黑貓宅配包裹查詢頁面查看退貨編號的貨態：`https://www.t-cat.com.tw/inquire/trace.aspx`

### 13.2 請求資訊

| 項目 | 值 |
|------|------|
| 測試區 URL | `https://sandbox-api.payuni.com.tw/api/home_delivery/refund` |
| 正式區 URL | `https://api.payuni.com.tw/api/home_delivery/refund` |
| 請求方式 | HTTP POST |
| 版本 | 固定 `1.0` |

### 13.3 請求參數（EncryptInfo）

| 參數 | 必要 | 類型 | 說明 | 備註 |
|------|------|------|------|------|
| MerID | Y | String | 商店代號 | |
| Timestamp | Y | Int | 時間戳記 | 格式：`time()` |
| ShipTradeNo | C | String | UNi 物流序號 | 若要針對正物流宅配單（原宅配單）進行退貨，須帶此參數 |
| MerTradeNo | C | String | 商店訂單編號 | 若不是原宅配單的退貨，此處需填寫新的自訂編號。限制長度 25，格式 `[A-Za-z0-9_-]`，10 分鐘內不可重複 |
| GoodsType | Y | Int | 寄件類型 | 1=常溫, 2=冷凍, 3=冷藏 |
| LgsType | Y | String | 物流類型 | 固定為 `HOME` |
| ShipType | Y | Int | 通路類型 | `2`=黑貓 |
| ServiceType | Y | Int | 退貨方式 | `3`=取貨不付款 |
| DeliveryTimeTag | Y | String | 希望配達時間 | `01`=13 時前, `02`=14-18 時, `04`=不指定 |
| Consignee | Y | String | 收件人姓名 | |
| ConsigneeMobile | Y | String | 收件人手機號碼 | 限填手機號碼 09 開頭，半形數字 |
| ConsigneeTelAreaCode | C | String | 收件人電話區碼 | 至少 2 碼、最多 3 碼數字 |
| ConsigneeTel | C | String | 收件人電話號碼 | 最多 8 碼數字 |
| ConsigneeAddress | Y | String | 收件人地址 | 最長 120 位。格式應為：縣市+鄉鎮市區+段弄巷街+號(+樓) |
| Consignor | Y | String | 退貨人姓名 | |
| ConsignorTelAreaCode | C | String | 退貨人電話區碼 | 至少 2 碼、最多 3 碼數字 |
| ConsignorTel | C | String | 退貨人電話號碼 | 最多 8 碼數字 |
| ConsignorMobile | Y | String | 退貨人手機號碼 | 限填手機號碼 09 開頭，半形數字 |
| ConsignorAddress | Y | String | 退貨人地址 | 最長 120 位。格式應為：縣市+鄉鎮市區+段弄巷街+號(+樓)。請填寫黑貓司機收取退件之地址 |
| ProdDesc | Y | String | 商品名稱 | 最長 20 位 |
| Spec | Y | Int | 規格代碼 | 1=60cm, 2=90cm, 3=120cm, 4=150cm（冷凍/冷藏不支援此規格） |
| ShipDate | Y | Date | 取件日期 | 格式：`YYYYMMDD`。須至少為 D+1，至多為 D+7 天。每日 16:25 後申請的黑貓退貨單，取件日請至少設定為 D+2 |
| Memo | C | String | 備註給黑貓 | 最長 100 位（英文一位，中文/全形兩位） |

#### 備註

- 以下參數會自動代入商店開通黑貓宅配物流時所填寫的聯絡資訊：
  1. 收件人姓名
  2. 收件人手機號碼
  3. 收件人電話區碼
  4. 收件人電話號碼
  5. 收件人地址

### 13.4 返回參數（EncryptInfo）

| 參數 | 類型 | 說明 | 備註 |
|------|------|------|------|
| Status | String | 狀態 | `SUCCESS`=成功，失敗請參考錯誤代碼 |
| Message | Int | 狀態說明 | 顯示原因 |
| MerID | String | 商店代號 | |
| TradeType | Int | 宅配類別 | `2`=逆物流(退貨) |
| MerTradeNo | String | 商店訂單編號 | |
| ShipTradeNo | String | UNi 物流序號 | |
| OBTNumber | String | 退貨編號 | |
| GoodsType | Int | 寄件型態 | 1=常溫, 2=冷凍, 3=冷藏 |
| LgsType | String | 物流型態 | 固定為 `HOME` |
| ShipType | Int | 通路類別 | `2`=黑貓 |
| ServiceType | Int | 取件方式 | `3`=取貨不付款 |
| Consignee | String | 收件人姓名 | |
| ConsigneeMobile | String | 收件人手機號碼 | |
| ConsigneeTel | String | 收件人聯絡電話 | 區碼+號碼。若有帶時回 `00-00000000`，若交易當下沒帶時回 `-` |
| ConsigneeAddress | String | 收件人地址 | |
| Consignor | String | 退貨人姓名 | |
| ConsignorMobile | String | 退貨人手機號碼 | |
| ConsignorTel | String | 退貨人電話 | |
| ConsignorAddress | String | 退貨人地址 | |
| Spec | Int | 規格代碼 | 1=60cm, 2=90cm, 3=120cm, 4=150cm |
| ShipDate | Date | 取件日期 | 格式：`YYYYMMDD` |
| DeliveryTimeTag | String | 希望配達時間 | `01`=13 時前, `02`=14-18 時, `04`=不指定 |
| ProdDesc | String | 商品名稱 | |
| Gateway | Int | 交易標記 | `1`=幕後 API |

### 13.5 退貨流程

1. 包裹簽收後 30 天內，付款人聯繫商店確認退貨
2. 商店填寫退貨資訊，申請黑貓退貨
3. 統一金流將退貨資訊送至宅急便
4. 宅急便提供退貨編號
5. 商店告知買家退貨編號與取件日期，請買家準備好退貨包裹
6. 於退貨申請時填寫的取件日，黑貓司機主動聯絡買家上門收取退件（攜帶退貨標籤）
7. 包裹收件完成，SOD 檔提供貨態與實際包裹尺寸
8. PAYUNi 後台顯示包裹貨態
9. 退件配至商店設定的寄件人地址

---

## 附錄：API Endpoint 速查表

| # | API 名稱 | Endpoint | 方式 | 版本 | 用途 |
|---|---------|----------|------|------|------|
| 1 | 超商物流建店 | `/api/logistics/setting` | POST | 1.0 | 開通 7-ELEVEN 超商物流 |
| 2 | 宅配物流建店 | `/api/home_delivery/setting` | POST | 1.0 | 開通黑貓宅配物流 |
| 3 | 建立超商物流單 | `/api/logistics/trade` | POST | 1.3 | 建立 B2C/C2C 物流單 |
| 4 | 建立宅配單 | `/api/home_delivery/trade` | POST | 1.2 | 建立黑貓宅配物流單 |
| 5 | 超商門市地圖 | `/api/logistics/ship_map` | Form POST | 1.1 | 消費者選擇取貨門市 |
| 6 | 超商出貨單列印 | `/api/logistics/print_label` | Form POST | 1.0 | 列印超商出貨標籤 |
| 7 | 宅配取號下載 PDF | `/api/home_delivery/get_obt_number_pdf` | Form POST | 1.0 | 取得宅配編號與託運單 PDF |
| 8 | 呼叫黑貓 | `/api/home_delivery/call_cat` | POST | 1.0 | 預約黑貓司機到府取件 |
| 9 | 物流單查詢 | `/api/logistics/query` | POST | 1.1 | 查詢物流單狀態 |
| 10 | 建立宅配退貨單 | `/api/home_delivery/refund` | POST | 1.0 | 建立黑貓退貨單（背景） |
