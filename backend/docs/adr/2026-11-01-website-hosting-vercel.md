# website 部署在 Vercel（function 區域 hnd1），單體部署在 OCI 東京 `oci-a1`；開 Cache Components／PPR

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-11-01 |
| 決定者 | Will（2026-11-01；兩份獨立草稿都建議自架，比對後維持 Vercel） |
| 影響範圍 | website 的部署設定、OpenNext／wrangler 與 Cloudflare 綁定的程式、`next.config.ts` 的渲染模型、website 的登入狀態；單體的部署位置 |
| 上層決策 | `2026-09-24-go-modular-monolith.md`（修訂其「部署目標維持 k8s ＋ Cloudflare」）；`2026-10-30-identity-self-hosted-federated-login.md` |

## 脈絡

- 系統從未上線，website 也從未部署（公司時期的部署 workflow 不在現行範圍）。
- website 現在以 OpenNext 1.20.6 打包成 Cloudflare Workers：R2 存 ISR 快取，3 個 Durable Object；約 40 個檔綁著 Cloudflare（middleware 的 `cf-*` header 與 workers.dev canonical host、CSP、Sentry tag、Apple Pay 關聯檔）。
- `cacheComponents`／PPR 刻意關閉：公司時期 #338 在 OpenNext/Workers 上 PPR 造成正式環境白畫面與 Worker OOM；OpenNext 的快取攔截（命中時不載入 22MB server bundle）也要求沒有 PPR。
- 因此登入狀態只能在瀏覽器探測（每次載入打 GetSession、2.5 秒 gate、約 48 檔依賴 `hydrated` 類旗標）。Will 要求登入狀態改由伺服器端決定，治本不留債。

約束：

- 外部：Vercel Hobby 方案限非商業使用，條款把「向訪客請求或處理付款」列為商業；Pro 每月 20 美元（Vercel Fair Use Guidelines、Pricing，2026-11-01 查）。
- 外部：Next.js 16 的 Cache Components 只支援 Node runtime；`'use cache'` 內不能讀 cookie（Next.js docs: Cache Components）。
- 外部：Vercel function 預設區域是 `iad1`（美東）；動態洞每次請求要打單體，區域要和單體同在東京。
- 外部：`oci-a1`（A1.Flex 4 OCPU／24GB，Always Free）同時跑真錢的交易 bot（homelab ADR `2026-09-26-single-vm-with-workload-boundaries`）。
- 繼承：「部署目標 k8s ＋ Cloudflare」來自公司時期；理由（公司既有叢集）已不存在，不再成立。
- 繼承：#338 與 OpenNext 快取攔截只在 Workers 上成立；換平台即不成立。
- 繼承：約 40 檔 Cloudflare 綁定是自己的程式、從未部署，改寫成本不是外部約束。

## 考慮過的方案

| 方案 | 代價 | 結論 |
|---|---|---|
| A. Vercel（`hnd1`）＋ Cache Components／PPR（業界基準：Next.js 的一級平台，Next.js docs: Deploying、Partial Prerendering） | 綁 Vercel 的快取與部署；開真實付款後每月 20 美元 | 採用 |
| B. 自架 Next `output: "standalone"` 於 `oci-a1`，Cloudflare 當 CDN | 0 元；但反代、TLS、ISR 快取存放、圖片、部署管線、監控都自建；和交易 bot 搶 CPU／記憶體，要硬限制 | 否決 |
| C. 另開一台 OCI Always Free VM 自架 Node（Caddy ＋ Cloudflare） | 0 元、與 bot 隔離；營運項目同 B；需自有網域 | 否決（自架選項中較佳者，見撤銷條件） |
| D. 留在 Cloudflare Workers，不開 PPR，用提示 cookie ＋ client 島 | 平台限制留著：公開頁的身分仍在 client 決定，PPR 等 OpenNext 修好 | 否決 |

## 決定

1. website 部署在 Vercel，function 區域設 `hnd1`（東京）。系統以測試模式金流運作期間用 Hobby。
2. 單體部署在 OCI 東京 `oci-a1`，以容器 CPU／記憶體上限與交易 bot 隔離。
3. 開 `cacheComponents`（PPR）。登入狀態由伺服器端的 DAL（`verifySession()`，讀 `__Host-hk_session` 問單體）決定，以 Suspense 動態洞串流；client 只接收伺服器的初值，不再自己探測。
4. 刪除 OpenNext、wrangler、R2／Durable Object 與 Workers 專屬程式，不留雙平台相容層。

## 理由

- 選 A 而非 B／C：三者渲染模型相同；差別只在營運。A 用每月 0–20 美元與廠商綁定，換掉 CDN、快取、預覽部署、憑證與部署管線的自建與維護；這些不是作品集要展示的重點，自建後每一項都是長期維護成本。
- 選 A 而非 D：D 保留的限制全來自 Workers；換平台後治本做法（伺服器端決定身分）才可行，D 只是治標。
- Hobby 的判斷是推論：測試模式不收真錢、無人獲利，判斷不算商業用途；不確定時依條款問 Vercel Support。
- 單體放 `oci-a1` 而非另開 VM：已有、同區（和 `hnd1` 同在東京）；代價是和 bot 共機，以容器資源上限處理。

## 預期結果

- 公開頁的靜態 shell 從 CDN 送出，登入相關區塊在同一個 response 串流，不再有 2.5 秒 gate 與「檢查中」狀態。
- website 不再含 Cloudflare 專屬程式；#338 類問題在 Vercel 上以回歸測試守住。

## 撤銷條件

- 開真實付款且不想付 Pro → 重評 C。
- 被 Vercel 的計費或限制卡住（function 執行時間、頻寬）→ 重評 C。
- `oci-a1` 上 hawkr 的資源上限影響交易 bot → 單體搬到另一台 VM。

## 後續

- 計畫：平台遷移（行為不變）→ 開 cacheComponents ＋ DAL ＋ 伺服器初值 → client 四態模型（`unknown | guest | signedIn | unavailable`）。
- 單體在 `oci-a1` 的容器資源上限與部署方式，在第一次部署前決定。

## 相關

- `2026-09-24-go-modular-monolith.md`（修訂「部署目標」）
- `2026-10-30-identity-self-hosted-federated-login.md`
- `website/next.config.ts`、`website/open-next.config.ts`（#338 的註解）
