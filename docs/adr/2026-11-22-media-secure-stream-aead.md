# 機敏檔用分塊 AEAD（STREAM）加信封加密，以串流讀取

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-11-21（Will 決定分塊 AEAD）；2026-11-22（Will 決定金鑰改用信封加密） |
| 影響範圍 | `backend/internal/modules/media` 的 secure 寫入、資產表、`ReadSecure`；seller 的 KYC 與治理佐證讀取端點 |
| 執行計畫 | `backend/docs/plans/2026-11-21-b1-media.md`（決定 B1-D4；步驟 B1-2c） |

## 脈絡

機敏檔（公司補充文件、稅籍證明、治理與檢舉佐證）在 R2 上只存 media 以應用層加密的密文；讀取由擁有者端點授權、記存取紀錄後代理內容（ADR `2026-10-03-media-references-owner-side.md` 2026-10-04 修訂）。約束：

- external：檔案上限數 MB 到數十 MB；後台在瀏覽器檢視，讀取是串流。
- 自己的決定（Will 2026-11-22 確認維持）：機敏檔在 R2 上只存應用層密文。平台已不收證件影像（ADR `2026-11-13-seller-platform-review-and-identity-matching.md`），走機敏通道的是公司補充文件、稅籍證明、治理與檢舉佐證、詢問回覆附件；這些內容不受限制，授權書可能帶個人姓名與簽名，而 R2 的伺服器端加密由 Cloudflare 持有金鑰，擋不住拿到 bucket 讀取權限的人。原出處 ADR `2026-07-20-seller-kyc-pii-encryption-scope.md` 已被取代，不再是依據。
- inherited：Java 用單段 AES-GCM，整檔讀進記憶體驗 tag 後回 `byte[]`，沒有 key id，不能換主密鑰（缺陷 5.7）。理由是當初檔案小、只有一把金鑰；兩點今天都不是約束，不沿用。
- inherited：不新增 Go 依賴（根 `AGENTS.md`「依賴與供應鏈」要逐一同意）。標準庫足以實作（計畫附件 `spike.md` (a)：只用 `crypto/*`、`encoding/binary`、`io`，測試與 benchmark 已跑）。

## 考慮過的方案

業界基準有兩層。密文格式：大檔案的 AEAD 用分段構造，每段獨立認證，段序號與末段旗標進 nonce，防止重排與截斷（Hoang 等〈Online Authenticated-Encryption and its Nonce-Reuse Misuse-Resistance〉，<https://eprint.iacr.org/2015/189>；Google Tink Streaming AEAD，<https://developers.google.com/tink/streaming-aead>；age，<https://age-encryption.org/v1>）。金鑰：信封加密，每檔隨機資料金鑰（DEK）由主密鑰（KEK）包裹後與資料分開存放（AWS KMS，<https://docs.aws.amazon.com/kms/latest/developerguide/kms-cryptography.html#enveloping>）。

| 方案 | 代價 | 判定 |
|---|---|---|
| 格式 A 基準：STREAM 分塊 AES-256-GCM，`ReadSecure` 回 `io.ReadCloser` | 自己實作格式要測重排、截斷、末段等邊界（spike 已涵蓋）；解密端要預讀一段判斷末段 | ✓ |
| 格式 B：沿用單段 GCM，讀完驗 tag 再回 `[]byte` | 記憶體隨檔案大小成長；竄改要讀完整檔才發現 | ✗ |
| 格式 C：引入 Tink 的 Go 套件 | 新增依賴與傳遞依賴；格式綁 Tink 的 keyset | ✗（標準庫可行時不加） |
| 格式 D：只用 R2 伺服器端加密 | 金鑰在 Cloudflare，不符合應用層加密的範圍 | ✗ |
| 金鑰 H：每檔金鑰由主密鑰經 HKDF 導出，密文 header 記 key id | 少一個欄位；但退役一把主密鑰（例如懷疑外洩）要把它加密過的所有檔案解密、重新加密、重寫 R2 物件 | ✗ |
| **金鑰 E 基準：每檔隨機 DEK，以 KEK 包裹後存在資產列（含 `kek_id`）；密文 header 只放版本、段大小與 nonce 前綴** | 資產表多一個欄位；讀取多一次解包 | ✓ |

## 決策

格式 A 加金鑰 E。KEK 設定缺少或格式錯誤時單體啟動失敗；執行中仍守「沒有 KEK 就不寫」。換 KEK 時新寫入用新 `kek_id`，背景工作用新 KEK 重新包裹舊資產的 DEK；R2 物件不動。舊 KEK 在所有 DEK 重新包裹完之後才退役。只有 DEK 本身外洩時才需要重新加密那一個檔案。

## 理由

格式：取捨是「自己維護一個分段格式（約數百行加測試）」換「記憶體與檔案大小無關、竄改在送出任何明文之前就失敗、不新增依賴」。B 要先把整檔讀進記憶體才能送出第一個位元組；C 的格式較成熟，但標準庫足夠時，新增依賴的供應鏈代價高於自己測一個公開構造。吞吐量不是限制：最差量到 340 MB/s，遠高於 R2 I/O（`spike.md` (a)，共用機器，只當下限）。

金鑰：取捨是「資產表多一欄、讀取多一次解包」換「輪替與退役主密鑰只改資料庫，不重寫大物件」。H 的好處是密文自描述、少一欄，但主密鑰輪替遲早要退役舊的，那時 H 的成本是重寫所有物件。這一點是起草 ADR 時兩份獨立草稿指出、原拍板沒比較到的，Will 2026-11-22 改採 E。

## 預期結果

- 竄改任一段時，被竄改段與其後的明文一個位元組都不送出（B1-2c integration 驗收）。
- 重新包裹 DEK 後，用新 KEK 能讀舊資產，而且 R2 物件的 ETag 不變（B1-2c 驗收）。
- 讀取記憶體不隨檔案大小成長（spike 量到 200 MiB 時 HeapAlloc 取樣最高 7.6 MiB）。

## Followup

- B1-2c：實作與驗收；資產表加 wrapped DEK 欄位（改寫分支內還沒併入 `dev` 的 media baseline migration）；段大小與 header 段大小上限在實作時定。
- 重新包裹的背景工作：B1-2c 提供函式與測試；營運子命令或 job 的形式在第一次需要輪替時決定。
- B1-8：各環境 KEK 與 `kek_id` 的設定列給 Will。

## 撤銷條件

- 需要跨語言讀取密文（例如其他服務直接解密）：改用有多語言實作的格式（Tink 或 age）。
- 改用雲端 KMS 託管 KEK：只換包裹與解包的實作，格式與資產欄位不變。

## 相關

- 計畫 `backend/docs/plans/2026-11-21-b1-media.md`（`c8870685` 起），附件 `spike.md` (a)。
