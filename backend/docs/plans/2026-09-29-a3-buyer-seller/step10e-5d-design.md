# 10e 第 5d 刀：買家性別改用 proto `Gender` 並由共用文案目錄顯示，buyer-app 個資與地址選單型別化，實作設計 v1.2

> **狀態**：v1.2。v1 經一輪獨立審查（2 高、7 中、10 低；全部處理，對照見附錄 A；全文在 session scratchpad `5d-design-review.md`，不入庫）。v1.1 經開工前 design-review（2 中、8 低；全部處理，對照見附錄 C；全文在 `5d-design-review-start.md`，不入庫），主要變更：觸發器的顯示規則兩端統一（G4）、地址的縣市與鄉鎮選單一起改成型別化（G13）、`ProfileEdit` 固定列出表單擁有的欄位、生日不再以 null 表示不改（G7）、`BuyerProfile` 改成 `final class`（G8）、G12 改成無條件、三元式的守門缺口移到計畫層級。**Will 2026-10-22 的決定（決策紀錄見 §0.6）**：沒有填性別時顯示「尚未設定」；G1 採 α（`Profile.gender` 改 `optional`，送 `UNSPECIFIED` 回 `invalid_argument`），同一刀修訂 `API 設計規範（Connect）.md:100`；buyer-app 編輯頁的「會員等級」那一行刪掉，個資管線的型別化併入本刀（G8，不另排 5e）；英文 Male／Female／Other／Not set 核准，本刀寫進 `en.json`；選單裡「尚未設定」是可以選的一項，選了就清除性別，沒有填時標成已選。**controller 2026-10-22 的決定**：G12 接受（`API 設計規範` :102 補一句，隨 Will 的 G1 選擇而來）；開工前審查 F1-F10 的處理方式（附錄 C）。沒有待決項目。

讀取的是 `local/10e` 的 `0e990f626`（與本機 `main` 相同），worktree 只有本文一個未追蹤檔［量測：`git status --short`］。
SSOT：計畫 10e 段（`2026-09-29-a3-buyer-seller.md:382`）；`step10e-5c-design.md` F6、F8、H9；`step10e-Lprime-design.md`（下稱 L′：:209 不收 Gender，§1.2 key 格式，§1.3 :136 `en.json` 只放有 message 的 key，§2.4 :263 缺字行為）；`step10e-L-brief.md` Q4（:109-130）；`step10e-L0-copy.md` A5（:93）；ADR `backend/docs/adr/2026-10-17-multi-locale-architecture.md`（:55 發佈門檻、:104 英文由 LLM 起草 Will 審、:108 未發佈語系只回報覆蓋率）；`backend/docs/開發規範/API 設計規範（Connect）.md:100,102`；契約 `api/hawkr/buyer/v1alpha1/buyer.proto`（`Gender` :150-156、`Profile.gender` :165）；根目錄、buyer-app、website 的 `AGENTS.md`。
路徑縮寫：`W` worktree 根、`BA`＝`W/buyer-app/lib`、`BT`＝`W/buyer-app/test`、`WS`＝`W/website`、`H`＝`W/backend/internal/httpapi/buyer`、`M`＝`W/backend/internal/modules/buyer`、`U`＝`W/backend/internal/platform/uitext`、`G`＝`W/backend/cmd/uitext-gen`、`API規範`＝`W/backend/docs/開發規範/API 設計規範（Connect）.md`。
標記：**[量測]** 讀碼或執行命令看到的；**[推論]** 由量測推出、沒有執行驗證；**[外部]** 業界做法。判定原則同 4a、4b、5c：上線前沒有相容性約束；照業界從零的做法；只有基準定不了的產品選擇、UI 改動、改契約列給 Will。

---

## 0. 前提題：這刀要改的程式之後會不會被重寫、搬走或刪除？

| 程式 | 之後的排程 | 判定 |
|---|---|---|
| buyer-app 個資：`BA/features/settings/data/buyer_profile_fields.dart`、`presentation/profile_edit_screen.dart`、`widgets/profile_edit_widgets.dart`、`BA/features/auth/data/auth_repository.dart` 的 `getMemberInfo`／`getProfile`／`updateProfile`／`_profileMap`，以及讀個資的四個呼叫端（H14） | 沒有排程。已走 Connect，是終點 | 會留下，照從零的做法做（G8） |
| buyer-app `BA/shared/widgets/app_dropdown.dart`、地址表單 `BA/features/address/presentation/address_management_screen.dart` 的縣市與鄉鎮選單 | 沒有 | 會留下（G4、G13） |
| website `WS/lib/profile/repo.ts`、`WS/lib/hooks/useAccount.ts`、`WS/features/account/screens/profile/ProfileClient.tsx` | 沒有。已走 Connect | 會留下 |
| 單體 `H/app.go` 的性別轉換 | 沒有。單體的終點 | 會留下；本刀改 |
| legacy gRPC `backend/internal/legacy/buyerserver/server.go:73-74` 與舊 proto `backend/api/proto/buyer/buyer.proto:186,209` | **會刪**：階段 C（`2026-09-24-go-modular-monolith.md:290`） | 不動：它讀模組的 `model.Gender`，不經 Connect 契約［量測：讀檔］ |

---

## 0.1 範圍

| 部分 | 內容 |
|---|---|
| 契約（G1） | `Profile.gender` 改 `optional Gender gender = 4;`，沒有填就不設值；`GENDER_UNSPECIFIED` 不使用 |
| 規範（G1） | `API規範:100` 改寫成現行做法（零值不是合法值的 enum，「沒有值」是業務狀態時用 `optional`），寫明這是本庫的取捨、AIP-149 不支持 |
| 共用文案目錄（G2、G6） | `buyer.Gender` 三個值的名詞與 `buyer.Profile.gender_unset`；`zh-TW.json` 與 `en.json` 都寫；`make uitext` |
| buyer-app（G3-G9） | 個資讀寫改成型別化（`BuyerProfile`、`ProfileEdit`），領域值是 proto `Gender?`；刪 `buyer_profile_fields.dart` 整檔與其測試；刪「會員等級」那一行；`AppDropdown<T>` 內部以 index 運作；顯示走目錄 |
| buyer-app 地址（G13） | 縣市與鄉鎮選單改成 `AppDropdown<County>`／`AppDropdown<Town>`，刪「顯示名稱 → 代碼」的反查 |
| website（G3-G7、G9） | `MemberProfile.gender` 與送出的 `gender` 改成 `Gender \| undefined`；刪兩處三元式；選單文字走 `useUiText()`；「不公開」改「尚未設定」；`Dropdown` 加「代表沒有值的那一項」；生日一律列進 mask（修正「清除」不生效） |
| 單體 | `H/app.go` 依欄位有沒有設值轉換；`UNSPECIFIED` 回 `invalid_argument` |

## 0.2 不在範圍

- 欄位標籤「性別」與 website 設定頁說明「暱稱、大頭貼、生日、性別」（`WS/features/account/screens/settings/SettingsPage.tsx:25`）：client 自己的畫面文字，不由契約值決定，不進目錄。
- 伺服器句子「gender 不合法」（`H/app.go:176`）、「性別不合法」（`M/internal/profile/profile.go:219`）：隨 I-server 語系化。
- REST 時期回應夾具裡的 `"gender":"male"`：`BT/shared/utils/avatar_wire_test.dart:33`、`seller-app/test/features/media/buyer_avatar_key_contract_test.dart:79`。兩者是逐字貼上的舊 REST 回應，測的是頭像欄位，沒有讀 gender［量測：讀檔］；改掉夾具裡的一個欄位等於讓「實打的回應」失真，所以不動（附錄 A R13）。它們是否該隨 `avatar_wire.dart` 一起退場，不是性別的問題。
- 新增守門規則（G10）。三元式的手寫文字表沒有守門，是 L′ 就有的缺口（`step10e-Lprime-design.md:596`、R6 `:856`），5d-2 記到計畫 10e 層級的 Pending（§7），不在本刀。

---

## 0.3 決策總表

