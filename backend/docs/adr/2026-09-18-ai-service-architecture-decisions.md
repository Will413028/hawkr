# ai-service 架構決策集

| 項目 | 值 |
|---|---|
| 狀態 | accepted（D7／D8 已 superseded，見各節） |
| 日期 | 2026-09-18（決策本身的落地日期見各列「日期」欄；D7／D8 於 2026-09-19 隨 #3519 刀 A 收斂） |
| 影響範圍 | `services/ai-service`、gateway AI 路由、採購子系統（55／57） |
| 規格 | [15 號 AI 微服務詳細設計書](../詳細設計書/15-AI%20微服務（AI%20Service）詳細設計書.md) |

> 本檔只收**有被否決的替代方案、且否決理由今天仍然成立**的決策。日期取自實際落地的
> commit。規格本身（欄位、契約、參數值）在 15 號，本檔不複述。

## D1 修圖額度用 reserve → confirm／release 三段式 ＋ 每店一把 transaction-scoped advisory lock

**考慮過**：(a) 單句原子 UPSERT increment（「用量 +1 且 `WHERE` 用量 < 上限」）；
(b) 只在 reserve 取鎖、confirm／release 不取（TDD 過程中的中途版本）。

**決策**：三段式。否決 (a) 的理由**不是它不夠原子**，而是行程被 SIGKILL 時已 commit 的
increment 沒有任何東西會退回，賣家額度悄悄卡死——而滾動更新是常態。否決 (b) 是因為它在
測試裡被抓到真的 race：confirm 不持鎖時，並發的 reserve 會讀到 increment 前的舊已確認數，
而該預約已被刪出未過期計數，兩邊都少算 ⇒ 多放行一個名額。

`accepted`｜2026-07-13｜複審：+6 個月

## D2 額度上限、用量計數與名額預約一律以 `store_id` 為鍵

**考慮過**：額度上限綁 `account_id`（「那是給**這個人**的容量，不因換店重置」）——
這個版本在**同一天、隔一個 commit** 被自己推翻。

**決策**：一律綁 store。

**要保存的是分層原則而不是結論**：第一版的錯不在「給人不給店」這個產品判斷本身，而在它
讓兩張**同時被同一支流程讀取**的表用了兩種鍵——於是「這個賣家的額度」在同一支流程裡會有
兩個不同答案。真正的規則是：`account_id` 是**身份鍵**，`store_id`／`buyer_id` 是**業務鍵**，
**業務表一律用業務鍵**。這條原則的效力範圍遠大於額度。

⚠️ schema 現況只看得到結論、看不到反轉，因此這一列刻意保留翻案過程：兩次改名只隔一個
commit，只讀現況的人會以為「一開始就這樣設計」。

`accepted`｜2026-07-13｜複審：+6 個月

## D3 額度**判定**路徑 fail-closed

**考慮過**：DB 錯誤時退回預設上限（曾經的實際行為）。

**決策**：fail-closed。fail-open 等於「資料庫壞掉時額度變無限」。同時保留「只做顯示的查詢
維持寬鬆」這個不對稱——分界線是**這條路徑會不會決定放行**。

`accepted`｜2026-07-10｜複審：+6 個月

## D4 登入身分 → 現任代理鍵的解析全部搬到 Gateway

**考慮過**：ai-service 收 raw `account_id`，自己 callback seller-service／buyer-service 解析
（原做法，連同整個 client package 已整包刪除）。

**決策**：搬到 Gateway。與其他所有域一致的 resolve-at-gateway 做法，每次 AI 呼叫少一個網路
hop；更重要的是服務端不再有第二次解析，也就沒有第二處會解錯。

`accepted`｜2026-08-06｜複審：+6 個月

## D5 解析失敗一律 fail-loud

**考慮過**：解析失敗退回輸入的 `account_id` 當代理鍵（原行為，而且被寫進介面註解、
正常化成「刻意設計」）。

**決策**：fail-loud。raw `account_id` 當 store 送給下游**不會噴錯**，只會查到零筆，
與「這個賣家真的沒有資料」完全無法區分；dev 上有相當比例的賣家處於刪店重開狀態，會實際踩到。

`accepted`｜2026-08-05｜複審：+6 個月

## D6 賣買家助手的聊天走 coordinator ＋ specialist（agents-as-tools 委派）

**考慮過**：(a) coordinator ＋ sub-agents（`transfer_to_agent` 轉交控制權）——否決：轉交之後
跨領域的單一提問無法彙整，多步驟更難；(b) 維持單一 all-tools 聊天、只改 prompt 或換模型——
否決：工具選不準是「一個 agent 掛太多工具」的結構問題，加長 prompt 治標；(c) 不動，維持
純按鈕——否決：等於放棄聊天。

