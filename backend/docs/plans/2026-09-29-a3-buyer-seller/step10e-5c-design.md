# 10e 第 5c 刀：地址標籤不再以中文字往返，website 修改地址不再覆寫標籤，實作設計 v1.2

> **狀態**：v1.2。v1 經一輪獨立審查（3 MED、6 LOW，9 項都改；D1 在審查列的兩個選項中選「舊 proto 不動」，理由見 F9；對照見附錄 A；全文在 session scratchpad `5c-design-review.md`，不入庫）。**Will 2026-10-21 F1 選 α**（刪掉地址標籤）；性別的同類問題另排 5d（照建議）。F1 以外的決定不需要 Will：沒有使用者看得到的文字或流程改變（§5）。v1.2 處理開工前 design-review（3 項：2 改、1 記，附錄 B）：同一個地址模型的 REST 殘留一起刪（F10、F11），buyer-app 表單結果改成型別化（F3），`reserved 2` 照做並記清理時點（§0.5 末列）。

讀取的是 `local/10e` 的 `a6f7b3815`，worktree 沒有未提交的改動［量測：`git status --short`］。
盤點：2026-10-21 的唯讀盤點（session scratchpad `inv-labelof.md`，不入庫）；本文用到的每一項都在 `a6f7b3815` 重新讀過，結論收進 §0.4。
SSOT：計畫 10e 段（`2026-09-29-a3-buyer-seller.md:382`「範圍外另排一刀：…buyer-app 以中文字反解地址標籤（`_labelOf`）」）；`step10e-Lprime-design.md`（下稱 L′：:209 不收 AddressLabel、:537 `_labelOf` 記成 buyer-app 待辦、:823 Pending「buyer-app 的地址標籤改存 enum」）；`step10e-L0-copy.md` A5（:89-94）；契約 `api/hawkr/buyer/v1alpha1/buyer.proto`（`AddressLabel` :158-164、`Address.label` :193 REQUIRED、`UpdateAddressRequest` :318-322）；根目錄、buyer-app、website 的 `AGENTS.md`。
路徑縮寫：`W` worktree 根、`B`＝`W/buyer-app/lib/features/address`、`WS`＝`W/website`、`M`＝`W/backend/internal/modules/buyer`、`H`＝`W/backend/internal/httpapi/buyer`。
標記：**[量測]** 讀碼或執行命令看到的；**[推論]** 由量測推出、沒有執行驗證；**[外部]** 業界做法，本次沒有重新查證。判定原則同 4a、4b：上線前沒有相容性約束；照業界從零的做法；只有基準定不了的產品選擇、UI 改動、改契約列給 Will。

---

## 0. 前提題：這刀要改的程式之後會不會被重寫、搬走或刪除？

| 程式 | 之後的排程 | 判定 |
|---|---|---|
| buyer-app `B/data/address_repository.dart`、`address_model.dart`、`B/presentation/address_management_screen.dart`、`address_controller.dart` | 沒有。已走 Connect，是終點 | 會留下，照從零的做法做 |
| website `WS/lib/addresses/repo.ts`、兩個地址表單 | 沒有。已走 Connect | 會留下 |
| buyer 模組的地址（`M/internal/address`、`H/app.go`） | 沒有。單體的終點 | 會留下；F1 選 α 時本刀改它 |
| legacy gRPC 的地址（`backend/internal/legacy/buyerserver/server.go:93`、舊 proto `backend/api/proto/buyer/buyer.proto:251,264,277` 與 Java 副本 `backend/services/java/shared/java-proto/src/main/proto/buyer/buyer.proto`） | **會刪**：階段 C 刪 `internal/legacy/`、`api/proto/`、`java-proto`（`backend/docs/plans/2026-09-24-go-modular-monolith.md:290`），在 Java 呼叫方（order-service 結帳的 `GetAddress`）退場之後 | 不在裡面最佳化；α 只拿掉 server.go 的賦值（F9） |
| buyer-app 個資的性別（`settings/data/buyer_profile_fields.dart`） | 沒有排程 | 會留下；不在本刀（F6，另排 5d） |
| buyer-app `order/data/order_status.dart` 的 `DeliveryTempLabel` | 隨 ordering 搬進單體退場（CodeMeta 凍結，根 `AGENTS.md` 共享代碼值列） | 不在裡面最佳化（F7） |

結論：本刀改的除了 legacy gRPC 都會留下；legacy 只做讓它編譯過的最小改動（F9）。

---

## 0.1 範圍

| 部分 | 內容 |
|---|---|
| buyer-app 地址 | 刪 `_labelOf`（`B/data/address_repository.dart:46-53`）與 enum→中文的 switch（`:34-38`）；`Address` 不再帶中文標籤（F2）；表單結果從 `Map<String, String>` 改成型別化的 `AddressFormResult`，沒有 `label`（F3）；刪 REST 時期的 `buyerId`、`fromJson`／`@JsonKey` 與只測它們的 `favorite_address_tsid_test.dart`（F10） |
| website 地址 | 修改時不寫標籤（update_mask 拿掉 `label`，F4）；刪字串版標籤與 `labels` 表；刪 REST 時期的 `buyerId`、`normalizeAddress`、TSID 註解與只測它的 `tests/address-tsid.test.ts`（F11） |
| 標籤本身（F1，**要 Will 決定**） | α：從契約、單體、DB 刪掉 `Address.label` 與 `AddressLabel`；β：保留，兩端建立時都送 `HOME` |

## 0.2 不在範圍

- 買家性別（`Gender`）的文字表與中間字串：另排 5d（F6），要 Will 決定「未設定」的文字。
- `DeliveryTempLabel`（CodeMeta 的溫層）以顯示文字當鍵：隨 ordering 搬遷退場（F7）。
- 地址標籤的選擇器與顯示（F1 的 γ）：觸發是產品要讓買家替地址分類。
- 新增守門規則（F8）。

---

## 0.3 決策總表

