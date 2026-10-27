# 媒體引用由擁有者端的同步 job 單一寫入：終點是 media 不認識業務，過渡是精確解除

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will 2026-10-03） |
| 日期 | 2026-10-03 |
| 影響範圍 | `internal/modules/media`（公開 API 與共用的同步演算法）、`internal/legacy/mediaservice`、`internal/modules/buyer`、`internal/modules/seller`、`internal/modules/content`、設計書 08 §4.5–§4.6.2 |
| 計畫 | `docs/plans/2026-09-29-a3-buyer-seller.md` 步驟 8b 審查的 CR2-4；遷移計畫 B1（本 ADR 取代 B1 的「其他模組的圖片轉正與釋放引用，移進業務列的 transaction，取代 media.Media 的順序契約」一行） |

## 脈絡

六個媒體擁有者（買家頭像、店家 logo 與封面、站外業者圖片、KYC 證件、治理佐證、content 的 banner 與食譜）都用 `ReleaseReferences(ref_type, ref_ids, keep)`：「這個 ref 下 keep 以外的引用全刪」（Java 是一條 `DELETE … NOT IN keep`）。`keep` 是呼叫當下的快照：

- 寫入路徑是 transaction 外 `FinalizeAndLink`（轉正加登記引用），之後才寫業務列。已登記、還沒 commit 的新圖不在快照裡。
- 五個收斂 job 在執行時讀已 commit 的列；content 在 commit 後同步呼叫，keep 是這次新轉正的圖。任何一個在轉正與 commit 之間執行（8b 審查 CR2-4），就刪掉新圖的引用。
- media GC 每天 03:00 刪除「沒有任何引用、而且建立滿 7 天」的資產：被誤釋放的舊資產隔天凌晨就被實體刪除，新資產在建立滿 7 天後被刪除。業務列指著的圖因此消失，事後無跡可循。
- etag 擋不住：第二個請求帶的是第一個請求 commit 之後的 etag，是合法的序列寫入。
- content 的 CARRYOVER 曾以同樣的理由否決 job（job 延遲拉長競態窗口），7a 改用 job 時沒有比較這一點。

約束逐條標來源：

- 外部：Java media-service 在 B1 之前不改。改它的 API 或 schema 是白工：B1 把整個 media 改寫成 Go。這是現在的約束，B1 開工時消失。
- 外部：Java 的 `ReleaseReferences` 是一條遠端 SQL。Go 這一側的鎖與逾時序列化不了它的執行時間：逾時之後才在遠端執行的 DELETE 照樣會生效（grpc-java 收到取消不會中止已發出的 JDBC statement）。
- 外部：`LinkImages`、`UnlinkImages` 已經存在，而且冪等（Link 是 `INSERT … ON CONFLICT DO NOTHING`，Unlink 只刪列出的 asset）；Link 不比對上傳者。轉正（Finalize）本身不建立引用，ref_type 在建立資產時就寫上。
- 外部：GC 的寬限是 7 天，從資產的 `created_at` 起算。轉正了卻沒有引用的資產（寫入失敗留下的）因此自然回收。
- 繼承：外部呼叫不在 DB transaction 內（`backend/AGENTS.md` 單體規則 4、5，buyer、seller、content 的 AGENTS 與六處 `必須在 transaction 外開始` 的守衛）；現在仍成立的理由：媒體呼叫背後是 R2 與一個會逾時的 Java 服務，交易內等它會佔住只有 10 條的連線池（`DB_MAX_CONNS` 預設值），慢一點就停掉整個單體。
- 繼承：每個模組擁有自己的 schema，不跨 schema 讀寫（單體規則 2）；演算法可以共用，資料表各自一份。

## 考慮過的方案

