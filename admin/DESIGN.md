---
name: "Hawkr Admin"
description: "與 Hawkr 網頁一致、成熟且重視營運安全的操作後台"
colors:
  primary-strong: "#b8441f"
  primary-strong-hover: "#8f3315"
  primary-strong-pressed: "#8f3315"
  primary: "#e4572e"
  cream: "#f8f6ef"
  paper: "#fffdf7"
  control: "#f0f1ea"
  panel: "#f2e9d6"
  ink: "#191c18"
  body: "#42493f"
  muted: "#72796e"
  muted-aa: "#5f675b"
  border: "#d9ddd2"
  success: "#2d7f50"
  success-soft: "#e7f3e9"
  warning: "#8a5a00"
  warning-soft: "#fff3d6"
  danger: "#b3261e"
  danger-soft: "#fce8e6"
  info: "#3f6f9e"
  info-soft: "#eaf2f9"
  delegated-secondary: "#f2a541"
  delegated-secondary-soft: "#fdf1e1"
  delegated-secondary-border: "#f2a541"
  focus-ring: "#b8441f"
typography:
  display:
    fontFamily: "var(--font-openhuninn), var(--font-roboto), 'Noto Sans TC', sans-serif"
    fontSize: "clamp(42px, 5vw, 70px)"
    fontWeight: 700
    lineHeight: 1.13
    letterSpacing: "0.01em"
  headline:
    fontFamily: "var(--font-openhuninn), var(--font-roboto), 'Noto Sans TC', sans-serif"
    fontSize: "clamp(27px, 2.2vw, 34px)"
    fontWeight: 700
    lineHeight: 1.2
    letterSpacing: "0.01em"
  metric:
    fontFamily: "var(--font-roboto), 'Noto Sans TC', system-ui, sans-serif"
    fontSize: "clamp(28px, 2.5vw, 34px)"
    fontWeight: 800
    lineHeight: 1.1
    letterSpacing: "-0.02em"
  title:
    fontFamily: "var(--font-roboto), 'Noto Sans TC', system-ui, sans-serif"
    fontSize: "16px"
    fontWeight: 750
    lineHeight: 1.5
    letterSpacing: "normal"
  body:
    fontFamily: "var(--font-roboto), 'Noto Sans TC', system-ui, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: "normal"
  button:
    fontFamily: "var(--font-roboto), 'Noto Sans TC', system-ui, sans-serif"
    fontSize: "14px"
    fontWeight: 700
    lineHeight: "20px"
    letterSpacing: "normal"
  label:
    fontFamily: "var(--font-roboto), 'Noto Sans TC', system-ui, sans-serif"
    fontSize: "13px"
    fontWeight: 700
    lineHeight: "18px"
    letterSpacing: "normal"
rounded:
  compact: "10px"
  control: "12px"
  card: "14px"
  panel: "16px"
  workspace: "24px"
  dialog: "28px"
  pill: "999px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "20px"
  "2xl": "24px"
  "3xl": "28px"
  "4xl": "32px"
components:
  button-primary:
    backgroundColor: "{colors.primary-strong}"
    textColor: "{colors.paper}"
    typography: "{typography.button}"
    rounded: "{rounded.control}"
    padding: "10px 16px"
    height: "44px"
  button-primary-hover:
    backgroundColor: "{colors.primary-strong-hover}"
    textColor: "{colors.paper}"
  button-primary-active:
    backgroundColor: "{colors.primary-strong-pressed}"
    textColor: "{colors.paper}"
  button-outline:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.body}"
    typography: "{typography.button}"
    rounded: "{rounded.control}"
    padding: "10px 16px"
    height: "44px"
  button-danger:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.danger}"
    typography: "{typography.button}"
    rounded: "{rounded.control}"
    padding: "10px 16px"
    height: "44px"
  status-success:
    backgroundColor: "{colors.success-soft}"
    textColor: "{colors.success}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    padding: "5px 11px"
  status-warning:
    backgroundColor: "{colors.warning-soft}"
    textColor: "{colors.warning}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    padding: "5px 11px"
  status-danger:
    backgroundColor: "{colors.danger-soft}"
    textColor: "{colors.danger}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    padding: "5px 11px"
  card:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.card}"
    padding: "22px"
  input:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.control}"
    padding: "10px 13px"
    height: "44px"
  delegated-trigger:
    backgroundColor: "{colors.delegated-secondary-soft}"
    textColor: "{colors.primary-strong}"
    typography: "{typography.button}"
    rounded: "{rounded.compact}"
    padding: "9px 16px"
    height: "44px"
  delegated-context:
    backgroundColor: "{colors.delegated-secondary-soft}"
    textColor: "{colors.primary-strong}"
    rounded: "{rounded.card}"
    padding: "15px 18px"