| # | 題目 | 決定 | 依據 |
|---|---|---|---|
| F1 | 地址標籤要不要存在（**改契約，要 Will 決定**） | **建議 α：刪掉。** `buyer.proto` 刪 `enum AddressLabel`，`Address` 的 2 號欄位改成 `reserved 2; reserved "label";`；單體刪 `model.Label`、驗證、`labelPB`／`labelModel`、DB 欄位與 CHECK；兩端 client 刪掉所有標籤程式。**替代 β**：契約不動，兩端建立時固定送 `ADDRESS_LABEL_HOME`、修改時不列 `label`（F5）。**替代 γ**：兩端加選擇器並顯示標籤（新的 UI 流程，AddressLabel 因兩端顯示進共用文案目錄，L′ :823）。不採用 γ 作為預設：現在沒有產品需求，而且這個功能曾經存在、在品牌改版時被拿掉（下一欄的產品歷史） | **產品歷史（H10，Will 決定時要看到）**：buyer-app 曾有標籤選擇器與顯示：`22b33360e`（Will，2026-03-01「shipping address does not save label」）在新增／修改表單加「家／公司／其他＋自訂文字」的選擇並送出；`ade3fbce5`（Will，同日）在地址清單以標籤當第一行、依標籤換圖示。`d09f9fa61`（前同事，2026-07-16「complete buyer brand refresh and UI updates」，478 個檔）改版時把選擇器、`_labelType` 與圖示一起刪掉，留下註解「Web 表單沒有額外的地址標籤欄位；編輯時保留舊值，新地址沿用『家』」——也就是對齊 website 表單而拿掉，不是遺漏［量測：`git show --stat` 與 diff］。所以不是「從來沒有需求」，而是「做過、改版時決定不要」；這支持 α，γ 等於推翻那次改版。必要條件：一個 REQUIRED 欄位要有人設定（使用者輸入）或有人讀（畫面或下游）。現況兩者都沒有：四個 client 沒有畫面顯示或讓使用者選標籤（H1、H2），單體只存不讀（H5；legacy gRPC 雖然轉送，但沒有讀者，H11），設計書沒有寫標籤這個功能（H6）。所以現在存的值全是 client 自己編的（buyer-app 編 HOME、website 編 OTHER，H3、H4），REQUIRED 只是在強迫 client 編造。業界：AIP-203 的 REQUIRED 指「沒有它請求就沒有意義」［外部］；地址標籤在零售平台是使用者選的選填欄位（例如蝦皮的「住家／公司」）［外部，本次未查證］。上線前 v1alpha1 不檢查 breaking（`api/buf.yaml` `ignore_unstable_packages`），DB 沒有正式資料，刪除沒有遷移成本。β 的代價：留一個沒有人設定也沒有人讀的必填欄位，與根 `AGENTS.md`「沿用也是決策」相悖；它的好處只是本刀小（client 兩端、沒有契約與 DB）。重新加回的觸發：產品要讓買家替地址分類（γ），屆時以選填加回並同時做選擇器與顯示 |
| F2 | buyer-app `Address.label` 的型別 | **刪掉這個欄位**（α、β 都一樣），不是改成 `AddressLabel`：改完之後 `lib/` 裡沒有任何讀者（唯一的讀者是表單的預設值 `B/presentation/address_management_screen.dart:506-508`，F3 刪掉它）。只有 γ 才需要型別化的欄位，那時欄位型別就是 proto 的 `AddressLabel`、顯示走 `uiText(l)` | 派工寫「改成 `AddressLabel`」；改型別後欄位沒有讀者，留著是死欄位（necessity condition 不成立）。`Address` 的 `fromJson`／`@JsonKey` 是 REST 時期的殘留，只有 TSID 測試在用（盤點 §6），本刀一起刪（F10）；protobuf 的 enum 也不是 json_serializable 能直接處理的型別，留下型別化欄位還得另外處理 JSON |
| F3 | buyer-app 表單結果與呼叫鏈 | **表單結果改成型別化**：`showAddressFormSheet`（`B/presentation/address_management_screen.dart:37,42`）的回傳從 `Map<String, String>?` 改成 `AddressFormResult?`，`AddressFormResult` 是同檔的 Dart 3 record typedef `({String receiverName, String receiverPhone, String townCode, String addressDetail, bool isDefault})`；`_onSave`（`:506-516`）刪 `existingLabel`／`label` 與「Web 表單沒有額外的地址標籤欄位…」註解，`Navigator.pop` 回 record，`isDefault` 直接是 `bool`（刪 `.toString()` 與呼叫端的 `== 'true'`）。三個呼叫端（`address_management_screen.dart:64-76`、`:92-106`，`home/presentation/home_screen.dart:192-204`）改讀具名欄位、拿掉 `label:` 與 `?? ''`。`AddressController.add`／`updateAddress` 與 `AddressRepository.addAddress`／`updateAddress` 拿掉 `label` 參數。修改時不再列 `label`（不論 α、β）。β：`addAddress` 固定送 `AddressLabel.ADDRESS_LABEL_HOME`，以具名常數表示（註解寫「沒有選擇器時的預設，F5」） | client 不寫沒有編輯的欄位（F4 同一個原則）。`Map<String, String>` 沒有必要條件：表單與呼叫端都在 buyer-app 內，沒有序列化或跨邊界的需要；它正是讓標籤以中文字往返、`isDefault` 以字串 `'true'` 往返的載體［量測：讀 `:37-42`、`:506-516` 與三個呼叫端］。從零：小型 record 由編譯器保證欄位與型別，鍵打錯或漏欄位是編譯錯誤；改動面就是本刀本來就要碰的三個呼叫端，所以併入（附錄 B #2）。用 record 不用 freezed 類別：不需要 `copyWith`、JSON 或相等以外的功能，record 自帶結構相等［推論］ |
| F4 | website 修改地址覆寫標籤（bug） | `updateAddress` 的 `updateMask` 拿掉 `"label"`（`WS/lib/addresses/repo.ts:131-137`），`address` 不設 `label`（`:122`）；`Address.label`、`AddressInput` 裡的 `label`、`labels` 表（`:46-50`）、`fromConnect` 的標籤（`:60-65`）都刪。update_mask 列的是表單擁有的欄位（收件人、電話、`postal_address`、`default_address`，表單都顯示），不改成只列有變動的欄位：並行覆寫由 etag 擋（`:117`，伺服器以 `RequiredVersionField` 檢查，`H/app.go:405`） | 根 `AGENTS.md`「UI 改動：bug 修正…可以直接做」。從零：update_mask 表示「client 要改的欄位」，伺服器只合併 mask 裡的（AIP-134［外部］；`H/app.go:413-416` 以 `rpc.ApplyUpdateMask` 套到現有的那筆［量測］）。表單沒有標籤欄位卻把 `label` 列進 mask，等於每次修改都把標籤寫成 OTHER（H4） |
| F5 | 兩端預設值不一致（只在 β） | 都送 `HOME`。這個值不在任何畫面出現，不是 UI 改動；但它仍是編出來的值，所以放在 F1 β 讓 Will 一起決定 | 舊 schema 的預設是 `home`（A3 的 buyer 盤點 `2026-09-29-a3-buyer-seller/buyer-inventory.md:28`）；buyer-app 從 2026-02-28 起新地址就是「家」（`976625d60`）；website 的 OTHER 是 Connect 搬遷時的後備值（`fd5abea2a`，`labels[...] ?? AddressLabel.OTHER`），不是產品決定［量測：`git log -S`］ |
| F6 | 性別的反解（`buyerGenderFromLabel`）在不在本刀 | **不在本刀，另排 5d，要 Will 核可。** 它是同一個反模式，而且比地址標籤嚴重：性別兩端都顯示（buyer-app 男性／女性／其他／「尚未設定」，website 男性／女性／其他／「不公開」），按根 `AGENTS.md` 共用文案目錄列「兩個以上 client 的表也不行」應該進目錄。L′ :209 與 L0 A5 不收的理由是「兩端都還用字串 wire 值」，這個前提不成立：`Profile.gender` 自 `5315cda8d`（2026-09-30）就是 proto `Gender` enum（`buyer.proto:150-156,173`），字串只是兩端各自的中間值（buyer-app `auth/data/auth_repository.dart:166-171,252-254`，website `lib/profile/repo.ts:61-67,106-113`）；`buyer_profile_fields.dart:17-27` 的註解（「proto 是 string」）已過時。5d 的內容：`Gender` 進目錄、兩端的領域值改成 proto `Gender`、刪 `BuyerGender` 字串與 `buyerGenderFromLabel`、顯示走 `uiText`；**要 Will 決定 `GENDER_UNSPECIFIED` 的文字**（「尚未設定」或「不公開」：website 的註解說空值代表不公開，`ProfileClient.tsx:67`，兩者語意不同） | 不併進 5c：要改目錄、兩端顯示文字與一句待核可的文案，5c 本身不需要核可；拆開讓 5c 可以直接做 |
| F7 | `order_status.dart:63-81` `DeliveryTempLabel` | 歸類：CodeMeta 的舊契約（`core/codemeta/code_meta.dart:2840` `ShippingTemperature` 是 CodeMeta 生成物），`labelFor` 同時接受代碼、值與顯示文字當輸入，`colorFor` 以顯示文字判斷顏色，是同一類「文字當鍵」；但 CodeMeta 凍結、隨模組搬遷退場（根 `AGENTS.md` 共享代碼值列），不在本刀改。另：`colorFor` 在 `lib/` 沒有呼叫者，只有 `test/features/order/order_status_presentation_test.dart:210-213` 在用［量測：`rg -n "DeliveryTempLabel" buyer-app/lib buyer-app/test`］，屬 ordering 搬遷時一起刪的死碼 | 觸發：ordering 搬進單體、buyer-app 訂單改走 Connect |
| F8 | 守門要不要補 | **不新增規則。** `check_enum_text` 抓不到 `_labelOf`，因為反方向（文字→enum）不在 D1／D2。地址的正向表 `AddressLabel.ADDRESS_LABEL_HOME => '家'`（`B/data/address_repository.dart:34-38`）鍵就是 `E.M`，D1 其實看得到，只因 manifest 的 `inCatalog: false`、`serverOwned: false` 而只回報（H8）。中間值讓 D1／D2 看不到的情形是性別（buyer-app、website 的 `'male'`）與 website 地址（`'home'`）：proto enum 先轉成自己的字串，文字表的鍵就不再是 `E.M`。結構上的修法是「領域值就是 proto enum」：5c 刪掉地址的中間值，5d 刪掉性別的，之後既有的 D1／D2 就看得到文字表。補一條「文字→`E.M`」的 D3 只會抓到 `_labelOf` 一處（5c 刪掉它）；性別的反解經過中間字串，D3 也抓不到。改抓「`E.M` 與英文字串互轉」會誤判合法的編碼（路由參數、查詢字串）。以顯示文字當 switch 鍵的寫法在兩支 App 還有十幾處（溫層、市集排序、seller-app 客戶篩選等），都是 CodeMeta 或還沒搬遷的功能，隨各自搬遷處理［量測：`rg --pcre2 "^\s*(case\s+)?'[\p{Han}][^']*'\s*(\|\||=>|:)" buyer-app/lib seller-app/lib`］ | 「一次性的模式不加 lint」：proto enum 的中間值只有地址標籤、性別兩例，本刀與 5d 都消掉。重新評估的觸發：5d 之後再出現 proto enum 的中間值，或 code-review 再抓到一次 |
| F9 | α 時舊 gRPC 的 `AddressInfo.label`（`string label = 10`）怎麼辦 | **舊 proto 與 Java 副本不動；`buyerserver/server.go:93` 拿掉 `Label:` 的賦值**，欄位從此是空字串（proto3 的 string 未設定就是 `""`），測試 `server_test.go:77`、`server_integration_test.go:51` 拿掉 `Label: buyer.Home`。不改成 `reserved 10` | 從零：一個沒有讀者的欄位不該在契約裡，所以終點是刪掉；但這份契約整份在階段 C 刪除（`2026-09-24-go-modular-monolith.md:290`，前提題 §0），不在裡面最佳化。約束帳：Java 副本可以改（與 `backend/api/proto/buyer/buyer.proto` 內容相同［量測：`diff` 去掉縮排後無差異］，`backend/scripts/gen-java-proto.sh` 生成），改它不是 REAL 約束擋住，而是代價與好處不成比例：要改兩份 proto、重新生成 `internal/legacy/gen`、多跑 `backend-proto-generated-diff.yml` 與 `backend-java-tests.yml`，換來的行為改變是零——`AddressInfo` 的讀者只讀縣市、區、明細（H11）。`AddAddressRequest.label = 9`、`UpdateAddressRequest.label = 10` 本來就是死的：buyerserver 不實作這兩個 RPC［量測：`rg -n "AddAddress\|UpdateAddress" backend/internal/legacy backend/services --type go --type java -l` 只有生成碼與整合測試呼叫模組的 `b.AddAddress`］。終點與觸發：階段 C 刪整份舊 proto 時一起消失；若在那之前有人要改這份舊 proto 的 buyer 地址訊息，順手 `reserved` |
| F10 | buyer-app `Address` 的 REST 殘留（α、β 都做） | **本刀一起刪**：`B/data/address_model.dart` 刪 `buyerId`（`:14-15`，含「44 號身分分離」註解）、`Address.fromJson`（`:25-26`）、所有 `@JsonKey`（`id` 的 `fromJson: tsidToInt`、`receiverName`／`receiverPhone`／`isDefault` 的 `name:`）、`part 'address_model.g.dart'` 與 `tsid_json.dart` 的 import；build_runner 重生 `address_model.freezed.dart`，並 `git rm` `address_model.g.dart`（不再有 `part` 指向它；build_runner 不一定清掉已提交的孤兒生成檔［推論］）。整檔刪 `buyer-app/test/features/visit/favorite_address_tsid_test.dart`：兩條測試都只測 `Address.fromJson`（TSID 容錯與 snake_case 無相容層），沒有別的內容。`lib/core/json/tsid_json.dart` **保留**：`social/data/social_model.dart` 還在用 | 必要條件是「REST 回 JSON、ID 是 TSID、可能以 number 送來而失精度」，今天不成立：地址走 Connect（§0），`Address` 只由 `_model()`（`B/data/address_repository.dart:31-42`）從 proto 建立，`_model()` 從不設 `buyerId`；proto `Address` 沒有 buyer_id 欄位、`id` 是 string；地址 ID 是 `BIGINT GENERATED ALWAYS AS IDENTITY`（`M/db/schema.sql:42`）。`Address.fromJson` 在 `lib/` 沒有呼叫者（H13）。與 F2 刪 `label` 同一個原則：client 的領域模型只放 wire 上有、有讀者的欄位（附錄 B #1） |
| F11 | website `Address` 的 REST 殘留（α、β 都做） | **本刀一起刪**：`WS/lib/addresses/repo.ts` 刪 `Address.buyerId`（`:17`）、`:13-14` 的「64-bit TSID > JS 2^53」註解、`normalizeAddress`（`:37-44`）；`AddressInput` 從 `Omit<Address, "id" \| "buyerId">`（`:35`）簡化成 `Omit<Address, "id">`。整檔刪 `WS/tests/address-tsid.test.ts`：三條測試都只測 `normalizeAddress` | 同 F10 的必要條件：`fromConnect` 從 proto 建 `Address`，`id` 本來就是字串，沒有 `buyerId` 來源；`normalizeAddress` 除了這個測試沒有呼叫者（H14）。審查原文寫「只有定義」，漏了這個測試檔，本文以量測為準（附錄 B #1） |

