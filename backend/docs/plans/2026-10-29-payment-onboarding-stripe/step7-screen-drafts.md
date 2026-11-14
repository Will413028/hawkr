# 步驟 7 畫面草案

計畫：[`../2026-10-29-payment-onboarding-stripe.md`](../2026-10-29-payment-onboarding-stripe.md) 步驟 7。狀態：Will 2026-11-03 同意，第 5 節六題全部採建議（計畫 D24）。盤點的快照 commit 是 `3f35e2f9`。

依據：D9、D22（平台只收市集資料）、D15（分成平台審核與收款帳戶兩區）、D13（v2 狀態，新帳戶顯示 `ACTION_REQUIRED`，文案不寫「逾期」或「停用」）、D16（平台審核核准後才建立外部帳戶）、D23（收款聯絡信箱）、D2／E1（Stripe 收件用託管頁面，平台收件用內嵌元件）。

## 0. 現況與開工前要先處理的落差

- 三個 client 都還沒有呼叫 `GetMyPaymentAccount`、`CreatePaymentOnboardingSession`，也沒有呼叫 admin 的 `GetPaymentAccount`、`RefreshPaymentAccount`、`RetryPaymentAccountCreate`。查法：在 `website/`、`seller-app/`、`admin/src` 用 `rg` 搜這些 RPC 名，結果是 0；seller-app 唯一的命中是 `data/kyc_fields.dart:108` 的欄位對照。
- **計畫有一個順序落差。** 後端送出時的必填清單（`backend/internal/modules/seller/internal/kyc/kyc.go` 的 `requirements`）仍然要求姓名、英文名稱、銀行、身分證正反面、存摺與身分證字號。同意書也仍然要求 `PAYUNI_TERMS`（`model/agreement.go:40` 的 `SubmitKYCAgreements`）。
  - 步驟 7 把表單刪減之後，`SubmitKYC` 會一律回 invalid_argument，整個流程走不通。
  - 計畫原本把這些刪除排在步驟 8，順序反了。建議做法見問題 Q1。
- 賣家看的收款帳戶狀態文案，目前不在共用文案目錄裡（`uitext/catalog` 搜 payment account 沒有結果）。依根目錄 `AGENTS.md` 的共用文案規則，同一個 enum 的對照表要放在目錄裡，不能在各 client 各寫一份。
- seller-app 沒有 deep link 機制（沒有 app_links），只有 app 回到前景時的刷新（`lib/app.dart:88`、`data/seller_status_refresh.dart:19`）。

## 1. 平台審核表單（刪減後）

刪掉的內容（D22）：身分證明、銀行帳戶、身分證正反面、存摺、英文名稱、`PAYUNI_TERMS` 同意勾選。英文名稱原本是 PayUni 需要的欄位，Stripe 的託管頁面會自己收商店名稱。

留下的內容：

- 類型選擇：個人或公司。公司要填統編，所以這一步還需要。
- 公司資訊：統編（含自動帶入）、公司名稱、登記日、公司地址、公司電話、販售類型。只有公司類型有這一區。
- 商家營運資訊：聯絡人、聯絡手機、客服信箱、店家地址（這四項是店家欄位，寫入 `UpdateMyStore`）。
- 新增：**收款聯絡信箱**（D23）。預設帶入登入信箱，可以修改，送出時必填。
- 補充文件（選填，只有公司類型）：要不要保留見 Q2。
- 確認送出：資料摘要、`KYC_PRIVACY_NOTICE` 同意勾選、缺漏清單。

### website（`CertificationsClient.tsx` 的表單外框，沿用左側章節錨點與底部操作列）

