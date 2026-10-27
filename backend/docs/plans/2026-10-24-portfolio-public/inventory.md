# 作品集轉 public 的盤點（步驟 1 附件）

快照 commit：e5d5a8d8e（程式碼與 332da5e51 相同；之後只新增了計畫與 ADR）。由 `gen_inventory.py` 依 `scan.py` 的規則產生；每個表格的每一列都有「去向」。
路徑裡的舊名、縮寫與同事代號一律換成代號（`<舊名>`、`<縮寫>-`、`<同事-1>` 等）；用 `scan.py --resolve '<代號化路徑>'` 還原成真實路徑。
「待 Will 確認」表的列都附有預設去向，Will 沒有回覆前依預設執行；問題代號見該表。

## 已決

- 2026-10-24（Will）
  - S1：`gateway` 文件檢視器寫死的憑證與簽章 key → 步驟 2 改由環境變數注入，沒有值時停用檢視器。
  - S2：同一個 32 字元、形狀像代理金鑰的值（出現在 11 個測試檔與測試設定），以及 7 個檔的十六進位值 → 視為舊公司的真實憑證，步驟 6 換成虛構值。
- 2026-10-25（Will）：Q-A9 子品牌 `<舊名-子品牌>` 與其中文寫法屬於舊名，一併改（計畫 D9；步驟 4 改中文字首的衍生名，英文識別碼歸 5f）。
- 2026-10-26（Will）：Q-A1 到 Q-A8 全部照預設（計畫 D12），去向已寫進第 1 節各列：
  - Q-A1 舊品牌吉祥物與子品牌的角色素材 → 全部刪除，換 `brand/hawkr/` 圖形或佔位 SVG（5c）。
  - Q-A2 Iconly 圖示 → 刪除，換 Material 系圖示（App 用 Flutter 內建 `Icons`，website 與 admin 用 Material Symbols 的 SVG；5d）。
  - Q-A3 設計稿匯出的 UI 圖示、插畫、截圖與介面圖 → UI 圖示刪除並換 Material 系（5d）；照片與插畫換程式產生的佔位 SVG（D2；5e）；Storybook 的極小圖形保留（5a）。
  - Q-A4 官方認證標章與第三方商標 → 刪除，改文字標籤（5a）。
  - Q-A5 jf-openhuninn 衍生字型 → 保留，家族名改成 Hawkr 的名字並重算 manifest，授權文件與 notice 照 OFL 保留（5b）。
  - Q-A6 影片（mp4、webm） → 刪除，不重錄（5a）。
  - Q-A7 Lottie 動畫 → 刪除（5c）。
  - Q-A8 QR 碼圖片 → 刪除原圖，換合成的 QR（編碼 `https://example.com`；5a）。

## 待 Will 確認的問題

| 代號 | 問題 | 預設 |
|---|---|---|
| Q-P1 | 測試資料裡通過檢查碼的統一編號與身分證字號：是合成值還是真實業者的號碼（repo 無法判定） | 一律換成合成值，並在 `scan.py` 寫入允許的合成值清單 |
| Q-P2 | 疑似真實業者的 email 網域、商號、農場名稱（denylist 的 uncertain 列）是否真有其人 | 換成虛構名稱 |
| Q-D1 | 設計書與規範裡出現文件作者本人（Will）的全名一處：是否保留 | 保留（Will 自己的名字） |

## 1. 素材逐檔

- 數量：216 個檔（計畫的 regex 涵蓋 129 個；另外納入字型 woff/woff2、影片 mp4/webm、Lottie JSON 共 87 個）。
- 產生計數的指令：`git ls-files | grep -E -i '\.(png|jpe?g|webp|svg|ico|gif|ttf|otf|pdf)$' | wc -l`（計畫範圍），`python3 backend/docs/plans/2026-10-24-portfolio-public/scan.py --category assets --report`（含擴充型態；沒有出處的檔記成「來源不明」而命中，步驟 5 完成後應為 0）。
- 快照：e5d5a8d8e（程式碼與 332da5e51 相同；之後只新增了計畫與 ADR）；本節的路徑與去向在步驟 5a 對齊到 `bfbefa5f4b` 之後的樹（步驟 3 改名的字型與素材、步驟 4 由 `brand/hawkr/` 產生的檔都用現在的路徑）。
- 來源的依據：首次 commit 的作者（git 歷史）、SVG 的生成器註解、同目錄的 README 與 manifest、檔名與目錄語意；沒有授權文件的一律記成「來源不明」。像素內容是否含舊名只抽樣目視，無法機械掃描；`含舊名` 欄只標路徑或 SVG 文字命中。
- 「引用檔數」是其他追蹤中的文字檔含該檔名的檔數（以檔名比對，同名檔會合併計算）；步驟 5 刪除前後都要重跑，刪除後必須是 0。

### G01 新品牌素材（8 檔，36 KB）

依據：Will 於 2026-10-23 新增的新品牌素材。型態 svg 8；這組檔案的 commit 次數：Will 15、他人 0。

| 素材 | 型態 | KB | 尺寸 | 來源 | 含舊名 | 引用檔數 | 去向 |
|---|---|---|---|---|---|---|---|
| `brand/hawkr/hawkr-app-icon.svg` | svg | 1 | - | 自製 | 否／像素未掃 | 3 | 保留（新品牌素材，brand/hawkr/README.md；首次 commit 為 Will） |
| `brand/hawkr/hawkr-favicon.svg` | svg | 1 | - | 自製 | 否／像素未掃 | 3 | 保留（新品牌素材，brand/hawkr/README.md；首次 commit 為 Will） |
| `brand/hawkr/hawkr-lockup-dark.svg` | svg | 8 | - | 自製 | 否／像素未掃 | 4 | 保留（新品牌素材，brand/hawkr/README.md；首次 commit 為 Will） |
| `brand/hawkr/hawkr-lockup.svg` | svg | 8 | - | 自製 | 否／像素未掃 | 4 | 保留（新品牌素材，brand/hawkr/README.md；首次 commit 為 Will） |
| `brand/hawkr/hawkr-mark-dark.svg` | svg | 1 | - | 自製 | 否／像素未掃 | 2 | 保留（新品牌素材，brand/hawkr/README.md；首次 commit 為 Will） |
| `brand/hawkr/hawkr-mark.svg` | svg | 1 | - | 自製 | 否／像素未掃 | 5 | 保留（新品牌素材，brand/hawkr/README.md；首次 commit 為 Will） |
| `brand/hawkr/hawkr-wordmark-dark.svg` | svg | 8 | - | 自製 | 否／像素未掃 | 3 | 保留（新品牌素材，brand/hawkr/README.md；首次 commit 為 Will） |
| `brand/hawkr/hawkr-wordmark.svg` | svg | 8 | - | 自製 | 否／像素未掃 | 5 | 保留（新品牌素材，brand/hawkr/README.md；首次 commit 為 Will） |

### G02 App 圖示、啟動圖與 favicon（41 檔，130 KB）

依據：AppIcon、favicon 原本是舊品牌 logo 的輸出尺寸；步驟 4 已由 `brand/hawkr/` 的 SVG 重新產生各尺寸。啟動圖與底色是 1 到 4 像素的單色或透明圖，沒有創作性。型態 ico 1, png 40；這組檔案的 commit 次數：Will 50、他人 67。

| 素材 | 型態 | KB | 尺寸 | 來源 | 含舊名 | 引用檔數 | 去向 |
|---|---|---|---|---|---|---|---|
| `admin/public/favicon.png` | png | 37 | 1400x1400 | 自製（`brand/hawkr/`） | 否／像素未掃 | 0 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `admin/src/app/favicon.ico` | ico | 7 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 5 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/LaunchBackground.imageset/background.png` | png | 1 | 1x1 | 自製（無創作性：1x1 的單色或透明圖） | 否／像素未掃 | 2 | 保留 |
| `buyer-app/ios/Runner/Assets.xcassets/LaunchBackground.imageset/darkbackground.png` | png | 1 | 1x1 | 自製（無創作性：1x1 的單色或透明圖） | 否／像素未掃 | 2 | 保留 |
| `buyer-app/ios/Runner/Assets.xcassets/LaunchImage.imageset/LaunchImage.png` | png | 1 | 1x1 | 自製（無創作性：1x1 的單色或透明圖） | 否／像素未掃 | 3 | 保留 |
| `buyer-app/ios/Runner/Assets.xcassets/LaunchImage.imageset/LaunchImage@2x.png` | png | 1 | 2x2 | 自製（無創作性：2x2 的單色或透明圖） | 否／像素未掃 | 3 | 保留 |
| `buyer-app/ios/Runner/Assets.xcassets/LaunchImage.imageset/LaunchImage@3x.png` | png | 1 | 3x3 | 自製（無創作性：3x3 的單色或透明圖） | 否／像素未掃 | 3 | 保留 |
| `buyer-app/ios/Runner/Assets.xcassets/LaunchImage.imageset/LaunchImageDark.png` | png | 1 | 1x1 | 自製（無創作性：1x1 的單色或透明圖） | 否／像素未掃 | 2 | 保留 |
| `buyer-app/ios/Runner/Assets.xcassets/LaunchImage.imageset/LaunchImageDark@2x.png` | png | 1 | 2x2 | 自製（無創作性：2x2 的單色或透明圖） | 否／像素未掃 | 2 | 保留 |
| `buyer-app/ios/Runner/Assets.xcassets/LaunchImage.imageset/LaunchImageDark@3x.png` | png | 1 | 3x3 | 自製（無創作性：3x3 的單色或透明圖） | 否／像素未掃 | 2 | 保留 |
| `buyer-app/ios/Runner/Assets.xcassets/devAppIcon.appiconset/Icon-App-1024x1024@1x.png` | png | 12 | 1024x1024 | 自製（`brand/hawkr/`） | 否／像素未掃 | 6 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-1024x1024@1x.png` | png | 10 | 1024x1024 | 自製（`brand/hawkr/`） | 否／像素未掃 | 6 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-20x20@1x.png` | png | 1 | 20x20 | 自製（`brand/hawkr/`） | 否／像素未掃 | 1 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-20x20@2x.png` | png | 1 | 40x40 | 自製（`brand/hawkr/`） | 否／像素未掃 | 1 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-20x20@3x.png` | png | 1 | 60x60 | 自製（`brand/hawkr/`） | 否／像素未掃 | 1 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-29x29@1x.png` | png | 1 | 29x29 | 自製（`brand/hawkr/`） | 否／像素未掃 | 1 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-29x29@2x.png` | png | 1 | 58x58 | 自製（`brand/hawkr/`） | 否／像素未掃 | 1 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-29x29@3x.png` | png | 1 | 87x87 | 自製（`brand/hawkr/`） | 否／像素未掃 | 1 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-40x40@1x.png` | png | 1 | 40x40 | 自製（`brand/hawkr/`） | 否／像素未掃 | 1 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-40x40@2x.png` | png | 1 | 80x80 | 自製（`brand/hawkr/`） | 否／像素未掃 | 1 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-40x40@3x.png` | png | 1 | 120x120 | 自製（`brand/hawkr/`） | 否／像素未掃 | 1 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-60x60@2x.png` | png | 1 | 120x120 | 自製（`brand/hawkr/`） | 否／像素未掃 | 1 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-60x60@3x.png` | png | 1 | 180x180 | 自製（`brand/hawkr/`） | 否／像素未掃 | 1 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-76x76@1x.png` | png | 1 | 76x76 | 自製（`brand/hawkr/`） | 否／像素未掃 | 1 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-76x76@2x.png` | png | 1 | 152x152 | 自製（`brand/hawkr/`） | 否／像素未掃 | 1 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-83.5x83.5@2x.png` | png | 1 | 167x167 | 自製（`brand/hawkr/`） | 否／像素未掃 | 1 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/ios/Runner/Assets.xcassets/stageAppIcon.appiconset/Icon-App-1024x1024@1x.png` | png | 13 | 1024x1024 | 自製（`brand/hawkr/`） | 否／像素未掃 | 6 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `seller-app/ios/Runner/Assets.xcassets/LaunchBackground.imageset/background.png` | png | 1 | 1x1 | 自製（無創作性：1x1 的單色或透明圖） | 否／像素未掃 | 2 | 保留 |
| `seller-app/ios/Runner/Assets.xcassets/LaunchBackground.imageset/darkbackground.png` | png | 1 | 1x1 | 自製（無創作性：1x1 的單色或透明圖） | 否／像素未掃 | 2 | 保留 |
| `seller-app/ios/Runner/Assets.xcassets/LaunchImage.imageset/LaunchImage.png` | png | 1 | 1x1 | 自製（無創作性：1x1 的單色或透明圖） | 否／像素未掃 | 3 | 保留 |
| `seller-app/ios/Runner/Assets.xcassets/LaunchImage.imageset/LaunchImage@2x.png` | png | 1 | 2x2 | 自製（無創作性：2x2 的單色或透明圖） | 否／像素未掃 | 3 | 保留 |
| `seller-app/ios/Runner/Assets.xcassets/LaunchImage.imageset/LaunchImage@3x.png` | png | 1 | 3x3 | 自製（無創作性：3x3 的單色或透明圖） | 否／像素未掃 | 3 | 保留 |
| `seller-app/ios/Runner/Assets.xcassets/LaunchImage.imageset/LaunchImageDark.png` | png | 1 | 1x1 | 自製（無創作性：1x1 的單色或透明圖） | 否／像素未掃 | 2 | 保留 |
| `seller-app/ios/Runner/Assets.xcassets/LaunchImage.imageset/LaunchImageDark@2x.png` | png | 1 | 2x2 | 自製（無創作性：2x2 的單色或透明圖） | 否／像素未掃 | 2 | 保留 |
| `seller-app/ios/Runner/Assets.xcassets/LaunchImage.imageset/LaunchImageDark@3x.png` | png | 1 | 3x3 | 自製（無創作性：3x3 的單色或透明圖） | 否／像素未掃 | 2 | 保留 |
| `seller-app/ios/Runner/Assets.xcassets/devAppIcon.appiconset/Icon-App-1024x1024@1x.png` | png | 12 | 1024x1024 | 自製（`brand/hawkr/`） | 否／像素未掃 | 6 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `seller-app/ios/Runner/Assets.xcassets/devLaunchImage.imageset/LaunchImage.png` | png | 1 | 1x1 | 自製（無創作性：1x1 的單色或透明圖） | 否／像素未掃 | 3 | 保留 |
| `seller-app/ios/Runner/Assets.xcassets/devLaunchImage.imageset/LaunchImage@2x.png` | png | 1 | 1x1 | 自製（無創作性：1x1 的單色或透明圖） | 否／像素未掃 | 3 | 保留 |
| `seller-app/ios/Runner/Assets.xcassets/devLaunchImage.imageset/LaunchImage@3x.png` | png | 1 | 1x1 | 自製（無創作性：1x1 的單色或透明圖） | 否／像素未掃 | 3 | 保留 |
| `seller-app/ios/Runner/Assets.xcassets/prodAppIcon.appiconset/Icon-App-1024x1024@1x.png` | png | 10 | 1024x1024 | 自製（`brand/hawkr/`） | 否／像素未掃 | 6 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `seller-app/ios/Runner/Assets.xcassets/stageAppIcon.appiconset/Icon-App-1024x1024@1x.png` | png | 13 | 1024x1024 | 自製（`brand/hawkr/`） | 否／像素未掃 | 6 | 保留（步驟 4 由 `brand/hawkr/` 產生） |

### G03 舊品牌 logo 與字標（30 檔，275 KB）

依據：原本是舊品牌的 logo、字標與啟動圖（來源不明，沒有授權文件）；步驟 4 已由 `brand/hawkr/` 重新產生的檔留下，其餘在 5a 刪除並改指向 Hawkr 圖（清單在本節之後的「已刪除的素材」）。留下的檔不是 `brand/hawkr/` 的輸出，就是沒有創作性的透明圖。型態 png 9, svg 21；這組檔案的 commit 次數：Will 31、他人 29。

| 素材 | 型態 | KB | 尺寸 | 來源 | 含舊名 | 引用檔數 | 去向 |
|---|---|---|---|---|---|---|---|
| `admin/public/brand/logo-horizontal-light.svg` | svg | 8 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 9 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `admin/public/brand/logo-horizontal.svg` | svg | 8 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 11 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `admin/public/brand/logo-mark.svg` | svg | 1 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 16 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/assets/brand/logo-grass.svg` | svg | 1 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 7 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/assets/brand/logo-horizontal-light.svg` | svg | 8 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 9 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/assets/brand/logo-horizontal.svg` | svg | 8 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 11 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/assets/brand/logo-mark.svg` | svg | 1 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 16 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/assets/brand/splash_mark.png` | png | 19 | 960x960 | 自製（`brand/hawkr/`） | 否／像素未掃 | 2 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/assets/brand/splash_mark_android12.png` | png | 26 | 1152x1152 | 自製（`brand/hawkr/`） | 否／像素未掃 | 1 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/assets/brand/splash_mark_wheat.png` | png | 19 | 960x960 | 自製（`brand/hawkr/`） | 否／像素未掃 | 2 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/assets/brand/splash_transparent.png` | png | 1 | 4x4 | 自製（無創作性：4x4 的單色或透明圖） | 否／像素未掃 | 1 | 保留 |
| `buyer-app/assets/brand/splash_transparent.svg` | svg | 1 | - | 自製（無創作性：只有一個矩形的 SVG） | 否／像素未掃 | 0 | 保留 |
| `buyer-app/assets/brand/wordmark.svg` | svg | 8 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 8 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `buyer-app/assets/hawkrlogo.png` | png | 32 | 1024x1024 | 自製（`brand/hawkr/`） | 否／像素未掃 | 9 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `seller-app/assets/brand/logo-four-color.png` | png | 6 | 380x379 | 自製（`brand/hawkr/`） | 否／像素未掃 | 2 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `seller-app/assets/brand/logo-four-color.svg` | svg | 1 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 4 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `seller-app/assets/brand/logo-grass.svg` | svg | 1 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 7 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `seller-app/assets/brand/logo-horizontal-light.svg` | svg | 8 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 9 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `seller-app/assets/brand/logo-horizontal.svg` | svg | 8 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 11 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `seller-app/assets/brand/logo-mark.svg` | svg | 1 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 16 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `seller-app/assets/brand/splash_mark.png` | png | 19 | 960x960 | 自製（`brand/hawkr/`） | 否／像素未掃 | 2 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `seller-app/assets/brand/splash_mark_wheat.png` | png | 19 | 960x960 | 自製（`brand/hawkr/`） | 否／像素未掃 | 2 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `seller-app/assets/brand/wordmark.svg` | svg | 8 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 8 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `seller-app/assets/hawkrlogo.png` | png | 32 | 1024x1024 | 自製（`brand/hawkr/`） | 否／像素未掃 | 9 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `website/public/brand/app-icon.svg` | svg | 1 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 3 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `website/public/brand/logo-grass.svg` | svg | 1 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 7 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `website/public/brand/logo-horizontal-light.svg` | svg | 8 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 9 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `website/public/brand/logo-horizontal.svg` | svg | 8 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 11 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `website/public/brand/logo-mark.svg` | svg | 1 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 16 | 保留（步驟 4 由 `brand/hawkr/` 產生） |
| `website/public/brand/wordmark.svg` | svg | 8 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 8 | 保留（步驟 4 由 `brand/hawkr/` 產生） |

### G07 字型（88 檔，3278 KB）

依據：字型的授權與重新生成方式見逐檔「來源」欄。型態 ttf 1, woff2 87；這組檔案的 commit 次數：Will 86、他人 2。

| 素材 | 型態 | KB | 尺寸 | 來源 | 含舊名 | 引用檔數 | 去向 |
|---|---|---|---|---|---|---|---|
| `admin/src/app/fonts/hawkr-rounded-2.0-subset.woff2` | woff2 | 474 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 5 | 保留＋THIRD_PARTY_NOTICES |
| `admin/src/app/fonts/roboto-latin.woff2` | woff2 | 36 | - | 開源（Roboto，OFL-1.1；5b 依檔內 name table 驗證） | 否／像素未掃 | 5 | 保留＋THIRD_PARTY_NOTICES |
| `backend/services/invoice-service/internal/domain/testdata/NotoSansTC-subset.ttf` | ttf | 44 | - | 開源（Noto Sans CJK TC，OFL-1.1；檔內 name table 無授權欄，依上游判斷） | 否／像素未掃 | 3 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-base-2c7daebd9ec7.woff2` | woff2 | 118 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-000-a64091e6e9aa.woff2` | woff2 | 11 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-001-86d9a1b18ca4.woff2` | woff2 | 12 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-002-45eff1405223.woff2` | woff2 | 11 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-003-72255d385abd.woff2` | woff2 | 18 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-004-d439cb109210.woff2` | woff2 | 25 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-005-ba6d06639413.woff2` | woff2 | 29 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-006-a3ce4e8080eb.woff2` | woff2 | 22 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-007-fe311a13bde8.woff2` | woff2 | 28 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-008-794c2eaa3ec7.woff2` | woff2 | 33 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-009-e85c3133ea48.woff2` | woff2 | 26 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-010-0ac101b43745.woff2` | woff2 | 28 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-011-5155b4e77501.woff2` | woff2 | 24 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-012-2fec08595018.woff2` | woff2 | 26 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-013-d8cfc9ebe053.woff2` | woff2 | 31 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-014-f05c57cf1395.woff2` | woff2 | 35 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-015-7caf600b33d8.woff2` | woff2 | 26 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-016-a8884fe0373d.woff2` | woff2 | 32 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-017-8f51740543f7.woff2` | woff2 | 30 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-018-b52ddb33ddad.woff2` | woff2 | 38 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-019-3b8f7b7959b6.woff2` | woff2 | 28 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-020-55d63e9582b4.woff2` | woff2 | 29 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-021-56b498817db5.woff2` | woff2 | 30 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-022-cd401915e966.woff2` | woff2 | 31 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-023-c9a2028710b8.woff2` | woff2 | 29 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-024-f13ac2ddacec.woff2` | woff2 | 37 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-025-2dad141ad616.woff2` | woff2 | 30 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-026-2f61662158b2.woff2` | woff2 | 30 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-027-892ab977e721.woff2` | woff2 | 36 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-028-34ac4615d51b.woff2` | woff2 | 41 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-029-56b0c94a07bc.woff2` | woff2 | 28 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-030-b4ab3cdd5589.woff2` | woff2 | 29 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-031-c2ded2aca1aa.woff2` | woff2 | 26 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-032-483f556efe6e.woff2` | woff2 | 31 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-033-7fd71c215768.woff2` | woff2 | 36 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-034-fecb0efd3f13.woff2` | woff2 | 37 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-035-90731a14dc5f.woff2` | woff2 | 26 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-036-13d474e954a1.woff2` | woff2 | 29 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-037-8750a29c5988.woff2` | woff2 | 34 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-038-a242d02b1dc5.woff2` | woff2 | 39 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-039-cf98bde0dee1.woff2` | woff2 | 34 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-040-23266ff05912.woff2` | woff2 | 37 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-041-c599e715dbad.woff2` | woff2 | 30 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-042-5120ad908043.woff2` | woff2 | 34 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-043-2835ce16e53d.woff2` | woff2 | 27 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-044-d8122221576d.woff2` | woff2 | 35 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-045-dc62fa396fbe.woff2` | woff2 | 30 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-046-d7fd5b0fd178.woff2` | woff2 | 32 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-047-c76b4d8857b0.woff2` | woff2 | 31 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-048-fa2ebf06d58d.woff2` | woff2 | 31 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-049-e161f1f6f37d.woff2` | woff2 | 36 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-050-242529490c30.woff2` | woff2 | 36 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-051-6e97cf8d25b2.woff2` | woff2 | 42 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-052-eebc62e3b3bc.woff2` | woff2 | 36 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-053-077391c466d3.woff2` | woff2 | 28 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-054-9746f4cae79c.woff2` | woff2 | 34 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-055-65871fa7883c.woff2` | woff2 | 27 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-056-44d48a5868b8.woff2` | woff2 | 29 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-057-77b37bb3f12b.woff2` | woff2 | 35 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-058-95b5eb66d79b.woff2` | woff2 | 37 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-059-5181a67130f0.woff2` | woff2 | 35 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-060-ca9d715c8b69.woff2` | woff2 | 35 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-061-fc8b88f649cd.woff2` | woff2 | 33 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-062-fea81222558b.woff2` | woff2 | 32 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-063-ae4729592d7f.woff2` | woff2 | 36 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-064-9b4596cb39c5.woff2` | woff2 | 33 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-065-7f91f9125389.woff2` | woff2 | 33 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-066-74d0009acb0b.woff2` | woff2 | 32 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-067-94474a95c77b.woff2` | woff2 | 34 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-068-c3b1eb75bb32.woff2` | woff2 | 31 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-069-61a22f8ae867.woff2` | woff2 | 37 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-070-5c4496d55820.woff2` | woff2 | 36 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-071-8d9d8fdb90c5.woff2` | woff2 | 33 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-072-5857859c62eb.woff2` | woff2 | 35 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-073-646609935aa5.woff2` | woff2 | 39 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-074-3aed740d0e43.woff2` | woff2 | 41 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-075-380577f88794.woff2` | woff2 | 42 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-076-e62f17c4160a.woff2` | woff2 | 45 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-077-77e3cd58b0bc.woff2` | woff2 | 30 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/assets/fonts/hawkr-rounded-v2/hawkr-rounded-v2-cold-078-de8e20246a24.woff2` | woff2 | 29 | - | 開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成 | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/lib/fonts/files/roboto-latin.woff2` | woff2 | 36 | - | 開源（Roboto，OFL-1.1；5b 依檔內 name table 驗證） | 否／像素未掃 | 5 | 保留＋THIRD_PARTY_NOTICES |
| `website/lib/fonts/files/zen-maru-gothic-latin-400.woff2` | woff2 | 11 | - | 開源（Zen Maru Gothic，OFL-1.1；5b 依檔內 name table 驗證） | 否／像素未掃 | 2 | 保留＋THIRD_PARTY_NOTICES |
| `website/lib/fonts/files/zen-maru-gothic-latin-500.woff2` | woff2 | 11 | - | 開源（Zen Maru Gothic，OFL-1.1；5b 依檔內 name table 驗證） | 否／像素未掃 | 1 | 保留＋THIRD_PARTY_NOTICES |
| `website/lib/fonts/files/zen-maru-gothic-latin-700.woff2` | woff2 | 11 | - | 開源（Zen Maru Gothic，OFL-1.1；5b 依檔內 name table 驗證） | 否／像素未掃 | 1 | 保留＋THIRD_PARTY_NOTICES |
| `website/lib/fonts/files/zen-maru-gothic-latin-900.woff2` | woff2 | 11 | - | 開源（Zen Maru Gothic，OFL-1.1；5b 依檔內 name table 驗證） | 否／像素未掃 | 1 | 保留＋THIRD_PARTY_NOTICES |

### G10 QR 碼（4 檔，1 KB）

依據：QR 碼圖片。原圖的內容未解碼，可能編碼了真實連結或帳號；5a 換成合成的 QR（`gen_qr.py`），以 OpenCV 解碼確認內容是 `https://example.com`。型態 png 4；這組檔案的 commit 次數：Will 4、他人 6。

| 素材 | 型態 | KB | 尺寸 | 來源 | 含舊名 | 引用檔數 | 去向 |
|---|---|---|---|---|---|---|---|
| `buyer-app/assets/rece_qr.png` | png | 1 | 225x225 | 自製（程式合成的 QR，編碼 `https://example.com`；`gen_qr.py`） | 否／像素未掃 | 1 | 保留（5a 已換成合成 QR） |
| `buyer-app/assets/rece_qr2.png` | png | 1 | 225x225 | 自製（程式合成的 QR，編碼 `https://example.com`；`gen_qr.py`） | 否／像素未掃 | 1 | 保留（5a 已換成合成 QR） |
| `seller-app/assets/rece_qr.png` | png | 1 | 225x225 | 自製（程式合成的 QR，編碼 `https://example.com`；`gen_qr.py`） | 否／像素未掃 | 1 | 保留（5a 已換成合成 QR） |
| `seller-app/assets/rece_qr2.png` | png | 1 | 225x225 | 自製（程式合成的 QR，編碼 `https://example.com`；`gen_qr.py`） | 否／像素未掃 | 1 | 保留（5a 已換成合成 QR） |

### G13 Storybook 預覽圖（12 檔，2 KB）

依據：Storybook 預覽用的極小幾何圖形（單檔小於 400 B），無創作性；仍列入以便逐檔確認。型態 svg 12；這組檔案的 commit 次數：Will 0、他人 12。

| 素材 | 型態 | KB | 尺寸 | 來源 | 含舊名 | 引用檔數 | 去向 |
|---|---|---|---|---|---|---|---|
| `website/.storybook/public/i/banner/public/product/gallery-fixture/1.svg` | svg | 1 | - | 自製（無創作性的幾何圖形） | 否／像素未掃 | 0 | 保留（開發用預覽圖；D12，5a） |
| `website/.storybook/public/i/banner/public/product/gallery-fixture/2.svg` | svg | 1 | - | 自製（無創作性的幾何圖形） | 否／像素未掃 | 0 | 保留（開發用預覽圖；D12，5a） |
| `website/.storybook/public/i/banner/public/product/gallery-fixture/3.svg` | svg | 1 | - | 自製（無創作性的幾何圖形） | 否／像素未掃 | 0 | 保留（開發用預覽圖；D12，5a） |
| `website/.storybook/public/i/banner/public/product/gallery-fixture/4.svg` | svg | 1 | - | 自製（無創作性的幾何圖形） | 否／像素未掃 | 0 | 保留（開發用預覽圖；D12，5a） |
| `website/.storybook/public/i/card/public/product/gallery-fixture/1.svg` | svg | 1 | - | 自製（無創作性的幾何圖形） | 否／像素未掃 | 0 | 保留（開發用預覽圖；D12，5a） |
| `website/.storybook/public/i/card/public/product/gallery-fixture/2.svg` | svg | 1 | - | 自製（無創作性的幾何圖形） | 否／像素未掃 | 0 | 保留（開發用預覽圖；D12，5a） |
| `website/.storybook/public/i/card/public/product/gallery-fixture/3.svg` | svg | 1 | - | 自製（無創作性的幾何圖形） | 否／像素未掃 | 0 | 保留（開發用預覽圖；D12，5a） |
| `website/.storybook/public/i/card/public/product/gallery-fixture/4.svg` | svg | 1 | - | 自製（無創作性的幾何圖形） | 否／像素未掃 | 0 | 保留（開發用預覽圖；D12，5a） |
| `website/.storybook/public/i/detail/public/product/gallery-fixture/1.svg` | svg | 1 | - | 自製（無創作性的幾何圖形） | 否／像素未掃 | 0 | 保留（開發用預覽圖；D12，5a） |
| `website/.storybook/public/i/detail/public/product/gallery-fixture/2.svg` | svg | 1 | - | 自製（無創作性的幾何圖形） | 否／像素未掃 | 0 | 保留（開發用預覽圖；D12，5a） |
| `website/.storybook/public/i/detail/public/product/gallery-fixture/3.svg` | svg | 1 | - | 自製（無創作性的幾何圖形） | 否／像素未掃 | 0 | 保留（開發用預覽圖；D12，5a） |
| `website/.storybook/public/i/detail/public/product/gallery-fixture/4.svg` | svg | 1 | - | 自製（無創作性的幾何圖形） | 否／像素未掃 | 0 | 保留（開發用預覽圖；D12，5a） |

### G14 程式產生的佔位圖（32 檔，35 KB）

依據：5e2 由產生器輸出的佔位圖：單色底加一個 Material Symbols 圖示，顏色讀 `website/design-system/cross-platform-tokens.json` 的 `brand.*`。型態 svg 32；這組檔案的 commit 次數：Will 32、他人 0。