---

## 0.4 查證過的現況（`a6f7b3815`）

| # | 事實 | 證據 |
|---|---|---|
| H1 | buyer-app 沒有畫面顯示或讓使用者選地址標籤；`Address.label` 在 `lib/` 唯一的讀者是表單的預設值（編輯沿用舊值，新增用「家」） | [量測] `B/presentation/address_management_screen.dart:506-508`；`rg -n "\.label\b" buyer-app/lib/features/address`（其餘是元件自己的 `label` 參數） |
| H2 | website、seller-app、admin 沒有任何地方讀或顯示地址標籤 | [量測] `rg -n "AddressLabel\|ADDRESS_LABEL" seller-app/lib admin/src website -g '!**/gen/**'` 只有 `WS/lib/addresses/repo.ts` |
| H3 | buyer-app 的往返：enum→「家／公司／其他」（`UNSPECIFIED` 也變「其他」），送出時 `_labelOf` 反解；修改時 `label` 一定非 null，一定進 mask | [量測] `B/data/address_repository.dart:34-38,46-53,137,150`；`address_management_screen.dart:101` |
| H4 | website 兩個表單都不帶 `label`；`createAddress` 因此送 OTHER，`updateAddress` 的 mask 固定含 `label`、值是 OTHER，伺服器照 mask 覆寫 → buyer-app 建立的 HOME 在 website 改一次就變 OTHER | [量測，讀碼] `WS/features/account/screens/addresses/AddressFormClient.tsx:110-117`、`AddressesClient.tsx:441-448`、`WS/lib/addresses/repo.ts:100,122,131-137`；`H/app.go:413-416`、`backend/internal/httpapi/rpc/fieldmask.go:21-44`（mask 裡的欄位有值就設）。**沒有執行重現**；現有 `repo.unit.test.ts` 6 條通過，沒有一條斷言 label［量測：`pnpm exec vitest run --project unit lib/addresses/repo.unit.test.ts`］ |
| H5 | 單體只存標籤，不讀：`model.Label` 只用在驗證、寫入、`changed` 比較與轉成 proto（Connect 與 legacy gRPC 兩處，後者見 H11）；沒有 pub view、事件或別的模組讀它 | [量測] `M/internal/address/address.go:38-39,173,214,232`、`H/app.go:128,180-204`；`M/db/schema.sql:44,53`；`rg -n "\.Label\b\|LabelValues" backend --type go` |
| H6 | 設計書與開發規範沒有寫地址標籤這個功能 | [量測] `rg -n "地址標籤\|address.?label\|住家" backend/docs -g '*.md'`（只有計畫文件與 API 設計規範一處無關的「住家」） |
| H7 | 現有測試鎖住往返：`sentLabel('家')`→HOME 等、清單 enum→「家、公司、其他」、`addAddress(label: '家')` | [量測] `buyer-app/test/features/address/address_etag_contract_test.dart:147-196,286`；`fvm flutter test` 該檔 11 條通過 |
| H8 | 守門：AddressLabel 在 manifest 是 `inCatalog: false`、`serverOwned: false`，buyer-app 的正向表只回報；`_labelOf` 不在 D1／D2 | [量測] `packages/hawkr_ui_text/tool/enum_manifest.g.json:11-15`、`tool/check_enum_text.dart:5-13`；盤點 §2 的 `--report` 輸出 |
| H9 | 性別：proto `Gender` enum；兩端各自以英文字串當中間值，顯示文字兩端各寫一份，「未設定」的文字不同 | [量測] F6 的檔案與行號；`git log -S'enum Gender' -- api/hawkr/buyer/v1alpha1/buyer.proto` → `5315cda8d` |
| H10 | buyer-app 曾有標籤選擇器與依標籤的顯示，2026-07-16 品牌改版時刪掉 | [量測] `git show --stat 22b33360e ade3fbce5 d09f9fa61` 與三者對 `address_management_screen.dart` 的 diff（`22b33360e` 加 `_labelType` 與送出；`ade3fbce5` 加 `_labelIcon` 與以標籤當第一行；`d09f9fa61` 刪 `_labelIconAsset`、`_labelType`，加「Web 表單沒有額外的地址標籤欄位」註解） |
| H11 | legacy gRPC 把標籤轉送給舊服務，但沒有讀者：`GetAddress`／`GetAddressList` 的 `AddressInfo.label` 由 `buyerserver/server.go:93` 設定；讀 `AddressInfo` 的 crm-service（`backend/services/crm-service/internal/client/buyer/client.go:77-90`）與 Java order-service（`BuyerGrpcClient`、`CheckoutService` 的 Uber dropoff）只讀縣市、區、明細 | [量測] `rg -n "GetLabel\|getLabel\(" backend/services backend/internal/legacy --type go --type java`（命中的都是發票、售後事件、delegation、AI 等別的訊息的 label，沒有 `AddressInfo`）；審查 D1 |
| H12 | seed 資料帶地址標籤：`backend/infra/seed-data/01-account.sql:77-83` 的 INSERT 有 `label` 欄位與 `'HOME'`／`'COMPANY'`；CI 不跑 seed | [量測] 讀檔；`rg -n "seed-data\|seed-test-data" .github/workflows` 無命中 |
| H13 | buyer-app `Address.buyerId` 永遠是 null、`Address.fromJson` 在 `lib/` 沒有呼叫者，兩者只有 `favorite_address_tsid_test.dart` 在用（整檔兩條測試都是 `Address.fromJson`）；`tsid_json.dart` 另有 `social_model.dart` 在用 | [量測] 讀 `B/data/address_model.dart`、`B/data/address_repository.dart:31-42`；`rg -n "Address\.fromJson\|buyerId" buyer-app/lib buyer-app/test`（地址相關只有生成檔與該測試）；`rg -ln "tsidToInt\|tsid_json" buyer-app/lib buyer-app/test` |
| H14 | website `normalizeAddress` 只有 `tests/address-tsid.test.ts` 呼叫（整檔三條測試都是它），`buyerId` 只出現在 `repo.ts` 與該測試 | [量測] `rg -n "normalizeAddress\b\|buyerId" website -g '!node_modules' -g '!.next'` |