---

# Design System: Hawkr Admin

## Overview

**Creative North Star: "沉穩的生鮮營運台"**

Hawkr Admin 是高頻操作的營運工具，不是行銷落地頁。整體以網頁的磚橘、暖米白與正式品牌資產建立可信度，再用清楚層級、緊湊但可掃描的資訊密度，讓管理員快速判斷狀態並安全完成工作。表現應成熟、安靜、明確；品牌存在於色彩、字型、Logo 與細節，而不是裝飾性特效。

本文件是登入頁、Admin shell、所有管理模組與 ADM 代行工作區的通用視覺真相。`PRODUCT.md` 與功能契約負責產品、安全及授權邊界；`.impeccable/surfaces/` 與 `docs/superpowers/specs/` 只能補充單一功能，不得覆蓋本文件的通用視覺規則。既有 CSS 中的舊 Figma 註解、legacy aliases 或一次性色碼也不是新畫面的依據。

**Key Characteristics:**

- 網頁一致的磚橘結構、暖色工作面與正式 SVG 品牌資產。
- 清楚、低噪音、適合長時間工作的營運密度。
- 邊框與色階優先，陰影只表示真正離開平面的介面。
- 狀態色承載語意；暖橘黃只標示 ADM 代行安全情境。
- 繁體中文優先、完整鍵盤操作、可縮放且誠實呈現資料狀態。

**The Website Authority Rule.** Hawkr 網頁是品牌資產與核心識別的來源；Admin 可參考外部產品的資訊層級，但不能複製其視覺身分。

## Colors

色彩以磚橘（primary-strong）建立結構與主要行動，以 Cream 作畫布、Paper 作工作面；品牌橘（primary）只提供輔助層次。所有新 UI 應使用 frontmatter 對應的 `--admin-*`／`--hk-*` semantic tokens，不新增頁面私有品牌色。

### Primary

- **營運磚橘** (`primary-strong`, `#b8441f`)：主要按鈕、側欄選取項、重要圖示與焦點方向。
- **磚橘互動階** (`primary-strong-hover`, `primary-strong-pressed`)：只用於磚橘元件的 hover／active，不另創深色。
- **品牌橘** (`primary`, `#e4572e`)：選取邊框、輔助強調與低權重品牌訊號，不與磚橘爭奪主行動；白字對比不足，不配一般尺寸的白字。

### Secondary

- **ADM 稽核金** (`delegated-secondary`, `delegated-secondary-soft`, `delegated-secondary-border`)：只用於代行工作階段、受控操作與唯讀安全提醒；不是一般 warning、促銷色或裝飾色。

### Neutral

- **暖米畫布** (`cream`)：頁面背景與長時間工作環境。
- **紙張工作面** (`paper`)：卡片、表格、表單、dialog 與可操作區。
- **控制層** (`control`) 與 **暖面板** (`panel`)：輸入、分段控制及次要資訊層。
- **墨色階** (`ink`, `body`, `muted`, `muted-aa`)：標題、正文與輔助文字；小字必須使用 AA muted，而不是過淡灰色。
- **安靜邊界** (`border`)：卡片、表格列、輸入與區塊分隔。

### Semantic

- **成功、警告、危險、資訊** (`success`, `warning`, `danger`, `info` 及各自 soft surface)：只表達對應狀態；文字與背景必須成對使用。
- **鍵盤焦點** (`focus-ring`)：所有可互動元素都要有一致、可見的 focus ring。

**The Brick Is Structure Rule.** 磚橘是導航與主要行動的結構色，不應大面積散落成裝飾斑塊。

**The Amber Means Delegation Rule.** 看到暖橘黃就應能推斷「目前是 ADM 受控代行或唯讀安全情境」。一般 Admin 功能不得借用這個語意。