| 素材 | 型態 | KB | 尺寸 | 來源 | 含舊名 | 引用檔數 | 去向 |
|---|---|---|---|---|---|---|---|
| `website/public/placeholders/ai-style.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 2 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/app-screen.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 2 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/avatar.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 3 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/badge-character.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/fruit-atlas.svg` | svg | 10 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/guide-page.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/guide-screen.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/journey-birth.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/journey-distribution.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/journey-harvest.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/journey-processing.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/journey-retail.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/journey-sorting.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/journey-table.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/phone-screen.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/post.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 4 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/scene-greenhouse.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 2 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/scene-irrigation.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 2 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/scene-market.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 2 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/scene-orchard.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/scene-sunrise.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/scene-sunset.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/vision.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/web-screen.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 4 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/welfare-discount.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/welfare-groupbuy.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/welfare-hero.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 2 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/welfare-holiday.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/welfare-local.svg` | svg | 2 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/welfare-scene-company.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/welfare-scene-harvest.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |
| `website/public/placeholders/welfare-scene-table.svg` | svg | 1 | - | 自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0） | 否／像素未掃 | 1 | 保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守） |

### G99 UI 圖示（1 檔，0 KB）

依據：App 與網站的 UI 圖示：目錄與檔名顯示是功能圖示（購物車、訂單、設定等），設計稿匯出的 SVG 或 PNG，首次 commit 皆為他人，SVG 沒有生成器或出處註解。型態 svg 1；這組檔案的 commit 次數：Will 1、他人 1。

| 素材 | 型態 | KB | 尺寸 | 來源 | 含舊名 | 引用檔數 | 去向 |
|---|---|---|---|---|---|---|---|
| `website/app/icon.svg` | svg | 1 | - | 自製（`brand/hawkr/`） | 否／像素未掃 | 8 | 保留（步驟 4 由 `brand/hawkr/` 產生） |

### 已刪除的素材

步驟 5 各子步驟刪除的素材（資料在 `deleted-assets.tsv`，每個子步驟追加）。「引用檔數」是刪除後其他追蹤中的文字檔仍含該檔名的檔數，必須是 0；同名檔另以路徑確認。

| 素材 | 型態 | 來源 | 群組 | 子步驟 | 引用檔數 | 去向 |
|---|---|---|---|---|---|---|
| `admin/public/icons/Actions & UI/search.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `@hawkr/icons` 的 `MaterialSymbol` |
| `admin/public/icons/certifications/cas.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤（認證的短名稱）或既有的圖示框加文字；無引用者直接刪除 |
| `admin/public/icons/certifications/gmp.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤（認證的短名稱）或既有的圖示框加文字；無引用者直接刪除 |
| `admin/public/icons/certifications/haccp.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤（認證的短名稱）或既有的圖示框加文字；無引用者直接刪除 |
| `admin/public/icons/certifications/iso.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤（認證的短名稱）或既有的圖示框加文字；無引用者直接刪除 |
| `admin/public/icons/certifications/organic.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤（認證的短名稱）或既有的圖示框加文字；無引用者直接刪除 |
| `admin/public/icons/certifications/tap.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤（認證的短名稱）或既有的圖示框加文字；無引用者直接刪除 |
| `admin/public/icons/icon24.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `@hawkr/icons` 的 `MaterialSymbol` |
| `admin/public/icons/label/Data Visualization/chart-line.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `@hawkr/icons` 的 `MaterialSymbol` |
| `admin/public/icons/label/icon/Group.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `@hawkr/icons` 的 `MaterialSymbol` |
| `admin/public/icons/label/icon24-3.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `@hawkr/icons` 的 `MaterialSymbol` |
| `admin/public/icons/label/label/icon-1.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `@hawkr/icons` 的 `MaterialSymbol` |
| `admin/public/icons/label/label/icon/no/24.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `@hawkr/icons` 的 `MaterialSymbol` |
| `admin/public/icons/payuni-logo.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `admin/src/app/fonts/openhuninn-2.0-subset.woff2` | woff2 | 開源字型 | G07 | 5b | 0 | 已刪除：由生成器重新生成並改名為 hawkr-rounded-2.0-subset.woff2 |
| `backend/services/java/media-service/src/main/resources/images.jpeg` | jpeg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/2.0x/seller_default.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/3.0x/seller_default.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/Auth_Screen/Google__G__logo.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/Auth_Screen/apple_logo.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/Auth_Screen/click.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/Default_Logo.png` | png | 來源不明（舊 logo） | G03 | 5a | 0 | 已刪除：引用改指向 `assets/hawkrlogo.png`（Hawkr 圓形圖，由 `brand/hawkr/` 產生） |
| `buyer-app/assets/Emptybox.json` | json | 來源不明（Lottie） | G09 | 5d1 | 0 | 已刪除：沒有引用的 Lottie（`scan.py` 的 Lottie 規則只看 `assets/lottie/` 路徑而沒掃到），直接刪除 |
| `buyer-app/assets/after_sales/announcement.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/after_sales/sales_1.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/after_sales/sales_2.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/after_sales/sales_3.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/after_sales/sales_4.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/after_sales/step_circle_active.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/after_sales/step_circle_inactive.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/after_sales/step_icon_chat.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/after_sales/step_icon_check.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/after_sales/step_icon_clock.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/after_sales/step_icon_edit.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/after_sales/step_line_active.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/after_sales/step_line_inactive.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/banner2.jpg` | jpg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/bar_icon/click_catgory.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/bar_icon/click_home.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/bar_icon/click_order.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/bar_icon/click_setting.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/bar_icon/click_wallet.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/bar_icon/non_catgory.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/bar_icon/non_home.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/bar_icon/non_order.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/bar_icon/non_setting.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/bar_icon/non_wallet.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/brand/auth-moon.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/cart/7-Eleven-Logo.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `buyer-app/assets/cart/Actions & UI/edit.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/cart/Finance/usd-circle.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/cart/Nature & Weather/temperature-half.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/cart/Uber-Logo.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `buyer-app/assets/cart/blackcat.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `buyer-app/assets/cart/chart.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/cart/citron.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/cart/clear_cart_list.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/cart/discount.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/cart/icon24.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/cart/items.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/category_icons/2.0x/processed.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/category_icons/3.0x/processed.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/category_icons/egg_dairy.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/category_icons/flower.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/category_icons/fruit.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/category_icons/grain.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/category_icons/meat.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/category_icons/processed.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/category_icons/seafood.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/category_icons/spice.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/category_icons/vegetable.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/chatbot_icon.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/checkout/card.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/checkout/cart.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/cliam.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/common/cart.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/common/cha.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/common/follow.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/common/following.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/default_buyer_avatar.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/default_seller_avatar.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/done.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f11.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f12.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f13.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f14.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f15.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f16.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f17.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f18.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f19.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f8.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f9.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f_nonselect.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/f_select.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/farmlicence1.jpg` | jpg | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：認證標章框改用 App 既有的 verified 圖示加名稱文字 |
| `buyer-app/assets/farmlicence2.jpg` | jpg | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：認證標章框改用 App 既有的 verified 圖示加名稱文字 |
| `buyer-app/assets/farmlicence3.jpeg` | jpeg | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：認證標章框改用 App 既有的 verified 圖示加名稱文字 |
| `buyer-app/assets/fruits/apple_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/apple_2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/apple_3.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/apple_4.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/avocado_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/banana_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/banana_2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/blueberry_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/guava_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/lemon_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/lemon_2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/lemon_3.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/lemon_4.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/orange_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/orange_2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/orange_3.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/strawberry_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/strawberry_2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/watermelon_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/fruits/watermelon_2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/home_icon/addr_b1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/addr_b2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/addr_b3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/addr_b4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/addr_b5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/addr_b6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/addr_b7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/addr_b8.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/addr_b9.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/ai1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/ai2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/ai3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/ai4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/ai5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/ai6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/ai7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/ai_b1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/ai_b2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/cart_icon.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `buyer-app/assets/home_icon/fram1/Frame 37304.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/fram1/Group 37281.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/home_icon/icon_after_sales.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/ic_location.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/2.0x/ic_711_logo.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `buyer-app/assets/icons/2.0x/ic_kuroneko_logo.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `buyer-app/assets/icons/3.0x/ic_711_logo.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `buyer-app/assets/icons/3.0x/ic_kuroneko_logo.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `buyer-app/assets/icons/date_picker/chevron_down.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/date_picker/chevron_left.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/date_picker/chevron_right.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/date_picker/chevron_up.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/heart_filled.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/heart_outline.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_711.svg` | svg | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `buyer-app/assets/icons/ic_711_logo.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `buyer-app/assets/icons/ic_announcement.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_arrow_down.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_back.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_block.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_car_gray.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_chat.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_chat_bubble.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_chevron_down.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_chevron_right.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_clipboard.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_clock.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_coupon.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_credit_card.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_delete.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_edit.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_edit.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_finance.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_flag_dollar.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_kuroneko.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `buyer-app/assets/icons/ic_kuroneko_logo.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `buyer-app/assets/icons/ic_location.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_logistics_truck.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_map_marker.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_minus.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_order_clipboard.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_people.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_phone.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_plus.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_progress_bell.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_progress_check.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_progress_close.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_progress_location.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_progress_truck.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_sparkle.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_star_filled.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_swipe_up.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_temp.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_temp.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_time.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_vegetable.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/ic_verified_badge.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/iconly_award_medal.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/iconly_board_pin.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/iconly_delivery_hand.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/iconly_question_mark_circle.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/iconly_shopping_cart_return.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/iconly_truck.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/market_favorite_filled.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/market_favorite_outline.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/payment_apple_pay.svg` | svg | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `buyer-app/assets/icons/payment_mastercard.svg` | svg | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `buyer-app/assets/icons/review/ic_clipboard.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/review/ic_plus.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/review/ic_vegetable.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/review/star_filled.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/review/star_outline.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/trash_delete.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/truck_electric.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/wallet/ic_vip.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/icons/wallet/ic_vip_crown.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/images/address_icon.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/images/banner_mascot.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/images/cart_icon_white.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/images/chat_icon_white.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/images/empty_icon.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/images/mascot.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/images/mascot_cow.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/images/search_icon.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/images/vegetable_icon.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/leaves/Group_37341.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/leaves/Vector-1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-10.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-11.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-12.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-13.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-14.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-15.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-16.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-17.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-18.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-3.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-4.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-5.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-6.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-7.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-8.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector-9.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector_2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector_3.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/leaves/Vector_4.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/lottie/iconly/ai_assistant_replying.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/cart.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/chat.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/checkout_coins.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/deliverybox_logo.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_address.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_after_sale.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_chat.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_coupon.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_favorites.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_member_card.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_merchant_search.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_merchants.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_notifications.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_orders.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_posts.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_products.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_recent_views.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_reviews.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_search.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/empty_subscriptions.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/merchant_favorite.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/merchant_messages.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/merchant_posts.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/merchant_shop.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/nav_home.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/nav_market.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/nav_orders.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/nav_user.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/nav_wallet.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/notification_logo.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/order_cancelled.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/order_completed.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/order_delivered.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/order_logistics_issue.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/order_paid.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/order_partial_refund.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/order_pickup.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/order_preparing.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/order_refunded.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/order_returning.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/order_shipping.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/order_unknown.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/order_waiting_payment.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/settings_favorite.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/settings_following.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/settings_viewed.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/text_size.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/truck_logo.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/wallet_coupon.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/lottie/iconly/wallet_member.json` | json | 來源不明（Iconly 匯出） | G05 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `buyer-app/assets/merchant_page/Location.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/ic_back.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/ic_chat.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/map-marker.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/merchant_b1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/merchant_b2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/merchant_b3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/merchant_b4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/merchant_b5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/merchant_b6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/merchant_card_p1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/merchant_card_p2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/merchant_card_p3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/merchant_card_p4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/merchant_card_p5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/merchant_p1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/merchant_page/merchant_p2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/merchant_p3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/merchant_p4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/merchant_page/temperature-half.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/nopost.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/notification/chat.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/notification/coupon.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/notification/message.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/notification/order.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/notification/post.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0（路徑比對；檔名另見 4 檔，與留下的同名素材合併計算） | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/notification/refund.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/notification/system.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/notification/trash-alt.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/notification/user.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/order_card/order_b1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_b2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_cancel1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_cancel2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_p1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_p10.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_p2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_p3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_p4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_p5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_p6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_p7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_p8.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_p9.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_part.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_pic1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_pic2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_pic3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_picture.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_redey.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_t1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_t2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_t3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_t4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_card/order_t5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_details/fram1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `buyer-app/assets/order_details/i2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_details/i3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_details/id1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_details/id10.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_details/id11.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_details/id4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_details/id5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_details/id6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_details/id7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_details/id8.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/order_details/id9.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/post_icon/back.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/cart.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/chat_gray.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/chatr.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/clock.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/love_filled.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/love_outline.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/more.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/post_b1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/post_b2.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/post_b3.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/post_b4.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/post_comment.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/post_share.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/post_t1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/share.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/post_icon/share_arrow.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/review_icon/i1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/review_icon/i2.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/review_icon/start_1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/review_icon/start_2.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/saleout.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/search_icon/search_b5.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/search_icon/search_icon1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/search_icon/search_icon2.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/search_icon/search_icon3.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/seller_default.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/setting_icon/addr_1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/addr_2.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/addr_3.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/addr_4.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/like.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/line.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/look.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/mail.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/addr_1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/addr_2.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/addr_3.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/addr_4.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/line.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/mail.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon10.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon11.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon12.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon13.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon14.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon1_1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon1_2.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon2.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon3.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon4.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon5.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon6.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon7.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon8.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon/setting_icon9.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon10.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon11.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon12.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon13.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon14.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon1_1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon1_2.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon2.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon3.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon4.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon5.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon6.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon7.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon8.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_icon/setting_icon9.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup1_icon/setting_setup1_icon1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup1_icon/setting_setup1_icon2.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup1_icon/setting_setup1_icon3.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup1_icon/setting_setup1_icon4.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup1_icon/setting_setup1_icon5.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup2_icon/setting_setup2_icon1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup2_icon/setting_setup2_icon2.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup2_icon/setting_setup2_icon3.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup2_icon/setting_setup2_icon4.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup2_icon/setting_setup2_icon5.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup2_icon/setting_setup2_icon6.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup3_icon/setting_setup3_icon1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup3_icon/setting_setup3_icon2.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup3_icon/setting_setup3_icon3.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup3_icon/setting_setup3_icon4.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup3_icon/setting_setup3_icon5.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/setting_setup3_icon/setting_setup3_icon6.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d1 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/user/b1.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/user/b2.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/user/b3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/b4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/b5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/b6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/b7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/back.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0（路徑比對；檔名字串另見 1 檔，是無關的網址或欄位值，已人工確認） | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/onboarding_back.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/t0.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/t00.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/t1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/t11.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/t12.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/t13.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/t14.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/t15.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/t16.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/t2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/t3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/t4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/t6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/w1.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `buyer-app/assets/user/w2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/w3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/user/w4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/wallet_icon/logo_hawkr.png` | png | 來源不明（舊 logo） | G03 | 5a | 0 | 已刪除：沒有程式引用，直接刪除 |
| `buyer-app/assets/wallet_point/default_merchant_cover.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `buyer-app/assets/wallet_point/no_card.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/wallet_point/no_coupon.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/wallet_point/wallet_back.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/wallet_point/wallet_empty_icon.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/wallet_point/wallet_icon_inner.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/wallet_point/wallet_tag.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `buyer-app/assets/wallet_point/wallettag_1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `buyer-app/assets/yoyo.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/Auth_Screen/Ellipse 219.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/Auth_Screen/Google__G__logo.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/Auth_Screen/apple_logo.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/Auth_Screen/click.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/Customer_status_Setting/Blocked.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/Customer_status_Setting/VIP.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/announcement.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/ic_coupon.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/ic_credit.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/ic_refund1.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/ic_reject.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/sales_1.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/sales_2.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/sales_3.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/sales_4.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/step_circle_active.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/step_circle_inactive.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/step_icon_chat.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/step_icon_check.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/step_icon_clock.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/step_icon_edit.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/step_line_active.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/after_sales/step_line_inactive.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/ai_edit.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `seller-app/assets/ai_maker/ai_b1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/ai_maker/ai_b2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/ai_maker/ai_b3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/ai_maker/ai_b4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/ai_maker/cart_icon.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/ai_maker/color1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/ai_maker/color2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/ai_maker/color3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/ai_maker/color4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/ai_maker/color5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/ai_maker/color6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/ai_maker/config1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `seller-app/assets/ai_maker/config2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `seller-app/assets/ai_maker/config3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `seller-app/assets/ai_maker/config4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `seller-app/assets/ai_maker/emoji1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/ai_maker/emoji2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/ai_maker/emoji3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/ai_maker/emoji4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/ai_maker/emoji5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/ai_maker/emoji6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/ai_maker/emoji7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/ai_maker/emoji8.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/ai_maker/emoji9.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/bar_icon/click_home.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/bar_icon/click_market.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/bar_icon/click_order.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/bar_icon/click_setting.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/bar_icon/click_wallet.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/bar_icon/non_home.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/bar_icon/non_market.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/bar_icon/non_order.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/bar_icon/non_setting.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/bar_icon/non_wallet.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/brand/auth-moon.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/cliam.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/common/empty.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/coupon/Label.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/coupon/Money.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/coupon/coupon.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/coupon/freeshipping.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/coupon/percent.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/coupon/percentage.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/coupon/shipping.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/coupon/shipping.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/customer_detail_screen/Menu.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/customer_detail_screen/chevron-left.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/customer_detail_screen/clipboard-notes.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `seller-app/assets/customer_detail_screen/icon24-1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/customer_detail_screen/icon24.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `seller-app/assets/customer_detail_screen/icon_copy_20.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/customer_detail_screen/map-marker.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `seller-app/assets/customer_detail_screen/message.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `seller-app/assets/customer_detail_screen/phone.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `seller-app/assets/customer_detail_screen/plus.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/customer_detail_screen/usd-circle.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `seller-app/assets/dashbord.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/delete.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/done.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/edit.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/f1.jpeg` | jpeg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/f1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/f10.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/f2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/f3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/f4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/f5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/f6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/b2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/b3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/b4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/b5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/b6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/b7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/back.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0（路徑比對；檔名字串另見 1 檔，是無關的網址或欄位值，已人工確認） | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/name_btn.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/name_go.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/name_text.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/t12.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/t16.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/t2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/t44.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/w1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/w2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/w3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farm/w4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/farmlicence1.jpg` | jpg | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：認證標章框改用 App 既有的 verified 圖示加名稱文字 |
| `seller-app/assets/farmlicence2.jpg` | jpg | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：認證標章框改用 App 既有的 verified 圖示加名稱文字 |
| `seller-app/assets/farmlicence3.jpeg` | jpeg | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：認證標章框改用 App 既有的 verified 圖示加名稱文字 |
| `seller-app/assets/fruits/apple_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/apple_2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/apple_3.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/apple_4.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/avocado_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/banana_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/banana_2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/blueberry_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/guava_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/lemon_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/lemon_2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/lemon_3.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/lemon_4.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/orange_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/orange_2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/orange_3.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/strawberry_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/strawberry_2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/watermelon_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/fruits/watermelon_2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/google.png` | png | 來源不明（標章或商標） | G04 | 5e1 | 0 | 已刪除：無引用（第三方商標）：無引用，直接刪除（第三方商標） |
| `seller-app/assets/home/Default_Logo.png` | png | 來源不明（舊 logo） | G03 | 5a | 0 | 已刪除：引用改指向 `assets/hawkrlogo.png`（Hawkr 圓形圖，由 `brand/hawkr/` 產生） |
| `seller-app/assets/home/after_sales.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/completed.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/default_buyer_avatar.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/home/group_add.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/home_icon1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/home_icon2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/home_icon3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/home_icon4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/home_icon5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/home_icon6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/home_icon7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/home_icon8.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/icon_after_sales.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/icon_completed.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/icon_in_progress.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/icon_orders.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/icon_pending.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/icon_sales.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/icon_subscribers.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/icon_views.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/icon_vip_bg.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/icon_vip_crown.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/in_progress.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/orders.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/pending.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/sales.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/home/views.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/icons/bell.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/blackcat.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `seller-app/assets/icons/date_picker/chevron_down.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/date_picker/chevron_left.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/date_picker/chevron_right.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/date_picker/chevron_up.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/gear.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/heart.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_711.svg` | svg | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `seller-app/assets/icons/ic_711_figma.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `seller-app/assets/icons/ic_711_logo.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `seller-app/assets/icons/ic_ai_avatar.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0（路徑比對；檔名字串另見 1 檔，是無關的網址或欄位值，已人工確認） | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_announcement.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_arrow_down.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_award.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_back.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_block.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_car_gray.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_chat.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_chat_bubble.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_chevron_down.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_chevron_right.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_clipboard.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_clock.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_coupon.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_credit_card.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_delete.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_edit.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_finance.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_flag_dollar.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_kuroneko.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `seller-app/assets/icons/ic_kuroneko_45.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `seller-app/assets/icons/ic_kuroneko_logo.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `seller-app/assets/icons/ic_location.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_logistics_truck.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_map_marker.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_minus.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_order.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_order_clipboard.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_people.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_phone.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_photo.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_plus.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_post_chat.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_post_chat_3x.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_post_love.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_post_love_3x.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_post_share.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_post_share_3x.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_post_shop.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_post_shop_bg.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_post_shop_composite.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_post_time.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_progress_bell.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_progress_check.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_progress_close.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_progress_location.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_progress_truck.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_sparkle.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_star_filled.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_star_gold.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_status_cert_2x.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_status_review.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_status_review_2x.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_status_shipping_2x.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_step_cert.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_step_cert_2x.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_step_shipping.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_step_shipping_2x.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_step_shop.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_step_shop_2x.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_subscribers.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_swipe_up.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_temp.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_time.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_vegetable.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_verified_badge.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/ic_verified_user.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/iconly_board_pin.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/iconly_delivery_hand.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/iconly_order_box.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/iconly_stock_courier.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/iconly_truck.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/iconly_users_outline.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/iconly_wallet.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/rocket.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/shopping_cart.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/star.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/icons/truck_electric.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/images/empty.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/images/icon_delete.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/images/icon_edit.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/ins1.jpg` | jpg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/ins2.jpeg` | jpeg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/ins3.jpeg` | jpeg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/leaves/Group_37341.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/leaves/Vector-1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-10.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-11.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-12.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-13.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-14.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-15.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-16.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-17.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-18.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-3.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-4.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-5.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-6.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-7.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-8.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector-9.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector_1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector_2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector_3.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/leaves/Vector_4.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/login/l1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/login/l10.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/login/l2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/login/l3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/login/l4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/login/l5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/login/l6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/login/l7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/login/l8.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/login/l9.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/login/lback.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/login/name_btn.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/login/name_go.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/login/name_text.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/login/onboarding_back.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/logo.svg` | svg | 來源不明（舊 logo） | G03 | 5a | 0（路徑比對；檔名字串另見 3 檔，是無關的網址或欄位值，已人工確認） | 已刪除：沒有程式引用，直接刪除 |
| `seller-app/assets/lottie/iconly/cart.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/chat.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/empty_coupons.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/empty_customers.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/empty_insights.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/empty_messages.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/empty_notifications.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/empty_orders.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/empty_posts.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/empty_products.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/empty_reviews.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/empty_search.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/empty_todos.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/empty_tracking.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/nav_home.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/nav_market.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/nav_orders.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/nav_seller_coupons.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/nav_seller_customers.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/nav_seller_inventory.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/nav_seller_posts.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/nav_seller_preview.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/nav_seller_products.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/nav_seller_shipping.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/nav_user.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/nav_wallet.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/lottie/iconly/text_size.json` | json | 來源不明（Iconly 匯出） | G05 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons 的靜態圖示（`HkIcon`／`AppIcons`），`lottie` 依賴一併刪除 |
| `seller-app/assets/merchant_page/Reward.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/chat_empty.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/map-marker.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/merchant_b1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/merchant_b2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/merchant_b3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/merchant_b4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/merchant_b5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/merchant_b6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/merchant_card_p1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/merchant_card_p2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/merchant_card_p3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/merchant_card_p4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/merchant_card_p5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/merchant_p1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：照片與插畫：換程式繪製的佔位 `HkPlaceholder`（單色底加 Material 圖示，不放檔案） |
| `seller-app/assets/merchant_page/merchant_p2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/merchant_p3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/merchant_p4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/merchant_page/post_empty.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/no.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/nopost.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/notification/chat.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/notification/coupon.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/notification/message.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/notification/order.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/notification/post.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0（路徑比對；檔名另見 4 檔，與留下的同名素材合併計算） | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/notification/refund.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/notification/system.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/notification/trash-alt.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/notification/user.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/order.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_b1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_b2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_cancel1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_cancel2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_p1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_p10.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_p11.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_p2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_p3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_p4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_p5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_p6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_p7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_p8.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_p9.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_part.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_redey.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_t1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_t2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_card/order_t3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/clipboard-notes.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/clock-eight.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/credit-card.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/i2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/i3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/icon24-1.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/icon24-2.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/icon24.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/id1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/id10.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/id11.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/id4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/id5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/id6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/id7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/id8.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/id9.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/map-marker.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/phone.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_details/usd-circle.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/order_icon/Marked.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/order_icon/announcement.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/order_icon/car.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/order_icon/empty.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/order_icon/label.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/order_icon/signal.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/page.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/post_icon/back.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/chat_gray.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/chatr.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/clock.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/love_filled.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/love_outline.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/more.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/post_b1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/post_b2.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/post_b3.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/post_b4.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/post_b5.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/post_b6.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/post_b7.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/post_comment.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/post_share.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/post_t1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/share.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/post_icon/share_arrow.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/product/ic_share.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/product/progress/camer.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/product/progress/chose_type.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/product/progress/shop_edit.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/product/progress/shop_speil.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/product/progress/shop_tags.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/product_setup0/f_nonselect.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/f_select.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/upload_b1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/upload_b2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/upload_b3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/upload_b4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/upload_b5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/upload_p0.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/upload_p1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/upload_p2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/upload_p3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/upload_p4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/upload_p5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/upload_p6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/upload_p7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/upload_p8.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup0/upload_p9.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup1/upload_b1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup1/upload_b2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `seller-app/assets/product_setup1/upload_b3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `seller-app/assets/product_setup1/upload_b4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup1/upload_b5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup1/upload_p0.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup1/upload_p1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup1/upload_p2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup1/upload_p3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup1/upload_p4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup1/upload_p5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup1/upload_p6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup1/upload_p7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup1/upload_p8.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup1/upload_p9.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup2/upload_b1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup2/upload_b2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup2/upload_p0.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup2/upload_p1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup2/upload_p2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup2/upload_p3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup2/upload_p4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup2/upload_p5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup2/upload_p6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup2/upload_p7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup3/upload_b1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup3/upload_b2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup3/upload_b3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/product_setup3/upload_b4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/product_setup3/upload_b5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/product_setup3/upload_p0.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup3/upload_p1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup3/upload_p2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup3/upload_p3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup3/upload_p4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup3/upload_p5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup3/upload_p6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup3/upload_p7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup3/upload_tag1.png` | png | 來源不明（標章或商標） | G04 | 5e1 | 0 | 已刪除：標章（改文字標籤）：改文字標籤（D12 Q-A4） |
| `seller-app/assets/product_setup3/upload_tag2.png` | png | 來源不明（標章或商標） | G04 | 5e1 | 0 | 已刪除：標章（改文字標籤）：改文字標籤（D12 Q-A4） |
| `seller-app/assets/product_setup3/upload_tag3.png` | png | 來源不明（標章或商標） | G04 | 5e1 | 0 | 已刪除：標章（改文字標籤）：改文字標籤（D12 Q-A4） |
| `seller-app/assets/product_setup3/upload_tag4.png` | png | 來源不明（標章或商標） | G04 | 5e1 | 0 | 已刪除：標章（改文字標籤）：改文字標籤（D12 Q-A4） |
| `seller-app/assets/product_setup3/upload_tag5.png` | png | 來源不明（標章或商標） | G04 | 5e1 | 0 | 已刪除：標章（改文字標籤）：改文字標籤（D12 Q-A4） |
| `seller-app/assets/product_setup3/upload_tag6.png` | png | 來源不明（標章或商標） | G04 | 5e1 | 0 | 已刪除：標章（改文字標籤）：改文字標籤（D12 Q-A4） |
| `seller-app/assets/product_setup3/upload_tag7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/product_setup4/card1_c1.png` | png | 來源不明（標章或商標） | G04 | 5e1 | 0 | 已刪除：無引用（第三方商標）：無引用，直接刪除（第三方商標） |
| `seller-app/assets/product_setup4/card1_c2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/card1_c3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/card1_c4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/card2_c1.png` | png | 來源不明（標章或商標） | G04 | 5e1 | 0 | 已刪除：無引用（第三方商標）：無引用，直接刪除（第三方商標） |
| `seller-app/assets/product_setup4/card2_c2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/card2_c3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/card2_c4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/t3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/upload_b1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/upload_b2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/upload_b3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/upload_p0.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/upload_p1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/upload_p2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/upload_p3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/upload_p4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/upload_p5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/upload_p6.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/upload_p7.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/upload_t1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/upload_t2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/upload_t3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/product_setup4/upload_t4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/qoute.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/real_name/icon_b1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/real_name/icon_b2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/real_name/icon_back.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/real_name/icon_company.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/real_name/icon_company_click.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/real_name/icon_individual.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/real_name/icon_individual_click.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/real_name/icon_t1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/real_name/icon_t2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/saleout.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/setting.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/setting_icon/line.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/setting_icon/mail.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/setting_icon/setting_icon1.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/setting_icon/setting_icon13.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/setting_icon/setting_icon2.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/setting_icon/setting_icon3.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/setting_icon/setting_icon4.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/setting_icon/setting_icon5.png` | png | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d2 | 0 | 已刪除：改用 Flutter 內建 Material Icons（`HkIcon`／`AppIcons`），放在原本的框；無引用者直接刪除 |
| `seller-app/assets/subscribe.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/wallet_icon/Liquid_glass_rectangle_bottom.svg` | svg | 來源不明（公司時期圖片） | G11 | 5c | 0 | 已刪除：無引用，直接刪除（含 `<縮寫>` 的 SVG id，5c 一併處理讓 brand-abbr 歸零） |
| `seller-app/assets/wallet_icon/Liquid_glass_rectangle_top.svg` | svg | 來源不明（公司時期圖片） | G11 | 5c | 0 | 已刪除：無引用，直接刪除（含 `<縮寫>` 的 SVG id，5c 一併處理讓 brand-abbr 歸零） |
| `seller-app/assets/wallet_icon/btm_selector.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/wallet_icon/card_bg.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/wallet_icon/hawkr_logo.png` | png | 來源不明（舊 logo） | G03 | 5a | 0 | 已刪除：沒有程式引用，直接刪除 |
| `seller-app/assets/wallet_icon/hawkr_logo.svg` | svg | 來源不明（舊 logo） | G03 | 5a | 0 | 已刪除：沒有程式引用，直接刪除 |
| `seller-app/assets/wallet_icon/icon_clear.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/wallet_icon/line_separator.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/wallet_icon/logo_hawkr.png` | png | 來源不明（舊 logo） | G03 | 5a | 0 | 已刪除：沒有程式引用，直接刪除 |
| `seller-app/assets/wallet_icon/logo_payuni.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `seller-app/assets/wallet_icon/noisy.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/wallet_icon/seller_business.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/wallet_icon/seller_individual.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/wallet_icon/wallet_b1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/wallet_icon/wallet_b2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/wallet_icon/wallet_b22.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/wallet_icon/wallet_b3.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/wallet_icon/wallet_b33.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：介面元件圖：改成程式繪製的元件（設計系統元件與 design token，不放圖） |
| `seller-app/assets/wallet_icon/wallet_b4.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `seller-app/assets/wallet_icon/wallet_b5.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：UI 圖示：換 Material 圖示（`AppIcons`／`HkIcon`），放在原本的位置 |
| `seller-app/assets/wallet_icon/wallet_coin_icon.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/wallet_icon/wallet_coin_inner.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/wallet_icon/wallet_save1.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/wallet_icon/wallet_save2.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `seller-app/assets/wallet_icon/wallet_tag.png` | png | 來源不明（公司時期圖片） | G11 | 5e1 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/app/[locale]/(content)/seller-guide/icon.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 1 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/account/clock-countdown.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/account/discount-tag.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/account/dollar-box.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/account/home.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/account/receipt-bill-success.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/account/shop-home.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/account/thumb-like.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/account/truck.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/app/apple.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/auth/double-tap.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/auth/key.json` | json | 來源不明（Lottie） | G09 | 5c | 0（路徑比對；檔名字串另見 5 檔，是無關的網址或欄位值，已人工確認） | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/auth/left-down.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/auth/left-up.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/auth/right-down.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/auth/right-up.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/auth/shop-home.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/auth/shopping-bag.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/commerce/cart.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/commerce/checkout-coins.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/corporate-welfare/benefit-ticket.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/corporate-welfare/contact-phone.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/corporate-welfare/match-target.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/corporate-welfare/shopping-cart-check.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/feed/iconly-chat.json` | json | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon` 的 Material Symbols 靜態圖示，Lottie 播放元件與 `lottie-react` 一併刪除 |
| `website/assets/lottie/feed/iconly-eye.json` | json | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon` 的 Material Symbols 靜態圖示，Lottie 播放元件與 `lottie-react` 一併刪除 |
| `website/assets/lottie/feed/iconly-heart.json` | json | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon` 的 Material Symbols 靜態圖示，Lottie 播放元件與 `lottie-react` 一併刪除 |
| `website/assets/lottie/feed/iconly-share.json` | json | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon` 的 Material Symbols 靜態圖示，Lottie 播放元件與 `lottie-react` 一併刪除 |
| `website/assets/lottie/forms/folder-image.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/forms/upload.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/home/nav-new.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/home/nav-origin.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/home/nav-tag.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/home/process-carrots.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/home/process-delivery.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/home/process-eye.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/home/process-tap.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/messages/messages-empty.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/messages/send-icon.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/orders/charity-coins.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/orders/delivery-box.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/orders/delivery-hand-send.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/orders/delivery-refresh.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/orders/dollar-receive.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/orders/party.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/orders/square-clock.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/orders/user-search.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/seller-features/iconly-discount-badge.json` | json | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon` 的 Material Symbols 靜態圖示，Lottie 播放元件與 `lottie-react` 一併刪除 |
| `website/assets/lottie/seller-features/iconly-home-2.json` | json | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon` 的 Material Symbols 靜態圖示，Lottie 播放元件與 `lottie-react` 一併刪除 |
| `website/assets/lottie/seller-features/iconly-two-users.json` | json | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon` 的 Material Symbols 靜態圖示，Lottie 播放元件與 `lottie-react` 一併刪除 |
| `website/assets/lottie/shared/agent-mark.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/ai-sparkle.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/bookcloud-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/cart-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/check-badge.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/capabilities/compare-candidates.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/capabilities/find-suppliers.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/capabilities/organise-requirements.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/clarify.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/color-cycle.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/complete.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/deliberating.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/entry-thinking-cover.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/happy.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/idle-ready.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/inspired.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/listening.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/needs-attention.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/01-vegetables-water-bamboo.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/02-fruits-pomelo.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/03-seafood-pacific-saury.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/04-meat-duck-breast.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/05-egg-dairy-cheese-wheel.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/06-processed-dried-fruit.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/07-spices-red-chili.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/08-grains-rice-ear.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/09-flowers-chrysanthemum.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/01-vegetables-spring-bamboo-shoot.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/02-fruits-spring-green-plum.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/03-seafood-spring-flying-fish.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/04-meat-spring-herb-chicken-leg.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/05-egg-dairy-spring-soft-boiled-egg.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/06-processed-spring-tofu-cube.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/07-spices-spring-garlic.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/08-grains-spring-wheat-ear.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/09-flowers-spring-white-calla-lily.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/01-vegetables-summer-okra.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/02-fruits-summer-irwin-mango.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/03-seafood-summer-spear-squid.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/04-meat-summer-grilled-pork-belly.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/05-egg-dairy-summer-yogurt-cup.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/06-processed-summer-popsicle.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/07-spices-summer-thai-basil.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/08-grains-summer-corn.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/09-flowers-summer-hydrangea.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/01-vegetables-cauliflower.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/02-fruits-jujube.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/03-seafood-mullet.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/04-meat-beef-hotpot-roll.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/05-egg-dairy-butter-block.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/06-processed-dumpling.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/07-spices-ginger.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/08-grains-red-bean-sack.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/09-flowers-poinsettia.lottie.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/source/reference-leaf.svg` | svg | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/spin-recolor.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/surprised.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/thinking.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/working.json` | json | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/delete.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/deliverybox-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/edit.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/face-id-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/food-registration.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/megaphone-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/message-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/mobile-nav-feed-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/mobile-nav-home-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/mobile-nav-market-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/mobile-nav-orders-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/mobile-nav-search-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/mobile-nav-user-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/network-world.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/receipt-bill-search-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0（路徑比對；檔名字串另見 1 檔，是無關的網址或欄位值，已人工確認） | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/refresh.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/ringtone3-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/rocket-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/scroll.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/share-card-export.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/shelf-down.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/shelf-up.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/shopping-bag-check.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/text-size.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/thumb-up.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/trophy-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/user-target-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/users-group-logo.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/shared/waving-hand.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/assets/lottie/toast/loading-circle.json` | json | 來源不明（Lottie） | G09 | 5c | 0 | 已刪除：改用 IconlyIcon 靜態圖示（品牌位置用 Hawkr 標誌），放在原本的框；無引用者直接刪除 |
| `website/lib/fonts/files/openhuninn-2.0-subset.woff2` | woff2 | 開源字型 | G07 | 5b | 0 | 已刪除：刪除（無程式引用；舊版子集含保留字型名） |
| `website/public/PNG/Group2.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/PNG/coming soon.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/PNG/download.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/PNG/mobilmenu.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/PNG/page1/logo.png` | png | 來源不明（舊 logo） | G03 | 5a | 0（路徑比對；檔名字串另見 35 檔，是無關的網址或欄位值，已人工確認） | 已刪除：e2e fixture 改指向 `/brand/logo-horizontal.svg` |
| `website/public/PNG/user/u1.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/PNG/user/u2.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/PNG/user/u3.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/PNG/user/u4.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/PNG/user/u5.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/defaults/buyer-default.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/defaults/post-placeholder.svg` | svg | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/defaults/seller-avatar.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/generated/<舊名-子品牌>-capability-icons-contact-sheet.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/generated/<舊名-子品牌>-capability-icons-optics-check.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/generated/<舊名-子品牌>-capability-icons-preview.webm` | webm | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/generated/<舊名-子品牌>-capability-icons-storyboard.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/generated/<舊名-子品牌>-entry-capability-icons-mobile.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/generated/<舊名-子品牌>-entry-capability-icons.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/generated/<舊名-子品牌>-lottie-contact-sheet.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/generated/<舊名-子品牌>-lottie-preview.webm` | webm | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/generated/<舊名-子品牌>-marble-optics-check.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/generated/<舊名-子品牌>-marble-palette-preview.webm` | webm | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/generated/hawkr-registration-lucky-badge-base.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/generated/hawkr-registration-lucky-badge.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/icons/arrow-right.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/download.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/flags/taiwan-circle.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/iconly/produce/avocado.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/iconly/produce/beet.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/iconly/produce/berries.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/iconly/produce/bilberry.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/iconly/produce/blueberry.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/iconly/produce/coconut.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/iconly/produce/corn.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/iconly/produce/dragonfruit.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/iconly/produce/kiwi.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/iconly/produce/lemon.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/iconly/produce/lychee.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/iconly/produce/mango.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/iconly/produce/pineapple.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/iconly/produce/strawberry.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/iconly/produce/tomato.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/image-off.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/loading/truck-filled.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/loading/truck-outline.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/location-pin.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/notification/chat.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/notification/coupon.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/notification/message.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/notification/order.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/notification/post.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0（路徑比對；檔名另見 4 檔，與留下的同名素材合併計算） | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/notification/refund.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/notification/system.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/notification/user.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/order-actions/cancel-order.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/order-actions/contact-buyer.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/order-actions/ship-confirm.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/payment/apple-pay.svg` | svg | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `website/public/icons/payment/credit-card.svg` | svg | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `website/public/icons/payment/mastercard.svg` | svg | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `website/public/icons/search-circle-bg.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/search-magnifier.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/search.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/seller-type/business.svg` | svg | 來源不明（設計稿匯出的 UI 圖示） | G99 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/seller-type/individual.svg` | svg | 來源不明（Iconly 匯出） | G05 | 5d3 | 0 | 已刪除：改用 `AppIcon`（`@hawkr/icons` 的 Material Symbols），放在原本的框；無引用者直接刪除 |
| `website/public/icons/shipping/7-eleven.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `website/public/icons/shipping/blackcat.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `website/public/icons/shipping/ic_711.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `website/public/icons/shipping/ic_kuroneko.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `website/public/icons/shipping/ic_uber.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `website/public/icons/shipping/uber.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤或既有圖示（名稱在旁邊），圖框尺寸不變 |
| `website/public/images/certs/cas.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤（認證的短名稱）或既有的圖示框加文字；無引用者直接刪除 |
| `website/public/images/certs/gmp.webp` | webp | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤（認證的短名稱）或既有的圖示框加文字；無引用者直接刪除 |
| `website/public/images/certs/haccp.webp` | webp | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤（認證的短名稱）或既有的圖示框加文字；無引用者直接刪除 |
| `website/public/images/certs/iso.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤（認證的短名稱）或既有的圖示框加文字；無引用者直接刪除 |
| `website/public/images/certs/organic.webp` | webp | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤（認證的短名稱）或既有的圖示框加文字；無引用者直接刪除 |
| `website/public/images/certs/tap.png` | png | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤（認證的短名稱）或既有的圖示框加文字；無引用者直接刪除 |
| `website/public/images/certs/tap.webp` | webp | 來源不明（標章或商標） | G04 | 5a | 0 | 已刪除：改文字標籤（認證的短名稱）或既有的圖示框加文字；無引用者直接刪除 |
| `website/public/images/corporate-welfare/corporate-welfare-clay-hero.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/corporate-welfare/corporate-welfare-employee-discount.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/corporate-welfare/corporate-welfare-hero.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/images/corporate-welfare/corporate-welfare-holiday-benefit.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/corporate-welfare/corporate-welfare-local-good.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/corporate-welfare/corporate-welfare-office-groupbuy.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/corporate-welfare/scroll-world/harvest.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/corporate-welfare/scroll-world/harvest.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/corporate-welfare/scroll-world/retail.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/corporate-welfare/scroll-world/retail.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/images/corporate-welfare/scroll-world/table.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/corporate-welfare/scroll-world/table.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/corporate-welfare/scroll-world/welfare.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/corporate-welfare/scroll-world/welfare.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/autumn/egg-dairy.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/autumn/flowers.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/autumn/fruits.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/autumn/grains.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/autumn/meat.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/autumn/processed.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/autumn/seafood.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/autumn/spices.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/autumn/vegetables.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/generic.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/spring/egg-dairy.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/spring/flowers.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/spring/fruits.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/spring/grains.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/spring/meat.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/spring/processed.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/spring/seafood.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/spring/spices.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/spring/vegetables.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/summer/egg-dairy.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/summer/flowers.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/summer/fruits.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/summer/grains.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/summer/meat.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/summer/processed.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/summer/seafood.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/summer/spices.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/summer/vegetables.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/winter/egg-dairy.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/winter/flowers.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/winter/fruits.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/winter/grains.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/winter/meat.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/winter/processed.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/winter/seafood.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/winter/spices.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/conversation-category-marbles/winter/vegetables.frame-164.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/entry-thinking-cover.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/autumn/01-vegetables-water-bamboo.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/autumn/02-fruits-pomelo.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/autumn/03-seafood-pacific-saury.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/autumn/04-meat-duck-breast.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/autumn/05-egg-dairy-cheese-wheel.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/autumn/06-processed-dried-fruit.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/autumn/07-spices-red-chili.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/autumn/08-grains-rice-ear.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/autumn/09-flowers-chrysanthemum.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/hero/autumn-vegetables.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/hero/spring-vegetables.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/hero/summer-vegetables.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/hero/winter-vegetables.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/spring/01-vegetables-spring-bamboo-shoot.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/spring/02-fruits-spring-green-plum.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/spring/03-seafood-spring-flying-fish.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/spring/04-meat-spring-herb-chicken-leg.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/spring/05-egg-dairy-spring-soft-boiled-egg.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/spring/06-processed-spring-tofu-cube.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/spring/07-spices-spring-garlic.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/spring/08-grains-spring-wheat-ear.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/spring/09-flowers-spring-white-calla-lily.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/summer/01-vegetables-summer-okra.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/summer/02-fruits-summer-irwin-mango.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/summer/03-seafood-summer-spear-squid.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/summer/04-meat-summer-grilled-pork-belly.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/summer/05-egg-dairy-summer-yogurt-cup.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/summer/06-processed-summer-popsicle.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/summer/07-spices-summer-thai-basil.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/summer/08-grains-summer-corn.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/summer/09-flowers-summer-hydrangea.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/winter/01-vegetables-cauliflower.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/winter/02-fruits-jujube.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/winter/03-seafood-mullet.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/winter/04-meat-beef-hotpot-roll.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/winter/05-egg-dairy-butter-block.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/winter/06-processed-dumpling.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/winter/07-spices-ginger.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/winter/08-grains-red-bean-sack.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-contained-v3/winter/09-flowers-poinsettia.frame-0.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/01-vegetables-water-bamboo.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/01-vegetables-water-bamboo.w112-8b028a5d4cfe.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/01-vegetables-water-bamboo.w48-82769793e723.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/01-vegetables-water-bamboo.w96-190c32819143.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/01-vegetables-water-bamboo.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/02-fruits-pomelo.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/02-fruits-pomelo.w112-250480d066b4.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/02-fruits-pomelo.w48-6261ab307e14.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/02-fruits-pomelo.w96-d90d20753bf1.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/02-fruits-pomelo.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/03-seafood-pacific-saury.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/03-seafood-pacific-saury.w112-327d817ff062.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/03-seafood-pacific-saury.w48-0d0851d7644b.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/03-seafood-pacific-saury.w96-577a5dbdf6d5.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/03-seafood-pacific-saury.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/04-meat-duck-breast.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/04-meat-duck-breast.w112-a75166e6bf6f.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/04-meat-duck-breast.w48-e0ae1da22af6.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/04-meat-duck-breast.w96-0be4948695a2.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/04-meat-duck-breast.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/05-egg-dairy-cheese-wheel.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/05-egg-dairy-cheese-wheel.w112-39411a763281.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/05-egg-dairy-cheese-wheel.w48-47407940777c.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/05-egg-dairy-cheese-wheel.w96-bd3eb93e6324.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/05-egg-dairy-cheese-wheel.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/06-processed-dried-fruit.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/06-processed-dried-fruit.w112-300c8d32ae6e.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/06-processed-dried-fruit.w48-f491fe18e089.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/06-processed-dried-fruit.w96-1c57af0b7782.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/06-processed-dried-fruit.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/07-spices-red-chili.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/07-spices-red-chili.w112-8863825cd80b.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/07-spices-red-chili.w48-988cc1b780da.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/07-spices-red-chili.w96-1b410ab99e3e.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/07-spices-red-chili.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/08-grains-rice-ear.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/08-grains-rice-ear.w112-2941f53d3331.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/08-grains-rice-ear.w48-d1050b21d89a.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/08-grains-rice-ear.w96-2901546b2486.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/08-grains-rice-ear.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/09-flowers-chrysanthemum.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/09-flowers-chrysanthemum.w112-f643d31de500.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/09-flowers-chrysanthemum.w48-f346f293a6b2.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/09-flowers-chrysanthemum.w96-b2570605d5d8.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/autumn/09-flowers-chrysanthemum.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/01-vegetables-spring-bamboo-shoot.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/01-vegetables-spring-bamboo-shoot.w112-f9cf614040ca.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/01-vegetables-spring-bamboo-shoot.w48-a2536c50e716.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/01-vegetables-spring-bamboo-shoot.w96-a94ff0ed6a21.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/01-vegetables-spring-bamboo-shoot.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/02-fruits-spring-green-plum.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/02-fruits-spring-green-plum.w112-46cb53a2849a.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/02-fruits-spring-green-plum.w48-afb472ef8d51.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/02-fruits-spring-green-plum.w96-d7796d92103c.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/02-fruits-spring-green-plum.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/03-seafood-spring-flying-fish.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/03-seafood-spring-flying-fish.w112-98de550faa9e.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/03-seafood-spring-flying-fish.w48-bd3eefdc2be2.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/03-seafood-spring-flying-fish.w96-2e49ddae0e82.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/03-seafood-spring-flying-fish.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/04-meat-spring-herb-chicken-leg.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/04-meat-spring-herb-chicken-leg.w112-0933ee91cdce.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/04-meat-spring-herb-chicken-leg.w48-1d067eb5cd87.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/04-meat-spring-herb-chicken-leg.w96-6c4bfea18b69.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/04-meat-spring-herb-chicken-leg.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/05-egg-dairy-spring-soft-boiled-egg.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/05-egg-dairy-spring-soft-boiled-egg.w112-02ded3e5c3b0.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/05-egg-dairy-spring-soft-boiled-egg.w48-36d4e3049965.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/05-egg-dairy-spring-soft-boiled-egg.w96-87d34abe0934.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/05-egg-dairy-spring-soft-boiled-egg.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/06-processed-spring-tofu-cube.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/06-processed-spring-tofu-cube.w112-d34a225e21a6.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/06-processed-spring-tofu-cube.w48-435fe7813203.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/06-processed-spring-tofu-cube.w96-5e5dc468cf22.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/06-processed-spring-tofu-cube.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/07-spices-spring-garlic.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/07-spices-spring-garlic.w112-6cc94c5ff8f6.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/07-spices-spring-garlic.w48-94f50a8197f4.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/07-spices-spring-garlic.w96-ba160fe31fef.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/07-spices-spring-garlic.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/08-grains-spring-wheat-ear.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/08-grains-spring-wheat-ear.w112-5cf53d5e440c.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/08-grains-spring-wheat-ear.w48-4a7485fae20b.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/08-grains-spring-wheat-ear.w96-7b088b7c4e84.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/08-grains-spring-wheat-ear.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/09-flowers-spring-white-calla-lily.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/09-flowers-spring-white-calla-lily.w112-b1a68b17b68c.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/09-flowers-spring-white-calla-lily.w48-07a1990a8921.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/09-flowers-spring-white-calla-lily.w96-0cea3dad2e3d.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/spring/09-flowers-spring-white-calla-lily.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/01-vegetables-summer-okra.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/01-vegetables-summer-okra.w112-62515c43d46d.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/01-vegetables-summer-okra.w48-b0710934124a.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/01-vegetables-summer-okra.w96-2db108a93088.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/01-vegetables-summer-okra.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/02-fruits-summer-irwin-mango.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/02-fruits-summer-irwin-mango.w112-405f4b70a59f.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/02-fruits-summer-irwin-mango.w48-e34a55b0d5f4.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/02-fruits-summer-irwin-mango.w96-512622b3f712.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/02-fruits-summer-irwin-mango.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/03-seafood-summer-spear-squid.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/03-seafood-summer-spear-squid.w112-9232619da9d9.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/03-seafood-summer-spear-squid.w48-44848e64a133.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/03-seafood-summer-spear-squid.w96-d3c9033194fb.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/03-seafood-summer-spear-squid.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/04-meat-summer-grilled-pork-belly.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/04-meat-summer-grilled-pork-belly.w112-9f225dbaad29.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/04-meat-summer-grilled-pork-belly.w48-1e699c0d8ff8.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/04-meat-summer-grilled-pork-belly.w96-ec81989f4750.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/04-meat-summer-grilled-pork-belly.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/05-egg-dairy-summer-yogurt-cup.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/05-egg-dairy-summer-yogurt-cup.w112-6269fdee1731.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/05-egg-dairy-summer-yogurt-cup.w48-721dbec7393f.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/05-egg-dairy-summer-yogurt-cup.w96-84eea30911ba.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/05-egg-dairy-summer-yogurt-cup.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/06-processed-summer-popsicle.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/06-processed-summer-popsicle.w112-76e4850b724b.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/06-processed-summer-popsicle.w48-da4bfd4c99ae.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/06-processed-summer-popsicle.w96-1c237f6f8f1e.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/06-processed-summer-popsicle.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/07-spices-summer-thai-basil.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/07-spices-summer-thai-basil.w112-c3e3512d203a.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/07-spices-summer-thai-basil.w48-a2c749c9cc2b.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/07-spices-summer-thai-basil.w96-cfe7b8630c9c.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/07-spices-summer-thai-basil.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/08-grains-summer-corn.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/08-grains-summer-corn.w112-6696a49c4f8c.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/08-grains-summer-corn.w48-4b4a28a8d7eb.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/08-grains-summer-corn.w96-d62c1e55b25e.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/08-grains-summer-corn.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/09-flowers-summer-hydrangea.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/09-flowers-summer-hydrangea.w112-091d6860ce92.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/09-flowers-summer-hydrangea.w48-142263d7e86c.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/09-flowers-summer-hydrangea.w96-80bca2a411f4.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/summer/09-flowers-summer-hydrangea.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/01-vegetables-cauliflower.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/01-vegetables-cauliflower.w112-f663b649a4b2.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/01-vegetables-cauliflower.w48-e8e8dbd741a2.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/01-vegetables-cauliflower.w96-a57b5036118a.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/01-vegetables-cauliflower.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/02-fruits-jujube.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/02-fruits-jujube.w112-f7d5e5248417.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/02-fruits-jujube.w48-c82329146240.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/02-fruits-jujube.w96-fc5ca1820538.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/02-fruits-jujube.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/03-seafood-mullet.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/03-seafood-mullet.w112-1589362449d8.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/03-seafood-mullet.w48-76e5f3aa75e6.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/03-seafood-mullet.w96-b511c45abff9.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/03-seafood-mullet.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/04-meat-beef-hotpot-roll.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/04-meat-beef-hotpot-roll.w112-e6507a297a8d.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/04-meat-beef-hotpot-roll.w48-3c8470a0c295.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/04-meat-beef-hotpot-roll.w96-1c2976213ab0.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/04-meat-beef-hotpot-roll.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/05-egg-dairy-butter-block.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/05-egg-dairy-butter-block.w112-3982b9d76747.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/05-egg-dairy-butter-block.w48-b9b357ec65a5.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/05-egg-dairy-butter-block.w96-93e3efb0eb96.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/05-egg-dairy-butter-block.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/06-processed-dumpling.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/06-processed-dumpling.w112-3d3c81f6a962.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/06-processed-dumpling.w48-5ba552820543.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/06-processed-dumpling.w96-f4d905066d6f.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/06-processed-dumpling.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/07-spices-ginger.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/07-spices-ginger.w112-65c0e53685a8.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/07-spices-ginger.w48-f868632f3b68.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/07-spices-ginger.w96-2f45cdb08746.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/07-spices-ginger.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/08-grains-red-bean-sack.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/08-grains-red-bean-sack.w112-f4b900612de2.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/08-grains-red-bean-sack.w48-5dc1b65c9a89.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/08-grains-red-bean-sack.w96-e6a09463839a.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/08-grains-red-bean-sack.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/09-flowers-poinsettia.png` | png | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/09-flowers-poinsettia.w112-a5a5aa4f6e19.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/09-flowers-poinsettia.w48-a9726f48b405.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/09-flowers-poinsettia.w96-98069b4adce9.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/winter/09-flowers-poinsettia.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/illustrations/scene-greenhouse.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/home-design/illustrations/scene-irrigation.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/home-design/illustrations/scene-market.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/home-design/illustrations/scene-orchard.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/home-design/illustrations/scene-sunrise.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/home-design/illustrations/scene-sunset.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/home-design/mascots/chef-jam.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/cowboy-pig.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/farmer-pitchfork.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/farmer-seedling.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/farmer-tomato-new.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/farmer-wheat.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/fisherman-fish.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/florist-bouquet.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/girl-apple.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/girl-milk-hover.gif` | gif | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/girl-milk.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/girl-orchard.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/nav/chef-jam.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/nav/cowboy-pig.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/nav/farmer-tomato-new.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/nav/fisherman-fish.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/nav/florist-bouquet.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/nav/girl-apple.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/nav/girl-milk.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/nav/rice-farmer.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/nav/tea-farmer.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/rice-farmer.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/mascots/tea-farmer.webp` | webp | 來源不明（舊角色素材） | G06 | 5c | 0 | 已刪除：位置改放 Hawkr 標誌（BrandLogo、BrandLogoLoader）或 IconlyIcon 圖示，放在原本的框；無引用者直接刪除 |
| `website/public/images/home-design/page-bg-tall-mobile.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/images/home-design/page-bg-tall.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/images/home-design/phone-orders.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/home-design/phone-product.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/home-design/phone-search.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/home/seasonal-fruits-atlas-v1.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0010-cover.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0030-contents.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0100-chapter-1.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0120-onboarding-shop-info.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0121-onboarding-otp.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0122-onboarding-complete.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0200-chapter-2.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0210-seller-shell-desktop.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0220-dashboard-overview.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0230-launch-tasks.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0300-chapter-3.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0310-shop-settings-copy.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0311-shop-settings-media.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0320-kyc-type.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0321-kyc-form-status.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0330-wallet-balance.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0341-tcat-address-status.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0350-cvs-shipping.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0400-chapter-4.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0410-product-list.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0420-product-new-basic.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0441-import-upload.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0442-import-job.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0443-import-batch-actions.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0444-import-row-edit.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0450-inventory.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0500-chapter-5.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0510-order-list.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0520-order-detail.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0521-order-fulfillment.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0522-order-exception-cancel.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0530-cvs-code.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0531-cvs-proof.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0535-shipment-anomaly.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0541-aftersales-status.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0600-chapter-6.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0610-coupon-list.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0620-coupon-new.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0630-coupon-readonly.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0640-post-list.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0641-post-detail.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0650-post-editor.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0700-chapter-7.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0710-customer-list.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0720-customer-detail.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0800-chapter-8.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0810-message-workbench.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/0820-chat-room.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/1000-chapter-10.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/1010-notifications.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/1020-notification-preferences.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/1030-ai-tools.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/1100-appendix.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/1120-appendix-first-run.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide-book/1130-appendix-limits.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/00-onboarding.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/00-onboarding.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/02-seller-landing.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/02-seller-landing.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/04-shop-setup.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/04-shop-setup.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/05-checklist.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/05-checklist.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/06-shop-profile.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/06-shop-profile.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/07-certification.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/07-certification.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/08-wallet.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/08-wallet.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/09-shipping.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/09-shipping.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/09b-cvs-store.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/09b-cvs-store.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/10-products.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/10-products.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/11-product-create.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/11-product-create.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/12-product-images.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/12-product-images.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/15-inventory.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/15-inventory.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/16-post-new.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/16-post-new.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/17-coupon-new.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/17-coupon-new.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/18-orders.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/18-orders.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/19-order-detail.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/19-order-detail.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/20-fulfillment.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/20-fulfillment.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/21-aftersales.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/21-aftersales.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/22-customers.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/22-customers.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/23-messages.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/23-messages.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-guide/24-insights.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-guide/24-insights.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/images/seller-onboarding/app-dashboard.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/app-notifications.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/app-orders.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/app-products.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/process-apply.png` | png | 來源不明（公司時期圖片） | G11 | 5e2 | 0（路徑比對；檔名字串另見 1 檔，是無關的網址或欄位值，已人工確認） | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/registration-lucky-character.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/scroll-world/v1/posters/birth.jpg` | jpg | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/scroll-world/v1/posters/distribution.jpg` | jpg | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/scroll-world/v1/posters/harvest.jpg` | jpg | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/scroll-world/v1/posters/processing.jpg` | jpg | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/scroll-world/v1/posters/retail.jpg` | jpg | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/scroll-world/v1/posters/sorting.jpg` | jpg | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/scroll-world/v1/posters/table.jpg` | jpg | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/scroll-world/v1/vid/birth.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-onboarding/scroll-world/v1/vid/distribution.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-onboarding/scroll-world/v1/vid/harvest.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-onboarding/scroll-world/v1/vid/processing.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-onboarding/scroll-world/v1/vid/retail.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-onboarding/scroll-world/v1/vid/sorting.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-onboarding/scroll-world/v1/vid/table.mp4` | mp4 | 來源不明（操作錄影） | G08 | 5a | 0 | 已刪除：移除播放器，畫面改顯示既有的 poster 圖；沒有 poster 的場景只留淡入淡出 |
| `website/public/images/seller-onboarding/web-dashboard.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/web-notifications.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/web-orders.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-onboarding/web-products.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |
| `website/public/images/seller-pricing/hero-ledger-village.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/images/seller-pricing/market-difference-village.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：無引用：無引用，直接刪除（只在 pubspec 宣告的連宣告一起刪，只在測試字串提到的測試一起改） |
| `website/public/images/seller-pricing/petals/village-vision.webp` | webp | 來源不明（公司時期圖片） | G11 | 5e2 | 0 | 已刪除：照片與插畫：換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（`public/placeholders/`） |