## 0.5 約束帳（本刀新出現的）

| 約束 | 判定 | 來源 |
|---|---|---|
| `Address.label` REQUIRED | SELF-IMPOSED（我們的契約，v1alpha1 不檢查 breaking）；F1 | `buyer.proto:193`、`api/buf.yaml` |
| DB 的 `label NOT NULL` 與 CHECK | SELF-IMPOSED（我們的 schema，上線前） | `M/db/schema.sql:44,53`；buyer `CARRYOVER.md:45` 把 label 列為封閉值域沿用，當時沒有問有沒有人設定或讀它 |
| L′ 把 `_labelOf` 記成待辦、L0 A5 把性別延後 | SELF-IMPOSED（計畫排序）；本刀執行前者，後者前提不成立（F6） | L′ :537、:823；L0 :94 |
| update_mask 只改 mask 裡的欄位 | **REAL**（契約語意 AIP-134，單體照它實作；F4 依賴它） | `H/app.go:413-416` |
| 地址修改帶 etag | **REAL**（契約） | `buyer.proto:319`「id 與 etag 必填」 |
| 舊 gRPC 的 `AddressInfo.label` 與 Java 副本 | SELF-IMPOSED（我們的舊契約，副本可以改）；整份在階段 C 刪除，F9 因此不改它 | `2026-09-24-go-modular-monolith.md:290`；F9 |
| 版面、UX 文案與流程的改動要先取得同意 | 授權規則（不是技術約束）；本刀沒有這類改動（§5），γ 才有 | 根 `AGENTS.md` 共通規則「UI 改動」 |
| client 不得手寫兩端以上的 enum 文字表 | 已核准的規則（ADR `2026-10-17-multi-locale-architecture.md`），本刀不重開；F6 依它判定性別該進目錄 | 根 `AGENTS.md` 跨專案契約「共用文案目錄」列 |
| α 刪欄位時 `reserved 2; reserved "label";` | SELF-IMPOSED：reserved 防的是已部署的 binary 或已存的資料把新欄位誤讀成舊欄位，上線前兩者都沒有（v1alpha1 不檢查 breaking），必要條件今天不成立；但成本近乎零、與既有 `reserved 5, 6, 7` 一致，也是 protobuf 的建議做法［外部］，**照做**。終點與觸發：**上線改名 v1 時一併清掉 v1alpha1 期間累積的 reserved**（v1 是新套件，沒有任何 client 見過 alpha 的欄號）；觸發是 buyer 套件從 `v1alpha1` 改名 `v1` 的那次變更 | `buyer.proto:189-190`；`API 設計規範（Connect）.md:17`；附錄 B #3 |