| # | 題目 | 決定 | 依據 |
|---|---|---|---|
| G1 | 「沒有填性別」在契約裡怎麼表達 | **α（Will 2026-10-22）：`optional Gender gender = 4;`**，註解：「買家自填的性別；沒有填時不設。UpdateProfile 把 gender 列進 update_mask 而沒有設值時清除；不接受 GENDER_UNSPECIFIED。」單體：`toProfile` 只在有存值時設定；`UpdateProfile` 在 `gender` 屬於可寫欄位時，沒有設值就清除，有值就轉換，值是 `UNSPECIFIED` 回 `invalid_argument`。**同一刀修訂 `API規範:100`**（§1.2） | **業界依據（v1 寫錯，更正）**：AIP-149 說 `optional`「should only ever be used for integers and floats」；AIP-126 對零值只寫「Default value. This value is unused.」，沒有談 optional enum［外部，審查 2026-10-22 以 WebFetch 查證 google.aip.dev/149、/126］。所以 optional enum **不是** AIP 的基準；照 AIP，「不設＝零值＝沒有值」反而比較接近 β。v1 說「L-brief Q4 標 VERIFIED」也不對：Q4 只把零值那句標 VERIFIED，「用 optional」標的是 INFERENCE（`step10e-L-brief.md` Q4「業界基準」段）［量測］。**選 α 是本庫自己的取捨**：(1) Q4（Will 2026-10-17）決定 UNSPECIFIED 不給文字、「沒有」用欄位有沒有設值表達，前例 `kyc_status`（`api/hawkr/admin/seller/v1alpha1/seller.proto:916-917`，`80c2dde6a`）；(2) `API規範:102` 要求 client 把 `UNSPECIFIED` 呈現成「未知」，β 卻要把它顯示成「尚未設定」，與 :102 牴觸；(3) α 讓目錄不需要替零值找文字（G2）。代價：與 AIP-149 不同，而且 `API規範:100` 現在的寫法（「零值是合法的值…才用」）與 Q4、`kyc_status` 已經不一致，本刀把 :100 改成現行做法。兩個生成端都支援：TS 產生 `x?: T \| undefined`（`buyer_pb.ts:223`）、Dart 產生 `hasX()`／`clearX()`（`buyer.pb.dart:548-550`）［量測］；`buf lint` 在套用 α 的副本上通過［量測，審查］ |
| G2 | 目錄放哪些 key | `buyer.Gender.GENDER_MALE`、`GENDER_FEMALE`、`GENDER_OTHER`（名詞）；`buyer.Profile.gender_unset`（訊息句，description 手寫：「買家沒有填性別時，性別欄位顯示的文字；也是選單裡清除性別的那一項。」）。名詞的 description 由 `-write` 取自 proto（「Gender 是買家自填的性別。（值 GENDER_MALE）」） | 生成器擋值號 0（`G/keys.go:122-124`）［量測］，名詞函式對 UNSPECIFIED 回「沒有文字」（L′-9 第 4 點）。訊息句 key 是 `<pkg>.<Message>.<lower_snake>`（`G/keys.go:108-115`）。`common.*` 只接受 `unknown` 與 `error.<CODE>`（`G/keys.go:59-71`），「尚未設定」是這個欄位的說法，掛 `Profile`。**已試跑**：審查在 scratchpad 副本套用 α 並加四個 key 後跑 `make uitext` 等價命令，成功；TS 產生 `buyerGender(v: Gender)`、`buyerProfileGenderUnset()`，Dart 產生 `extension BuyerGenderUiText on Gender { uiText(l) }`、`buyerProfileGenderUnset(l)`，兩份 manifest 的 `inCatalog` 變成 true，description 如上［量測，審查］ |
| G3 | 兩端的領域值 | **proto `Gender`**：buyer-app `Gender?`（null＝沒有填），website `Gender \| undefined`。刪所有英文字串中間值與文字→值的反解 | 5c F6、F8；中間字串是 REST 時期「後端自由字串、枚舉由前端定」的遺留（`buyer_profile_fields.dart:16-25`），`5315cda8d` 起必要條件就不成立［量測］ |
| G4 | 選單怎麼回傳值、觸發器怎麼顯示、怎麼標示已選 | **buyer-app：`AppDropdown<T>`，對外以值、對內以 index。** 對外：`items: List<T>`、`itemLabel: String Function(T)`（必填）、`selectedItem: T?`、`onSelected: ValueChanged<T>`。對內：`_AppDropdownOverlay` 只拿到 `labels: List<String>` 與 `selectedIndex: int?`，篩選結果是原 index 的清單 `List<int>`，指到的項目是 `int? _hoveredIndex`，`onSelect(int)` 回 index，外層呼叫 `onSelected(items[i])`。`selectedIndex = items.indexWhere((e) => e == selectedItem)`（不用 `indexOf`：`List<T>.indexOf` 不收 `T?`，F5）。**觸發器的顯示，兩端同一條規則（controller 2026-10-22）**：`text = value == null ? placeholder : itemLabel(value)`、`hasValue = value != null`，沒有值時用 placeholder 樣式。觸發器與選單的已選分開算：選單裡 `indexWhere` 找到的那一項標成已選。所以性別沒有填時，觸發器是 placeholder 樣式的「尚未設定」，打開選單時「尚未設定」那一項標成已選（Will 2026-10-22）；值不在清單裡時（`Profile.city` 是自由字串，存的縣市可能不在 `counties` 裡），觸發器仍顯示它的文字、選單沒有已選項，不會被藏成 placeholder。`AppDropdown` 的預設觸發器（地址）照這條算（`app_dropdown.dart:119,122,140,156`）；`ProfileDropdownField<T>` 自己畫 `_ProfileSelectionSurface`（`profile_edit_widgets.dart:686-705`），也照這條算出 `text` 與 `hasValue` 傳給 surface，語意標籤是 `'$label，$text'`，`selectedItem: value` 交給 `AppDropdown` 只用來標已選。每一項加 `Semantics(selected: isSelected, button: true)`（外觀不變）。**website：`Dropdown` 的 `DropdownItem` 加選填 `empty?: boolean`**，表示「代表沒有值的那一項」：`value` 對到 `empty` 項時，觸發器的文字、`is-placeholder` 與浮動標籤的 `hasContent` 都當成沒有值（`Dropdown.tsx:139-156,445-451` 改看 `shown = selectedItem?.empty ? undefined : selectedItem`），選單仍以 `item.value === selectedValue` 標已選（`:657`）。性別 `value={gender === undefined ? "" : String(gender)}`，第一項 `{ value: "", label: 尚未設定, empty: true }`。其他呼叫端不設 `empty`，行為不變。website 的 `floatingLabel` 在沒有值、沒有 focus 時只顯示欄位標籤，focus 或打開時才顯示 placeholder（`:447-451`）：這是元件既有的浮動標籤樣式，文字規則與 buyer-app 相同，placeholder 何時可見沿用元件現狀（§5 #5） | v1 的設計把 `T?` 直接放進 overlay，值是 null 的「尚未設定」用長按拖選永遠選不到（審查 R7）；index 讓「沒有指到」與「指到值為 null 的那一項」在 `int?` 上自然分開，呼叫端的型別就是領域型別。v1.1 沒有定義觸發器怎麼顯示，`ProfileDropdownField` 若照 `indexOf` 推，存的縣市不在清單時會被藏成 placeholder；website G4 與 §3.2 一個用 `""`、一個用 `undefined`，互相矛盾（開工前審查 F1）［量測：讀 `Dropdown.tsx`、`profile_edit_widgets.dart`］。website 現在 `""` 會讓第一項被選取、觸發器以有值的樣式顯示（`Dropdown.tsx:139-142,445-451`）［量測］，所以要「placeholder 樣式＋選單標已選」只能讓元件分得出「沒有值的那一項」；加一個選填旗標比在呼叫端繞路小，而且是元件語意（HTML `<select>` 的 placeholder option 也是同一個概念［外部］）。業界：Flutter `DropdownMenu<T>` 以 entry 為單位，值可以是任何型別［外部］。`itemLabel` 必填：`toString()` 當預設會在 enum 上顯示 `GENDER_MALE`［推論］ |
| G5 | 選項清單從哪來 | 由生成的 enum 推導：Dart `Gender.values` 去掉 `GENDER_UNSPECIFIED`，TS `GenderSchema.values` 去掉值號 0；前面加「沒有填」那一項。順序是 proto 值號（男性、女性、其他，與兩端現在相同） | 目錄以 enum 為單位要求非零值都有文字（Q4、`G/model.go:656-725`）［量測］，推導的選項一定有文字；`Gender.values`（`buyer.pbenum.dart:53`）、`GenderSchema`（`buyer_pb.ts:1453`）［量測］ |
| G6 | 英文 | **寫進 `en.json`（Will 2026-10-22 核准文字）**：`buyer.Gender.*` 三句與 `buyer.Profile.gender_unset`。`en.json` 從 `{}` 變成只有這四個 key | **部分填寫不違反任何既有規則**（附錄 B 逐條量測）：生成器只對 `required` 語系檢查完整（`G/model.go:656-725`，`config.json` `required: ["zh-TW"]`），非 required 語系只印覆蓋率（`G/model.go:732-756`），生成器自己的測試寫明「非 required 只回報覆蓋率」（`G/build_test.go:64-66`）；L′ §1.3 :136 規定 `en.json`「只放有 message 的 key」，正是部分填寫的形狀；ADR :108「已發佈語系 100% 覆蓋（未發佈只回報）」，en 沒有發佈；ADR :55「不做缺翻譯就退回中文」是**發佈**門檻，管的是已發佈的介面，buyer-app 與 website 買家頁的已發佈語系都只有 zh-TW（`BA/core/l10n/ui_locale.dart:8`、`WS/lib/intl/locales.ts` `publishedLocales`）［量測］；ADR :104 英文由 LLM 起草、Will 審，本次即是。三個執行期的行為：Go、Dart 在生成時把 zh-TW 文字寫進 en 缺字的分支，TS 的 `en.gen.json` 與 zh-TW 合併（L′ §2.4 :263；`G/model.go:758-778` `Effective`）；golden 對每個 key 每個語系各一筆，四個 key 的 en 期望值是英文，其餘仍是 zh-TW（`G/golden.go:74-90`）［量測讀碼；推論：沒有試跑含 en 的 `make uitext`］。既有的 en 缺字測試鎖的是別的 key（Dart `packages/hawkr_ui_text/test/golden_test.dart:60-66` 用 `seller.MeasureKind.MEASURE_KIND_WARNING`；TS `tests/format.test.ts:119-143` 斷言 en 與 zh-TW 的 key 集合相同、每個值非空，合併後仍成立）［量測］，不會因此失敗［推論］。**唯一要注意的**：L′ :52 寫「L′ 是 `{}`（ADR：英文內容之後補）」，那是 L′ 當時的狀態描述，不是規則；5d-2 在 L′ 文末加一行修訂說明 en 開始有內容 |
| G7 | 送出時 update_mask | **mask 剛好是表單擁有的欄位，一律列出；null／`undefined` 只有一個意思：沒有值（清除）。** buyer-app 的表單擁有暱稱、電話、縣市、職業、性別、生日：`ProfileEdit` 六個欄位都是必填的 record 欄位（`gender: Gender?`、`birthDate: Date?`），mask 固定是 `['display_name','phone','city','job','gender','birth_date']`。buyer-app 的生日選擇器沒有清除（`profile_edit_screen.dart:704` 只在選到日期時設定），所以 `birthDate` 只在原本就沒有生日時是 null，送出的清除等於不做事［推論］。website 的表單擁有暱稱、性別、生日：`ProfileEdit = { nickname: string; gender: Gender \| undefined; birthDate: string }`（`birthDate` 空字串表示沒有值），mask 固定是 `["display_name","gender","birth_date"]`；`DatePicker` 有「清除」（`DatePicker.tsx:507-511` 呼叫 `onChange("")`），清除後儲存就清掉生日。格式錯誤的生日仍丟錯、不送出（`repo.ts:75-79`） | G7 原則（5c F4）：mask 列表單擁有的欄位。v1.1 讓 `gender: null` 是清除、`birthDate: null` 是不改，同一個型別裡兩個 null 意思相反（開工前審查 F8）；那是沿用「生日沒有清除」（`auth_repository.dart:174-175`、`repo.ts:97-99`），但單體支援清除（`H/app.go:262-270`：mask 有 `birth_date` 而沒有值就清除）［量測］。**順帶修正一個現有的缺陷**：website 現在 `birthDate: birthday \|\| undefined`（`ProfileClient.tsx:68`），清除後是 `undefined`，`birth_date` 不進 mask（`repo.ts:99-100`），所以按「清除」再儲存，生日其實沒有清掉，重新載入後又出現［量測讀碼；推論：沒有實機重現］。根 `AGENTS.md`「bug 修正可以直接做」；列在 §5 #12。`updateProfile` 只有 `ProfileClient` 一個呼叫者［量測］ |
| G8 | buyer-app 個資的 `Map<String, dynamic>` 管線 | **本刀型別化（Will 2026-10-22）**，不留 `as Gender?` 轉型。讀：`AuthRepository.getProfile()` 回 **`BuyerProfile`（`final class`，const 建構子、九個 final 欄位：`id`、`etag`、`displayName`、`avatarUrl`、`phone`、`Gender? gender`、`Date? birthDate`、`city`、`job`）**；`getMemberInfo([String? memberId])` 改名 `tryGetProfile()` 回 `BuyerProfile?`，刪沒有用的 `memberId` 參數與「歷史相容」註解。寫：`updateProfile(ProfileEdit edit)`，`ProfileEdit` 是 record（G7）。刪 `_profileMap`、`createdAt`／`updatedAt`／`avatarKey`（沒有讀者）。編輯頁：`BuyerProfile? _profile` 取代 `Map? _user` 與 session map 合併；**刪「會員等級」那一行**（Will 2026-10-22）：`ProfileAvatarEditor` 刪 `memberLevel` 參數與那個標籤（`profile_edit_widgets.dart:22,29,36,95-112`），`buyer_profile_fields.dart` 整檔刪除。讀取失敗時的畫面維持現狀（§3.1） | 從零：個資的讀寫是型別化的。v1 延後到 5e、留一個轉型，是根 `AGENTS.md`「不以『之後再清』為由留下臨時寫法」禁止的（審查 R10）。**`BuyerProfile` 用 class、`ProfileEdit` 用 record**（開工前審查 F9）：前者是跨四個 feature 的讀取模型，與地址的讀取模型 `Address`（class）同一類；後者是表單結果，與 5c 的 `AddressFormResult`（record）同一類。用普通 `final class` 不用 freezed：不需要 `copyWith`、JSON 或相等［推論］。讀個資的呼叫端 5 處：`BA/core/session/session_provider.dart:161`（暱稱）、`profile_edit_screen.dart:123`（全部）、`BA/features/settings/presentation/settings_screen.dart:222`（頭像）、`BA/shared/widgets/comments_bottom_sheet.dart:178`（頭像，經 `_pickAvatarFromMap` → `resolveAvatarKey` 讀 `avatarKey`，與 `avatarUrl` 同值，`auth_repository.dart:241-250`）、`BA/features/category/presentation/category_landing_screen.dart:130`（暱稱與頭像，另外試讀不存在的 `name`、`username`）［量測］；沒有一處 `jsonEncode` 這個 map［量測，審查］。`Date` 用 proto 的 `google.type.Date`（與 wire 同型別）；字串生日與「格式錯誤丟 `FormatException`」（`auth_repository.dart:176-184`）由型別取代。「會員等級」沒有來源（H11） |
| G9 | 不認得的 enum 值怎麼顯示 | **兩端一致：顯示「尚未設定」，也就是和「沒有填」相同。** 這不是本刀選的呈現，而是兩端的解碼器在 app 程式碼之前就把它丟掉：Connect 的 JSON 編碼把 enum 寫成值名，兩端都設 `ignoreUnknownFields: true`，不認得的值名被略過、欄位維持沒有設值。顯示函式在型別上仍要處理「有值卻沒有文字」：兩端寫 `?? commonUnknown`（L′-9 第 4 點的寫法），註解寫明在 JSON 解碼下到不了 | **v1 寫錯，更正**：v1 說 buyer-app 會顯示「未知」。量測：Dart `apiTransport` 預設 `JsonCodec()`（`packages/hawkr_api/lib/transport.dart:37`），connectrpc 2.0.0 的 `JsonCodec` 預設 `ignoreUnknownFields = true`（`connectrpc-2.0.0/lib/src/protobuf.dart:68,74`），protobuf 6.1.0 的 proto3 JSON 解碼遇到不認得的值名在 `ignoreUnknownFields` 時回 null、不設欄位（`protobuf-6.1.0/lib/src/protobuf/proto3_json.dart:336-348`；binary 解碼則放進 unknown fields，`coded_buffer.dart:69-77`，本專案不用）。website：connect-web 預設 `ignoreUnknownFields = true`（`@connectrpc/connect@2.2.0 dist/esm/protocol/serialization.js:28`），protobuf-es 2.15.0 遇到不認得的值名回 `tokenIgnoredUnknownEnum` 並略過（`@bufbuild/protobuf dist/esm/from-json.js:337-340,572-579`）［量測讀碼；推論：沒有實際送一個未知值名］。**對照 `API規範:102`**（「client 遇到 UNSPECIFIED 或不認得的值，走『未知』的呈現」）：:102 的前提是 client 看得到那個值；對 `optional` enum 欄位，JSON 解碼後「不認得」與「沒有設」無法分辨，client 沒有東西可以顯示成「未知」。要讓它顯示「未知」只能關掉 `ignoreUnknownFields`（整個回應因一個新值而失敗）或改 binary 編碼，兩者都比問題本身大。所以 :102 對 optional enum 不適用，補一句（G12，controller 2026-10-22 接受）。上線後的風險（舊版 App 把新值當成沒有填、使用者儲存時清掉）記在 Pending |
| G10 | 守門 | **不新增規則。** 進目錄後 manifest 的 `Gender` 是 `inCatalog: true`，D1／D2（Dart switch、map）與 R1／R2（TS object key、`case`）會擋以 `Gender.X` 為鍵的手寫表。三元式不在兩個工具的範圍（`check_enum_text.dart:13`、`check-enum-text.ts:16` 寫明已知漏網），那是 L′ 就有的缺口（`step10e-Lprime-design.md:596`、R6 `:856`），不是 5d 造成：本刀刪掉唯一一個 Gender 三元式，其他 client 現在也沒有以目錄 enum 值為條件、回傳顯示文字的三元式［量測，開工前審查：`rg --pcre2 "===? [A-Z]\w+\.[A-Z_]+\s*\?\s*['\"][^'\"]*\p{Han}"` 只有 admin 一處動作按鈕］。所以從 5d 移出，5d-2 記到計畫 10e 層級的 Pending：結構修法是在兩個 AST 工具各加一條三元式規則（條件是 `==`／`===` 比 `E.M`、分支有漢字），要不要另排一小刀由 Will 決定。v1.1 的觸發條件「I-CI 的逐檔漢字 ratchet」不存在：I-CI 不在主計畫（`rg -n I-CI 2026-09-29-a3-buyer-seller.md` 0 筆），只在 brief，而且逐檔 ratchet 擋不準三元式（同一個檔刪一句、加一個三元式就過）（開工前審查 F6） | 5c F8 |
| G11 | 伺服器推導 | 沒有可推導的：性別是買家輸入的值；「沒有填」由欄位有沒有設值表達 | 根 `AGENTS.md`「契約先於實作」 |
| G12 | `API規範:102` 補一句 | **接受（controller 2026-10-22，隨 Will 的 G1 α 而來）**，5d-1 無條件做：「`optional` 的 enum 欄位，JSON 解碼時不認得的值與沒有設值無法分辨，呈現同『沒有設值』。」與 :100 的修訂同一個 commit | G9 的量測。不補的話，:102 對 optional enum 寫了一件做不到的事 |
| G13 | buyer-app 地址的縣市與鄉鎮選單 | **改成 `AppDropdown<County>`／`AppDropdown<Town>`，刪「顯示名稱 → 代碼」的反查**（controller 2026-10-22）。狀態從 `String? _selectedCountyCode`／`_selectedTownCode`（`address_management_screen.dart:463-464`）改成 `County? _county`／`Town? _town`；編輯時 `_town = townByCode(a.townCode)`、`_county = _town == null ? null : countyByCode(_town.countyCode)`（`:476-478`）；縣市選單 `items: counties`、`itemLabel: (c) => c.name`、`selectedItem: _county`、`onSelected: (c) => setState(() { _county = c; _town = null; })`；鄉鎮選單 `items: _county == null ? const [] : townsByCounty(_county!.code)`、`itemLabel: (t) => t.name`、`selectedItem: _town`、`onSelected: (t) => setState(() => _town = t)`；儲存 `townCode: _town!.code`（`:517`，表單驗證已要求有鄉鎮，`:495-498`）。刪 `_onCitySelected`、`_onDistrictSelected`（`:706-723`）。`townByName`（`hawkr_twaddr.dart:45`）在 buyer-app 沒有呼叫者，本刀不碰它（套件的公開查詢） | 與性別同一種形狀：選單回傳顯示名稱，再以名稱線性反查代碼（`:707`、`:717-719`）［量測］，是 5c F8 的「以顯示文字當鍵」；`AppDropdown<T>` 泛型化之後，這兩個呼叫端本來就要改（開工前審查 F4），沿用名稱反查是一個要記錄的決定，從零的做法是選單直接回傳 `County`／`Town`。`County`、`Town` 已有值相等（`packages/hawkr_twaddr/lib/src/town.dart:16-25,39-44`）［量測］，`indexWhere((e) => e == selectedItem)` 可直接用。使用者看得到的改變：沒有（文字、順序、搜尋以 `itemLabel` 比對，與現在相同）［推論］；名稱在同一縣市內唯一，現在的反查不會選錯，所以這不是修 bug，是消掉反模式 |

