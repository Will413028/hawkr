# Web 前端契約同步規範

> **適用範圍（2026-09-26 起）**：對外契約改為根目錄 `api/` 的 Protobuf＋Connect（ADR `backend/docs/adr/2026-09-26-external-contract-protobuf-connect.md`），TS client 由共用套件 `@hawkr/api` 的 `pnpm --filter @hawkr/api gen:api` 生成（website、admin 共用），慣例在〈API 設計規範（Connect）〉。§四的工具選型與 §五的遷移策略由該 ADR 與遷移計畫「模組完成的定義」取代。還沒搬進單體的模組仍用手寫的 client，§六的開發者守則對它們仍然適用，查 wire 形狀時看現有 API 清單 `backend/api/openapi/openapi.yaml`。

> 對應 `hawkr-website` (Next.js + TS) 與 backend-payuni gateway 之間 OpenAPI / 型別契約的同步機制。**目標**：讓後端 swagger 變動 → web 端 TS 型別自動跟隨；杜絕「後端改了，web 沒同步，跑起來才壞」。

## 一、當前狀態（2026-08-13 實查）

| 項目 | 現狀 |
|---|---|
| Backend swagger 規格 | **Swagger 2.0**（`swaggo/swag` 產出）— 不是 OpenAPI 3.x |
| Backend swagger 檔案 | `gateway/docs/{swagger.json,swagger.yaml}`，由 `swag init` 從 handler 註解生成。**不在版控**（2026-08-05 起 gitignore），唯一消費者是 Apidog；要在本地取得就自己跑 `cd gateway && make swagger`（見 `Gateway Swagger 註解規範.md` §3）|
| Backend 路徑數 | 約 370（重數方式：把 `gateway/internal/` 底下 handler 註解的 `@Router` 路徑去重後計數）|
| Web 端 wire types | **手寫**於 `lib/api/openapi-types.ts` — 註解標明「Source of truth: 預設模組.openapi.json」但實際是手寫對照 |
| Web 端格式約定 | **JSON body 一律 camelCase**（backend#2235 收斂，見 [API 設計規範](./API%20設計規範.md)）；snake_case 只剩 URL / query string 參數，且在呼叫點組出來、不寫進型別 |
| 同步機制 | **無**（依賴開發者自覺）|

## 二、問題與風險

1. **後端改 swagger 註解（欄位 / endpoint / 狀態碼）後，web 端 TS 不會自動跟**
2. Web `openapi-types.ts` 是手寫 → 容易漂移
3. TS compiler 抓不到「我以為的型別 ≠ 後端真的回的型別」
4. 部署後才在 runtime 看到 type mismatch

## 三、目標流程

```
後端 handler 註解改動
  → CI 重產 gateway/docs/swagger.json
  → web 端透過 codegen 從 swagger.json 產出 generated types
  → web 端 commit generated types
  → tsc 抓到 cascade，逐處修正
```

## 四、工具選型評估

`openapi-typescript`（主流）**只支援 OpenAPI 3.x**，不吃 Swagger 2.0。要對 Swagger 2.0 用，有三條路：

