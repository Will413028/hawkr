# Hawkr Design System Guidelines

> **權威入口：** 專案根目錄的 [`DESIGN.md`](../DESIGN.md) 是目前供設計與 AI 工具讀取的品牌真值。本文件保留品牌細節與實作背景；若兩者或程式現況不一致，依 root `DESIGN.md` 記錄差異，並以 `styles/foundation/00-color-config.css` 的執行 token 為準。

來源：

- 品牌素材與色票：`../brand/hawkr/README.md`
- 色彩設定入口：`styles/foundation/00-color-config.css`
- 現有前端 tokens：`styles/foundation/01-tokens-theme.css`
- UI Kit tokens：`styles/components/ui-kit.css`
- Logo assets：`public/brand/logo-horizontal.svg`、`public/brand/logo-horizontal-light.svg`、`public/brand/logo-mark.svg`

這份文件的目的，是把品牌手冊整理成前端、設計、Claude 後續改版都能共同遵守的規則。實作時以 `brand/hawkr/README.md` 的色票為主，現有專案 token 為輔；若兩者衝突，往色票收斂。

色彩維護原則：

- 要改全站品牌色，先改 `styles/foundation/00-color-config.css`。
- 頁面 CSS 不直接新增 HEX；需要頁面專屬顏色時，先在 `00-color-config.css` 新增 `--page-*` token。
- `01-tokens-theme.css` 和 `styles/components/ui-kit.css` 只負責把全站 token 轉接給舊元件與 UI kit。
- 圖示強調色由 `--icon-accent`、`--icon-accent-secondary` 管理。

## 1. 品牌方向

Hawkr 的視覺核心是「每日採收、從產地到日常餐桌」。

品牌給人的感覺應該是：

- 新鮮：明亮的橘紅、產地直送。
- 穩定：磚橘、暖白、低對比陰影、乾淨留白。
- 溫暖：暖橘黃、手繪線條、生活感插圖。
- 不過度科技化：避免冷硬藍紫漸層、玻璃霓虹、過深黑色金屬感。

設計語氣：

- 買家端：清楚、舒服、可信任，像逛一個有溫度的日常市集。
- 賣家端：務實、密度較高、資訊清楚，像一個好用的工作台。
- 行銷頁：可以更有留白與圖像，但仍以食材、產地、季節為主。

## 2. Logo 使用準則

### 2.1 可用 Logo

目前專案內已有正式 SVG：

| 用途 | 檔案 | 建議使用情境 |
| --- | --- | --- |
| 橫式 Logo | `public/brand/logo-horizontal.svg` | Navbar、Footer 淺色背景、文件抬頭 |
| 橫式淺色 Logo | `public/brand/logo-horizontal-light.svg` | 深色模式 Navbar、深色 footer |
| 圖標 Logo | `public/brand/logo-mark.svg` | App icon、avatar fallback、favicon、loading mark |

Logo SVG 使用到的色票包含：

- `#E4572E` 品牌橘（遮雨棚）
- `#F2A541` 暖橘黃（條紋）
- `#1F2933` 墨藍灰（檯面，深色底改 `#F5F7FA`）

### 2.2 禁止事項

品牌規範：

- 不重新繪製 Logo。
- 不改變 Logo 比例、文字、圖形位置。
- 不替 Logo 加框、描邊、外發光。
- 不把 Logo 放在複雜照片、複雜 pattern 或低對比背景上。
- 不任意替換成非授權色。

### 2.3 安全距離

落地建議如下：

- 前端工程暫定規則：
  - 橫式 Logo：四周 clear space 至少為 Logo 顯示高度的 `25%`。
  - Mark Logo：四周 clear space 至少為 mark 寬度的 `20%`。
  - Hero、品牌頁、登入頁：可提高到 `32%` 到 `50%`，讓 Logo 更有呼吸感。
- 安全距離內不可放：文字、icon、照片、商品圖、分隔線、badge。

建議 CSS token：

