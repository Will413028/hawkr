# hawkr

電商平台，服務買家、賣家與平台營運三方。

每個子專案有自己的 `AGENTS.md`，在該目錄工作前先讀它。本檔只放跨專案的地圖、契約與規則。指令檔一律是 `AGENTS.md`；個人或本機的補充放在 `AGENTS.local.md`（不進版控）。

## 地圖


| 目錄 | 內容 | 與 CI 相同的檢查 |
|---|---|---|
| `backend/` | Go 後端，正在從 `backend/services/` 的微服務遷移成 modular monolith | `.github/workflows/backend-*.yml` |
| `website/` | Next.js 買家與賣家網頁（Cloudflare Workers） | `.github/workflows/website-ci.yml` |
| `admin/` | Next.js 營運後台 | `.github/workflows/admin-ci.yml` |
| `buyer-app/` | Flutter 買家 App | `.github/workflows/buyer-ci.yml` |
| `seller-app/` | Flutter 賣家 App | `.github/workflows/seller-ci.yml` |
| `api/` | 對外 API 的 Protobuf 契約（Connect） | `.github/workflows/api-contract.yml` |
| `packages/hawkr-api-ts/` | `api/` 的 TypeScript 生成物（`src/gen`）與 Connect 執行期（錯誤分類 `connect/errors`、重試 `connect/retry`、表單送出分類 `connect/submit`）與手寫的 `proto/date`（`google.type.Date` 與 `YYYY-MM-DD`）、`proto/field-mask`（依 descriptor 檢查路徑的 update_mask）、`text/server-text`（Go `TrimSpace` 與以 rune 計長的伺服器文字規則），套件名 `@hawkr/api`，website 與 admin 共用 | `.github/workflows/api-contract.yml` |
| `packages/hawkr_api/` | `api/` 的 Dart 生成物，兩支 App 共用 | `.github/workflows/api-contract.yml` |
| `packages/hawkr_auth/` | Clerk session 與原生 OAuth 接線，兩支 App 共用 | `.github/workflows/dart-packages.yml` |
| `packages/hawkr_app_update/` | App 版本門檻的檢查邏輯，兩支 App 共用；行為規格在 `docs/spec.md` | `.github/workflows/dart-packages.yml` |
| `packages/hawkr_twaddr/` | 內政部 8 碼鄉鎮市區代碼表與查詢；資料檔 `lib/src/twaddr_data.g.dart` 由 `backend/internal/platform/twaddr` 的 CSV 生成，不得手改，查詢是手寫的。兩支 App 共用 | `.github/workflows/dart-packages.yml`、`.github/workflows/twaddr-generated-diff.yml` |
| `packages/hawkr-icons-ts/` | Material Symbols（Apache-2.0）的 SVG 圖形與繪製元件 `MaterialSymbol`，套件名 `@hawkr/icons`；`src/material-symbols.gen.ts` 由 `tools/gen.ts` 依手寫的 `src/names.ts` 從 `@material-symbols/svg-400` 生成，不得手改，drift 由套件自己的測試守。website 與 admin 共用 | `.github/workflows/ts-packages.yml` |
| `packages/hawkr-twaddr-ts/` | 內政部 8 碼鄉鎮市區代碼表與查詢的 TypeScript 版，套件名 `@hawkr/twaddr`；資料檔 `src/twaddr.gen.ts` 由 `backend/internal/platform/twaddr` 的 CSV 生成，不得手改，查詢 `src/lookup.ts` 是手寫的。website 與 admin 共用 | `.github/workflows/ts-packages.yml`、`.github/workflows/twaddr-generated-diff.yml` |
| `packages/hawkr-ui-text-ts/` | 共用文案目錄（契約值到使用者面文字）的 TypeScript 版，套件名 `@hawkr/ui-text`；`src/**/*.gen.*` 與 `tests/*.gen.*` 由 `backend/cmd/uitext-gen` 生成，不得手改，`src/format.ts` 與守門工具 `tools/check-enum-text.ts` 是手寫的。website 與 admin 共用 | `.github/workflows/ts-packages.yml`、`.github/workflows/ui-text-generated-diff.yml` |
| `packages/hawkr_ui_text/` | 共用文案目錄的 Dart 版（純 Dart）；`lib/src/ui_text.g.dart`、`tool/enum_manifest.g.json`、`test/golden.g.json`、`test/golden_calls.g.dart` 由 `backend/cmd/uitext-gen` 生成，不得手改，`UiLocale` 與守門工具 `tool/check_enum_text.dart` 是手寫的。兩支 App 共用 | `.github/workflows/dart-packages.yml`、`.github/workflows/ui-text-generated-diff.yml` |
| `brand/` | 作品集改名後的品牌素材（`brand/hawkr/`：logo 的 SVG 與用法） | 無 |
| `scripts/` | 跨專案的檢查 | `.github/workflows/repo-hygiene.yml` |
| 根目錄供應鏈設定 | `SECURITY.md`、`.github/dependabot.yml`、`.github/zizmor.yml`、`.gitleaks.toml` 與 `.gitleaks-baseline.json`、`osv-scanner.toml`、`scripts/govulncheck-allowlist.json` | `.github/workflows/supply-chain.yml`（govulncheck、OSV-Scanner、gitleaks、zizmor） |


