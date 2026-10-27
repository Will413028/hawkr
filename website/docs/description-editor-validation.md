# 商品描述編輯器：編輯、文字色盤與圖片

本次修改直接使用正式 `features/seller/components/DescriptionEditor.tsx`，預覽透過同檔匯出的 `DescriptionEditorCanvas` 注入本地 state、上傳 stub 與限定路徑的本地 MSW 圖片 fixture。既有 `ProductDescription` 文件格式與後端欄位維持相容。同次提交也包含共用欄位的 App 外觀與 PostContent 高亮對齊，完整範圍和 SemVer 意圖見 [元件庫交付](./component-library.md#本次-ui-更新)。

## 操作規則

- 常用工具直接顯示，對齊與顏色收在「更多格式」。一鍵整理 Markdown 標題、重複清單符號與空白項目，整理可復原；閱讀預覽保留同一份文件，放大編輯支援 Escape 收合與焦點返回。
- Backspace／Delete 合併段落時保留未選內容；跨內文、標題、清單選取只刪掉所選範圍，保留清單其他項目與行內格式。
- 刪除區塊依穩定 ID 執行。最後一段刪光後仍可繼續輸入，刪除、立即復原、重做使用同一個序列。
- 表格跨格選取清除所選文字，保留首尾未選部分、表格列欄、未選儲存格與對齊。跨表格和一般段落的混合選取會提示分段刪除，避免破壞表格或圖片。
- 文字顏色在工具列「A」開啟：色域、色相、HEX、RGB、品牌常用色。所有調整先保留在色盤草稿，按「套用顏色」才更新文件；取消或 Escape 不改內容。一次套用是一個可復原動作。
- 色盤保留編輯區原選取；Escape 先關色盤並回到觸發按鈕，不關閉放大編輯區。色盤會依視窗與螢幕鍵盤可用高度調整位置及捲動。
- 圖片不用填寫說明。空白區塊可拖放或選擇圖片，完成後保留原圖比例，顯示固定的更換／移除操作。舊圖片的替代文字仍保存在文件中。選取圖片用外框標示，不淡化商品顏色。
- 上傳使用原檔及真實進度 callback；只接受圖片與 10 MiB 以下檔案。超量檔案立即提示改選；網路失敗可在原位置重試。更換失敗保留舊圖，成功替換可一步復原／重做，不沿用舊 imageKey 或 assetId。
- 移除或重建圖片區塊後，晚到的上傳回應不會寫回文件。上傳期間仍可編輯文字；閱讀預覽需等待上傳完成。這是忽略失效回應，不代表取消已送出的 HTTP 請求。
- 刪光後的 `<br>`、不換行空白與空字串都使用同一份空白快照，避免一次刪除佔兩個復原步驟。
- 文字格式支援 CSS 與語意標籤轉換，保存顏色後的粗體、斜體與底線；同樣式使用固定欄位順序，避免復原重建後被誤判成新編輯。

## 覆蓋範圍

Storybook 測試比對實際編輯區、`onChange` 文件及瀏覽器錯誤。刪除案例使用 Vitest Browser 的原生鍵盤；程式只設定選取 Range，不手動刪除 DOM 假造結果。普通預覽保留固定文件供操作，不執行交易。

- `DescriptionEditor.stories.tsx`：日常編輯、空白、窄版、選取格式、Markdown 貼上、清單／標題／對齊、整理與預覽。
- `DescriptionEditorDeletion.stories.tsx`：12 個段落、清單、區塊、混合媒體與快速復原案例。
- `DescriptionEditorTableDeletion.stories.tsx`：單格與跨格刪除，嚴格比對所有儲存格、對齊、前後段落和復原。
- `DescriptionEditorColor.stories.tsx`：5 個局部／完整選取、取消、放大模式、顏色搭配粗斜底線與逐步復原案例。
- `DescriptionEditorImage.stories.tsx`：空白、實際進度、失敗重試、更換及移除復原、上傳途中移除、檔案驗證、閱讀預覽與窄版深色。只攔截 `public/storybook/` 指定 fixture 圖片，真正的上傳 API 不執行。
- `DescriptionColorPicker.stories.tsx`：4 個色域／色相、HEX／RGB 驗證、取消與 Escape、窄版深色／停用案例。

瀏覽器另實看桌面圖片、375px 深色圖片、320px 空白圖片區，以及桌面、375px 與 320px 色盤，並檢查放大模式的 Escape 與焦點返回。這些是本機 Chromium 與 Storybook 證據；未執行正式商品寫入、遠端 CI、部署或 iOS／Android 原生鍵盤測試。

## 2026-09-12 最終本機結果

| 檢查 | 結果 |
| --- | --- |
| MCP `test-run` 完整 Web／基礎回歸，含 a11y | 116 個案例全部通過，內含 42 個編輯器、色盤與圖片案例 |
| 相關 Node unit | 81 個全部通過，含格式保存、快照序列、空白刪除、清單、表格、圖片與 object key |
| `pnpm storybook:typecheck` | 通過 |
| 完整 Node unit（獨立提交工作區） | 2471 通過、1 個既有跳過，沒有失敗 |
| Vitest unit（獨立提交工作區） | 206 個測試檔、1926 個案例通過 |
| 完整 `pnpm lint`（獨立提交工作區） | 通過；保留既有非阻擋 warnings／infos |
| 修改範圍 Biome（CSS Modules parser）／`git diff --check` | 通過 |
| 原生刪除與復原實看 | 圖片／表格間整段刪光只增加 1 步；一次 Ctrl+Z 還原文字，圖片及 4 個儲存格保留，沒有瀏覽器執行錯誤 |
| 圖片 UI 與選檔 | 桌面、375px 深色及 320px 空白圖片區無水平溢出；手機操作按鈕 44px。原生選檔取消無錯誤，重新選圖成功且無說明欄；本地 stub 不連上傳 API |
| 響應式色盤 | 375px 文件寬度 375px；320px 放大模式色盤左右各留 8px，沒有水平溢出 |

這份表格記錄本次來源提交前的本機驗證；同批 Web 回歸包含 Input 與 PostContent stories，但不代表重新執行兩 App 的原生測試。人工元件庫契約仍為 draft，沒有填入本次遠端 CI 或已發布結果。

本次變更進入 dev 後，依 [元件庫維護規範](./ui-library-maintenance.md#dev-自動快照流程) 經中央 Admin CI 的三端完整檢查與 private R2 不可變快照發布，成功更新版本指標後才會反映在管理後台文件入口。Web CI、中央快照 CI、來源 SHA／artifact digest、後台啟用與產品部署分別核對；目前尚待本次推送後的 run 和入口證據，不能由上表推定成功。