```css
:root {
	--brand-logo-clearspace-ratio-horizontal: 0.25;
	--brand-logo-clearspace-ratio-mark: 0.2;
}
```

### 2.4 最小尺寸

| Logo | Desktop | Mobile | 備註 |
| --- | ---: | ---: | --- |
| 橫式 Logo | 寬 `160px+` | 寬 `120px+` | Navbar 可依高度縮放，但文字需清楚 |
| Mark | `40px+` | `32px+` | 小於 32px 時細節會糊 |
| 直式 Logo | 寬 `120px+` | 寬 `96px+` | 適合品牌場景，不建議塞在一般卡片 |

## 3. 色彩系統

### 3.1 品牌色票

| Token | 名稱 | HEX | RGB | 建議用途 |
| --- | --- | --- | --- | --- |
| `--color-brand-primary` | 品牌橘 | `#E4572E` | 228, 87, 46 | logo、大標題（24px 以上，或 18.66px 以上粗體）、裝飾；不配一般尺寸的白字（3.68:1） |
| `--color-brand-primary-strong` | 磚橘 | `#B8441F` | 184, 68, 31 | 互動元件：主按鈕底、連結、選取態、focus；白字 5.41:1 |
| `--color-brand-primary-deep` | 深磚橘 | `#8F3315` | 143, 51, 21 | hover、pressed；白字 7.92:1 |
| `--color-brand-secondary` | 暖橘黃 | `#F2A541` | 242, 165, 65 | 輔色底與少量強調，配墨色文字 7.19:1，不配白字 |
| `--color-brand-ink` | 墨藍灰 | `#1F2933` | 31, 41, 51 | Hawkr 的文字色、hairline 與陰影的色相 |
| `--color-brand-ink-on-dark` | 深底墨白 | `#F5F7FA` | 245, 247, 250 | 深色底上的文字與 logo 檯面 |
| `--color-brand-primary-soft` | 橘紅淡階 | `#FCEBE6` | 252, 235, 230 | primary 12% 混白；淺底、標籤，配墨色文字不低於 4.5:1 |
| `--color-brand-secondary-soft` | 橘黃淡階 | `#FDF1E1` | 253, 241, 225 | secondary 16% 混白；淺底、季節裝飾 |

每個色票另有 `-rgb` 變體（例如 `--color-brand-ink-rgb: 31 41 51`），給 `rgb(var(...) / alpha)` 的半透明用法。各前景與背景組合的對比由 `backend/docs/plans/2026-10-24-portfolio-public/contrast.py` 計算。

### 3.2 狀態色

狀態色是通用色，不屬於品牌色票，值固定。

| Token | 名稱 | HEX | 建議用途 |
| --- | --- | --- | --- |
| `--color-status-success` | 成功 | `#2D7F50` | 成功、已完成、可上架 |
| `--color-status-warning` | 警示 | `#E0C44C` | warning、活動標籤、優惠提示 |
| `--color-status-danger` | 危險 | `#EA6021` | 錯誤、高強度促銷，不宜大面積 |
| `--color-status-info` | 資訊 | `#6194D0` | active info、配送進度 |
| `--color-status-info-strong` | 深資訊 | `#1E5FAB` | 深藍資訊、物流、特殊狀態 |
| `--color-status-info-soft` | 淡資訊 | `#9FCCDD` | 冷鏈、淡色資訊底 |

### 3.3 Light Mode 建議

```css
:root {
	--color-brand-primary: #e4572e;
	--color-brand-primary-strong: #b8441f;
	--color-brand-primary-deep: #8f3315;
	--color-brand-secondary: #f2a541;
	--color-brand-ink: #1f2933;

	--surface-page: #f8f6ef;
	--surface-card: #fffdf7;
	--surface-muted: #f3f0e7;
	--surface-elevated: #ffffff;

	--text-heading: #191c18;
	--text-body: #42493f;
	--text-muted: #72796e;
	--text-inverse: #fffdf7;

	--action-primary: #b8441f;
	--action-primary-hover: #8f3315;
	--action-secondary: #b8441f;
	--action-accent: #f2a541;

	--border-soft: rgba(31, 41, 51, 0.14);
	--border-warm: rgba(242, 165, 65, 0.28);
	--overlay: rgba(31, 41, 51, 0.42);
}
```