## 0.4 查證過的現況（`0e990f626`）

| # | 事實 | 證據 |
|---|---|---|
| H1 | 契約：`enum Gender { UNSPECIFIED=0; MALE=1; FEMALE=2; OTHER=3 }`，`Profile.gender = 4` 沒有 `optional`、沒有註解 | [量測] `buyer.proto:150-156,165` |
| H2 | 單體：讀出時沒有存值就不設；寫入時 `UNSPECIFIED` 轉成 `""` 再清成 NULL；DB 只存 `MALE`／`FEMALE`／`OTHER`（CHECK）；seed 只有 NULL、MALE、FEMALE | [量測] `H/app.go:107-108,153-177,236-261`；`M/internal/profile/profile.go:385-391`；`M/db/schema.sql:16,28`；`backend/infra/seed-data/01-account.sql:62-66`（審查） |
| H3 | `rpc.WritableFields`：沒有 mask 時以 `Has()` 取有設值的欄位；`"*"` 取全部；明列時依 descriptor 展開，synthetic oneof 不算 branch | [量測] `backend/internal/httpapi/rpc/fieldmask.go:60-100` |
| H4 | buyer-app 讀：`_profileMap` 把 enum 轉成 `'male'` 等字串；編輯頁以 `resolveBuyerProfileGender` 取回 | [量測] `auth_repository.dart:239-266`；`profile_edit_screen.dart:167`；`buyer_profile_fields.dart:35-39` |
| H5 | buyer-app 寫：`remotePayload` 固定含 `nickname`、`phone`、`city`、`job`、`gender`，生日有值才含；repository 以字串 switch 轉回 enum，空字串變 `UNSPECIFIED` | [量測] `profile_edit_screen.dart:336-347`；`auth_repository.dart:153-206` |
| H6 | buyer-app 顯示與選擇：四個中文字的清單，選到後由中文字反解 | [量測] `buyer_profile_fields.dart:41-65`；`profile_edit_screen.dart:574-582,682-686` |
| H7 | website 讀寫：三元式往返 `"male"` 等；`gender !== undefined` 才列 mask | [量測] `WS/lib/profile/repo.ts:35,61-68,87,96,106-113`；`useAccount.ts:331,349` |
| H8 | website 顯示：placeholder 與第一項是「不公開」 | [量測] `ProfileClient.tsx:17,28,67,186-196` |
| H9 | admin 與 seller-app 沒有任何性別的顯示、對照或送出（seller-app 只有 REST 夾具） | [量測] `rg -i -c 'gender\|性別' admin seller-app/lib` 0 個檔 |
| H10 | 目錄：預設受眾 186 key，前綴只有 `common`、`seller`；`en.json` 是 `{}`；`required: ["zh-TW"]` | [量測] `U/catalog/*.json` |
| H11 | 「會員等級」一律顯示「未設定」：session map 只有 `id`、`nickname`（`profile_edit_screen.dart:117`），`_profileMap` 沒有 `level`，`resolveBuyerProfileLevel` 遇到空值回「未設定」（`buyer_profile_fields.dart:9`），不是空字串，所以元件的「一般會員」退回（`profile_edit_widgets.dart:36`）用不到 | [量測讀碼，審查複核]；沒有在實機上看 |
| H12 | 測試：`BT/features/settings/buyer_profile_fields_test.dart` 整檔是 level 與 gender（`:1-25` 的 `dart:convert`、`_buyersMeBody`、`_liveProfile` 只給 gender 用）；website `repo.unit.test.ts` 三處呼叫 `updateProfile`（`:36,:57,:68`），都沒有 gender；`BT/features/auth/buyer_profile_avatar_contract_test.dart` 以 map 呼叫 `updateProfile`（`:60,82,99,159`）並讀 map（`:45-53`）；`BT/features/auth/auth_repository_identity_test.dart:123` 呼叫 `getProfile()`；`H/app_test.go:189-201` 只測三個值的往返 | [量測] 讀檔 |
| H13 | `AppDropdown` 只收字串；呼叫端：地址兩處、`ProfileDropdownField` 一處（再被性別與縣市使用）；測試：`BT/shared/widgets/app_dropdown_test.dart`、`BT/features/settings/profile_edit_widgets_test.dart:100-130,285-330`、`BT/features/address/address_management_sheet_test.dart:61-69,107-110`（以 `tester.widget<AppDropdown>` 取元件並呼叫 `onSelected`） | [量測] `rg -n 'AppDropdown' buyer-app` |
| H14 | 讀個資的五個呼叫端與各自讀的欄位，見 G8 | [量測] |