## Typography

**Display Font:** openhuninn，以 Roboto、Noto Sans TC、sans-serif 後備。

**Body Font:** self-hosted Roboto，以 Noto Sans TC、system-ui、sans-serif 後備。

**Data Treatment:** 同一 UI 字型堆疊，數字使用 tabular numerals。

**Character:** openhuninn 只為品牌頁標與登入主標增加 Hawkr 的人味；所有資料、控制項與表格維持 Roboto 的清楚與穩定。不要新增 Inter、展示字或第三方字型依賴。

### Hierarchy

- **Display** (`typography.display`)：只用於登入 hero 等少數品牌主標。
- **Headline** (`typography.headline`)：頁面唯一的 H1；不在同頁重複大型標題。
- **Metric** (`typography.metric`)：統計值、倒數與關鍵數字，使用 tabular numerals。
- **Title** (`typography.title`)：卡片、dialog 與工作區段標題。
- **Body** (`typography.body`)：說明、表格內容與營運文案；長文控制在約 68ch。
- **Button / Label** (`typography.button`, `typography.label`)：動詞優先、短且明確，不使用模糊的「確定」或未解釋縮寫。

**The Display Font Is Rare Rule.** openhuninn 不進入表格、輸入、按鈕、徽章或密集資料；稀少才保留品牌辨識度。

## Layout

標準 Admin shell 使用固定 256px sidebar 與 72px topbar；主要內容上限 1600px，頁面 padding 依 viewport 在 24px 至 44px 間調整。預設統計與快速操作為四欄；賣家五項統計在寬桌機維持五欄，1280px 以下改成 3 + 2，640px 以下單欄。

主要斷點為 1280px、960px 與 640px：1280px 降低 grid 欄數；960px 將 sidebar 改為 modal drawer 並隱藏登入 hero；640px 將頁首、操作與卡片改為單欄。ADM workspace 另在 1180px、980px、760px 與 640px 重排安全 context、側欄與 dialog，但目標店家、操作者、原因、範圍與期限不得因響應式而消失。

表格在窄螢幕由自身容器水平捲動，不能撐寬整個 document；empty/loading/error 內容必須留在初始可視區。複合控制元件只能有一層可見邊界，搜尋 input 不得在外框內再畫第二層框。

**The Operations Density Rule.** 優先掃描、比較與決策；不要把 Admin 頁做成 marketing hero、巨型留白或卡片拼貼展覽。

## Elevation & Depth

普通卡片、表格與 filter bar 採 flat-by-default：Paper 工作面、安靜邊框、無常駐陰影。陰影只用於登入卡、浮動 topbar、mobile drawer、dialog、skip link 等真正脫離文件平面的介面。

### Shadow Vocabulary

- **Low:** `--admin-shadow-low`，只處理極輕的浮起狀態。
- **Lift:** `--admin-shadow-lift`，用於登入卡或暫時浮動工作面。
- **Dialog:** `--admin-shadow-dialog`，只用於 modal／drawer 與阻斷式確認。

**The Flat-By-Default Rule.** 一般內容卡使用 border 或 shadow 擇一；只有明確浮動的 chrome／overlay 可同時保留弱邊界與陰影。

## Shapes

控制項使用穩健的 12px 曲率；卡片 14px、面板 16px、工作區大型容器 24px、阻斷式 dialog 28px。10px compact radius 可用於密集 row actions；999px pill 只給狀態徽章、segmented controls 與少量浮動導覽，不用於普通卡片。

按鈕、輸入、nav row 與 icon button 的可點擊高度原則上至少 44px。圖示容器通常是 38px 至 44px 的圓角方形；品牌 Logo 不加不必要容器、陰影或重新著色。

**The Calm Corners Rule.** 同一層級只使用一種 radius；不要在一張卡內堆疊多個相近但不同的圓角外框。

## Components

### Brand

- 所有 Logo 經 `AdminBrand` 使用 `/brand/logo-horizontal.svg`、`/brand/logo-horizontal-light.svg` 或 `/brand/logo-mark.svg`。
- 磚橘背景使用 light lockup；Paper／Cream 使用 dark lockup；mark 只用於 favicon 或極窄空間。
- 禁止使用退役 raster Logo、文字重畫 Logo、任意 recolor 或拉伸比例。