## 2. 文件逐檔

- 數量：525 個檔（`.md`、`.mdx`、`.rst`、`.txt`，或在任一 `docs/` 目錄且非程式、非素材）。
- 指令：`git ls-files | grep -E '\.(md|mdx|rst|txt)$'` 加上 `docs/` 目錄下的非程式檔；清洗類別由 `python3 backend/docs/plans/2026-10-24-portfolio-public/scan.py --category brand,colleague,merchant,pii --report` 的逐檔結果決定；作者占比由 `python3 backend/docs/plans/2026-10-24-portfolio-public/scan.py --category authors --report`（`git log HEAD --name-only` 依 Will 與他人分 commit 數）。
- 快照：e5d5a8d8e（程式碼與 332da5e51 相同；之後只新增了計畫與 ADR）。清洗類別：品牌（舊名、縮寫、法人）、人名（同事）、特店資訊（特店代號、金鑰名稱、測試帳號的值）；個資另見第 3 節。作者欄是觸及該檔的 commit 數（未追蹤改名，改名前的歷史算在舊路徑）。
- D1：全部保留，清洗後公開；不移除文件。

目錄層級的作者占比（commit 數）：

| 目錄 | Will | 他人 | 他人占比 | 去向 |
|---|---|---|---|---|
| `backend/docs/詳細設計書` | 469 | 1256 | 72% | （統計，不是待辦）保留 |
| `backend/docs/plans` | 575 | 0 | 0% | （統計，不是待辦）保留 |
| `backend/docs/代碼管理（CodeMeta）` | 123 | 278 | 69% | （統計，不是待辦）保留 |
| `backend/internal` | 346 | 0 | 0% | （統計，不是待辦）保留 |
| `backend/docs/adr` | 258 | 63 | 19% | （統計，不是待辦）保留 |
| `backend/docs/開發規範` | 134 | 114 | 45% | （統計，不是待辦）保留 |
| `backend/docs/基礎建設` | 87 | 33 | 27% | （統計，不是待辦）保留 |
| `website/docs` | 52 | 30 | 36% | （統計，不是待辦）保留 |
| `backend/docs/_sidebar.md` | 26 | 43 | 62% | （統計，不是待辦）保留 |
| `backend/scripts` | 59 | 10 | 14% | （統計，不是待辦）保留 |
| `backend/services` | 44 | 20 | 31% | （統計，不是待辦）保留 |
| `backend/docs/基本設計書` | 7 | 54 | 88% | （統計，不是待辦）保留 |
| `backend/docs/runbooks` | 49 | 10 | 16% | （統計，不是待辦）保留 |
| `backend/AGENTS.md` | 51 | 6 | 10% | （統計，不是待辦）保留 |
| `backend/docs/index.md` | 13 | 31 | 70% | （統計，不是待辦）保留 |
| `AGENTS.md` | 35 | 0 | 0% | （統計，不是待辦）保留 |
| `backend/README.md` | 21 | 12 | 36% | （統計，不是待辦）保留 |
| `website/stories` | 15 | 16 | 51% | （統計，不是待辦）保留 |
| `website/README.md` | 17 | 12 | 41% | （統計，不是待辦）保留 |
| `backend/docs/第三方整合` | 11 | 17 | 60% | （統計，不是待辦）保留 |
| `backend/docs/速查參考` | 15 | 12 | 44% | （統計，不是待辦）保留 |
| `backend/infra` | 25 | 2 | 7% | （統計，不是待辦）保留 |
| `seller-app/docs` | 16 | 11 | 40% | （統計，不是待辦）保留 |
| `website/design-system` | 11 | 16 | 59% | （統計，不是待辦）保留 |
| `backend/docs/補充資料` | 8 | 18 | 69% | （統計，不是待辦）保留 |