Light mode 使用規則：

- 主要 CTA 使用 `#B8441F`，白字；hover 與 pressed 用 `#8F3315`。
- `#E4572E` 不配一般尺寸的白字，只給 logo、大標題與裝飾。
- 大面積背景以 `#F8F6EF`、`#FFFDF7` 與兩個淡階 `#FCEBE6`、`#FDF1E1` 為主。
- `#F2A541` 用於價格促銷、焦點、分隔線，配墨色文字，不要取代主 CTA。
- 錯誤、強促銷使用 `#EA6021`，避免整頁大面積使用。

### 3.4 Dark Mode 建議

```css
[data-theme="dark"] {
	color-scheme: dark;

	--surface-page: #0e1c1a;
	--surface-card: #1a2e25;
	--surface-muted: #213a2e;
	--surface-elevated: #28473a;

	--text-heading: #f5e9c3;
	--text-body: rgba(232, 239, 226, 0.86);
	--text-muted: rgba(232, 239, 226, 0.62);
	--text-inverse: #191c18;

	--action-primary: #f2a541;
	--action-primary-hover: color-mix(in srgb, #f2a541 82%, #ffffff);
	--action-secondary: #b8441f;
	--action-accent: #f2a541;

	--border-soft: rgba(245, 222, 165, 0.16);
	--border-warm: rgba(241, 205, 112, 0.28);
	--overlay: rgba(0, 0, 0, 0.62);
}
```

Dark mode 使用規則：

- 不使用純黑背景，使用森林夜色 `#0E1C1A`。
- 主 CTA 改用暖橘黃 `#F2A541` 搭配深色文字，確保辨識度。
- 深色卡片使用 `#1A2E25`、`#213A2E`，不要所有區塊都同一色。
- Logo 優先使用 `logo-horizontal-light.svg`。
- 邊框與陰影降低存在感，靠層級色和留白區分。

### 3.5 現有專案顏色落差

共享 UI kit 的 `--hkui-primary` 是磚橘 `#B8441F`；`--hkui-primary-*` 色階與 `--hkui-secondary-*` 色階是由品牌色票推導的定值。新程式不要再自訂另一組相近的品牌色；每個 feature CSS 只吃 semantic token，例如 `--action-primary`、`--surface-card`、`--text-body`。

## 4. 字體系統

### 4.1 品牌字體

| 語系 | 字體 | 用途 | Line height | Spacing |
| --- | --- | --- | --- | --- |
| 中文 | 源柔ゴシックP Medium | 中文標題 | 140% | 10% |
| 中文 | 源柔ゴシックP Normal | 中文內文 | 180% | 20% |
| 英文 | Axiforma Book | 英文標題 | 110% | 0% |
| 英文 | Avenir Next Regular | 英文內文與數字 | 150% | 2% |

### 4.2 Web 字體 fallback

源柔ゴシックP、Axiforma 不一定能直接用在網頁，建議：

```css
:root {
	--font-zh: "Gen Jyuu Gothic P", "Noto Sans TC", system-ui, sans-serif;
	--font-en: "Avenir Next", "Axiforma", "Inter", system-ui, sans-serif;
	--font-ui: "Inter", "Noto Sans TC", system-ui, -apple-system, sans-serif;
	--font-display: "Avenir Next", "Noto Sans TC", system-ui, sans-serif;
}
```

若沒有授權可上 web font：

- UI 先維持 `Inter + Noto Sans TC`。
- 品牌頁、行銷頁可再評估採購或 self-host 指定字體。
- 數字金額可使用 `Avenir Next` fallback，沒有就走 system。

