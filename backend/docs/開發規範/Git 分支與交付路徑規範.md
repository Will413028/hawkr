# Git 分支與交付路徑規範

> **適用範圍（2026-09-24 起）**：不再適用：這是公司時期 `dev`、`main` 雙分支與 promote 的流程。monorepo 只有 `main` 一條分支，規則見根目錄的 `AGENTS.md`。


> 制定：2026-08-05。
> 源起：2026-06-08 連續 4 個 PR base=`main` 造成 dev 部署 baseline drift ＋ prod 付款 down 9 分鐘。
> 本檔為最終權威。
> 相關：[開發工作流程](開發工作流程.md)、[Cloudflare Worker 與 Serverless 部署治理規範](Cloudflare%20Worker%20與%20Serverless%20部署治理規範.md)

## 1. 分支策略（backend-payuni）

| Branch | 角色 | 修改方式 |
|---|---|---|
| `main` | prod | 僅由 `dev` 合入 |
| `dev` | dev cluster 集成 | **所有 PR 的 base** |
| `dev-alexis` | 個人長期工作分支 | 所有 commit 在此 |

**標準流程**：

```
git fetch origin
git checkout dev-alexis
git merge origin/dev
<commit>
git push
gh pr create --base dev --head dev-alexis
```

→ 盯 CI（失敗自動修一次）→ 綠了 merge → 盯 deploy ＋ smoke → dev 驗證。

**例外**：僅「prod 緊急事故」經使用者授權可開 `hotfix/*` → `main`，同時 cherry-pick 回 `dev`。

**禁止**：`base=main` 的 PR（唯一例外即上述 hotfix）；未經授權另開工作分支。

### 1.1 破壞性契約變更的告知義務（#1728，2026-07-22）

對外契約的**破壞性變更**（硬切欄位／改端點）merge 進 `dev` 之後，**會隨團隊例行的
dev→main 合併一起上 prod**。若前端／依賴端尚未就緒，prod 直接炸——OTP 硬切 07-21 上 prod、
web/App 未同步，登入 detour 白屏。

**PR body 寫「前端配合後才可上 prod」沒有用，沒人看。** 此類 PR 必須三者做齊才算盡到告知義務：

1. 標題加 `[HOLD-MAIN]` 前綴
2. 開一張 blocking issue 釘住「dev→main 的前置條件」
3. 在 Slack／口頭明確告知會執行 dev→main 的人

> 但在套用這三件套之前，先讀 §3——要先問「這個窗口需不需要它」。

## 2. 各 repo 的交付路徑（2026-08-04 使用者定版，不要再自己推定）

| repo | 路徑 | 把關者 |
|---|---|---|
| `backend-payuni` | **PR**，base 一律 `dev` | CI |
| `hawkr-website` / `hawkr-admin` | **直接 push `dev`**，禁開分支 | CI（push 觸發） |
| `hawkr_buyer` / `hawkr_seller`（Flutter） | **直接 push `dev`**，禁開分支 | **本地驗證 ＋ Xcode Cloud** |

### 2.1 Flutter 兩個 App repo 的放行條件（六項齊備才可 push，缺一不可）

0. **`dart format --output=none --set-exit-if-changed lib/` exit 0**
   ⚠️ 不要接 pipe——`tail`／`head` 會吃掉 exit code。
1. **`flutter analyze lib/` 乾淨**（本地語法確認）
2. **`flutter test` 全綠**（2026-08-04 使用者裁決納入）——兩個 repo 的 CI 都有獨立的測試 job、
   seller 另有 coverage ratchet，代表團隊在乎它；而 App 沒有 dev 環境可部署驗證，本地就是最後一道關。
3. **本地模擬器完整測試**——一律 `flutter run` 直連遠端 dev 後端，
   **禁用既有 build／`Runner.app` 走 `simctl install` 跳過**。
