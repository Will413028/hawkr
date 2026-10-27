---
name: website-e2e-verify
description: Runs and extends website's Playwright e2e suite (website/tests/e2e/) against mocked server responses. Use when a website change needs browser-level regression evidence or new e2e coverage; covers the run command, the required API_BASE_URL override, parallel-session ports, mock conventions, and the wire-shape and mutation rules that keep a mock-based spec discriminating.
---

# website-e2e-verify

`website/tests/e2e/*.spec.ts` 以 Playwright 跑，所有對伺服器的呼叫都由 `page.route` 攔截，不需要後端。
它證明「前端在給定的後端回應下，畫面與流程對不對」；「後端真的回這個形狀」由 `backend-e2e-verify` 或後端測試證明。
一個改動兩邊都可能要跑。

## 怎麼跑

在 `website/` 執行：

```bash
API_BASE_URL=http://127.0.0.1:9/api/v1 pnpm exec playwright test tests/e2e/<file>.spec.ts --project=chromium
```

- **一律帶 `API_BASE_URL`**：`playwright.config.ts` 的 `webServer.env` 沒有這個變數時，預設指向舊公司的 API，未攔截的 SSR 請求會連出去。指向一個不存在的本機 port，漏掉的 mock 會失敗而不是連到外部。
- **用 `pnpm exec`，不用 `npx`**：在 git worktree 裡 `npx` 會沿目錄往上找到主 checkout 的 `@playwright/test`，報 `Requiring @playwright/test second time` 與 `No tests found`。
- 不用先 `pnpm dev`：`webServer` 會自己起 `next dev`（CI 是 `pnpm start`），`reuseExistingServer` 會沿用已在跑的那個。
- 與其他 session 平行跑時，自己固定一個 port：`PLAYWRIGHT_PORT=3177`。不要為了清 port 去 kill 別人的 `next dev`。
- Hawkr Agent（`features/procurement-assistant/**`）頁面由 `NEXT_PUBLIC_HAWKR_AGENT_ENABLED` 擋；dev flavor 預設開啟。若 `.env.local` 設成 `prod` flavor 或 `false`，相關 spec 會卡在等 `[data-assistant-mode="live"]` 直到逾時，此時前面加 `NEXT_PUBLIC_HAWKR_AGENT_ENABLED=true`。
- `@live` tag 的 spec 會打外部服務，不跑。

`package.json` 的封裝：`test:e2e`（全部）、`test:e2e:hermetic`（排除 `@live`）、`test:e2e:contract`（`@contract`）、`test:e2e:ui`（UI mode 除錯）、`test:reliability`、`test:seo`（另一份 config）。單一檔案用 `pnpm exec playwright test <file>` 或 `pnpm test:e2e -- <file>`，兩者等價。

website CI（`.github/workflows/website-ci.yml`）不跑 Playwright，e2e 的證據只來自本機執行；回報時寫明跑了哪些檔、哪些 project、幾個 pass／skip。

## Mock 慣例（`tests/e2e/helpers/mock-gateway.ts`）

- Playwright 對同一個 URL 用**最後註冊**的 handler。萬用攔截（`**/api/v1/app/**` 回中性 `{code:0,data:{}}`）先註冊，精確路徑在它之後、`page.goto` 之前註冊。
- 登入帳號來自單體的 identity Connect API（`GetSession`）：`setupGatewayMocks(page, { identity, session })` 組出標準回應；`seedStoredActiveRole(page, "buyer" | "seller")` 以 `addInitScript` 寫入 active role。Connect 回應用 `fulfillConnect`／`fulfillConnectError`。
- 登入狀態本身屬於 Clerk：mock 造不出 Clerk session，middleware 擋住的受保護頁面需要真的 Clerk session。寫新 spec 前先確認目標頁面在沒有 Clerk session 時是否進得去。
- 漏 mock 的症狀通常不是報錯，而是停在 loading 或按鈕永遠 disabled：先確認萬用規則有接住，再往下查。
- 採購相關的共用 mock 在 `tests/e2e/helpers/mock-procurement-rfq.ts`；已抽出共用 setup 的 spec，新情境直接呼叫共用函式，只換最後要斷言的那支 handler。現有 spec 以 `ls tests/e2e/*.spec.ts` 為準。

## Mock 必須是真實的 wire 形狀

mock 造得出任何形狀，也就造得出一個永遠會過的測試。mock、單元測試與程式碼註解共用同一個錯誤假設時，三者互相印證而沒有一個碰到現實。

1. **payload 形狀要有來源**：契約的唯一來源（根目錄 `api/` 的 proto、`backend/api/openapi/openapi.yaml`），或對本機後端實際打一次、把回應落檔再抄。不要從程式碼註解或既有 mock 抄，那兩者正是會一起漂移的東西。
2. **同一個概念在不同 transport 上形狀可能不同**（例如串流的 `outcome.interrupts` 與 unary 的 `result.interruptId`）。改其中一個時不要順手統一成另一個的形狀。
3. **後端契約有動時**：先在後端定案契約，再回填前端 mock，不要反過來猜。

## 新增或改測試的判準

- **先看它紅一次**：在改動前的程式碼上跑新測試，確認它失敗於預期原因；只對修正版跑綠燈不算驗證。
- **突變驗證**：對關鍵的錯誤狀態、race 或 rollback 斷言，把被測的 production 防護拿掉，spec 必須紅；有沒有修正都綠，代表 mock 沒有鑑別力。只把 mock 改成回 200 只證明 fixture 變了。
- 同步點用「request 已觀察到」或 UI 狀態，不用 sleep 猜請求送出了沒；控制回應時序用 deferred route。
- 同一個判準（例如面板的掛載、開啟鈕、`hidden`）只能有一個來源函式；`tests/hawkr-agent-candidate-canvas.test.ts` 是對原始碼字面的 `node --test` 斷言，重構成共用函式後要改斷言去釘「共用同一個值」，不要改回內聯條件讓它變綠。

## 領域入口

- Pricebook 全市場、雷達、追蹤清單：[references/pricebook-market-watchlist.md](references/pricebook-market-watchlist.md)