---

## 1. 契約與單體（只在 F1 選 α）

- `buyer.proto`：刪 `enum AddressLabel`（:158-164）；`Address` 加 `reserved 2; reserved "label";`（與既有的 `reserved 5, 6, 7` 同一處；清理時點見 §0.5 末列）。三個生成端重新生成：backend `make connect`、`pnpm --filter @hawkr/api gen:api`、`packages/hawkr_api` 的 `../../api/generate.sh buf.gen.yaml`（PATH 先放 `$HOME/.local/bin`；Dart 先暖機，見 `AGENTS.local.md`）。enum 消失會改到 manifest：`make -C backend uitext` 重新生成 `enum_manifest.g.json` 與 `enum-manifest.gen.json`。
- 單體：`M/db/schema.sql` 刪 `label` 欄位與 `shipping_addresses_label_check`，`atlas migrate diff`（Docker 先確認）；`M/db/queries/shipping_addresses.sql:32,39` 拿掉 label，`make sqlc`；刪 `model.Label`、`Home`／`Company`／`OtherAddress`、`LabelValues`（`M/internal/model/model.go:38-47,76-77,128,138`）與 `M/buyer.go:105-106,198-203,507-508` 的轉出；`Validate` 刪標籤檢查（`address.go:38-39`，句子「地址標籤不合法」隨之刪），`address.go:72` 的讀出、`changed` 與寫入參數拿掉 label；`H/app.go` 刪 `labelPB`、`labelModel`（句子「label 必須指定」隨之刪）、`addressPaths["Label"]`（`:239`）與呼叫處。
- legacy gRPC（F9）：`backend/internal/legacy/buyerserver/server.go:93` 刪 `Label:` 那行；舊 proto 與 Java 副本不動。
- seed（H12）：`backend/infra/seed-data/01-account.sql:77-83` 的 INSERT 拿掉 `label` 欄位與四列的值（CI 不跑 seed，不改不會被擋，`backend/infra/seed-test-data.sh` 會在本機直接失敗）。
- 測試跟著刪：`M/buyer_integration_test.go:594` 的 CHECK 對照列、`M/internal/address/address_test.go:30-39`、`M/version_integration_test.go:42,83,192,203`、`H/app_test.go` 與 `H/buyer_integration_test.go` 的標籤斷言、`backend/internal/legacy/buyerserver/server_test.go:77`、`server_integration_test.go:51`。程式與測試的遺漏都會被編譯抓到；seed 與下面的文件不會。
- `scripts/check-enum-text-cross.sh:12` 註解的「單端的表（AddressLabel、…）」改成只舉 admin 單端 enum。
- 模組文件（指令檔只放現行規則）：buyer `AGENTS.md:29`「gender、label、狀態…有 CHECK」拿掉 label；`M/CARRYOVER.md:32`「補 gender/label/phone/date 驗證」、`:45`「source/status／label/gender」、`:100`「gender、label、buyer/membership 狀態有 CHECK」拿掉 label；沿用盤點表（5 欄：沿用的機制｜必要條件｜今天是否成立｜業界在同樣條件下的做法｜決定，`CARRYOVER.md:9`）補一列：「地址標籤（label：HOME／COMPANY／OTHER，REQUIRED＋CHECK）｜有人設定（使用者選）或有人讀（畫面或下游）｜不成立：沒有畫面設定也沒有讀者，buyer-app 的選擇器 2026-07-16 品牌改版刪除（`d09f9fa61`），值全由 client 編造；legacy gRPC 轉送但沒有讀者｜零售平台的地址標籤是使用者選的選填欄位；沒有功能就不存欄位｜刪除（5c F1 α）；產品要讓買家替地址分類時以選填加回，同時做選擇器與顯示」。

