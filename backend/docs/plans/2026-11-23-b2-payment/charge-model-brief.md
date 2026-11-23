# B2 買家付款的 Stripe Connect 收款模式（charge model）簡報

## 0. 前提問題
- 有沒有要改的東西已排定重寫？沒有。ADR `docs/adr/2026-11-13-payment-onboarding-provider-port-stripe.md` 第 41 行選 destination charges，但自己標了「推論，未核出處（推測）」；master plan B10 寫 separate charges and transfers。兩份文件互相矛盾，這份簡報要做的就是二選一（核對出處：ADR 第 41 行）。
- **真實的限制**：
  - 賣家是 v2 `recipient` 帳戶，只有 `stripe_transfers`。recipient 帳戶不能申請 `card_payments`，所以不能用 `on_behalf_of`，也不能用 direct charges（直接在賣家帳戶上收款）（核對出處：docs.stripe.com/connect/service-agreement-types、docs.stripe.com/connect/destination-charges「settlement merchant」段）。
  - 平台是 US，賣家要在 US/CA/UK/EEA/CH 才能跨境轉帳（核對出處：separate-charges-and-transfers「Cross-border transfers」段）。
  - 平台承擔負餘額（`losses_collector=application`）（題目給定）。
  - 「出貨並完成後才把錢給賣家」：這是產品規則（賣家自己出貨、買家確認收貨或逾期自動完成、有售後）（題目給定，fulfillment ADR）。
- **自己加的限制**：沿用 PayUni「訂單完成後撥款」的做法本身不算理由（AGENTS.md「沿用也是決策」）。但從零設計，C2C 市集一樣需要「履約前先由平台保管」，所以這條規則保留（推論）。

## 1. 業界基準
- Stripe 的分法（核對出處：https://docs.stripe.com/connect/charges）：
  - destination charges 是「建立付款後**馬上**把錢轉給賣家」，適合 Airbnb、Lyft。
  - separate charges and transfers（簡稱 SCT：付款與轉帳分開做）的適用情況寫了「付款時還不能轉帳」、「一筆付款分給多位賣家」，也寫了「多賣家共用購物車的電商市集」。
- Stripe 不提供 escrow（法律上的第三方託管），但可以延後撥款到賣家銀行（payout），US 最多放 2 年（核對出處：https://docs.stripe.com/connect/manual-payouts）。
- Stripe 自己的 marketplace 指南：用 destination charges 或 SCT 都可以（核對出處：https://docs.stripe.com/connect/marketplace）。
- Shopee、Lazada 的 escrow、eBay managed payments 的延後撥款、Etsy 對新賣家的保留款：做法都是平台先收，履約或確認後才放款（推論，憑一般認知，本次沒有逐一核對這些來源）。

## 2. 選項
**A. destination charges（付款時就轉給賣家）**
- 優點：每筆付款一次 API 呼叫就完成；退款時帶 `reverse_transfer=true`，會依比例把賣家那份拉回來（核對出處：destination-charges「Issue refunds」段）。
- 缺點：錢在付款時就進了賣家帳上，和「完成後才撥款」的規則相反。出貨前的取消、售後都要靠轉帳撤回（reversal），而賣家餘額不夠時，退款請求會直接報錯（核對出處：connect/charges 的 Refunds 表）。
- 什麼情況會壞：賣家一提領，退款就拉不回錢，只能由平台吸收。
- 可逆性：改成其他模式只需要改 adapter，資料模型幾乎不用動（推論）。

**A'. destination charges 加上賣家改成手動撥款（manual payouts）**
- 錢留在賣家的 Stripe 餘額裡，由平台決定什麼時候撥到銀行（2 年內）（核對出處：manual-payouts）。
- 缺點：
  - 法律上這筆錢已經是賣家的；平台要另外記帳追蹤哪些錢可以撥（推論）。
  - `on_behalf_of` 不能用（recipient 限制）。
  - 一筆付款只能有一個收款方，以後如果做多賣家購物車就得重做（核對出處：connect/charges）。

**B. SCT，轉帳時帶 `source_transaction`，在訂單完成時建立 Transfer（推薦）**
- 付款時錢留在平台；訂單完成時才建立 Transfer。
  - 帶上 `source_transaction` 後，即使付款的錢還沒變成可用，轉帳也一定建立成功，等款項可用時才實際執行（核對出處：separate-charges-and-transfers「Transfer availability」段）。
  - recipient 帳戶收到轉帳後要多等 24 小時才可用（核對出處：service-agreement-types）。
- 完成前的取消或退款：只動平台退款，不需要撤回任何轉帳。
- 完成後的售後：退款加上對該筆轉帳做 reversal；賣家可用餘額不夠時就拉不回來，由平台吸收。這和 `losses_collector=application` 一致（核對出處：Refunds 表、transfer reversal 段）。
- Stripe 寫明「只在你承擔 connected account 負餘額時才建議用 SCT」，hawkr 正好符合（核對出處：SCT「Cross-border transfers」段末）。
- 代價：
  - 要多做一個「完成即轉帳」的 worker（類似舊的 GrantPaymentWorker），加上冪等鍵與對帳。
  - 轉帳時如果 `stripe_transfers` 已不是 active，會失敗，所以要有重試和人工處理（推論）。
- 什麼情況會壞：見第 4 節。
- 可逆性：高。轉帳時點是我們自己的 worker 決定的，要改成付款即轉也只是調整時點（推論）。

## 3. 建議（只依真實限制調整）
採用 B：SCT 加 `source_transaction`、`transfer_group=order_id`，訂單完成時轉帳，賣家撥款維持自動（預設排程）。
- 理由一：「完成才撥款」這條規則，只有 B 能讓錢在完成之前留在平台的帳上。
- 理由二：recipient 帳戶本來就排除了 `on_behalf_of`，A 和 B 在費率、帳單上的商家名稱上沒有差別（推論，依 connect/charges 的 fees 段）。
- 理由三：B 保留了以後做多賣家購物車的可能，但這不是現在的需求，不拿來當主要理由。
- 需要更正 ADR 第 41 行：把「destination charges（推測）」改成 B，並附上述出處。

## 4. 什麼事實會推翻這個建議
- 平台付款後能保管款項的期限：SCT 頁沒寫上限（只核對了 SCT 頁，沒有全站搜尋）。如果 Stripe 有一個比「完成期限加售後期限」還短的保管期限，建議就要改成 A'。
- 如果正式上線時 Stripe 審核認定「平台保管款項」屬於需要牌照的資金傳遞（money transmission），也要改成 A'。這只影響上線，現在是測試模式的作品集，不受影響（推論）。

## 5. 最終狀態
- 付款：PaymentIntent 建在平台上，帶 `transfer_group=<order_id>`。
- 訂單完成時：用 `source_transaction=<charge>` 建立 Transfer，金額為「付款金額 − 平台抽成」，冪等鍵用 order_id。
- 完成前退款：只對平台做 refund。
- 完成後退款：refund 加上 transfer reversal；賣家餘額不夠時，由平台吸收並記帳。
- 上架閘門維持「`stripe_transfers` 為 active」。