## 0.5 約束帳

| 約束 | 判定 | 來源 |
|---|---|---|
| `API規範:100`「`optional` 只在零值合法時用」 | SELF-IMPOSED（我們的規範）；與 Q4、`kyc_status` 已不一致，本刀改成現行做法 | `API規範:100` |
| AIP-149「`optional` 只用在整數與浮點數」 | 外部基準，不是約束；本庫依 Q4 偏離，理由寫進規範 | AIP-149［外部］ |
| `API規範:102`「UNSPECIFIED 或不認得的值走『未知』」 | SELF-IMPOSED；對 optional enum 做不到（G9），G12 補一句（已接受） | `API規範:102` |
| 兩端 Connect 解碼 `ignoreUnknownFields = true` | SELF-IMPOSED（我們的 transport 設定，沿用函式庫預設），但改它的代價遠大於問題；不改 | G9 |
| 目錄不收值號 0 | 已核准的規則（Q4），生成器機械化 | `G/keys.go:122-124` |
| 兩個以上 client 的 enum 文字表要進目錄 | 已核准的規則（ADR 2026-10-17） | 根 `AGENTS.md` |
| 非 required 語系可以部分填寫 | 已核准的規則（L′ §1.3、ADR :108），G6 依它 | 附錄 B |
| `AppDropdown` 只收字串 | SELF-IMPOSED（我們的元件）；G4 | `app_dropdown.dart:30-32` |
| 地址選單以縣市、鄉鎮名稱反查代碼 | SELF-IMPOSED（我們的畫面）；G13 本刀改 | `address_management_screen.dart:706-723` |
| website `Dropdown` 分不出「代表沒有值的那一項」 | SELF-IMPOSED（我們的元件）；G4 加選填 `empty` | `Dropdown.tsx:139-156,445-451` |
| buyer-app 個資走 `Map<String, dynamic>` | SELF-IMPOSED（REST 遺留）；G8 本刀改 | `auth_repository.dart:127-266` |
| update_mask 只改 mask 裡的欄位 | **REAL**（契約語意 AIP-134，單體照它實作） | `fieldmask.go:60-93` |
| 版面、UX 文案與流程的改動要先取得同意 | 授權規則；§5 的看得到的改變都已核准或屬於跟隨契約 | 根 `AGENTS.md` |

## 0.6 決策紀錄

| 日期 | 誰 | 決定 | 本文位置 |
|---|---|---|---|
| 2026-10-21 | Will | 性別另排 5d（5c F6） | 計畫 :382 |
| 2026-10-22 | Will | 沒有填性別顯示「尚未設定」 | §5 #4 |
| 2026-10-22 | Will | G1 α：`Profile.gender` 改 `optional`，送 `UNSPECIFIED` 回 `invalid_argument`；同一刀修訂 `API規範:100` 成現行做法（零值不是合法值的 enum 用 `optional`，前例 Q4 與 `kyc_status`，寫明 AIP-149 不支持、是本庫的取捨）；修正 CARRYOVER 的 AIP-126 引用與本文的業界依據 | G1、§1.2、§7 |
| 2026-10-22 | Will | 刪除 buyer-app 編輯頁的「會員等級」；個資管線型別化併入 5d，不另排 5e、不留暫時轉型 | G8、§3.1 |
| 2026-10-22 | Will | 英文 Male／Female／Other／Not set 核准，本刀寫進 `en.json`；部分填寫要查清楚生成器與兩個執行期的行為 | G6、附錄 B |
| 2026-10-22 | Will | 選單裡「尚未設定」可以選、選了就清除，沒有填時標成已選；修 G4 的 null 問題；補長按選「尚未設定」、已選標示、隱含 mask 送 `UNSPECIFIED` 的測試 | G4、§6 |
| 2026-10-22 | controller | G12 接受：`API規範:102` 補「optional enum 的不認得值呈現同沒有設值」，隨 Will 的 G1 而來，5d-1 無條件做 | G12、§1.2 |
| 2026-10-22 | controller | 開工前審查 F1-F10：觸發器 `value == null ? placeholder : itemLabel(value)` 兩端相同，沒有值用 placeholder 樣式、選單仍標「尚未設定」已選，存的縣市不在清單也要顯示（F1）；目錄 enum 數的兩個測試（F2）；漏列的測試檔（F3）；地址縣市與鄉鎮選單本刀型別化（F4，G13）；`indexWhere`（F5）；三元式缺口移到計畫層級（F6）；G12 無條件（F7）；`ProfileEdit` 列出表單擁有的欄位、不以 null 表示不改（F8）；`BuyerProfile` 改 `final class`（F9）；刪重複的 golden 斷言（F10） | 附錄 C |

---

## 1. 契約、規範與單體

### 1.1 契約與單體

- `buyer.proto:165`：`optional Gender gender = 4;`，加 G1 的註解。三個生成端重新生成：backend `make connect`、`pnpm --filter @hawkr/api gen:api`、`packages/hawkr_api` 的 `../../api/generate.sh buf.gen.yaml`（環境見 `AGENTS.local.md`）。
- `H/app.go`：`toProfile`（`:107-108`）改成 `g := genderPB(*p.Gender); out.Gender = &g`；`genderModel`（`:165-177`）刪 `UNSPECIFIED → ""`，`UNSPECIFIED` 與不認得的值都回 `invalid_argument`；`UpdateProfile`（`:255-261`）：`has("gender")` 時 `msg.Gender == nil` 設 `in.Gender = &""`（模組清成 NULL），否則 `genderModel(*msg.Gender)`。`genderPB` 的後備（`:162`）保留：CHECK 讓它到不了，只讓 switch 完整。生日（`:262-270`）不改：mask 有 `birth_date` 而沒有值就清除，G7 依它。
- **三種 mask 模式下的行為**（H3）：明列 `gender`：如上。沒有 mask（隱含模式，以 `Has()` 取欄位）：沒有設 `gender` → 不列、不改；**明確送 `UNSPECIFIED` → 有設值、被列入、回 `invalid_argument`**，這是行為改變（現在零值在隱含模式下 `Has()` 為 false、被略過）［推論］。`"*"`：沒有設值 → 清除。
- 模組、DB、seed、legacy 不改（H2，§0）。

### 1.2 `API規範:100` 與 `:102`（Will 2026-10-22；:102 controller 2026-10-22）

- :100 改成：「`optional`：零值是合法的值、而且要與『沒有填』區分時用，例如 `optional int32 servings`；以及 enum 的『沒有值』是業務狀態（使用者沒有填、店家還沒有申請）時，例如 `optional Gender gender`、`optional KYCStatus kyc_status`，這時不接受 `UNSPECIFIED`。後者是本庫的取捨（L-brief Q4，Will 2026-10-17）：AIP-149 只建議整數與浮點數用 `optional`，AIP-126 的零值是『不使用』；我們選擇以欄位有沒有設值表達業務上的『沒有』，讓零值不承載意義、client 不必替它找文字。訊息型別（`Timestamp`）本來就能區分有沒有值。」
- :102 句末加：「`optional` 的 enum 欄位，JSON 解碼時不認得的值與沒有設值無法分辨，呈現同『沒有設值』。」

## 2. 共用文案目錄（G2、G6）