- `packages/hawkr-*-ts` 只發佈 TS 原始碼：消費端以 `transpilePackages` 編譯，用 subpath exports，不做 barrel（集中匯出的 `index.ts`）。
- 所有 CI 都在根目錄的 `.github/workflows/`；子專案不放 `.github/`。公司時期的部署與維運 workflow 不在現行範圍。
- 工具版本：Go 看 `backend/.go-version`；Node 看根目錄 `.nvmrc`（website、admin、api-contract 的 CI 都讀它）；pnpm 看根目錄 `package.json` 的 `packageManager`，website 與 admin 是同一個 pnpm workspace，在根目錄 `pnpm install`（一份 `pnpm-lock.yaml`，兩個 app 共用的依賴版本在根目錄 `pnpm-workspace.yaml` 的 `catalog`），之後的指令在各 app 目錄執行；Flutter 看各 App 與 `packages/` 各套件的 `.fvmrc`；buf 看 `api/.buf-version`。

## 跨專案契約


| 契約 | 唯一來源 | 消費端 |
|---|---|---|
| 對外 API（Protobuf＋Connect） | `api/`（`buf lint`、`buf breaking`） | website、admin 經 `@hawkr/api`，兩支 App 經 `packages/hawkr_api`，client 不放自己的生成碼。各生成端用自己目錄的 `buf.gen.yaml`（backend 是 `buf.gen.connect.yaml`）生成：backend `make connect`、TS `pnpm --filter @hawkr/api gen:api`、Dart `../../api/generate.sh buf.gen.yaml`；不得手改生成檔。慣例：`backend/docs/開發規範/API 設計規範（Connect）.md`；決策：ADR `backend/docs/adr/2026-09-26-external-contract-protobuf-connect.md` |
| 舊 gateway 的 REST（`/api/v1`，還沒搬進單體的模組） | 現有 API 清單 `backend/api/openapi/openapi.yaml`；gateway 由 CI 對齊它 | 四個 client 的手寫 client。模組搬進單體時改用上一列生成的 client，刪除手寫的呼叫 |
| 身分驗證（顧客 Clerk、後台 Cloudflare Access） | ADR `backend/docs/adr/2026-09-27-identity-clerk.md`（含文末修訂）；單體在 `backend/internal/httpapi/rpc/session.go` 讀 Bearer（顧客 web 沒有時才讀 Clerk `__session` cookie），由 identity 依 issuer 驗證 | website 經同源反代打 `/api/connect/*`、`/api/v1/*`，帶 `getToken()` 的 Bearer，導覽下載靠 cookie；App 以 `clerk_auth` 帶 Bearer。admin 整個網域在 Access 後面，BFF 把 `Cf-Access-Jwt-Assertion` 轉成 Bearer（本機用 dev admin token），不讀 team domain 或 AUD tag（只放單體部署設定）；金鑰與這些設定都不進版控 |
| 共享代碼值（列舉、錯誤原因） | 對外看得到的：`api/` 的 proto enum 與 `ErrorReason`；儲存的代碼：擁有資料的模組（Go 型別＋CHECK）；顯示文字：各 client 的呈現層（ADR `backend/docs/adr/2026-09-25-codemeta-retirement.md`） | CodeMeta（`backend/docs/代碼管理（CodeMeta）/`）凍結，只服務還沒搬進單體的舊契約，隨模組搬遷退場；不新增值域、不手改生成檔 |
| 共用文案目錄（只由契約值決定的使用者面文字；分界與守門細則見 ADR `backend/docs/adr/2026-10-17-multi-locale-architecture.md` 的「目錄分界」修訂） | `backend/internal/platform/uitext/catalog/`，backend `make uitext` 生成；格式見 `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-Lprime-design.md` | Go `internal/platform/uitext`、TS `@hawkr/ui-text`、Dart `hawkr_ui_text`；模組的伺服器句子在模組的 `internal/text`。client 不在目錄或伺服器擁有的 enum 成員確定成立的分支或資料裡產生文字，不寫寫死的錯誤後備句，同一個 enum 的對照表不在兩個 client 各寫（`check-enum-text` 等守門） |


