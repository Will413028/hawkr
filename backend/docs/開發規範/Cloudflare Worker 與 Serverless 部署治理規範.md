# Cloudflare Worker 與 Serverless 部署治理規範

> 現況（作品集轉 public 步驟 2，D7）：公司時期的部署環境設定已刪除——`infra/mq/envs/{dev,prod}`、`mq-topology.yml` 等部署 workflow、`monitoring/` 的 prod exporter 與 `prod-infra.json`、image-gateway 的 dev／production 環境與 custom domain、website／admin 的 `wrangler.jsonc` env 區塊。下文提到這些的部署、plan／apply 與環境操作段落是公司時期的歷史紀錄，不是現行流程；部署設定等選定 demo 部署目標時重寫。

> 制定：2026-07-28（源起：backend#1907 盤點——prod 網頁 API 反代中樞 `api-first-party-proxy` 僅存在於 CF dashboard，無版控、無 review、不可重建）。
> 本檔為最終權威。

## 1. 原則（不可違反）

所有 serverless 執行單元（Cloudflare Worker / Pages、GCP Cloud Run 等）一律：

1. **源碼入版控**——含 wrangler/部署配置（routes、bindings、cron triggers 隨配置檔納管）。
2. **變更走 PR → review → 測試 gate**——與一般程式碼同等待遇。
3. **CI 部署**——禁止 dashboard 手寫、禁止本機 `wrangler deploy` / `deploy.sh` 直上正式環境（= 全域「唯一上線路徑」原則的 serverless 面）。

理由：手動部署 = 線上版本與 repo 對應關係不可考、繞過全部測試防線、災難時不可重建。

## 2. 現有 Worker ↔ 版控 / CI 對應表（SSOT）