- `U/catalog/zh-TW.json` 的 `default` 加四個 key（§5）；`buyer.Profile.gender_unset` 的 description 手寫（G2），名詞由 `-write` 寫入。
- `U/catalog/en.json`：`{"default": {...四個 key 的 message...}}`，沒有 description、沒有 `staff` 段（L′ §1.3）。
- `make -C backend uitext`。**已試跑**（開工前審查，scratchpad 副本，含 en 四個 key）：輸出 `覆蓋率 en：4/190 key 有翻譯（缺的以 zh-TW 的文字補）`，生成差異 17 個檔，Dart 產生 `(UiLocale.en, 1) => 'Male'` 等分支，三份 golden 各多 8 列（4 key × 2 語系）、en 是英文；`go test ./internal/platform/uitext/... ./cmd/uitext-gen/...`、`buf lint`、`buf breaking` 都過［量測，審查］。更正：TS 的 `messages/en.gen.json` 生成時就是完整 190 key（缺的已填 zh-TW），不是執行期合併。
- **寫死目錄 enum 數的兩個測試要改成 17**（Gender 進目錄，與 en 無關）：`packages/hawkr_ui_text/test/check_enum_text_test.dart:173` `hasLength(16)`、`packages/hawkr-ui-text-ts/tests/format.test.ts:189` `toBe(16)`［量測，審查：不改各 1 個失敗］。

## 3. clients

### 3.1 buyer-app 個資

- 刪 `BA/features/settings/data/buyer_profile_fields.dart` 整檔（gender 的全部與 `resolveBuyerProfileLevel`，G8）與 `BT/features/settings/buyer_profile_fields_test.dart` 整檔（含 `:1-25` 的 `dart:convert`、`_buyersMeBody`、`_liveProfile`，只為鎖「gender 是字串」#90）。
- `BA/features/auth/data/auth_repository.dart`（G8）：新增 `final class BuyerProfile` 與 record typedef `ProfileEdit = ({String id, String etag, String displayName, String phone, String city, String job, Gender? gender, Date? birthDate})`，typedef 註解寫明「每個欄位都是表單擁有的；null 是沒有值，送出時清除」（G7）。`getProfile()` 回 `BuyerProfile`（`gender: p.hasGender() ? p.gender : null`、`birthDate: p.hasBirthDate() ? p.birthDate : null`、`avatarUrl: p.hasAvatar() ? p.avatar.uri(ImagePreset.avatar) : ''`）；`getMemberInfo` → `tryGetProfile()`；`updateProfile(ProfileEdit e)`：設 `displayName`、`phone`、`city`、`job`，`gender`、`birthDate` 非 null 才設，mask 固定 `['display_name','phone','city','job','gender','birth_date']`；刪 `_profileMap`。`updateAvatar` 不變。
- 四個讀者改讀欄位：`session_provider.dart:161`（`displayName`）、`settings_screen.dart:222`（`avatarUrl`，刪 `:220-221` 的 REST 註解）、`comments_bottom_sheet.dart:178`（`avatarUrl`，不再經 `_pickAvatarFromMap`）、`category_landing_screen.dart:130`（`displayName`、`avatarUrl`，刪 `name`／`username` 的試讀）。
- `BA/shared/widgets/app_dropdown.dart`（G4）：`AppDropdown<T>`；overlay 改收 `labels`、`selectedIndex`（`items.indexWhere((e) => e == selectedItem)`），`_filteredItems` → `List<int> _filtered`，`_hoveredItem` → `int? _hoveredIndex`，`_findItemAtPosition` 回 `int?`，`onSelect(int)`；預設觸發器的文字與樣式照 G4 的規則（`selectedItem == null ? placeholder : itemLabel(selectedItem)`，`:119,122,140,156`）；每一項包 `Semantics(selected: isSelected, button: true)`。
- `profile_edit_widgets.dart`：`ProfileDropdownField<T>`（`:662-710`，`value: T?`、`items: List<T>`、`itemLabel`、`onSelected: ValueChanged<T>`）：`text = value == null ? placeholder : itemLabel(value)`、`hasValue = value != null`，傳給 `_ProfileSelectionSurface` 與語意標籤（`:686-705`），`AppDropdown(selectedItem: value)` 只用來標已選；`ProfileAvatarEditor` 刪 `memberLevel` 與等級標籤（`:22,29,36,95-112`）。
- `profile_edit_screen.dart`：`BuyerProfile? _profile` 與 `bool _loaded` 取代 `Map? _user`；`_load`：`tryGetProfile()`，有結果時填欄位，沒有時 `_profile` 維持 null、表單照現在一樣以 session 暱稱顯示（`_loaded = true`）；刪 `memberLevel`、`resolveBuyerProfileLevel` 的呼叫、`:140-160` 的 REST 時期註解；`Gender? gender`；性別欄位 `ProfileDropdownField<Gender?>(value: gender, items: [null, ...Gender.values.where((g) => g != Gender.GENDER_UNSPECIFIED)], itemLabel: (g) => _genderText(g, l), placeholder: buyerProfileGenderUnset(l), onSelected: _selectGender)`，`l = ref.watch(uiLocaleProvider)`；`_genderText(g, l) => g == null ? buyerProfileGenderUnset(l) : (g.uiText(l) ?? commonUnknown(l))`（註解：JSON 解碼下 `??` 到不了，G9）；縣市 `ProfileDropdownField<String>(value: city.isEmpty ? null : city, items: [for (final c in counties) c.name], itemLabel: (s) => s, …)`：`Profile.city` 是自由字串，不改成 `County`（存的值可能不在清單裡，G4 保證仍顯示）。`_save` 建 `ProfileEdit`（`birthDate` 由 `DateTime` 轉 `Date(year:, month:, day:)`）；`_profile == null` 時顯示現有的「個人資料版本資訊不存在，請重新載入」（原本由 repository 丟出，`auth_repository.dart:157`，搬到畫面；文字不變）。`updateAvatar` 的 `id`／`etag` 取 `_profile`。build 的 `_user == null` 改 `!_loaded`。
- 測試跟著改型別：`BT/features/auth/buyer_profile_avatar_contract_test.dart`（`:45-53` 讀欄位；`:60,82,99,159` 改傳 `ProfileEdit`；`:76-93` 的格式錯誤生日測試刪除，型別保證；`:96-109` 的「空生日不進 mask」改成「mask 固定六個欄位、生日 null 時 `hasBirthDate()` 為 false」）、`BT/features/auth/auth_repository_identity_test.dart:123`、**`BT/core/session/session_provider_test.dart:19-32`**（fake 覆寫 `getMemberInfo([String?])` 回 Map，改成 `tryGetProfile()` 回 `BuyerProfile?`）、`BT/features/settings/profile_edit_widgets_test.dart:110-118`（`ProfileDropdownField<String>`、加 `itemLabel`、`value: ''` 改 `null`）與 `:285-330`（刪 `memberLevel:`）、**`BT/shared/widgets/app_dropdown_test.dart:28`**（`AppDropdown<String>`、加 `itemLabel`）、`BT/features/settings/profile_edit_screen_test.dart`（假 client 的 `updateProfile` 斷言跟著固定 mask）。`BT/features/notification/device_registration_test.dart:22` 的 fake 只覆寫 `currentAccountId`，不受影響［量測］。

### 3.2 buyer-app 地址（G13）

- `address_management_screen.dart`：狀態 `County? _county`、`Town? _town`（取代 `:463-464`）；初始化（`:476-478`）、表單完整的判斷（`:495-498`）、儲存（`:517`）改讀它們；兩個選單（`:597-630`）改成 `AppDropdown<County>`／`AppDropdown<Town>`，`itemLabel` 是 `name`；刪 `_onCitySelected`、`_onDistrictSelected`（`:706-723`）。placeholder「縣市」「請先選擇縣市」「鄉鎮市區」與 `enabled: _county != null` 不變。
- `BT/features/address/address_management_sheet_test.dart:61-110`：`tester.widget<AppDropdown<County>>`／`<Town>`；`onSelected(countyByCode('63000')!)`（取代 `onSelected('臺北市')`）；斷言 `selectedItem?.name`、`items.map((t) => t.name)`。

### 3.3 website

- `WS/components/ui/Dropdown.tsx`（G4）：`DropdownItem` 加 `empty?: boolean`；`const shown = selectedItem?.empty ? undefined : selectedItem`，觸發器文字、`is-placeholder`、`hasContent` 改看 `shown`（`:139-156,445-451`）；選單的 `isSelected` 不變（`:657`）；打開時捲到已選項（`:189-196`）照舊看 `selectedItem`。
- `WS/lib/profile/repo.ts`：`MemberProfile.gender?: Gender`；`fetchProfile`：`gender: profile?.gender`；匯出 `ProfileEdit = { nickname: string; gender: Gender | undefined; birthDate: string }`（G7）；`updateProfile(body: ProfileEdit, …)`：mask 固定 `["display_name", "gender", "birth_date"]`，`gender: body.gender`，`birthDate: body.birthDate ? parseBirthDate(body.birthDate) : undefined`；刪 `:97-100` 的「生日沒有清除」註解與條件。
- `WS/lib/hooks/useAccount.ts:331,349`：改用 `ProfileEdit`。
- 新檔 `WS/features/account/screens/profile/genderOptions.ts`：`GENDER_CHOICES`（G5）、`genderOptions(ut)` 回 `[{ value: "", label: ut(buyerProfileGenderUnset()), empty: true }, ...]`、`genderFromValue(v)`、`genderText(ut, g)`（`g === undefined ? 尚未設定 : optionalText(ut, buyerGender(g)) ?? ut(commonUnknown())`，G9）。
- `ProfileClient.tsx`：`useState<Gender | undefined>`；`const options = genderOptions(ut)`；`placeholder={options[0].label}`、`items={options}`、`value={gender === undefined ? "" : String(gender)}`、`onChange={(v) => setGender(genderFromValue(v))}`；送出 `birthDate: birthday`（不再 `|| undefined`，`:68`）；刪 `:67` 的註解。

## 5. 文案表（A＝已核准、C＝跟隨契約或 SSOT、E＝沿用）

使用者看得到的改變逐項列出；N 類沒有（英文 Will 2026-10-22 已核准）。