改契約時，同一次變更內更新唯一來源與所有消費端。三端 design token 由 website 的 `pnpm design:check --apps-root ..` 比對（website CI 會跑）。

## 共通規則

- **契約先於實作**：前後端不一致時，先找該契約的唯一來源判斷誰該改，不以放寬後端驗證當作預設修法。issue 與既有實作是線索，不是規格。
- **不留技術債**：不以「之後再清」為由留下臨時寫法；不為了避開單一案例加特判，那代表設計有問題，回頭修設計。
- **上線前不受相容性約束**：系統從未上線。公司時期為已上線系統訂的相容性規則——對外 API 同一版本不改契約、DB migration 的 expand-contract、App 舊版相容——在上線前都不是約束。做設計決策時以業界在同樣條件下的最佳做法為準，「和現有程式或公司時期規範一致」不能單獨當成限縮選項的理由。上線後這些規則才生效。
- **沿用也是決策**：搬遷或修改既有程式時，每個照搬的機制（ID 方案、extension、套件、共用 helper、舊的檢查邏輯）都先回答「從零設計會怎麼做」。答案不同就提出來決定，不默默在既有結構上修補，也不只記成待決。搬進單體的模組把結論寫進模組 `CARRYOVER.md` 的沿用盤點表（不放 `AGENTS.md`：它每次都載入，只放現行規則）（`backend/docs/plans/2026-09-24-go-modular-monolith.md`「模組完成的定義」）。
- **UI 改動**：bug 修正、純粹跟隨 API 契約的調整可以直接做；版面、UX 文案與流程的改動要先取得同意。取得同意後仍要先重用既有元件與 pattern，樣式一律走 design token，沿用既有的密度與互動；真的沒有可重用的才新建，並先說明為什麼。
- **使用者面文字**：買家與賣家介面是 zh-TW／en，admin 只有 zh-TW；語系的選擇、文字資源與伺服器文字見 ADR `backend/docs/adr/2026-10-17-multi-locale-architecture.md`，用語以 `backend/docs/開發規範/使用者面用語術語表.md` 為準。
- **依賴與供應鏈**：已知漏洞的修復期限自發現日起算：critical 7 天、high 30 天、medium 90 天、low 在下一次相關更新時；暫不修的放進基準（`osv-scanner.toml` 的 `IgnoredVulns`、`scripts/govulncheck-allowlist.json`），每筆寫理由與期限，期限到了 CI 會失敗。agent 不自行新增或升級依賴：先列出套件名稱與版本，核對該版本存在、發佈滿兩週，經同意才改；`uses:` 一律釘完整 40 碼 SHA 並在行尾註明版本。
- **Git**：只有 `main` 一條分支。commit 使用 Conventional Commits，例如 `feat(backend): …`，只提交本次範圍的檔案，不用 `git add -A`。push 依使用者授權。
- **驗證**：完成前跑與 CI 相同的檢查並看結果；看 CI 時看每個 job 自己的結論，不只看 run 的整體狀態。沒有驗證到的部分明確說出來。