4. **留下證據截圖**（每個受影響機能：修前／修後）。
5. **主對話親自 Read 過每一張截圖並確認結果沒問題**——不接受只看 agent 的文字回報；
   agent 說「已驗證」不算，你自己沒看過就不算。

**放行六項之外，還有一步在 push 之後：回填 App 版本 gate 的 `minimum_version`**

上面六項守的是「這份 code 能不能推」；這一步守的是「推出去的版本，強制更新機制認不認得」。

**時機在送審通過／實際放量之後**（不是 push 當下，也不是送審當下）：
到 admin 的「App 版本管理」，把該 `app_type × platform`（BUYER／SELLER × IOS／ANDROID）
的 `minimum_version` 更新為**這個剛發布的版本**。

⚠️ **填一個尚未在商店上架的版本，會把所有使用者鎖在更新畫面外且無法自救**——
商店根本沒有那個版本可以裝。**admin 頁送出前的二次確認（並列四組現值 ＋ 警語）是唯一防護**：
後端沒有「已發布版本」的權威來源（Android build number 走 `git rev-list --count`、
iOS 走 Xcode Cloud run 序號，皆非 semver；repo 內也無商店查詢管道），
因此做不出伺服端拒絕條件，**這一步只能靠人記得**。

漏做不會報錯、不會有 log，只會**靜默**：gate 永遠停在上線那天的版本，等於不存在。
機制見 [content 模組規格](../modules/content.md) §5 與 [App 版本門檻 Runbook](../runbooks/app-version-gate.md)。

### 2.2 為什麼 Flutter 與 web 的把關方式不同（理解這個才不會又推定錯）

- Flutter 的 `flutter_quality.yml` 觸發條件是 `pull_request`，**push `dev` 不會跑**——
  這是設計如此，不是漏洞。
- **App 沒有 dev 環境可以部署驗證**。web 推上 `dev` 就有一個真的站可以打；
  App 推上去只是進版控，真正的建置與分發交給 **Xcode Cloud**。
- 所以 **client 端 App 的品質保證重心在「本地測試結果」**，不在 CI。
  這是**把關位置的差異，不是標準比較鬆**。

#### ⚠️ merge 進 `main` 就是發版，沒有中間關卡（2026-08-07 使用者澄清）

**Xcode Cloud 是 Apple 雲端直接 watch GitHub `main` 的 changed event 自動部署，不經 GitHub Actions。**

推論有三，都會影響決策：

1. **`dev` → `main` 的 promote 本身就是「發版」動作**，不存在「先合進 main、之後再決定要不要發」。任何必須在發版前完成的前置（例如後端設定、feature flag、契約對側就緒），都得在**按下 merge 之前**做完。
2. 因此 **`main` 上的 App code 必須隨時處於「可以立刻出現在使用者手機上」的狀態**。`[HOLD-MAIN]` 這類「先合著、暫時別上」的做法對 web 可行（部署還有一關），對 App **不成立**。
3. GitHub Actions 上與 iOS 建置／分發相關的 workflow（TestFlight、App Store Connect、fastlane）**都已無作用**——實際部署鏈不經過它們。看到這類 workflow 紅燈或 cancelled，先確認它是不是早已交棒的死檔，再判斷要不要修。

**Android**：目前**未正式發佈、暫時無上架計畫**（同日澄清）。因此 CI 的 Android 產物檢查（APK size budget 等）保護的是一個目前不存在的交付路徑——已改為手動觸發，不隨 PR 自動跑（`hawkr_buyer#93`）。未來要上架時再把 trigger 加回來。

### 2.3 教訓（2026-08-04）：CI 沒跑 ≠ 流程違規

頭像修復推上兩個 App repo 後，發現 CI 沒觸發 → 判定「我推錯路徑、應該走 PR」→ 對使用者宣稱自己違規。
**兩次都錯**：第一次是沒查就推；第二次是查了一半（只看到 `on: pull_request`）就把「CI 沒跑」
等同於「流程違規」，沒有問「這個 repo 的把關者本來是不是 CI」。使用者當場澄清規則。

