# CodeMeta 退場：共享代碼值依用途交給契約、模組與呈現層

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-25 |
| 影響範圍 | `docs/代碼管理（CodeMeta）/` 與生成器、`internal/platform/codemeta`、`api/`、各模組的 schema 與傳輸層、四個 client 的 CodeMeta 生成物；〈API 設計規範（Connect）〉§四、〈PostgreSQL 設計規範〉§五、根目錄與 `backend/` 的 `AGENTS.md`、遷移計畫 |
| 上層決策 | `2026-09-24-go-modular-monolith.md` D2、D3、D8；Connect 試點計畫 |
| 相關 | `2026-09-25-api-error-model.md`（錯誤原因的形式） |

## 脈絡

CodeMeta 是公司時期為「22 個服務、Java 與 Go、五個 repo、沒有共同的 IDL」設計的：四份 Markdown（206 個值域、373 個錯誤碼）是唯一來源，1,699 行的 `generate.sh` 輸出 Java、Go、Dart、TS，前端靠人工拷貝加漂移檢查。同一張表混了四種值，讀者與變更的原因都不同：

| 種類 | 例子 | 誰讀 | 為什麼會改 |
|---|---|---|---|
| 對外看得到的列舉 | `OrderStatus`、`AppType` | 四個 client | 功能、API 設計 |
| 儲存的代碼 | `orders.status` 存的 Value | 擁有那張表的模組 | schema 設計 |
| 顯示文字 | Label「待付款」 | 使用者 | UX 文案 |
| 錯誤碼與訊息 | `VERSION_CONFLICT` | client 的分支、使用者 | 錯誤處理、UX 文案 |

現況（2026-09-25 查證）：

- 漂移檢查在 monorepo 失效：`check-copies.sh` 找的是舊的 sibling repo，4 項全部 SKIP、exit 0。四份 client 拷貝已經比 Markdown 多出值（`ShippingStatus` 994、`OrderCancelReason` 9105、`UBER_CANCEL_RECONCILIATION_REQUIRED`、整個 `DeliveryMethodUnavailableReason`），照 README 重新生成再拷貝，會刪掉 client 正在用的值。
- 單體實際只用到 `AccountType`、`AppType`、`AppPlatform` 與通用的錯誤原因；content 的 schema 已另外手寫 `CHECK (app_type IN ('BUYER','SELLER'))`，是 CodeMeta 值域的第二份。
- client 沒有錯誤原因的常數：生成器的錯誤碼只輸出 Java 與 Go，試點通過條件 4「對應到 CodeMeta 生成的常數」照現況做不到。
- proto 以 `string` 帶 CodeMeta 的 Value（Connect 規範 §四），生成的 client 只看到 `string`，上線後 `buf breaking` 也看不到值被刪或改名。

約束：

- 外部：系統從未上線，值的表示法與編號都可以改（根目錄 `AGENTS.md`）。
- 外部：後端與四個 client 在同一個 repo，可以在同一個 commit 改。
- 外部（上線後）：舊版 App 會收到它不認得的新值。
- 外部：第三方定義的代碼（PayUni、Uber、黑貓的狀態碼）不能改名或改號。
- 繼承：模組擁有自己的規則與 schema（monolith D2、D3）。仍成立：已 accepted、正在執行。
- 繼承：舊服務並存（D8）：Java 服務、還沒搬的 Go 服務與 gateway 的 REST，在各自模組搬遷前仍用 CodeMeta 的 Value。仍成立到階段 C。
- 繼承：buf lint 用 STANDARD：列舉值帶型別名前綴、零值是 `_UNSPECIFIED`。仍成立（錯誤原因的例外見錯誤模型 ADR）。
- 繼承：只有 zh-TW 一個語系。仍成立：產品範圍沒變。
- 拿掉，前提（多服務、多語言、多 repo）都已不存在：「CodeMeta 是所有共享值的唯一來源」、「前端在不同 repo，只能人工拷貝」、「外部契約傳數字的 Value」、「錯誤文案集中在錯誤碼.md」、〈PostgreSQL 設計規範〉§五的「CodeMeta 欄位只由應用層驗證、不綁 migration」。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| **基準：依用途分給各自的擁有者**。對外可見的值域在 IDL 以 enum 定義（AIP-126）；錯誤原因的值域也在 IDL 定義（googleapis 的 `google/api/error_reason.proto`）；儲存的表示法由擁有資料的服務決定，不與 API 型別共用（protobuf Best Practices：Use Different Messages For RPC APIs and Storage）；顯示文字屬於 client 的呈現層（Flutter internationalization、next-intl） | 對外可見又要儲存的值域寫在 proto、Go、CHECK 三處，靠測試綁住；每個列舉一個轉換；admin 要自己擁有約 99 個值域的顯示文字；Dart 生成的 enum 是 class，沒有編譯期的窮舉檢查 | **採用** |
| A. 保留 CodeMeta，proto 用 `string` 引用，只把拷貝改成直接生成進 monorepo | 值域不在契約裡：client 只有 `string`，`buf breaking` 看不到刪改；兩套生成器；中央的表記錄各模組的欄位，與 D2、D3 的擁有權方向相反 | 否決，過渡期保留給舊契約 |
| B. 由 CodeMeta 生成 proto enum | 兩段生成；proto 的 `reserved`、`deprecated`、所屬 package 要在 Markdown 另外發明語法；擁有權仍集中 | 否決 |
| C. proto 管全部：顯示文字寫在 enum value 的 custom option，自寫外掛生成各端 | UX 文案進了契約，改一個字就是契約變更；要自己維護三種語言的外掛；同一個值在買家、賣家、後台的文字本來就不同 | 否決 |
| D. CodeMeta 縮成以 proto 值名為 key 的 label 目錄 | proto 與 Markdown 兩邊都要改；自訂生成器仍在 | 否決；label 漂移成為實際問題時的後備 |
| E. 資料庫代碼表加查詢 API | 這些值會驅動程式分支，需要編譯期常數；文字變成執行期依賴 | 否決 |