基準 1（引用和業務列同一個 transaction）：Rails Active Storage 的 attachment 列與業務列一起 commit，被換掉的 blob 在 commit 後 `purge_later`，從未掛上的上傳由寬限期掃除（`unattached.where(created_at < 2.days.ago)`）。沒有不同步的窗口。
基準 2（引用在另一個系統）：要在「刪引用」與「並發加引用」之間建立先後關係，做法是互斥、條件式刪除（世代號）或帶寬限期的 mark-and-sweep（Git 的 `gc.pruneExpire`）。「拿快照推導其餘全刪」又沒有 fencing，一定會競爭。

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準 1 的終點：media 只存資產與生命週期（暫存 → 已認領 → 刪除）、用途與可見性；引用只存在擁有者的資料表（業務列本來就存 asset_id）；刪除是擁有者以 outbox 發出的「刪除 asset X」；GC 只過期未認領的暫存上傳；機敏下載由擁有者授權後向 media 要短效簽章網址 | B1 一次做完；現在不能做，因為 Java media 還在 | ✓ 終點，B1 開工時實作 |
| **J（過渡）**：引用只由每個 ref 序列化的同步 job 寫入；寫入端只 Finalize；被換掉的資產記成退場；job Link 目前的、精確 Unlink 退場的 | 每個模組一組表（版本與退場）與一個 job；新資產要在 7 天寬限內被 Link | ✓ |
| A 現狀：收斂 job 對齊快照，沒有 fencing | 會實體刪圖 | ✗ |
| C 精確解除，但延遲到 staged token 的有效期之後再執行 | 延遲是租約，沒有強制的上界：程式裡沒有 request deadline 涵蓋 job 進入點；KYC 依序轉正多張；Go 與 Java 是兩個時鐘 | ✗ |
| F 擁有者列鎖互斥：登記引用移進業務 transaction，收斂也在 transaction 內取同一把鎖 | 鎖只序列化 Go 這一側，擋不住逾時之後才在遠端執行的 DELETE；交易內呼叫外部服務違反單體規則 4、5，並佔連線池（登記最多約 16 秒，吃完一個請求的 15 秒期限）；新建實體（通報的新 UUID、第一次 KYC 存檔、新 banner）在 commit 前沒有列可鎖，鎖父列會讓匿名通報持有賣家的店家列鎖，鎖帳號則擴散到 identity | ✗ |
| G outbox 檢查：解除指令放 outbox，寫入者在 transaction 內取消對自己要寫的 asset 還沒執行的指令，遇到執行中或完成時間晚於轉正的就中止 | 多一張指令表、一套狀態機與中止路徑；以 DB 時鐘比較完成時間；「keep 以外全刪」若保留，同樣擋不住遠端遲到的 DELETE | ✗（機制比 J 多，沒有更強） |
| H Java 條件式刪除：引用帶 `linked_at`，釋放帶 `linked_before` | 要改 Java 的 schema、RPC 與 GC，B1 重寫時全部丟掉 | ✗（白工，約束 REAL） |
| E1 等 B1 | 修好之前一直有這個競態；違反「不留技術債」 | ✗ |
| E2 把 B1 提前 | 上傳、機敏下載、GC、R2 整個 media 現在改寫，打亂階段 A→B 的順序 | ✗ |

## 決策

**終點（B1 開工時）**：media 是 business-agnostic。它只存資產與其生命週期、用途與可見性。引用只存在擁有者的資料表。刪除是擁有者以 outbox 發出的明確指令，GC 只過期未認領的暫存上傳。機敏下載由擁有者授權、向 media 要短效簽章網址。`asset_reference` 與 media 端的 `(ref_type, ref_id)` 一併消失；同步 job 與退場表也一起消失。

**過渡（Java media 不變）**：在現有 Java API 上實作 J。