### 4.3 Web typography scale

| Token | Size | Line height | 用途 |
| --- | ---: | ---: | --- |
| `--text-xs` | 12px | 16px | badge、時間、輔助說明 |
| `--text-sm` | 14px | 20px | 表單提示、卡片次資訊 |
| `--text-base` | 16px | 24px | UI 內文、表單、一般內容 |
| `--text-lg` | 18px | 28px | 商品卡標題、重要列表 |
| `--text-xl` | 20px | 28px | 區塊小標 |
| `--display-sm` | 24px | 32px | 卡片群組標題、Modal title |
| `--display-md` | 30px | 40px | 頁面標題 |
| `--display-lg` | 36px | 44px | 大型頁面標題 |
| `--display-xl` | 48px | 56px | Hero headline |
| `--display-2xl` | 60px | 66px | 大型品牌頁，少用 |

### 4.4 字距規則

- 一般 UI 不使用負字距，維持 `letter-spacing: 0`。
- 中文品牌標題可使用 `0.04em` 到 `0.10em`，但只用於 Hero、品牌段落、海報感標題。
- 中文長內文不要用 20% 字距照搬到 UI，會太散；Web 內文建議 `0` 到 `0.02em`。
- 英文與數字維持 `0`，小字可用 `0.02em` 增加辨識。

## 5. 間距與版面

### 5.1 4px base spacing

現有 `styles/components/ui-kit.css` 已經有完整 `--hkui-space-*`，建議繼續使用。

| Token | px | 用途 |
| --- | ---: | --- |
| `--space-1` | 4 | icon 與文字極小間距 |
| `--space-2` | 8 | badge、button icon gap、緊湊列表 |
| `--space-3` | 12 | 表單內距、卡片內小間隔 |
| `--space-4` | 16 | 一般元件間距 |
| `--space-5` | 20 | 卡片 padding mobile |
| `--space-6` | 24 | 卡片 padding desktop、section 內距 |
| `--space-8` | 32 | 區塊間距、小頁面 gutter |
| `--space-10` | 40 | 大區塊內距 |
| `--space-12` | 48 | section spacing |
| `--space-16` | 64 | page section spacing |
| `--space-20` | 80 | hero 或大型 landing section |
| `--space-24` | 96 | 大型品牌頁留白 |

### 5.2 Page layout

| 裝置 | Page gutter | Content max width | 建議 |
| --- | ---: | ---: | --- |
| Mobile | 16px 到 20px | 100% | 卡片單欄、避免左右滿版貼邊 |
| Tablet | 24px 到 32px | 960px | 列表可 2 欄 |
| Desktop | 40px 到 48px | 1320px 到 1440px | 市集、商品列表、訂單工作台 |
| Wide desktop | 48px 到 64px | 1440px | 不要讓文字行寬無限延展 |

### 5.3 Component spacing

- Button：
  - height 40 / 44 / 48
  - padding-x 16 / 20 / 24
  - icon gap 8
- Input：
  - height 44 / 48
  - padding-x 16
  - label gap 8
- Card：
  - compact padding 16
  - normal padding 20 到 24
  - dashboard padding 24 到 32
- Section：
  - mobile vertical 40 到 56
  - desktop vertical 64 到 96

## 6. Radius、陰影與邊框

目前專案已經同時有 `--brand-radius-*` 和 `--hkui-radius-*`。建議統一語意：

| Token | Value | 用途 |
| --- | ---: | --- |
| `--radius-sm` | 8px | small button、chip、input |
| `--radius-md` | 12px | 一般 control、filter pill |
| `--radius-lg` | 16px | 小卡片、商品內圖 |
| `--radius-xl` | 20px | 一般卡片、dropdown |
| `--radius-2xl` | 28px | 大卡片、modal panel |
| `--radius-3xl` | 36px | 品牌型大面板 |
| `--radius-full` | 9999px | pill、avatar、icon button |

使用規則：