```
┌ 商家認證 ───────────────────────────────────────────────┐
│ ┌章節────────┐  ┌ 1 公司資訊（公司才有）──────────────┐ │
│ │ 1 公司資訊 │  │ 統一編號 [________] [自動帶入]       │ │
│ │ 2 營運資訊 │  │ 公司名稱 / 登記日 / 地址 / 電話      │ │
│ │ 3 補充文件 │  │ 販售類型 [▼]                         │ │
│ │ 4 確認送出 │  └─────────────────────────────────────┘ │
│ │ 缺漏：2 項 │  ┌ 2 商家營運資訊 ─────────────────────┐ │
│ └────────────┘  │ 聯絡人 / 聯絡手機 / 客服信箱 / 店家地址 │ │
│                 │ 收款聯絡信箱 [login@example.com   ]   │ │
│                 │  ⓘ 金流商會用這個信箱聯絡你收款事宜  │ │
│                 └─────────────────────────────────────┘ │
│                 ┌ 4 確認送出 ─────────────────────────┐ │
│                 │ ☐ 我已閱讀〈個人資料告知〉            │ │
│                 │ ⓘ 身分與銀行帳號會在審核通過後，由     │ │
│                 │   金流商收集，平台不經手               │ │
│                 └─────────────────────────────────────┘ │
│                          [儲存草稿]  [送出審核]         │
└──────────────────────────────────────────────────────────┘
```

### seller-app（沿用 `KycStepCard` 的精靈步驟）

- 個人：營運資訊 → 確認送出（2 步）。
- 公司：公司資訊 → 營運資訊 →（補充文件）→ 確認送出。
- 收款聯絡信箱放在「商家營運資訊」那一步的最後一個欄位，用 `KycTextField`，預設值是登入信箱。確認頁的提示文字與 website 相同。

## 2. 兩區的狀態頁（D15）

取代現在的鎖定狀態畫面：website 的 `CertificationsClient.tsx:1035-1094`，seller-app 的 `_LockedStatusView`。

- 第一區「平台審核」：送出後就出現。
- 第二區「收款帳戶」：平台審核核准（`APPROVED`）後才有內容，核准之前顯示灰色的「平台審核通過後開放設定」。
- 樣式：website 用 `.seller-card` 加 `.seller-cert__status is-{tone}`；seller-app 用 `HkFrostedCardSurface` 加 `HkStatusBadge`。

```
┌ 商家認證 ────────────────────────────────────────┐
│ ┌ 平台審核 ──────────────────────── [已通過] ┐ │
│ │ 2026-11-03 核准 · 公司 · 統編 12345678       │ │
│ │ [檢視送出的資料]                              │ │
│ └──────────────────────────────────────────────┘ │
│ ┌ 收款帳戶 ──────────────────────── [待設定] ┐ │
│ │ 提供身分與銀行資料後即可開始收款。            │ │
│ │ 資料由金流商收集，平台不經手。                │ │
│ │                          [前往設定收款帳戶]   │ │
│ │ 上次更新 11/03 14:20  [重新整理]              │ │
│ └──────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────┘
```

### 平台審核區的狀態

讀 `KYCApplication.status`（`KYCStatus`），文字是 7a 新加進目錄的鍵；沒有申請或已取消都顯示為「未送出」。

| 狀態 | 徽章 | 內容 | 動作 |
|---|---|---|---|
| NOT_STARTED、DRAFT | 未送出／草稿 | 不出現狀態頁，直接進表單 | 開始填寫／繼續填寫 |
| SUBMITTED | 審核中 | 送出時間 | 取消申請 |
| REJECTED | 需修改 | 退回的總理由與逐項問題（沿用現有的退回提示） | 修改後重新送出 |
| APPROVED | 已通過 | 核准時間 | 檢視送出的資料 |

### 收款帳戶區的狀態

文案放進共用文案目錄，enum 是 `PaymentAccountStatus`。