- 寫入端：transaction 外只做 `Finalize`（轉正，不建立引用）。transaction 內寫業務列、把被換掉的資產記成退場（每個擁有模組的 `retired_assets`）、遞增這個 ref 的 `media_ref_sync.seq` 並排入同步 job。已退場的資產不能寫回同一個 ref：同一個 staged token 在有效期內重新轉正會回到同一個資產，寫入端拒絕（InvalidInput）；保留名單只接受目前的資產。
- 同步 job（`<模組>.media_sync`）：以 (ref_type, ref_id) 為唯一鍵（涵蓋排隊中、執行中、等重試與排程中，River 規定執行中不能排除），所以同一個 ref 的同步不會並行。執行期間擁有者又改變了狀態不靠重新排入（它會被去重），而靠結算：先讀 seq，再讀退場資產、最後讀業務列目前的資產（這個順序讓並行的寫入只造成「版本偏舊、多跑一輪」，不會產生假的重疊），Link 目前的、Unlink 精確列出的退場資產，最後在同一個 transaction 內以「seq 仍等於讀到的值」為條件的 UPDATE 記下已同步，條件成立而且沒有等待第二次解除的資產時，在這個 transaction 內以 `river.JobCompleteTx` 完成 job；0 列就再跑一輪。鎖序：寫入端先鎖版本列（`BumpRefSeq`）、再插入 river_job（唯一索引）；結算同一個順序，先 UPDATE 版本列、才碰 river_job，所以不會互鎖，而且不會有「結算已 commit、job 還是 running」的空檔（那段期間寫入端排入的新 job 會被唯一鍵略過，更新就遺失）。寫入端排入的 job 若遇到排程中（第二次解除的 snooze）的 job 也會被去重，所以更新最晚在 HealDelay 內被處理。這個 job 是這個 ref 引用的唯一寫入者。
- 保底掃描（`<模組>.media_sweep`，啟動即執行、每 15 分鐘）：補排 `synced_seq < seq` 而且閒置超過 10 分鐘的 ref（job 遺失、被丟棄或重試用盡都會被補上，排入有唯一性，重複補排安全），並清除過期的退場紀錄；這是 payuni 同步已有的同一種保底。`media_ref_sync` 的列不刪除：刪掉再建會讓 seq 從頭算，同步讀到的舊 seq 可能剛好相等而誤判為已同步；讀取不寫入。
- 終態：店家關閉（logo 與封面）、KYC 申請取消、個資清理（證件列刪除）、買家關閉、content 的 banner 與食譜刪除、站外業者封存與合併（圖片欄位在這些動作裡就被清成 NULL，而且還原不會把圖帶回來），把目前的資產全部記成退場並排入同步；沒有列、已關閉或已取消的 ref 沒有目前的資產。沒有終態路徑的 ref（治理佐證寫入後不可改、不可刪）沒有退場。
- 刪除所有補償路徑（`compensate`、content 在 commit 後的同步釋放）：轉正了卻沒被使用的資產沒有引用，由 GC 在建立滿 7 天後回收，這正是基準 1 對「上傳了但沒掛上」的做法。
- 為什麼不靠時間：同一個 ref 只有一個引用寫入者，沒有「Link 與 Release 並發」；遲到的 Unlink 只會刪掉退場的資產，而退場的資產不會被寫回；遲到的 Link 最多讓退場的資產多留一份引用（方向是漏，不是刪）。這包括 Java 側逾時之後才執行的 RPC。
- 退場紀錄的保留：保留到「已經解除、第二次解除也做過（第一次解除成功之後至少 15 秒，也就是一次呼叫的逾時 5 秒加遠端排隊的餘裕 10 秒，用來收掉逾時之後才在遠端生效的 LinkImages），而且退場滿 2 小時」才清除（由保底掃描清除，整個模組範圍）。2 小時是 staged token 最長有效期（機敏上傳完成後的預覽期限 1 小時）的兩倍；過了有效期，同一個 token 不可能再轉正回同一個資產，寫回檢查就不再需要。每次同步都會再解除所有還在保留期內的退場資產，所以晚到的 Link 被下一次同步修復。Link 與 Unlink 都是單次呼叫加 5 秒逾時、沒有內層重試（唯一的呼叫者是 River job，重試由 job 負責）；一次同步的 job 逾時 30 秒（Link 加 Unlink 各 5 秒，加資料庫與餘裕）。