## 決策

CodeMeta 凍結，只服務還沒搬走的舊契約，隨模組搬遷逐項退場，階段 C 整個刪除。

- **D1 對外可見、client 會分支的封閉集合**：proto enum，放在擁有它的模組的 package（只有後台用的放 `hawkr.admin.*`；兩個以上模組用到才放 `common`）。零值 `<ENUM>_UNSPECIFIED`，值帶前綴。client 遇到 `UNSPECIFIED` 或不認得的值，一律走「未知」的呈現，不當成錯誤。第一個：content 的 `app_type`、`platform` 改成 `AppType`、`AppPlatform`。
- **D2 開放集合**（第三方的原生碼、分類、常變的選項）：不做成 enum，依 AIP-126 用有文件說明的 `string`，或做成可查詢的資源。第三方的原值另存欄位，由 adapter 對應到自己的值域。
- **D3 儲存的代碼**：擁有資料的模組以 Go 具名字串型別加常數定義；DB 欄位是 `text` 加 CHECK。**DB 的值＝Go 常數的值＝proto 值名去掉型別前綴**（UPPER_SNAKE），不再分配數字號段。兩道測試：CHECK 的值集合等於 Go 的 `Values()`；proto 與 Go 的轉換雙向完整（刻意不對外的值列白名單）。模組內部的技術碼留在模組的 `internal/`。不用 PostgreSQL 的 native enum：已有的值刪不掉、不能重排。
- **D4 錯誤原因**：proto 的 `ErrorReason` 列舉，形式與規則在 `2026-09-25-api-error-model.md`。
- **D5 顯示文字**：client 的呈現層，以生成的 enum 為 key。TS 用 `Record<Enum, string>`，少一個值就編譯失敗；Dart 用 map 加一支走訪 `values` 的測試。兩支 App 共用的放手寫的共用 Dart 套件，不放進生成目錄。伺服器產生的文字（推播、PDF、錯誤訊息）由產生它的模組擁有。
- **D6 值的說明**（Markdown 的 Description）：搬到模組 Go 常數的註解與詳細設計書；proto 的註解只寫 client 需要知道的語意。

## 過渡

0. **立即，不等試點結論**：對帳現存的漂移（上述 4 項依設計書 06、65、66 決定補進 Markdown 或從 client 刪除）；`--web`、`--admin`、`--flutter` 改成直接寫進 monorepo 的四個路徑，CI 檢查 drift；刪除 `check-copies.sh` 與相關的 sibling repo 設定。
1. **凍結**：CodeMeta 不再新增值域。每個值域標 `擁有者`（預設 CodeMeta）；擁有者不是 CodeMeta 的，Markdown 那一段只是舊契約的投影，由測試保證與擁有者一致，不得單獨修改。`internal/platform/codemeta` 的 import 以 arch test 的名單凍結、只准減少。`AccountType` 要解析 gateway 傳來的 `X-Account-Type`，留在 platform 到 identity 搬進單體為止。
2. **content，在試點步驟 6 之前**：`AppType`、`AppPlatform` 照 D1、D3；`VERSION_CONFLICT` 移到 `ErrorReason`。
3. **每個模組搬遷時**（寫進遷移計畫的「模組完成的定義」）：逐一決定它的值域與錯誤碼的去向（proto enum、模組的 Go 型別、client 的呈現層、刪除），結論寫進沿用盤點；client 的那個模組換成生成的 client 時，把顯示文字搬進呈現層。
4. **階段 C**：沒有任何值域的擁有者是 CodeMeta、也沒有舊的消費端時，刪除 CodeMeta 目錄、生成器、`internal/platform/codemeta`、`pkg/codemeta`、`pkg/errors/bizcode`、gateway 的鏡像，以及各 client 的 CodeMeta 生成物。