| 方案 | 做法 | 優點 | 缺點 |
|---|---|---|---|
| **A. 升 backend swagger gen 到 OpenAPI 3** | Go side 改用支援 OpenAPI 3 的 swagger 套件（如 `swaggo/swag v2` beta 或替代套件） | 一次解決，web 直接用 openapi-typescript | 影響 backend，需驗證生成輸出與 Gateway 路由註解兼容性 |
| **B. swagger2openapi 中轉** | web 端先用 [swagger2openapi](https://github.com/Mermade/oas-kit) 把 swagger.json 轉 openapi3.json，再餵 openapi-typescript | backend 不動 | 雙工具鏈、雙生成步驟、處理 conversion edge cases |
| **C. swagger-typescript-api** | 原生支援 Swagger 2.0 + OpenAPI 3，單一工具直接吃 swagger.json | 單一 dep | 與 openapi-typescript 生態不同，學習成本 |

**推薦**：**方案 C**（`swagger-typescript-api`） — 單一 dep、直接吃當前 swagger.json、不動 backend。

## 五、漸進遷移策略

不立即取代 `lib/api/openapi-types.ts`（含業務 enum / helper / Role mapping），分階段：

### Phase 1：工具引入（PR 1）

- 加 `swagger-typescript-api` 到 web `devDependencies`
- 加 npm script：`pnpm gen:openapi`，從本機 `backend-payuni/gateway/docs/swagger.json` 產出 `lib/api/.generated/types.ts`
- `.gitignore`：**不** ignore `.generated/`（要 commit 進去，避免 CI/CD 跑 codegen 動作）
- 加 `package.json` postinstall hook（可選）

### Phase 2：第一個 endpoint 試點（PR 2）

- 選一個小 endpoint（例 `/app/buyers/me`）
- 把對應手寫 type 從 `openapi-types.ts` 改成從 `.generated/types.ts` re-export
- 跑 `pnpm tsc` 確認 cascade 全綠
- 部署 dev 驗證

### Phase 3：批次遷移（PR N）

- 依優先級分批：高頻 endpoint 先（feed / order / cart / checkout）
- 業務語意 helper（`Role`、`ACCOUNT_TYPE`）**保留**在 `openapi-types.ts` 不動
- 純 wire interface 全換成 generated re-export

### Phase 4：CI 自動同步

- backend `.github/workflows/deploy-k8s.yml` 部署完成後，**通知**（或自動 PR）到 web repo 重 generate
- 或 web `.github/workflows/sync-types.yml` 排程拉 backend swagger.json + 跑 generate + 開 PR

## 六、開發者守則（當前 / 過渡期）

在 codegen pipeline 未上線前：

### 後端工程師動 swagger 註解時必做

1. `cd <repo>/backend/gateway && make swagger`（生成物已 gitignore，不會進 diff）
2. `grep -A 3 -B 1 "<old_field>" docs/swagger.yaml` 確認舊註解位置
3. 改 handler `// @Param` / `// @Success` 註解（**只改註解**，生成檔不入版控也不必 commit）
4. **grep 三個前端 repo 的對應引用**（動 wire 欄位 / API path / 錯誤碼時強制）：
   ```bash
   grep -rn "<old_field>\|<old_endpoint>" ~/Git/hawkr-website/lib/
   grep -rn "<old_field>" ~/Git/hawkr_buyer/lib/ --include="*_model.dart"
   grep -rn "<old_field>" ~/Git/hawkr_seller/lib/ --include="*_model.dart"
   ```
   Flutter 側欄名多半藏在 `@JsonKey(name: "...")`，要一併 grep。**命中卻只推後端 PR ＝ 部署後 web / buyer / seller 立刻壞**。
5. 命中 web `openapi-types.ts` → inline / handoff 處理

### Web 工程師串接新 endpoint 時必做

1. `cd <repo>/backend/gateway && make swagger && grep -A 20 "<endpoint>" docs/swagger.yaml` 看 wire 型別（生成物已 gitignore；或直接查 Apidog）
2. 在 `lib/api/openapi-types.ts` 手動補對應 interface（**JSON body 欄名 camelCase**；query string 的 snake_case 參數在呼叫點組出來，不寫進型別）
3. 在 `lib/api/<domain>.ts` 寫 client method
4. 引用業務 helper（`Role`, `ACCOUNT_TYPE`）走原 `openapi-types.ts` exports

## 七、相關規範

- [Gateway Swagger 註解規範](./Gateway%20Swagger%20註解規範.md) —— 契約的 SSOT 是 handler 註解本身；`gateway/docs/` 生成物不入版控，唯一消費者是 Apidog
- [Git 分支與交付路徑規範](./Git%20分支與交付路徑規範.md) —— `hawkr-website` 直接 push `dev`（禁開分支），backend-payuni 一律走 PR

## 八、待決議

- [ ] 方案 A/B/C 最終選擇（建議 C，待 PR 1 驗證）
- [ ] Generated types 是否 commit 進 git？建議**是**（避免 CI 跑 codegen 變動）
- [ ] Phase 4 自動同步機制（cron / webhook / GitHub Action）