**決策**：coordinator ＋ specialist。specialist 只掛一兩個工具讓選擇單純，coordinator 全程
掌控、一輪內可鏈式呼叫多個再彙整。**代價誠實記錄：每次委派多一次模型呼叫。** 同時保留一個
環境開關可回退到 (b)。

`accepted`｜2026-07-10｜複審：+6 個月

## D7 新 gRPC 表面用 method-policy registry 做 fail-closed 授權，管轄範圍靠服務前綴白名單界定

**考慮過**：讓它對整個 server 生效（會把既有 `AIService`、gRPC reflection 與 health check
全部擋死）；把免身分的方法排除在管轄外或乾脆不登記（會讓「免身分」變成讀不出是誰決定的狀態）。

**決策**：兩層。這是「chained interceptor 是 server-wide」這條約束逼出來的形狀，不是偏好。
代價是兩張表沒有任何機制保證同步，只能靠一支 invariant 測試鎖進 CI。

**2026-09-19 更新（#3519 刀 A）**：D8 的複審條件（既有 `AIService` 的下一次改動）成立，
`AIService` 依授權軸拆分並全數納管，「讓它對整個 server 生效會把既有 `AIService` 全部擋死」
這個否決理由不再適用——沒有既有 `AIService` 需要被擋死。**這裡「考慮過」而否決的第一個方案，
現在就是實際做法**：`policyScopedServices` 服務前綴白名單與其兩層設計已刪除，改為單表
default-deny 對整個 server 生效，豁免只剩 health 與 reflection（第二個否決方案——「免身分排除
在管轄外或不登記」——仍然維持否決，`ActorNone` 依然是必須明確登記的合法狀態，不受這次收斂影響）。
兩張表沒有機制保證同步、只能靠 invariant 測試鎖進 CI 這條代價，隨第二張表一併消失。
規格見 [15 號 §6.2、§8.1](../詳細設計書/15-AI%20微服務（AI%20Service）詳細設計書.md)。

`superseded`｜2026-08-15｜2026-09-19 隨 #3519 刀 A 收斂，見上方更新

## D8 既有 `AIService` 刻意留在 method-policy 管轄之外

**考慮過**：一併納管（身分改成只從 metadata 取）。

**決策**：維持現況，並用測試把這個狀態釘住。納管會被 fail-closed 擋掉既有流量。
這是**接受一個形狀上的 fail-open**，換既有流量不中斷；緩解是網路層的 pod 白名單。

**2026-09-19 更新（#3519 刀 A）**：複審條件（「隨既有 `AIService` 的下一次改動一併重評」）
成立。重評結果與當時「考慮過」而否決的方案同方向、走得更遠：不是原地納管，是依授權軸拆分重編成
`AIImageService`／`AIAdminService`／`RecommendationService`，三者與 `AIAssistantService`／
`AIProcurementService` 一併納入單表 default-deny，身分只從 metadata 取（`AIAdminService`
額度三支的 `store_id` 除外——那是管理者指定操作目標，不是身分自報）。原本的代價（「納管會
擋掉既有流量」）之所以不再成立，是因為前提改變了：功能**尚未正式上線**，可收緊契約、可
fail-closed，不必再顧慮既有流量中斷。這不代表當時的判斷是錯的——在「已有既有流量」的前提下，
維持現況、用測試釘住現況，仍然是對的取捨。規格見 [15 號 §1.3、§6.2、§8.1、§11](../詳細設計書/15-AI%20微服務（AI%20Service）詳細設計書.md)。

`superseded`｜2026-08-15｜2026-09-19 隨 #3519 刀 A 收斂，見上方更新

## D9 HTTP 面只剩探活與指標，業務流量全走 gRPC，且不留 HTTP fallback

**考慮過**：(a) 維持 HTTP／SSE；(b) big-bang 一次切；(c) 三階段 additive → cutover → 刪除。
另外「保留 HTTP fallback」被單獨否決。

**決策**：(c) ＋不留 fallback。分階段換到可獨立回滾（回滾 Gateway 而不動 ai-service）；
不留 fallback 是因為留了會讓 gRPC 的問題被舊路徑掩蓋。

`accepted`｜2026-08-17｜複審：+6 個月

## D10 資料存取層從 gorm 換成 sqlc ＋ pgx，分六波各自 PR 與部署驗證；gorm 永久留在 `go.mod`

**考慮過**：(a) 維持現狀（起因的直覺判斷是「gorm 沒在用，直接移除」，盤點後證實相反：
它接在正在服務的路徑上）；(b) 一次性大爆改。