- 工具型 UI、表格、工作台卡片不要過度圓角，8 到 16px 即可。
- 行銷與品牌區塊可以使用 24 到 36px，但不要卡片套卡片。
- Modal 和大型空狀態可以使用 28px。
- 邊框優先使用 `rgba(31, 41, 51, 0.12)` 這類低對比墨色，不用灰黑粗線。

陰影建議：

```css
:root {
	--shadow-sm: 0 2px 8px rgba(51, 76, 15, 0.05);
	--shadow-md: 0 8px 24px rgba(51, 76, 15, 0.08);
	--shadow-lg: 0 16px 42px rgba(51, 76, 15, 0.12);
	--shadow-modal: 0 24px 70px rgba(15, 33, 15, 0.22);
}

[data-theme="dark"] {
	--shadow-sm: 0 2px 8px rgba(0, 0, 0, 0.28);
	--shadow-md: 0 10px 28px rgba(0, 0, 0, 0.36);
	--shadow-lg: 0 18px 48px rgba(0, 0, 0, 0.46);
	--shadow-modal: 0 28px 80px rgba(0, 0, 0, 0.58);
}
```

## 7. 彈出視窗與對話框

現有專案已有多套 modal：

- `components/feed/PostDetailModal.tsx` + `.post-modal`
- `components/seller/ShipmentDialogs.tsx` + `.shipment-dialog`
- `app/contact/page.tsx` 的 `.dialog-overlay` / `.dialog-card`
- legacy `.dialog` in `styles/components/decorations-overlays.css`

建議之後建立一套共用 Dialog primitive，再逐步替換。

### 7.1 Dialog 結構

```tsx
<Dialog>
	<DialogBackdrop />
	<DialogPanel>
		<DialogHeader />
		<DialogBody />
		<DialogFooter />
	</DialogPanel>
</Dialog>
```

### 7.2 Dialog 視覺規格

| 部位 | Light mode | Dark mode |
| --- | --- | --- |
| Backdrop | `rgba(15, 33, 15, 0.42)` + blur 4 到 8px | `rgba(0, 0, 0, 0.62)` + blur 4 到 8px |
| Panel bg | `#FFFDF7` 或 `--surface-card` | `#1A2E25` |
| Panel border | `rgba(51, 76, 15, 0.12)` | `rgba(245, 222, 165, 0.16)` |
| Radius | 24 到 28px | 24 到 28px |
| Shadow | `--shadow-modal` | `--shadow-modal` |
| Header padding | 24px 24px 16px | 同 light |
| Body padding | 0 24px 24px | 同 light |
| Footer padding | 16px 24px 24px | 同 light |

### 7.3 Dialog size

| Size | Width | 用途 |
| --- | ---: | --- |
| `sm` | 420px | 確認、簡短表單 |
| `md` | 560px | 一般建立/編輯 |
| `lg` | 720px | 多欄資訊、圖片 |
| `xl` | 880px | 地圖、複雜流程 |
| `fullscreen-mobile` | mobile only | 聊天、貼文詳情、地圖 |

### 7.4 Dialog 行為規則

- 所有 modal 必須有 `role="dialog"` 或 `role="alertdialog"`。
- 必須有可辨識 title，使用 `aria-labelledby`。
- Esc 可關閉，除非是不可中斷付款流程。
- Close button 固定右上，尺寸 32 到 40px。
- Footer action 排序：
  - 取消 / 次要動作在左或前。
  - 確認 / 主要動作在右或後。
- Pop-up 中的 icon 遵守品牌規範：小尺寸、搭配短文字、背景乾淨，不當大型裝飾。

### 7.5 Modal 收斂建議

短期：

- 保留現有 class，但把 overlay、panel bg、radius、shadow 改吃共用 token。
- 新增 `.hk-dialog-*` tokens，不急著改 JSX。

中期：