| 狀態 | 徽章（tone） | 文案（zh-TW 草案） | 動作 |
|---|---|---|---|
| 平台審核未核准 | （灰） | 平台審核通過後開放設定 | 無 |
| NOT_CREATED | 準備中（info） | 正在建立收款帳戶，通常幾分鐘內完成。 | 重新整理 |
| ACTION_REQUIRED | 待設定（warning） | 提供身分與銀行資料後即可開始收款。資料由金流商收集，平台不經手。 | 前往設定收款帳戶 |
| UNDER_REVIEW | 審核中（info） | 金流商正在審核你的資料，不需要額外操作。 | 重新整理 |
| ENABLED | 已啟用（success） | 收款已啟用。若 `payouts_active=false`，再加一行：撥款到銀行帳戶尚未啟用，金流商可能會通知你補件。 | 無（見 Q5） |
| DISABLED | 無法啟用（danger） | 金流商目前無法啟用你的收款帳戶，請聯絡客服。 | 聯絡客服 |

- `ACTION_REQUIRED` 同時代表「新帳戶還沒開始設定」與「要補件」（D13）。文案寫成中性的「待設定」，不寫「逾期」「停用」「補件逾期」。
- 不顯示 `requirements` 的細項給賣家：內容由金流商定義，Stripe 的頁面自己會列出要補什麼，client 只看狀態。

### 開通流程

- **website，託管頁面（HOSTED）**：
  - 呼叫 `CreatePaymentOnboardingSession`，`return_url` 是 `/seller-center/me/certifications?payment=return`，`refresh_url` 是 `…?payment=refresh`。
  - 拿到 url 後整頁導過去。
  - 回到 `return` 時重讀 `GetMyPaymentAccount`；狀態可能還沒同步，所以顯示「正在確認資料…」，輪詢幾次。
  - 回到 `refresh` 時自動重新取得連結再導過去。
- **website，內嵌元件（EMBEDDED）**：在收款帳戶區的位置展開 Stripe 的 `ConnectAccountOnboarding` 元件，用 `client_secret` 初始化。`onExit` 時重讀狀態。需要新增依賴與公開金鑰，見 Q4。
- **seller-app**：只支援 HOSTED，用 `launchUrl(externalApplication)` 開外部瀏覽器（託管頁面不支援 WebView）。
  - `return_url` 與 `refresh_url` 指向 website 的一個靜態頁，內容是「設定完成／連結已過期，請回到 hawkr 賣家 App」。
  - App 回到前景時沿用 `seller_status_refresh` 重讀狀態。
  - 不新增 deep link：它要的條件（從瀏覽器自動切回 App）用「回到前景就刷新」已經滿足。見 Q3。
  - 收到 EMBEDDED 時顯示「請到網頁版完成收款設定」。

### 開通任務卡與閘門提示

- `OnboardingProgressCard`（兩端都有）的「商家認證」任務拆成「平台審核」與「收款帳戶」兩項，都在「公開前必須」群組。
- 閘門缺少 `ONBOARDING_COMPLETE` 時的提示「商家認證已通過」（目錄 `zh-TW.json` 約 795 行的 `SELLER_REQUIREMENT_ONBOARDING_COMPLETE`；372 行是另一個鍵，不動）改成「平台審核與收款帳戶都完成」，這是步驟 4 留下來的工作。

## 3. admin 收款帳戶卡

放在商家詳情 `admin/src/app/admin/merchants/[id]/page.tsx` 基本資料分頁，位置是現在 `PayUniStatusCard` 的地方（L1192），錨點 `#payment-account`。審核待辦的連結（`queue-item.ts:68`）改帶這個錨點。PayUni 的兩張卡在步驟 8 刪除。

```
┌ 收款帳戶 ─────────────────────────── [待設定] ┐
│ 金流商        stripe                            │
│ 外部帳戶      acct_1UXV…4CJw  [複製]            │
│ 收件方式      金流商收件（託管頁面）            │
│ 轉帳能力      未啟用      撥款能力   未啟用     │
│ 狀態原因      requirements_past_due             │
│ 上次讀取      2026-11-03 14:20                  │
│ ▸ 待補條件（原始內容）                          │
│   { "entries": [ … ] }        ← 折疊的 <pre>    │
│                    [重新整理]  [重試建立]       │
└─────────────────────────────────────────────────┘
```