**決策**：六波。**否決 (b) 的理由是回滾粒度**：每一波各自 PR、各自 merge 並確認 dev 部署
正常之後才推下一波，波次之間沒有交叉依賴，出事只要退掉那一波。big-bang 的失敗面是整個
資料存取層，而換 driver 最容易錯的地方（型別 override、`ON CONFLICT` 分支、刪除語意）
往往要等真流量打到那條查詢才現形。代價是遷移期間兩套 driver 必須在同一個 fx 圖裡共存。

⚠️ **「移除 gorm」不是可達終點**——ADK 的 session service 建構簽名只接受 gorm dialector，
要移除必須自製 pgx 版，屬另立專案的量級。

`accepted`｜2026-08-21｜複審：隨 ADK 升版重評

## D11 平台側與採購側各自一個 pgx 連線池，刻意不合併

**考慮過**：共用單一池；不具名 provide。

**決策**：不合併。採購刻意與既有 AI 表零外鍵、目標是可整包抽出，共用連線池會在抽出那天
製造耦合——**這個理由與資料存取層用什麼工具無關**。具名不是風格偏好：DI 圖上已有採購那個
同型別的池，同型別重複提供會直接建圖失敗。

`accepted`｜2026-08-21｜複審：隨採購抽離與否重評

## D12 採購與賣買家助手共用同一個 ADK `session.Service` instance，採購側用另一個 ADK app 名稱分區

**考慮過**：在採購側重新 provide 一次 session service。

**決策**：共用 instance、分開分區。重新 provide 會產生第二個各自獨立的連線池與第二套
session service 設定，即使底層是同一張表；而分區名稱分開，是為了抽成獨立服務時 session
資料不會與其他 domain 混在一起。兩件事方向看似相反，其實一個管連線與設定、一個管資料分區。

`accepted`｜2026-08-09｜複審：隨採購抽離與否重評

## D13 三份共用資料一律用行程內 TTL 快取，本服務零 Redis 依賴

**考慮過**：接一個 Redis 進來。

**決策**：行程內快取。風格清單、商品推薦、分類索引三者的共同性質決定了它：全站共用、
體積小、可重算、過期一點也不會出錯。Redis 要付的是一個新的網路依賴、一套新的失效語意
與一組新的故障模式，換到的只有「跨 pod 一致」。

**代價誠實記錄**：部署窗口內兩個 pod 的快取不一致是已知且被接受的，它影響「Admin 剛改的
風格多久全站看到」，不影響任何決定放行的判定。

🔴 **觸發條件**：副本數要往上長時，要換的是**快取策略本身**，**不是替現況補一層 Redis**。

`accepted`｜2026-04-05｜複審：副本數要往上長時重評

## D14 採購的抽取 provider 做成三選一（gemini／deepseek／gemma），部署預設 deepseek，code 層 fallback 留 gemini

**考慮過**：固定單一 provider；預設維持 gemini。

**決策**：三選一 ＋ 預設 deepseek。**否決理由是一次有數據的比較**（2026-08-08 benchmark）：

| provider | 通過率 | 延遲 |
|---|---|---|
| deepseek | 30／30 | 約 1.4s |
| gemma（OpenRouter） | 25／30 | 約 4.3s |
| gemini（既有基準） | 約 50%（方法論不同，不直接可比） | — |

code 層 fallback 刻意留 gemini，讓本機與測試零設定可跑。連帶紀律：k8s 宣告 optional、
把關留在 application 層 fail fast。

⚠️ 這份數據原本只存在於 `k8s/values/ai-service.yaml` 的註解與一則 commit 訊息中，
前者被整理一次就會消失——那正是它必須寫在這裡的原因。

`accepted`｜2026-08-08｜複審：換模型或換 provider 時重評

## D15 採購的報價外送刻意不接 MQ

**考慮過**：(a) 現在就接一條 publisher 上去；(b) 外送函式回 error，讓未實作的列走既有的
重試與失敗計數。

**決策**：只做到交易性 outbox 落地 ＋ worker 掃描 ＋ lease 認領，外送函式留成介面骨架，
記一筆指名道姓的 Warn 然後把該列標成完成。否決 (a)：本服務的設定結構沒有任何 MQ 連線
欄位，憑空接一條上去等於**替下一階段先決定它的形狀**。否決 (b)：回 error 會讓「投遞尚未
實作」這個已知狀態**長期偽裝成告警**，把真正的投遞故障淹掉。

`accepted`｜2026-08-15｜複審：55／57 下一階段開工時