- `ShipmentDialogs` 改用共用 `<DialogShell>`。
- `PostDetailModal` 因為是 media viewer，可以保留特殊 layout，但 overlay、close button、mobile fullscreen 規格一致。

長期：

- 刪除 legacy `.dialog`。
- Contact success dialog 改用同一套 Dialog。

## 8. 圖像、Pattern 與 Icon

### 8.1 Key Visual Pattern

品牌規範建議把農作植物做成簡約塊面剪影，搭配有機弧度與大地色。

適合：

- 網頁 Header 背景
- 社群 Banner
- 包裝袋
- 展場形象牆
- 大面積品牌區塊

規則：

- 可滿版出血，或大面積留白加局部色塊。
- 不在 pattern 上壓複雜文字。
- 若文字必須放上去，需有足夠留白或半透明乾淨底。

### 8.2 Narrative scene

適合：

- 產地故事
- 關於我們
- 活動頁
- 空狀態大型插圖

規則：

- 保留足夠呼吸空間。
- 不任意改插圖色彩。
- 不把插圖拿來當純裝飾填滿整頁。

### 8.3 Spot illustration

適合：

- 小型提示
- 單一訊息區塊
- onboarding step

規則：

- 四周至少保留 `1.5x` 圖案寬度的呼吸空間。
- 放在米色或白色純色背景。
- 不滿版放大。

### 8.4 System icons

品牌規範指定 icon 要有手繪線條與溫度，但在前端操作性 UI 中，也要維持辨識度。

建議：

- 功能按鈕的圖示一律用 `AppIcon`（Material Symbols）。
- 品牌或流程插圖 icon 可用手繪風 SVG。
- Navbar、表單、工作台不要用過度裝飾 icon。
- icon button 尺寸 36 / 40 / 44px。
- icon 與文字 gap 8px。
- tooltip 用於只有 icon 的 unfamiliar action。

### 8.5 圖示強調色

少數圖示用 CSS 變數取品牌強調色，集中設定位置：

```css
:root {
	--icon-accent: var(--color-brand-primary-strong);
	--icon-accent-secondary: var(--color-brand-secondary);
}

[data-theme="dark"] {
	--icon-accent: var(--color-brand-secondary);
	--icon-accent-secondary: var(--color-brand-primary);
}
```

使用規則：

- 產品圖示一律走 `AppIcon`，顏色預設 `currentColor`；需要強調色時用 `var(--icon-accent)`。
- 深色模式下強調色改用暖橘黃加品牌橘，避免深色在深色背景上看不清楚。

## 9. 元件準則

### 9.1 Button

| Variant | 背景 | 文字 | 用途 |
| --- | --- | --- | --- |
| Primary | `--action-primary` | `--text-inverse` | 加入購物車、送出、付款 |
| Secondary | transparent / soft surface | `--action-primary` | 次要操作、返回 |
| Accent | `--color-brand-secondary` | `#191C18` | 優惠、活動、深色模式主 CTA |
| Danger | `#EA6021` | white | 刪除、取消、錯誤修正 |

Button 規則：

- 文字不可爆版，窄版使用 icon-only 或縮短文案。
- icon + text gap 固定 8px。
- 同一群 action 裡只放一個 primary。

### 9.2 Card

Card 用途：

- 商品卡
- 訂單卡
- 賣家資訊卡
- 內容列表 item

Card 規則：

- 不要 card 裡再包整個 card。
- 工作台型卡片保持 12 到 16px radius。
- 商品卡可 16 到 20px radius。
- 卡片 hover 使用 border + shadow 小幅變化，不要大幅跳動。

### 9.3 Form

Form 規則：

- Label 14px / 600，body color。
- Input height 44 到 48px。
- Focus ring 使用 `--color-brand-primary-strong` 或 `--action-primary`，不要藍紫預設。
- Error 使用 `#EA6021`，搭配文字，不只依賴顏色。
- Helper text 12 到 14px。

### 9.4 Badge / Chip

