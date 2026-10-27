---
name: "Hawkr"
description: "溫暖、可信，讓人更容易看懂來源與交易狀態的日常食品市集。"
colors:
  brand-primary: "#e4572e"
  brand-primary-strong: "#b8441f"
  brand-primary-deep: "#8f3315"
  brand-secondary: "#f2a541"
  brand-ink: "#1f2933"
  brand-ink-on-dark: "#f5f7fa"
  brand-primary-soft: "#fcebe6"
  brand-secondary-soft: "#fdf1e1"
  status-success: "#2d7f50"
  status-warning: "#e0c44c"
  status-danger: "#ea6021"
  status-info: "#6194d0"
  status-info-strong: "#1e5fab"
  status-info-soft: "#9fccdd"
  surface-page: "#f8f6ef"
  surface-card: "#fffdf7"
  surface-muted: "#f5f1e8"
  surface-panel: "#f2e9d6"
  surface-control: "#f0f1ea"
  ink-heading: "#191c18"
  ink-body: "#42493f"
  ink-muted: "#72796e"
  dark-surface-page: "#1f2933"
  dark-ink-heading: "#f5f7fa"
  dark-ink-body: "rgb(245 247 250 / 0.82)"
  white: "#ffffff"
  action-primary-hover: "#8f3315"
  ui-primary-hover: "#a43c1a"
typography:
  display:
    fontFamily: "var(--font-openhuninn), 'Hiragino Maru Gothic ProN', 'Yuanti TC', 'Yuanti SC', var(--font-primary)"
    fontSize: "48px"
    fontWeight: 700
    lineHeight: 1.2
  headline:
    fontFamily: "var(--font-openhuninn), 'Hiragino Maru Gothic ProN', 'Yuanti TC', 'Yuanti SC', var(--font-primary)"
    fontSize: "36px"
    fontWeight: 700
    lineHeight: 1.2
  title:
    fontFamily: "var(--font-roboto), system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang TC', 'Noto Sans TC', 'Microsoft JhengHei', 'Heiti TC', sans-serif"
    fontSize: "24px"
    fontWeight: 700
    lineHeight: 1.2
  body:
    fontFamily: "var(--font-roboto), system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang TC', 'Noto Sans TC', 'Microsoft JhengHei', 'Heiti TC', sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.5
  input-body:
    fontFamily: "var(--font-roboto), system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang TC', 'Noto Sans TC', 'Microsoft JhengHei', 'Heiti TC', sans-serif"
    fontSize: "16px"
    fontWeight: 400
    lineHeight: "24px"
  label:
    fontFamily: "var(--font-roboto), system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang TC', 'Noto Sans TC', 'Microsoft JhengHei', 'Heiti TC', sans-serif"
    fontSize: "14px"
    fontWeight: 600
    lineHeight: "20px"
rounded:
  foundation-xs: "6px"
  foundation-sm: "8px"
  foundation-md: "12px"
  foundation-lg: "16px"
  foundation-xl: "24px"
  foundation-2xl: "32px"
  ui-sm: "8px"
  ui-md: "12px"
  ui-lg: "14px"
  ui-xl: "20px"
  ui-2xl: "28px"
  ui-3xl: "36px"
  ui-4xl: "45px"
  pill: "999px"
  full: "9999px"
  circle: "50%"
spacing:
  ui-0_5: "2px"
  space-1: "4px"
  ui-1_5: "6px"
  space-2: "8px"
  ui-2_5: "10px"
  space-3: "12px"
  ui-3_5: "14px"
  space-4: "16px"
  space-5: "20px"
  space-6: "24px"
  ui-7: "28px"
  space-8: "32px"
  space-10: "40px"
  space-12: "48px"
  ui-14: "56px"
  space-16: "64px"
  space-20: "80px"
  space-24: "96px"
components:
  button-primary:
    backgroundColor: "{colors.brand-primary-strong}"
    textColor: "{colors.white}"
    typography: "{typography.label}"
    rounded: "{rounded.ui-md}"
    padding: "0 16px"
    height: "40px"
  button-primary-hover:
    backgroundColor: "{colors.ui-primary-hover}"
    textColor: "{colors.white}"
    typography: "{typography.label}"
    rounded: "{rounded.ui-md}"
    padding: "0 16px"
    height: "40px"
  button-secondary:
    backgroundColor: "transparent"
    textColor: "{colors.brand-primary-strong}"
    typography: "{typography.label}"
    rounded: "{rounded.ui-md}"
    padding: "0 16px"
    height: "40px"
  card-default:
    backgroundColor: "{colors.surface-card}"
    textColor: "{colors.ink-heading}"
    typography: "{typography.body}"
    rounded: "{rounded.ui-lg}"
  input-default:
    backgroundColor: "{colors.surface-card}"
    textColor: "{colors.ink-body}"
    typography: "{typography.input-body}"
    rounded: "{rounded.ui-md}"
    padding: "0 16px"
    height: "48px"
  dialog-default:
    backgroundColor: "{colors.surface-card}"
    textColor: "{colors.ink-heading}"
    typography: "{typography.body}"
    rounded: "{rounded.ui-2xl}"
    width: "512px"
---

# Design System: Hawkr

## Overview

**Creative North Star: "深入職人故事的市集"**

「深入職人故事的市集」是品牌 owner 已確認的現行北極星，不等同對外 slogan。它要求品牌與商品內容讓人理解誰在製作、為何這樣做，以及產品有何具體差異；交易頁仍以規格、價格、溫層、配送與狀態清楚為優先，不為了敘事犧牲操作效率。

Hawkr 不是冷調科技平台，也不是只做氛圍的農村風形象站。現行視覺要讓人感到溫暖、可信、日常可親，同時把商品來源、賣家身分、規格、溫層、配送與交易狀態放在裝飾之前。食品與品牌內容是主角，介面負責把它們說清楚。

同一套品牌有四種介面任務：品牌首頁、關於頁與招商頁負責說服（Persuade）；市集、搜尋、購物車、結帳、帳戶、聊天室與賣家中心負責操作（Operate）；FAQ、政策與指南負責閱讀（Read）；少數核准的單頁敘事負責體驗（Experience）。四種模式共用品牌骨架，但資訊密度、構圖與動態強度不同，不應被壓成同一種寬鬆落地頁版型。`/seller` 的 scroll world 屬 route-owned 體驗，不是全站模板。

### Scope and source authority

