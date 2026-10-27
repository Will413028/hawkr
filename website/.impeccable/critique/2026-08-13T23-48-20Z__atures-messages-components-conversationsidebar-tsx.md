---
target: features/messages/components/ConversationSidebar.tsx
total_score: 23
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 4
timestamp: 2026-08-13T23-48-20Z
slug: atures-messages-components-conversationsidebar-tsx
---
# 買家訊息收件匣側欄 — critique

Method: A = 隔離 sub-agent（未被 detector 錨定）· B = 主脈絡
⚠️ 偏離：B 的 detector 結果在 A 回報前進入綜合脈絡。A 本身乾淨，綜合不是。

## Design Health Score：23/40（Acceptable）

| # | 啟發式 | 分 | 關鍵問題 |
|---|---|---|---|
| 1 | 系統狀態可見 | 2 | Agent 分頁無 loading 分支，冷啟動謊稱「還沒有對話」 |
| 2 | 貼近真實世界 | 3 | 分頁數字是總數，使用者預期是未讀 |
| 3 | 使用者控制 | 3 | 復原還原資料不還原處境 |
| 4 | 一致性與標準 | 2 | tablist 只有 ARIA 外殼，無方向鍵 |
| 5 | 錯誤預防 | 2 | 觸控常駐刪除鈕；命中區重疊 8px |
| 6 | 辨識而非回憶 | 3 | 收合軌隱藏分頁但保留 section state |
| 7 | 彈性與效率 | 2 | 無鍵盤切換；50 筆硬上限 |
| 8 | 美感與極簡 | 3 | Agent 在第一列之前出現四次 |
| 9 | 錯誤復原 | 1 | 重試鈕是死碼（兩個 layout 都沒傳 onRefresh） |
| 10 | 說明與文件 | 2 | 「AI」標籤 8px |

## 研究依據

Refero screens 8 · flows 21。21 個 delete-conversation flow 中 5/6（ChatGPT 7036、Meta AI 13356、XChat 11674、BeReal 4937、Sora 7836）有確認步驟，僅 Gizmo 10036 立即刪除。
空狀態語彙參照 Notion Updates 2804cebe、Gmail 9761846e、BoldVoice 25fa1968。
分頁選中態與計數參照 WhatsApp fedfb836。

## detector

91 筆，0 筆落在本次改動行上（git diff --unified=0 行號區間逐筆比對）。全為既有 token 債。detector 未抓到任何一條上列 P1。

## 已修

P1 Agent 分頁載入／降級狀態；P1 觸控收起刪除鈕 + gap 10px；P1 兩個 layout 接上 onRefresh + ↻ 換 IconlyIcon；分頁數字改未讀 + 跨分頁提示；P2 50 筆截斷誠實文案；P2 改名超長提示；P2 分頁 44px 命中區；P2 tablist 方向鍵 + aria-controls；P3 section-label 死碼。

## 未修（需產品決策）

- hero 卡與 Agent 分頁重複（A 建議刪 hero）——surface brief 與 PRODUCT.md 都定義了 hero，屬產品決策。
- hero 的 aria-current 在賣家分頁時仍可能標成 current page。
- section state 未由路由推導：深連結／冷載入到 agent 對話時，側欄仍停在賣家分頁。
- 真正的游標分頁（目前只讓文案誠實，沒有載入更多）。
- 「AI」標籤 8px。
- agent 列不帶需求狀態（澄清中／待詢價／已回報價）。