## 3. clients

### 3.1 buyer-app（F2、F3、F10）

- `B/data/address_model.dart`：刪 `label`（:16）、`buyerId`（:14-15）、`fromJson`（:25-26）、所有 `@JsonKey`、`part 'address_model.g.dart'`、`tsid_json.dart` 的 import（F10）；`fvm dart run build_runner build --delete-conflicting-outputs` 重新生成 `address_model.freezed.dart`，`git rm B/data/address_model.g.dart`。
- 整檔刪 `buyer-app/test/features/visit/favorite_address_tsid_test.dart`（F10）；`lib/core/json/tsid_json.dart` 保留（`social_model.dart` 在用）。
- `B/data/address_repository.dart`：刪 `:34-38` 的 switch 與 `:46-53` 的 `_labelOf`；`addAddress`、`updateAddress` 刪 `label` 參數與 `:137`、`:150`。β：`addAddress` 送 `label: _defaultLabel`（`static const _defaultLabel = AddressLabel.ADDRESS_LABEL_HOME;`，註解「沒有選擇器時的預設（5c F5）」）。α：生成碼沒有 `label`，什麼都不送。
- `B/presentation/address_controller.dart:22-33,44-59` 刪 `label` 參數。
- `B/presentation/address_management_screen.dart`（F3）：新增 `typedef AddressFormResult = ({String receiverName, String receiverPhone, String townCode, String addressDetail, bool isDefault});`；`showAddressFormSheet` 與 `showModalBottomSheet` 的型別參數（`:37`、`:42`）改成它；`_onSave` 刪 `:506-508`（含註解），`Navigator.pop` 回 record。三個呼叫端（`:64-76`、`:92-106`、`home/presentation/home_screen.dart:192-204`）改讀 `result.receiverName` 等具名欄位，不再傳 `label`，`isDefault: result.isDefault`。

### 3.2 website（F4、F11）

- `WS/lib/addresses/repo.ts`（F11）：刪 `Address.buyerId`（:17）、`:13-14` 的 TSID 註解、`normalizeAddress`（:37-44）；`AddressInput` 改成 `Omit<Address, "id">`（:35）。整檔刪 `WS/tests/address-tsid.test.ts`。
- `WS/lib/addresses/repo.ts`（F4）：刪 `Address.label`（:24，`AddressInput` 隨之沒有）、`labels`（:46-50）、`fromConnect` 的標籤（:60-65）；`updateAddress` 的 `address` 不設 `label`、mask 拿掉 `"label"`。β：`createAddress` 送 `label: AddressLabel.HOME`（具名常數，註解同 3.1）；α：不送。
- α 時 `AddressLabel` 的 import（:4）一起刪；β 時保留。
- 兩個表單不用改（本來就不帶 label）。

## 5. 文案表（5c 的部分；A＝已核准、C＝跟隨契約或 SSOT、E＝沿用、**N＝新寫，要 Will 核可**）

使用者看得到的文字沒有新增也沒有改變：標籤從未在任何畫面出現（H1、H2）。

| # | 位置 | 改前 → 改後 | 類 |
|---|---|---|---|
| 1 | `B/data/address_repository.dart:35-37`「家／公司／其他」 | 刪（從未顯示，只是往返的中間值） | C |
| 2 | 伺服器句子「地址標籤不合法」（`M/internal/address/address.go:39`）、「label 必須指定」（`H/app.go:203`） | 只在 α：刪（欄位不存在） | C |

本刀沒有 N 類。5d（性別）會有一句：`GENDER_UNSPECIFIED` 的文字（F6）。

## 6. 測試

兩個 client 的 `fieldMask` 都依 descriptor 檢查路徑，不認得的路徑丟例外（Dart `packages/hawkr_api/lib/field_mask.dart:12`、TS `@hawkr/api` 的 `proto/field-mask`）。所以 α 刪掉 `label` 之後，「把 `label` 加回 mask」不是測試斷言能抓的突變（會先丟例外或編譯失敗），`hasLabel()`／`label` 也不存在。α 與 β 的測試因此分開寫。

### 6.1 buyer-app（`buyer-app/test/features/address/address_etag_contract_test.dart`）

兩者共同：
- 刪 `:147-196` 的 label group；`:286` 的 `label: '家'` 拿掉。
- 新增：只改收件人時，送出的 `update_mask.paths` **等於** `['receiver_name']`；改地址時等於 `['postal_address']`（斷言整個清單，不只「不含 label」）。
- `address_management_sheet_test.dart`（F3）：新增一條填表送出、接住 `showAddressFormSheet` 回傳的測試，斷言 `AddressFormResult` 的五個欄位等於輸入，`isDefault` 是 `bool`（現在的 helper `:167` 丟掉回傳值，沒有任何測試鎖住表單結果，盤點 §5）。沒有 `label` 由型別保證。**突變**：`_onSave` 把 `isDefault` 固定成 `false`、或 `receiverName` 改填電話時要失敗。
- 整檔刪 `test/features/visit/favorite_address_tsid_test.dart`（F10；被測的 `Address.fromJson` 不存在，留著編譯失敗）。

α：
- 上面的清單斷言就是全部；標籤不存在由編譯保證。**突變**：在 `updateAddress` 多加一個表單沒有改的路徑（例如無條件 `fields.add('receiver_phone')`），清單斷言要失敗。

β：
- 修改送出的 `address.hasLabel()` 是 false。**突變**：把 `if (label != null) fields.add('label')` 改成無條件加 `'label'` 並設值時，清單斷言與 `hasLabel()` 斷言要失敗。
- 新增：`addAddress` 送出 `ADDRESS_LABEL_HOME`。**突變**：常數改成 `OTHER` 時要失敗。