本檔是設計、工程與 AI 工具共用的現行品牌入口，描述「應該如何使用」既有系統；實際渲染仍由程式碼決定。判讀衝突時依下列順序查證：

1. `styles/foundation/00-color-config.css`：品牌色、light/dark semantic roles 與少數明確 route tokens。
2. `styles/foundation/01-tokens-theme.css`：字體、尺寸、間距、圓角、陰影、層級、動態與相容 alias。
3. `styles/foundation/02-reset-layout.css`：全域排版、頁面 canvas 與互動基線。
4. `styles/globals.css` 的 Tailwind bridge：把 CSS variables 暴露給 utility layer。
5. `styles/components/ui-kit.css`、`components/ui/*` 與 `components/shared/*`：共享元件的 production contract。
6. route／feature CSS：只可表達該任務的密度、構圖與已核准例外，不得重寫品牌 foundation。
7. 經核准的 surface brief：補充單一頁面的設計意圖，不可暗中取代全站規則。

`.impeccable/design.json` 是本檔的機器可讀 sidecar，不是另一個 runtime token source；每次修改本檔都必須同步產生。若文件與程式不一致，標示為 current divergence 並建立收斂方向，不可自行補造第三套標準。歷史文件只能提供背景，不能覆寫本檔與 production source。

### Surface modes

| Mode | 主要任務 | 密度與構圖 | 品牌表現 |
| --- | --- | --- | --- |
| **Persuade** | 首頁、關於、招商 | 較多留白、明確敘事節奏、少量高品質 CTA | 真實職人、商品與產地證據可主導畫面 |
| **Operate** | 市集、搜尋、結帳、帳戶、訊息、賣家中心 | 資訊密、狀態鄰近、工具列與列表優先 | 品牌退到 token、字體與微互動，不搶任務 |
| **Read** | FAQ、政策、指南 | 受控行寬、清楚目錄、長文掃讀 | 圓體只作章節入口，正文保持克制 |
| **Experience** | 核准的單頁敘事 | 可有場景式構圖與較強動態，但必須有靜態／reduced-motion 結果 | 只屬該 route；不可抽象成全站元件 |

**Key Characteristics:**

- 暖白紙感實色表面，搭配 Hawkr 的橘紅與暖橘黃。
- 真實職人、商品、食品影像與商家資料優先；故事要連回可核對的來源與商品資訊。
- 品牌頁有呼吸感，交易與後台操作頁保持有效率的資訊密度。
- 圓潤但有角色分工；不是每個容器都做成大圓角卡片。
- 動態只服務狀態、導引或敘事，不做全站環境裝飾。

**Truth Before Decoration.** 來源、賣家、規格、價格、溫層、配送、庫存與交易狀態是第一層內容。故事與情緒只能建立在可核對資訊上；缺資料時清楚標示「尚未提供」，不可用裝飾、虛構數字或泛用承諾補空白。

**Global / Route Boundary.** 全站規則負責品牌與互動一致性；route 只負責任務所需的編排。任何新 route token 都要有明確名稱、用途與移除條件，不得因一頁好看就升格成 foundation。

**Implementation Parity.** 文件描述的是目標契約，不等於所有舊畫面已符合。遇到本檔標示的 current divergence，先重用最接近的 production primitive，再以獨立修正工作收斂；不要在新頁複製債務。

## Colors

主色是 Hawkr 的橘紅（logo 原色），互動元件改用加深一階的磚橘；輔色是暖橘黃，文字色是墨藍灰。暖白、奶油與低彩度灰米構成日常市集的底；綠色、黃色、橘紅與藍色只在狀態、溫層或有明確語意的內容中出現。

### Primary

- **Hawkr Orange / 品牌橘** (`#e4572e`)：logo、大標題（24px 以上，或 18.66px 以上粗體）與裝飾；白字對比只有 3.68:1，不配一般尺寸的白字。
- **Hawkr Brick / 磚橘** (`#b8441f`)：互動元件——主按鈕底、連結、選取態與 focus；白字 5.41:1。
- **Deep Brick / 深磚橘** (`#8f3315`)：互動元件的 hover 與 pressed；白字 7.92:1。

### Secondary

- **Hawkr Amber / 暖橘黃** (`#f2a541`)：輔色底與少量溫暖強調，配墨色文字（7.19:1），不配白字，不是每顆 CTA 的預設色。
- **Hawkr Ink / 墨藍灰** (`#1f2933`)：Hawkr 的文字色與 logo 檯面。
- **Ink On Dark / 深底墨白** (`#f5f7fa`)：深色底上的文字與 logo 檯面。
- **Primary Soft / 橘紅淡階** (`#fcebe6`)、**Secondary Soft / 橘黃淡階** (`#fdf1e1`)：primary 12%、secondary 16% 與白色混合的淺色階，配墨色文字不低於 4.5:1；用於淺底、標籤與季節裝飾。

### Status

狀態色是通用色，不屬於品牌色票，值固定。

- **Success / 成功** (`#2d7f50`)：正向狀態、已完成或可上架。
- **Warning / 警示** (`#e0c44c`)：warning 與活動提示。
- **Danger / 危險** (`#ea6021`)：danger、error 輔助或高強度促銷，不宜大面積使用。
- **Info / 資訊** (`#6194d0`)：active info、配送進度與 frozen 溫層；**Info Strong** (`#1e5fab`) 用於需要較深對比的物流與資訊狀態，**Info Soft** (`#9fccdd`) 用於淡色資訊底。

### Neutral

- **Warm Page / 暖白頁面** (`#f8f6ef`)：全站 light mode 的頁面底色。
- **Cream Card / 奶油卡面** (`#fffdf7`)：主要卡片與浮起表面。
- **Muted Paper / 柔米紙面** (`#f5f1e8`)：低對比區塊與次要表面。
- **Sand Panel / 沙色面板** (`#f2e9d6`)：需要較明確區隔的面板。
- **Soft Control / 柔和控制面** (`#f0f1ea`)：flat input 與低強度控制面。
- **Living Ink / 生命墨色** (`#191c18`)：標題與最高對比資訊。
- **Warm Charcoal / 暖炭灰** (`#42493f`)：正文；不以純黑作為預設。
- **Quiet Gray / 靜灰** (`#72796e`)：次要資訊與輔助文字。

### Dark Mode