| 文件 | 類型 | 品牌處數 | 人名處數 | 特店資訊處數 | 個資處數 | 作者(commit) | 去向 |
|---|---|---|---|---|---|---|---|
| `.agents/skills/backend-e2e-verify/SKILL.md` | agent skill | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `.agents/skills/website-e2e-verify/SKILL.md` | agent skill | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `.agents/skills/website-e2e-verify/references/pricebook-market-watchlist.md` | agent skill | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 35／他人 0 | 保留（無需清洗） |
| `README.md` | README | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `THIRD_PARTY_NOTICES.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `admin/.impeccable/surfaces/src-app-admin-merchants-id-operate-page-tsx.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 2／他人 2 | 保留（無需清洗） |
| `admin/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 19／他人 5 | 保留（無需清洗） |
| `admin/DESIGN.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 4／他人 1 | 保留（無需清洗） |
| `admin/PRODUCT.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 4／他人 1 | 保留（無需清洗） |
| `admin/README.md` | README | 0 | 0 | 0 | 0 | Will 17／他人 7 | 保留（無需清洗） |
| `admin/src/app/fonts/OFL-1.1-Roboto.txt` | 其他文件 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 51／他人 6 | 保留（無需清洗） |
| `backend/QUICKSTART.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 12／他人 3 | 保留（無需清洗） |
| `backend/README.md` | README | 0 | 0 | 0 | 0 | Will 21／他人 12 | 保留（無需清洗） |
| `backend/api/proto/README.md` | README | 0 | 0 | 0 | 0 | Will 1／他人 2 | 保留（無需清洗） |
| `backend/docs/_sidebar.md` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 26／他人 43 | 保留（無需清洗） |
| `backend/docs/adr/2026-06-29-identity-defer-store-entity.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-07-07-identity-distributed-principal.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-07-11-identity-store-stint-symmetry.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-07-20-seller-kyc-pii-encryption-scope.md` | ADR | 0 | 0 | 0 | 0 | Will 3／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-08-19-identity-current-store-id-rpc.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-08-22-agent-runtime-delivery-decisions.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-08-22-agent-runtime-v6-divergence.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-08-29-membership-billing-payuni-period.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-06-fulfillment-docs-split-into-six.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-06-uber-webhook-slice-in-payuni-webhook-service.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-11-channel-fee-grossup-in-adapter.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-11-free-shipping-subsidy-attribution.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-11-ledger-carrier-model-compatibility.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-11-logistics-amount-semantics-separation.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-11-logistics-cost-true-up.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-11-logistics-finance-domain-ownership.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-11-payuni-channel-fee-bearer.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-11-responsibility-based-cost-attribution.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-11-uber-commerce-model-seller-direct.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-11-uber-logistics-fee-charge-trigger.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-12-inbound-payload-raw-bytes.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-12-return-leg-classifier-canonical.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-13-category-alias-offline-generation.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-13-procurement-category-fallback.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-14-pricebook-freshness-describes-age-not-usability.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-14-uber-create-idempotency-precommit-key.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-account-blacklist-writer-in-usecase.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-adm-delegation-authority-test.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-adm-delegation-authz-propagation.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-adm-delegation-service-ownership.md` | ADR | 0 | 0 | 0 | 0 | Will 4／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-adm-delegation-session-lifecycle.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-adm-delegation-suspension-and-notification.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-adm-delegation-workspace-host.md` | ADR | 0 | 0 | 0 | 0 | Will 3／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-ai-service-architecture-decisions.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-app-version-gate-decisions.md` | ADR | 0 | 0 | 0 | 0 | Will 5／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-apple-oauth-verification-strategy.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-buyer-membership-entitlement-model.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-checkout-free-shipping-and-degradation.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-content-moderation-consolidation.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-deal-temperature-outcome-definition.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-ga4-cross-platform-analytics.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-holiday-cache-and-coverage-monitoring.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-homepage-dynamic-layout-ranking.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-im-websocket-delivery-architecture.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-internal-auth-single-shared-hmac-secret.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-invoice-service-architecture.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-media-storage-r2-image-gateway.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-mitake-sms-otp-backend-issued.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-mq-dedicated-exchange-per-domain.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-mq-social-counter-async-delta.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-order-outbox-over-failing-publish-variant.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-procurement-assistant-mvp-a-decisions.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-procurement-llm-extract-retry-and-timeout.md` | ADR | 1 | 0 | 0 | 0 | Will 1／他人 1 | 保留，清洗：品牌 |
| `backend/docs/adr/2026-09-18-product-view-stats-ownership-and-storage.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-rfq-acceptance-order-atomicity.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-rfq-snapshot-hash-preimage.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-seller-capability-authorization.md` | ADR | 0 | 0 | 0 | 0 | Will 3／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-seller-governance-schema-and-enforcement.md` | ADR | 0 | 0 | 0 | 0 | Will 3／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-seo-structured-data-and-canonical-source.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-shipping-fee-cache-no-warmup-no-scheduler.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-sse-push-platform-architecture.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-subscription-hold-expiry-redundant-column.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-web-session-ssot-and-cache-layering.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-18-webhook-two-phase-transaction.md` | ADR | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-20-coordinator-vs-all-tools-fallback.md` | ADR | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-20-seller-stint-assistant-history.md` | ADR | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-24-go-modular-monolith.md` | ADR | 0 | 0 | 0 | 0 | Will 14／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-25-api-error-model.md` | ADR | 0 | 0 | 0 | 0 | Will 7／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-25-api-image-uri-template.md` | ADR | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-25-codemeta-retirement.md` | ADR | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-25-openapi-toolchain.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-25-optimistic-concurrency-etag.md` | ADR | 0 | 0 | 0 | 0 | Will 6／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-25-row-identity-by-data-kind.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-26-app-ios-only.md` | ADR | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-26-external-contract-protobuf-connect.md` | ADR | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-27-app-version-gate-shared-package.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-27-delegation-module.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-27-identity-clerk.md` | ADR | 0 | 0 | 0 | 0 | Will 8／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-27-identity-opaque-sessions.md` | ADR | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-27-module-internal-structure.md` | ADR | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-29-seller-suspension-and-business-identity-in-monolith.md` | ADR | 0 | 0 | 0 | 0 | Will 6／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-30-account-created-provisions-buyer.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-09-30-account-deletion-module.md` | ADR | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-02-seller-governance-in-monolith.md` | ADR | 0 | 0 | 0 | 0 | Will 15／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-02-seller-kyc-payuni-in-monolith.md` | ADR | 0 | 0 | 0 | 0 | Will 14／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-03-media-references-owner-side.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-03-public-store-identity-and-visibility.md` | ADR | 0 | 0 | 0 | 0 | Will 5／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-03-recommended-sellers-scoring.md` | ADR | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-03-vendor-portal-invite-scoped-access.md` | ADR | 0 | 0 | 0 | 0 | Will 6／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-04-address-town-code.md` | ADR | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-04-admin-roles.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-04-prospect-contact-points-and-matching.md` | ADR | 0 | 0 | 0 | 0 | Will 5／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-05-notification-module.md` | ADR | 0 | 0 | 0 | 0 | Will 5／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-05-store-exposure-to-legacy.md` | ADR | 0 | 0 | 0 | 0 | Will 5／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-07-connect-without-perspective.md` | ADR | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-09-store-seo-platform-generated.md` | ADR | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-17-multi-locale-architecture.md` | ADR | 0 | 0 | 0 | 0 | Will 7／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-17-social-actor-identity.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-18-seller-governance-consolidated.md` | ADR | 0 | 0 | 0 | 0 | Will 10／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-19-seller-kyc-payuni-consolidated.md` | ADR | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-20-admin-review-queue.md` | ADR | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-23-social-counts-in-stats-tables.md` | ADR | 0 | 0 | 0 | 0 | Will 5／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/2026-10-24-portfolio-public.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/adr/README.md` | ADR | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/api-fixtures/pricebook-trend-summary/group-ready-two-markets.json` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/api-fixtures/pricebook-trend-summary/insufficient-data.json` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/api-fixtures/pricebook-trend-summary/market-closed-week.json` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/api-fixtures/pricebook-trend-summary/missing-baseline-yoy364.json` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/api-fixtures/pricebook-trend-summary/partial-current-week.json` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/api-fixtures/pricebook-trend-summary/ready-crop-fresh.json` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/api-fixtures/pricebook-trend-summary/ready-crop-week.json` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/index.md` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 13／他人 31 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-24-go-modular-monolith.md` | 計畫 | 0 | 0 | 0 | 0 | Will 59／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-24-go-modular-monolith/README.md` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-24-go-modular-monolith/cochange.py` | 計畫 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-24-go-modular-monolith/db-tables.csv` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-24-go-modular-monolith/event-anomalies.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-24-go-modular-monolith/event-pairs.csv` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-24-go-modular-monolith/events.csv` | 計畫 | 0 | 0 | 0 | 0 | Will 7／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-24-go-modular-monolith/external-systems.csv` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-24-go-modular-monolith/flows.md` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-24-go-modular-monolith/idrefs.csv` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-24-go-modular-monolith/replicas.csv` | 計畫 | 0 | 0 | 0 | 0 | Will 8／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-24-go-modular-monolith/rpc-calls.csv` | 計畫 | 0 | 0 | 0 | 0 | Will 8／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-24-go-modular-monolith/rpc-method.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-24-go-modular-monolith/rpc-pairs.csv` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-24-go-modular-monolith/rpc-servers.csv` | 計畫 | 0 | 0 | 0 | 0 | Will 13／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-25-connect-contract-pilot.md` | 計畫 | 0 | 0 | 0 | 0 | Will 33／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-27-a2-identity.md` | 計畫 | 0 | 0 | 0 | 0 | Will 20／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller.md` | 計畫 | 0 | 0 | 0 | 0 | Will 111／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/buyer-api.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/buyer-inventory.md` | 計畫 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/enum-text-guard-design.md` | 計畫 | 0 | 0 | 0 | 0 | Will 5／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/gating-decision-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/public-read-backend-inventory.md` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/public-read-client-inventory.md` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/seller-api-client-inventory.md` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/seller-api-gateway-inventory.md` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/seller-domain-inventory.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/seller-interface-inventory.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10-decision-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10a-decision-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10b-decision-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10c-decision-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 6／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10d-2b-decisions.md` | 計畫 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10d-3a-actor-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10d-3a-decision-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10d-3a-owner-surface-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10d-a7-social-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10d-d3b-decision-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10d-decision-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-2c-design.md` | 計畫 | 0 | 0 | 0 | 0 | Will 7／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-2c2-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-2c3-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3-baseline-check.md` | 計畫 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3-brief-v1.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3-research.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3ca-design.md` | 計畫 | 0 | 0 | 0 | 0 | Will 8／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3cb-design.md` | 計畫 | 0 | 0 | 0 | 0 | Will 19／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3cd-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3cd-inventory.md` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-design-review-start.md` | 計畫 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-design.md` | 計畫 | 0 | 0 | 0 | 0 | Will 6／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-industry-research.md` | 計畫 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-inventory.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-queue-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-queue-review1.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-queue-review2.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-4a-design.md` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-4b-design.md` | 計畫 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-5a-design.md` | 計畫 | 0 | 0 | 2 | 0 | Will 2／他人 0 | 保留，清洗：特店資訊 |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-5b-design.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-5c-design.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-5d-design.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-L-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-L0-copy.md` | 計畫 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-Lprime-design.md` | 計畫 | 0 | 0 | 0 | 0 | Will 11／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-decision-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 6／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-i18n-brief-v1.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-i18n-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-i18n-research-repo.md` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-i18n-research-web.md` | 計畫 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-labels-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-labels-research.md` | 計畫 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-measure-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-reports-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 9／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-reports-facts.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-reports-research.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-seo-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social.md` | 計畫 | 0 | 0 | 0 | 0 | Will 34／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/alignment-facts.md` | 計畫 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/block-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/check-inventory.py` | 計畫 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/client-inventory.md` | 計畫 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/count-rest-ops.py` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/counts-baseline.md` | 計畫 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/counts-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 6／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/d3-read-model-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/decision-brief.md` | 計畫 | 0 | 0 | 0 | 0 | Will 6／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/domain-inventory.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/interface-inventory.md` | 計畫 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/step-1a-builder-baseline.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/step-1a-design.md` | 計畫 | 0 | 0 | 0 | 0 | Will 7／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/step-1a-perf/README.md` | 計畫 | 0 | 0 | 0 | 0 | Will 8／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/step-1a-perf/explain-hashtag-posts.txt` | 計畫 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/step-1a-perf/explain-selected.txt` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/step-1a-perf/seed.sql` | 計畫 | 0 | 0 | 0 | 0 | Will 6／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/step-1a-perf/spike-results.txt` | 計畫 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/step-1a-perf/spike-trgm.sql` | 計畫 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/step-1b-perf/README.md` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/step-1b-perf/seed-blocks.sql` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/plans/2026-10-18-a7-social/step-1b-perf/seed-heavy-viewer.sql` | 計畫 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/docs/postmortems/food-safety/_template.md` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 3／他人 1 | 保留（無需清洗） |
| `backend/docs/runbooks/food-safety-incident-runbook.md` | runbook | 0 | 0 | 0 | 0 | Will 10／他人 0 | 保留（無需清洗） |
| `backend/docs/runbooks/food-safety/sql/01-list-pending.sql` | runbook | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/runbooks/food-safety/sql/02-ack.sql` | runbook | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/runbooks/food-safety/sql/03-patch-status.sql` | runbook | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/runbooks/food-safety/sql/06-restore-suspension.sql` | runbook | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/runbooks/food-safety/sql/07-decide-appeal.sql` | runbook | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/runbooks/food-safety/sql/08-notify-seller.sql` | runbook | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/runbooks/food-safety/sql/09-mark-fda-reported.sql` | runbook | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/docs/runbooks/invoice-cancelled-orders-backfill.md` | runbook | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/runbooks/invoice-inbox-stranded-runbook.md` | runbook | 0 | 0 | 0 | 0 | Will 0／他人 4 | 保留（無需清洗） |
| `backend/docs/runbooks/product-rekey-backfill-gate.md` | runbook | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/docs/runbooks/seller-food-registration-runbook.md` | runbook | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/runbooks/seller-governance-shared.md` | runbook | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/runbooks/seller-notification-identity-store-id-backfill.md` | runbook | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/runbooks/seller-violation-management-runbook.md` | runbook | 0 | 0 | 0 | 0 | Will 13／他人 0 | 保留（無需清洗） |
| `backend/docs/runbooks/uber-quote-retention.md` | runbook | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/runbooks/uber-quote-retention/sql/01-preview.sql` | runbook | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/runbooks/uber-quote-retention/sql/02-delete.sql` | runbook | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/runbooks/uber-quote-retention/sql/03-verify.sql` | runbook | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/代碼管理（CodeMeta）/README.md` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 11／他人 14 | 保留（無需清洗） |
| `backend/docs/代碼管理（CodeMeta）/scripts/generate.sh` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 27／他人 30 | 保留（無需清洗） |
| `backend/docs/代碼管理（CodeMeta）/內部技術碼.md` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 8／他人 63 | 保留（無需清洗） |
| `backend/docs/代碼管理（CodeMeta）/業務狀態碼.md` | backend 其他文件 | 8 | 0 | 0 | 0 | Will 29／他人 47 | 保留，清洗：品牌 |
| `backend/docs/代碼管理（CodeMeta）/業務類型碼.md` | backend 其他文件 | 2 | 0 | 0 | 0 | Will 22／他人 65 | 保留，清洗：品牌 |
| `backend/docs/代碼管理（CodeMeta）/踩坑與變遷紀錄.md` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/docs/代碼管理（CodeMeta）/錯誤碼.md` | backend 其他文件 | 1 | 0 | 0 | 0 | Will 25／他人 58 | 保留，清洗：品牌 |
| `backend/docs/基本設計書/00-總覽.md` | 基本設計書 | 0 | 0 | 0 | 0 | Will 0／他人 11 | 保留（無需清洗） |
| `backend/docs/基本設計書/06-物流微服務（Shipping Service）基本設計書.md` | 基本設計書 | 0 | 0 | 0 | 0 | Will 1／他人 2 | 保留（無需清洗） |
| `backend/docs/基本設計書/56-ADM 代行營運（Delegated Operations）基本設計書.md` | 基本設計書 | 0 | 0 | 0 | 0 | Will 3／他人 1 | 保留（無需清洗） |
| `backend/docs/基本設計書/58-電子發票服務（Invoice Service）基本設計書.md` | 基本設計書 | 0 | 0 | 0 | 0 | Will 2／他人 3 | 保留（無需清洗） |
| `backend/docs/基本設計書/65-物流履約平台（Fulfillment Platform）基本設計書.md` | 基本設計書 | 0 | 0 | 0 | 0 | Will 1／他人 10 | 保留（無需清洗） |
| `backend/docs/基本設計書/66-Uber Direct 即時配送整合基本設計書.md` | 基本設計書 | 0 | 0 | 0 | 0 | Will 0／他人 15 | 保留（無需清洗） |
| `backend/docs/基本設計書/67-定期配送訂閱（Subscription Delivery）基本設計書.md` | 基本設計書 | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/基本設計書/68-7-ELEVEN 大宗寄倉（B2C）基本設計書.md` | 基本設計書 | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/基本設計書/69-商家自有出貨（Merchant Self-Delivery）基本設計書.md` | 基本設計書 | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/基本設計書/70-物流財務結算（Logistics Settlement & Receivable）基本設計書.md` | 基本設計書 | 0 | 0 | 0 | 0 | Will 0／他人 9 | 保留（無需清洗） |
| `backend/docs/基礎建設/ATM 退款結果 Webhook 遺失維運處置.md` | 基礎建設 | 0 | 0 | 0 | 0 | Will 1／他人 2 | 保留（無需清洗） |
| `backend/docs/基礎建設/CI_CD 自動化部署（K8s + Helm）.md` | 基礎建設 | 0 | 0 | 0 | 0 | Will 3／他人 4 | 保留（無需清洗） |
| `backend/docs/基礎建設/Cloudflare CDN 與 DDoS 防護配置指南.md` | 基礎建設 | 0 | 0 | 0 | 0 | Will 5／他人 2 | 保留（無需清洗） |
| `backend/docs/基礎建設/Discord 告警通知設定指南.md` | 基礎建設 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/docs/基礎建設/Discord 監控 Bot 設計書.md` | 基礎建設 | 0 | 0 | 0 | 0 | Will 4／他人 2 | 保留（無需清洗） |
| `backend/docs/基礎建設/MQ Topology 管理（Terraform）.md` | 基礎建設 | 1 | 0 | 0 | 0 | Will 11／他人 5 | 保留，清洗：品牌 |
| `backend/docs/基礎建設/Prod RabbitMQ 3-Node Cluster.md` | 基礎建設 | 0 | 0 | 0 | 0 | Will 14／他人 1 | 保留（無需清洗） |
| `backend/docs/基礎建設/Tailscale VPN 連線 Prod 機器指南.md` | 基礎建設 | 0 | 0 | 0 | 0 | Will 7／他人 1 | 保留（無需清洗） |
| `backend/docs/基礎建設/業務告警分流與 dev-警報 路由設定.md` | 基礎建設 | 0 | 0 | 0 | 0 | Will 1／他人 2 | 保留（無需清洗） |
| `backend/docs/基礎建設/監控架構完整教學 (Monitoring Architecture Guide).md` | 基礎建設 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/docs/基礎建設/系統架構與資料流向.md` | 基礎建設 | 0 | 0 | 0 | 0 | Will 26／他人 11 | 保留（無需清洗） |
| `backend/docs/基礎建設/系統監控架構（Monitoring）.md` | 基礎建設 | 0 | 0 | 0 | 0 | Will 8／他人 3 | 保留（無需清洗） |
| `backend/docs/第三方整合/AMEGO 電子發票 API 總覽.md` | 供應商文件 | 0 | 0 | 3 | 6 | Will 2／他人 1 | 保留，清洗：特店資訊、個資 |
| `backend/docs/第三方整合/PayUni 代理商專用 API 總覽.md` | 供應商文件 | 0 | 0 | 0 | 0 | Will 2／他人 2 | 保留（無需清洗） |
| `backend/docs/第三方整合/PayUni 物流官方 API 總覽.md` | 供應商文件 | 0 | 0 | 0 | 1 | Will 0／他人 1 | 保留，清洗：個資 |
| `backend/docs/第三方整合/PayUni 費用一覽.md` | 供應商文件 | 0 | 0 | 0 | 2 | Will 0／他人 1 | 保留，清洗：個資 |
| `backend/docs/第三方整合/PayUni 逆向金流 API 總覽.md` | 供應商文件 | 0 | 0 | 0 | 0 | Will 0／他人 2 | 保留（無需清洗） |
| `backend/docs/第三方整合/PayUni 金流官方 API 總覽.md` | 供應商文件 | 0 | 0 | 0 | 0 | Will 1／他人 4 | 保留（無需清洗） |
| `backend/docs/第三方整合/TGOS API 整合指南.md` | 供應商文件 | 0 | 0 | 0 | 0 | Will 5／他人 0 | 保留（無需清洗） |
| `backend/docs/第三方整合/Uber Direct API 總覽.md` | 供應商文件 | 0 | 0 | 0 | 0 | Will 1／他人 6 | 保留（無需清洗） |
| `backend/docs/補充資料/45-首頁動態區塊排序與營運干預提案（PM版）.md` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 6／他人 0 | 保留（無需清洗） |
| `backend/docs/補充資料/product_import_e2e_sample.csv` | backend 其他文件 | 0 | 0 | 0 | 1 | Will 0／他人 8 | 保留，清洗：個資 |
| `backend/docs/補充資料/product_import_e2e_sample.md` | backend 其他文件 | 1 | 1 | 0 | 0 | Will 2／他人 9 | 保留，清洗：品牌、人名 |
| `backend/docs/補充資料/product_import_wizard_sample2.csv` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/docs/詳細設計書/00-總覽.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 29／他人 37 | 保留（無需清洗） |
| `backend/docs/詳細設計書/00-詳細設計書範本.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 4／他人 2 | 保留（無需清洗） |
| `backend/docs/詳細設計書/01-閘道微服務（Gateway）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 5／他人 10 | 保留（無需清洗） |
| `backend/docs/詳細設計書/02-產品微服務（Product Service）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 6／他人 62 | 保留（無需清洗） |
| `backend/docs/詳細設計書/03-訂單微服務（Order Service）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 6／他人 82 | 保留（無需清洗） |
| `backend/docs/詳細設計書/04-購物車微服務（Cart Service）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 1／他人 20 | 保留（無需清洗） |
| `backend/docs/詳細設計書/05-支付服務（Payment Service）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 4／他人 62 | 保留（無需清洗） |
| `backend/docs/詳細設計書/06-物流微服務（Shipping Service）詳細設計書.md` | 詳細設計書 | 2 | 0 | 2 | 1 | Will 3／他人 67 | 保留，清洗：品牌、特店資訊、個資 |
| `backend/docs/詳細設計書/07-優惠券微服務（Coupon Service）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 4／他人 24 | 保留（無需清洗） |
| `backend/docs/詳細設計書/08-多媒體微服務（Media Service）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 8／他人 22 | 保留（無需清洗） |
| `backend/docs/詳細設計書/09-售後微服務（AfterSales Service）詳細設計書.md` | 詳細設計書 | 6 | 0 | 0 | 0 | Will 3／他人 74 | 保留，清洗：品牌 |
| `backend/docs/詳細設計書/10-PayUni Webhook 服務（payuni-webhook-service）詳細設計書.md` | 詳細設計書 | 0 | 0 | 2 | 0 | Will 2／他人 18 | 保留，清洗：特店資訊 |
| `backend/docs/詳細設計書/11-PayUni Java Client（payuni-java-client）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 2／他人 9 | 保留（無需清洗） |
| `backend/docs/詳細設計書/12-通知微服務（Notification Service）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 7／他人 18 | 保留（無需清洗） |
| `backend/docs/詳細設計書/13-通知模板引擎與收件箱（Notification Template）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 21／他人 23 | 保留（無需清洗） |
| `backend/docs/詳細設計書/14-即時通訊微服務（IM Service）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 7／他人 6 | 保留（無需清洗） |
| `backend/docs/詳細設計書/15-AI 微服務（AI Service）詳細設計書.md` | 詳細設計書 | 12 | 0 | 0 | 0 | Will 27／他人 4 | 保留，清洗：品牌 |
| `backend/docs/詳細設計書/16-顧客管理微服務（CRM Service）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 8／他人 2 | 保留（無需清洗） |
| `backend/docs/詳細設計書/17-登入與帳號建立流程（Login & Account）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 19／他人 18 | 保留（無需清洗） |
| `backend/docs/詳細設計書/18-賣家實名認證（Merchant Certification）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 25／他人 8 | 保留（無需清洗） |
| `backend/docs/詳細設計書/19-農業認證驗證（Certification Verification）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 3／他人 14 | 保留（無需清洗） |
| `backend/docs/詳細設計書/20-結帳模組（Checkout Module）詳細設計書.md` | 詳細設計書 | 1 | 0 | 0 | 0 | Will 2／他人 23 | 保留，清洗：品牌 |
| `backend/docs/詳細設計書/21-運費計算模組（Shipping Fee Module）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 1／他人 9 | 保留（無需清洗） |
| `backend/docs/詳細設計書/22-商品配送驗證（Product Shipping Validation）詳細設計書.md` | 詳細設計書 | 1 | 0 | 0 | 0 | Will 0／他人 10 | 保留，清洗：品牌 |
| `backend/docs/詳細設計書/23-門市管理模組（Store Pickup Management）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 5 | Will 4／他人 5 | 保留，清洗：個資 |
| `backend/docs/詳細設計書/24-節假日管理（Holiday Management）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 1／他人 8 | 保留（無需清洗） |
| `backend/docs/詳細設計書/25-Banner 動作跳轉（Banner Action）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 2／他人 2 | 保留（無需清洗） |
| `backend/docs/詳細設計書/26-輸入驗證模組（Java InputValidator）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 1 | Will 2／他人 8 | 保留，清洗：個資 |
| `backend/docs/詳細設計書/27-地址搜尋與驗證（Address Proxy）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 7／他人 2 | 保留（無需清洗） |
| `backend/docs/詳細設計書/28-商品瀏覽統計（Product View Stats）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 6／他人 4 | 保留（無需清洗） |
| `backend/docs/詳細設計書/29-Admin 異常訂單管理 API 詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 1／他人 13 | 保留（無需清洗） |
| `backend/docs/詳細設計書/30-Sign in with Apple 認證（Apple OAuth）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 6／他人 3 | 保留（無需清洗） |
| `backend/docs/詳細設計書/31-內容檢舉與封鎖（Content Reporting & Block）詳細設計書.md` | 詳細設計書 | 5 | 0 | 0 | 0 | Will 3／他人 2 | 保留，清洗：品牌 |
| `backend/docs/詳細設計書/32-商家治理與食安通報（Seller Governance & Food Safety）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 28／他人 7 | 保留（無需清洗） |
| `backend/docs/詳細設計書/33-商品 Hashtag 模組（Product Hashtag）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 1／他人 8 | 保留（無需清洗） |
| `backend/docs/詳細設計書/34-貼文 Hashtag 模組（Post Hashtag）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 4／他人 2 | 保留（無需清洗） |
| `backend/docs/詳細設計書/35-商品列表排序與銷量反正規化（Product Sort & Sales Count）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 1／他人 8 | 保留（無需清洗） |
| `backend/docs/詳細設計書/36-MQ 與事件總覽（MQ & Event Catalog）.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 27／他人 108 | 保留（無需清洗） |
| `backend/docs/詳細設計書/38-Flyway 自動同步與 DB 自動初始化（Flyway Auto Sync & DB Auto Init）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 2／他人 4 | 保留（無需清洗） |
| `backend/docs/詳細設計書/39-買家服務（Buyer Service）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 8／他人 26 | 保留（無需清洗） |
| `backend/docs/詳細設計書/40-商品匯入精靈服務（Product Import Service）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 3／他人 50 | 保留（無需清洗） |
| `backend/docs/詳細設計書/41-Realtime SSE 推播平台（Realtime SSE Push Platform）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 3／他人 10 | 保留（無需清洗） |
| `backend/docs/詳細設計書/43-評論微服務（Review Service）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 1／他人 7 | 保留（無需清洗） |
| `backend/docs/詳細設計書/44-帳號與賣家買家身份模型（Account & Seller-Buyer Identity）詳細設計書.md` | 詳細設計書 | 2 | 0 | 0 | 0 | Will 18／他人 9 | 保留，清洗：品牌 |
| `backend/docs/詳細設計書/45-首頁區塊動態排序與營運干預（Homepage Dynamic Layout & Merchandising）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 6／他人 3 | 保留（無需清洗） |
| `backend/docs/詳細設計書/46-賣家能力權限層（Seller Capability Authorization）詳細設計書.md` | 詳細設計書 | 0 | 1 | 0 | 0 | Will 5／他人 15 | 保留，清洗：人名 |
| `backend/docs/詳細設計書/47-GA4 分析追蹤（GA4 Cross-Platform Analytics）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 4／他人 1 | 保留（無需清洗） |
| `backend/docs/詳細設計書/48-SEO 優化與現況（SEO Optimization）詳細設計書.md` | 詳細設計書 | 1 | 0 | 0 | 0 | Will 4／他人 1 | 保留，清洗：品牌 |
| `backend/docs/詳細設計書/49-多媒體儲存與圖片服務（Cloudflare R2 & Image Gateway）詳細設計書.md` | 詳細設計書 | 0 | 1 | 0 | 0 | Will 5／他人 13 | 保留，清洗：人名 |
| `backend/docs/詳細設計書/50-食譜內容管理（Recipe Content Management）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 6／他人 6 | 保留（無需清洗） |
| `backend/docs/詳細設計書/51-簡訊發送與手機 OTP（Mitake SMS & Phone OTP）詳細設計書.md` | 詳細設計書 | 0 | 1 | 0 | 0 | Will 6／他人 6 | 保留，清洗：人名 |
| `backend/docs/詳細設計書/52-微服務間內部授權（Internal Service Authorization）詳細設計書.md` | 詳細設計書 | 0 | 1 | 0 | 0 | Will 1／他人 4 | 保留，清洗：人名 |
| `backend/docs/詳細設計書/53-Web 前端快取架構與版本一致性（Web Frontend Cache & Deployment Consistency）詳細設計書.md` | 詳細設計書 | 1 | 1 | 0 | 0 | Will 3／他人 12 | 保留，清洗：品牌、人名 |
| `backend/docs/詳細設計書/54-App 版本檢查與強制更新（App Version Gate）.md` | 詳細設計書 | 0 | 1 | 0 | 0 | Will 8／他人 10 | 保留，清洗：人名 |
| `backend/docs/詳細設計書/55-Hawkr 幫找採購助手（Procurement Assistant）詳細設計書.md` | 詳細設計書 | 2 | 0 | 0 | 0 | Will 1／他人 0 | 保留，清洗：品牌 |
| `backend/docs/詳細設計書/56-ADM 代行營運（Delegated Operations）詳細設計書.md` | 詳細設計書 | 0 | 1 | 0 | 1 | Will 28／他人 22 | 保留，清洗：人名、個資 |
| `backend/docs/詳細設計書/57-Hawkr 幫找 Agent Runtime（Agent Runtime）詳細設計書.md` | 詳細設計書 | 5 | 0 | 0 | 0 | Will 1／他人 0 | 保留，清洗：品牌 |
| `backend/docs/詳細設計書/58-電子發票服務（Invoice Service）詳細設計書.md` | 詳細設計書 | 9 | 1 | 1 | 6 | Will 16／他人 50 | 保留，清洗：品牌、人名、特店資訊、個資 |
| `backend/docs/詳細設計書/58A-電子發票前端實作規格.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 8／他人 16 | 保留（無需清洗） |
| `backend/docs/詳細設計書/59-買家會員分級與 Agent 額度方案（Buyer Membership & Entitlement）詳細設計書.md` | 詳細設計書 | 1 | 0 | 0 | 0 | Will 9／他人 1 | 保留，清洗：品牌 |
| `backend/docs/詳細設計書/60-買家訂閱計費引擎（Buyer Subscription Billing Engine）詳細設計書.md` | 詳細設計書 | 3 | 0 | 0 | 0 | Will 8／他人 1 | 保留，清洗：品牌 |
| `backend/docs/詳細設計書/61-Pricebook 各品項自適應 P50 色階詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 4／他人 1 | 保留（無需清洗） |
| `backend/docs/詳細設計書/62-Pricebook 全市場品項行情.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 6／他人 1 | 保留（無需清洗） |
| `backend/docs/詳細設計書/64-Hawkr Agent 商務線（RFQ Publication、Seller Quote、Buyer Acceptance）詳細設計書.md` | 詳細設計書 | 4 | 0 | 0 | 0 | Will 2／他人 0 | 保留，清洗：品牌 |
| `backend/docs/詳細設計書/65-物流履約平台（Fulfillment Platform）詳細設計書.md` | 詳細設計書 | 14 | 0 | 0 | 0 | Will 4／他人 65 | 保留，清洗：品牌 |
| `backend/docs/詳細設計書/65A-物流履約前端實作規格.md` | 詳細設計書 | 1 | 0 | 0 | 0 | Will 1／他人 24 | 保留，清洗：品牌 |
| `backend/docs/詳細設計書/66-Uber Direct 即時配送整合詳細設計書.md` | 詳細設計書 | 4 | 0 | 0 | 0 | Will 2／他人 42 | 保留，清洗：品牌 |
| `backend/docs/詳細設計書/67-定期配送訂閱（Subscription Delivery）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 2／他人 6 | 保留（無需清洗） |
| `backend/docs/詳細設計書/68-7-ELEVEN 大宗寄倉（B2C）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 1／他人 17 | 保留（無需清洗） |
| `backend/docs/詳細設計書/69-商家自有出貨（Merchant Self-Delivery）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 2／他人 7 | 保留（無需清洗） |
| `backend/docs/詳細設計書/70-物流財務結算（Logistics Settlement & Receivable）詳細設計書.md` | 詳細設計書 | 0 | 0 | 0 | 0 | Will 1／他人 22 | 保留（無需清洗） |
| `backend/docs/速查參考/Apple Pay Merchant Cert 輪替 SOP.md` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 7／他人 2 | 保留（無需清洗） |
| `backend/docs/速查參考/Redis 快取管理.md` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 3／他人 3 | 保留（無需清洗） |
| `backend/docs/速查參考/前端屏幕與 API 映射表.md` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 1／他人 2 | 保留（無需清洗） |
| `backend/docs/速查參考/常用指令.md` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 2／他人 3 | 保留（無需清洗） |
| `backend/docs/速查參考/資料庫管理命令速查表.md` | backend 其他文件 | 0 | 0 | 0 | 0 | Will 2／他人 2 | 保留（無需清洗） |
| `backend/docs/開發規範/API 設計規範.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 6／他人 6 | 保留（無需清洗） |
| `backend/docs/開發規範/API 設計規範（Connect）.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 34／他人 0 | 保留（無需清洗） |
| `backend/docs/開發規範/Cloudflare Worker 與 Serverless 部署治理規範.md` | 開發規範 | 1 | 0 | 0 | 0 | Will 3／他人 11 | 保留，清洗：品牌 |
| `backend/docs/開發規範/DB Migration 相容性規範（expand-contract）.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 2／他人 2 | 保留（無需清洗） |
| `backend/docs/開發規範/Gateway Swagger 註解規範.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 4／他人 4 | 保留（無需清洗） |
| `backend/docs/開發規範/Git 分支與交付路徑規範.md` | 開發規範 | 0 | 3 | 0 | 0 | Will 2／他人 4 | 保留，清洗：人名 |
| `backend/docs/開發規範/Go gRPC 錯誤處理規範.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 8／他人 1 | 保留（無需清洗） |
| `backend/docs/開發規範/Java gRPC 錯誤處理規範.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 3／他人 8 | 保留（無需清洗） |
| `backend/docs/開發規範/Java 微服務開發規範.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 4／他人 11 | 保留（無需清洗） |
| `backend/docs/開發規範/MQ 與事件管理規範.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 4／他人 9 | 保留（無需清洗） |
| `backend/docs/開發規範/PayUni 錯誤回應收斂規範.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 0／他人 3 | 保留（無需清洗） |
| `backend/docs/開發規範/PostgreSQL 設計規範與最佳實踐.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 11／他人 3 | 保留（無需清洗） |
| `backend/docs/開發規範/Proto 管理規範.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 4／他人 3 | 保留（無需清洗） |
| `backend/docs/開發規範/Web 前端契約同步規範.md` | 開發規範 | 0 | 2 | 0 | 0 | Will 4／他人 3 | 保留，清洗：人名 |
| `backend/docs/開發規範/使用者面用語術語表.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 15／他人 0 | 保留（無需清洗） |
| `backend/docs/開發規範/固定 IP 出口與第三方轉發規範（Egress Proxy）.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 2／他人 3 | 保留（無需清洗） |
| `backend/docs/開發規範/多副本排程搶佔規範.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 4／他人 3 | 保留（無需清洗） |
| `backend/docs/開發規範/多媒體與圖片 URL 規範.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 3／他人 4 | 保留（無需清洗） |
| `backend/docs/開發規範/後端服務責任邊界規範.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 6／他人 14 | 保留（無需清洗） |
| `backend/docs/開發規範/新增微服務指南.md` | 開發規範 | 2 | 0 | 0 | 0 | Will 4／他人 7 | 保留，清洗：品牌 |
| `backend/docs/開發規範/硬上限依賴接入治理規範（配額與容量）.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 8／他人 5 | 保留（無需清洗） |
| `backend/docs/開發規範/設計文件治理規範.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 0／他人 5 | 保留（無需清洗） |
| `backend/docs/開發規範/賣家能力權限層規範.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 0／他人 2 | 保留（無需清洗） |
| `backend/docs/開發規範/開發工作流程.md` | 開發規範 | 0 | 0 | 0 | 0 | Will 3／他人 3 | 保留（無需清洗） |
| `backend/infra/mq/README.md` | README | 0 | 0 | 0 | 0 | Will 24／他人 2 | 保留（無需清洗） |
| `backend/infra/mq/tools/requirements.txt` | 其他文件 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/accountdeletion/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 9／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/accountdeletion/CARRYOVER.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/agent/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/agent/CARRYOVER.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/buyer/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 17／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/buyer/CARRYOVER.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 17／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/content/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 21／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/content/CARRYOVER.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/delegation/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 8／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/delegation/CARRYOVER.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/discovery/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/discovery/CARRYOVER.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 5／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/identity/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 22／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/identity/CARRYOVER.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 6／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/media/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 6／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/reviewqueue/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/reviewqueue/CARRYOVER.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/seller/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 90／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/seller/CARRYOVER.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 81／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/social/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 19／他人 0 | 保留（無需清洗） |
| `backend/internal/modules/social/CARRYOVER.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 17／他人 0 | 保留（無需清洗） |
| `backend/monitoring/README.md` | README | 0 | 0 | 0 | 0 | Will 11／他人 0 | 保留（無需清洗） |
| `backend/pkg/geocode/testdata/NOTES.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/pkg/outbox/query_template.sql.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/scripts/README.md` | README | 0 | 0 | 0 | 0 | Will 10／他人 4 | 保留（無需清洗） |
| `backend/scripts/proto/numeric-allowlist.txt` | 其他文件 | 0 | 0 | 0 | 0 | Will 49／他人 6 | 保留（無需清洗） |
| `backend/services/ai-service/internal/agent/eval_notes.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/services/ai-service/internal/procurement/cropidentity/README.md` | README | 0 | 0 | 0 | 0 | Will 5／他人 0 | 保留（無需清洗） |
| `backend/services/invoice-service/internal/domain/testdata/OFL-1.1-NotoSansTC.txt` | 其他文件 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/services/invoice-service/internal/domain/testdata/README.md` | README | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/services/java/media-service/README.md` | README | 0 | 0 | 0 | 0 | Will 3／他人 3 | 保留（無需清洗） |
| `backend/services/java/order-service/README.md` | README | 0 | 0 | 0 | 0 | Will 2／他人 3 | 保留（無需清洗） |
| `backend/services/java/payment-service/枚舉變更清單.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/services/java/payment-service/測試與部署指南.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 1／他人 2 | 保留（無需清洗） |
| `backend/services/java/product-service/README.md` | README | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/services/java/shared/common-core/README.md` | README | 0 | 0 | 0 | 0 | Will 1／他人 3 | 保留（無需清洗） |
| `backend/services/java/shared/java-proto/README.md` | README | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `backend/services/java/shared/java-proto/src/main/proto/README.md` | README | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/services/java/shared/observability-common/Observability基礎設計書.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/services/java/shared/observability-common/Observability詳細設計書.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/services/java/shared/observability-common/README.md` | README | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `backend/services/java/shared/observability-common/分散式追蹤與監控方案.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `backend/services/pricebook-service/model/README.md` | README | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `backend/services/pricebook-service/reference/MARKET_GEOGRAPHY.md` | 其他文件 | 2 | 0 | 0 | 0 | Will 2／他人 0 | 保留，清洗：品牌 |
| `backend/services/pricebook-service/reference/README.md` | README | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `backend/services/pricebook-service/testdata/parity/README.md` | README | 0 | 0 | 0 | 0 | Will 5／他人 0 | 保留（無需清洗） |
| `backend/services/pricebook-service/testdata/parity/spatial/README.md` | README | 0 | 0 | 0 | 0 | Will 4／他人 0 | 保留（無需清洗） |
| `backend/services/seller-service/internal/usecase/testdata/payuni_error_codes.txt` | 其他文件 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `backend/tests/contract/golden/README.md` | README | 0 | 0 | 0 | 0 | Will 0／他人 2 | 保留（無需清洗） |
| `backend/workers/image-gateway/README.md` | README | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `brand/hawkr/README.md` | README | 0 | 0 | 0 | 0 | Will 5／他人 0 | 保留（無需清洗） |
| `buyer-app/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 7／他人 0 | 保留（無需清洗） |
| `buyer-app/README.md` | README | 1 | 0 | 0 | 0 | Will 8／他人 6 | 保留，清洗：品牌 |
| `buyer-app/docs/architecture.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 1／他人 2 | 保留（無需清洗） |
| `buyer-app/docs/component-library.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 6／他人 9 | 保留（無需清洗） |
| `buyer-app/docs/testing_strategy.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 3／他人 1 | 保留（無需清洗） |
| `buyer-app/ios/Runner/Assets.xcassets/LaunchImage.imageset/README.md` | README | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `packages/hawkr-api-ts/README.md` | README | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `packages/hawkr-icons-ts/README.md` | README | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `packages/hawkr-twaddr-ts/README.md` | README | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `packages/hawkr_auth/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `patches/UPSTREAM-WATCH.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `seller-app/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 7／他人 0 | 保留（無需清洗） |
| `seller-app/ARCHITECTURE.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 5／他人 1 | 保留（無需清洗） |
| `seller-app/README.md` | README | 0 | 0 | 0 | 0 | Will 13／他人 7 | 保留（無需清洗） |
| `seller-app/design-system/hawkr-seller/MASTER.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `seller-app/docs/architecture/presentation-boundary.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `seller-app/docs/component-library.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 4／他人 8 | 保留（無需清洗） |
| `seller-app/docs/design/SELLER_APP_PRODUCT_UX_MASTER_SPEC.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 9／他人 1 | 保留（無需清洗） |
| `seller-app/docs/testing_strategy.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 3／他人 1 | 保留（無需清洗） |
| `seller-app/ios/Runner/Assets.xcassets/LaunchImage.imageset/README.md` | README | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `seller-app/ios/Runner/Assets.xcassets/devLaunchImage.imageset/README.md` | README | 0 | 0 | 0 | 0 | Will 0／他人 2 | 保留（無需清洗） |
| `website/.impeccable/critique/2026-08-13T23-48-20Z__atures-messages-components-conversationsidebar-tsx.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `website/.impeccable/surfaces/account-messages-assistant-conversationid-page-tsx.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 3／他人 2 | 保留（無需清洗） |
| `website/.impeccable/surfaces/app-<舊名-子品牌>-rfq-preview-screen-page-tsx.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 4／他人 1 | 保留，清洗：品牌 |
| `website/.impeccable/surfaces/home-grainient-background.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `website/.impeccable/surfaces/seller-pricing-page-tsx.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 3／他人 7 | 保留（無需清洗） |
| `website/AGENTS.md` | 指令檔 | 0 | 0 | 0 | 0 | Will 12／他人 2 | 保留（無需清洗） |
| `website/DESIGN.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 8／他人 4 | 保留（無需清洗） |
| `website/PRODUCT.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 7／他人 5 | 保留（無需清洗） |
| `website/README.md` | README | 0 | 0 | 0 | 0 | Will 17／他人 12 | 保留（無需清洗） |
| `website/app/README.md` | README | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `website/assets/README.md` | README | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `website/assets/fonts/hawkr-rounded-v2/OFL-1.1.txt` | 其他文件 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `website/components/README.md` | README | 0 | 0 | 0 | 0 | Will 2／他人 1 | 保留（無需清洗） |
| `website/components/shared/README.md` | README | 0 | 0 | 0 | 0 | Will 2／他人 2 | 保留（無需清洗） |
| `website/design-system/README.md` | README | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `website/design-system/cart-typography.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 3／他人 2 | 保留（無需清洗） |
| `website/design-system/cross-platform.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 4／他人 6 | 保留（無需清洗） |
| `website/design-system/foundations/README.md` | README | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `website/design-system/migration/component-inventory.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 0／他人 2 | 保留（無需清洗） |
| `website/design-system/patterns/README.md` | README | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `website/design-system/tokens/README.md` | README | 0 | 0 | 0 | 0 | Will 2／他人 3 | 保留（無需清洗） |
| `website/docs/ai-style-thumbnails.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `website/docs/analytics-events.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 6／他人 1 | 保留（無需清洗） |
| `website/docs/backend-issues/B-247-empty-agent-conversations.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `website/docs/component-library.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 4／他人 5 | 保留（無需清洗） |
| `website/docs/content-copy-audit.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `website/docs/description-editor-validation.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `website/docs/design-system-guidelines.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 7／他人 4 | 保留（無需清洗） |
| `website/docs/directory-architecture.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 3／他人 2 | 保留（無需清洗） |
| `website/docs/frontend-foundation.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `website/docs/legal/self-issue-attestation-terms.md` | 法律頁 | 0 | 0 | 0 | 0 | Will 3／他人 1 | 保留（無需清洗） |
| `website/docs/legal/seller-agreements.json` | 法律頁 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `website/docs/legal/seller-food-registration-notice.md` | 法律頁 | 0 | 0 | 0 | 0 | Will 3／他人 0 | 保留（無需清洗） |
| `website/docs/legal/seller-kyc-notice.md` | 法律頁 | 0 | 0 | 0 | 0 | Will 5／他人 0 | 保留（無需清洗） |
| `website/docs/legal/seller-terms.md` | 法律頁 | 0 | 0 | 0 | 0 | Will 5／他人 0 | 保留（無需清洗） |
| `website/docs/navigation-recovery.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `website/docs/progressive-loading-bkm.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `website/docs/seller-payment-logistics-flow.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 2／他人 3 | 保留（無需清洗） |
| `website/docs/skeleton-alignment.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `website/docs/three-page-split-plan.md` | 子專案文件 | 0 | 0 | 0 | 0 | Will 5／他人 3 | 保留（無需清洗） |
| `website/docs/transaction-policy.md` | 子專案文件 | 1 | 0 | 0 | 0 | Will 2／他人 3 | 保留，清洗：品牌 |
| `website/features/README.md` | README | 0 | 0 | 0 | 0 | Will 1／他人 4 | 保留（無需清洗） |
| `website/lib/README.md` | README | 0 | 0 | 0 | 0 | Will 0／他人 3 | 保留（無需清洗） |
| `website/lib/ai-chat/__tests__/agui-frames/replay-candidate-search-rail.sse.txt` | 其他文件 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `website/lib/fonts/README.md` | README | 0 | 0 | 0 | 0 | Will 4／他人 4 | 保留（無需清洗） |
| `website/lib/fonts/files/OFL-1.1-Roboto.txt` | 其他文件 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `website/lib/fonts/files/OFL-1.1-ZenMaruGothic.txt` | 其他文件 | 0 | 0 | 0 | 0 | Will 1／他人 0 | 保留（無需清洗） |
| `website/lib/services/README.md` | README | 0 | 0 | 0 | 0 | Will 0／他人 2 | 保留（無需清洗） |
| `website/public/data/letswrite-taiwan-map-basic-LICENSE.txt` | 其他文件 | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `website/scripts/fonts/hawkr-rounded-admin-subset.txt` | 其他文件 | 0 | 0 | 0 | 0 | Will 2／他人 0 | 保留（無需清洗） |
| `website/scripts/fonts/openhuninn-v2-core.txt` | 其他文件 | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `website/scripts/fonts/requirements.txt` | 其他文件 | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `website/scripts/performance-baseline.README.md` | 其他文件 | 0 | 0 | 0 | 0 | Will 1／他人 1 | 保留（無需清洗） |
| `website/stories/Introduction.mdx` | 其他文件 | 0 | 0 | 0 | 0 | Will 3／他人 1 | 保留（無需清洗） |
| `website/stories/apps/BuyerApi.mdx` | 其他文件 | 0 | 0 | 0 | 0 | Will 6／他人 8 | 保留（無需清洗） |
| `website/stories/apps/SellerApi.mdx` | 其他文件 | 0 | 0 | 0 | 0 | Will 4／他人 5 | 保留（無需清洗） |
| `website/stories/documentation/Maintenance.mdx` | 其他文件 | 0 | 0 | 0 | 0 | Will 2／他人 2 | 保留（無需清洗） |
| `website/styles/README.md` | README | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留（無需清洗） |
| `website/third_party/react-bits/LICENSE.md` | 第三方授權文件 | 0 | 0 | 0 | 0 | Will 0／他人 1 | 保留，原樣（第三方授權文件，不清洗） |
| `website/third_party/react-bits/NOTICE.md` | 第三方授權文件 | 0 | 0 | 0 | 0 | Will 1／他人 4 | 保留，原樣（第三方授權文件，不清洗） |

## 3. 個資逐處

- 數量：疑似真實 125 組（路徑 x 種類）、假資料與服務位址 439 組。
- 指令：`python3 backend/docs/plans/2026-10-24-portfolio-public/scan.py --category pii --report`（只算疑似真實；假資料不計入）。判定規則在 `scan.py` 的 `classify_*`：
  - email：保留網域（example.*、*.test、*.invalid、*.localhost）與單兩字母佔位網域是假資料；gmail 等公用信箱的本機部分是 test、user、admin 等是假資料；其餘一律疑似真實。
  - 手機：同一個數字重複、連號、後五碼以上為 0 是假資料；其餘疑似真實。
  - 身分證字號：檢查碼通過才算；`x123456789` 類的連號與公開範例是假資料。
  - 統一編號：同一行有「統一編號、統編、tax id」等詞，8 碼通過檢查碼才算（通過檢查碼的測試號碼無法判定真假，見 Q-P1）。
  - 地址：縣市＋區＋路＋號；含「測試、某、火星」或門牌 1、100、999 是假資料。
- 快照：e5d5a8d8e（程式碼與 332da5e51 相同；之後只新增了計畫與 ADR）。判定欄的理由只寫規則，不寫命中的值。

| 個資位置 | 種類 | 判定 | 理由 | 去向 |
|---|---|---|---|---|
| `admin/tests/admin-sidebar-profile.test.ts:L85,L92` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `admin/tests/delegated-operation-governance.test.ts:L44` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `admin/tests/delegated-operation-handoff.test.ts:L37` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `admin/tests/delegated-session-revocation-summary.test.ts:L61,L74,L142,L159` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `admin/tests/fda-export.test.ts:L146,L163` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `admin/tests/fda-export.test.ts:L145,L162` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `admin/tests/food-registration.test.ts:L87,L92` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `admin/tests/merchant-kyc-application.test.ts:L161` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `admin/tests/merchant-kyc-application.test.ts:L762,L1349,L1359` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `admin/tests/merchant-kyc-application.test.ts:L179,L576,L599,L638,L663,L665,L708,L709,L736` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `admin/tests/merchant-kyc-application.test.ts:L1347,L1357` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `admin/tests/merchant-reveal-state.test.ts:L33` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `admin/tests/merchant-store-detail.test.ts:L212,L214` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `admin/tests/merchant-store-list.test.ts:L111,L113,L118,L119,L123,L135,L154` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `admin/tests/merchant-store-list.test.ts:L107,L109` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `admin/tests/report-detail-page.test.ts:L234,L261` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `admin/tests/report-view.test.ts:L59,L68` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `admin/tests/seller-prospects-lib.test.ts:L72,L73,L83,L84,L95,L96,L97,L108,L109,L119…共19處` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `admin/tests/seller-prospects-merge-pages.test.ts:L75,L875,L902,L939,L975,L1006` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `admin/tests/seller-prospects-pages.test.ts:L77,L127` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/.env.template:L100` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/.env.template:L109` | email | 服務位址 | 服務或平台的收件位址（resend、github、google 等） | 保留 |
| `backend/AGENTS.md:L52` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/api/openapi/openapi.yaml:L9071` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/cmd/hawkr/admin_role_integration_test.go:L31,L36,L39,L56` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/cmd/hawkr/admin_role_test.go:L24` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/cmd/hawkr/agent_events_integration_test.go:L91,L92,L93,L140` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/cmd/hawkr/dev_seed.go:L26,L42` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/cmd/hawkr/dev_seed.go:L31` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/cmd/hawkr/dev_seed.go:L47` | national-id | 疑似真實 | 檢查碼通過且不是範例號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/cmd/hawkr/dev_seed.go:L25,L26,L41,L42` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/cmd/hawkr/dev_seed_integration_test.go:L142` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/cmd/hawkr/dev_seed_integration_test.go:L225` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/cmd/hawkr/seller_relay_integration_test.go:L56` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/cmd/hawkr/seller_views_integration_test.go:L106,L117` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/docker-compose-local.yml:L1594` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/docs/plans/2026-10-18-a7-social/step-1a-perf/seed.sql:L24` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/docs/runbooks/food-safety-incident-runbook.md:L249,L271` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/docs/runbooks/seller-governance-shared.md:L108` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/docs/第三方整合/AMEGO 電子發票 API 總覽.md:L2216` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/docs/第三方整合/AMEGO 電子發票 API 總覽.md:L128` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/docs/第三方整合/AMEGO 電子發票 API 總覽.md:L2214,L3250` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/docs/第三方整合/AMEGO 電子發票 API 總覽.md:L3255,L3511` | ubn | 假資料 | 重複數字 | 保留（假資料） |
| `backend/docs/第三方整合/AMEGO 電子發票 API 總覽.md:L576,L688,L689,L703,L3069` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/docs/第三方整合/PayUni 物流官方 API 總覽.md:L233` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/docs/第三方整合/PayUni 費用一覽.md:L82` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/docs/第三方整合/PayUni 費用一覽.md:L105,L302` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/docs/補充資料/product_import_e2e_sample.csv:L60` | phone | 疑似真實 | 不是重複或連號的號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/docs/補充資料/product_import_e2e_sample.md:L57` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/docs/詳細設計書/06-物流微服務（Shipping Service）詳細設計書.md:L2496` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/docs/詳細設計書/06-物流微服務（Shipping Service）詳細設計書.md:L719,L736` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/docs/詳細設計書/09-售後微服務（AfterSales Service）詳細設計書.md:L3339` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/docs/詳細設計書/13-通知模板引擎與收件箱（Notification Template）詳細設計書.md:L408,L409,L410` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/docs/詳細設計書/17-登入與帳號建立流程（Login & Account）詳細設計書.md:L112` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/docs/詳細設計書/18-賣家實名認證（Merchant Certification）詳細設計書.md:L805,L929` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/docs/詳細設計書/23-門市管理模組（Store Pickup Management）詳細設計書.md:L249,L266,L309,L354,L382` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/docs/詳細設計書/23-門市管理模組（Store Pickup Management）詳細設計書.md:L251,L268,L311` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/docs/詳細設計書/26-輸入驗證模組（Java InputValidator）詳細設計書.md:L133` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/docs/詳細設計書/26-輸入驗證模組（Java InputValidator）詳細設計書.md:L48,L123` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/docs/詳細設計書/27-地址搜尋與驗證（Address Proxy）詳細設計書.md:L42,L67,L77,L133` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/docs/詳細設計書/32-商家治理與食安通報（Seller Governance & Food Safety）詳細設計書.md:L267` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/docs/詳細設計書/32-商家治理與食安通報（Seller Governance & Food Safety）詳細設計書.md:L266` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/docs/詳細設計書/41-Realtime SSE 推播平台（Realtime SSE Push Platform）詳細設計書.md:L1179` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/docs/詳細設計書/51-簡訊發送與手機 OTP（Mitake SMS & Phone OTP）詳細設計書.md:L153` | email | 服務位址 | 服務或平台的收件位址（resend、github、google 等） | 保留 |
| `backend/docs/詳細設計書/51-簡訊發送與手機 OTP（Mitake SMS & Phone OTP）詳細設計書.md:L205,L226` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/docs/詳細設計書/56-ADM 代行營運（Delegated Operations）詳細設計書.md:L3374` | phone | 疑似真實 | 不是重複或連號的號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/docs/詳細設計書/58-電子發票服務（Invoice Service）詳細設計書.md:L4248,L9827` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/docs/詳細設計書/58-電子發票服務（Invoice Service）詳細設計書.md:L8484` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/docs/詳細設計書/58-電子發票服務（Invoice Service）詳細設計書.md:L8485` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/docs/詳細設計書/58-電子發票服務（Invoice Service）詳細設計書.md:L8471,L10788,L11599` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/gateway/cmd/doc.go:L10` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/gateway/internal/handler/address_geocode.go:L103` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/gateway/internal/handler/address_geocode_test.go:L113` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/gateway/internal/handler/address_geocode_test.go:L88,L166,L218,L226,L248,L267,L284` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/gateway/internal/handler/admin_delegated_governance_list_test.go:L60,L67` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/gateway/internal/handler/admin_delegation_authorization_contract_test.go:L106,L122,L184,L189` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/gateway/internal/handler/checkout_legacy_golden_test.go:L270` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/gateway/internal/handler/checkout_legacy_golden_test.go:L269` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/gateway/internal/handler/checkout_preservedata_e2e_test.go:L144` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/gateway/internal/handler/checkout_test.go:L227,L269,L423,L453` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/gateway/internal/handler/delegation/diff_test.go:L212` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/gateway/internal/handler/delegation/diff_test.go:L199` | phone | 疑似真實 | 不是重複或連號的號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/gateway/internal/handler/seller_crm_golden_test.go:L38` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/gateway/internal/handler/seller_crm_golden_test.go:L39` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/gateway/internal/handler/seller_crm_golden_test.go:L37,L96,L112` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/gateway/internal/handler/seller_crm_idor_test.go:L26,L85,L141` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/gateway/internal/handler/seller_public_pii_test.go:L71` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/gateway/internal/handler/seller_public_pii_test.go:L69,L70` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/gateway/internal/handler/shipping_buyer_test.go:L59` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/gateway/internal/handler/testdata/golden/seller_customer_detail_success.json:L1` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/gateway/internal/handler/testdata/golden/seller_customer_detail_success.json:L1` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/gateway/internal/handler/testdata/golden/seller_customer_detail_success.json:L1` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/gateway/internal/handler/testdata/golden/seller_customer_list_success.json:L1` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/gateway/internal/handler/vendor_portal_test.go:L85,L171` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/gateway/internal/middleware/logger_allowlist_test.go:L17` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/gateway/internal/middleware/logger_allowlist_test.go:L16` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/gateway/internal/middleware/logger_allowlist_test.go:L15,L22,L24,L26,L47` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/gateway/internal/middleware/logger_diagnostic_integration_test.go:L41,L54` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/gateway/internal/middleware/logger_integration_test.go:L35,L56` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/gateway/internal/middleware/logger_test.go:L56,L58,L185,L187` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/gateway/internal/middleware/logger_test.go:L32` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/gateway/internal/middleware/vendor_portal_session_test.go:L60,L144` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/infra/seed-data/01-account.sql:L43,L44,L45,L46,L47,L49` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/infra/seed-data/01-account.sql:L65,L66,L67,L80,L81,L82,L83` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/infra/seed-data/06-shipping.sql:L27,L28,L32,L33,L44,L45` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/httpapi/agent/agent_integration_test.go:L297` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/httpapi/agent/agent_integration_test.go:L76,L78,L80,L151,L432` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/httpapi/agent/public_test.go:L125,L262,L466,L546` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/httpapi/buyer/buyer_integration_test.go:L263,L512,L515,L769` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/httpapi/identity/identity_integration_test.go:L311,L332,L359,L371,L390,L391,L471,L480` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/httpapi/rpc/images_test.go:L17` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/httpapi/rpc/updatemaskfields_test.go:L29` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/internal/httpapi/rpc/web_origin_test.go:L17` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/httpapi/seller/admin_integration_test.go:L219` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/httpapi/seller/admin_integration_test.go:L477,L1011` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/internal/httpapi/seller/admin_integration_test.go:L219` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/httpapi/seller/admin_kyc_integration_test.go:L29,L95,L246` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/internal/httpapi/seller/admin_kyc_integration_test.go:L645` | national-id | 疑似真實 | 檢查碼通過且不是範例號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/httpapi/seller/admin_kyc_integration_test.go:L645,L684` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/httpapi/seller/admin_kyc_test.go:L734,L757,L845` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/internal/httpapi/seller/admin_kyc_test.go:L117` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/internal/httpapi/seller/admin_kyc_test.go:L734,L757,L845` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/httpapi/seller/app_test.go:L189,L204` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/httpapi/seller/governance_cases_test.go:L285` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/httpapi/seller/governance_test.go:L226,L311` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/httpapi/seller/kyc_integration_test.go:L326` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/httpapi/seller/kyc_integration_test.go:L437` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/httpapi/seller/kyc_integration_test.go:L36,L306` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/internal/httpapi/seller/kyc_integration_test.go:L878` | national-id | 疑似真實 | 檢查碼通過且不是範例號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/httpapi/seller/kyc_integration_test.go:L436` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/httpapi/seller/kyc_integration_test.go:L195,L211,L319,L326,L878,L969,L971` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/httpapi/seller/kyc_test.go:L114,L121,L237,L253` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/internal/httpapi/seller/kyc_test.go:L245` | national-id | 疑似真實 | 檢查碼通過且不是範例號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/httpapi/seller/kyc_test.go:L151,L155,L245,L328,L373,L579` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/httpapi/seller/prospect_integration_test.go:L179,L180,L225,L292,L294,L454,L459,L601,L602,L658` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/httpapi/seller/prospect_integration_test.go:L107,L119,L128` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/httpapi/seller/prospect_integration_test.go:L106,L110,L151,L159,L294,L319,L425,L428` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/httpapi/seller/prospect_test.go:L117,L390,L394,L567` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/httpapi/seller/prospect_test.go:L230` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/httpapi/seller/prospect_test.go:L112,L153,L229,L237,L567,L572` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/httpapi/seller/public_test.go:L91` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/httpapi/seller/reports_test.go:L119,L137,L207` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/httpapi/seller/seller_integration_test.go:L279` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/httpapi/seller/seller_integration_test.go:L259,L279,L454` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/httpapi/vendor_portal_test.go:L160` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/legacy/accountservice/server_integration_test.go:L142` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/legacy/buyerserver/server_integration_test.go:L51` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/legacy/buyerserver/server_test.go:L74` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/legacy/delegationservice/server_integration_test.go:L148,L176,L190,L198,L290,L322,L323` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/legacy/delegationservice/server_test.go:L41,L66` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/legacy/gatewayproxy/vendor_portal_test.go:L86` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/legacy/gatewayproxy/vendor_portal_test.go:L96,L146` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/legacy/sellerserver/invites_integration_test.go:L40` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/legacy/sellerserver/invites_test.go:L176,L188` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/legacy/sellerserver/server_integration_test.go:L171,L368,L378,L384` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/legacy/sellerserver/server_integration_test.go:L201,L203` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/internal/legacy/sellerserver/server_integration_test.go:L215,L229,L239,L247` | national-id | 疑似真實 | 檢查碼通過且不是範例號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/legacy/sellerserver/server_integration_test.go:L166,L171,L252,L368` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/agent/agent_integration_test.go:L810,L1148,L1150,L1152` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/agent/agent_integration_test.go:L85,L86,L149,L151,L164,L178,L181,L265,L395,L569…共30處` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/modules/agent/internal/model/model_test.go:L6,L11,L20,L21,L22` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/agent/internal/model/model_test.go:L11` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/buyer/buyer_integration_test.go:L115,L293,L347` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/buyer/internal/address/address_test.go:L17,L25,L46,L72` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/buyer/internal/profile/profile_test.go:L13,L19` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/buyer/invariants_integration_test.go:L27,L43` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/buyer/version_integration_test.go:L42,L83,L193` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/delegation/admin_role_integration_test.go:L25,L50,L51,L82` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/delegation/delegation_integration_scenarios_test.go:L168,L173,L236,L252,L253,L254,L355,L388,L396,L443…共51處` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/delegation/internal/authorization/delegation_test.go:L21,L22,L25` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/delegation/internal/session/delegation_test.go:L18` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/identity/admin_access_migration_integration_test.go:L42,L57` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/identity/admin_roles_integration_test.go:L55,L124,L140,L142,L174,L176,L208,L257,L310` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/identity/admin_roles_integration_test.go:L92,L152,L163,L164,L166,L272,L273` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/modules/identity/identity_integration_test.go:L377,L392,L478,L562,L568,L863,L902,L908,L922,L923…共14處` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/identity/internal/auth/google_test.go:L31,L52` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/modules/identity/internal/clerk/clerk_test.go:L211,L247,L248` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/identity/internal/oidc/oidc_test.go:L81,L86` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/identity/internal/oidc/policy_test.go:L90,L95` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/identity/internal/provider/access_test.go:L61,L82,L100` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/seller/cases_dataset_integration_test.go:L72` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/seller/cases_integration_test.go:L931` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/seller/foodreg_integration_test.go:L538` | national-id | 疑似真實 | 檢查碼通過且不是範例號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/modules/seller/governance_evidence_integration_test.go:L410` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/seller/governance_integration_test.go:L207,L211,L365,L369,L480,L481,L482,L483,L498,L509…共15處` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/seller/governance_integration_test.go:L188,L191,L207,L211,L300,L355,L357,L361,L369,L384…共19處` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/seller/internal/model/kyc_registration_test.go:L149` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/modules/seller/internal/model/kyc_test.go:L74,L77` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/seller/internal/model/kyc_test.go:L10,L49` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/internal/modules/seller/internal/model/kyc_test.go:L82` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/seller/internal/model/kyc_test.go:L92` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/modules/seller/internal/model/payuni_test.go:L322,L323` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/seller/internal/model/payuni_test.go:L322,L323` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/seller/internal/model/prospect_match_test.go:L22,L23` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/modules/seller/internal/model/prospect_test.go:L81,L82,L83,L90,L104,L105,L123,L124,L128` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/seller/internal/payuni/storedata_test.go:L15,L49` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/seller/internal/payuni/storedata_test.go:L15,L48` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/seller/kyc_identity_hashes_integration_test.go:L60,L63` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/internal/modules/seller/kyc_integration_test.go:L1383` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/seller/kyc_integration_test.go:L27,L28` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/internal/modules/seller/kyc_integration_test.go:L29,L30` | national-id | 疑似真實 | 檢查碼通過且不是範例號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/modules/seller/kyc_integration_test.go:L32` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/modules/seller/kyc_issues_integration_test.go:L344` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/modules/seller/kyc_registry_integration_test.go:L766,L767,L772,L773,L774,L775,L790,L806,L807` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/modules/seller/measure_integration_test.go:L2141,L2156` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/seller/migration_integration_test.go:L56,L177,L180` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/seller/migration_integration_test.go:L56,L177,L180` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/seller/payuni_integration_test.go:L309,L372,L1010,L1025,L1032,L1045,L1049,L1067,L1072,L1429…共15處` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/seller/payuni_integration_test.go:L312,L530,L1025,L1067,L1071,L1118,L1148,L1157,L1206,L1218…共14處` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/modules/seller/prospect_integration_test.go:L1595,L1596,L1630` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/seller/prospect_integration_test.go:L217,L224,L228,L304,L307,L324,L327,L341,L344,L349…共79處` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/modules/seller/seller_integration_test.go:L298` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/modules/seller/seller_integration_test.go:L278,L298,L329,L453,L499` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/platform/gcis/gcis_test.go:L73` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/internal/platform/gcis/gcis_test.go:L87` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/internal/platform/payuni/client_test.go:L164,L169` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/internal/platform/phone/normalize_test.go:L9,L34` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/internal/platform/seal/seal_test.go:L50,L54,L89,L97,L101` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/internal/platform/twaddr/twaddr_test.go:L106` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/monitoring/.env.example:L29` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/pkg/crypto/crypto_test.go:L23,L44,L45,L54,L62,L66` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/pkg/database/dsn_test.go:L15` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/pkg/geocode/parse_test.go:L13` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/pkg/geocode/parse_test.go:L14` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/pkg/geocode/testdata/NOTES.md:L5,L6,L7,L22` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/pkg/geocode/testdata/tgos_queryaddr_response.json:L5,L32` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/pkg/geocode/testdata/tgos_queryaddr_response_fuzzy.json:L5` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/pkg/geocode/testdata/tgos_queryaddr_response_fuzzy.json:L32` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/pkg/geocode/testdata/tgos_queryaddr_response_noresult.json:L5` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/pkg/geocode/testdata/tgos_queryaddr_response_validate_nomatch.json:L5` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/pkg/geocode/tgos_client_test.go:L42,L65,L79` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/pkg/logger/redact_test.go:L63,L64,L116,L138` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/pkg/logger/redact_test.go:L26,L27` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/pkg/mail/guard/guard_test.go:L55,L58,L80,L81,L150` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/pkg/mail/guard/guard_test.go:L69,L78,L79,L178,L179` | email | 服務位址 | 服務或平台的收件位址（resend、github、google 等） | 保留 |
| `backend/pkg/mail/guard/guard_test.go:L16,L76,L77,L119,L123,L127,L130,L136,L146,L150…共12處` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/pkg/mail/logmail/logmail_test.go:L17,L18,L42,L75` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/pkg/mail/mail_test.go:L14,L15,L16,L17,L22,L23,L50,L51,L61,L62…共11處` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/pkg/mail/mailfactory/mailfactory_test.go:L40` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/pkg/mail/resend/resend_test.go:L20,L21,L57,L58` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/pkg/payuni/client_test.go:L56,L69` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/pkg/payuni/crypto_test.go:L167` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/pkg/payuni/crypto_test.go:L161,L165,L212,L218` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/pkg/payuni/crypto_test.go:L160,L197` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/pkg/payuni/crypto_test.go:L18,L75,L171,L212,L218` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/pkg/phone/normalize.go:L19,L23` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/pkg/phone/normalize_test.go:L12,L13,L14,L15,L16,L17,L18` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/pkg/urlsafety/validate_test.go:L27` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/scripts/characterize.sh:L149` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/ai-service/internal/grpc/procurement_seller_response_handler_test.go:L51,L96` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/ai-service/internal/procurement/commerce/seller_response_evidence_test.go:L39,L113` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/crm-service/internal/repository/postgres/crm_repository_test.go:L596,L616,L638,L664,L680,L681` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/crm-service/internal/repository/postgres/mapper_test.go:L23,L51` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/crm-service/internal/usecase/crm_record_order_test.go:L85,L96,L116,L117,L139,L169,L170` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/crm-service/internal/usecase/crm_test.go:L63,L98` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/services/crm-service/internal/usecase/crm_test.go:L71,L95,L96,L118,L140,L141,L281,L394,L395,L423…共16處` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/invoice-service/internal/client/amego/client_test.go:L21` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/invoice-service/internal/client/order/client_test.go:L95,L139` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/invoice-service/internal/config/config_test.go:L59` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/invoice-service/internal/domain/receipt_pdf_test.go:L38` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/invoice-service/internal/grpc/address_detail_test.go:L25,L35,L37,L65,L79,L81` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/services/invoice-service/internal/grpc/notification_resend_test.go:L65,L117` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/invoice-service/internal/grpc/query.go:L554` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/services/invoice-service/internal/grpc/query_read_test.go:L418,L576,L696` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/invoice-service/internal/grpc/query_test.go:L263` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/invoice-service/internal/grpc/tax_profile_test.go:L129` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/services/invoice-service/internal/usecase/admin_issuance_test.go:L42,L106` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/invoice-service/internal/usecase/checkout_test.go:L716` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/invoice-service/internal/usecase/correction_carrier_test.go:L122` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/invoice-service/internal/usecase/correction_intake.go:L423` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/invoice-service/internal/usecase/correction_intake_test.go:L22,L29,L31,L33,L37` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/invoice-service/internal/usecase/correction_reissue_fields_test.go:L114` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/invoice-service/internal/usecase/idempotency_test.go:L141` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/invoice-service/internal/usecase/integration_test.go:L217` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/invoice-service/internal/usecase/issuance_test.go:L279` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/invoice-service/internal/usecase/masking.go:L28,L29` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/invoice-service/internal/usecase/masking.go:L53` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/invoice-service/internal/usecase/masking_test.go:L16` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/invoice-service/internal/usecase/masking_test.go:L11,L12,L13` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/invoice-service/internal/usecase/masking_test.go:L33` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/invoice-service/internal/usecase/preference_test.go:L403,L404,L405,L406` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/invoice-service/internal/usecase/query_integration_test.go:L37` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/invoice-service/internal/usecase/query_read.go:L348` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/invoice-service/internal/usecase/receipt_issue_test.go:L23,L132` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/invoice-service/internal/usecase/receipt_wording_test.go:L72,L79` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/invoice-service/internal/usecase/tax_profile_test.go:L454` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/services/invoice-service/internal/worker/integration_test.go:L101` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/invoice-service/internal/worker/peer_e2e_integration_test.go:L147` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/service/AfterSaleResolutionServiceTest.java:L89` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/coupon-service/src/test/java/io/github/will413028/hawkr/coupon/grpc/CouponGrpcServiceTargetAccountLookupTest.java:L129,L151,L167,L191,L248,L256` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/media-service/README.md:L253` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/media-service/src/test/java/io/github/will413028/hawkr/media/service/ServerUploadSecureEncryptionTest.java:L51` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/checkout/CheckoutConfirmInvoiceIntakeTest.java:L222,L230` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/checkout/CheckoutConfirmInvoiceIntakeTest.java:L157` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/checkout/CheckoutServiceInvoiceInputSpecTest.java:L41,L46,L70` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/checkout/CheckoutUberQuoteWiringTest.java:L95,L164` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/checkout/InvoiceIntakeResolverTest.java:L212,L213,L229,L244,L245` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/checkout/OrderPolicySnapshotChainContractTest.java:L318` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/checkout/OrderPolicySnapshotChainContractTest.java:L312` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/checkout/ShippingUberQuoteAssessorTest.java:L46,L88,L164,L199,L219` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/checkout/UberCheckoutProbeWiringTest.java:L215` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/checkout/UberCheckoutProbeWiringTest.java:L148,L189,L214` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/checkout/UberCheckoutWiringTest.java:L221` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/checkout/UberOrderEligibilityGuardTest.java:L61` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/grpc/OrderGrpcServiceDelegatedReadTest.java:L88,L155,L175` | phone | 疑似真實 | 不是重複或連號的號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/grpc/OrderInvoiceSnapshotInternalTest.java:L105,L217` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/grpc/OrderInvoiceSnapshotReviseTest.java:L62` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/repository/OrderInvoiceSnapshotConstraintIT.java:L162,L179,L188` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/service/OrderCreationTxHandlerInvoiceSnapshotTest.java:L119,L137` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/service/OrderCreationTxHandlerInvoiceSnapshotTest.java:L91` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/service/OrderPolicyPersistenceOnCreationPathsTest.java:L81` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/service/UberOrderCreationGuardWiringTest.java:L158,L178,L195` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/service/UberOrderCreationGuardWiringTest.java:L158,L178,L195` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/util/MaskUtilsTest.java:L15` | phone | 疑似真實 | 不是重複或連號的號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/adapter/grpc/MembershipBillingGrpcServiceTest.java:L74,L88` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/application/service/ApplePayMerchantValidatorTest.java:L121` | email | 服務位址 | 服務或平台的收件位址（resend、github、google 等） | 保留 |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/application/service/MembershipBillingApplicationServiceTest.java:L141,L150,L344,L354,L372,L468` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/application/service/MembershipBillingEventFactoryTest.java:L72,L80` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/application/service/MembershipChargeResultApplicationServiceTest.java:L54,L90,L109,L137,L145,L154` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/services/java/product-service/src/test/java/io/github/will413028/hawkr/product/converter/ProductProtoConverterCertificationClearTest.java:L65` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/product-service/src/test/java/io/github/will413028/hawkr/product/service/GovCertificationServiceDetailedTest.java:L141,L160` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/java/product-service/src/test/java/io/github/will413028/hawkr/product/service/GovCertificationServiceDetailedTest.java:L115` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/validation/InputValidator.java:L153` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/shared/payuni-java-client/src/main/java/io/github/will413028/hawkr/payuni/shipping/ShippingClient.java:L350` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/shared/payuni-java-client/src/test/java/io/github/will413028/hawkr/payuni/payment/PaymentClientRequiredFieldsTest.java:L118,L127,L137,L151,L164,L166,L168,L174,L184,L197…共12處` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/CvsSettingTest.java:L54` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/CvsSettingTest.java:L43` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/CvsSettingTest.java:L42` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/CvsShipmentWebhookTest.java:L57,L59` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/CvsTradeTest.java:L46` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/HomeDeliverySettingTest.java:L39` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/HomeDeliverySettingTest.java:L38` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/provider/FulfillmentCapabilityMigrationTest.java:L336,L534,L554,L579,L615,L656` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/provider/uber/UberAddressStructureTest.java:L87,L88` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/provider/uber/UberAmountBoundaryContractTest.java:L170,L171` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/provider/uber/UberCapabilityBehaviourTest.java:L130` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/provider/uber/UberCreateFailureMappingTest.java:L361,L365` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/provider/uber/UberCreateFailureMappingTest.java:L363,L367` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/provider/uber/UberQuotePayloadHashTest.java:L28,L63,L112` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/service/BuyerStorePickupServiceTest.java:L35,L63` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/util/AddressNormalizerTest.java:L47,L48,L138,L139` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/util/AddressNormalizerTest.java:L16,L30,L111,L119,L120,L126,L127,L132,L133` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/notification-service/internal/config/config_test.go:L123,L161,L223,L246,L267,L286,L292,L308` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/notification-service/internal/worker/testdata/vendor_invite_message.json:L3,L7` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/notification-service/internal/worker/vendor_invite_golden_test.go:L23,L24,L67,L68` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/notification-service/internal/worker/vendor_invite_test.go:L37,L39,L50,L51,L69,L86,L136,L137` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/pricebook-service/internal/domain/geography/boundary_test.go:L80,L84` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/pricebook-service/reference/market_registry.v1.csv:L7,L17` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/services/pricebook-service/reference/market_registry.v1.csv:L3,L4,L5,L6,L8,L9,L10,L11,L12,L13…共22處` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/seller-service/internal/client/gcis/client_test.go:L171,L192` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/seller-service/internal/domain/deliverable_email_test.go:L6` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/domain/deliverable_email_test.go:L13` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/domain/food_safety_report.go:L49,L50,L51,L52,L53` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/domain/payuni_merchant_profile_outbox_test.go:L11,L28` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/seller-service/internal/domain/prospect_match_test.go:L20,L21` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/seller-service/internal/domain/seller_governance_test.go:L100,L101,L102,L103,L104,L105,L170,L190` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/domain/seller_test.go:L80` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/seller-service/internal/domain/seller_test.go:L38,L52,L53` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/grpc/handler_additional_test.go:L127,L131` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/grpc/handler_additional_test.go:L132,L198` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/grpc/mapper_test.go:L276,L291,L292,L442,L476` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/grpc/mapper_test.go:L427,L462` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/services/seller-service/internal/grpc/mapper_test.go:L27,L274,L288,L289` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/grpc/seller_prospect_handler_test.go:L216` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/grpc/seller_prospect_handler_test.go:L216` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/grpc/seller_prospect_migration_dryrun_integration_test.go:L80,L81` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/grpc/seller_prospect_migration_dryrun_integration_test.go:L50,L76,L77` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/seller-service/internal/grpc/vendor_portal_handler_test.go:L43,L53` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/repository/postgres/bank_account_crypto_test.go:L166,L220` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/services/seller-service/internal/repository/postgres/nullable_test.go:L13` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/repository/postgres/payuni_merchant_profile_outbox_integration_test.go:L31,L42` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/seller-service/internal/repository/postgres/seller_prospect_admin_integration_test.go:L134,L138,L140,L144` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/repository/postgres/seller_prospect_duplicates_integration_test.go:L49,L71` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/repository/postgres/seller_prospect_tax_id_index_integration_test.go:L37,L99,L167` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/seller-service/internal/repository/postgres/step_data_crypto_test.go:L16` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/services/seller-service/internal/repository/postgres/vendor_portal_assurance_integration_test.go:L58` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/repository/postgres/vendor_portal_integration_test.go:L32,L45,L65,L66,L118,L122,L130,L136,L156,L160…共15處` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/repository/postgres/vendor_portal_testenv_test.go:L42` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/address_geocode_provisioning_test.go:L26,L35` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/seller-service/internal/usecase/address_geocode_test.go:L57` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/address_geocode_test.go:L92,L98,L121,L128,L131,L146,L161` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/seller-service/internal/usecase/certification_approve_payuni_test.go:L223,L549` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/certification_kyc_reference_lifecycle_test.go:L162` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/certification_mask_test.go:L10` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/certification_submit_new_test.go:L84` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/certification_submit_new_test.go:L71,L109,L244,L269,L300,L331,L359,L383` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/certification_test.go:L1042` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/certification_test.go:L514,L564,L565,L742,L782,L822,L862,L1102,L1233,L1269` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/certification_test.go:L877,L878,L879,L882` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/certification_test.go:L928` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/seller-service/internal/usecase/certification_validator_test.go:L20,L21,L22,L194,L205,L277,L304,L323` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/certification_validator_test.go:L129,L131,L174,L289,L316` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/certification_validator_test.go:L130,L146,L147,L148` | national-id | 疑似真實 | 檢查碼通過且不是範例號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/seller-service/internal/usecase/certification_validator_test.go:L47,L48,L49,L53,L78,L184,L194,L205,L226,L276…共12處` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/certification_validator_test.go:L288` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `backend/services/seller-service/internal/usecase/governance_test.go:L197,L209,L220,L246,L260,L261,L262,L309,L462,L485` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/mock_test.go:L29,L32` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/mock_test.go:L32` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/payuni_merchant_profile_test.go:L311` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/seller_cache_bust_test.go:L451` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/seller_delegation_cas_test.go:L151,L222` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/seller_payuni_locked_field_test.go:L43` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/seller_payuni_profile_outbox_test.go:L145` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/seller-service/internal/usecase/seller_payuni_profile_outbox_test.go:L43,L99` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/seller_prospect_media_test.go:L159` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/seller-service/internal/usecase/seller_prospect_test.go:L256,L262,L393,L394,L403,L404,L410,L498,L499` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/seller_prospect_test.go:L159,L180,L185,L283,L301` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/seller-service/internal/usecase/seller_prospect_test.go:L493,L494` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/seller_revive_tx_integration_test.go:L41` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/seller_test.go:L20,L23,L674,L722,L723,L816,L839,L840,L906,L931…共12處` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/seller_test.go:L23,L53,L89,L152,L153,L155,L156,L216,L264,L290…共27處` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `backend/services/seller-service/internal/usecase/seller_test.go:L445,L449,L450,L470,L476,L477` | phone | 疑似真實 | 不是重複或連號的號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `backend/services/seller-service/internal/worker/payuni_merchant_profile_outbox_worker_test.go:L108,L128,L145,L166,L186,L208` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `buyer-app/lib/catalog/component_catalog.dart:L820,L977` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/lib/features/order/data/after_sale_status_notifier.dart:L51` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `buyer-app/lib/features/settings/presentation/profile_edit_screen.dart:L491` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/lib/features/settings/presentation/settings_screen.dart:L606` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `buyer-app/test/core/observability/sentry_scrub_test.dart:L20,L57` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `buyer-app/test/features/address/address_etag_contract_test.dart:L248` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/address/address_management_sheet_test.dart:L138,L171,L202,L215` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/address/address_selection_widgets_test.dart:L83,L86,L240,L265,L282` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `buyer-app/test/features/address/address_selection_widgets_test.dart:L55,L82,L85,L133,L237,L263,L279` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/address/store_pickup_form_sheet_test.dart:L63,L70` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `buyer-app/test/features/address/store_pickup_form_sheet_test.dart:L60` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/address/store_pickup_model_test.dart:L9,L28` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/auth/buyer_profile_avatar_contract_test.dart:L20` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/auth/buyer_profile_gender_contract_test.dart:L16` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/cart/cart_control_consistency_test.dart:L279,L286,L295` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/cart/cart_store_selection_test.dart:L331` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/cart/cart_surface_rwd_test.dart:L367,L374` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/cart/checkout_payment_sheet_layout_test.dart:L96` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/cart/checkout_uber_contract_test.dart:L191` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `buyer-app/test/features/cart/checkout_uber_contract_test.dart:L37,L190,L311` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/cart/shipping_single_selection_test.dart:L112,L120` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/cart/shipping_unsupported_filter_test.dart:L178,L186` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/order/order_card_redesign_test.dart:L79,L270` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `buyer-app/test/features/order/order_card_redesign_test.dart:L269` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/order/order_detail_redesign_test.dart:L208` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `buyer-app/test/features/order/order_detail_redesign_test.dart:L207` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/settings/profile_edit_screen_test.dart:L150,L158` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `buyer-app/test/features/settings/profile_edit_screen_test.dart:L84` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `buyer-app/test/features/settings/profile_edit_widgets_test.dart:L15` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `buyer-app/test/shared/utils/avatar_wire_test.dart:L33` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `buyer-app/test/shared/utils/avatar_wire_test.dart:L33` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `packages/hawkr-api-ts/src/gen/google/rpc/error_details_pb.ts:L545` | email | 服務位址 | 服務或平台的收件位址（resend、github、google 等） | 保留 |
| `packages/hawkr_api/lib/gen/google/rpc/error_details.pb.dart:L1053` | email | 服務位址 | 服務或平台的收件位址（resend、github、google 等） | 保留 |
| `packages/hawkr_auth/test/clerk_test.dart:L44,L421,L424` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `seller-app/lib/features/seller/data/kyc_form.dart:L379` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `seller-app/lib/features/seller/presentation/kyc_wizard_steps.dart:L392` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `seller-app/lib/features/settings/data/tgos_address_service.dart:L126` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `seller-app/lib/features/settings/presentation/help_center_screen.dart:L16` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `seller-app/lib/features/settings/presentation/shipping_settings/shipping_settings_screen.dart:L697` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `seller-app/test/core/observability/sentry_scrub_test.dart:L20,L48` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `seller-app/test/core/session/my_store_test.dart:L283` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `seller-app/test/features/customer/customer_detail_more_menu_test.dart:L31,L46,L105,L158` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `seller-app/test/features/customer/customer_repository_camel_parse_test.dart:L118,L129` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `seller-app/test/features/customer/customer_repository_camel_parse_test.dart:L74,L91,L116` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `seller-app/test/features/media/buyer_avatar_key_contract_test.dart:L116` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `seller-app/test/features/media/buyer_avatar_key_contract_test.dart:L77` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `seller-app/test/features/media/buyer_avatar_key_contract_test.dart:L78` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `seller-app/test/features/order/order_card_compact_layout_test.dart:L265` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `seller-app/test/features/order/order_card_redesign_test.dart:L185` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `seller-app/test/features/order/order_detail_redesign_test.dart:L161` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `seller-app/test/features/order/order_detail_redesign_test.dart:L162` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `seller-app/test/features/order/order_field_contract_test.dart:L37,L81,L336,L337` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `seller-app/test/features/order/order_field_contract_test.dart:L80,L335,L351` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `seller-app/test/features/order/order_item_image_key_contract_test.dart:L45` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `seller-app/test/features/order/order_list_layout_test.dart:L162` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `seller-app/test/features/procurement/procurement_error_test.dart:L241,L246` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `seller-app/test/features/procurement/seller_quote_repository_test.dart:L456,L463` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `seller-app/test/features/seller/data/kyc_form_test.dart:L89,L95,L103,L122,L210,L219,L288,L294,L512` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `seller-app/test/features/seller/data/kyc_form_test.dart:L104,L130` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `seller-app/test/features/seller/data/kyc_ocr_test.dart:L10,L27` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `seller-app/test/features/seller/data/kyc_store_contact_test.dart:L12,L32,L39,L69,L80,L86` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `seller-app/test/features/seller/data/kyc_store_contact_test.dart:L11,L31,L38` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `seller-app/test/features/seller/data/kyc_wizard_test.dart:L939` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `seller-app/test/features/seller/data/kyc_wizard_test.dart:L482,L485,L488,L495` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `seller-app/test/features/seller/data/kyc_wizard_test.dart:L938` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `seller-app/test/features/seller/data/kyc_wizard_test.dart:L266,L846,L858,L860,L874,L883,L894,L904` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `seller-app/test/features/seller/data/store_editor_test.dart:L265` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `seller-app/test/features/seller/data/store_editor_test.dart:L263,L276,L277` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `seller-app/test/features/seller/presentation/profile_screen_test.dart:L385,L389` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `seller-app/test/features/seller/presentation/seller_certification_upload_screen_test.dart:L509,L526,L542` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `seller-app/test/features/settings/data/cvs_shipping_mutation_test.dart:L194` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `seller-app/test/features/settings/shipping_sender_test.dart:L63,L72,L125,L132` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `seller-app/test/features/settings/shipping_settings_prefill_test.dart:L21,L183,L185,L194,L203,L212,L213,L226,L227,L238…共22處` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/app/api/public/seller-leads/__tests__/route.unit.test.ts:L12` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/app/api/public/seller-leads/__tests__/route.unit.test.ts:L11` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/components/shared/Footer.tsx:L185,L202` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/components/shared/Link.navigation.unit.test.ts:L357` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/components/shared/SellerFooter.tsx:L56,L59` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/components/shared/__tests__/view-transitions-prefetch.test.ts:L217,L218` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/features/account/screens/help/HelpPage.tsx:L83` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/features/cart/components/StorePickupPickerSheet.tsx:L315` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/features/cart/components/UberShippingOption.unit.test.ts:L42` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `website/features/content/contact/ContactPage.tsx:L21,L22` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/features/content/contact/ContactPage.tsx:L267` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/features/invoices/carrierDisplay.ts:L28` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `website/features/invoices/carrierDisplay.unit.test.ts:L15,L17,L23` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `website/features/procurement-assistant/__tests__/buyer-quote-contract.test.ts:L96,L107` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/features/procurement-assistant/__tests__/buyer-quote-contract.test.ts:L98` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/features/procurement-rfq-prototype/ExternalVendorPortal.tsx:L389,L840` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/features/procurement-rfq-prototype/ExternalVendorPortal.tsx:L836` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/features/procurement-rfq-prototype/RfqPrototype.tsx:L1177` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/features/product/cert-render-surface.unit.test.ts:L85,L91,L96,L100,L112` | phone | 疑似真實 | 不是重複或連號的號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `website/features/seller-center/profile/SellerProfileClient.unit.test.ts:L283,L292` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/features/seller-center/profile/SellerProfileClient.unit.test.ts:L280,L291` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/features/seller-center/profile/certifications/CertificationsClient.unit.test.ts:L1104,L1114,L1146,L1152,L1173,L1192,L1195,L1198,L1203,L1284…共11處` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/features/seller-center/profile/certifications/CertificationsClient.unit.test.ts:L1089,L1094,L1103,L1151,L1177,L1217,L1283,L1293` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/features/seller-center/profile/shipping/ShippingClient.tsx:L87` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/features/seller-center/profile/shipping/ShippingClient.tsx:L429,L701` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/features/seller-center/profile/shipping/ShippingClient.unit.test.ts:L122,L171,L190,L218,L302,L343,L349,L364` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/features/seller-center/profile/shipping/prefill.unit.test.ts:L57,L67,L78,L89,L183,L189,L198,L205,L219` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/features/seller/components/AddressAutocompleteField.tsx:L37` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/lib/addresses/__tests__/schema.test.ts:L8,L21,L37,L53,L69,L101` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/lib/addresses/repo.unit.test.ts:L27,L86` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/lib/buyer/seller-match-chat.unit.test.ts:L83,L356` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/lib/buyer/seller-match-chat.unit.test.ts:L84` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/lib/checkout/draft.unit.test.ts:L47` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `website/lib/checkout/draft.unit.test.ts:L46` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/lib/checkout/repo.unit.test.ts:L89` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `website/lib/checkout/repo.unit.test.ts:L88,L105,L145,L182,L206` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/lib/config/__tests__/media-env.unit.test.ts:L57` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `website/lib/intl/navigation.unit.test.ts:L154` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/lib/seller/kyc.unit.test.ts:L37` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `website/lib/seller/payuni-locked-fields.unit.test.ts:L36,L40` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/lib/seller/shipping-suggestions.unit.test.ts:L55` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `website/lib/seller/shipping-suggestions.unit.test.ts:L62` | address | 疑似真實 | 縣市區路號俱全且不含佔位字樣 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `website/lib/seller/store-contact-saver.unit.test.ts:L39,L46,L49,L51,L52,L69,L76,L84` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/lib/seller/store-contact.unit.test.ts:L96,L97` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/lib/seller/store-form.unit.test.ts:L115,L122,L125` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/lib/seller/store-form.unit.test.ts:L114,L180,L182` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/lib/seller/validators.ts:L53` | national-id | 假資料 | 公開的範例號碼或重複數字 | 保留（假資料） |
| `website/stories/web/Cart.stories.tsx:L228,L237,L274` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/analytics-page-view-privacy.test.ts:L123,L140,L145` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/tests/analytics-page-view-privacy.test.ts:L133,L171` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/checkout-draft.test.ts:L54,L59,L64,L75,L78,L88,L100,L103` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/checkout-repo.test.ts:L83` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/e2e/cart-seller-selection.spec.ts:L177` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/e2e/protected-dynamic-routes.spec.ts:L126,L865` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `website/tests/e2e/protected-dynamic-routes.spec.ts:L864` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/tests/e2e/protected-dynamic-routes.spec.ts:L125,L549,L863` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/e2e/protected-dynamic-routes.spec.ts:L176,L220` | ubn | 疑似真實 | 同行有統編關鍵字且 8 碼通過檢查碼（Q-P1） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱；待 Will 確認 [Q-P1] |
| `website/tests/e2e/seller-dashboard-onboarding-actions.spec.ts:L23` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/e2e/seller-orders-count-vs-list.spec.ts:L69` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `website/tests/e2e/seller-orders-count-vs-list.spec.ts:L68` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/e2e/seller-summary-mobile-layout.spec.ts:L24` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/e2e/seller-termination-notice.spec.ts:L21` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/e2e/seller-unavailability-messages.spec.ts:L24` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/env-origins.test.ts:L73` | email | 疑似真實 | 網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號） | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |
| `website/tests/logout-local-privacy.test.ts:L108,L115,L134,L163` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/media-objectkey-contract.test.ts:L57` | address | 假資料 | 含佔位字樣或門牌 1、100、999 | 保留（假資料） |
| `website/tests/navigation-observability.test.ts:L621` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/tests/orders-repo.test.ts:L146,L155,L167` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/performance-baseline.test.ts:L160` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/tests/seller-coupon-repo.test.ts:L138,L192,L208` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/seller-e2e-fixtures.test.ts:L56` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/seller-ga4-conversions.test.ts:L38` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/tests/seller-ga4-conversions.test.ts:L18,L37,L59,L69,L106` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/seller-onboarding-persistence.test.ts:L18,L68` | phone | 假資料 | 重複、連號或零填充的號碼 | 保留（假資料） |
| `website/tests/seller-pricing-page.spec.ts:L1184,L1201` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/tests/sentry-client-before-send.test.ts:L53` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/tests/sentry-scrub.test.ts:L16,L37` | email | 假資料 | 保留網域或明顯佔位 | 保留（假資料） |
| `website/tests/sentry-scrub.test.ts:L39` | phone | 疑似真實 | 不是重複或連號的號碼 | 換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱 |

## 4. 供應商文件與測試帳號資訊

- `backend/docs/第三方整合/` 共 8 個檔（計畫記 8）；指令：`git ls-files backend/docs/第三方整合 | wc -l`。
- 其餘位置由 `python3 backend/docs/plans/2026-10-24-portfolio-public/scan.py --category merchant --report` 找出：標籤（特店代號、金鑰名稱、測試帳號）後面接著看起來像值的字串，排除佔位字樣。只列位置與種類，不列值。
- 快照：e5d5a8d8e（程式碼與 332da5e51 相同；之後只新增了計畫與 ADR）。

| 位置 | 種類 | 去向 |
|---|---|---|
| `backend/docs/第三方整合/AMEGO 電子發票 API 總覽.md` | 供應商文件（整份） | 保留；清洗特店代號與測試帳號資訊，保留欄位說明與格式（D1、步驟 6） |
| `backend/docs/第三方整合/PayUni 代理商專用 API 總覽.md` | 供應商文件（整份） | 保留；清洗特店代號與測試帳號資訊，保留欄位說明與格式（D1、步驟 6） |
| `backend/docs/第三方整合/PayUni 物流官方 API 總覽.md` | 供應商文件（整份） | 保留；清洗特店代號與測試帳號資訊，保留欄位說明與格式（D1、步驟 6） |
| `backend/docs/第三方整合/PayUni 費用一覽.md` | 供應商文件（整份） | 保留；清洗特店代號與測試帳號資訊，保留欄位說明與格式（D1、步驟 6） |
| `backend/docs/第三方整合/PayUni 逆向金流 API 總覽.md` | 供應商文件（整份） | 保留；清洗特店代號與測試帳號資訊，保留欄位說明與格式（D1、步驟 6） |
| `backend/docs/第三方整合/PayUni 金流官方 API 總覽.md` | 供應商文件（整份） | 保留；清洗特店代號與測試帳號資訊，保留欄位說明與格式（D1、步驟 6） |
| `backend/docs/第三方整合/TGOS API 整合指南.md` | 供應商文件（整份） | 保留；清洗特店代號與測試帳號資訊，保留欄位說明與格式（D1、步驟 6） |
| `backend/docs/第三方整合/Uber Direct API 總覽.md` | 供應商文件（整份） | 保留；清洗特店代號與測試帳號資訊，保留欄位說明與格式（D1、步驟 6） |

文件與程式碼中出現特店代號、金鑰名稱與測試帳號值的位置：

| 位置 | 種類 | 去向 |
|---|---|---|
| `admin/tests/merchant-kyc-application.test.ts:L1104` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-5a-design.md:L102,L128` | 特店代號 | 移除值，保留欄位名稱與格式說明（步驟 6） |
| `backend/docs/第三方整合/AMEGO 電子發票 API 總覽.md:L225,L3286,L3363` | 金鑰名稱 | 移除值，保留欄位名稱與格式說明（步驟 6） |
| `backend/docs/詳細設計書/06-物流微服務（Shipping Service）詳細設計書.md:L4559,L4560` | 特店代號 | 移除值，保留欄位名稱與格式說明（步驟 6） |
| `backend/docs/詳細設計書/10-PayUni Webhook 服務（payuni-webhook-service）詳細設計書.md:L181,L182` | 特店代號 | 移除值，保留欄位名稱與格式說明（步驟 6） |
| `backend/docs/詳細設計書/58-電子發票服務（Invoice Service）詳細設計書.md:L8371` | 金鑰名稱 | 移除值，保留欄位名稱與格式說明（步驟 6） |
| `backend/gateway/internal/handler/invoice_preservedata_e2e_test.go:L217` | 測試帳號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/gateway/internal/handler/payuni_redirect_test.go:L128,L253` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/internal/httpapi/seller/admin_kyc_test.go:L437` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/internal/legacy/sellerconsumer/consumer_test.go:L50` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/internal/modules/seller/payuni_integration_test.go:L826,L883` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/internal/platform/payuni/client_test.go:L102,L107` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/pkg/payuni/client_test.go:L33,L54,L69,L105` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/invoice-service/internal/usecase/checkout_test.go:L563` | 測試帳號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/invoice-service/internal/worker/integration_test.go:L152` | 金鑰名稱 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/domain/repository/PaymentTransactionRepository.java:L75` | 特店代號 | 改由環境變數或設定注入，值不進版控（步驟 2 與步驟 6） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/CryptoTest.java:L84` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/agentfee/AgentFeeSubmitPreCommitTest.java:L36` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/logistics/application/LogisticsCollectionServiceTest.java:L45` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/logistics/application/LogisticsMultiPeriodCashConservationTest.java:L48` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/logistics/application/LogisticsReconciliationServiceTest.java:L44` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/logistics/application/ProviderStatementImporterTest.java:L36` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/logistics/scheduler/LogisticsCollectionSchedulerLeaseTest.java:L36` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/product-service/src/test/java/io/github/will413028/hawkr/product/service/GovCertificationServiceTest.java:L62` | 金鑰名稱 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/shared/payuni-java-client/src/test/java/io/github/will413028/hawkr/payuni/crypto/PayUniCryptoServiceTest.java:L25` | 金鑰名稱 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/CvsPrintLabelTest.java:L24,L25` | 金鑰名稱 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/CvsSettingTest.java:L23,L24` | 金鑰名稱 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/CvsShipMapTest.java:L21,L22` | 金鑰名稱 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/CvsShipmentWebhookTest.java:L29,L30` | 金鑰名稱 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/CvsTradeTest.java:L24,L25` | 金鑰名稱 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/DecryptTest.java:L13,L14` | 金鑰名稱 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/HomeDeliverySettingTest.java:L22,L23` | 金鑰名稱 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/seller-service/internal/mq/golden_contract_test.go:L51` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/seller-service/internal/usecase/certification_test.go:L928` | 測試帳號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |
| `backend/services/seller-service/internal/usecase/payuni_merchant_profile_test.go:L31` | 特店代號 | 已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變） |

## 5. gitleaks

- 數量：70 筆（計畫記 72 筆：27 筆雜湊、17 筆要人看）；本次分類：測試值 38、私鑰字面（非金鑰） 2、雜湊誤報 30。
- 指令：`git archive HEAD | tar -x -C <空目錄>`，再 `gitleaks dir <空目錄> --redact --no-banner -f json -r <輸出> --exit-code 0`；只讀 RuleID、檔案與行號，Secret 與 Match 欄位已遮蔽並且不寫入本檔。
- 快照：e5d5a8d8e（程式碼與 332da5e51 相同；之後只新增了計畫與 ADR）。（本次執行）
- 分類規則：鎖定檔與純十六進位字串是雜湊誤報；測試檔、設計書範例與供應商文件的範例值是測試值；其餘是需要人看。分類是機械規則加上跨檔比對（同一個值出現的檔數，在記憶體內比對，不輸出）。
- 沒有發現「非雜湊、非測試值」的明文金鑰；唯一的非測試檔高熵字串（S1，已決）只記位置與規則，不展開。

| 位置 | 規則 | 分類 | 依據 | 去向 |
|---|---|---|---|---|
| `admin/tests/delegated-operation-upstream.test.ts:L61` | generic-api-key | 測試值 | 值是 UUID 範例（冪等鍵） | 保留（範例值）；加入 .gitleaksignore |
| `api/buf.yaml:L9` | generic-api-key | 雜湊誤報 | BSR 模組參照，不是憑證 | 加入 .gitleaksignore，附理由（模組參照） |
| `backend/cmd/hawkr/payuni_config_test.go:L16` | generic-api-key | 測試值 | 測試檔內的十六進位值（長度 32，變數 key）；形狀像金鑰，已決（S2）視為舊公司的真實憑證 | 已決（S2）：換成明顯的虛構值（步驟 6） |
| `backend/docs/第三方整合/AMEGO 電子發票 API 總覽.md:L225` | generic-api-key | 測試值 | 供應商文件公布的測試值：同一個值也出現在其他檔（2 個檔） | 供應商文件依步驟 6 清洗；測試檔換虛構值 |
| `backend/docs/第三方整合/AMEGO 電子發票 API 總覽.md:L3286` | generic-api-key | 測試值 | 供應商文件公布的測試值：同一個值也出現在其他檔（2 個檔） | 供應商文件依步驟 6 清洗；測試檔換虛構值 |
| `backend/docs/第三方整合/AMEGO 電子發票 API 總覽.md:L3363` | generic-api-key | 測試值 | 供應商文件公布的測試值：同一個值也出現在其他檔（2 個檔） | 供應商文件依步驟 6 清洗；測試檔換虛構值 |
| `backend/docs/第三方整合/AMEGO 電子發票 API 總覽.md:L3400` | generic-api-key | 測試值 | 供應商文件公布的測試值：同一個值也出現在其他檔（2 個檔） | 供應商文件依步驟 6 清洗；測試檔換虛構值 |
| `backend/docs/第三方整合/PayUni 金流官方 API 總覽.md:L779` | generic-api-key | 測試值 | 供應商文件的範例值 | 供應商文件依步驟 6 清洗 |
| `backend/docs/詳細設計書/05-支付服務（Payment Service）詳細設計書.md:L4057` | private-key | 私鑰字面（非金鑰） | 只出現標頭字串（解析或剝除用的程式字面，或設計書的說明文字），沒有金鑰本體 | 保留（誤報）；加入 .gitleaksignore 並附理由 |
| `backend/docs/詳細設計書/08-多媒體微服務（Media Service）詳細設計書.md:L193` | generic-api-key | 測試值 | 設計書或 README 裡的範例值（變數 -，值以 ... 結尾或為說明用） | 改成明顯的佔位文字 |
| `backend/docs/詳細設計書/08-多媒體微服務（Media Service）詳細設計書.md:L206` | generic-api-key | 測試值 | 設計書或 README 裡的範例值（變數 -，值以 ... 結尾或為說明用） | 改成明顯的佔位文字 |
| `backend/docs/詳細設計書/56-ADM 代行營運（Delegated Operations）詳細設計書.md:L2483` | generic-api-key | 測試值 | 值是 UUID 範例（冪等鍵） | 保留（範例值）；加入 .gitleaksignore |
| `backend/docs/詳細設計書/56-ADM 代行營運（Delegated Operations）詳細設計書.md:L3431` | generic-api-key | 測試值 | 值是 UUID 範例（冪等鍵） | 保留（範例值）；加入 .gitleaksignore |
| `backend/gateway/internal/handler/admin_delegation_contract_test.go:L266` | generic-api-key | 測試值 | 值是 UUID 範例（冪等鍵） | 保留（範例值）；加入 .gitleaksignore |
| `backend/gateway/internal/handler/admin_delegation_contract_test.go:L336` | generic-api-key | 測試值 | 值是 UUID 範例（冪等鍵） | 保留（範例值）；加入 .gitleaksignore |
| `backend/gateway/internal/handler/admin_delegation_contract_test.go:L354` | generic-api-key | 測試值 | 值是 UUID 範例（冪等鍵） | 保留（範例值）；加入 .gitleaksignore |
| `backend/gateway/internal/middleware/logger_integration_test.go:L185` | generic-api-key | 測試值 | 測試檔內的值（變數 -） | 換成明顯的虛構值，或以 .gitleaksignore 放行並附理由 |
| `backend/gateway/internal/middleware/ratelimit_subject_test.go:L15` | generic-api-key | 測試值 | 測試檔內的值（變數 testOriginCISecret） | 換成明顯的虛構值，或以 .gitleaksignore 放行並附理由 |
| `backend/internal/modules/delegation/db/migrations/atlas.sum:L4` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/internal/modules/identity/db/migrations/atlas.sum:L8` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/internal/modules/identity/db/migrations/atlas.sum:L14` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/internal/modules/seller/db/migrations/atlas.sum:L16` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/internal/platform/migrate/testdata/atlas-generated/atlas.sum:L14` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/internal/platform/payuni/client_test.go:L19` | generic-api-key | 測試值 | 測試檔內的十六進位值（長度 32，變數 testKey）；形狀像金鑰，已決（S2）視為舊公司的真實憑證 | 已決（S2）：換成明顯的虛構值（步驟 6） |
| `backend/pkg/logger/redact_test.go:L21` | generic-api-key | 測試值 | 測試檔內的值（變數 input） | 換成明顯的虛構值，或以 .gitleaksignore 放行並附理由 |
| `backend/pkg/payuni/client_test.go:L13` | generic-api-key | 測試值 | 測試檔內的十六進位值（長度 32，變數 testHashKey）；形狀像金鑰，已決（S2）視為舊公司的真實憑證 | 已決（S2）：換成明顯的虛構值（步驟 6） |
| `backend/services/invoice-service/db/migrations/atlas.sum:L13` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/invoice-service/db/migrations/atlas.sum:L16` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/invoice-service/internal/client/amego/sign_test.go:L11` | generic-api-key | 測試值 | 測試檔內的值（變數 officialAppKey） | 換成明顯的虛構值，或以 .gitleaksignore 放行並附理由 |
| `backend/services/invoice-service/internal/usecase/admin_manual_unlock_test.go:L525` | generic-api-key | 測試值 | 測試檔內的值（變數 IdempotencyKey） | 換成明顯的虛構值，或以 .gitleaksignore 放行並附理由 |
| `backend/services/invoice-service/internal/usecase/seller_write_test.go:L286` | generic-api-key | 測試值 | 測試檔內的值（變數 ExpectedVersion） | 換成明顯的虛構值，或以 .gitleaksignore 放行並附理由 |
| `backend/services/java/media-service/README.md:L308` | generic-api-key | 測試值 | 設計書或 README 裡的範例值（變數 -，值以 ... 結尾或為說明用） | 改成明顯的佔位文字 |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/application/service/ApplePayMerchantValidator.java:L277` | private-key | 私鑰字面（非金鑰） | 只出現標頭字串（解析或剝除用的程式字面，或設計書的說明文字），沒有金鑰本體 | 保留（誤報）；加入 .gitleaksignore 並附理由 |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/CryptoTest.java:L78` | generic-api-key | 測試值 | 沙盒憑證樣貌：同一個值出現在 10 個測試檔與測試設定（變數 realKey）；視為舊公司的真實憑證 | 已決（S2）：換成虛構值（步驟 6） |
| `backend/services/java/product-service/src/test/java/io/github/will413028/hawkr/product/service/GovCertificationServiceTest.java:L62` | generic-api-key | 測試值 | 測試檔內的值（變數 API_KEY） | 換成明顯的虛構值，或以 .gitleaksignore 放行並附理由 |
| `backend/services/java/shared/payuni-java-client/src/test/java/io/github/will413028/hawkr/payuni/crypto/PayUniCryptoServiceTest.java:L25` | generic-api-key | 測試值 | 沙盒憑證樣貌：同一個值出現在 10 個測試檔與測試設定（變數 AGENT_KEY）；視為舊公司的真實憑證 | 已決（S2）：換成虛構值（步驟 6） |
| `backend/services/java/shared/payuni-java-client/src/test/java/io/github/will413028/hawkr/payuni/webhook/PayUniWebhookParserTest.java:L22` | generic-api-key | 測試值 | 沙盒憑證樣貌：同一個值出現在 10 個測試檔與測試設定（變數 TEST_KEY）；視為舊公司的真實憑證 | 已決（S2）：換成虛構值（步驟 6） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/CvsPrintLabelTest.java:L24` | generic-api-key | 測試值 | 沙盒憑證樣貌：同一個值出現在 10 個測試檔與測試設定（變數 HASH_KEY）；視為舊公司的真實憑證 | 已決（S2）：換成虛構值（步驟 6） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/CvsSettingTest.java:L23` | generic-api-key | 測試值 | 沙盒憑證樣貌：同一個值出現在 10 個測試檔與測試設定（變數 HASH_KEY）；視為舊公司的真實憑證 | 已決（S2）：換成虛構值（步驟 6） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/CvsShipMapTest.java:L21` | generic-api-key | 測試值 | 沙盒憑證樣貌：同一個值出現在 10 個測試檔與測試設定（變數 HASH_KEY）；視為舊公司的真實憑證 | 已決（S2）：換成虛構值（步驟 6） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/CvsShipmentWebhookTest.java:L29` | generic-api-key | 測試值 | 沙盒憑證樣貌：同一個值出現在 10 個測試檔與測試設定（變數 HASH_KEY）；視為舊公司的真實憑證 | 已決（S2）：換成虛構值（步驟 6） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/CvsTradeTest.java:L24` | generic-api-key | 測試值 | 沙盒憑證樣貌：同一個值出現在 10 個測試檔與測試設定（變數 HASH_KEY）；視為舊公司的真實憑證 | 已決（S2）：換成虛構值（步驟 6） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/DecryptTest.java:L13` | generic-api-key | 測試值 | 沙盒憑證樣貌：同一個值出現在 10 個測試檔與測試設定（變數 HASH_KEY）；視為舊公司的真實憑證 | 已決（S2）：換成虛構值（步驟 6） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/HomeDeliverySettingTest.java:L22` | generic-api-key | 測試值 | 沙盒憑證樣貌：同一個值出現在 10 個測試檔與測試設定（變數 HASH_KEY）；視為舊公司的真實憑證 | 已決（S2）：換成虛構值（步驟 6） |
| `backend/services/notification-service/db/migrations/atlas.sum:L2` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/notification-service/db/migrations/atlas.sum:L8` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/notification-service/db/migrations/atlas.sum:L58` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/notification-service/db/migrations/atlas.sum:L59` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/notification-service/db/migrations/atlas.sum:L64` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/notification-service/db/migrations/atlas.sum:L65` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/notification-service/db/migrations/atlas.sum:L66` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/notification-service/db/migrations/atlas.sum:L75` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/notification-service/db/migrations/atlas.sum:L76` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/notification-service/db/migrations/atlas.sum:L83` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/notification-service/db/migrations/atlas.sum:L90` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/notification-service/db/migrations/atlas.sum:L94` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/notification-service/db/migrations/atlas.sum:L96` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/notification-service/db/migrations/atlas.sum:L102` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/review-service/db/migrations/atlas.sum:L6` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/seller-service/db/migrations/atlas.sum:L43` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `backend/services/seller-service/internal/repository/postgres/payuni_merchant_profile_update_integration_test.go:L60` | generic-api-key | 測試值 | 測試檔內的值（變數 ID） | 換成明顯的虛構值，或以 .gitleaksignore 放行並附理由 |
| `backend/services/seller-service/internal/repository/postgres/payuni_merchant_profile_update_integration_test.go:L68` | generic-api-key | 測試值 | 測試檔內的值（變數 ID） | 換成明顯的虛構值，或以 .gitleaksignore 放行並附理由 |
| `backend/services/seller-service/internal/repository/postgres/payuni_merchant_profile_update_integration_test.go:L74` | generic-api-key | 測試值 | 測試檔內的值（變數 StoreID） | 換成明顯的虛構值，或以 .gitleaksignore 放行並附理由 |
| `buyer-app/ios/Podfile.lock:L317` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `buyer-app/ios/Podfile.lock:L339` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `packages/hawkr-api-ts/buf.gen.yaml:L17` | generic-api-key | 雜湊誤報 | BSR 模組參照，不是憑證 | 加入 .gitleaksignore，附理由（模組參照） |
| `packages/hawkr_api/buf.gen.yaml:L20` | generic-api-key | 雜湊誤報 | BSR 模組參照，不是憑證 | 加入 .gitleaksignore，附理由（模組參照） |
| `seller-app/ios/Podfile.lock:L327` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `seller-app/ios/Podfile.lock:L350` | generic-api-key | 雜湊誤報 | 鎖定檔的內容雜湊 | 加入 .gitleaksignore，附理由（雜湊） |
| `website/lib/api/delegated.unit.test.ts:L62` | generic-api-key | 測試值 | 測試檔內的值（變數 idempotencyKey） | 換成明顯的虛構值，或以 .gitleaksignore 放行並附理由 |