## 理由

- **共享的需求已經變了。** monolith D1（後端只剩 Go）、D6（沒有 MQ）之後，跨語言的共享只剩「後端與四個 client」，這正是對外 IDL 的工作；buf 已經為四端生成程式、檢查 breaking 與 drift。值域放進契約，client 拿到的是型別而不是字串。
- **四種值的擁有者與變更流程不同。** 上線後，對外列舉受 `buf breaking` 管，儲存的代碼受 migration 管，顯示文字要經過 UX 同意。放在同一張表，「前端可見」、`ADMIN_ENUMS`、`GO_LABEL_ENUMS` 這些 allowlist 就是為了把它們再拆開而長出來的。
- **取捨：放棄「一份表生成全部」，換成「各自擁有、由測試綁住」。** 可以接受，因為 CodeMeta 實際上也沒做到前者：CHECK 已有第二份、client 有手寫的對照、拷貝已經漂移。新的測試比對的是實際生效的東西（DB 的約束、proto 的 descriptor），不是另一份拷貝。
- **不選 C**：顯示文字是 UX，不該走契約審查；同一個值在不同端的文字本來就不同。
- **DB 的值＝常數＝proto 值名去前綴**：一個名稱貫穿三層，轉換只是去前綴，直接查 DB 也看得懂。

## 後果

- **接受的代價**
  - 新增一個對外可見的值要改 proto、Go、CHECK、client 的呈現層，靠測試綁住。
  - Dart 失去編譯期的窮舉（例如 buyer-app 的 `shippingPresentationOf`），改用測試補。
  - wire 上的值從數字字串變成列舉名稱，每個 client 的呼叫點都要改（隨 Connect 遷移本來就要改）。
  - 舊版 App 收到新值時，JSON 解析得到 `UNSPECIFIED`、原值丟失：connectrpc Dart 的 `JsonCodec` 預設忽略未知的值，不會整個回應失敗。
- **要改的文件**：根目錄 `AGENTS.md` 的契約表；`backend/AGENTS.md` 的 CodeMeta 規則；〈API 設計規範（Connect）〉§四；〈PostgreSQL 設計規範〉§五；CodeMeta README（凍結與擁有者屬性）；遷移計畫的 0.2、「模組完成的定義」、階段 C；試點通過條件 4。
- **重新評估的條件**：需要第二個語系（顯示文字改成共用的翻譯目錄或翻譯管理工具）；轉換與 Go 型別的樣板量明顯拖慢搬遷（評估由 proto 生成模組的 Go 型別）；API 開放給第三方（重新檢查哪些值域應改成 `string`）；Connect 試點不通過（列舉改用 OpenAPI 的 `enum`，其餘決定不變）。

## 來源

- 本 ADR 是與 Will 討論後的決定（2026-09-25），另以兩份獨立草稿對照；無其他外部來源文件。
- AIP-126：https://google.aip.dev/126
- googleapis `google/api/error_reason.proto`：https://github.com/googleapis/googleapis/blob/master/google/api/error_reason.proto
- Protobuf Best Practices：https://protobuf.dev/best-practices/dos-donts/ ；Enum Behavior：https://protobuf.dev/programming-guides/enum/
- Buf lint rules：https://buf.build/docs/lint/rules/
- PostgreSQL enum 的限制：https://www.postgresql.org/docs/current/datatype-enum.html
- 本機查證：connectrpc Dart 2.0.0 `lib/src/protobuf.dart`（`JsonCodec.ignoreUnknownFields` 預設 true）、protobuf 6.1.0 `lib/src/protobuf/proto3_json.dart`（不認得的 enum 字串在忽略未知時為 null）

## 修訂：第二語系成立，顯示文字改走共用文案目錄（2026-10-17，Will 決定）

原決策段保留歷史。重新評估的條件「需要第二個語系」已成立（買家與賣家介面 zh-TW／en，admin 只有 zh-TW），決定見 `2026-10-17-multi-locale-architecture.md`。

- 脈絡的約束「繼承：只有 zh-TW 一個語系。仍成立」不再成立：買家與賣家介面有 `zh-TW`、`en` 兩個語系，英文介面在該介面沒有未翻譯文字時才發佈；admin 仍只有 zh-TW。
- 後果的重新評估條件「顯示文字改成共用的翻譯目錄或翻譯管理工具」：採用共用目錄（TS 生成 next-intl namespace、Dart 與 Go 直接生成程式碼），key 用不帶版本的 descriptor 路徑，說明取自 proto 註解；現在不用 TMS，觸發條件見新 ADR。
- D5「顯示文字：client 的呈現層，以生成的 enum 為 key」的歸屬不變，只是「只由契約值決定、兩個以上 client 用到」的文字從各 client 手寫的對照表移到共用目錄（`step10e-labels-brief.md`）；伺服器產生的文字仍由產生它的模組擁有，依請求或帳號語系組字。