- **Night Surface / 深夜頁面** (`#1f2933`)：dark mode 頁面底色；card、muted、panel、elevated 是由深到淺的中性藍灰階（`#28333f`、`#242f3a`、`#2b3642`、`#2f3b47`）。
- **Ink On Dark / 深底標題** (`#f5f7fa`)：dark mode 標題。
- **Soft Light Ink / 柔亮正文** (`rgb(245 247 250 / 0.82)`)：dark mode 正文。
- 主要 theme action 使用暖橘黃 (`#f2a541`)，配墨色文字；hover 為暖橘黃混入 18% 白色。`brand-primary` 與 `brand-primary-strong` 在暗色底上不當一般文字用。
- dark mode 仍靠色階與邊界建立層次，不把整站改成純黑、霓虹或全玻璃介面。

### Semantic surface ladder

畫面應消費 semantic role，而不是直接依色票名稱猜用途。下表是 production foundation 的層次關係；實際值以 `00-color-config.css` 為準。

| Role | Light | Dark | 用途 |
| --- | --- | --- | --- |
| `surface-page` | 暖白 `#f8f6ef` | 深藍灰 `#1f2933` | 唯一頁面 canvas |
| `surface-card` | 奶油白 `#fffdf7` | 中性深色卡面 `#28333f` | 一般內容與商品卡 |
| `surface-muted` | 柔米 `#f5f1e8` | 中性深色 `#242f3a` | 次要區塊、row hover |
| `surface-panel` | 沙色 `#f2e9d6` | 中性深色面板 `#2b3642` | 工具區、分組面板 |
| `surface-elevated` | 近白實色 | 較亮中性深色 `#2f3b47` | dropdown、popover、dialog |
| `surface-glass` | 高透明暖白 | 受控深綠玻璃 | 導覽、overlay 上方表面；不是卡片預設 |

文字同樣按 `heading → body → muted → disabled` 遞減，邊界按 `subtle → default → strong → focus` 遞增。不要用 disabled 灰表示一般輔助文字，也不要只靠 surface 顏色差異表達 selected、error 或交易狀態。

### Theme mechanics

- 根節點以 `data-theme` 切換；目前預設 light、未跟隨 system，選擇儲存在 `hk:theme`。
- boot script 會在 hydration 前上色，避免首屏閃爍；新 theme UI 不得繞過這條生命週期。
- dark mode 是靜態的多層森林夜色 canvas，搭配受控的星點／光暈背景與半透明表面；它不是純黑色板，也不是每層都 blur 的 glassmorphism。
- 同一個畫面只能有一個 page canvas。route section 應使用 transparent 或 semantic surface 疊層，不可再鋪一張競爭的 full-page background。

### Route-owned palettes

- 首頁與相鄰品牌頁有暖色 route palette，而且現況已有 dark adaptation；它不是 light-only，也不是第二套全站品牌色。
- chat、map 與部分賣家工作區有任務專用 token，只能留在其 feature scope。
- 賣家中心的高密度操作表面、`/seller` 的 scroll world 與單頁活動色盤都屬 route exception；若抽離原任務就失去意義，不得複製到一般卡片或 CTA。

### Contrast and state

- 正文、按鈕、欄位與必要狀態須達 WCAG AA；一般文字至少 4.5:1，大字至少 3:1。邊界或非文字控制元件的必要視覺差異至少 3:1。
- success、warning、danger、info 與溫層不能只靠色相；同時提供文字、圖示、形狀或位置線索。
- hover、selected、focus、disabled、loading 與 error 必須可彼此區分；focus ring 不能被陰影或 overflow 裁掉。
- 漸層只可服務有方向性的品牌光線、遮罩或狀態過渡；不能把兩個品牌色混成沒有語意的 CTA。

**The One Brand Entry Rule.** 新增顏色先接到 `00-color-config.css` 的品牌或 semantic role，再由元件消費；頁面不可另建一套相近但不同的品牌色。

**The Meaning Before Accent Rule.** 暖色與藍色只有在能說出狀態意義時才使用，不能為了「更活潑」平均灑在每個區塊。

**Current divergence.** `--theme-action-primary` 在 dark mode 轉為暖橘黃，但共享 `Button` 使用的 `--hkui-primary` 目前仍維持磚橘。兩者不是同一個 dark-mode contract；新元件優先重用共享 `Button`，待專案另行收斂。

## Typography

**Display Font:** jf-openhuninn，接圓體系統字與 UI 字體 fallback。  
**Body Font:** self-hosted Roboto（Latin），接系統繁中字體。  
**Label Font:** 與 body 相同，以字重而非另一套字型區分。

**Character:** 大標題帶一點圓潤與人情味，正文則保持清楚、克制、可長時間閱讀。品牌感集中在 `h1`、`h2` 與少數行銷標題；資料表、表單、價格與操作標籤留在 UI 字體。

### Hierarchy

- **Display** (700, 48px, 1.2): 只用於頁面主標與少量品牌場景；手機版依版面縮放，不以任意超大字製造存在感。
- **Headline** (700, 36px, 1.2): 重要品牌章節與較大型頁面標題；共享 Dialog 標題使用 24px component scale，不套用本層。句子要短，避免每節都套同樣置中格式。
- **Title** (700, 24px, 1.2): 卡片、功能模組與工作區標題，優先清楚而非裝飾。
- **Body** (400, 14px, 1.5): 預設正文與 UI 資訊；長文控制行寬與段落節奏。
- **Label** (600, 14px, 20px line-height): 按鈕、狀態與導覽；欄位標籤在共享 `Input` 是 500，不要為了醒目一律加粗。

### Production type scales

foundation 與 UI kit 都在運作，使用時先看任務而不是自行插值：

| Scale | 尺寸 | 建議用途 |
| --- | --- | --- |
| UI tiny / small | 12 / 14px | caption、metadata、密集控制資訊；12px 不作長正文 |
| UI medium / large | 16 / 18px | 一般輸入、重要正文、卡片小標 |
| Display small / medium | 24 / 30px | 工作區主標、dialog 標題、品牌小節 |
| Display large / x-large / 2x-large | 36 / 48 / 60px | 品牌敘事主標；60px 僅限有空間的 desktop hero |

字重角色以 400 正文、500 欄位與次要強調、600 操作／狀態、700 標題與關鍵數值為主。900 已存在於 foundation，但只可用於少數核准的強調，不是一般頁面標題預設。不要在同一層級混用三種粗細，也不要用全大寫英文字與寬字距冒充品牌。

### Content typography