- 資料用 `GetPaymentAccount`。回 not_found 時，卡片顯示「尚未建立（平台審核核准後自動建立）」。
- 「重新整理」呼叫 `RefreshPaymentAccount`：它只排入背景工作，不等結果。成功後跳 toast「已排入重新讀取，稍後重新載入」，按鈕約 5 秒後自動重讀一次。
- 「重試建立」只在有 `create_failed_time` 時出現，並顯示「建立失敗於 …」。呼叫 `RetryPaymentAccountCreate`，不加確認視窗：重試是冪等的，與現有的「重新綁定」一致。
- 徽章用 `StatusBadge`，文字來自共用文案目錄（與賣家用同一個 enum，admin 的文字要不要另外寫見 Q6）。
- 待補條件沿用 notification-templates 的 inline `<pre>`，不新建共用的 JSON 檢視元件。
- 身分與銀行的 KYC 卡（`KycIdentityCard` 等）步驟 7 不動，步驟 8 與欄位一起刪。

## 4. 條款

- `website/docs/legal/seller-kyc-notice.md`：
  - 改成「平台只收市集登記資料（統編、聯絡方式）；身分與銀行帳號由金流商（目前為 Stripe）依其隱私政策收集」。
  - 防詐比對碼的說明改成：統編，以及金流商提供的銀行帳號比對碼。
- `seller-agreements.json` 的 `PAYUNI_TERMS` 在步驟 8 拿掉（D11）：`GetSellerAgreements` 要求每個種類都有現行版本，必須和 `AgreementKinds()` 一起刪。7a 只讓送出不再要求它。

## 5. 要請 Will 決定的問題

- **Q1 後端必填清單怎麼放**（擋住整個步驟 7）：
  - 建議：步驟 7 開頭加一個子步驟 7a。內容是從 `requirements` 刪掉身分、銀行、證件與英文名稱，從 `SubmitKYCAgreements` 拿掉 `PAYUNI_TERMS`，並在目錄加收款帳戶文案。proto 欄位、密文欄位與 enum 照原計畫在步驟 8 刪。
  - 未採：把步驟 8 的 KYC 個資刪除整段提前，會讓步驟 7 變得太大。
- **Q2 公司的「補充文件（選填）」要不要保留**：
  - 建議：保留。它是平台審核市集政策用的公司證明，不是身分證件。
  - 另一個選項：一起刪，那平台就完全不收任何檔案。D22 寫的是「證件」，沒寫到公司文件。
- **Q3 seller-app 的回跳方式**：
  - 建議：回跳到 website 的靜態頁，App 回到前景時刷新，不新增依賴。
  - 另一個選項：用 universal link 自動切回 App。要新增 `app_links`，並設定 apple-app-site-association。
- **Q4 website 內嵌元件的依賴**：
  - 要新增 `@stripe/connect-js` 與 `@stripe/react-connect-js`，並用環境變數 `NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY` 放公開金鑰（不是秘密，但不進版控）。
  - 同意後我先核對兩個版本存在、發佈滿兩週，再列出版本號給你看。
  - 另一個選項：website 也只做託管頁面。但步驟 7 的驗收寫的是「以內嵌元件完成一次平台收件」，選這個就要改驗收。
- **Q5 已啟用之後的「管理收款帳戶」入口**：
  - 建議：步驟 7 不做。ENABLED 只顯示狀態，修改資料等 Stripe 寄信，或之後再加 Account management 元件。
  - 要做的話，HOSTED 需要另一種 account link（`account_update`），會改到 proto。
- **Q6 admin 的徽章文字**：
  - 建議：與賣家共用同一組目錄文字（待設定、審核中…）。後台另外顯示原始的 `status_reason`，就看得出細節。
