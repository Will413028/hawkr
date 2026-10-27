# Hawkr 三頁分拆規劃

> 將現有混合式首頁拆成 **首頁 / 關於我們 / 招商** 三頁，並訂定深淺模式覆蓋規範。
> 制定日期：2026-05-26

**狀態：Historical planning snapshot。** 現行品牌與視覺依 [root DESIGN.md](../DESIGN.md)；本檔中的頁面構想、認證、數字與服務承諾均不是已驗證的產品真值，不得直接轉成對外文案。

---

## 一、現況快照（首頁裝太多）

[`features/home/HomePage.tsx`](../features/home/HomePage.tsx) 共 9 個區塊，混合了**商業導購 + 品牌敘事 + 招商**三種任務：

| # | 區塊 (行) | 目前任務 | 應該歸屬 |
|---|---|---|---|
| 1 | Hero (452-502) | 搜尋 + 雙 CTA | 🏠 首頁 |
| 2 | Categories (504-538) | 6 類別導覽 | 🏠 首頁 |
| 3 | This Week's New (540-566) | 商品輪播 | 🏠 首頁 |
| 4 | Merchants (568-592) | 精選賣家 4 卡 | 🏠 首頁（精簡） |
| 5 | Demo / How It Works (594-677) | 3 步驟手機 mockup | ℹ️ 關於我們 |
| 6 | Story Trail (679-768) | 認證 / 保護 / 在地敘事 | ℹ️ 關於我們 |
| 7 | Daily Updates (770-786) | Feed 3 篇 | 🏠 首頁 |
| 8 | `HomeProcessCards` (788-789) | 物流 / 履約故事 | ℹ️ 關於我們 |
| 9 | Seller CTA (791-819) | 招商深色帶 | 💼 招商頁（首頁留精簡條） |

---

## 二、🏠 首頁（commerce-first，加重商業元素）

**目標：訪客 30 秒內看到「想買 → 怎麼買 → 為什麼現在買」**

### 保留
- Hero（改寫為主打**今日特價 / 季節商品**，弱化敘事）
- Categories 6 類別
- This Week's New 商品輪播
- Merchants 4 卡（保留，當社群信任）
- Daily Updates Feed
- 底部 **招商精簡條**（一行 + 按鈕 → `/sellers-recruit`），長版搬走

### 新增商業元素（建議優先順序）
1. **頂部促銷 strip**（黑五式 announcement bar）— 滿 NT$899 免運 / 新客券碼，全站常駐
2. **限時優惠專區** — 倒數計時 + 折扣商品 3-4 卡（Hero 下方第一屏外）
3. **熱銷排行榜** — Top 10 商品 / Top 5 賣家，左右兩欄
4. **季節推薦** — 「五月當令」帶圖文卡 + 跳轉 `/market/products?season=may`
5. **新客領券 CTA** — 中段 banner，未登入顯示「註冊送 NT$100」
6. **真實購買 UGC** — Instagram-style 4-6 張顧客曬單照（接 `/feed` 內 `type=review` 的貼文）
7. **電子報訂閱**（footer 上方）— 每週嚴選 + 折扣碼

### 移除（搬到別頁）
- Demo / How It Works（→ 關於我們）
- Story Trail 3 stops（→ 關於我們）
- HomeProcessCards 全版（→ 關於我們，首頁留 3 icon 摘要）
- Seller CTA 深色全版（→ 招商頁，首頁留 1 行 strip）

---

## 三、ℹ️ 關於我們 `/about`

**新檔案：** `app/(public)/about/page.tsx` + `features/about/AboutPage.tsx`

### 區塊建議（由上到下）
1. **品牌 Hero** — 「為什麼有 Hawkr」一句話 + 創辦故事 30 字
2. **使命與承諾**（3 卡）— 從 Story Trail 搬：產地直送 / SGS 認證 / 在地小農
3. **如何運作** — 搬 `HowItWorksSection` / `HomePage.tsx` 594-677 的 3 步驟手機 mockup
4. **供應鏈與履約**（完整版）— 搬 `HomeProcessCards`，補上冷鏈、包材、退換貨流程
5. **品質保證 & 認證 logo wall** — SGS、產銷履歷、有機標章
6. **數字成績單** — 累計農場數 / 出貨單數 / 滿意度（社證型）
7. **永續承諾** — 1 段文字 + 圖
8. **聯絡 / FAQ 入口** — links to `/contact`、`/faq`

→ 內容大量可從 [`lib/content/site-content.ts`](../lib/content/site-content.ts) 與既有 Story Trail 剪貼，**零新文案啟動成本**。