| # | 位置 | 改前 → 改後 | 類 |
|---|---|---|---|
| 1 | `buyer.Gender.GENDER_MALE`（buyer-app 選單與欄位、website 選單） | zh 男性 → 男性（兩端相同，改由目錄提供）；en（新）**Male** | zh E；en A |
| 2 | `buyer.Gender.GENDER_FEMALE` | zh 女性 → 女性；en **Female** | zh E；en A |
| 3 | `buyer.Gender.GENDER_OTHER` | zh 其他 → 其他；en **Other** | zh E；en A |
| 4 | `buyer.Profile.gender_unset`（placeholder 與選單第一項） | buyer-app「尚未設定」不變；**website「不公開」→「尚未設定」**；en **Not set** | A |
| 5 | 沒有填性別時的觸發器（兩端規則：`value == null ? placeholder : itemLabel(value)`，placeholder 樣式） | buyer-app：顯示 placeholder 樣式的「尚未設定」，與現在相同；website：沿用 `floatingLabel`，沒有 focus 時只見欄位標籤「性別」，focus 或打開時見「尚未設定」，與現在相同（文字由「不公開」改） | A（controller 2026-10-22） |
| 6 | 打開選單時，沒有填性別 | 「尚未設定」那一項沒有標示 → **標成已選**（兩端） | A（Will 2026-10-22） |
| 7 | buyer-app 選單：長按拖到「尚未設定」放開 | 改前會選到；改後仍選到並清除 | E |
| 8 | buyer-app 選單每一項的讀螢幕語意 | 沒有 → 已選的那一項唸出「已選取」（外觀不變） | C（G4） |
| 9 | buyer-app 個資的縣市：存的值不在清單裡 | 改前與改後都顯示它（G4 的規則保證不被藏成 placeholder） | E |
| 10 | buyer-app 地址的縣市與鄉鎮選單（G13） | 文字、順序、搜尋、placeholder 都不變；只是選到後不再以名稱反查 | E（沒有看得到的改變） |
| 11 | buyer-app 編輯頁頂部「會員等級」標籤（一律「未設定」） | **整個標籤刪除** | A（Will 2026-10-22） |
| 12 | website 個資：按生日的「清除」後儲存 | 改前生日沒有清掉、重新載入又出現（缺陷）→ **改後真的清除** | C（bug 修正，G7） |
| 13 | 不認得的性別值 | 兩端都顯示「尚未設定」（解碼器略過，G9）；改前 buyer-app「尚未設定」、website「不公開」 | C |
| 14 | 送出 `UNSPECIFIED` 的 API 呼叫端（沒有 UI 會這樣送） | 改前清除 → 回 `invalid_argument`，句子是現有的「gender 不合法」 | C |
| 15 | buyer-app 編輯頁讀取失敗時 | 不變：表單以登入暱稱顯示，儲存時提示「個人資料版本資訊不存在，請重新載入」 | E |
| 16 | website `ProfileClient.tsx:67` 註解「"" 代表不公開」 | 刪（不是使用者看得到的文字） | C |
| 17 | 欄位標籤「性別」 | 不變，留在各 client | E |

## 6. 測試（突變寫明測試要抓的改壞方式；型別保證另列，不算突變證據）

### 6.1 backend

- `H/app_test.go` `TestEnumMappings`：`genderModel(UNSPECIFIED)` 回 `invalid_argument`。**突變**：加回 `UNSPECIFIED → ""` 要失敗。
- `H` 的 profile 測試：(a) 存 `MALE` 讀出 `Gender != nil` 且是 `MALE`；沒有存值時 `Gender == nil`。(b) mask `["gender"]`、不設 → 讀回 nil、DB NULL。(c) mask `["gender"]`、設 `UNSPECIFIED` → `invalid_argument`、資料不變。(d) **沒有 mask**、設 `UNSPECIFIED` → `invalid_argument`；沒有 mask、不設 gender、只改暱稱 → 既有性別不變。**突變**：`toProfile` 沒有存值也設指標 → (a) 失敗；nil 分支改成不動 → (b) 失敗；`genderModel` 接受 `UNSPECIFIED` → (c)(d) 失敗。

### 6.2 目錄

- golden 自動含四個 key 的 zh-TW 與 en 各一筆（審查已量測），三端各自斷言；不另加 golden 斷言（v1.1 那條與生成物和 `golden_test.dart:60-66` 重複，刪除，F10）。**突變**：手改 `zh-TW.json` 或 `en.json` 一個字不重新生成 → `ui-text-generated-diff.yml` 紅；刪 zh-TW 的 `GENDER_OTHER` → 生成失敗；`en.json` 加一個 zh-TW 沒有的 key → 生成失敗（L′ §2.2 第 4 條）。
- **目錄 enum 數的兩個測試**：`check_enum_text_test.dart:173`、`format.test.ts:189` 改成 17（F2）。**突變**：Gender 沒有進目錄（`zh-TW.json` 漏加）→ 兩者都失敗。

### 6.3 buyer-app

- repository（新檔 `BT/features/auth/buyer_profile_gender_contract_test.dart`，用 `buyer_profile_avatar_contract_test.dart` 的假 client）：(a) `gender: FEMALE` 送出 `profile.gender == FEMALE`；(b) `gender: null` 送出 `hasGender() == false`；(c) `getProfile()` 對沒有設的回 `gender == null`、對 `OTHER` 回 `GENDER_OTHER`；(d) mask 斷言整個清單 `['display_name','phone','city','job','gender','birth_date']`，`birthDate: null` 時 `hasBirthDate() == false`。**突變**：(b) 改成設 `UNSPECIFIED` → 失敗；mask 改成「有值才列」→ (d) 失敗；(c) 沒有設時回 `UNSPECIFIED` → 失敗。
- `app_dropdown_test.dart`：(a) `AppDropdown<int?>`，`items: [null, 1, 2]`；點值為 null 的那一項，`onSelected` 收到 null。(b) **長按拖到值為 null 的那一項放開**，`onSelected` 被呼叫一次且收到 null。(c) **已選標示**：`selectedItem: null` 時第一項 `Semantics` 是 selected、其他不是；`selectedItem: 2` 時第三項 selected。(d) 兩項文字相同（`itemLabel: (_) => '同'`），`selectedItem` 是第二項時只有第二項 selected。(e) `AppDropdown<String>` 的 `selectedItem: null` 沒有任何一項 selected、觸發器顯示 placeholder；`selectedItem: '不在清單'` 時觸發器顯示「不在清單」、沒有任何一項 selected。**突變**：長按結束改成「值非 null 才選」→ (b) 失敗；選中判斷改成比文字 → (d) 失敗；觸發器改成看 `selectedIndex` → (e) 第二句失敗。（`onSelected` 改傳文字會編譯失敗：型別保證。）
- `profile_edit_widgets_test.dart`：`ProfileDropdownField<Gender?>` 的 `value: null` 時 surface 顯示 placeholder 樣式的「尚未設定」、打開後「尚未設定」那一項 selected；`ProfileDropdownField<String>` 的 `value: '某縣'`（不在 `items`）時 surface 顯示「某縣」。**突變**：`hasValue` 改成 `selectedIndex >= 0` → 第二條失敗。
- 編輯頁（`BT/features/settings/profile_edit_screen_test.dart`，F3）：假 repository 回 `FEMALE`，欄位顯示「女性」；選「尚未設定」後儲存，送出的 `gender` 沒有設值、mask 含 `gender`；回 `gender` 沒有設時「尚未設定」那一項 selected；頂部沒有「未設定」「一般會員」文字。**突變**：`itemLabel` 寫死中文的表、「尚未設定」送出 `UNSPECIFIED`、或等級標籤留著 → 失敗。
- 地址（`address_management_sheet_test.dart`，G13）：(a) 選縣市後鄉鎮選單的 `items` 是該縣市的 `Town`、`selectedItem` 是 null；(b) 選鄉鎮後儲存，表單結果的 `townCode` 是那個 `Town.code`；(c) 編輯既有地址時兩個選單的 `selectedItem` 是 `townByCode` 對應的 `County`／`Town`。**突變**：選縣市時沒有清掉 `_town` → (a) 失敗；儲存改用別的鄉鎮（例如清單第一個）→ (b) 失敗。名稱反查不存在由型別保證。

### 6.4 website

- `repo.unit.test.ts`：(a) `fetchProfile` 沒有 `gender` → `undefined`、`Gender.FEMALE` → `Gender.FEMALE`；(b) `updateProfile({..., gender: undefined})` 的 `profile.gender` 是 `undefined`；`gender: Gender.OTHER` 送 `Gender.OTHER`；(c) **生日清除**：`birthDate: ""` 時 `paths` 含 `"birth_date"`、`profile.birthDate` 是 `undefined`；(d) 既有三處呼叫（`:36,:57,:68`）改成完整的 `ProfileEdit`，`paths` 一律斷言 `["display_name","gender","birth_date"]`（`:64-73` 的「沒有生日就不進 mask」改成 (c)）。**突變**：`undefined` 改送 `Gender.UNSPECIFIED`；mask 改回「有值才列」→ (c)(d) 失敗。
- `genderOptions.unit.test.ts`：用 `uiTextFor("zh-TW")` 斷言標籤 `["尚未設定","男性","女性","其他"]`、第一項 `empty: true`、`genderFromValue(String(Gender.FEMALE)) === Gender.FEMALE`、`genderFromValue("") === undefined`；`uiTextFor("en")` 是 `["Not set","Male","Female","Other"]`。placeholder 取自 `options[0]`，同一個斷言涵蓋。**突變**：手寫「不公開」、`GENDER_CHOICES` 漏了 0 的過濾 → 失敗。
- `Dropdown` 元件測試（既有的 `WS/tests/inputFields.test.ts:107` 旁加）：`value` 對到 `empty` 項時觸發器有 `is-placeholder`、打開後該項 `aria-selected="true"`；沒有 `empty` 的項照舊以有值樣式顯示。**突變**：`shown` 改回 `selectedItem` → 第一句失敗；`isSelected` 改看 `shown` → 第二句失敗。

## 7. Commit 計畫