### 6.2 website（`WS/lib/addresses/repo.unit.test.ts`）

兩者共同：
- 新增：`updateAddress` 的 `updateMask.paths` **等於** `["receiver_name", "receiver_phone", "postal_address", "default_address"]`。先寫、在改動前執行，要失敗（現在多了 `"label"`）——這就是 H4 的重現。
- 整檔刪 `WS/tests/address-tsid.test.ts`（F11；被測的 `normalizeAddress` 不存在）。`buyerId` 不存在由 `tsc` 保證。

α：
- 既有 fixture 的 `label: 0`（:54,67,71,111）刪掉（生成型別沒有這個欄位，留著 `tsc` 失敗）。**突變**：mask 多加一個存在但表單沒有改的路徑（例如 `"etag"`）時，清單斷言要失敗。

β：
- 新增：`updateAddress` 送出的 `address.label` 是預設值 `AddressLabel.UNSPECIFIED`（0），mask 不含 `"label"`。**突變**：mask 加回 `"label"`、或 `address` 設回 `label` 時要失敗。
- 新增：`createAddress` 送出 `AddressLabel.HOME`。**突變**：改回 `OTHER` 時要失敗。
- fixture 的 `label: 0` 留著無妨。

### 6.3 backend（只在 α）

- `go test ./internal/modules/buyer/... ./internal/httpapi/buyer/... ./internal/legacy/buyerserver/...`（integration 帶 `DOCKER_HOST` 與 `-tags integration`）：建立與修改不帶標籤仍成功；CHECK 對照測試不再有 shipping_addresses 那列；legacy `GetAddress` 的 `AddressInfo.label` 是空字串、其餘欄位不變。
- seed：本機 `backend/infra/seed-test-data.sh` 跑一次（CI 不跑，H12）。

## 7. Commit 計畫

| # | 內容 | 審查 |
|---|---|---|
| 5c-1（α） | 契約、三個生成端、uitext manifest、單體（schema、migration、sqlc、model、httpapi）、legacy `buyerserver` 的賦值（F9）、seed、buyer-app（含 F3 的型別化表單結果、F10 的 REST 殘留與 `address_model.g.dart`、`favorite_address_tsid_test.dart` 的刪除）、website（含 F11 的 REST 殘留與 `tests/address-tsid.test.ts` 的刪除）、`check-enum-text-cross.sh` 註解，同一個 commit（根 `AGENTS.md`「改契約時，同一次變更內更新唯一來源與所有消費端」） | code-review |
| 5c-1（β） | buyer-app 與 website（含 F3、F10、F11），同一個 commit（同一個缺陷的兩端） | code-review |
| 5c-2 | 文件：計畫 10e 的 5c 完成紀錄（:382 的範圍外項目標完成）；L′ 文末加修訂：`:34`、`:209`、`:537` 的「AddressLabel 只有 buyer-app 一端顯示」不成立（2026-07-16 起任何 client 都不顯示，H1、H10），`:823` 的 Pending 結案並寫明 F1 的結果；α 時 buyer `AGENTS.md:29` 與 `CARRYOVER.md`（§1）；5d 的排程（F6）記進計畫 | 併入完成前的 design-review |

改動小，design-review 只在 5c-2 之後看整刀一次；實作派 Sonnet subagent，主 session 寫派工、驗證與審查。

## 8. 驗證命令（派工前照當時的 `.github/workflows/` 再核對）

- buyer-app（`buyer-ci.yml` → `_flutter-quality.yml`）：`dart format --output=none --set-exit-if-changed lib test integration_test tool`、`flutter analyze`、`dart run custom_lint`、`bash scripts/deadcode.sh`、`packages/hawkr_ui_text` 的 `dart run tool/check_enum_text.dart ../../buyer-app/lib`、完整測試（CI 是 `bash scripts/ci-flutter-test.sh --no-pub --exclude-tags hermetic-integration --coverage --coverage-path coverage/lcov.info`，另跑 `--tags hermetic-integration`）、**覆蓋率 ratchet**：`dart run tool/coverage_ratchet.dart --lcov coverage/lcov.info --baseline tool/coverage_baseline.json --floor-baseline <base 的 tool/coverage_baseline.json> --summary coverage/coverage-summary.json`（`_flutter-quality.yml:116-157`；刪 `_labelOf` 與 label group 會同時減少已覆蓋行數，比例可能低於 base，派工時就要跑，不交給 CI）、`dart run build_runner build --delete-conflicting-outputs` 後 `git diff --exit-code`（`address_model.freezed.dart` 有變、`address_model.g.dart` 已刪，重跑後不得再出現）。
- website（`website-ci.yml:83-91`，在 `website/`）：`pnpm run lint`（含 `tsc`、biome、`check-enum-text`）、`pnpm run design:check --apps-root ..`、`pnpm run test:unit`、`pnpm test`、`pnpm run build`（帶 `API_BASE_URL=http://127.0.0.1:9/api/v1` 與同值的 `NEXT_PUBLIC_API_BASE_URL`）、build 之後 `pnpm run test:build-output`。
- 跨專案：`bash scripts/check-enum-text-cross.sh`。
- 只在 α 另加：`api-contract.yml`（`buf lint`、googleapis 版本一致、`buf breaking`；`packages/hawkr_api` 的生成物 drift、`dart analyze`、`dart format`、`dart test`；`packages/hawkr-api-ts` 的生成物 drift、`tsc --noEmit`、biome、vitest）；`backend-monolith.yml`（路徑含 `backend/internal/**`，涵蓋 legacy buyerserver：gofmt、`go vet`（含 `-tags integration`）、`go test ./internal/...`、`make lint-monolith`、`GOWORK=off go mod tidy` 無 diff）；`ui-text-generated-diff.yml`（`make -C backend uitext` 後 `git diff --exit-code`）、`ui-text-guard.yml`；`seller-ci.yml`、`admin-ci.yml`、`ts-packages.yml`、`dart-packages.yml`（都被 `api/**` 或生成套件的路徑觸發）。seller-app 與 admin 沒有用到 AddressLabel（H2），預期只需要跑過。不在 CI、本機要跑：`make -C backend atlas-validate-all`（Docker）、seed（§6.3）。F9 不改舊 proto，所以 `backend-proto-generated-diff.yml`、`backend-java-tests.yml` 不會觸發。
- 看 CI 時看每個 job 自己的結論（根 `AGENTS.md` 驗證）。

## 9. 沒有驗證的部分

