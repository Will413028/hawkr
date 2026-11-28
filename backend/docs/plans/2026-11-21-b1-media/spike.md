# B1-0 spike 結論

快照：`b1-media` 分支 `c8870685`（base dev `17bb31c5`），2026-11-21。標記：【量】＝跑過指令或打開檔案；【推】＝推論；【未驗】＝沒驗到，附原因。(b)(c)(d) 由唯讀 subagent 查，主 session 抽查標「抽查」的項目。

## (a) 只用標準庫實作分塊 AEAD（給 B1-D4）

**結論：可行，不需新增依賴【量】。** 拋棄式實作（STREAM 構造：每段 AES-256-GCM，nonce＝隨機前綴＋big-endian uint32 段序號＋1 byte 末段旗標；每檔金鑰由主密鑰經 HKDF-SHA256 導出；header 含版本、key id、salt、段大小）只 import `crypto/aes`、`crypto/cipher`、`crypto/hkdf`、`crypto/rand`、`crypto/sha256`、`encoding/binary`、`errors`、`fmt`、`io`。原始碼不留在 repo（B1-2 重寫正式版）。

```
$ cat go.mod
module spike/streamaead

go 1.27.3
$ go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./...
spike/streamaead
$ go test -race -count=1 ./...      # 主 session 重跑
ok  	spike/streamaead	4.394s
```

測試涵蓋【量】：0／1／段−1／段／段＋1／3 段／10 MiB 的來回；竄改 header、首段、中段、末段都回錯誤，且被竄改段與其後的明文一個位元組都沒送出；段邊界截斷、段中截斷、兩段對調、重複附加末段、尾端雜訊、未知 key id 都回錯誤；加入第二把主密鑰後舊 key id 仍可解；段序號溢位回錯誤（以強制設定序號測）。

吞吐量【量，雜訊大，只當下限】：`GOMAXPROCS=2 go test -run xxx -bench . -benchmem -count 3`，20 MiB，Apple arm64，跑前 `uptime` load average 23.16（共用機器）。

```
BenchmarkEncrypt/seg=64KiB-2      114  10987603 ns/op  1908.65 MB/s   331281 B/op  344 allocs/op
BenchmarkEncrypt/seg=64KiB-2      144  13619334 ns/op  1539.83 MB/s   292940 B/op  344 allocs/op
BenchmarkEncrypt/seg=64KiB-2      133  14317052 ns/op  1464.79 MB/s   304990 B/op  344 allocs/op
BenchmarkEncrypt/seg=1024KiB-2    100  10292558 ns/op  2037.54 MB/s  2318325 B/op   44 allocs/op
BenchmarkEncrypt/seg=1024KiB-2    122  18310858 ns/op  1145.31 MB/s  2280494 B/op   44 allocs/op
BenchmarkEncrypt/seg=1024KiB-2    100  61740907 ns/op   339.67 MB/s  2318325 B/op   44 allocs/op
BenchmarkDecrypt/seg=64KiB-2      100  10089723 ns/op  2078.50 MB/s   221125 B/op  345 allocs/op
BenchmarkDecrypt/seg=64KiB-2      174   8424707 ns/op  2489.29 MB/s   221041 B/op  345 allocs/op
BenchmarkDecrypt/seg=64KiB-2      100  10145787 ns/op  2067.02 MB/s   221125 B/op  345 allocs/op
BenchmarkDecrypt/seg=1024KiB-2    134   8009461 ns/op  2618.34 MB/s  3165560 B/op   45 allocs/op
BenchmarkDecrypt/seg=1024KiB-2    183   7764889 ns/op  2700.81 MB/s  3165376 B/op   45 allocs/op
BenchmarkDecrypt/seg=1024KiB-2    168   8128012 ns/op  2580.15 MB/s  3165474 B/op   45 allocs/op
BenchmarkSingleShot/Seal-2        198   6034158 ns/op  3475.47 MB/s   105958 B/op    0 allocs/op
BenchmarkSingleShot/Seal-2        211   5787065 ns/op  3623.86 MB/s    99429 B/op    0 allocs/op
BenchmarkSingleShot/Seal-2        183   6182414 ns/op  3392.12 MB/s   114643 B/op    0 allocs/op
BenchmarkSingleShot/Open-2        204   7471980 ns/op  2806.69 MB/s   102801 B/op    0 allocs/op
BenchmarkSingleShot/Open-2        217   5865661 ns/op  3575.30 MB/s    96642 B/op    0 allocs/op
BenchmarkSingleShot/Open-2        212   6633022 ns/op  3161.68 MB/s    98922 B/op    0 allocs/op
```