---

## 四、💼 招商 `/sellers-recruit`

**現況：** 已有 [`features/seller/landing/SellerLandingPage.tsx`](../features/seller/landing/SellerLandingPage.tsx) 在 `/seller`。

**做法二選一：**
- **A. 沿用 `/seller`**，只增強內容（最小變更）✅ 推薦
- B. 新建 `/sellers-recruit` 並 301 → `/seller`

### 在現有基礎上補強
現有：Hero / 3 卡 benefits / Storefront examples 4 卡

**新增：**
1. **平台規模與成績**（社證）— 累計賣家數 / 月活買家 / 平均客單
2. **抽成與收費透明表** — 一張清楚的費率表（轉化關鍵）
3. **上架流程 4-step** — 申請 → 審核 → 上架 → 出貨
4. **賣家案例專訪**（深度版）— 1-2 位賣家短訪談 + 數字成長
5. **入駐 FAQ**（5-8 題）— 用既有 FAQ 結構
6. **CTA：申請入駐**（表單或預約諮詢）— 頁尾 sticky 或 jumbo button
7. **合作品牌 logo wall**（如果有）

---

## 五、訪客 Navbar 更新

[`lib/content/data.ts:837-880`](../lib/content/data.ts#L837-L880) 目前只有 3 項。

### 建議改版（5 項，搜尋移右側 icon）
```
Hawkr 市集  Hawkr 動態  關於我們  招商合作   |   🔍  ☀️  📱  🛒  登入
/market    /feed    /about    /sellers-recruit
```

- **搜尋** 從主 nav 拉到右側 icon（省空間，符合電商慣例）
- **關於我們** 簡單 link，無 mega menu
- **招商合作** 簡單 link，無 mega menu（賣家進入時可加 badge「招募中」）

**程式變更點：** `lib/content/data.ts` 的 `navItems` 陣列 + [`components/shared/Navbar.tsx`](../components/shared/Navbar.tsx) 右側區把 search 從 mega menu 改成 IconButton。

---

## 六、Footer 更新

[`components/shared/Footer.tsx`](../components/shared/Footer.tsx) Quick Links 欄目：

### Before
```
首頁 / Hawkr 動態 / Hawkr 市集 / 搜尋 / 成為賣家
```

### After
```
首頁 / Hawkr 市集 / Hawkr 動態 / 搜尋
關於我們 ← 新增
招商合作 ← 重命名自「成為賣家」（連到 /sellers-recruit 或保留 /seller）
```

或拆兩欄：「**探索**」(市集/動態/搜尋) +「**關於 Hawkr**」(關於我們/招商合作/聯絡我們)。

---

## 七、路由與檔案佈局

```
app/(public)/
├── about/page.tsx                 ← 新增，import AboutPage
├── sellers-recruit/page.tsx       ← 可選；或保留 /seller
└── ...

features/
├── about/                         ← 新資料夾
│   ├── AboutPage.tsx
│   └── components/
│       ├── BrandStorySection.tsx       ← 從 HomePage 搬
│       ├── HowItWorksSection.tsx       ← 已存在，啟用
│       └── SupplyChainSection.tsx      ← 從 HomeProcessCards 搬
├── home/HomePage.tsx              ← 精簡 + 加商業區塊
└── seller/landing/SellerLandingPage.tsx ← 補強
```

**Route group：** 三頁全放 `(public)`，跟 `/`、`/feed`、`/market` 同層級。

---

# 深淺模式覆蓋規範

## 一、為什麼會出現對比 drift（先講清楚原因）

目前已知問題來源：
1. **手寫 `dark:` overrides** 跟 token cascade 不同步 → 改了 token 沒翻譯到 dark
2. **arbitrary `var()`**（如 `bg-[var(--hkui-content1)]`）→ 跳過 `@theme inline`，dark 不會自動翻
3. **brand-invariant token 誤用**（`hk-primary-strong/hk-brand-secondary/hk-cream`）當一般卡片底 → dark 下變死黑或刺眼
4. **疊太多半透明**（玻璃 + 漸層 + 文字 opacity）→ light 看起來剛好、dark 變模糊

→ 規範核心：**所有「會翻色的角色」一律用 `@theme inline` 的語意 utility，禁止 `dark:` 重述同值，禁止 arbitrary `var()`。**

---

## 二、Token Ladder（三頁統一）

### 表面層級（由深到淺）
| 角色 | utility | light | dark | 用在 |
|---|---|---|---|---|
| 頁面底 | `bg-background` | 米白 | 深森綠 | `<body>` 已預設 |
| 主要卡片 | `bg-hk-surface-card` | 白 | ink-glass 半透 | 商品卡、賣家卡 |
| 次要區塊 | `bg-hk-surface-muted` | 米黃 | 深一階 | banner 帶狀區 |
| 玻璃面板 | `bg-hk-surface-glass` | 米色+ink tint | 白 veil | 側欄、控件、tab |
| Hover | `bg-hk-surface-glass-hover` | – | – | 互動 hover |
| 點擊 | `bg-hk-surface-glass-active` | – | – | – |

### 文字層級（4 級對比）
| 角色 | utility | 用在 | WCAG |
|---|---|---|---|
| 標題 | `text-hk-heading` | h1-h3 | AAA |
| 內文 | `text-hk-body` | p、cards | AA |
| 次要 | `text-hk-muted` | meta、caption | AA |
| 最淡 | `text-hk-faint` | disabled、placeholder | A |
| 強調 | `text-hk-accent` | active tab、selected label | – |
| 品牌 | `text-hk-primary` | CTA 文字、key number | – |

### Hairline 3 級
`border-hk-border-subtle` < `border-hk-border-soft`（預設）< `border-hk-border-strong`

### Active / Selected
底：`bg-hk-primary-subtle`　框：`border-hk-primary-border`（dark = 亮金）

### 狀態色（brand-invariant，不翻）
`text-danger/success/warning-600`、`text-hk-success/warning/danger`

---

## 三、三頁 Surface 對應表

### 🏠 首頁
| 區塊 | 容器 surface | 文字 | dark 注意 |
|---|---|---|---|
| Announcement strip 促銷帶 | `bg-hk-primary-subtle` + `text-hk-primary` | accent | dark 自動變淡金底，✅ |
| Hero | `bg-background` + 漸層裝飾 | heading + body | 漸層要寫雙色 `from-/dark:from-`（這是設計差異不是同值翻譯，需要 `dark:`） |
| Categories grid | `bg-hk-surface-card` 卡 + `border-hk-border-soft` | heading + muted | – |
| 限時優惠 | `bg-hk-surface-muted` 區塊底 + `bg-hk-surface-card` 卡 | – | 倒數計時數字用 `text-hk-primary` |
| 熱銷榜 | `bg-hk-surface-card` | – | 排名數字用 `text-hk-accent` |
| 新客券 banner | `bg-hk-primary-subtle` + `border-hk-primary-border` | `text-hk-primary` | dark 是亮金，要避開飽和金色背景，✅ |
| UGC grid | `bg-hk-surface-card` 卡 | – | 圖片底寫 `bg-default-100` 防 placeholder 撞背景 |
| Daily Updates | `bg-hk-surface-card` | – | – |
| 招商精簡條 | `bg-hk-surface-muted` 或 primary-strong 實心 | – | 用 primary-strong 實心要明確用 `bg-hk-primary-strong` + `text-hk-cream/90`（brand-invariant，刻意不翻） |
| 電子報 | `bg-hk-surface-glass` | – | – |

### ℹ️ 關於我們
| 區塊 | 容器 | 注意 |
|---|---|---|
| Brand hero | `bg-background` + 大圖 | 圖片上的文字一律加 `bg-gradient-to-t from-black/60` 保證對比，**不需 dark:**（雙模都需要） |
| 使命 3 卡 | `bg-hk-surface-card` + `border-hk-border-soft` | – |
| How it Works | `bg-hk-surface-muted` 底 + `bg-hk-surface-card` 步驟卡 | 手機 mockup 邊框 `border-hk-border-strong` |
| 供應鏈流程 | `bg-hk-surface-glass` 玻璃帶 | dark 自動白 veil，✅ |
| 認證 logo wall | `bg-hk-surface-card` + grayscale filter | dark 下 grayscale 反差小 → 改 `grayscale dark:grayscale-0 dark:opacity-80` |
| 數字成績單 | `bg-hk-primary-subtle` 大區 | 數字 `text-hk-primary`，標籤 `text-hk-muted` |
| 永續承諾 | `bg-hk-surface-muted` | – |

### 💼 招商
| 區塊 | 容器 | 注意 |
|---|---|---|
| Hero | primary-strong 實心 `bg-hk-primary-strong` + `text-hk-cream` | brand-invariant，雙模同色，✅ |
| 平台優勢 3 卡 | `bg-hk-surface-card` | 數字 `text-hk-primary` |
| 收費透明表 | `bg-hk-surface-card` 表格 + `border-hk-border-soft` row 線 | 強調 row 用 `bg-hk-primary-subtle` |
| 上架流程 4 step | `bg-hk-surface-muted` 底 + 步驟圓 `bg-hk-primary-subtle` | 連線用 `border-hk-border-subtle` dashed |
| 賣家案例 | `bg-hk-surface-card` 大卡 + 引言 `text-hk-body italic` | 頭像底 `bg-default-100` |
| FAQ 手風琴 | `bg-hk-surface-card` + 展開後 `bg-hk-surface-muted` | – |
| 申請 CTA sticky | `bg-hk-surface-glass` 玻璃條 + `border-hk-border-strong` 上邊線 | dark 下白 veil 半透，✅ |

---

## 四、禁止清單（PR review checklist）

審 PR 時直接抓這 6 條：

1. ❌ 寫 arbitrary value 直拉 `--hkui-…` / `--theme-…` token（例如 bg-[var(--hkui-content1)]）— 改用 `bg-content1` / `text-hk-body`
2. ❌ `dark:text-white` / `dark:bg-zinc-900` 之類**翻譯同一個語意值** — 改用會 cascade 的 token
3. ❌ `bg-hk-primary-strong` / `bg-hk-brand-secondary` 拿來當一般卡底 — 這是 brand-invariant，dark 下不翻會死黑/刺金
4. ❌ 文字 `opacity-50` 疊在玻璃面上 — 改用 `text-hk-muted` 或 `text-hk-faint`
5. ❌ 圖片上的文字沒加 scrim — 一律 `bg-gradient-to-t from-black/60 via-black/20`
6. ❌ 漸層只寫一組 — Hero / banner 漸層必須提供 `dark:from-* dark:to-*` 對應（這屬於「換設計」不是「翻色」）

---

## 五、驗收方法（每個 PR）

```bash
pnpm dev
```

**手動 6 點檢查**（每頁都跑）：
1. 切到 dark（navbar 太陽 icon）→ 滾完整頁，不能有純白色塊未翻譯
2. 切到 light → 不能有 dark-only 殘留（如深灰底沒消失）
3. Chrome DevTools → Lighthouse → Accessibility，contrast 不能有 warning
4. 找一個 brand 漸層區（hero）→ dark 下文字要可讀（眼睛瞇 3 秒測試）
5. 找一個玻璃面（rail / sticky CTA）→ dark 下不能霧到看不清文字
6. 開 `Cmd+F` 找 `dark:` 出現次數，跟 PR 前比對 → 沒理由的增加要 reject

**自動檢查（一次性建議加）**：
```bash
# grep 出規範違反
rg "bg-\[var\(--(hkui|theme)" features/{home,about,seller}
rg "dark:(bg|text)-(white|black|zinc|slate|gray)" features/{home,about,seller}
```
這 2 條 grep 應該都是**零命中**。可寫成 pre-commit hook。

---

## 六、Hero / Banner 的特殊處理（最容易出問題）

Hero、招商深色帶、首頁促銷 banner 這類**裝飾性視覺**通常需要 `dark:` 換配色，不是 cascade 能解。規範：

```tsx
// ✅ 推薦寫法
<section className="
  bg-gradient-to-br
  from-hk-cream to-white
  dark:from-[#1a2620] dark:to-[#0f1814]
">
```

- 漸層色出現 `dark:` **正當**，因為設計刻意換配色
- 但**文字** still 用 `text-hk-heading` / `text-hk-body`（cascade 翻譯）
- **不要**寫 `dark:text-white` —— 讓 token 處理

→ 把這類 hero 整理成 3-4 個共用 wrapper class（如 `.hk-hero-warm`、`.hk-hero-primary`），放 `styles/components/hero.css`，避免每頁複製貼上漸層配色 drift。

---

# 實施順序（5 PR）

| PR | 內容 | 暗色驗收 |
|---|---|---|
| PR1 | 新增 `/about` + 補強 `/seller` 殼 | 兩頁 dark/light 6 點檢查 |
| PR2 | Nav + Footer 加連結 | 新項目 hover state dark/light |
| PR3 | 首頁瘦身 | 確認移除區塊沒留下孤立 `dark:` 樣式 |
| PR4 | 首頁加商業元素 | 每個新區塊獨立 dark/light 截圖比對 |
| PR5 | 共用 hero/banner wrapper 抽出 + grep 規則加 pre-commit | dark 模式 regression test |

這順序確保**任何時間點上線都不會有破損頁面**，且每個 PR 可獨立 review。
