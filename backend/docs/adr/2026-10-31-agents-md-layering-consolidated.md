# AGENTS.md 分層、大小上限與子領域檔案（整合 2026-10-28-agents-md-layering 與它的三則修訂，加上 F＋B）

| 項目 | 值 |
|---|---|
| 狀態 | accepted；取代 `2026-10-28-agents-md-layering.md`（該檔修訂已滿 3 段，依 `README.md` 慣例整合；它的脈絡、方案與歷史保留在原檔） |
| 日期 | 2026-10-31（Will 決定 F＋B）；其餘決定日期見各條 |
| 影響範圍 | repo 內所有 `AGENTS.md`、`cmd/new-module` 的模板、`.instruction-lint.toml`、`.pre-commit-config.yaml`、`docs/開發規範/設計文件治理規範.md` 的文件 taxonomy、單體 ADR `2026-09-24-go-modular-monolith.md` 對模組 `AGENTS.md` 的定義 |
| 執行計畫 | `docs/plans/2026-10-28-agents-md-layering.md` |

## 脈絡

`AGENTS.md` 是 coding agent 的指令檔，碰到目錄就整份載入。2026-10-28 的問題（seller 模組那份 168,637 bytes，原因是規格逐條寫進指令檔）由第一版決策解決，完整的脈絡與被否決的方案見原 ADR。2026-10-31 的新問題：上限擋住了一次爆量，擋不住逐條累積。每條不變量都往同一個 6,000 bytes 的檔案加一行，加到上限就有人被迫刪規則或硬塞。證據（量測，`git log` 與 `wc -c`）：identity 在連續 5 個 commit 內 6,003 → 8,662 bytes；social 6,416；seller 6,335／6,436；social 在整理後 5,996，離上限 4 bytes。

約束：
- 外部：agent 的 context 有上限，指令檔碰目錄就整份載入。Codex 串接的合計上限預設 32KiB（Codex 文件，未在本機實測）。
- 繼承：模組固定七節、各層上限（本 ADR 的前身所訂，我們自己的決定，可以改）。
- 外部（工具行為，2026-10-31 查證文件）：Claude Code 在讀到子目錄的檔案時才載入該子目錄的指令檔；Codex 從根目錄串接到工作目錄；agents.md 規範是離編輯檔案最近的那份優先、每個 package 一份。

## 考慮過的方案

| 題目 | 方案 | 代價 | 判定 |
|---|---|---|---|
| 逐條累積 | 基準：指令檔保持短（Claude Code memory 文件建議每份 200 行以內），子目錄各放自己的檔案、碰到才載入（Claude Code memory、agents.md、Codex AGENTS.md 文件） | 多幾個小檔；規則要分「跨子領域」與「單一子領域」 | 採用（F） |
| | 維持單檔，上限不動，超過就刪規則 | 刪的是守門測試的入口；每次新增不變量都要和別人搶位置 | 否決 |
| | 調高模組上限 | 等於沒有上限；碰模組任何檔都付更多 context | 否決 |
| | 拆成多個模組 | 架構變更（依賴表、schema、事件），原 ADR 已否決併入 | 否決 |
| 防回彈 | 只靠 CI 擋（推上去才發現） | 超限的 commit 已經寫好，回頭改成本高 | CI 保留 |
| | 加 pre-commit hook，動到 `AGENTS.md` 時跑同一支腳本（B） | 要裝 hook；檢查本身是毫秒級 | 採用 |

## 決策