- 繁體中文標點與語句必須自然；標題在語意斷點換行，不以固定 `<br>` 綁死所有 viewport。
- 長文採可讀行寬與 1.5–1.75 行高，段落靠 spacing 分節，不用連續卡片切碎政策或指南。
- 價格、數量、百分比、時間與表格欄位使用 `font-variant-numeric: tabular-nums`；貨幣符號、單位與值不可拆到兩行。
- truncation 只用在可從上下文或 detail view 取回全文的地方；錯誤、狀態、價格與必要規格不可無提示截斷。
- `Zen Maru Gothic` 目前只存在於特定 `.hkh-page` route fallback，不是全站第三套品牌字體。

**The Brand Headline Rule.** 圓體負責品牌標題，Roboto 加系統繁中字體負責操作與閱讀；不要再引入 Inter 或另一套網頁中文字型來製造「設計感」。

**The Information Weight Rule.** 字級與字重應反映任務優先順序：在操作頁，價格、狀態與下一步可高於品牌句；在閱讀頁，標題層級與段落節奏高於裝飾 eyebrow。

## Layout

全站 foundation 以 4px 為基礎節奏，常用間距從 4px 到 96px；共享 UI kit 另保留 2px、6px、10px、14px、28px、56px 等 production microsteps，只供元件內部對齊，不得逐頁自造間距。密度由 surface mode 決定，但相鄰距離仍從既有 scale 選取。

### Canvas, container and grid

- 頁面只有一個 body canvas。全站最大寬度是 1440px，主要內容寬度是 1320px；這兩個值是上限，不是所有 section 必須填滿的寬度。
- foundation 的 global gutter 目前是 0；實際左右 padding 由 route container 負責。新增頁面要沿用相鄰 route 的 container，不能誤認為全站已自動提供安全邊距。
- 品牌頁可用較寬 section 與較大 vertical rhythm；操作頁優先保留 toolbar、filter、表單、列表與鄰近狀態。Read mode 應使用較窄文字欄，不把 1320px 當文章行寬。
- Grid 先依內容最小可用寬度自適應，再決定欄數。商品卡、統計卡與表單欄位不為了對稱而擠到難以閱讀。

### Responsive chrome

breakpoint 是現有元件的責任邊界，不是一套可任意套用的裝置名稱：

| Gate | 現行責任 |
| --- | --- |
| 1280px | public navigation 進入較緊湊配置 |
| 840px（inclusive） | desktop top-navigation 收合 |
| `not (min-width: 840px)` | 五欄浮動 mobile bottom navigation 顯示 |
| 768px | public top bar 轉為 56px 平面列 |

560px、640px、881px、900px、960px 等值目前是 feature／route 的局部重排點，不是 foundation breakpoint scale。要新增 gate，先證明內容在哪個寬度失效；不要只因常見裝置尺寸再加一個 media query。

桌面 public navigation 是 84px 浮起 pill，與頁面頂端保留 16px；mobile top bar 是 56px。840px 以下另有 68px 高、距底 10px 的五欄浮動 bottom navigation，內容需預留空間。賣家中心、訊息 thread 與獨立流程可隱藏 bottom navigation；這些是明確 route exception，不應以 CSS 偶然覆蓋。

### Mobile, zoom and hit areas

- 手機優先保留任務順序：主要動作、價格、關鍵資訊、錯誤與狀態不可被裝飾或圖像擠走。品牌敘事可重新排序，操作流程不能只靠 hover 才能理解。
- touch-dominant、icon-only 與關鍵動作的實際命中區至少 44×44px；新工作優先採 48px control，或以透明 padding 擴大 hit area。
- 現有 `Button`、`Tabs` 的 40px md、`ScPill` 32px、Chip close 20px 與 Dialog close 36px 是 current divergence；視覺高度不能被宣稱為已符合 44px 觸控標準。
- 在 200% zoom 與窄 viewport 下，主要內容仍需可到達，不能產生雙向頁面捲動；工具列可換行或轉成 menu，資料表則依其專用規則處理。
- root viewport 使用 `interactiveWidget: resizes-content`，但目前沒有 `viewport-fit: cover` 與全站 safe-area compensation。新 fixed chrome 若要碰觸螢幕邊緣，必須先建立一致的 safe-area contract，不可局部猜測。

**The Mode-Aware Density Rule.** 說服、操作、閱讀、體驗四種頁面共用品牌骨架，不共用同一種 section 高度、置中構圖、卡片數量或動態強度。

**The Content Breakpoint Rule.** 以內容何時讀不清、點不到或失去順序來決定重排，不以手機／平板／桌機名稱決定任意數字。

## Elevation & Depth

現行系統採「實色分層為主、陰影輔助」的混合方式。頁面、卡片與面板先靠暖白色階和低對比邊界建立層次；陰影用於可互動卡片、浮層、選單與對話框，不替代資訊階層。

### Shadow Vocabulary

目前有兩套仍在 production 的陰影 ladder，不能混寫成單一尺度：

| Foundation | 值 | 用途 |
| --- | --- | --- |
| xs | `0 1px 2px rgb(25 28 24 / .05)` | 細微邊緣分離 |
| sm | `0 2px 6px rgb(25 28 24 / .06)` | 小型控制與低浮起 |
| md | `0 12px 30px rgb(31 41 51 / .07), 0 2px 8px rgb(25 28 24 / .04)` | 一般 section／舊 layout |
| lg | `0 18px 42px rgb(31 41 51 / .1), 0 5px 14px rgb(25 28 24 / .06)` | 明確浮層 |
| xl | `0 12px 28px rgb(25 28 24 / .14), 0 4px 10px rgb(25 28 24 / .06)` | 高層 overlay |
| 2xl | `0 24px 60px rgb(25 28 24 / .2), 0 8px 18px rgb(25 28 24 / .08)` | 最大深度，極少使用 |

| Shared UI | 值 | 用途 |
| --- | --- | --- |
| sm | `0 2px 8px rgb(31 41 51 / .06)` | 小型浮起元件與 restrained hover |
| md | `0 4px 20px rgb(31 41 51 / .06), 0 2px 8px rgb(0 0 0 / .04)` | 共享 Card 預設 |
| lg | `0 12px 32px rgb(31 41 51 / .1), 0 4px 12px rgb(0 0 0 / .06)` | interactive card、dropdown |
| deep | `0 8px 32px rgb(31 41 51 / .15)` | 明確離開 canvas 的 surface |
| glass | `0 40px 80px rgb(31 41 51 / .18)` | modal、sheet 等最高共享表面 |