### Buttons

- Primary、outline、danger 均使用 12px radius、至少 44px 高與明確動詞。
- Hover／active 只改 semantic 色階；一般操作不做彈跳、發光或大幅位移。
- Focus 使用 3px focus ring 與 2px offset；disabled 必須同時不可操作且保留可辨識標籤。

### Status Badges and Filters

- Badge 使用 semantic soft surface、同色文字與弱邊框；pill 代表狀態，不代表一般內容。
- Segmented filter 的選取狀態為 Paper + 磚橘，未選取狀態位於 Control surface；窄螢幕可在元件內水平捲動。

### Cards and Metrics

- 標準卡片使用 Paper、14px radius、22px padding 與 border-first 層次。
- Metric card 最小高度 118px；label 必須避開右上 icon，長標籤可換行但所有同列卡片等高。
- Empty、loading、error、disabled 與 unavailable 都要有真實狀態，不使用假資料填滿畫面。

### Inputs and Fields

- 單一欄位至少 44px 高、12px radius、Paper 或低對比暖白背景。
- Focus 以磚橘 border 加低強度 ring 表示；錯誤使用 Danger semantic pair。
- Search、日期範圍或 icon + input 等 compound control 只由外層持有可見邊界。

### Tables

- 表頭為暖灰工作面、13px bold label；資料列 14px body，數字使用 tabular numerals。
- 以 row divider 取代每格框線；hover 只做輕微 Paper／Cream tonal shift。
- 操作按鈕仍需達 44px；手機在 table container 內捲動。

### Navigation

- Sidebar 使用深色 Ink（`#1f2933`）底、分組標籤與 44px nav row；active 使用磚橘填色（白字）加左側 3px 橘色條（`#e4572e`），橘色只用在選取項與主按鈕。
- Mobile sidebar 是真正的 dialog：開啟時 trap focus、背景 inert、鎖 body scroll、Escape 關閉並 restore trigger focus。
- Topbar 只保留真實的操作與上下文；不要放無功能搜尋、通知或設定圖示。

### Dialogs and ADM Safety Context

- Dialog 必須有可讀標題、清楚 primary／cancel、focus trap、Escape policy、背景隔離及關閉後還焦。
- ADM 代行 context 持續顯示 actor、target、reason、scope、expiry 與結束入口；暖橘黃是安全辨識，不是裝飾。
- 訂單履約與敏感能力必須以明確唯讀／禁止狀態呈現，不能只靠 disabled button 暗示授權。

Motion 只服務狀態：一般控制 150–180ms，必要互動最多 250ms；skeleton shimmer 是唯一允許的 gradient。`prefers-reduced-motion` 下必須移除位移、shimmer 與非必要動畫。

## Do's and Don'ts

### Do:

- **Do** 先讀本文件，再查看同類既有元件與 `src/styles/AdminVisualRefresh.css`；優先延伸 tokens 與共用元件。
- **Do** 使用繁體中文、真實欄位與可恢復的 loading／empty／error／conflict 狀態。
- **Do** 在 desktop、mobile、keyboard 與 200% zoom 驗證新 UI；確保小字 AA、焦點可見、控制項至少 44px。
- **Do** 保留 Admin 與 ADM 操作者身分；功能安全與授權依 `PRODUCT.md`、server contract 與功能 spec。
- **Do** 在刻意修改通用視覺系統時，同步更新本文件與 `.impeccable/design.json`。

### Don't:

- **Don't** 從舊 `--color*`、`--brand-*`、頁面 raw hex 或歷史 Figma 註解建立新 palette；它們只供 legacy compatibility。
- **Don't** 引入裝飾性 gradient、glassmorphism、紫色狀態、粒子、GSAP、Lenis、Vanta 或 React Bits 效果來「增加質感」。
- **Don't** 建立雙層輸入框、巢狀卡片外框、普通卡片的常駐大陰影，或 border + shadow 的無理由疊加。
- **Don't** 放沒有功能的搜尋、通知、設定或假 dashboard 數據；本機 mock／截圖也不能宣稱是 DEV 或 production proof。
- **Don't** 用 `切換身分`、`登入賣家` 等文案暗示 impersonation，也不能把 client-side hidden／disabled 當成授權。