## 6. issue、PR 連結與 `git show <sha>` 指引

- issue 或 PR 連結：53 個檔（計畫記 53）；`git show <sha>` 指引：16 個檔（計畫記 16）；僅以 `#編號` 或 `owner/repo#編號` 引用 issue 或 PR：737 個檔。
- 指令：`python3 backend/docs/plans/2026-10-24-portfolio-public/scan.py --category refs --report`；也可用 `git grep -l -I -E 'github\.com/[^/]+/[^/]+/(issues|pull)/[0-9]+'` 與 `git grep -l -I -E 'git show [0-9a-f]{7,40}'` 交叉核對。
- 快照：e5d5a8d8e（程式碼與 332da5e51 相同；之後只新增了計畫與 ADR）。

### 6a. issue 或 PR 連結

| 檔案 | 處數 | 去向 |
|---|---|---|
| `admin/src/lib/codemeta/generated.ts:L310,L425,L448,L900,L1239,L1278` | 6 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/adr/2026-09-18-procurement-llm-extract-retry-and-timeout.md:L8` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/代碼管理（CodeMeta）/業務狀態碼.md:L708,L740,L763,L885,L1081,L1108…共8處` | 8 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/代碼管理（CodeMeta）/業務類型碼.md:L956,L1016` | 2 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/代碼管理（CodeMeta）/錯誤碼.md:L616` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/09-售後微服務（AfterSales Service）詳細設計書.md:L666,L2482,L4082,L4736,L5457,L5518` | 6 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/15-AI 微服務（AI Service）詳細設計書.md:L780,L784,L828,L851,L853,L855…共12處` | 12 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/20-結帳模組（Checkout Module）詳細設計書.md:L1036` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/22-商品配送驗證（Product Shipping Validation）詳細設計書.md:L147` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/31-內容檢舉與封鎖（Content Reporting & Block）詳細設計書.md:L381,L382,L383,L384,L385` | 5 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/44-帳號與賣家買家身份模型（Account & Seller-Buyer Identity）詳細設計書.md:L196,L581` | 2 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/48-SEO 優化與現況（SEO Optimization）詳細設計書.md:L194` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/53-Web 前端快取架構與版本一致性（Web Frontend Cache & Deployment Consistency）詳細設計書.md:L800` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/55-Hawkr 幫找採購助手（Procurement Assistant）詳細設計書.md:L477,L619` | 2 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/57-Hawkr 幫找 Agent Runtime（Agent Runtime）詳細設計書.md:L49,L427,L558,L559,L625` | 5 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/58-電子發票服務（Invoice Service）詳細設計書.md:L124,L399,L2450,L9737,L9893,L11160…共8處` | 8 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/59-買家會員分級與 Agent 額度方案（Buyer Membership & Entitlement）詳細設計書.md:L348` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/60-買家訂閱計費引擎（Buyer Subscription Billing Engine）詳細設計書.md:L287,L387,L398` | 3 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/64-Hawkr Agent 商務線（RFQ Publication、Seller Quote、Buyer Acceptance）詳細設計書.md:L7` | 4 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/65-物流履約平台（Fulfillment Platform）詳細設計書.md:L94,L601,L715,L1430,L3561,L4069…共14處` | 14 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/65A-物流履約前端實作規格.md:L629` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/docs/詳細設計書/66-Uber Direct 即時配送整合詳細設計書.md:L922,L1081,L3550,L3552` | 4 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/internal/platform/codemeta/code_meta.go:L1370,L1410,L1650,L1890,L1938,L2688…共8處` | 8 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/pkg/codemeta/registry.go:L212,L452,L500,L1208,L1514,L1628` | 6 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/services/ai-service/internal/procurement/usecase/graph.go:L20` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/services/ai-service/internal/procurement/usecase/graph_nodes.go:L389` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/services/ai-service/internal/procurement/usecase/graph_test.go:L55` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/services/invoice-service/internal/domain/logistics_fee.go:L53` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/FreshnessLabel.java:L7` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/PricebookQualityReasonCode.java:L7` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/ProcurementPublicationAudienceMode.java:L7` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/ProcurementRfqInviteDeliveryState.java:L7` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/ProcurementRfqPublicationApprovalState.java:L7` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/ProcurementRunStatus.java:L7` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/ProcurementSellerResponseStatus.java:L7` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/ProcurementSellerResponseType.java:L7` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `buyer-app/lib/core/codemeta/code_meta.dart:L1253,L1289,L1505,L1725,L1769,L2467…共8處` | 8 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `packages/hawkr_api/lib/gen/google/rpc/error_details.pb.dart:L81,L83,L182,L184,L244,L246…共28處` | 28 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `packages/hawkr_api/lib/gen/google/type/date.pb.dart:L66,L68` | 2 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `packages/hawkr_api/lib/gen/hawkr/agent/v1alpha1/agent.pb.dart:L58,L60,L130,L132,L194,L196…共18處` | 18 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `packages/hawkr_api/lib/gen/hawkr/buyer/v1alpha1/buyer.pb.dart:L101,L103,L303,L305,L463,L465…共118處` | 118 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `packages/hawkr_api/lib/gen/hawkr/common/v1alpha1/deadline.pb.dart:L57,L59` | 2 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `packages/hawkr_api/lib/gen/hawkr/common/v1alpha1/image.pb.dart:L47,L49` | 2 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `packages/hawkr_api/lib/gen/hawkr/common/v1alpha1/postal_address.pb.dart:L51,L53` | 2 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `packages/hawkr_api/lib/gen/hawkr/common/v1alpha1/store_card.pb.dart:L85,L87` | 2 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `packages/hawkr_api/lib/gen/hawkr/content/v1alpha1/content.pb.dart:L70,L72,L175,L177,L246,L248…共34處` | 34 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `packages/hawkr_api/lib/gen/hawkr/discovery/v1alpha1/discovery.pb.dart:L49,L51,L116,L118,L179,L181` | 6 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `packages/hawkr_api/lib/gen/hawkr/identity/v1alpha1/identity.pb.dart:L57,L59,L110,L112,L160,L162…共14處` | 14 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `packages/hawkr_api/lib/gen/hawkr/seller/v1alpha1/report.pb.dart:L87,L89,L251,L253,L363,L365…共14處` | 14 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `packages/hawkr_api/lib/gen/hawkr/seller/v1alpha1/seller.pb.dart:L68,L70,L147,L149,L314,L316…共168處` | 168 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `seller-app/lib/core/codemeta/code_meta.dart:L1253,L1289,L1505,L1725,L1769,L2467…共8處` | 8 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `website/lib/codemeta/generated.ts:L641,L656,L746,L861,L884,L1309…共8處` | 8 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |
| `website/middleware.ts:L29` | 1 | 改成新 repo 的 issue（D3）或拿掉（步驟 6） |

### 6b. `git show <sha>` 指引

| 檔案 | 處數 | 去向 |
|---|---|---|
| `admin/PRODUCT.md:L61` | 1 | 改成指向檔案（步驟 6） |
| `backend/AGENTS.md:L83` | 1 | 改成指向檔案（步驟 6） |
| `backend/cmd/new-module/templates/characterization/legacy.sh.tmpl:L5` | 1 | 改成指向檔案（步驟 6） |
| `backend/docs/adr/2026-09-13-category-alias-offline-generation.md:L85` | 2 | 改成指向檔案（步驟 6） |
| `backend/docs/adr/2026-09-14-pricebook-freshness-describes-age-not-usability.md:L8` | 2 | 改成指向檔案（步驟 6） |
| `backend/docs/adr/2026-10-03-vendor-portal-invite-scoped-access.md:L68` | 1 | 改成指向檔案（步驟 6） |
| `backend/docs/adr/2026-10-05-notification-module.md:L59` | 1 | 改成指向檔案（步驟 6） |
| `backend/docs/plans/2026-09-24-go-modular-monolith.md:L157` | 1 | 改成指向檔案（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10c-decision-brief.md:L106` | 1 | 改成指向檔案（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-measure-brief.md:L58` | 1 | 改成指向檔案（步驟 6） |
| `backend/docs/詳細設計書/64-Hawkr Agent 商務線（RFQ Publication、Seller Quote、Buyer Acceptance）詳細設計書.md:L39,L40` | 2 | 改成指向檔案（步驟 6） |
| `backend/internal/httpapi/social/characterization/driver_test.go:L6` | 1 | 改成指向檔案（步驟 6） |
| `backend/services/ai-service/internal/agent/return_policy_test.go:L12` | 1 | 改成指向檔案（步驟 6） |
| `website/docs/analytics-events.md:L5` | 1 | 改成指向檔案（步驟 6） |
| `website/styles/globals.css:L85` | 1 | 改成指向檔案（步驟 6） |
| `website/tests/visual/mobile-bottom-nav.spec.ts:L8` | 1 | 改成指向檔案（步驟 6） |

### 6c. 以編號引用 issue 或 PR（不是連結）

| 檔案 | 處數 | 去向 |
|---|---|---|
| `admin/src/app/admin/anomalies/page.tsx:L35,L1166` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `admin/src/app/admin/notification-templates/page.tsx:L160,L712` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `admin/src/app/admin/reports/page.tsx:L28,L33` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `admin/src/app/api/admin/delegated-operations/audit-logs/route.ts:L42` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `admin/src/features/delegated-operations/components/AllAuditLogConsole.tsx:L37` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `admin/src/features/delegated-operations/components/SessionListConsole.tsx:L192` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `admin/src/lib/codemeta/generated.ts:L310,L425,L448,L900…共8處` | 8 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `admin/src/lib/posts/adminList.ts:L25` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `admin/tests/admin-bff-no-seller-scope.test.ts:L52` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `admin/tests/admin-post-list-media-contract.test.ts:L12` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `admin/tests/delegated-operation-all-audit-logs.test.ts:L13,L193` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `admin/tests/notification-template-envelope-contract.test.ts:L172` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/.env.template:L123` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/.golangci.yml:L64` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/api/openapi/openapi.yaml:L9498,L9606,L19114,L19173…共10處` | 10 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/api/proto/account/account.proto:L95,L330,L479,L635` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/api/proto/ai/ai.proto:L118,L512,L554,L568…共10處` | 10 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/api/proto/buyer/buyer.proto:L55,L766` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/api/proto/notification/notification.proto:L24` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/api/proto/order/order.proto:L1157` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/api/proto/pricebook/pricebook.proto:L105,L127,L307,L314…共9處` | 9 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/api/proto/seller/seller.proto:L53,L170,L197,L640…共7處` | 7 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/api/proto/shipping/shipping.proto:L189,L192,L1335,L1336…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/dev.sh:L55,L76,L164,L619` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docker-compose-local.yml:L4` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/adr/2026-09-20-coordinator-vs-all-tools-fallback.md:L8` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/adr/2026-09-20-seller-stint-assistant-history.md:L8` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/seller-interface-inventory.md:L85,L86,L87,L88…共19處` | 19 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/runbooks/food-safety/sql/01-list-pending.sql:L6` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/runbooks/food-safety/sql/02-ack.sql:L8` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/runbooks/food-safety/sql/03-patch-status.sql:L10` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/runbooks/food-safety/sql/06-restore-suspension.sql:L12` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/runbooks/food-safety/sql/07-decide-appeal.sql:L10` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/runbooks/food-safety/sql/08-notify-seller.sql:L13` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/runbooks/food-safety/sql/09-mark-fda-reported.sql:L9` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/runbooks/invoice-cancelled-orders-backfill.md:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/runbooks/seller-notification-identity-store-id-backfill.md:L1,L4,L15,L23` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/代碼管理（CodeMeta）/scripts/generate.sh:L1119,L1200` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/代碼管理（CodeMeta）/內部技術碼.md:L537` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/代碼管理（CodeMeta）/業務狀態碼.md:L251,L689,L708,L740…共13處` | 14 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/代碼管理（CodeMeta）/業務類型碼.md:L915,L956,L976,L1016` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/基礎建設/ATM 退款結果 Webhook 遺失維運處置.md:L3` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/基礎建設/MQ Topology 管理（Terraform）.md:L18,L359,L378` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/基礎建設/Prod RabbitMQ 3-Node Cluster.md:L122,L128` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/基礎建設/業務告警分流與 dev-警報 路由設定.md:L3` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/詳細設計書/15-AI 微服務（AI Service）詳細設計書.md:L780,L784,L828,L851…共12處` | 12 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/詳細設計書/20-結帳模組（Checkout Module）詳細設計書.md:L1036` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/詳細設計書/33-商品 Hashtag 模組（Product Hashtag）詳細設計書.md:L181` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/詳細設計書/36-MQ 與事件總覽（MQ & Event Catalog）.md:L253,L326,L409,L1873…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/詳細設計書/57-Hawkr 幫找 Agent Runtime（Agent Runtime）詳細設計書.md:L23,L368,L374` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/詳細設計書/58-電子發票服務（Invoice Service）詳細設計書.md:L1451,L5779` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/詳細設計書/65A-物流履約前端實作規格.md:L1579` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/詳細設計書/67-定期配送訂閱（Subscription Delivery）詳細設計書.md:L477,L1729,L2573,L2627` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/詳細設計書/68-7-ELEVEN 大宗寄倉（B2C）詳細設計書.md:L3041,L3066` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/詳細設計書/69-商家自有出貨（Merchant Self-Delivery）詳細設計書.md:L215` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/開發規範/API 設計規範.md:L531` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/開發規範/DB Migration 相容性規範（expand-contract）.md:L178` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/開發規範/Java 微服務開發規範.md:L124` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/開發規範/MQ 與事件管理規範.md:L50,L54,L220,L221…共8處` | 9 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/開發規範/多副本排程搶佔規範.md:L105,L265,L348,L350` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/開發規範/後端服務責任邊界規範.md:L125,L150,L245` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/開發規範/新增微服務指南.md:L79` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/docs/開發規範/硬上限依賴接入治理規範（配額與容量）.md:L100` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/buyerid/resolver.go:L53` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/config/config.go:L37` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/grpc/ai_assistant_client.go:L23` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/grpc/circuit_breaker_interceptor.go:L38` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/grpc/envelope_guard_interceptor.go:L71` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/grpc/envelope_guard_interceptor_test.go:L145` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/grpc/pricebook_client.go:L110,L125,L140,L148…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/grpc/shipping_client.go:L286,L293` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/admin_notification_template.go:L170` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/ai.go:L1289,L1352,L1398,L1454` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/checkout.go:L862,L973` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/checkout_suspension_envelope_test.go:L19,L24,L119` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/coupon_test.go:L833` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/envelope_tier2_shape_test.go:L15` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/governance.go:L11,L198,L205` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/marketplace.go:L171` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/marketplace_test.go:L1120,L1182` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/moderation.go:L144,L197,L231,L303` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/notification_preference.go:L16` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/order_test.go:L289` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/post_test.go:L165,L168` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/pricebook.go:L214,L323` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/product.go:L217,L406,L861,L993` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/product_test.go:L540` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/recommendation.go:L48,L103` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/recommended_sellers_test.go:L134` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/restock_subscription_helper.go:L13` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/search.go:L55,L136` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/search_test.go:L32` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/seller.go:L364` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/seller_test.go:L88,L224` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/shipment_anomaly.go:L71,L72` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/testutil_mock_defaults_test.go:L14` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/handler/testutil_test.go:L591` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/middleware/body_guard.go:L83` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/middleware/body_guard_test.go:L246` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/middleware/idempotency.go:L38` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/middleware/idempotency_test.go:L17,L96` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/middleware/logger.go:L161` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/middleware/logger_allowlist_test.go:L9` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/middleware/ratelimit_ai_routes_test.go:L103,L107,L111` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/procurement/wire.go:L265,L406` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/proxy/agui_golden_test.go:L122` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/response/bizhttp.go:L99` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/response/bizhttp_payuni_merchant_profile_test.go:L15` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/response/response.go:L168,L414` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/router/constants.go:L238` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/router/invoice_idempotency_coverage_test.go:L13,L106` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/router/routes_admin_notification.go:L13` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/router/routes_ai_cache_control_test.go:L28` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/router/routes_dev.go:L11` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/router/routes_dev_test.go:L12,L54` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/router/routes_governance.go:L11` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/router/routes_moderation.go:L9` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/router/routes_shipping.go:L96` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/sse/capgroup.go:L13` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/sse/connection.go:L48` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/sse/di.go:L33` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/sse/hub.go:L28,L433` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/sse/hub_reclaim_test.go:L11` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/gateway/internal/sse/hub_test.go:L174` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/infra/mq/README.md:L260,L318,L398` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/internal/legacy/gen/account/account.pb.go:L1715,L2709,L3769` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/internal/legacy/gen/account/accountconnect/account.connect.go:L190,L666` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/internal/legacy/gen/buyer/buyerconnect/buyer.connect.go:L217,L914` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/internal/legacy/gen/seller/seller.pb.go:L2872,L4992,L10118` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/internal/legacy/gen/seller/sellerconnect/seller.connect.go:L348,L459,L1709,L1820` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/internal/platform/codemeta/code_meta.go:L1370,L1410,L1650,L1890…共12處` | 12 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/monitoring/grafana/provisioning/dashboards/json/auth-rotation.json:L5,L30` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/monitoring/prometheus/rules/business-alerts.yml:L2,L24,L43,L72…共8處` | 8 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/monitoring/prometheus/rules/infra-alerts.yml:L357` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/monitoring/sql-exporter/sql_exporter.yml:L1,L84,L100,L114` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/codemeta/registry.go:L212,L452,L500,L613…共10處` | 10 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/codemeta/registry_test.go:L158` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/crypto/crypto.go:L8` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/envelope/envelope.go:L208,L250` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/envelope/envelope_test.go:L47,L129` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/middleware/grpc/interceptor/error_interceptor.go:L62,L107` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/middleware/grpc/interceptor/error_interceptor_test.go:L12` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/payuni/client.go:L91` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/payuni/client_test.go:L72,L117` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/proto/account/account.pb.go:L1715,L2709,L3769` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/proto/account/account_grpc.pb.go:L119,L527` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/proto/ai/ai.pb.go:L747,L3699,L3901,L3915…共6處` | 6 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/proto/ai/ai_grpc.pb.go:L1345,L1347,L1349,L1352…共8處` | 8 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/proto/buyer/buyer_grpc.pb.go:L115,L713` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/proto/notification/notification_grpc.pb.go:L56,L257` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/proto/pricebook/pricebook.pb.go:L1017,L1070,L1764,L3399…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/proto/pricebook/pricebook_grpc.pb.go:L137,L159,L437,L459` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/proto/seller/seller.pb.go:L2872,L4992,L10118` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/proto/seller/seller_grpc.pb.go:L151,L263,L1314,L1426` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/proto/shipping/shipping.pb.go:L9473,L9474,L9600` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/proto/shipping/shipping_grpc.pb.go:L186,L188,L878,L880` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/redis/blacklist.go:L69` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/rolestate/rolestate.go:L7` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/server/grpc.go:L51` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/server/grpc_test.go:L82` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/server/http.go:L119` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/pkg/server/http_test.go:L67` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/scripts/proto/numeric-allowlist.txt:L19,L20,L158,L286…共8處` | 8 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/cmd/coordinator-eval/testdata/corpus.yaml:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/db/migrations/20260819135326_procurement_backfill_failure_reason.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/db/migrations/20260819145458_procurement_backfill_status_values.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/db/migrations/20260822061013_procurement_candidate_set_run_id.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/db/schema/procurement.sql:L149,L220` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/db/sqlc.yaml:L51` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/actor/actor.go:L90` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/cropidentity/histogram_boundary_source_test.go:L10` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/domain/artifact.go:L14,L157` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/domain/block.go:L3` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/domain/claim_codes.go:L4,L22` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/domain/dialogue_plan.go:L4,L29,L76` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/domain/dialogue_planner_test.go:L123` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/domain/pick_next_question.go:L44,L174` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/domain/pick_next_question_test.go:L70` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/domain/recorded.go:L5` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/domain/slots.go:L79` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/domain/steps.go:L76,L285` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/repository/advisory_lock_namespace_test.go:L77` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/repository/dbtest_test.go:L47` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/repository/price_preview.go:L164` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/usecase/candidate_platform_lane_test.go:L206` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/usecase/candidate_search.go:L220,L435` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/usecase/histogram_boundary_source_test.go:L9` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/usecase/run_dispatch.go:L126` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/usecase/run_dispatch_test.go:L40` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/usecase/surface_pipeline.go:L17` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/usecase/surface_realizer.go:L30,L41` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/usecase/surface_validator.go:L115` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/ai-service/internal/procurement/usecase/turn.go:L103,L120,L123,L127` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/im-service/internal/client/seller/client.go:L115` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/im-service/internal/client/seller/client_test.go:L205` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/im-service/internal/config/config.go:L38` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/im-service/internal/usecase/message.go:L111` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/invoice-service/db/queries/invoice_cancelled_orders.sql:L2` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/invoice-service/db/schema/schema.sql:L1087` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/invoice-service/internal/client/amego/client_test.go:L422` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/invoice-service/internal/client/amego/codes.go:L318` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/invoice-service/internal/repository/postgres/gen/invoice_cancelled_orders.sql.go:L30` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/invoice-service/internal/usecase/idempotency.go:L20` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/invoice-service/internal/usecase/submit.go:L538` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/pom.xml:L31,L228` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/config/AfterSalesConstants.java:L22` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/entity/AfterSaleCaseEntity.java:L71,L78,L123,L130…共6處` | 6 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/entity/RefundTaskEntity.java:L69` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/grpc/AfterSalesGrpcService.java:L63,L447,L659,L1710` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/grpc/client/BuyerGrpcClient.java:L31,L45,L91,L95…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/grpc/client/CouponGrpcClient.java:L16` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/grpc/client/OrderGrpcClient.java:L18` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/grpc/client/PaymentGrpcClient.java:L16,L40,L66` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/grpc/client/SellerGrpcClient.java:L34,L37,L73,L134…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/grpc/client/ShippingGrpcClient.java:L16,L44` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/mapper/AfterSaleCaseMapper.java:L43,L65,L90,L136…共12處` | 12 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/mapper/RefundTaskMapper.java:L125,L145,L164,L183` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/mq/AfterSalesEventPublisherImpl.java:L139,L168,L431,L478…共7處` | 7 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/mq/RefundResultConsumer.java:L297` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/mq/UnknownRefundFallbackConsumer.java:L119` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/mq/payload/AppealSubmittedPayload.java:L18` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/mq/payload/AwaitingBuyerRefundPagePayload.java:L18` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/mq/payload/PlatformDecisionMadePayload.java:L19` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/mq/payload/RefundCompletedPayload.java:L25` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/scheduler/AutoConfirmReceiptScheduler.java:L21,L72` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/scheduler/CaseProcessTimeoutWorker.java:L42,L101,L117,L135…共7處` | 7 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/scheduler/RefundSyncWorker.java:L22,L41,L68,L93` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/scheduler/RefundTaskWorker.java:L36,L89,L94,L114…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/scheduler/ReturnStuckAlertScheduler.java:L39,L68,L91,L157…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/scheduler/ReturnTimeoutWorker.java:L30,L72` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/scheduler/ReturnTwentyFourHourWarnScheduler.java:L32` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/scheduler/StorePickupTimeoutScheduler.java:L22,L72` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/service/AfterSaleLabelRetryService.java:L26,L46,L72` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/service/AfterSalePayoutHoldService.java:L36` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/service/AfterSalesEventPublisher.java:L7,L113,L118,L142…共9處` | 9 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/service/AtmRefundReconciler.java:L23` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/service/CaseAutoRejectService.java:L29,L33,L40,L81…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/service/CaseEscalationService.java:L20,L22,L28,L58…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/service/EventPrincipals.java:L4` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/service/EventPublishFailedException.java:L4` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/service/PrincipalResolutionFailedException.java:L5` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/service/ReturnPendingTimeoutCloseService.java:L18,L31,L71` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales/service/ReturnReceiptService.java:L48,L50,L101,L200…共11處` | 11 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/resources/db/migration/V3__add_refund_outcome.sql:L2` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/main/resources/db/migration/V9__add_scheduler_lease_columns.sql:L1,L26,L38,L74` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/grpc/client/PrincipalResolveDeadlineTest.java:L31,L56,L111` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/mapper/AfterSaleCaseMapperIT.java:L43,L313,L345` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/mapper/AfterSaleLabelRetryLeaseIT.java:L37` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/mq/AwaitingBuyerRefundPagePayloadGoldenTest.java:L14` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/mq/EventPrincipalPreResolutionTest.java:L28,L62` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/mq/GoldenPayloadKeys.java:L15` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/mq/RefundCompletedPayloadGoldenTest.java:L14` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/mq/ReturnShipmentStatusConsumerTest.java:L65` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/mq/ShipmentLabelConsumerTest.java:L66` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/scheduler/AutoConfirmReceiptSchedulerTest.java:L37,L91` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/scheduler/RefundSyncWorkerClaimTest.java:L29,L77` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/scheduler/RefundTaskWorkerClaimTest.java:L33,L97` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/scheduler/ReturnStuckAlertSchedulerTest.java:L71,L87,L117,L158…共8處` | 8 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/scheduler/ReturnTwentyFourHourWarnSchedulerTest.java:L25` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/scheduler/SchedulerPrincipalPreResolutionTest.java:L36` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/scheduler/StorePickupTimeoutSchedulerTest.java:L37,L92` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/service/AfterSaleLabelRetryServiceClaimTest.java:L29` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/service/AfterSaleResolutionServiceTest.java:L171` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/service/AtmRefundReconcilerTest.java:L32,L47,L107,L178` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/service/CaseTimeoutPublishInTxTest.java:L24,L52` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/service/ReturnPendingTimeoutCloseServiceTest.java:L21,L42` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/service/ReturnReceiptServiceTest.java:L52,L96,L109,L264…共6處` | 6 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/service/SellerCancelServiceTest.java:L129` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/cart-service/pom.xml:L31` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/cart-service/src/main/java/io/github/will413028/hawkr/cart/grpc/client/BuyerGrpcClient.java:L33` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/cart-service/src/main/java/io/github/will413028/hawkr/cart/grpc/client/SellerGrpcClient.java:L109` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/cart-service/src/main/java/io/github/will413028/hawkr/cart/service/CartService.java:L75,L93,L372` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/cart-service/src/test/java/io/github/will413028/hawkr/cart/service/CartServiceSelfItemTest.java:L139,L298` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/coupon-service/pom.xml:L29` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/coupon-service/src/main/java/io/github/will413028/hawkr/coupon/entity/projection/CouponExpiringRef.java:L6` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/coupon-service/src/main/java/io/github/will413028/hawkr/coupon/event/CouponEventPublisher.java:L53` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/coupon-service/src/main/java/io/github/will413028/hawkr/coupon/event/CouponExpiringEvent.java:L9` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/coupon-service/src/main/java/io/github/will413028/hawkr/coupon/grpc/CouponGrpcService.java:L444` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/coupon-service/src/main/java/io/github/will413028/hawkr/coupon/grpc/client/AccountGrpcClient.java:L25` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/coupon-service/src/main/java/io/github/will413028/hawkr/coupon/grpc/client/BuyerGrpcClient.java:L29,L43,L66` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/coupon-service/src/main/java/io/github/will413028/hawkr/coupon/grpc/client/OrderGrpcClient.java:L28` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/coupon-service/src/main/java/io/github/will413028/hawkr/coupon/grpc/client/SellerGrpcClient.java:L31,L46,L105` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/coupon-service/src/main/java/io/github/will413028/hawkr/coupon/mapper/CouponInstanceMapper.java:L208` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/coupon-service/src/main/java/io/github/will413028/hawkr/coupon/scheduler/CouponExpiringWarningJob.java:L21,L35` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/coupon-service/src/test/java/io/github/will413028/hawkr/coupon/grpc/CouponGrpcServiceSelfIssueTest.java:L124` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/main/java/io/github/will413028/hawkr/media/client/SellerGrpcClient.java:L24,L55` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/main/java/io/github/will413028/hawkr/media/controller/SecureMediaController.java:L124` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/main/java/io/github/will413028/hawkr/media/entity/MediaAssetEntity.java:L40,L85` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/main/java/io/github/will413028/hawkr/media/entity/UploadSessionEntity.java:L34,L85` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/main/java/io/github/will413028/hawkr/media/repository/MediaAssetRepository.java:L77,L104` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/main/java/io/github/will413028/hawkr/media/repository/UploadSessionRepository.java:L58,L85,L117,L146` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/main/java/io/github/will413028/hawkr/media/scheduler/MediaGarbageCollectionJob.java:L37,L77,L206,L279…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/main/java/io/github/will413028/hawkr/media/service/ImportAssetService.java:L1006` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/main/java/io/github/will413028/hawkr/media/util/AssetFilename.java:L6,L31` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/main/resources/db/migration/V8__add_media_gc_lease_columns.sql:L1,L26,L39` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/main/resources/db/migration/V9__add_original_filename.sql:L1,L35,L42` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/test/java/io/github/will413028/hawkr/media/controller/SecureMediaControllerTest.java:L49` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/test/java/io/github/will413028/hawkr/media/grpc/ServerReadSecureAssetTest.java:L44` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/test/java/io/github/will413028/hawkr/media/scheduler/MediaGarbageCollectionJobClaimTest.java:L45` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/test/java/io/github/will413028/hawkr/media/scheduler/MediaGarbageCollectionJobTest.java:L78,L107` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/test/java/io/github/will413028/hawkr/media/service/ImportAssetServiceFinalizeOwnershipTest.java:L136` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/test/java/io/github/will413028/hawkr/media/service/UploadSessionServiceTest.java:L20` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/media-service/src/test/java/io/github/will413028/hawkr/media/util/AssetFilenameTest.java:L11` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/pom.xml:L31,L257` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/checkout/CheckoutService.java:L125,L154,L1301` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/checkout/CheckoutValidator.java:L364` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/checkout/UberOrderEligibilityGuard.java:L27` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/checkout/client/CheckoutCouponGrpcClient.java:L22` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/checkout/client/SellerGrpcClient.java:L36,L119` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/config/OrderConfig.java:L90` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/entity/OrderAggregateResult.java:L6` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/grpc/client/AfterSalesGrpcClient.java:L162` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/grpc/client/BuyerGrpcClient.java:L34,L123` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/grpc/client/CartGrpcClient.java:L29,L135` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/grpc/client/PaymentGrpcClient.java:L24` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/grpc/client/ProductGrpcClient.java:L32,L44` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/mq/OrderEventPublisher.java:L30,L63` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/mq/OrderOutboxPublisher.java:L29,L63,L90,L131…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/mq/dto/OrderPaidPayload.java:L25` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/repository/AutoRefundTaskRepository.java:L30,L57` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/repository/CartCleanupRetryTaskRepository.java:L30,L57` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/repository/CouponConsumptionTaskRepository.java:L30,L57` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/repository/GrantPaymentTaskRepository.java:L47,L73,L168` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/repository/OrderOutboxMapper.java:L23,L52` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/repository/OrderRepository.java:L74,L114,L1095,L1134…共6處` | 6 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/repository/StockTxRecordRepository.java:L100` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/scheduler/OrderOutboxPublisherScheduler.java:L20,L33,L78,L96` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/scheduler/ShipmentCreationBackstopScheduler.java:L26,L34,L100` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/service/OrderCompletionTxHandler.java:L36,L115,L128` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/service/OrderService.java:L95,L143,L262,L882…共6處` | 6 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/worker/AutoRefundWorker.java:L25,L38` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/worker/CartCleanupRetryWorker.java:L21,L34` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/worker/CouponConsumptionWorker.java:L20,L33` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/worker/GrantPaymentWorker.java:L31,L64` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/worker/StockCompensationWorker.java:L28,L43,L82` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/resources/application.yml:L135` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/resources/db/migration/V10__add_shipment_backstop_given_up_and_outbox_index.sql:L1,L25` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/main/resources/db/migration/V11__add_order_cancelled_at.sql:L1,L11` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/checkout/CheckoutServiceSelfPurchaseTest.java:L22,L76,L139` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/checkout/CheckoutServiceStoreCodeTest.java:L132` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/checkout/CheckoutValidatorFreeShippingTest.java:L15,L55` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/mq/OrderOutboxPublisherCancelledEnrichTest.java:L146,L217` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/mq/ShippingEventConsumerRecoveryTest.java:L30,L31` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/repository/OrderCancelTimestampIT.java:L33` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/repository/OrderCancelTimestampSqlTest.java:L25` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/repository/OrderItemRepositoryOrderingSqlTest.java:L9` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/repository/RefundRequestRepositoryOrderingSqlTest.java:L9` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/repository/RepositoryOrderingSqlTest.java:L24` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/scheduler/OrderOutboxPublisherSchedulerClaimTest.java:L29,L123,L164` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/scheduler/ShipmentCreationBackstopSchedulerClaimTest.java:L29,L45,L198,L246` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/service/OrderServiceNotificationPayloadTest.java:L101,L164,L189` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/util/OrderShippingProgressTest.java:L10` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/worker/AutoRefundWorkerTest.java:L79,L103` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/worker/GrantPaymentWorkerTest.java:L269,L293` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/worker/StockCompensationWorkerClaimTest.java:L28,L113` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/worker/TaskWorkerClaimLeaseTest.java:L31` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/pom.xml:L29,L295` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/adapter/grpc/PaymentGrpcService.java:L860` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/adapter/grpc/client/OrderGrpcClient.java:L25` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/adapter/webhook/handler/WebhookHandleResult.java:L8` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/application/service/PaymentApplicationService.java:L750,L808` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/domain/GrantResponseRules.java:L118` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/domain/repository/PaymentTransactionRepository.java:L207` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/infrastructure/mq/PaymentOutboxPublisher.java:L39,L75,L124,L143` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/infrastructure/mq/PaymentOutboxWriter.java:L22,L48,L63,L66…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/infrastructure/persistence/mapper/OutboxEventMapper.java:L35` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/infrastructure/persistence/mapper/PaymentTransactionMapper.java:L595` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/scheduler/ExpiredTransactionCloseService.java:L12` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/scheduler/ExpiredTransactionScheduler.java:L18` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/scheduler/OutboxEventPublisherScheduler.java:L20,L67` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/scheduler/PendingPaymentReconcileScheduler.java:L34,L76,L126` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/scheduler/PendingPaymentReconcileService.java:L102,L268` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/adapter/webhook/handler/PaymentNotifyHandlerConflictPathTest.java:L31` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/adapter/webhook/handler/PaymentNotifyHandlerFailedPathTest.java:L31` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/application/service/AtmRefundAcceptedNoActiveTest.java:L25` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/application/service/AtmRefundWebhookFinalizeTest.java:L26` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/application/service/RescueRefundAmountLimitTest.java:L34` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/domain/GrantResponseRulesTest.java:L101` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/infrastructure/mq/PaymentOutboxPublisherClaimTest.java:L29` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/scheduler/ExpiredTransactionOutboxTest.java:L33` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/scheduler/PendingPaymentReconcileSchedulerClaimTest.java:L30,L135` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/scheduler/PendingPaymentReconcileServiceFailureTest.java:L121,L137,L152,L170…共6處` | 6 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payuni-webhook-service/pom.xml:L29,L244` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payuni-webhook-service/src/main/java/io/github/will413028/hawkr/webhook/infrastructure/mq/OutboxPublisher.java:L57,L76` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payuni-webhook-service/src/main/java/io/github/will413028/hawkr/webhook/infrastructure/persistence/mapper/OutboxEventMapper.java:L63` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payuni-webhook-service/src/main/java/io/github/will413028/hawkr/webhook/scheduler/OutboxPublisherScheduler.java:L21,L33,L80` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payuni-webhook-service/src/main/java/io/github/will413028/hawkr/webhook/service/WebhookReceiveService.java:L242` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payuni-webhook-service/src/test/java/io/github/will413028/hawkr/webhook/infrastructure/mq/OutboxPublisherGuardedWriteTest.java:L27` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/payuni-webhook-service/src/test/java/io/github/will413028/hawkr/webhook/scheduler/OutboxPublisherSchedulerClaimTest.java:L29,L107` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/main/java/io/github/will413028/hawkr/productimport/entity/ProductImportJobEntity.java:L43` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/main/java/io/github/will413028/hawkr/productimport/grpc/client/MediaGrpcClient.java:L22` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/main/java/io/github/will413028/hawkr/productimport/grpc/client/ProductGrpcClient.java:L32,L51` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/main/java/io/github/will413028/hawkr/productimport/grpc/client/ShippingGrpcClient.java:L23` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/main/java/io/github/will413028/hawkr/productimport/helper/HtmlToBlocksConverter.java:L24` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/main/java/io/github/will413028/hawkr/productimport/listener/MediaCallbackListener.java:L115` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/main/java/io/github/will413028/hawkr/productimport/mapper/ProductImportItemAssetMapper.java:L46` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/main/java/io/github/will413028/hawkr/productimport/mapper/ProductImportJobMapper.java:L69,L83` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/main/java/io/github/will413028/hawkr/productimport/scheduler/ProductImportGcJob.java:L48,L79,L194,L276` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/main/java/io/github/will413028/hawkr/productimport/service/CommitJobService.java:L59` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/main/java/io/github/will413028/hawkr/productimport/service/CsvParseService.java:L244,L435` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/main/java/io/github/will413028/hawkr/productimport/service/ItemMutationService.java:L345,L497,L898,L961…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/main/resources/db/migration/V10__add_product_import_gc_lease.sql:L1,L29` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/main/resources/db/migration/V5__add_media_dedup_callback_index.sql:L2` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/test/java/io/github/will413028/hawkr/productimport/CodeMetaCommentDriftTest.java:L38` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/test/java/io/github/will413028/hawkr/productimport/grpc/ProductImportServiceImplTest.java:L24` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/test/java/io/github/will413028/hawkr/productimport/scheduler/ProductImportGcJobClaimTest.java:L41` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-import-service/src/test/java/io/github/will413028/hawkr/productimport/scheduler/ProductImportGcJobTest.java:L144` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-service/src/main/java/io/github/will413028/hawkr/product/grpc/client/BuyerGrpcClient.java:L30` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-service/src/main/java/io/github/will413028/hawkr/product/grpc/client/MediaGrpcClient.java:L31` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-service/src/main/java/io/github/will413028/hawkr/product/grpc/client/SellerGrpcClient.java:L35` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-service/src/main/java/io/github/will413028/hawkr/product/grpc/client/ShippingGrpcClient.java:L25` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-service/src/main/resources/db/migration/V5__add_nonneg_checks_hashtag_sales.sql:L2` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/product-service/src/test/java/io/github/will413028/hawkr/product/domain/text/DescriptionTextExtractorTest.java:L14` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/grpc/InternalAuthConfig.java:L4` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/grpc/InternalAuthSigner.java:L11` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/grpc/InternalAuthVerifier.java:L7` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/grpc/PrincipalResolution.java:L37` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/mq/payload/ShipmentAnomalyDetectedPayload.java:L29` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/AfterSalesEventType.java:L38` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/FreshnessLabel.java:L7` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/PricebookQualityReasonCode.java:L7` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/ProcurementDealTemperatureReason.java:L7` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/ProcurementErrorReason.java:L36,L37` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/ProcurementPublicationAudienceMode.java:L7` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/ProcurementPublicationState.java:L7` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/ProcurementRfqInviteDeliveryState.java:L7` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/ProcurementRfqPublicationApprovalState.java:L7` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/ProcurementRunStatus.java:L7` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/ProcurementSellerResponseStatus.java:L7` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/ProcurementSellerResponseType.java:L7` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/test/java/io/github/will413028/hawkr/common/grpc/InternalAuthSignerTest.java:L10` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/test/java/io/github/will413028/hawkr/common/grpc/PrincipalResolutionTest.java:L16` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/common-core/src/test/java/io/github/will413028/hawkr/common/utils/code/helper/AfterSaleCaseStatusRulesTest.java:L10,L54` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/java-proto/src/main/proto/account/account.proto:L95,L330,L479,L635` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/java-proto/src/main/proto/ai/ai.proto:L118,L512,L554,L568…共10處` | 10 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/java-proto/src/main/proto/buyer/buyer.proto:L55,L766` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/java-proto/src/main/proto/notification/notification.proto:L24` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/java-proto/src/main/proto/order/order.proto:L1157` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/java-proto/src/main/proto/pricebook/pricebook.proto:L105,L127,L307,L314…共9處` | 9 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/java-proto/src/main/proto/seller/seller.proto:L53,L170,L197,L640…共7處` | 7 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shared/java-proto/src/main/proto/shipping/shipping.proto:L189,L192,L1335,L1336…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/client/BuyerGrpcClient.java:L41,L96` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/client/MediaGrpcClient.java:L153` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/client/OrderGrpcClient.java:L18` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/client/SellerGrpcClient.java:L17` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/entity/ShipmentAnomalyAcknowledgmentEntity.java:L13` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/entity/ShipmentEntity.java:L106,L111` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/entity/projection/HolidayYearCoverageRef.java:L6` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/mapper/HolidayMapper.java:L15` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/mapper/ShipmentAnomalyQueryMapper.java:L63,L190,L226,L257…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/mapper/ShipmentMapper.java:L312,L335,L359,L381…共6處` | 6 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/mq/ShipmentAnomalyEventPublisher.java:L108` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/mq/ShippingEventPublisher.java:L132` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/scheduler/ProcessingDeadlineScheduler.java:L27,L48,L107,L118…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/scheduler/ShipmentLabelReconcileScheduler.java:L39,L59,L117` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/scheduler/ShipmentStatusReconcileScheduler.java:L30,L92` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/scheduler/TcatCallStuckAlertScheduler.java:L29,L52,L97` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/service/ShipmentAnomalyService.java:L125,L133` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/service/ShippingService.java:L231,L1961,L2058,L2305` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/service/dto/ShipmentCreatedPayload.java:L28,L35` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/resources/db/migration/V10__add_shipment_lease_columns.sql:L1,L26,L44` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/main/resources/db/migration/V6__extend_shipment_anomaly_ack_actor.sql:L2` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/contract/ShippingPayloadGoldenContractTest.java:L22` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/mapper/ShipmentAnomalyQueryMapperSqlTest.java:L9` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/mq/ShipmentAnomalyEventPublisherTest.java:L80,L114,L146,L179` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/mq/ShippingEventPublisherTest.java:L21` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/scheduler/ProcessingDeadlineSchedulerClaimTest.java:L29` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/scheduler/ProcessingDeadlineSchedulerTest.java:L45,L88` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/scheduler/ShipmentReconcileClaimTest.java:L34` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/scheduler/TcatCallStuckAlertSchedulerClaimTest.java:L29` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping/service/ShippingServiceTest.java:L53,L471,L545,L559…共8處` | 8 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/Makefile:L13` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260501000001_add_buyer_info_to_order_placed_template.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260501000002_add_buyer_info_to_review_created_template.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260527000000_member_pending_template_reason_fallback.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260531000000_rebind_order_placed_to_shipment_created.sql:L16` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260618141717_order_payload_snake_case.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260619000708_shipping_payload_snake_case.sql:L1,L2` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260703130000_template_ssot_baseline.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260703130001_1176_ghost_template_cleanup.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260703140000_1176_delete_broken_refund_templates.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260703150000_1176_coupon_expiring_snake.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260703160000_1176_flipped_events_template_snake.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260703170000_1286_enable_payout_completed.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260713020000_post_comment_title_actor_name_camelcase.sql:L5` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260713130000_fix_order_cancelled_buyer_exclude_suspended_reason.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260714000000_1508_enable_shipment_delivered_buyer.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260714010000_1514_seller_suspension_reason_label.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260828130000_add_merchant_verification_under_review_template.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260830100000_seller_target_field_store_id_rekey_batch1.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260830100002_seller_identity_migrated_batch1.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260830100003_notification_announcement_identity_migrated_marker.sql:L1,L17,L42` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260830110000_seller_target_field_store_id_rekey_batch2.sql:L1,L4,L5,L28…共13處` | 13 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260830110001_seller_identity_migrated_batch2.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260830120000_seller_target_field_store_id_rekey_batch3.sql:L1,L13,L18` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260830120001_seller_identity_migrated_batch3.sql:L1,L5` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/migrations/20260903050000_seller_invoice_target_field_store_id_rekey.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/db/queries/device_tokens.sql:L23` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/internal/domain/device_token.go:L29` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/internal/grpc/server_admin.go:L81` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/internal/repository/postgres/device_token_identity_test.go:L6,L41` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/internal/repository/postgres/gen/device_tokens.sql.go:L43` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/internal/repository/postgres/notification_inbox_test.go:L186` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/internal/usecase/notification_inbox.go:L32,L188` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/internal/usecase/notification_inbox_test.go:L585,L705` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/internal/worker/di.go:L70` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/internal/worker/seller_deleted.go:L16,L49` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/internal/worker/template_contract_test.go:L3` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/internal/worker/template_handler.go:L91,L96,L107,L397` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/notification-service/internal/worker/template_handler_test.go:L133,L558,L1439,L1471…共11處` | 11 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/db/migrations/20260830034310_bootstrap_category_crop_mapping_exact_matches.sql:L1,L14` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/db/migrations/20260830053759_backfill_category_crop_mapping_group_code.sql:L32` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/db/migrations/20260902170000_bootstrap_ingest_partitions.sql:L14` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/db/queries/category_crop_mapping.sql:L36` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/db/queries/observation.sql:L180` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/app/app_validate_test.go:L85,L92` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/client/category/client.go:L3` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/config/config.go:L26,L42,L49,L74` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/config/config_test.go:L38` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/domain/category_mapping.go:L33` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/domain/forecast/datameta.go:L28,L156` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/domain/forecast/gate.go:L118,L131` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/domain/forecast/group.go:L124,L357` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/domain/forecast/trend.go:L3,L136,L188,L241` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/domain/geography/boundary.go:L1,L24,L75` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/domain/geography/boundary_test.go:L56` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/domain/geography/geography.go:L104` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/domain/geography/registry.go:L2` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/domain/repository.go:L60,L72,L77,L141…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/domain/spatial/adaptive_scale.go:L11,L12,L33,L45…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/domain/spatial/rollup.go:L7,L38,L40` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/domain/spatial/scale.go:L21,L25` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/domain/spatial/scale_test.go:L35` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/domain/spatial/surface.go:L13` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/grpc/di.go:L47,L48` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/grpc/handler.go:L83,L90,L97,L104…共14處` | 14 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/grpc/handler_test.go:L60,L583,L1081,L1196` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/grpc/spatial_surface_integration_test.go:L59,L150` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/repository/postgres/category_mapping_repo.go:L22` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/repository/postgres/gen/category_crop_mapping.sql.go:L140` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/repository/postgres/gen/observation.sql.go:L528` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/repository/postgres/observed_price_repo_test.go:L73,L318,L387` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/datameta.go:L10,L21` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/di.go:L43,L45,L58,L63…共7處` | 7 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/forecast.go:L71,L480,L650,L678…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/forecast_curve.go:L50` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/forecast_curve_test.go:L33,L608` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/forecast_test.go:L29,L563,L821,L1389` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/list_category_mapping_worklist.go:L12` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/observed_price.go:L33,L77` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/observed_price_test.go:L877` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/regional_comparison.go:L46,L56,L81,L88…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/regional_comparison_test.go:L37,L180` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/spatial_surface.go:L44,L64,L129,L130…共10處` | 11 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/spatial_surface_test.go:L59,L70,L153,L189…共17處` | 17 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/trend_summary.go:L36` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/trend_summary_fixtures_test.go:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/trend_summary_integration_test.go:L3` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/internal/usecase/upsert_category_mapping.go:L67` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/pricebook-service/reference/MARKET_GEOGRAPHY.md:L3` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/review-service/internal/usecase/review_usecase.go:L80,L245` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/review-service/internal/usecase/review_usecase_test.go:L790` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/db/migrations/20260714083918_1534_suspension_reason_code_name_to_value.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/db/migrations/20260902164500_null_stale_webhook_raw_response.sql:L10` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/db/queries/payuni_merchant_profile_updates.sql:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/db/queries/seller_deleted_outbox.sql:L6,L14` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/app/cron/governance_jobs.go:L15` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/client/account/client.go:L57` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/client/gcis/client.go:L10,L102` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/client/gcis/client_test.go:L146` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/config/config.go:L37` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/domain/errors.go:L22,L34` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/domain/food_safety_report.go:L121,L123` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/domain/merchant_verification_outbox.go:L22,L23,L27,L86` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/domain/merchant_verification_outbox_test.go:L51` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/domain/payuni_merchant_profile_update.go:L10,L40` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/domain/seller_deleted_outbox.go:L96` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/domain/seller_governance_test.go:L204,L205` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/domain/seller_suspension.go:L24,L63` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/domain/seller_suspension_test.go:L5` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/domain/seller_violation_report.go:L11` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/grpc/governance_handler.go:L15,L481` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/grpc/mapper_test.go:L265` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/grpc/suspension_handler.go:L23` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/grpc/suspension_reason_mapping_test.go:L118,L138` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/mq/golden_contract_test.go:L45` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/mq/merchant_event_publisher.go:L29,L46` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/repository/postgres/gen/payuni_merchant_profile_updates.sql.go:L77` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/repository/postgres/gen/seller_deleted_outbox.sql.go:L30,L87` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/repository/postgres/resolve_principals_integration_test.go:L87` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/repository/postgres/seller_deleted_outbox.go:L56` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/account_client.go:L24,L45` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/address_geocode.go:L74` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/address_geocode_test.go:L121,L143,L151,L155` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/cert_payuni_axis_test.go:L152` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/certification_kyc_reference_lifecycle_test.go:L14` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/certification_reject_payuni_test.go:L15` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/certification_validator.go:L145` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/delete_seller_account.go:L102,L202` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/delete_seller_account_test.go:L127` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/dto/admin_res.go:L26` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/dto/payuni_res.go:L24` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/governance_test.go:L41,L54,L215,L235…共6處` | 6 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/payuni_client.go:L48` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/payuni_merchant_profile.go:L22,L89,L93,L139` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/payuni_merchant_profile_test.go:L255` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/seller_suspension.go:L78` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/seller_suspension_test.go:L138` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/violation.go:L142,L361` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/usecase/violation_report.go:L30,L73` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/worker/seller_deleted_outbox_claim_test.go:L12` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/worker/seller_deleted_outbox_scheduler.go:L24` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/worker/seller_deleted_outbox_worker.go:L10,L77` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/worker/seller_deleted_outbox_worker_test.go:L34,L164,L553,L570` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/seller-service/internal/worker/seller_lifecycle_role_sync_test.go:L12` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/social-service/internal/client/buyer/client_test.go:L139` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/social-service/internal/client/seller/client.go:L96,L121` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/social-service/internal/client/seller/client_test.go:L199,L252` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/social-service/internal/config/config.go:L116` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/services/social-service/internal/usecase/comment_snapshot_test.go:L11` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `backend/tests/contract/golden/README.md:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/core/codemeta/code_meta.dart:L1253,L1289,L1505,L1725…共12處` | 12 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/core/routing/app_router.dart:L321,L329` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/ai_assistant/data/ai_assistant_repository.dart:L37` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/ai_assistant/presentation/ai_assistant_screen.dart:L695` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/cart/data/cart_model.dart:L23` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/cart/data/cart_model.freezed.dart:L19,L246` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/cart/presentation/cart_screen.dart:L148` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/cart/presentation/widgets/cart_product_tile.dart:L39` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/category/data/product_item_model.dart:L3` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/order/data/after_sale_model.dart:L37,L41,L210,L368` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/order/data/after_sale_model.freezed.dart:L21,L22,L314,L318` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/order/data/after_sale_repository.dart:L156,L199` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/order/presentation/after_widgets/after_sale_detail_screen.dart:L190,L325,L540,L653…共7處` | 7 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/order/presentation/after_widgets/after_sale_list_screen.dart:L205,L286` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/order/presentation/after_widgets/arbitration_view_screen.dart:L22` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/order/presentation/after_widgets/submit_appeal_screen.dart:L18` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/order/presentation/order_card_widgets/after_sale_expanded_section.dart:L203` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/features/product/data/hashtag_extractor.dart:L3` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/shared/utils/amount_utils.dart:L3` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/shared/utils/price_display.dart:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/lib/shared/utils/recipient_name_validator.dart:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/test/features/social/post_contract_no_legacy_keys_test.dart:L4` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `buyer-app/tool/presentation_io_policy.json:L7,L14,L21,L28…共8處` | 8 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `patches/UPSTREAM-WATCH.md:L79` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `pnpm-workspace.yaml:L37` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `scripts/check-fullwidth-var-expansion.sh:L6,L55` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/docs/architecture/presentation-boundary.md:L3` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/config/media_image.dart:L64` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/core/codemeta/code_meta.dart:L1253,L1289,L1505,L1725…共12處` | 12 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/features/ai_assistant/data/ai_assistant_repository.dart:L33` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/features/auth/presentation/welcome_screen.dart:L20` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/features/chat/data/chat_repository.dart:L116` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/features/order/data/after_sale_model.dart:L67` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/features/order/data/after_sale_model.freezed.dart:L20,L310` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/features/post/data/post_hidden.dart:L7` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/features/post/data/post_model.dart:L38,L75` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/features/post/data/post_repository.dart:L517` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/features/product/data/hashtag_extractor.dart:L6,L41` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/features/product/data/product_upload_service.dart:L558` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/features/product/presentation/product_setup_info_screen.dart:L211` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/features/seller/data/bank_book_ocr_service.dart:L23,L144,L226,L268…共6處` | 6 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/features/seller/data/id_card_ocr_service.dart:L31,L60,L97,L154…共12處` | 12 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/shared/utils/price_display.dart:L1` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/shared/widgets/hashtag_autocomplete_popover.dart:L38` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/shared/widgets/pasted_table_parser.dart:L27` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/shared/widgets/rich_text_editor.dart:L346,L435,L591,L675` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/lib/shared/widgets/rich_text_editor_text_block.dart:L399,L767` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/test/core/networking/page_envelope_contract_test.dart:L28` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/test/core/router/stale_bootstrap_redirect_guard_test.dart:L1,L4,L171` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/test/core/session/session_provider_test.dart:L225` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/test/features/post/post_media_contract_test.dart:L10` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/test/features/product/data/hashtag_extractor_test.dart:L133` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/test/shared/widgets/pasted_table_parser_test.dart:L92` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `seller-app/test/shared/widgets/rich_text_editor_table_test.dart:L6` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/app/[locale]/(commerce)/orders/[orderNo]/page.tsx:L5` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/docs/transaction-policy.md:L207` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/cart/CartClient.tsx:L267,L295` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/cart/components/CartProductTile.tsx:L12,L117,L149,L323` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/market/components/AppliedFilters.tsx:L45` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/payments/screens/PaymentLandingClient.tsx:L18` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/payments/screens/PaymentReturnClient.tsx:L27` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/procurement-assistant/BuyerQuoteComparison.tsx:L26` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/procurement-assistant/QuoteBatchSummary.tsx:L95` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/procurement-assistant/__tests__/QuoteBatchSummary.test.ts:L22` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/procurement-assistant/__tests__/RfqDraftFlow.snapshot-reconfirm.test.ts:L176` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/procurement-assistant/__tests__/rfq-price-contract.test.ts:L368` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/procurement-assistant/__tests__/runtime-snapshot-contract.test.ts:L16` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/procurement-assistant/activity.unit.test.ts:L216` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/procurement-assistant/buyer-quote-contract.ts:L12` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/procurement-assistant/narrative.ts:L23` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/procurement-assistant/steps.ts:L7` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/procurement-assistant/useBuyerQuoteComparison.ts:L20` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/procurement-assistant/useRfqRuntimeSnapshot.ts:L18` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/procurement-assistant/useRfqRuntimeSnapshot.unit.test.ts:L8` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/product/components/ProductPurchaseBox.tsx:L69,L132` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/seller-center/insights/CropCatalogSearch.tsx:L79` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/seller-center/products-import/adapters/importItemAdapter.ts:L269,L325,L380` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/seller-center/products-import/components/ImportItemEditForm.tsx:L273,L443` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/seller-center/products-import/i18n.ts:L100,L202` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/seller-center/products-import/types.ts:L101,L180,L199,L395…共5處` | 5 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/seller-center/products/components/SkuShippingEditor.tsx:L136` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/features/seller-center/wallet/WalletClient.tsx:L48` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/api/index.ts:L6,L20,L32` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/api/openapi-types.ts:L34,L202,L229,L268` | 4 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/cart/types.ts:L27` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/checkout/repo.ts:L343` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/codemeta/__tests__/enums.unit.test.ts:L45` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/codemeta/enums.ts:L74,L90` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/codemeta/generated.ts:L641,L656,L746,L861…共10處` | 10 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/fonts/README.md:L178` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/fonts/roboto.ts:L10` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/fonts/zenMaruGothic.ts:L12` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/market/listFilters.ts:L91` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/notifications/types.ts:L19` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/pricebook/repo.ts:L26` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/products/clipboardGrid.ts:L3` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/products/pricing.ts:L5,L99` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lib/seller/canonical-redirect.ts:L10,L32` | 2 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lighthouserc.desktop.cjs:L46` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/lighthouserc.mobile.cjs:L52` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/stories/web/BuyerQuoteComparison.stories.tsx:L12` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/stories/web/QuoteBatchSummary.stories.tsx:L11` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/styles/features/cart.css:L452` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/styles/features/procurement-assistant/buyer-quote-compare.module.css:L2` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/styles/features/procurement-assistant/quote-batch-summary.module.css:L2` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/styles/features/seller-center.css:L15987` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/tests/<舊名-子品牌>-candidate-canvas.test.ts:L65,L74,L109` | 3 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/tests/description-clipboard-grid.test.ts:L13` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/tests/description-compact.test.ts:L8` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/tests/description-editorjs-list.test.ts:L10` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/tests/e2e/helpers/mock-procurement-rfq.ts:L60` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/tests/e2e/insights-optional-market.spec.ts:L69` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/tests/home-initial-loading.test.ts:L92` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/tests/import-item-description.test.ts:L9` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/tests/procurement-assistant-turn-stream.test.ts:L86` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |
| `website/tests/seller-reviews-media-contract.test.ts:L12` | 1 | 保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明） |

## 7. `.claude/`、`.agents/` 與其他檔案的個人路徑與私人參照

- `.claude/` 與 `.agents/` 追蹤中的路徑 6 個（`.claude/skills/*` 是指向 `.agents/skills/*` 的符號連結）；全樹含個人路徑或私人參照的檔 114 個。
- 指令：`git ls-files .claude .agents`；`python3 backend/docs/plans/2026-10-24-portfolio-public/scan.py --category private --report`（本機路徑 `/Users/…`、second-brain 與私人 wiki 路徑）。
- 快照：e5d5a8d8e（程式碼與 332da5e51 相同；之後只新增了計畫與 ADR）。

| 檔案 | 型態 | 私人參照處數 | 去向 |
|---|---|---|---|
| `.agents/skills/backend-e2e-verify/SKILL.md` | 檔案 | 1 | 移除個人路徑與私人參照（步驟 6） |
| `.agents/skills/backend-e2e-verify/mq_capture.go.tmpl` | 檔案 | 0 | 保留；內文對舊名與舊路徑的清洗依第 2 節 |
| `.agents/skills/website-e2e-verify/SKILL.md` | 檔案 | 0 | 保留；內文對舊名與舊路徑的清洗依第 2 節 |
| `.agents/skills/website-e2e-verify/references/pricebook-market-watchlist.md` | 檔案 | 0 | 保留；內文對舊名與舊路徑的清洗依第 2 節 |
| `.claude/skills/backend-e2e-verify` | 符號連結 | 0 | 保留（符號連結） |
| `.claude/skills/website-e2e-verify` | 符號連結 | 0 | 保留（符號連結） |

其他檔案的個人路徑與私人參照：

| 位置 | 種類 | 去向 |
|---|---|---|
| `AGENTS.md:L5` | second-brain | 移除或改成相對路徑（步驟 6） |
| `admin/wrangler.jsonc:L7` | 私人 wiki 路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/.gitignore:L155` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/AGENTS.md:L54` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/Makefile:L345` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/cmd/jetgen/tables.go:L54` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-27-a2-identity.md:L96,L143` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller.md:L382` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/buyer-inventory.md:L347` | 私人 wiki 路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/public-read-backend-inventory.md:L113` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/public-read-client-inventory.md:L42` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/public-read-client-inventory.md:L42,L83,L149,L184,L208` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/seller-api-client-inventory.md:L266` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/seller-api-client-inventory.md:L266` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3ca-design.md:L724` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3ca-design.md:L1603,L1605` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-design.md:L406` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-design.md:L413` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-inventory.md:L3` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-inventory.md:L3` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-queue-review1.md:L3` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-queue-review1.md:L3` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-queue-review2.md:L3` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-queue-review2.md:L3` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-4a-design.md:L252` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-4b-design.md:L106,L243` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-4b-design.md:L106` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-5a-design.md:L219,L430` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-5c-design.md:L100` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-5c-design.md:L100` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-5d-design.md:L119` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-L0-copy.md:L5` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-L0-copy.md:L5` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-Lprime-design.md:L5` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-Lprime-design.md:L5` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-10-18-a7-social/alignment-facts.md:L59,L126,L127` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-10-18-a7-social/alignment-facts.md:L59,L126,L127` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-10-18-a7-social/client-inventory.md:L41` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/plans/2026-10-18-a7-social/step-1a-perf/README.md:L32` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/docs/詳細設計書/02-產品微服務（Product Service）詳細設計書.md:L3053` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/docs/開發規範/Web 前端契約同步規範.md:L82,L96` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/services/ai-service/db/schema/procurement.sql:L637` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/services/ai-service/internal/procurement/goldenset/testdata/scenarios.yaml:L89` | 私人 wiki 路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/services/ai-service/internal/procurement/usecase/graph_resume_protocol_test.go:L132` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/services/pricebook-service/internal/domain/forecast/datameta.go:L53` | 私人 wiki 路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/services/pricebook-service/internal/domain/forecast/datameta_test.go:L10` | 私人 wiki 路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/services/pricebook-service/internal/domain/forecast/gate.go:L125` | 私人 wiki 路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/services/pricebook-service/internal/usecase/forecast.go:L481` | 私人 wiki 路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/services/pricebook-service/internal/usecase/forecast_test.go:L1226` | 私人 wiki 路徑 | 移除或改成相對路徑（步驟 6） |
| `backend/services/pricebook-service/testdata/parity/README.md:L96,L107` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/services/pricebook-service/testdata/parity/regenerate/gen_fallback_fixture.py:L34` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/services/pricebook-service/testdata/parity/regenerate/gen_parity_fixture.py:L33` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/services/pricebook-service/testdata/parity/spatial/README.md:L98,L102` | second-brain | 移除或改成相對路徑（步驟 6） |
| `backend/services/pricebook-service/testdata/parity/spatial/regenerate/gen_spatial_fixture.py:L7,L17` | second-brain | 移除或改成相對路徑（步驟 6） |
| `buyer-app/lib/core/observability/sentry_scrub.dart:L13` | 私人 wiki 路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/lib/core/routing/app_router.dart:L19` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/lib/features/category/presentation/category_screen.dart:L35` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/lib/features/product/presentation/search_results_screen.dart:L29` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/lib/features/seller/presentation/widgets/merchant_shop_tab.dart:L28` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/lib/features/settings/presentation/my_subscription_screen.dart:L33` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/core/navigation/single_navbar_ownership_test.dart:L53` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/features/home/banner_popup_dialog_test.dart:L2` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/features/home/home_address_section_test.dart:L2` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/features/home/home_banner_contract_test.dart:L3,L4` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/features/home/home_compact_above_fold_test.dart:L5,L6` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/features/home/home_latest_posts_controller_test.dart:L1,L2` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/features/home/home_latest_posts_section_test.dart:L1,L2` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/features/home/home_product_controller_test.dart:L4,L5,L6` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/features/home/home_product_repository_test.dart:L5` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/features/home/home_product_section_rwd_test.dart:L3,L4` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/features/home/product_add_to_cart_sheet_test.dart:L3` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/features/product/product_purchase_surface_parity_test.dart:L9` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/features/seller/merchant_icon_policy_test.dart:L49` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/integration/hermetic_app_smoke_test.dart:L16` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/integration/hermetic_home_product_flow_test.dart:L14,L15` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/shared/widgets/accessibility_control_inventory_test.dart:L9,L10` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/shared/widgets/bottom_sheet_handle_policy_test.dart:L12,L13` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/shared/widgets/chat_icon_policy_test.dart:L31` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/test/tool/coverage_ratchet_test.dart:L29` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/tool/architecture_check.dart:L8` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/tool/coverage_baseline.json:L102,L103,L104,L105,L106,L107,L108,L109,L110,L111…共11處` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `buyer-app/tool/presentation_io_policy.json:L20` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `seller-app/lib/core/observability/sentry_scrub.dart:L13` | 私人 wiki 路徑 | 移除或改成相對路徑（步驟 6） |
| `seller-app/test/tool/coverage_ratchet_test.dart:L29` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/.impeccable/surfaces/account-messages-assistant-conversationid-page-tsx.md:L6` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/AGENTS.md:L43` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/app/api/public/tab-bootstrap/v1/home/route.ts:L3` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/components/shared/Navbar.tsx:L34` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/debug-storybook.log:L2` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/features/about/AboutPage.tsx:L5` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/features/home/HomePage.tsx:L7,L8,L9,L10,L11,L12` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/features/home/components/HomeCompactMarketStage.tsx:L9,L10,L11` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/features/home/components/HomeCompactMerchants.tsx:L8` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/features/home/components/HomeCompactSearchExperience.tsx:L16` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/features/home/components/HomeMemberMobileSearch.tsx:L3` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/features/home/components/HomePostDetailViewer.tsx:L11` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/features/home/components/HomeProcessCards.tsx:L6` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/features/home/components/public-tab-image-scheduling.unit.test.ts:L16,L19,L40` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/features/home/server/__tests__/home-bootstrap-budget.test.ts:L2` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/features/market/components/MarketProductsList.append.unit.test.ts:L12` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/scripts/check-no-emoji.ts:L25` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/stories/web/InlineLoading.stories.tsx:L11,L12,L13` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/app-icon-system.test.ts:L83` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/app-image-presets.test.ts:L35,L58,L91` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/<舊名-子品牌>-production-visibility.test.ts:L28` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/<舊名-子品牌>-seasonal-integration.test.ts:L9` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/e2e/helpers/preload-hygiene.ts:L10` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/home-chrome.test.ts:L5,L6,L7,L17` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/home-compact-merchants-prefetch-gate.test.ts:L9` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/home-compact-search-lazy.test.ts:L6,L10,L20` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/home-image-fallback-sizing.test.ts:L7,L13` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/home-merchant-prize-icon-lazy.test.ts:L10,L11,L12` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/home-navbar-search.test.ts:L8,L12,L16` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/home-post-detail-viewer-lazy.test.ts:L17,L20,L21,L24,L69,L72,L112` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/home-search-recovery.test.ts:L6` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/popular-search.test.ts:L7` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/post-gif-media-contract.test.ts:L318,L351` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/post-seller-link.test.ts:L31` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/recommended-seller-logo-contract.test.ts:L81` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/seller-cover-layout.test.ts:L111` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/seller-link-canonical.test.ts:L94` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/tab-bootstrap-cache.test.ts:L27` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/tablet-navbar-layout.test.ts:L15` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/tests/tailwind-source-boundary.test.ts:L134` | 本機路徑 | 移除或改成相對路徑（步驟 6） |
| `website/third_party/react-bits/NOTICE.md:L39` | 本機路徑 | 移除或改成相對路徑（步驟 6） |

## 8. 計畫範圍以外的盤點發現

以下不是逐列去向表，是步驟 3 到 6 要處理的事實；沒有表格列，所以不進 `--check-inventory` 的覆蓋檢查。

- 計畫的素材 regex 沒涵蓋字型 woff/woff2（88 個）、影片（36 個）與 Lottie JSON；本檔已納入第 1 節。
- 檔名含舊名的追蹤檔很多（`scan.py --category brand-path`）；字型的檔案名稱與檔內家族名、Lottie 的 `nm` 欄位、App 圖示、圖片像素都含舊名，純文字的 `rename.py` 處理不到，要重新生成或替換（第 1 節）。
- 文件作者本人（Will）的全名出現在一份設計書的作者欄（Q-D1）。
- `scripts/check-legacy-endpoints.py` 與 `legacy-endpoints-baseline.json` 本身含舊名與舊網域，`rename.py` 要跳過，步驟 7 改造。

