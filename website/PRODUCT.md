# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

- 主要使用者是受邀的一般買家，常見情境是公司活動、員工福利或團購，需要一次採購約 30–100 人份的食品。
- 買家的工作不是學會採購術語，而是把自然語句整理成賣家能正式報價的精確需求，清楚比較條件，再自行確認交易。
- 平台賣家、受控外部業者、ADM 與客服是後續協作者；他們不改變買家對需求與成交的決定權。

## Product Purpose

「Hawkr Agent」是 Hawkr 訊息中心內的採購經理 Agent。它只追問真正缺漏、歧義或衝突的需求，形成可追溯的需求版本，再讓買家分別選擇平台賣家與外部業者詢價。成功指標是買家能提交可報價的精確需求、收到有效報價並安全進入既有交易與履約流程，而不是 Agent 自動替買家下單。

## Positioning

它把自然語句、動態澄清、可稽核需求版本、雙路候選與密封詢價接成一條採購經理流程；對話負責理解與導覽，固定且版本化的 action 才能送出、接受、付款或處理後續狀態。

## Operating Context

- Web 的唯一新對話入口位於既有訊息中心，採用 `/account/messages/assistant/procurement`；既有對話使用 `/account/messages/assistant/{conversationId}` 深連結。舊 `/assistant/new` 與 `/account/find-sellers` 只做相容導向，不再承載第二套 Agent runtime，也不把 Agent 偽裝成真人聊天對象。
- 買家先看到單題動態澄清、已回答摘要與 Request Snapshot；確認需求後才進入候選搜尋、分路選擇與詢價。
- 平台與外部兩條 sourcing lane 共用同一需求版本，但各自保存候選、勾選、確認、送出、冪等與恢復狀態。
- 交易時間模型為 48 小時詢價、24 小時比較、接受報價後 2 小時一次付清全額；付款成功才成立唯一訂單，之後由賣家自行履約。

## Capabilities and Constraints

- Agent 不重問已辨識欄位；一次只顯示一題，不顯示固定題數、百分比、模型提問理由或私密推理。
- 選項只更新本地草稿；按「下一步」才提交 typed answer。回上一題會建立新 branch，受影響的候選、價格與選擇由 server 宣告失效。
- 候選預設不勾選；「查看詳情」只讀資料，只有 checkbox 或明確加入動作會改變收件清單。
- 候選不是供貨承諾。正式有效報價必須包含品項與需求內容、數量、包裝、含運總價、交期、有效期與承接條款。
- 高於預算或外部目標邀價的有效報價仍完整顯示，不使用「最佳」或「最推薦」代替買家決定。
- 外部業者完成平台註冊、KYC、賣家資格與 PAYUNi 合作商店驗證前，只能比較報價，不能接受、建立付款保留或付款。
- Agent 與 ADM 不得取得賣家出貨能力；付款成功後仍由賣家備貨、出貨與處理物流。
- 公開處理進度只可呈現 allowlist 工作事件、權威結果、可恢復錯誤與下一步。使用者 prompt 只可出現在 canonical turn request 與依保留政策授權的對話資料，不得再複製到 public event、Activity Rail、analytics 或 support export；原始 tool I/O、chain-of-thought、secret、非必要 PII 與密封報價不得進入 DOM、accessibility tree、log、Sentry 或 analytics。
- 現行 AssistantWorkspace／useProcurementSession 是 Web canonical boundary；durable AgentRun ledger、replayable SSE、typed Artifact、immutable Quote Approval、付款與外部 onboarding 等後續契約仍須依 v6 文件的個別 gate 驗收。前端不得把 local worktree、fixture、CI、DEV-LIVE 或 PROD-LIVE 證據混寫，也不得把視覺完成誤稱為已啟用交易能力。
- Open decision：外部業者尚未完成平台驗證時，是否提供不具拘束力的「有興趣」動作，仍待產品／法務決定。

## Brand Commitments

- 產品名稱使用「Hawkr」與「Hawkr Agent」。Agent 以品牌記號與清楚的 AI 小標識別，不使用假真人姓名、真人頭像、已讀或在線狀態。小型身分表面沿用 `HawkrAgentMark`；Entry 是靜態 Hawkr 標誌，active run 使用 `HawkrAgentSprite`（處理中是標誌的載入動態，其餘狀態是靜態標誌），terminal Public Narrative 改用一枚語意 `AppIcon`，不重複 Agent avatar。三個 Entry 能力入口使用 `HawkrAgentCapabilityIcon` 的 `AppIcon` glyph，只是能力提示，不是 server 狀態或交易成功證據；文字 label 與 canonical event 才是權威。
- 延續現有 Hawkr 設計系統與 `AppIcon` 圖示入口；不引入紫色 AI 漸層、霓虹科技感或泛用 SaaS 儀表板語言。
- 繁體中文文案要讓一般買家能理解，不要求使用者先懂 RFQ、CAS、Grant 等內部術語。

## Evidence on Hand

- 需求基線：Google Docs 上的 `Hawkr_Hawkr 幫找 Agent_詳細設計書_v6.0`（文件 ID `11dUv-sP1xgURWz6eVIqqvovDaFfNW8qQAG_PANq6To0`）。文件中的 TARGET／FIXTURE、分支 source、CI、DEV-LIVE 與 PROD-LIVE 標籤必須保留其證據邊界。
- 現行品牌與介面規範：`DESIGN.md`、`styles/foundation/00-color-config.css`、`styles/foundation/01-tokens-theme.css`、`styles/components/ui-kit.css` 與 `components/ui/*`。
- 現行訊息中心：`app/[locale]/(account)/account/messages/*`、`features/messages/*`、`features/account/screens/messages/*` 與 `styles/features/messages.css`。
- 本 repo 只能證明 Buyer Web 程式與本地驗證；後端契約、PAYUNi、Email、KYC、CI、部署與正式環境需各自補證據。

## Product Principles

1. 先精確，再找貨；只問真正阻塞報價的問題。
2. 對話負責理解，typed artifact 與固定 action 負責交易事實。
3. 候選、報價、付款與訂單是不同承諾層級，文案與狀態不可混用。
4. 平台與外部兩條 lane 分別選擇、確認、送出與恢復。
5. 每一步可重播、可追溯，且只揭露完成任務所需的安全資訊。

## Accessibility & Inclusion

- 主要 touch target 至少 44×44px，狀態使用 icon 加文字，不只靠顏色。
- Desktop 新對話先顯示單一中央 Entry；CURRENT 候選階段可用 server candidate set 展開 38／62 候選事實工作台，但不得冒充 typed ArtifactEnvelope 或已驗證商家。未來其他成果只有真實 typed artifact 存在時才展開。Mobile 不自動喚起鍵盤，維持單欄並以具名 handoff 開啟較長 artifact；關閉後要保留對話位置與草稿。
- 問題切換後焦點移到題目 heading；overlay 需 focus trap、Esc、關閉後返回觸發器並使背景 inert。
- 倒數不逐秒向 screen reader 宣讀；支援鍵盤、中文 IME composing、200% zoom、WCAG AA 對比與 `prefers-reduced-motion`。