| Worker | 源碼位置 | CI |
|---|---|---|
| hawkr-website-dev / -prod | hawkr-website repo | 同 repo `deploy.yml` |
| hawkr-website-tail-alerts | hawkr-website `cloudflare/tail-worker/` | 同 repo `deploy.yml`（website#347 納管） |
| hawkr-website-canary-dev / -prod | hawkr-website `cloudflare/canary/` | 同 repo `deploy.yml`（`--env dev`/`production`；快取新鮮度探針＋SLO，cron 每 5 分鐘、無 route，website#339 M2；**＋失效後合成讀者（`WarmQueue` DO，backend#2269）**——收 `/warm` 通知後兩階段暖讀首頁，保證 `"max"` profile 條目有讀者觸發重生，見詳細設計書 53 §4.5；觀測設施 continue-on-error 不改寫主站部署成敗） |
| hawkr-admin-dev / -prod | hawkr-admin repo | 同 repo `deploy.yml` |
| image-gateway-dev / -production | backend `workers/image-gateway/` | backend `deploy-worker.yml` |
| api-first-party-proxy | backend `workers/api-first-party-proxy/` | backend `deploy-worker.yml`（#1907 納管；單一 script，僅 main 部署） |
| ~~hawkr-router~~ | 已退役（2026-07-28）——/sitemap* 為網頁遷 Workers 前殘留（網頁 Worker 原生服務 sitemap，此 route 反攔截）、/s/* 為死功能零引用；源碼存證於 git 歷史 `workers/hawkr-router/` | — |
| hawkr-apple-pay-domain | backend `workers/hawkr-apple-pay-domain/` | backend `deploy-worker.yml`（#1907 納管；單一 script，僅 main 部署。實測為 .well-known 的實際服務者且涵蓋 api./test-api. host，判定保留） |

- 新增 / 退役 Worker 時**必須同步更新本表**與 `docs/基礎建設/系統架構與資料流向.md` 的 Worker 表。
- **2026-08-04 稽核**（`/workers/scripts` 實查）：帳號線上共 11 支 script，與本表**逐支對得上、零缺口**——`api-first-party-proxy`、`hawkr-admin-dev`/`-prod`、`hawkr-apple-pay-domain`、`hawkr-website-canary-dev`/`-prod`、`hawkr-website-dev`/`-prod`、`hawkr-website-tail-alerts`、`image-gateway-dev`/`-production`。同日修正本表一處**重複列**（canary 曾被寫成兩列，同名同源碼；canary 只有 **1 份源碼 × 2 個 wrangler env**，不是兩支獨立 worker——重複會讓「表列數 vs 線上數」的稽核永遠對不上）。
- **CF Pages projects**（2026-07-28 #1907 治理後終態）：`hawkr-docs`（manual.example.com，源碼 `<org>/Hawkrmanual`，該 repo GitHub Actions wrangler 部署——合規）；`eureka-frontend`（使用者個人專案，非 hawkr 資產，移出治理範圍）。殘留三項（hawkr-admin / hawkr-website 舊 Pages、hawkr-seller-web 實驗）已刪除，刪除前完整存證見 #1907。新增 Pages 同受本規範約束。
- GCP serverless（`serverless/egress-proxy/`，Cloud Run）：源碼版控＋CI 部署（`.github/workflows/deploy-egress-proxy.yml`，push main path-filter＋dispatch；重用 deploy.sh 為單一真相，deployer SA=egress-proxy-deployer）。tgos-proxy 已於 2026-07-28 收編退役（TGOS 轉發併入 egress-proxy 白名單；backend#1914，服務快照存證於票內、源碼存證於 git 歷史 `serverless/tgos-proxy/`）。

## 3. 新增 Worker 流程

1. 源碼放對應 repo（後端支撐類 → backend `workers/<name>/`；前端應用附屬 → 該應用 repo 內，如 website `cloudflare/`）。
2. `wrangler.jsonc` 完整宣告 routes / bindings / cron / observability（`"observability": { "enabled": true }`）——dashboard 不得持有任何 repo 外配置。
3. 部署接入該 repo 既有 CI（backend：`deploy-worker.yml` 加 path-filter；website / admin：`deploy.yml`）。
4. secret 一律 `wrangler secret put` 手動管理（CI 不落 secret 明文），並在該 worker 的 README 列出 secret 清單與來源。
5. 更新 §2 對應表＋架構文件。

## 4. 稽核方式（盤點缺口用）

```
# 列出帳號全部 Worker，逐一對照 §2 表
curl -s -H "Authorization: Bearer <Hawkr_ReadAll>" \
  https://api.cloudflare.com/client/v4/accounts/<account_id>/workers/scripts | jq -r '.result[].id'
# Pages projects 同理：/pages/projects
```

判定：任何線上 script / project 在 §2 表中無對應 → 治理缺口，開票納管或退役。`modified_on` 晚於該 worker 最後一次 CI 部署 → 有人手動改過，立即比對差異。

## 5. Zone 層配置（Ruleset / DNS）治理

制定：2026-07-30（源起：backend#2024——該 zone 的三條 cache rule **只存在於 dashboard**，
沒人知道內容是什麼，其中一條 expression 用錯欄位、從未生效也沒人發現）。

Worker 有 CI 可以綁，**zone 層配置沒有**：Cloudflare Ruleset（Cache Rules / Single Redirect /
Configuration Rules / Rate limiting / WAF custom）與 DNS 記錄只能在 dashboard 或 API 改。
所以這一層的治理手段是「**快照入版控 ＋ drift 稽核**」，而不是 CI 部署：

1. **SSOT 位置**：`k8s/external-services/cloudflare/`
   - `rulesets/<phase>.json`：機器可讀快照，正規化後可直接當 entrypoint PUT 的 payload
   - `cache-rules.md` / `redirect-rules.md`：人類可讀記錄（規則語意、已知缺陷、重建指令）
   - `README.md`：zone 全部配置的索引（含 ruleset 清單、DNS 殘留盤點、稽核指令）
2. **改動流程**：dashboard 唯讀 → 用 API 改（或 dashboard 改完立刻）→ **同一個 PR 回寫快照**。
   沒回寫 = 下一個人看到的 repo 是錯的，等於白做。
3. **稽核**：`k8s/external-services/cloudflare/audit-zone-config.sh`（唯讀，線上 vs 快照，drift 時 exit 1）。
   發現 drift 時**先判斷哪邊才對**再同步，禁止「線上這樣就這樣」地覆蓋快照——那會讓 dashboard 重新變成 SSOT。
4. **CF 託管 ruleset 不納管**（Normalization / Managed Free Ruleset / DDoS L7）：內容由 Cloudflare 維護，我方只能開關。
5. **DNS 記錄退役**：刪除前必須 grep 全部 repo 的內部消費者（worker 上游 / env / 硬編碼），
   **零流量不是充分判準**（刪 CF Pages 曾因只看流量而造成 sitemap 530 兩小時）。
   退役步驟：先降級（關 proxy 或改指向）觀察 → 確認第三方平台側已停用 → 才刪 DNS → 保留原值以便回退。

**expression 寫法紅線**：路徑判斷一律用 `http.request.uri.path`。
`http.request.full_uri` **含 scheme 與 host**（`https://host/path?query`），
拿它配 `starts_with("/...")` 恆為 false ——這個錯誤在本 zone 已出現兩次（一條舊 www 轉址、一條 well-known bypass），
兩次都是「規則看起來存在、實際從未生效」，最難發現的那種壞。

## 6. 禁止事項

- 禁止在 CF dashboard 建立 / 編輯 Worker 與 Pages（唯讀查看不限）
- 禁止本機 `wrangler deploy` 上 dev / prod（本機 `wrangler dev` 開發不限）
- 禁止 dashboard 持有 repo 外的 routes / 環境變數配置（secret 除外）
- 測試性 Worker（如 smoke 驗證）用完必刪，不得殘留線上
- 禁止改了 zone ruleset 而不回寫 `k8s/external-services/cloudflare/rulesets/` 快照（§5）