1. **三層與上限**（沿用，2026-10-28）：根目錄 12,000 bytes；子專案（repo 第一層目錄）10,000；模組、`packages/*` 與更深的目錄 6,000；單行 500 字元。根目錄放專案地圖、跨專案契約、共通規則；子專案放指令、技術慣例、特有的坑；模組放固定七節（職責、擁有的資料、發布的讀取介面、模組內部結構、不變量、不負責、決策紀錄），`cmd/new-module` 的模板與 `TestGeneratedAgentsHasOnlyTheFixedSections` 守住。大小由 instruction-lint 依 `.instruction-lint.toml` 檢查，例外（allowlist）有理由與期限（2026-10-29 起為空）。
2. **模組行為規格**在 `docs/modules/<模組>.md`，是唯一來源；舊設計書依 `docs/開發規範/設計文件治理規範.md` §7：新規格完整涵蓋且描述的系統已停運就刪，否則標為該服務的現行規格、搬進單體時刪（沿用，2026-10-28）。
3. **兄弟共用的規則**（沿用，2026-10-28）：能機械檢查的交給 CI 或測試；描述不存在的流程的刪除；內容已在文件裡的，指令檔只留「做 X 前先讀 Y」；不指向兄弟目錄的指令檔（不會被載入）。真正共用、無法檢查、又不屬於根目錄的規則才放一份中立檔、以 `@路徑` 引用，且要把引入的 bytes 算進上限；目前這類規則是 0 條，不建機制。
4. **子領域檔案（F，2026-10-31）**：模組內某個子領域（`internal/<子領域>/`）有只涉及它的不變量時，放在 `backend/internal/modules/<模組>/internal/<子領域>/AGENTS.md`，一個子領域一份、選用、上限同樣 6,000。固定形狀：標題、一行目的、指向 `docs/modules/<模組>.md` 的指標，然後是不變量，每條一句規則加守門測試與規格位置。模組根的 `AGENTS.md` 仍是固定七節；它的「不變量」只列跨子領域的規則加一行指向那些檔案，「擁有的資料」指向 `db/schema.sql` 並依子領域分組、不逐表列。搬移只搬不改：每一條的意思不變，搬的對照記在 commit 內文。行為規格仍以 `docs/modules/<模組>.md` 為唯一來源。第一次套用：seller、identity、social。
5. **本機 hook（B，2026-10-31）**：`.pre-commit-config.yaml` 的 `instruction-lint`（pre-commit 階段，動到 `AGENTS.md`、`CLAUDE.md` 或 `.instruction-lint.toml` 時觸發，掃全部追蹤的檔），執行與 CI 相同的 instruction-lint。`default_install_hook_types` 加入 `pre-commit`，既有 clone 要重跑一次 `pre-commit install`。CI 的步驟不變。

## 理由

指令檔的成本是每次載入，文件的成本是需要時才讀。單一檔案的上限讓所有規則爭同一塊空間，每新增一條就更接近上限；按子領域放，碰哪個子領域才付那個子領域的成本，跨子領域的規則才常駐。子領域目錄本來就存在、就是改動時會碰的地方，所以載入點和規則的適用範圍對得上。pre-commit 把「超限」從推上去之後才發現，提前到寫的當下。

## 已知限制

- Claude Code 是在 Read 子目錄的檔案時載入該目錄的指令檔；只用 Edit／Write 而沒有先 Read 的情況，是否載入未確認。緩解：改既有檔案之前本來就會 Read；跨子領域的規則仍在模組根。
- Codex 只串接根目錄到工作目錄，所以只有工作目錄在子領域內時才看得到子領域檔案；從模組根或 repo 根工作的 session 看不到。緩解：模組根有一行列出有哪些子領域檔案，加上規格與守門測試本身會擋違規。
- 子領域檔案是選用的：沒有只涉及單一子領域的規則就不建。

## 預期結果

- 模組根 `AGENTS.md` 不隨每條新不變量成長；seller、identity、social 的根檔在 4,500 bytes 以內（量測：4,463／3,380／4,101），留 1,500 bytes 以上的餘裕。
- 碰任何模組時，路徑上的 `AGENTS.md` 合計仍在 28,000 bytes 以內。
- 新模組的骨架仍只有七個固定節，`main_test.go` 守住，也允許（不要求）子領域檔案。

## 撤銷條件

- agent 工具改成按需載入指令檔的片段，或 Claude Code 與 Codex 改變子目錄指令檔的載入行為時，重新評估上限與子領域檔案。
- 子領域檔案出現同一條規則在兩處漂移的實例時，重新評估位置。
- 規格放 `docs/modules/` 後，出現同一條規格在兩處漂移的實例時，重新評估位置。

## 來源

- Claude Code memory：<https://code.claude.com/docs/en/memory>（每份 200 行以內；子目錄的檔案在讀到該目錄時才載入）
- AGENTS.md 規範：<https://agents.md/>（離編輯檔案最近的優先；每個 package 一份）
- Codex AGENTS.md：<https://learn.chatgpt.com/docs/agent-configuration/agents-md>（從根目錄串接到工作目錄；合計預設 32KiB）

## 修訂：檢查改由 instruction-lint 執行（2026-10-31，Will 決定）

脈絡：檢查腳本 `scripts/check-agents-md.py` 只服務這個 repo，同類檢查在其他 repo 各寫一份。改用共用工具 instruction-lint（Will 自有，釘 v0.2.0），規則與上限集中在根目錄 `.instruction-lint.toml`。

決策：決策 1 與 5 的檢查器改為 instruction-lint；三層上限、單行 500 字元、七節、hook 觸發條件與 CI 步驟的數值和範圍不變。例外清單改為設定檔的 `allowlist`，每筆有理由與到期日（最長 90 天）。模組七節的來源改為設定檔的 R2 shape，`cmd/new-module/instruction-lint.json` 由它生成（`instruction-lint config --format json`），`TestGeneratedAgentsHasOnlyTheFixedSections` 讀這個檔，CI 比對生成檔與設定一致。原決策段保留歷史。