- 最差的一次（加密 1 MiB 段 340 MB/s）仍遠高於上傳檔案的網路與 R2 寫入速度，吞吐量不是選 A 或 B 的限制【推】。這組數字量的是記憶體內加解密，不含 R2 I/O。
- 兩種段大小在雜訊內分不出高下；段大小留給 B1-2 決定。
- 記憶體【量】：延遲產生 200 MiB 密文解到 `io.Discard`，取樣到的 HeapAlloc 最高 1.8 MiB（64 KiB 段）、7.6 MiB（1 MiB 段），與檔案大小無關。每 256 次 read 取樣一次，是取樣值不是真正峰值。
- 給 B1-2 的設計備註：解密端要預讀一段才知道哪段是末段；header 進 HKDF info 或每段 AAD 擇一即可；段大小在 header 內要設上限，避免惡意 header 逼出大配置。
- 沒做：fuzzing、逐位元組短讀測試。

## (b) R2 是否有依前綴的生命週期規則（給 B1-D3）

**結論：repo 內沒有任何 bucket 規則，也沒有任何規則依賴 category 前綴【量】；Cloudflare 帳號裡沒有 hawkr 的媒體 bucket（`cf r2 buckets list` 只有其他專案的 5 個，加上本次建的 sandbox）【量】，所以沒有線上規則要相容。**

- 現在的樹【量】：`*.tf`、wrangler 設定、lifecycle／CORS 的 JSON、shell、`.github/workflows`、compose、Java `application.yml` 都搜不到 lifecycle、CORS 或前綴規則。`backend/workers/image-gateway/wrangler.toml:12-14` 只有 R2 binding；`backend/infra/` 只剩 mq。本 repo 的歷史也沒有：`git log --all --diff-filter=D --name-only -- 'backend/infra/**'` 無輸出。
- 公司時期 repo 的舊設定（已不在本 repo；只當線索）【量，讀自舊 repo 歷史】：dev 與 prod 的 lifecycle 相同——`tmp/` 前綴 30 天過期、整個 bucket 的未完成 multipart 7 天中止；沒有 `public/`、`secure/` 或 category 前綴的規則；CORS 是整個 bucket 一條。
- 過期引用：detailed-design 49 號 `:500` 仍寫 CORS 的唯一來源是 `infra/r2-cors/`，該路徑不存在。
- 對 B1 的含意：暫存前綴沿用 `tmp/`（Java `AbstractStorageService.java:29` 的 `tempPrefix` 預設值），線上若仍有舊規則才對得上；bucket 規則目前不在版控，B1-8「需要人做的事」要包含把 lifecycle 與 CORS 放回版控。

## (c) 在 CI 對 Java media 跑特性測試（給 B1-1、B1-D10）

**結論：可行，不需付費資源、不超過標準 runner 限制（停止條件未觸發）；估 4–6 分鐘【推，沒實際起過服務】。有兩個阻礙要先處理。**

- runner【量】：repo 是 public，workflow 全用 `ubuntu-latest`；concurrency 都是各 workflow 自己的 `<name>-${{ github.ref }}`（只看了 Java 與 monolith 兩個 workflow）。
- 現有 harness【量】：`backend/scripts/characterize.sh:129-163` 以 `go build` 編出單體與 gateway 兩個執行檔後起行程，只需要 `make infra-up` 的 Postgres 與 Redis；請求路徑是 測試 → 單體（dev token）→ gateway → 舊服務。media 目前是 `ServeFakeMedia` 的假 gRPC 服務。
- Java media 啟動需求【量】：Postgres 的 media 資料庫（Flyway 自己遷移）；`TOKEN_SECRET` 或 `TOKEN_ALLOW_EPHEMERAL=true`（都沒有就拒絕啟動，`StagedTokenService.java:67`）；不需要 Redis、RabbitMQ、Nacos（`NACOS_ENABLED=false`）。缺 `ENCRYPTION_MASTER_KEY` 只警告，但機敏加解密不能用。seller 的 `ResolvePrincipals` 可指向單體的 legacy listener。
- gateway【量＋推】：media 路由掛的驗證讀單體注入的身分 header，dev token 由 `hawkr dev-token` 簽，不需要 identity stack【量】。沒有 product、order 等其他 gRPC 後端時 gateway 能否啟動【推】：依據只有 `characterize.sh` 沒設它們。
- 既有 CI 的實測時間【量】：Java 全 reactor 單元測試 99 秒（run 44545769755，Maven cache 熱）、Java integration job 4 分 2 秒；`backend-monolith` run 44917895745 的 `go test -race` 約 34 秒、integration 80 秒、還原 Go 快取 71–73 秒。
- 建議的 job 形狀：Postgres 與 Redis 用 `services:`（不用 compose）；`./mvnw package -pl :media-service -am` 後以 runner 的 JDK 21 直接 `java -jar`（省 docker build）；`scripts/characterize.sh <模組>`；服務清單做成 matrix 或輸入參數，每個舊服務一支 `legacy.sh`，B2–B10 只加項目與情境。這與計畫 B1-1 寫的「compose 子集」不同，理由是既有 harness 本來就起行程。
- **阻礙 1（物件儲存，列成 B1-D12）**：Java 的 R2 endpoint 寫死為 `https://<account>.r2.cloudflarestorage.com`（`R2StorageService.java:49`），沒有 path-style 開關，CI 的 S3 替身接不上。R2 設定不完整時服務照常啟動（`R2StorageService.java:95-107` 只警告）：建立上傳會話回假 URL、完成時跳過 HEAD 回查（`MediaAssetService.java:244`），機敏上傳與預覽會失敗。
- **阻礙 2（B1-1 的停止條件）**：REST driver 只收送 JSON（`internal/httpapi/characterize/rest.go:27-65`），表達不了 multipart 上傳與原始位元組回應；要先加 raw 方法。