## 後果

- **唯一依賴時間的地方**：新資產要在 media GC 的 7 天寬限（從資產的 `created_at` 起算）內被同步 job Link。同步由 River 重試（`MaxAttempts` 25，退避為 attempt^4 秒、最長 1 小時（`MaxBackoff`），media 恢復後最晚一小時內就會 Link；25 次約橫跨 19 小時），重試用盡或 job 遺失由保底掃描補排新的 job，每次失敗記 error log「media 引用同步失敗，會重試」（帶 ref_type、ref_id、attempt）。監控訊號在擁有者這一邊，不依賴 River 的 job 列（River 預設 7 天後刪除已丟棄的 job）：各模組 `media_ref_sync` 中 `synced_seq < seq` 的最舊 `updated_at` 的年齡；超過 24 小時，保底掃描記 error log「media 引用同步落後過久」。告警本身屬單體監控（CD 待辦）。
- 殘餘：第二次解除之後才在遠端生效的逾時 LinkImages（遲到超過約 15 秒）會留下一份退場資產的引用：方向是漏，不會誤刪，資產永遠不被 GC。這是 Java media 沒有條件式刪除（方案 H）換來的代價，B1 之後消失。目前指著的資產同時是退場資產（不變式被破壞）時，同步仍然 Link 目前的、略過這些資產的解除、記 error log 並完成，不讓新圖在寬限內一直沒有引用。
- content 的 banner 與食譜的 asset ID 欄位與 key 成對、`NOT NULL`（圖片必填）；不 backfill，本機有舊列的開發資料庫要重灌（上線前沒有相容性約束）。banner 的預覽與彈窗共用同一個 ref，同一個資產可以同時在兩個欄位：退場是「寫入前這個 ref 的所有資產減去寫入後的所有資產」，不逐欄位比對。
- 每個擁有模組各一份薄的 SQL 層（版本表、退場表與查詢，各自的 schema）與 job 外殼；seq 協定、拒絕寫回、結算、Link 與 Unlink 的演算法和保底掃描只有一份，在 media 模組的 `internal/refsync`（`media.RefSyncer`、`media.SyncChanged`、`media.SyncReject`），模組以 `SyncQueries` 介面提供自己的 SQL。測試用的記憶體內 Fake（`media.NewFake`）模擬 `UNIQUE(asset, ref_type, ref_id)` 加 ON CONFLICT DO NOTHING、精確 Unlink，並能讓逾時之後才生效的 RPC 延後執行。
- 站外業者封存：圖片欄位在封存時就清成 NULL（`prospects_images_check`），還原不會把圖帶回來，所以封存把兩張圖記成退場，還原之後換圖仍會登記目前的那一張，退場的圖不會回來。
- 重新評估的條件：B1 開工（改成終點）；或保底掃描的落後訊號在寬限內出現過（先檢查原因，再決定是否縮短掃描週期或閒置門檻）。

## 修訂：機敏下載的終點是擁有者端點代理（2026-10-04，Will 決定）

本節修訂上方終點的「機敏下載由擁有者授權、向 media 要短效簽章網址」；原決策段保留歷史。

**脈絡**：8f 的後台 KYC 證件檢視採 just-in-time 存取：擁有者授權、綁定用途、限時、每次讀取都留紀錄（業界基準：CloudTrail S3 data events 逐物件記錄讀取、GCP Access Transparency、Azure PIM）。簽章網址在到期前可以重複取用，只留得下簽發紀錄，不是讀取紀錄；R2 上的機敏檔又是 media 自己加密的密文，簽章網址拿到的是密文。

**決策**：機敏下載由擁有者的端點授權、先記存取紀錄再代理內容。擁有者呼叫 `media.Media.ReadSecure`；B1 只把這個 port 後面的 adapter 從 Java 的 `ServerReadSecureAsset` 換成單體內的 media，端點、授權與存取紀錄不變。端點的路徑與傳輸規則在〈API 設計規範（Connect）〉「檔案回應」。