同一元件只選一套來源。新共享元件優先使用 `--hkui-shadow-*`；既有 route 若使用 foundation shadow，應透過 token 維持，不手抄 value。

常用選擇錨點：

- **Foundation md** (`box-shadow: 0 12px 30px rgb(31 41 51 / .07), 0 2px 8px rgb(25 28 24 / .04)`)：既有 foundation section／layout。
- **Shared UI md** (`box-shadow: 0 4px 20px rgb(31 41 51 / .06), 0 2px 8px rgb(0 0 0 / .04)`)：共享 Card 預設。
- **Shared UI glass** (`box-shadow: 0 40px 80px rgb(31 41 51 / .18)`)：真正離開頁面層級的 dialog／sheet。

### Layering and overlays

foundation z-index ladder 為 0、10、100、200、250、300、400、500、600、700，依序對應 base、raised、dropdown、sticky、nav、overlay、modal、popover、tooltip、toast。每個 overlay 應在最小必要 stacking context 內解決，不能用任意四位數互相競價。

- border、tonal surface、shadow、backdrop 依序增加深度；同一元件不必四者全用。
- overlay 必須讓背景降權但仍可理解上下文；dialog 開啟時，背景互動由 `inert` 與 focus trap 阻擋，不靠深色遮罩假裝不可用。
- glass surface 必須有足夠 fallback 底色與邊界；在不支援 blur、high contrast 或 reduced transparency 的情境仍可閱讀。
- sticky chrome 只在持續需要的任務使用，並預留其實際高度，避免遮住 anchor、toast 或最後一列內容。

**Current divergence.** 共享 `Dialog` 仍使用 raw `z-[1200]`，NotificationToast 與部分 route loader 也高於 foundation ladder。這些值只描述現況，不是新 overlay 可複製的 precedent。

**The Solid-by-Default Rule.** 暖白實色是預設材質；模糊玻璃只用於導覽、overlay、dialog 或被明確設計的品牌舞台，不能成為每個 section 的共同皮膚。

**The Minimum Necessary Depth Rule.** 先用 spacing、邊界與 tonal surface 分組；只有當元件真的離開原平面或需要互動回饋時才加陰影與 backdrop。

## Shapes

現況有兩套仍在使用的圓角：foundation scale 服務頁面與舊版 layout，UI scale 服務共享 `Button`、`Card`、`Input` 與 `Dialog`。這是已知技術差異，不是邀請新增第三套尺寸。

共享元件應透過其 API 與 UI token 取半徑：標準 control 使用 12px、預設 Card 使用 14px、Dialog 使用 28px。pill 只留給導覽、chip、segmented control 與明確的膠囊操作；圓形只留給 avatar、icon button 與狀態點。

| 角色 | 建議圓角 | 說明 |
| --- | --- | --- |
| 小型控制、內嵌狀態 | 8px | 適合密集工作區，不作大面板 |
| Button、Input、Dropdown trigger | 12px | 共享 control 預設 |
| Card | 14px | 現行 UI Card 的辨識值 |
| section panel、較大媒體框 | 16–20px | 需要比 card 更寬的容器 |
| Dialog / Sheet | 28px | 離開頁面層級的表面 |
| 品牌 hero shell | 32–45px | 只限已有明確 route composition |
| pill / full | 999px / 9999px | 導覽、chip、segmented control |
| circle | 50% | avatar、狀態點、正方形 icon button |

容器與圖片的曲線要有內外關係：內層媒體若貼齊外框，沿用或略小於外層半徑；若有 padding，內層半徑至少比外層小一個 spacing step。不要讓小圖片使用比外卡更大的圓角，也不要用不同圓角暗示不存在的互動層級。

商品與人物影像以裁切目的決定比例：方形、4:3、直式與 video ratio 都已有 production 用例。列表中的同類媒體要固定 ratio，避免載入後 layout shift；`object-fit: cover` 只在裁切不會破壞標示、包裝文字或主體時使用。

**The Role Before Radius Rule.** 先判斷元件角色，再選既有半徑；不要把 36px 或 45px 大圓角套到每張卡片，也不要逐頁手寫相近數字。

**The Nested Curve Rule.** 內外曲線必須反映真實容器關係；如果兩層沒有不同的背景、邊界或互動責任，就不需要再包一個圓角殼。

## Components

production component 是語意、狀態與無障礙行為的集合，不只是外觀片段。新介面先找 `components/ui/*` 或 `components/shared/*`，透過 props 與 token 組合；只有在責任不同且現有 API 無法安全擴充時才新增 primitive。

所有可互動元件至少要定義 default、hover（若裝置支援）、active、focus-visible、disabled；會等待結果者再加 loading，會被選取者再加 selected，會驗證者再加 invalid。狀態必須保留版面尺寸、可讀名稱與鍵盤路徑，不能只換顏色或讓內容突然消失。

### Buttons

- **Sizes:** `sm / md / lg` 視覺高度為 32 / 40 / 48px；預設 md 是 40px、12px 圓角、16px 水平 padding。觸控主動作優先用 lg，或擴大命中區。
- **Solid / primary:** 磚橘實底、白字；一個決策區只保留一個視覺最強的主要動作。
- **Bordered / primary:** 次要動作，保留品牌色邊界；不能和 solid 同強度。
- **Light:** 安靜、低強度或鄰近操作。相容名稱 `flat → light`、`faded / ghost → bordered`、`shadow → solid`，不是新的視覺階層。
- **States:** hover 使用 UI primary hover，active 可縮至 0.97，focus-visible 保留共享 ring。disabled 與 `aria-disabled` 不得觸發 action。
- **Loading:** 保留原按鈕寬度，原文字可視隱藏但仍維持幾何；一般操作改用中性灰背景、品牌 LOGO 小動畫與明確的繁中處理文字，物流保留 Truck、上傳保留 Upload 動畫。雙向切換若有兩組處理文案，以 `loadingReservationLabel` 預留較寬者，同時讓 `loadingLabel` 說明本次實際動作。所有 loading 按鈕須同步 `disabled`、`aria-busy`，並在 `prefers-reduced-motion` 下停止動態、保留可辨識的靜態圖示，避免版面跳動與重複送出。
- **Links:** 導航使用真正的 `<a>`／Link；只有執行當前頁動作才用 `<button>`。不要用 div 模擬按鈕，也不要把 disabled link 留成可導覽。

### Cards / Containers

