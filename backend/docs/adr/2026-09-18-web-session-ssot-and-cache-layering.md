# Web session 的 SSOT 與快取分層

| 項目 | 值 |
|---|---|
| 狀態 | superseded by `2026-09-27-identity-opaque-sessions.md`（單體裡驗證憑證與 session 的資料在同一個 process，三個決策的前提都不成立）。遷移計畫 A2 落地前，現行程式仍依本檔運作 |
| 日期 | 2026-09-18 |
| 影響範圍 | gateway（`JWTAuth` / `resolveIdentity`）、account-service（`web_sessions`）、Redis |

## 脈絡

web 憑證是 gateway 持有的 opaque session。最初 Redis 本身就是 session 的權威：
gateway 另存一份 30 秒 grace 的本機 last-known-good 正向副本，Redis blip 時由它頂住。
2026-07-16 prod 一次 23 分鐘 Redis outage 證明這個分層站不住——那段時間沒有任何一層
答得出 session，已登入使用者全數 503。

之後把權威搬到 account-service 的 `web_sessions` row，Redis 降為加速層。
本 ADR 記錄由此衍生的三個決策，它們共用同一個前提：
**只有 `web_sessions` row 能證明 session 存在；快取只能加速，不能改變判決。**

規格本身見 `docs/詳細設計書/17-登入與帳號建立流程（Login & Account）詳細設計書.md` §3。

## 決策一：gateway 不保留 session 的本機正向副本，也不做跨 replica 撤銷事件

prod gateway 有 2 個 replica。撤銷發生在其中一個 pod，另一個 pod 手上可能還握著這枚
session 的正向副本。

| 方案 | 代價 | 判定 |
|---|---|---|
| 跨 pod 廣播 `SESSION_REVOKED`（Redis pub/sub） | 廣播管道與故障元件是同一個 Redis；投遞是 best-effort（subscriber 重啟、漏收），永遠不能當正確性保證——保證仍然是 TTL。加了它只會讓人以為洞關了 | 否決 |
| 縮短正向副本的 grace | 只是把曝險除以一個常數，洞還在，同時削弱 Redis blip 的吸收能力 | 否決 |
| **不留正向副本** | 撤銷延遲＝本機正向快取 TTL，所以最好的 TTL 是不存在 | **採用** |

正向副本原本的正當性來自「Redis 是權威，blip 等於沒有人答得出來，last-known-good 勝過
503」。權威搬進 `web_sessions` 之後這個前提就不成立了：DB 在 Redis 故障期間答得出來，
本機副本只可能與它相矛盾。

負向 entry（「SSOT 已說查無」）保留：它只能拒絕、不能放行，方向本身安全。

## 決策二：Redis session key 取 `sha256(token)`，且不做 legacy 雙讀遷移層

| 面向 | `session:<原始 token>` | `session:<sha256(token)>` |
|---|---|---|
| 撤銷方算不算得出鍵 | 算不出。`web_sessions` 只存 `token_hash`，原始 token 從不到達 account-service——它刪得掉 row，卻叫不出該 row 對應的快取鍵。唯一替代是 `SCAN session:*`（O(keyspace) 且有 race） | 算得出。刪 row 時 `RETURNING token_hash`，`session:`+hash 就是這些 row 自己的快取鍵 |
| keyspace 洩漏什麼 | 可直接冒用的 bearer secret——讀得到 keyspace 即等於帳號接管 | 只有 hash，背後已無可竊之物 |
| 成本 | — | 零：同一個 sha256 每個請求本來就為負向快取鍵算過一次 |

**刻意不做 legacy 雙讀遷移層。**「一次 DEL 新舊兩把鍵」或「讀不到新鍵就回頭讀舊鍵、命中後
改名」這類相容層只對**登出**成立（那條路徑手上有原始 token）；對**帳號級撤銷**不成立：
account-service 算得出 `session:<hash>`、永遠算不出 `session:<原始 token>`，精準撤銷刪掉
hash 鍵之後舊鍵仍在，下一次讀取的相容層又會把它接回新鍵——**已撤銷的 session 復活**。

改採既有的讀穿 SSOT 回填：舊鍵一律 miss → 讀 `web_sessions` → 以新鍵回填，切換當下的
使用者不會被登出，殘留舊鍵由自身 TTL 自然消滅。

## 決策三：撤銷動作 fail-closed，包含登入輪替

「Redis 掛也要能登入」的保證針對的是**快取**（降級 mint）。輪替撤銷失敗這條分支在 Redis
outage 下走不到——它需要 account-service 已回答 OAuth RPC（服務活著）卻拒絕刪除。
此時使用者仍以舊 session 登入中，代價是重試而非鎖死；且 `DeleteWebSession` 冪等
（row 不存在＝刪 0 列＝成功），死 cookie 不會把人擋在登入外。

反之若 fail-open，新 cookie 覆蓋舊 cookie 後，舊 session 會變成「沒人持有 token、無法撤銷、
活到 `expires_at`」的影子會話。

## 後果

- 每個 Redis 查無或錯誤都會讀穿 account-service，因此讀穿路徑必須有負向快取與逾時上限，
  否則帶著死 cookie 的流量會直接變成 DB QPS。
- 兩層都答不出來時回 503 而非猜測；503 不是登出，cookie 不清。
- 重新評估條件：若 gateway 改為單 replica，或 session 權威再次搬家，決策一與決策二的前提
  都要重新檢查。