- H4 的覆寫是讀碼推出的，沒有在本機實際以 website 修改一筆 HOME 地址重現；§6.2 的第一條測試在修之前執行即是重現。
- 業界的地址標籤做法（F1 的蝦皮例子、AIP-203、AIP-134）沒有重新查證。
- F8 的「十幾處以顯示文字當鍵」是 regex 的粗估，沒有逐處分類（不在本刀）。
- α 的 Atlas migration、sqlc 生成與 seed 要 Docker，本次沒有試跑。
- 覆蓋率 ratchet 在刪除後是否仍過 base，本次沒有跑（§8 要求派工時跑）。
- H10 的「對齊 website 表單而拿掉」是依 `d09f9fa61` 的註解推論，沒有問相關同事。

---

## 附錄 A：審查意見的處理（v1 → v1.1；審查全文在 session scratchpad `5c-design-review.md`）

| # | 意見（嚴重度） | 處理 |
|---|---|---|
| D1 | α 漏了 legacy gRPC：`buyerserver/server.go:93` 把標籤送進舊 proto `AddressInfo.label`，沒決定 reserve 還是留成空字串（MED） | **改**：新增 F9、H11、§0 前提題一列、§0.5 一列、§1。決定留著舊 proto 與 Java 副本、只刪 server.go 的賦值：整份舊 proto 在階段 C 刪除（`2026-09-24-go-modular-monolith.md:290`），Java 副本改得到但改了換不到任何行為（沒有讀者，H11），不在要刪的程式裡最佳化 |
| D2 | α 漏了 seed `backend/infra/seed-data/01-account.sql:77-83`（MED） | **改**：H12、§1 補 seed，§6.3、§8 本機跑一次（CI 不跑） |
| D3 | F1 的依據漏了產品歷史：buyer-app 曾有選擇器與依標籤的圖示，`d09f9fa61` 改版刪掉（MED） | **改**：F1 加產品歷史、H10［量測：`git show --stat` 與 diff 三個 commit］；γ 不採用的理由改成「做過、改版時決定不要」 |
| D4 | L′ :34、:209、:537 的「只有 buyer-app 一端顯示」不成立，5c-2 只結 :823（LOW） | **改**：§7 5c-2 在 L′ 文末加修訂 |
| D5 | §1 的 Go 清單不全，buyer `AGENTS.md:29`、`CARRYOVER.md:32` 會過時（LOW） | **改**：§1 補 `buyer.go:198-203`、`address.go:72`、`version_integration_test.go`、`H/app.go:239`、legacy 測試、`AGENTS.md:29`、`CARRYOVER.md:32,45,100` |
| D6 | §6 的測試在 α 下寫不出來（`hasLabel()` 不存在，加回 `'label'` 會被 `fieldMask` 丟例外）（LOW） | **改**：§6 分 α／β 寫；兩者都斷言整個 mask 清單，α 的突變改成多加一個未改的路徑 |
| D7 | F8 說中間值讓 D1／D2 看不到地址的表，不正確（正向表鍵就是 `E.M`，只因 `inCatalog: false` 只回報）（LOW） | **改**：F8 更正；中間值的說法只留給性別與 website 地址。結論不變 |
| D8 | §8 漏了 coverage ratchet、`test:build-output`、Atlas（LOW） | **改**：§8 補 ratchet（派工時跑）、`test:build-output`、`api-contract.yml` 的完整步驟、`atlas-validate-all`；舊 proto 的兩條 workflow 因 F9 不改而不觸發 |
| D9 | CARRYOVER 補列只有 4 格，表是 5 欄（LOW） | **改**：§1 補成 5 欄 |

## 附錄 B：開工前的 design-review（v1.1 → v1.2，3 項；全文在 session scratchpad `5c-design-review-start.md`）

| # | 發現 | 處理 |
|---|---|---|
| 1 | 同一個地址模型的 REST 殘留照搬：buyer-app `buyerId`、`fromJson`／`@JsonKey`，website `buyerId`、`normalizeAddress`、TSID 註解；必要條件（REST JSON、TSID 失精度）已不成立（A，高） | **改**：本刀一起刪（F10、F11、H13、H14，§0.1、§3、§6、§7）；只測它們的 `favorite_address_tsid_test.dart`、`tests/address-tsid.test.ts` 整檔刪（審查漏了後者）；`tsid_json.dart` 因 `social_model.dart` 在用而保留 |
| 2 | buyer-app 表單結果是 `Map<String, String>`，沒有必要條件也沒有觸發（C，中） | **改**：併入本刀，改成 record `AddressFormResult`，三個呼叫端跟著改（F3、§3.1、§6.1） |
| 3 | `reserved 2; reserved "label";` 上線前必要條件不成立，清理時點沒人決定（A，低） | **記**：照做；§0.5 末列寫明「上線改名 v1 時一併清掉 v1alpha1 的 reserved」，觸發是 buyer 套件改名 `v1` 的那次變更 |

## 實作時的修訂

- website 兩個地址表單的編輯判斷改以 `editing`／`editTarget` 物件本身判斷，`updateAddress(id, etag, body)` 的 etag 由物件帶入；不新增使用者面文字（修正輪曾加兩句「找不到要編輯的地址…」，分支到不了且未經核可，已以型別消除）。
- proto 的 `reserved 2`／`"label"` 併入既有的 `reserved` 行，語意同 §1。

## 附錄 C：完成前的 design-review（4 項；全文在 session scratchpad `5c-design-review-done.md`，不入庫）

| 編號 | 發現 | 處理 |
|---|---|---|
| 1 | [A 高] REST 形狀殘留：buyer-app 與 website 的 `Address` 收件人姓名、電話可為 null、etag 預設空字串，消費端 `?? ''` 與執行期空 etag 檢查 | 改：兩端改非 null 必填（契約 REQUIRED；etag 由單體 `toAddress` 以 NOT NULL 的 version 產生），刪執行期檢查；website `updateAddress` 改必帶 etag |
| 2 | [A 中] buyer-app 地址 ID 仍是 `int` | 記：`CARRYOVER.md` 新列，觸發「buyer-app 結帳改走 Connect（ordering 搬進單體）」；`checkout_payment_sheet.dart` 的過時註解改成現況 |
| 3 | [C 中] 完成紀錄的未驗證項與 §6.2、§6.3、§8 衝突 | 改：H4 以新的 mask 清單測試在 base `a6f7b3815` 重現（mask 多出 `label`）；coverage ratchet 依本機規則交 CI；完整 seed 腳本本機缺容器與 Java DB，改以拋棄式 Postgres `make migrate`＋`01-account.sql` 驗證 |
| 4 | [A 低] CARRYOVER 新列的業界做法沒有證據等級 | 改：兩列都標證據等級 |

code-review 兩輪（第一輪 0 缺陷；修正後的累積審查 0 程式缺陷、1 LOW 文件缺陷，已改）。