- **Corner Style:** 共享 Card 預設 14px；大半徑只用於已定義的 hero、modal 或特定產品殼層。
- **Background:** 奶油卡面搭配低對比品牌邊界。
- **Shadow Strategy:** 預設使用 UI card；只有 `isHoverable` 或 `isPressable` 才浮起。
- **Internal Padding:** Card root 不帶固定 padding；`CardHeader`、`CardBody`、`CardFooter` 各自預設 20px。
- **Nesting:** 不以「卡片裡再放卡片」當作預設分組方式；先用 spacing、divider 或標題層級。
- **Media:** `CardMedia` 支援 square、video、portrait 等固定比例；產品主圖經 `AppImage`／Next Image 載入，預留尺寸並提供符合內容的 alt。
- **Interaction:** 整張卡導覽優先用真正 Link，整張卡執行動作用真正 Button。內有多個 action 時，卡片本身不可再成為單一大按鈕。
- **Current divergence:** `Card isPressable` 目前只補 `role="button"` 與 `tabIndex=0`，沒有 Enter／Space 啟動；在修正前不得把它描述為完整鍵盤模式，也不要在新流程複製。

### Inputs / Fields

- **Style:** `Input`、`Textarea`、`Select` 預設 bordered / md，使用奶油卡面、1px input border、16px 文字、16px 水平 padding 與 12px 圓角；單行欄位至少 48px 高，`Textarea` 保持多行高度與 12px 垂直 padding。這組外觀對齊買家／賣家 App 的一般 outlined 欄位。
- **Variants / Density:** 明確指定的 flat、faded、underlined 保留原有表面與尺寸。bordered / sm 保留滑鼠操作的 32px 精簡尺寸，在 coarse pointer 裝置至少 44px；不要把共用 control token 的 32 / 40 / 48px 級距當成 bordered / md 的實際高度。
- **Focus:** bordered 欄位以單一道磚橘邊框由 1px 加厚至 1.5px；錯誤且聚焦時為 2px 危險色邊框。以內側描邊維持內容位置，不加外圍 halo；其他明確 variant 保留既有 focus 外觀。
- **Labels:** bordered 欄位的外置 label 為 16px、字重 600。`floatingLabel` 模式在空白且未聚焦時將 label 放在欄內；聚焦或已有內容時縮至 12px，停留在上邊框缺口，填寫後仍可見。placeholder 只提供格式範例，不能代替欄位名稱、預填值或必要說明。
- **Error / Hint:** bordered 欄位的說明與錯誤文字為 14px，從欄位左側內縮 16px，與輸入內容對齊；錯誤文字沿用可讀性較高的 `danger-text`，不直接套用 App 邊框的亮橘。錯誤放在欄位旁且持續存在，透過 `aria-invalid` 與 `aria-describedby` 關聯；成功送出前不以 toast 取代欄位錯誤。
- **Disabled / Read-only:** bordered 停用欄位保留奶油表面與可讀文字，以停用邊框及游標表達狀態。disabled 表示不可操作，read-only 表示可閱讀／複製但不可修改；兩者的游標、對比與送出行為不同。
- **Input methods:** 數字、電話、日期與搜尋設定正確 `type`／`inputMode`。文字欄不得在中文 IME composing 階段誤送出、過早驗證或攔截 Enter。
- **Dropdown:** 使用 searchable dropdown／listbox 處理長選項，提供可見 label、鍵盤上下移動、Enter 選取、Esc 關閉與 empty state；短且原生語意足夠時保留 `select`。
- **Current divergence:** 共享 `Input` 已連結 label、hint 與 error；`Textarea`、`Select` 雖顯示訊息，尚未提供同等 ID／`aria-describedby` 關聯。新畫面不可宣稱所有 shared fields 已全面一致。
- **Platform boundary:** 此契約對齊一般欄位的外觀與狀態，不表示所有 App 專用輸入或 Web 舊畫面已相同。Web 保留原生語意、鍵盤操作與深色 theme；App 的字體、hint 色及賣家玻璃填色差異見 `design-system/cross-platform.md`。

### Chips, badges and status

- **Chip:** 用於狀態、分類、標籤或可移除資訊，現有高度 24 / 28 / 32px，支援 solid、flat、bordered、dot。Chip 不負責頁面導覽，也不應承載完整句子。
- **Badge:** 疊在 icon、avatar 或物件上表達數量／新狀態；數量過大時採既定縮寫，不讓 badge 撐壞宿主。若資訊重要，宿主旁仍要有可讀文字。
- **Filter rail:** URL／狀態篩選優先使用 `ScFilters + ScPill`：單軌橫向捲動、tablist 語意、選中項目可見且自動置中。不要把 Chip、Tabs 與 navigation 混成同一列。
- **Temperature:** frozen 使用冷鏈藍、chilled 使用 primary、ambient 使用栗棕；顏色必須跟著溫層文字與圖示，不可當裝飾 chip。
- **Current divergence:** `TemperatureTag` 仍含舊版 literal 顏色與 18px 尺寸，尚未成為 canonical primitive；Chip close 目前只有 20px 命中區。不得複製其色碼或觸控債務建立新 variant。

### Tabs, segmented controls and switches

- `Tabs` 用來切換同一工作區內的 peer views，必須維持 `tablist / tab / tabpanel` 關係，並支援 Arrow、Home、End 與焦點／選取區別。
- underlined tabs 適合內容層級；solid、bordered、light 可用於較緊密的 view switch。若切換會改變 URL 或跨頁，改用 Link navigation，不偽裝成 tab。
- segmented control 只處理少數互斥選項；每個選項文字短且同層級。超過可掃讀數量時改用 select、filter rail 或 navigation。
- `Switch` 只表示立即生效的 on/off 設定，label 放在控制外並描述結果；需要送出、會造成重大變更或不是布林值時使用 checkbox／radio 加確認 action。

### Date and calendar

- `DatePicker` 與 Input family 共用 flat／bordered／faded／underlined、32 / 40 / 48px 尺寸與欄位狀態；不要在同一表單另做一套日期外觀。
- 顯示預設採 `zh-TW`，可用西元或民國年；資料 wire value 維持 `YYYY-MM-DD`，date-time 維持 local `YYYY-MM-DDTHH:mm`。畫面格式、儲存格式與時區責任要分開說明。
- 支援手動輸入與 calendar 選取，min／max 不可選日期同時有 disabled 語意。日期錯誤放在欄位旁，不等到送出後才用 toast 說明。
- desktop calendar 就近錨定欄位；768px 以下以 portaled modal 呈現並鎖定背景捲動。上一月、下一月、年月跳轉與日期按鈕都要有可讀名稱。