**判準**：發現「預期會跑的東西沒跑」時，先確認**它本來就該跑嗎**，再下違規結論。
把「與我的預期不符」直接讀成「有人做錯了」，是同一種未查證宣稱。

### 2.4 教訓（2026-08-04，同日第二件）：只掛 `pull_request` 的 gate 在 dev 上從未執行

補跑 `workflow_dispatch` 驗證上述條件時，buyer 的 Static Checks 紅了——**`format: failure`**，
而 buyer repo `dev` 分支上有 3 個檔案（08-01／08-02 推入）自始未通過 `dart format`。

**成因正是 §2.2 描述的結構**：格式關卡只掛 `pull_request`，而該 repo 的慣例是直接 push `dev`
→ **這道 gate 在 dev 上從未執行過**，漂移無聲累積。

⚠️ step 層會顯示 `Check Dart formatting: success`（它掛 `continue-on-error`），
真正的 `outcome: failure` 只在最後匯總 step 才現形——**看 job 的 step 清單會看走眼，要看匯總輸出**。

→ 所以放行條件第 0 項（`dart format`）不是形式要求，它是這個 repo **唯一**會執行的格式把關。

## 3. 跨 repo 破壞性遷移：什麼時候可以直接切、什麼時候必須擴張—收縮

§1.1 那三件套（`[HOLD-MAIN]` ＋ blocking issue ＋ 口頭告知）**全部依賴人記得**，
而 #1728 已經證明它會失效。**但在補強它之前，要先問對的問題：這個窗口需不需要它。**

**判準只有一個：prod 上有沒有會被打斷的真實流量。**（注意不是「有沒有真實使用者」——prod 已上架、有真人，但這不等於有存量交易。）

| 階段 | 做法 | 理由 |
|---|---|---|
| **prod 已上架但尚無存量交易**（現況，2026-08-06 實查；見 56 號設計書 A5） | **直接切**，代價僅限同波發布的協調 | 相容層**保護不到任何人**。加了才是技術債——這正是本 repo 一路在刪的東西 |
| **prod 已有真實交易流量** | **必須擴張—收縮** | 先加新端點 → 前端切換上線 → **確認舊端點零流量後才移除**。任何時間點的 prod 都自洽，不依賴兩個 promote 的順序或時機 |

⚠️ 第一列的「現況」是**會過期的前提**，不是永久事實：動手做破壞性切換之前**重查一次 prod 交易量**，翻掉了就走第二列。

⚠️ **不要把上線後的儀式套到上線前**（會製造保護不到人的相容層），
也**不要把上線前的捷徑帶到上線後**（會製造真實使用者看得到的破口）。

### 3.1 擴張—收縮要成立的唯一條件

那個「之後會移除」的承諾**必須被機械保證**。舊端點加 `REMOVE-WHEN: <可量測條件>`
（例如「流量連續 N 天為 0」），並讓 CI 在條件滿足卻還沒移除時轉紅。
**沒有這一步，擴張—收縮就退化成「留著沒人敢刪」的技術債。**

### 3.2 教訓（2026-08-03）：兩個 repo 各自 promote 的時間差

`crop-names` 端點在 backend 移除、website 同步改用新端點，**dev 上兩邊配對完整、順序也正確**。
但 `dev→main` 是**兩個 repo 各自 promote**——website 那次部署失敗，於是 prod 出現
「後端已移除、前端還在呼叫」的窗口。時間差設計上只有 15 分鐘，**因為部署失敗實際變成數小時**。

**這不是誰粗心，是流程結構本身允許它發生。**

而真正讓它拖那麼久的是另一件事：**website 的 main 部署失敗當時不會發告警**
（step 層通知看不到 `deploy-cloudflare` job 的失敗），發現它純粹靠有人在輪詢——
已補 job 層 `notify-main-failure`。