| # | 內容 | 審查 |
|---|---|---|
| 5d-1 | 契約（G1）、三個生成端、`H/app.go` 與測試、`API規範:100` 與 `:102`、目錄 `zh-TW.json`／`en.json` 與 `make uitext` 的生成物、兩個目錄 enum 數的測試、buyer-app（型別化個資、`AppDropdown<T>`、地址的 `County`／`Town` 選單、刪會員等級與 `buyer_profile_fields.dart`）、website（`Dropdown` 的 `empty`、性別、生日一律列 mask），同一個 commit（根 `AGENTS.md`「改契約時，同一次變更內更新唯一來源與所有消費端」；`AppDropdown<T>` 的呼叫端在同一個 commit 才編譯得過） | code-review |
| 5d-2 | 文件：計畫 10e 的 5d 完成紀錄（:382 標完成）；**計畫 10e 層級的 Pending 加一列**：「三元式的手寫文字表沒有守門（L′ 就有的缺口，`step10e-Lprime-design.md:596`、R6 `:856`）。結構修法：兩個 AST 守門工具各加一條三元式規則（`==`／`===` 比 `E.M`、分支有漢字）。觸發：Will 決定排一小刀，或 code-review 抓到一個新的三元式文字表」；L′ 文末加修訂：:209「Gender（字串 wire 值）」不成立、Gender 已進目錄，:52 的「en 是 `{}`」自 5d 起不再成立（部分填寫照 §1.3）；`M/CARRYOVER.md` 沿用盤點表補一列：「性別的『沒有填』（`GENDER_UNSPECIFIED` 當清除）｜契約要能表達沒有填｜成立，但零值承載意義，與 Q4 相反｜AIP-126 零值不使用、AIP-149 只建議數值用 `optional`［外部］；本庫依 Q4 以欄位有沒有設值表達（`API規範:100`）｜改 `optional`，`UNSPECIFIED` 回 `invalid_argument`（5d G1 α）」 | 併入完成前的 design-review |

開工前的 design-review 已做（附錄 C）；完成前再做一次。實作派 Sonnet subagent，主 session 寫派工、驗證與審查。

## 8. 驗證命令（派工前照當時的 `.github/workflows/` 再核對）

- buyer-app（`buyer-ci.yml` → `_flutter-quality.yml`）：`dart format --output=none --set-exit-if-changed lib test integration_test tool`、`flutter analyze`、`dart run custom_lint`、`bash scripts/deadcode.sh`、`packages/hawkr_ui_text` 的 `dart run tool/check_enum_text.dart ../../buyer-app/lib`、完整測試（兩組 tag）、覆蓋率 ratchet（同 5c §8）。
- website（`website-ci.yml`）：`pnpm run lint`、`pnpm run design:check --apps-root ..`、`pnpm run test:unit`、`pnpm test`、`pnpm run build`（帶 `API_BASE_URL` 與 `NEXT_PUBLIC_API_BASE_URL` 為 `http://127.0.0.1:9/api/v1`）、`pnpm run test:build-output`。
- 目錄：`ui-text-generated-diff.yml`、`ui-text-guard.yml`（`bash scripts/check-enum-text-cross.sh`）、`ts-packages.yml`、`dart-packages.yml`；`make uitext` 的覆蓋率輸出貼進驗證紀錄。
- 契約與單體：`api-contract.yml`（`buf lint`、`buf breaking`、兩個生成套件的 drift 與測試）、`backend-monolith.yml`（gofmt、`go vet`（含 `-tags integration`）、`go test ./internal/...`、`make lint-monolith`、`GOWORK=off go mod tidy` 無 diff）；`seller-ci.yml`、`admin-ci.yml` 預期只需要跑過（H9）。
- 殘留檢查：`rg -n "'male'|\"male\"|BuyerGender|buyerGender(From)?Label|resolveBuyerProfile|memberLevel|getMemberInfo|_onCitySelected|_onDistrictSelected|_selectedCountyCode|_selectedTownCode" buyer-app/lib website/lib website/features` 要沒有結果；`rg -n "Gender\.[A-Z_]+ \?|=== Gender\." website/lib website/features buyer-app/lib` 要沒有結果（三元式不在守門範圍，G10）。
- 看 CI 時看每個 job 自己的結論。

## 9. 沒有驗證的部分

- G9 的解碼行為是讀函式庫原始碼推得，沒有實際送一個未知值名給兩端。
- §1.1 隱含 mask 模式下 `UNSPECIFIED` 的行為改變是讀 `fieldmask.go` 推得，§6.1 (d) 驗證。
- G4 `AppDropdown<T>` 與 G13 的改動量是讀碼估計，沒有試改。
- G7 的 website 生日清除缺陷是讀碼推出，沒有在瀏覽器重現；§6.4 (c) 在改動前執行即是重現。
- G13「沒有看得到的改變」是讀碼推論（文字、順序、搜尋都由同一份 `counties`／`townsByCounty` 產生）。

---

## Pending

| 項目 | 觸發 | 內容 |
|---|---|---|
| 舊版 App 把新的性別值當成沒有填並在儲存時清掉（G9） | 上線後（App 版本開始落後於契約）且 `Gender` 加值 | 性別只在使用者改過時才列進 mask；或加值前先發一版能顯示新值的 App |

三元式的守門缺口不在本刀的 Pending：它是 L′ 就有的缺口，5d-2 記到計畫 10e 層級（§7、G10）。

---

## 附錄 A：審查意見的處理（v1 → v1.1；審查全文在 session scratchpad `5d-design-review.md`）

| # | 意見（嚴重度） | 處理 |
|---|---|---|
| R1 | G1 的業界依據錯：AIP-149 只建議數值用 `optional`，AIP-126 沒談 optional enum；Q4 對 optional 標的是 INFERENCE（中） | **改**：G1 依據重寫，明說 optional enum 不是 AIP 基準、是本庫依 Q4 的取捨；§0.5 加 AIP-149 一列；5d-2 的 CARRYOVER 列改掉 AIP-126 引用 |
| R2 | G8 寫三個讀者，實際 5 處（低） | **改**：G8、H14 列出 5 處與各自讀的欄位 |
| R3 | §9 的 G2 已驗證（低） | **改**：G2 標審查的試跑結果，§9 刪掉那一點 |
| R4 | `API規範:100` 與 Q4、`kyc_status` 牴觸，:102 與 β 牴觸（高） | **改**：G1 正面列出牴觸；§1.2 修訂 :100；:102 另立 G12（v1.2：已接受） |
| R5 | α 之後隱含 mask 模式明確送 `UNSPECIFIED` 會回 `invalid_argument`（中） | **改**：§1.1、§6.1 (d)、§5 #14 |
| R6 | G2 生成器支援已試跑（資訊） | **改**：G2 寫入試跑結果 |
| R7 | `AppDropdown<T>` 在 `T` 可為 null 時長按選不到 null 項（高） | **改**：G4 內部改用 index；§6.3 (b) |
| R8 | 「尚未設定」沒有填時會標成已選（中） | **改**：Will 決定要標；§5 #6；§6.3 (c) |
| R9 | `address_management_sheet_test.dart` 要改型別（中） | **改**：§3.2（v1.2 改成 `County`／`Town`） |
| R10 | G8 延後並留轉型是技術債（中） | **改**：G8 併入本刀 |
| R11 | `buyer_profile_fields_test.dart` 只刪一段會留下沒用的夾具（中） | **改**：整檔刪除 |
| R12 | `repo.unit.test.ts` :57、:68 也要補 gender（低） | **改**：§6.4 (d) |
| R13 | REST 夾具 `"gender":"male"`（低） | **記**：不改，§0.2 寫明理由 |
| R14 | 「同文字點第二個」抓不到選中判斷的突變（中） | **改**：`Semantics(selected:)`，§6.3 (c)(d) |
| R15 | 型別保證不是突變（低） | **改**：§6 分開列 |
| R16 | 缺長按選 null 項、隱含 mask 送 `UNSPECIFIED` 的測試（中） | **改**：§6.3 (b)、§6.1 (d) |
| R17 | website placeholder 沒被測到（低） | **改**：placeholder 取自 `genderOptions(ut)[0]` |
| R18 | G10 沒寫三元式不被守門（低） | **改**：G10 寫明（v1.2 移到計畫層級，附錄 C F6） |
| R19 | G9 對 Dart 不認得值的說法錯（低） | **改**：G9 重寫，兩端一致顯示「尚未設定」 |
| R20 | 文案表漏了已選標示與不認得值（中） | **改**：§5 |
| R21 | 會員等級的決定屬於 5d（中） | **改**：Will 決定刪除；G8、§5 #11 |

## 附錄 B：`en.json` 部分填寫的查證（Will 決定 3）

| 檢查 | 結果 | 證據 |
|---|---|---|
| 生成器的完整性檢查 | 只對 `required` 語系（zh-TW）；en 缺 key 不報錯 | [量測] `G/model.go:656-725`；`G/build_test.go:64-66`「required 的語系要完整；非 required 只回報覆蓋率」 |
| 覆蓋率 | 非 required 語系印覆蓋率；試跑輸出 `en：4/190` | [量測] `G/model.go:732-756`；開工前審查試跑 |
| en 不能多出 key、不能有 staff | 本刀四個 key 都在 zh-TW、沒有 staff | [量測] L′ §2.2 第 4 條（:234）、§1.3（:133-136） |
| Go、Dart 執行期 | 生成時把 zh-TW 文字寫進 en 缺字的分支；有英文的 key 輸出英文 | [量測] `G/model.go:758-778` `Effective`；試跑的 Dart 生成物有 `(UiLocale.en, 1) => 'Male'` |
| TS 執行期 | `en.gen.json` 生成時就是完整 190 key（缺的填 zh-TW）；四個 key 是英文 | [量測] 開工前審查試跑；`tests/format.test.ts:119-143` 斷言 key 集合相同、值非空，試跑通過 |
| golden | 每個 key 每個語系一筆；四個 key 的 en 期望英文 | [量測] `G/golden.go:74-90`；試跑三份 golden 各多 8 列；既有的 en 缺字斷言（`golden_test.dart:60-66`）通過 |
| 其他測試 | 寫死目錄 enum 數的兩個測試會失敗（與 en 無關，是 Gender 進目錄） | [量測] 試跑；§2、§6.2 |
| ADR 與 L′ 的規則 | **沒有違反**：L′ §1.3「只放有 message 的 key」是部分填寫的形狀；ADR :108「未發佈只回報」；ADR :55 的「不做缺翻譯就退回中文」是發佈門檻，en 在兩個介面都沒有發佈；ADR :104 英文由 LLM 起草、Will 審 | [量測] 讀 L′、ADR。L′ :52「L′ 是 `{}`」是狀態描述，5d-2 加修訂 |

## 附錄 C：開工前 design-review 的處理（v1.1 → v1.2；全文在 session scratchpad `5d-design-review-start.md`）