配送溫層使用 [跨平台契約](../design-system/cross-platform.md) 的 `requested-web-legacy-parity` 例外：Web 與兩 App 共用透明細框、11px 字級與18px基準膠囊，保留文字縮放；不套用下列一般 Badge 的高度及淡底建議，舊色票的對比限制需三端一起調整。

用途：

- 溫層：冷凍、冷藏、常溫
- hashtag
- 訂單狀態
- 賣家驗證

建議：

- 高度 24 到 32px。
- radius full。
- padding-x 8 到 12px。
- 狀態色要淡底深字。
- `#E4572E` 可作 icon 或 border，文字建議磚橘 `#B8441F`。

## 10. 現有專案整理與建議

### 10.1 已經做得好的地方

- Logo assets 已經在 `public/brand/`，且顏色符合 Hawkr 色票。
- `components/shared/BrandLogo.tsx` 已封裝 horizontal / mark / grass，深色模式也有 light logo swap。
- `styles/components/ui-kit.css` 已有完整 spacing、font-size、radius、shadow token。
- `styles/foundation/01-tokens-theme.css` 已有 light / dark mode 基礎。
- 最近 CSS 已拆分，`styles/globals.css` 現在適合只作 import manifest。

### 10.2 最大問題

- `styles/foundation/00-color-config.css` 已是正式色與 light/dark semantic role 的單一入口；「尚未建立正式 token」不再是現況。
- 部分 feature 與元件仍直接使用 `#2e5e2e`、`#269674`、`#99a670`、`#ee964b` 等舊 literal。它們是待遷移 drift，不是第二套可繼續使用的 palette。
- theme action 在 dark mode 轉為暖橘黃，但共享 `Button` 的 `--hkui-primary` 仍維持磚橘；兩者尚未統一。
- foundation 與 UI kit 各有一套正在使用的 radius scale；不可假裝已合併，也不可新增第三套。
- Dialog overlay／z-index、navigation chrome 與 Logo 預設 drop-shadow 仍有 hardcode 或品牌規範落差，應逐項收斂。

### 10.3 建議落地順序

1. 新介面只讀 root `DESIGN.md`、現有 semantic token 與共享元件；不再新增 literal 品牌色。
2. 以實際 route 為單位清理 legacy literals，逐頁做 light/dark 與視覺回歸，不進行一次性全域替換。
3. 明確決定 dark mode 的 theme action 與 UI-kit primary 是否同一行為，再改 token；在此之前保留並標示差異。
4. 讓 Dialog overlay／z-index、navigation chrome 與 Logo surface 回到 shared token 或正式品牌規範。
5. 半徑收斂須先定 component role mapping，再逐元件遷移；禁止只做機械數字替換。

## 11. Claude 後續改版注意事項

給 Claude 或其他協作者的規則：

- 新增頁面、聊天室、modal、卡片時，不要直接寫 HEX，先找 token。
- 若 token 不存在，先加 semantic token，不要在 feature CSS 裡散落新色。
- Logo 一律用 `BrandLogo` component 或 `public/brand` SVG。
- 新增 modal 要遵守第 7 節，不要再做新的 overlay / panel 風格。
- 深色模式要同時確認 `data-theme="dark"`。
- 商品、聊天室、訂單這類工具頁避免行銷式大卡片堆疊，保持資訊清楚。
- 背景 pattern 只用在品牌感區塊，不要放在表單或大量文字後方。

## 12. Checklist

新增或改版 UI 前檢查：

- [ ] 是否使用品牌色票或 semantic token？
- [ ] 是否有 light / dark mode 對應？
- [ ] Logo 是否有足夠安全距離？
- [ ] 文字是否有足夠對比？
- [ ] 字級是否符合 typography scale？
- [ ] 間距是否使用 4px scale？
- [ ] Modal 是否符合共用 Dialog 規格？
- [ ] icon 是否用於操作引導，而不是單純裝飾？
- [ ] mobile 版文字是否不會溢出？
- [ ] 有沒有 card 包 card？