## (d) 是否有程式依 object key 的 category 前綴決定行為（給 B1-D3）

**結論：沒有【量】。** 所有解析 key 的地方只分 `public/`／`secure/` 或第一段：

| 位置 | 看什麼 |
|---|---|
| `backend/workers/image-gateway/src/index.ts:35,63` | 路由 `/i/{preset}/{key}`；只放行 `public/` |
| `backend/pkg/mediaurl/url.go:23-34,41-63,73-130` | `i/` 與呼叫端傳入的 `public/` 白名單 |
| `website/lib/media/cdn.ts:88-115`、`admin/src/utils/media.ts:95-97` | 第一段（`public`、`secure`、`tmp`、`i`） |
| `buyer-app/lib/config/media_image.dart:44-76` | 不看 key 片段 |
| Java `R2StorageService.java:304` | `public/` 前綴決定是否剝除 EXIF |

- preset 由呼叫端指定；`image-presets.generated.ts`、`image_presets.g.dart` 沒有 category、`public`、`secure` 字樣（產生器 `gen-client-presets.mjs`）。
- Java 只寫入 category（`AbstractStorageService.java:65-67`），沒有地方讀回，也沒有依前綴列出或刪除物件。
- 【未驗】Cloudflare zone 的 Cache Rules（兩個 repo 都沒有快照）；seller-app 沒有單獨逐檔檢查。
- 順帶：Java 兩個寫入點的 key 形狀不一致——`AbstractStorageService` 有 ownerId 段，`MediaAssetService.java:434,503` 沒有（沒深入查；新實作不沿用）。

## (e) R2 預簽上傳能綁什麼（給 B1-D5）

**結論：預簽 PUT 可以把 `Content-Length`（確切大小）、`Content-Type`、`Content-MD5`、`If-None-Match` 綁進簽章【量，sandbox bucket 實測】；不能綁大小範圍（R2 不支援表單 POST）【量，官方文件】。**

官方文件（2026-11-21 讀取）：

| 問題 | 文件寫的 | 出處 |
|---|---|---|
| 預簽 PUT 能否綁 `Content-Type` | 能：簽章含這個 header，不符回 `403 SignatureDoesNotMatch` | `https://developers.cloudflare.com/r2/api/s3/presigned-urls/` |
| 預簽 PUT 能否綁 `Content-Length` | **沒寫** | 同上 |
| HTML 表單 POST（POST policy、`content-length-range`） | **不支援**：「`POST` (multipart form uploads via HTML forms) is not currently supported.」 | 同上 |
| 可預簽的方法與期限 | `GET`、`HEAD`、`PUT`、`DELETE`；1 秒到 7 天 | 同上 |
| 預簽網址的網域 | 只能用 `<ACCOUNT_ID>.r2.cloudflarestorage.com`，不能用自訂網域 | 同上 |
| `PutObject` 的完整性 header | 支援 `Content-MD5`；`x-amz-checksum-*` 在 PutObject 是否支援沒寫 | `https://developers.cloudflare.com/r2/api/s3/api/` |
| 條件寫入 | `PutObject` 支援 `If-Match`、`If-None-Match`，原子判定 | 同上 |
| 單次 PUT 上限 | 5 GiB | `https://developers.cloudflare.com/r2/objects/upload-objects/` |
| lifecycle | 可依前綴；範例單位是天；過期後「通常 24 小時內」移除；未完成 multipart 預設 7 天中止 | `https://developers.cloudflare.com/r2/buckets/object-lifecycles/` |
| 瀏覽器預簽 PUT 的 CORS | `AllowedMethods` 對應方法、`AllowedHeaders` 列 client 會送的 header、`ExposeHeaders` 可放 `ETag` | `https://developers.cloudflare.com/r2/buckets/cors/` |