| # | 發現（嚴重度） | 處理 |
|---|---|---|
| V1 | 附錄 B 成立；試跑 `make uitext` 含 en 四個 key 成功，`en：4/190`；TS `en.gen.json` 是生成時合併 | **改**：§2、附錄 B 換成試跑結果與更正 |
| F1 | 沒有值時觸發器的顯示沒有定義，website G4 與 §3.2 矛盾；`ProfileDropdownField` 若照 `indexOf` 推，存的縣市不在清單會被藏起來（中） | **改**（controller 決定）：G4 定義兩端同一條規則 `value == null ? placeholder : itemLabel(value)`，沒有值用 placeholder 樣式，觸發器與選單的已選分開算；`ProfileDropdownField<T>` 的 surface 照這條；website `Dropdown` 加選填 `empty`，性別用 `""`＋`empty` 項；§3.1、§3.3、§5 #5、#6、#9；§6.3、§6.4 測試 |
| F2 | 寫死目錄 enum 數的兩個測試會失敗（中） | **改**：§2、§6.2 改成 17 |
| F3 | 漏列測試檔：`session_provider_test.dart:19-32`、`app_dropdown_test.dart:28`、`profile_edit_widgets_test.dart:110-118`、編輯頁測試的檔案（低） | **改**：§3.1 列入；編輯頁測試寫明在 `profile_edit_screen_test.dart`；`device_registration_test.dart` 查過不受影響 |
| F4 | 地址的縣市與鄉鎮沿用名稱反查代碼（低） | **改**（controller 決定）：新增 G13、§3.2、§6.3 地址測試與突變、§7；沒有使用者看得到的改變（§5 #10） |
| F5 | `items.indexOf(selectedItem)` 編譯不過（低） | **改**：`indexWhere((e) => e == selectedItem)`（G4、§3.1） |
| F6 | 三元式的 Pending 放錯地方，觸發條件 I-CI 不存在（低） | **改**：從 5d 的 Pending 移除，5d-2 寫進計畫 10e 層級，寫明是 L′ 的既有缺口、結構修法是兩個 AST 工具加規則、由 Will 決定是否排刀；刪 I-CI 觸發（G10、§7） |
| F7 | G12 已接受，文件仍寫「待定」「若核可」（低） | **改**：狀態列、G12、§0.6、§1.2、§7 全部改成無條件 |
| F8 | `ProfileEdit` 的兩個 null 意思相反（低） | **改**：G7 改成 mask 剛好是表單擁有的欄位、一律列出、null 只表示沒有值；buyer-app 生日沒有清除所以 null 只在本來沒有生日時出現；website 有「清除」，順帶修正清除不生效的缺陷（§5 #12、§6.4 (c)） |
| F9 | `BuyerProfile` 用 9 欄位的 record（低） | **改**：`final class`（照 `Address`），`ProfileEdit` 保持 record（G8） |
| F10 | §6.2 新增的 Dart golden 斷言重複（低） | **改**：刪除（§6.2） |

---

## 實作時的修訂

5d-1 實作之後，與 v1.2 不同的地方（驗證輸出在 session scratchpad `5d-impl-report.md`）：

- 5d-1：§6.1 的單體測試寫在 handler 單元層（`H/app_test.go`：`TestEnumMappings` 加 `UNSPECIFIED` 的斷言、新增 `TestToProfileGenderIsSetOnlyWhenStored`、`TestUpdateProfileGender` 五個子案例），以記下輸入的假模組驗 `in.Gender`，沒有新增打 DB 的整合測試。理由：值的轉換與三種 mask 模式都在 handler，DB 端的 NULL 清除是模組既有行為（`TestProfilePartialUpdateAndPendingWrites` 與 `profile_test.go` 已涵蓋）。「`UNSPECIFIED` 回 invalid_argument」的斷言讀的是傳輸層錯誤的 code 前綴（`rpc.Error` 回的不是 `*connect.Error`，要到 interceptor 才轉換），測試檔裡的 `isInvalidArgument` 做這件事；加一個「不經過模組」的斷言（`calls == 0`）。
- 5d-1：`AppDropdown` 每一項的 `Semantics` 帶 `ValueKey('app-dropdown-item-$index')`（index 是原清單的位置，篩選後仍唯一），測試用它找已選標示；設計只寫「每一項加 `Semantics(selected:, button:)`」。
- 5d-1：`_ProfileSelectionSurface` 的參數由 `value`＋`placeholder` 改成算好的 `text`＋`hasValue`（G4 的規則放在 `ProfileDropdownField` 與 `ProfileSelectionField` 兩個呼叫端，surface 只畫）；`ProfileSelectionField` 的外觀與語意標籤不變。
- 5d-1：website 的 `Dropdown` 多匯出一個純函式 `resolveDropdownSelection(items, value)`，回 `{ selected, shown }`（G4 的 `selectedItem` 與 `shown`）。原因：website 沒有 DOM 測試環境（`inputFields.test.ts` 用 `renderToStaticMarkup`），選單的 `aria-selected` 要開啟後才渲染，測不到；改成：SSR 標記斷言觸發器（`is-placeholder`、placeholder 文字）、純函式斷言選單標已選的是 `selected` 而不是 `shown`。§6.4 的「打開後該項 `aria-selected="true"`」因此沒有逐字實作，用這個函式的 `selected` 取代。
- 5d-1：website 的 `genderOptions.ts` 的 `GENDER_CHOICES` 是 `GenderSchema.values` 去掉值號 0，`genderText` 對沒有文字的值回 `commonUnknown`（G9）；`ProfileClient` 以 `useMemo` 算 `options`，`placeholder` 取 `options[0].label`。
- 5d-1：`updateProfile`（website）收到的 `birthDate` 空字串仍進 mask 而且不設值（G7）；`useAccount.ts` 的兩處型別改成 `ProfileEdit`。
- 5d-1：buyer-app `ProfileEditScreen._load` 在讀不到個資時把 `_loaded` 設成真、`_profile` 維持 null，暱稱用 session 的暱稱（§3.1）；`_save` 在 `_profile == null` 時丟 `ServerException('個人資料版本資訊不存在，請重新載入')`，文字與原本 repository 丟的相同。生日由 `Date` 轉 `DateTime` 時，年月日有任何一個是 0 視為沒有（與 `formatDate` 的規則一致）。
- 5d-1：`profile_edit_screen_test.dart` 的 `pumpScreen` 加 `gender`、`locale` 兩個參數；英文語系的案例用 `uiLocaleProvider.overrideWithValue(UiLocale.en)`，驗證文字來自目錄（Female／Not set／Male／Other）。§6.3 沒有寫英文案例，這是為了讓「寫死中文的表」的突變抓得到。
- 5d-1：`buyer_profile_avatar_contract_test.dart` 刪「格式錯誤的生日」測試（型別保證，§3.1），改成「mask 固定六個欄位」與「沒有生日仍列 `birth_date`、不設值」。
- 5d-1：`scripts/legacy-endpoints-baseline.json` 跟著更新（刪掉 `buyer_profile_fields_test.dart`，company-domain 1 → 0）；`check-legacy-endpoints.py` 不更新就紅。
- 5d-1：§8 的 `rg` 殘留檢查第一條的結果不是空：`buyer-app/lib/shared/widgets/top_navigation.dart` 有 `memberLevel` 參數。完成前 design-review 查出整個 `TopNavigation`、`buildTopNavigationAppBar`、`TopNavStyle`、`kTopNav*` 都是沒有呼叫端的死碼，`hygiene_policy.json` 那筆具名例外的移除條件已成立，所以不只刪參數：另案以 `refactor(buyer-app)` 刪除（保留 `NotificationBellButton`），不在 5d 的 commit 裡。

修正輪（完成前 design-review 與 code-review 之後）：

- 5d-1（DR2）：「年月日有任何一個是 0 就視為沒有」的規則只留一份，在 `packages/hawkr_api/lib/dates.dart`：新增 `isDateSet`、`dateToDateTime`、`dateFromDateTime`，`formatDate` 改用 `isDateSet`；編輯頁不再自己寫那條規則與 `Date(...)` 的組裝。理由：放在 `_toBuyerProfile` 會讓 `BuyerProfile.birthDate` 的語意變成「已正規化」，但 `ProfileEdit` 反方向的 `DateTime → Date` 仍要有地方放，且 TS 端的 `proto/date` 也是共用套件做這件事。`dates_test.dart` 與編輯頁的生日案例（完整生日原樣送回、缺欄位的生日視為沒有）驗證；把 `dateToDateTime` 對部分日期的處理改壞，編輯頁測試變紅。
- 5d-1（DR3）：刪掉 `backend/internal/modules/buyer/AGENTS.md` 新增的那一條：它描述的是傳輸層（`httpapi/buyer`）的行為，與 proto 註解重複，proto 註解才是唯一來源。實際動到的指令檔與文件：`buyer-app/AGENTS.md` 新增「下拉選單」慣例（`AppDropdown<T>`／`ProfileDropdownField<T>` 以領域值為單位，不以顯示文字當鍵再反查）；`M/CARRYOVER.md` 沿用盤點表新增性別「沒有填」一列（設計 §7 排在 5d-2，隨 5d-2 提交）。
- 5d-1（L1）：`app_test.go` 的 `isInvalidArgument` 沒有改成 `connect.CodeOf`：`rpc.Error` 回的是 `*transportError`，不是 `*connect.Error`，`connect.CodeOf` 對它回 unknown（試過，三個案例變紅）；經 `rpc.NewInterceptor` 轉換又需要 procedure 與呼叫者，單元測試不經過它。保留字串前綴判斷，與 `seller/app_test.go`、`kyc_test.go` 的既有寫法相同，並在函式上說明原因。
- 5d-1（L2）：規範 :102 補的那句（G12）是「optional enum 的不認得的值呈現同沒有設值」，所以兩端一致改成顯示「尚未設定」，不顯示「未知」：buyer-app 的 `_toBuyerProfile` 與 website 的 `fetchProfile`（`knownGender`）在讀取邊界把 `GENDER_UNSPECIFIED` 與不認得的值當成沒有填（null／`undefined`），不認得的值因此不會被原值寫回；顯示函式在值沒有文字時回 `gender_unset`，不再用 `commonUnknown`。改了 G9 的「顯示函式 `?? commonUnknown`」寫法（G9 的結論「兩端顯示尚未設定」不變）。測試：buyer-app 的 gender contract（UNSPECIFIED 讀成 null）、編輯頁（伺服器回 UNSPECIFIED 顯示「尚未設定」、沒有「未知」）；website 的 `repo.unit.test.ts`（UNSPECIFIED 與 99 讀成 `undefined`）、`genderOptions.unit.test.ts`（沒有文字的值同沒有值）；各做一個突變見紅。
