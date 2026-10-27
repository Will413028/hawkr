# 商家治理與食安通報：資料表歸屬、約束形式與執行機制

| 項目 | 值 |
|---|---|
| 狀態 | accepted；決策二、三、七、八 superseded by `2026-10-18-seller-governance-consolidated.md`，決策四改以處分列計算 |
| 日期 | 2026-09-18（決策作成於 2026-05） |
| 影響範圍 | seller-service（治理三表 ＋ cron）、gateway（公開食安端點）、hawkr-admin |

規格本身見
`docs/詳細設計書/32-商家治理與食安通報（Seller Governance & Food Safety）詳細設計書.md`。
本檔只保存「為什麼選它、否決了什麼」。

## 為什麼存在這份 ADR

這批決策原本以 `D1`～`D21` 的代號承載，指向一份名為
`openspec/changes/add-seller-governance-and-food-safety/` 的文件。
**該目錄在本 repo 已不存在**（`git ls-files` 對 `openspec/` 零命中；
對照組：同一支命令抓得到 `docs/superpowers/*` 共 8 檔，證明查詢有效）。

⚠️ 因此本檔記錄的是**從設計書正文還原得出的決策內容**，
不是從原始 OpenSpec 文件複製。原文件裡若另有本檔未涵蓋的決策，已無從查證。

## 決策一：治理三表獨立放 seller-service，不共用內容檢舉的 `reports` 表

內容檢舉（UGC：貼文／留言／訊息／評論／使用者）與商家治理（商家實體＋商品＋訂單）
的 schema 差異大——severity 值域不同、處罰結構不同、透明化政策不同
（前者隱藏內容不公開，後者在賣家頁**公開**違規紀錄）。
硬塞進同一張表會讓兩邊的 CHECK 條件互相牽制。

⇒ 三張表放 seller-service，與 social-service 的 `reports` 分離。

## 決策二：食安通報獨立一張表，不與登入端檢舉共用

食安通報有兩個登入端檢舉沒有的強制條件：**`reporter_phone` 必填**、
**嚴重度固定 `critical`**。共用一張表就得把這兩條寫成條件式 CHECK，
而它們是「這一類通報的定義」而非「某些列的例外」。

## 決策三：penalty 欄位拆解 ＋ DB CHECK enforce 互斥，而不是 JSON blob

六種 `penalty_type` 各自需要不同的欄位組合（`warning` 不得有金額與到期日、
`full_suspension` 必須有到期日、`fine` 必須有金額……）。

| 方案 | 代價 | 判定 |
|---|---|---|
| 一個 `penalty_detail` JSONB | 組合正確性只能靠 application 保證，寫錯不會被擋 | 否決 |
| **欄位拆解 ＋ DB CHECK 互斥** | 新增 penalty 種類要改 CHECK | **採用** |

同樣的形式用在 source FK：`source_seller_report_id` 與
`source_food_safety_report_id` 兩個 nullable 欄位 ＋ CHECK 互斥
（三種來源：admin 主動建、由賣家檢舉轉、由食安通報轉），
而不是一個 `source_type` ＋ 一個多型 id。

## 決策四：`status` 只有兩值，「目前是否生效」即時計算

`seller_violations.status` 只有 `enforced` / `revoked`。
「這筆處罰現在生效嗎」由 `status = 'enforced' AND (expires_at IS NULL OR expires_at > now())`
即時算出，**不另存一個 `expired` 狀態**——存了就需要有人把它推過去，
而那個推的人就是下一個會漏掉的排程。

## 決策五：重複檢舉防呆用 partial unique，不用「24 小時滑動窗」

| 方案 | 代價 | 判定 |
|---|---|---|
| 應用層查「近 24h 有沒有同一筆」 | 兩個並發請求會同時查到「沒有」⇒ race | 否決 |
| **同一日曆日 partial unique**（登入端）／**同 phone＋seller 同 ISO week**（食安端） | 邊界在日／週的切點上，不是嚴格的 24h／7d | **採用** |

race-free 的代價是換一個略微不同的語意，而這個語意對使用者一樣說得通。

## 決策六：公開食安端點用雙層 IP rate limit，不上 captcha

10 分鐘 5 次 ＋ 1 小時 10 次兩層，加上 DB 的 partial unique。
hCaptcha／Turnstile 被否決：它們替公開通報這條**緊急管道**增加一道可能失敗的
第三方依賴，而食安通報恰恰是最不能被擋住的那一條。

## 決策七：自動升級規則用 `pg_advisory_xact_lock` 串行

「近 90 天 medium ≥ 3 筆 → 升 high」這條規則要讀既有筆數再寫入，
並發時兩筆同時讀到 2 就會各自寫成 3。用 advisory lock 以 seller 為粒度串行，
而不是把整張表鎖住。

## 決策八：critical 4h SLA 不上 Discord／SMS／email push

SLA 的計時以「admin ack」為結束點，催辦手段是 **admin UI 的視覺強調
（基礎紅 → 1h 黃 → 3h 紅色閃爍）＋ 30 分鐘排班輪詢**。

push 通道被否決的理由：push 會產生「已讀但沒處理」的假安全感，
而視覺強調是每次打開 admin UI 都會再看到一次的持續訊號。

⚠️ 重新評估條件：出現「admin 不會定時打開 UI」的排班型態時，這個決策要重看。

## 決策九：`sellers.status` 是單欄多來源，用衍生函式而非各路徑寫死

`sellers.status` 同時承載軟刪、admin 手動停權、治理停權三個來源。
優先序 `deleted > suspended > suspended_governance > active` 由
`recomputeSellerStatus(sellerId)` 單一函式衍生。

否決的是「每條路徑自己判斷該寫什麼值」——那會讓治理 cron 在賣家同時有
admin 停權時把狀態清成 `active`，繞過停權。單欄多來源只要有第二個 writer
就會漂移，這是結構性的，不是紀律問題。