### Dialogs / Sheets

- **Panel:** 預設 28px 圓角、奶油卡面、品牌邊界與 dialog shadow；header/body 使用 24px 水平 padding。
- **Behavior:** 保留 `aria-modal`、可讀標題、初始焦點、Tab focus trap、Esc、dismiss、scroll lock、背景 `inert` 與關閉後焦點還原。
- **Dialog:** 適合需要集中決策、短表單或不可忽略的確認；內容過長時 body 自己捲動，header 與 action 保持可到達。
- **Bottom sheet:** 窄 viewport 的短選單、篩選或行動任務，從底部進入，最高不超過可視高度的 82%；保留拖曳以外的明確關閉方式。
- **Side sheet:** 適合較寬 viewport 的 detail／edit context，不取代完整頁面，也不在 sheet 裡再開同層 sheet。
- destructive action 說明對象與後果，主要危險按鈕不因位置或顏色讓人誤按；能復原時先提供復原。
- **Current divergence:** overlay 顏色與 raw `z-[1200]` 尚未升格成 foundation token，close button 命中區目前為 36px。新增 dialog 直接重用共享元件，不複製這些值。

### Navigation

- **Public / buyer top:** desktop 為 84px 浮起 pill，768px 以下為 56px 平面列；active route、hover 與 focus-visible 各有獨立狀態。
- **Mobile bottom:** 840px 以下顯示五欄浮動導覽，項目是實際 Link、48px 高並使用 `aria-current`。頁面需預留 68px chrome 與 10px bottom gap。
- **Exceptions:** 賣家中心、訊息 thread 與獨立沉浸流程可隱藏 buyer bottom nav；隱藏規則由 route contract 控制，不用 `z-index` 或 accidental overflow 遮掉。
- **Seller:** 與 public chrome 共用品牌與基本互動語言，但入口、密度與工作狀態可不同；不得假裝成買家主導覽。
- **Boundary:** buyer/seller navigation 是 route-aware chrome，不是其他卡片、filter 或 segmented control 的造型範本。

### Loading, empty, error and toast

- **Route loading:** skeleton 的幾何應接近最終內容，避免載入完成後大幅跳動；production skeleton 已用 `role="status"`、`aria-live="polite"`。不要用一個全頁 spinner 取代可預測的結構。
- **Inline loading:** 保留觸發元件與上下文，說明正在處理的對象；背景重整不應把已讀內容清空。
- **Empty:** 說明「目前沒有什麼、為什麼可能沒有、下一步是什麼」。一般工作區用安靜基礎版；品牌動畫版只在語氣合適的空狀態使用。
- **Error:** 欄位／區塊失敗使用鄰近、持續存在且可重試的 alert；致命頁面同時提供恢復與回首頁出口。不要把需要閱讀或重試的錯誤只放進短暫 toast。
- **Toast:** 只處理短暫完成回饋或不阻斷的更新，位置需避開 bottom nav 與 dialog。相同事件合併，不用連續 toast 轟炸使用者。
- loading、empty、error 不能虛構資料；保留最後成功內容時，要清楚標示資料時間與更新失敗。

### Tables and lists

- 只有需要跨列、跨欄比較的資料才用 semantic `table`；一般內容序列使用 `ul / ol / li`，定義資料使用 `dl`。
- 表頭描述欄位，排序按鈕可由鍵盤操作並暴露排序狀態；數字右對齊並使用 tabular numbers，文字與狀態通常左對齊。
- 專案目前沒有 canonical Table primitive。高密度賣家表格採水平捲動、固定欄寬／skeleton 對齊與必要 sticky anchor；小螢幕可保留第一欄錨點，或轉為資訊順序清楚的卡片列表。
- 整列導覽使用真正 Link，整列 action 使用真正 Button；多 action row 不可再讓整列成為不明確的 click target。
- 空表、載入、錯誤、無搜尋結果與無權限是不同狀態，文案與下一步不可共用一個泛用 placeholder。

### Icons, imagery and data graphics

- product UI icon 只透過 `AppIcon` 使用（底層是 `@hawkr/icons` 的 Material Symbols，不直接 import `@hawkr/icons`）；預設 24px、outlined、`currentColor`；選取、已按讚等狀態用 `fill` 版並保留文字或形狀線索，不只靠顏色。有可見文字時 icon 為 `aria-hidden`；單獨表意時提供 `title` 或等價 accessible name。
- 不用 emoji 取代產品圖示。
- 商品圖優先呈現真實品項、包裝與尺度；職人圖優先能看出人物、工作與環境的關係。裝飾插畫不能假裝成真實供應、認證、產地或交易證據。
- 圖片 alt 描述對任務有用的內容，不重複相鄰標題；純裝飾圖使用空 alt。含文字的圖片必須另有可讀文字，不以海報像素取代 HTML。
- 圖表、價格趨勢與市場資料同時標示指標、單位、時間、新鮮度、樣本／範圍與缺失狀態。顏色不單獨編碼系列，也不把估計值畫成已確認事實。

### Media upload and crop

- `ImageUploader` 的 empty、dragging、uploading、preview、error、disabled 狀態保持同一個欄位位置；上傳進度使用可讀 status，不讓使用者猜檔案是否仍在處理。
- 明列接受格式、檔案大小、張數與用途；`accept` 只改善檔案挑選器，不是安全驗證。client 與 server 驗證失敗都要保留檔名、原因與重試方式。
- avatar、cover、商品圖與證明文件各自有固定 ratio／`cover` 或 `contain` 規則。證明文件不可因 `cover` 裁掉文字，商品圖也不應在不同卡片任意改比例。
- crop 是 opt-in 下一步；確認前不覆寫原檔，取消後回到 uploader，並維持焦點與未完成佇列。每個 preset 說明最終用途，而不是只顯示抽象比例。
- preview 的更換、移除不能只在 hover 出現；touch 與 keyboard 有等價 action。刪除已上傳素材時，依可復原性提供確認或 undo。

### Brand Logo