實測（2026-11-21）：sandbox bucket `hawkr-sandbox-media`（只限它的 Object Read＋Write token；憑證位置記在本機個人脈絡檔），腳本 `r2_presign_probe.py`（同目錄；自己算 SigV4 query 簽章，payload 是 `UNSIGNED-PAYLOAD`，從環境變數讀 `R2_*`）。

```
$ python3 r2_presign_probe.py
0 control (only host signed), 2000 bytes text/plain: HTTP 200
1a signed len=1000 type=png, send 1000 png: HTTP 200
1b signed len=1000 type=png, send 2000 png: HTTP 403 SignatureDoesNotMatch
1c signed len=1000 type=png, send 999 png: HTTP 403 SignatureDoesNotMatch
1d signed len=1000 type=png, send 1000 text/plain: HTTP 403 SignatureDoesNotMatch
1e signed type=png only, send 5000 png: HTTP 200
  HEAD c0: HTTP 200
  HEAD c1a: HTTP 200
  HEAD c1b: HTTP 404
  HEAD c1c: HTTP 404
  HEAD c1d: HTTP 404
  HEAD c1e: HTTP 200
2a signed if-none-match:*, first PUT: HTTP 200
2b signed if-none-match:*, second PUT same key: HTTP 412 PreconditionFailed
2c signed if-none-match:*, client omits the header: HTTP 403 SignatureDoesNotMatch
3a signed content-md5, matching body: HTTP 200
3b signed content-md5, different body same length: HTTP 400 BadDigest
```

- **大小**：簽了 `content-length` 後，大小多一個或少一個位元組都回 403，而且不落檔（1b、1c 的 HEAD 是 404）。綁的是確切大小，不是上限：client 建立上傳時先報檔案大小，伺服器檢查不超過上限後把它簽進網址。沒簽的 header 不受限（對照組 0、1e）。
- **型別**：與文件一致（1d）。
- **不覆寫**：`If-None-Match: *` 可簽進預簽網址並生效（2a、2b）；client 不帶就 403（2c）。暫存 key 可以做到只寫一次。
- **內容完整性**：`Content-MD5` 可簽，內容不符回 400（3b）。
- token 範圍【量】：對 sandbox bucket 可寫可讀；對同帳號另外兩個 bucket 的 `PutObject`、`ListObjectsV2` 都回 `AccessDenied`。
- lifecycle【量】：`cf r2 buckets lifecycle update hawkr-sandbox-media --force --body '{"rules":[…{"id":"tmp-expire-1d","conditions":{"prefix":"tmp/"},"deleteObjectsTransition":{"condition":{"type":"Age","maxAge":86400}}}]}'` 後讀回一致（sandbox 現在就是這條規則）。API 也收下 `maxAge: 3600`（讀回 3600，之後改回 86400）；小於一天的規則實際多久刪【未驗】，文件只說「通常 24 小時內」。

對 B1-D5 的含意：

- B1-3 驗收的「二選一」可以寫死成**儲存端簽章條件**：預簽 PUT 簽 `content-length`、`content-type`、`if-none-match: *`，超量或型別不符在 R2 就被拒、不產生暫存物件。伺服器端處理仍要依 magic bytes 驗格式（`Content-Type` 是 client 宣告的）。
- 預簽網址只能用 `<ACCOUNT_ID>.r2.cloudflarestorage.com`，瀏覽器直傳要設 bucket CORS（`AllowedHeaders` 至少 `content-type`、`if-none-match`；`content-length` 由瀏覽器自帶）。瀏覽器端的 preflight 沒實測【未驗】，留給 B1-3 的 website e2e。
- 機敏檔明文在暫存區停留的時間上限＝處理 job 的延遲（正常路徑處理完就刪）；處理失敗時由 GC 與 `tmp/` lifecycle（1 天＋最多 24 小時）兜底。這是否可接受屬 D5 已寫的「超出可接受範圍就列回決定題」；我的判斷是可接受（暫存區私有、沒有公開網址），沒有列回【推】。