- **API:** 新介面只透過 `BrandLogo` 與 `brandAssets` 使用正式 horizontal、mark 或 grass 資產。
- **Default:** horizontal / md / plain；dark horizontal 使用 wheat-cream 資產。
- **Boundary:** `plain` 是品牌標準。soft、pill、solid 是相容中的表面 variant，不得推論為正式 logo 規範。
- **Clear space:** 未有獨立品牌資產規格前，工程基線以 horizontal 顯示高度的 25%、mark 的 20% 保留淨空；最小 mark 建議 mobile 32px、desktop 40px。若品牌資產規格更新，以核准資產規格覆寫。
- **Integrity:** 不拉伸、旋轉、描邊、重畫路徑、套漸層或任意換色；在複雜照片上先提供可控實色／遮罩背景，不靠 glow 補對比。
- **Current divergence:** 現行 `.brand-logo__lockup` 仍有預設 drop-shadow，與「標誌不加外光」的品牌規範有落差；不要把這個 shadow 複製到新品牌資產。

### Motion

- **UI timing:** 共享 UI 採 120ms（小狀態）、200ms（一般 transition）、320ms（dialog／sheet）與 decelerating `cubic-bezier(0.22, 1, 0.36, 1)`。
- **Foundation timing:** route／舊 layout 另有 150ms、250ms、350ms，以及 standard、emphasized、bounce、linear-out easing。同一元件不可混用兩套 timing；新 shared primitive 優先 UI timing。
- **Purpose:** motion 只用於 hover、focus、展開、modal、狀態 continuity、導引或單頁敘事。page／sheet transition 才使用 emphasized easing；bounce 僅限一次性愉悅回饋。
- **Reduced motion:** 移除位移、視差、scrub 與持續循環，保留必要狀態切換；skeleton 停止 shimmer。不可只把 duration 改成極短卻留下閃動。
- **Ambient animation:** 全站持續漂動的葉片已移除，不得以另一組 particles、orbs 或無任務動畫補回。

### Accessibility and input parity

- 所有功能以鍵盤可到達、順序合理，focus-visible 清楚且 dialog 關閉後返回觸發點；不建立 focus trap 以外的鍵盤死角。
- hover 只作增強，不能承載唯一資訊。pointer、touch、keyboard 與中文 IME 都能完成相同核心任務。
- icon-only control、狀態圖、圖表與 loading 有可讀名稱；動態更新使用合適的 `aria-live`，但不讓非必要更新反覆打斷。
- 表單錯誤、成功、交易狀態與庫存變化以文字加視覺訊號表達。required、disabled、read-only 與 invalid 不只靠顏色。
- 在 200% zoom、文字放大、reduced motion、dark mode 與窄 viewport 下驗證核心流程；設計稿中的理想版面不是通過條件。
- 新元件需以 production DOM 驗證，不能只憑 sidecar preview 或靜態截圖宣稱無障礙完成。

**The Primitive Before Pattern Rule.** 先用現有 primitive 組合真實任務，再抽象重複模式；不要先畫一張泛用卡片，再把所有資料硬塞進去。

## Do's and Don'ts

### Do:

- **Do** 先讓使用者看懂商品來源、商家、規格、溫層、配送與交易狀態，再加裝飾。
- **Do** 使用真實商品、食品影像與商家資料；品牌插畫只服務關於、產地或單頁敘事。
- **Do** 透過 semantic token、共享元件、`BrandLogo` 與現行 spacing scale 建立一致性。
- **Do** 讓品牌頁有呼吸、操作頁有效率、閱讀頁易掃描；以任務調整密度。
- **Do** 讓動態對狀態、導引或敘事有明確用途，並提供 reduced-motion 結果。
- **Do** 先選 Persuade、Operate、Read 或 Experience mode，再決定密度、容器與 motion。
- **Do** 為 loading、empty、error、disabled、focus、dark mode、200% zoom 與窄 viewport 設計可驗證結果。
- **Do** 把 route exception、current divergence 與正式 foundation 分開命名，讓下一個人知道什麼可重用、什麼只能維護。

### Don't:

- **Don't** 使用舊草案的 `#ee964b` 等舊色作為新頁面的品牌主色，也不要新增相近的「另一種品牌色」。
- **Don't** 把藍紫科技漸層、霓虹、純黑金屬感或滿版 glassmorphism 當成 Hawkr 的品牌語言。
- **Don't** 用全站飄葉、particles、orbs 或持續漂移的裝飾製造「自然感」。
- **Don't** 每節都複製 eyebrow／hashtag、置中大標、三張等權卡片與重複 CTA 的 AI 落地頁公式。
- **Don't** 為了填空在交易、帳戶或賣家工具頁加入通用人物插畫、假儀表板或無來源數字。
- **Don't** 用「重新定義」、「從產地到餐桌」、「安心、新鮮、永續」等空泛套語填滿 hero，也不要在沒有證據時承諾所有商品新鮮直送、絕對安心或賣家收益；商業與文案真值應另由 `PRODUCT.md` 固定。
- **Don't** 把 `/seller` 的 scroll world、Vanta、旋轉 badge 或 scrub video 升格成全站元件；它們必須逐項證明對該頁有用。
- **Don't** 在既有兩套圓角之外新增任意 radius，也不要把每個容器都做成卡片。
- **Don't** 用 div 模擬 Link、Button、Tab 或 Table，也不要讓整張 Card 的 click handler 取代正確鍵盤語意。
- **Don't** 用 toast 取代欄位錯誤、致命錯誤或需要重試的持續狀態。
- **Don't** 把 32px／40px 的視覺高度寫成已符合 44px 觸控命中區；未達標就明列債務或擴大 hit area。
- **Don't** 從 sidecar preview、舊指南或單一路由抄 literal 色碼、shadow、z-index 與 breakpoint，宣稱它是全站 token。

### Release acceptance

設計或實作要被視為符合本系統，至少要能回答：

1. 這個頁面屬哪一種 surface mode，主要任務與第一層資訊是什麼？
2. 顏色、字體、間距、圓角、陰影與 icon 是否來自正確 source layer？
3. 新值是 foundation、shared component 還是 route exception？名稱與範圍是否清楚？
4. primary、secondary、loading、empty、error、disabled、focus 與 selected 狀態是否齊全？
5. keyboard、touch、中文 IME、44px hit area、WCAG AA、200% zoom、reduced motion 與 dark mode 是否有 production 驗證？
6. 真實商品、職人、價格、供應、認證與成效主張是否有來源；未知資訊是否誠實降級？
7. mobile top／bottom navigation、sticky action、dialog、toast 與 route loader 是否互不遮擋？
8. 靜態設計、local test、CI、deployment 與 live runtime 證據是否被分開陳述？
