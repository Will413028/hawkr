# 刀 L′ 實作設計 v2：共用文案目錄（多語系形狀）

**Will 的決定（2026-10-17）：§7 的文案全照建議**（(a) 14 句伺服器原因句、(b) SellerRequirement 4 個名詞、(c) select 後備文字、(d) 停權期間句兩端保留）。設計由一個 subagent 起草、另一個審查、第三個依審查修訂。

基準：worktree `<10e worktree>`，branch `local/10e`，HEAD `8f7f6ed11`。只讀調查，沒有改 repo。
SSOT（Will 2026-10-17）：ADR `backend/docs/adr/2026-10-17-multi-locale-architecture.md`（下稱 ADR）、`backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-i18n-brief.md`（I2、I3、I4、I7）、`step10e-L-brief.md`（Q1、Q3–Q5 照建議，Q2 被 i18n 第 4 題取代，Q6／Q7 照 I7 表）、`step10e-labels-brief.md`（D-L1 c 被 b′ 取代；D-L2 不變）、`step10e-L0-copy.md`（D1–D7 全照建議，下稱 L0）、術語表 `backend/docs/開發規範/使用者面用語術語表.md`。
本文是實作者唯一要讀的文件：v1 與審查的結論都已併入，§11 是相對 v1 的變更紀錄。

讀法：§0 現況，§1–§6 設計，§7 **待 Will 核可的文案**（L′-1 之前要定），§8 子 commit 計畫（實作者照這一節做），§9 已定的工程決定（原 v1 §8 的分流結果），§10 風險與 10d 協調，§11 變更紀錄。

---

## 0. 查證過的現況（設計依賴的事實）

| 事實 | 證據 |
|---|---|
| `api/` 有 60 個 enum；賣家端原因 enum 是 `repeated enum`：`Measure.remediation_unavailable_reasons = 19`、`appeal_unavailable_reasons = 22`、`GetMyStoreOpeningResponse.unavailable_reasons = 2`；`Measure` 最大欄位號 22，`reserved 9, 14` 已存在；`GetMyStoreOpeningResponse` 只有 1、2 | `api/hawkr/seller/v1alpha1/seller.proto:456-461,1223-1270` |
| admin 的 3a 先例：`MeasureActionUnavailability { reason = 1; google.rpc.LocalizedMessage message = 2; }` | `api/hawkr/admin/seller/v1alpha1/measure.proto:155-174` |
| 3a 的 Go 文字函式經模組根套件轉出，handler `toActionUnavailabilities` 呼叫它 | `backend/internal/modules/seller/governance.go:247-249`；`backend/internal/httpapi/seller/governance_measures.go:60-70` |
| 賣家端錯誤句與可用性原因是兩份：`errAppealUnavailable`（`measure/appeal.go:19-35`）、`errRemediationUnavailable`（`measure/remediation.go:17-36`）、`OpeningBlocker.Err()`（`model/model.go:117-130`，STORE_EXISTS 回 **AlreadyExists**，其他 FailedPrecondition）各自寫字面值；handler 只轉 enum（`httpapi/seller/measures.go:162-170`、`app.go:171-173`） | 同左 |
| 原因的優先序：Appeal＝REVOKED、ALREADY_APPEALED、APPEAL_WINDOW_ENDED、REQUEST_PENDING；任何申覆存在就先出現 ALREADY_APPEALED，所以 REQUEST_PENDING 排第一時一定是待處理的解除請求。Remediation＝NOT_RISK、NOT_ACTIVE、APPEAL_PENDING／REMEDIATION_PENDING、COOLDOWN | `model/measure.go:877-917,576-596` |
| 開店擋下的入口：`model.OpeningBlock`（`onboarding/onboarding.go:102`）、可用性 `Module.StoreOpening`（`seller.go:424`） | 同左 |
| `rpc.LocalizedMessage(text)` 固定 `errorLocale = "zh-TW"`；`fromAppErr` 是唯一產生 `FieldViolation` 的地方，每條都帶 `LocalizedMessage` | `backend/internal/httpapi/rpc/errors.go:28-34,145-170` |
| 兩個 client 在 `FieldViolation` 沒有 `localized_message` 時退回 `description` | `packages/hawkr-api-ts/src/connect/errors.ts:155-158`；`packages/hawkr_api/lib/errors.dart:97-105` |
| Connect code 預設文字 TS 與 Dart 各一份，逐字相同；`submit.ts:33`、`submit.dart:58` 末端寫死「系統錯誤」。TS `submitError(err, method?)` 已有第二參數；Dart `submitError(error, {idempotency, request})` | `errors.ts:95-125`；`errors.dart:199-217`；`submit.ts:18`；`submit.dart:40-59` |
| 錯誤 helper 的呼叫點：TS 38 處（website＋admin 21 檔），Dart 23 處（seller-app 7 檔、buyer-app 3 檔） | `rg -n '\b(submitError\|userMessage\|defaultErrorText)\(' website admin -g '!**/*.test.*'`；同式對 `seller-app/lib buyer-app/lib` |
| 模組的 enum 是去前綴的字串，轉換在 `rpc.EnumToProto` | `backend/internal/httpapi/rpc/enums.go:12-66` |
| 伺服器手寫名詞表：`terminationGroundLabels`、`riskGroundLabels`、`Label()`；regex 綁定 `TestRiskGroundLabelsEqualTheProtoLabels` | `rg -n 'Label\(\)' backend/internal --type go` |
| website 的 next-intl 是空殼：`i18n/request.ts` 會 `await requestLocale`、一律回 `defaultMessages`；`getTranslations`／`useTranslations` 0 次；`app/layout.tsx:195` 把 `defaultMessages` 直接交給 client provider；next-intl 4.13.0，底層 `use-intl` 4.13.0、`intl-messageformat` 11.2.8 | `rg -n 'useTranslations\|getTranslations' website`；`pnpm-lock.yaml:14428,15956-15961` |
| protoc_plugin 的 enum 是 `class X extends ProtobufEnum`，不是 Dart `enum`：switch 不檢查窮舉、要 `_` 分支；名稱可能被 `omit_enum_names` 拿掉。ADR D2「窮舉由 Dart `switch` 保證」因此不精確 | `packages/hawkr_api/lib/gen/hawkr/seller/v1alpha1/seller.pbenum.dart:882-888` |
| `intl 0.20.2`：兩支 App 都解析到（transitive）；seller-app 經 `flutter_localizations`（釘 0.20.2），buyer-app 沒有依賴 `flutter_localizations`，經其他套件帶入 | `seller-app/pubspec.lock:1083-1090`；`buyer-app/pubspec.lock` |
| `kyc_status` 以 UNSPECIFIED 承載「尚未申請」只在 `admin.seller.v1alpha1.FoodRegistration.kyc_status = 14`；handler 只在非 nil 時設；integration test 斷言 `GetKycStatus() == UNSPECIFIED` | `api/hawkr/admin/seller/v1alpha1/seller.proto:832-833`；`backend/internal/httpapi/seller/foodreg.go:122-124`；`admin_integration_test.go:816` |
| 同名 enum 只有 `FoodRegistrationState`（admin 與 seller 各一）；另有巢狀 enum `identity.v1alpha1.<Message>.DeletionState` | `api/hawkr/identity/v1alpha1/identity.proto:81` |
| AddressLabel：只有 buyer-app 顯示（`address_repository.dart:30-48`，含用中文字反解的 `_labelOf`）；website 地址用 `"home"/"company"/"other"` 字串、沒有顯示「家／公司」的文字 | `website/lib/addresses/repo.ts:46-66` |
| website 與 seller-app 的時間格式逐字相同：`YYYY/MM/DD HH:mm`（台北）；website 另有 `formatTaipeiDateTimeCeilMinute`，seller-app 的冷卻用手寫進位；日期 `YYYY/MM/DD`：website `formatTaipeiDate`，seller-app 食品登錄用 `_date`（**裝置時區**，不是台北） | `website/lib/date/taipei.ts:205-211,244-250,312-318`；`packages/hawkr_api/lib/taipei_time.dart:16-26`；`seller-app/lib/features/wallet/presentation/food_registration_form.dart:242-246` |
| `twtime` 目前只有 `Taipei` | `backend/internal/platform/twtime/twtime.go` |
| KYC 銀行欄位：website 是一個控制項（選銀行同時設 `bankName`、`bankCode`）；seller-app 是兩個控制項（「金融機構代碼」「銀行名稱」）；proto 順序 BANK_NAME=7、BANK_CODE=8 | `CertificationsClient.tsx:1854-1865`；`kyc_wizard_steps.dart:486-505`；`seller.proto:648-649` |
| Dart 套件 CI 讀套件的 `.fvmrc`、format 只檢查 `lib test`；paths `packages/hawkr_*/**` | `.github/workflows/dart-packages.yml` |
| `_flutter-quality.yml` 的 hygiene step 在 App 目錄跑 `scripts/deadcode.sh`；buyer-app 沒有 Makefile | `.github/workflows/_flutter-quality.yml:58-63`；`ls buyer-app/Makefile` 不存在 |
| `api/generate.sh` 先比對 `buf --version` 與 `api/.buf-version` | `api/generate.sh:12-17` |
| 先例：twaddr 一支 Go 程式輸出三份、drift workflow | `backend/internal/platform/twaddr/generate.go:1-6`；`.github/workflows/twaddr-generated-diff.yml` |

---

## 1. 目錄的來源、結構與擁有者

### 1.1 位置與擁有者

| 路徑 | 內容 | 擁有者 |
|---|---|---|
| `backend/internal/platform/uitext/catalog/zh-TW.json` | 來源語系（`message`、`description`、`descriptionOverride`、`note`、`params`、`staff` 段、`variants` 段） | 目錄（跨端） |
| `backend/internal/platform/uitext/catalog/en.json` | 英文；只放有 `message` 的 key，L′ 是 `{}`（ADR：英文內容之後補） | 同上 |
| `backend/internal/platform/uitext/catalog/config.json` | `{"sourceLocale":"zh-TW","locales":["zh-TW","en"],"required":["zh-TW"]}` | 同上 |
| `backend/internal/platform/uitext/`（package `uitext`） | Go 執行期：`Locale`、`Text`、context 縫、複數規則、生成的 `catalog_gen.go`、共用 golden `testdata/golden.gen.json` | platform（不得依賴模組；`internal/arch` 已管） |
| `backend/internal/platform/uitext/icu/` | ICU MF1 受限子集的 parser、AST 與參考格式器（golden 的期望值由它算） | platform |
| `backend/cmd/uitext-gen/` | 生成器（`package main`，開發工具，不部署） | backend |
| `packages/hawkr-ui-text-ts/`（`@hawkr/ui-text`） | TS 生成物＋少量手寫；website、admin 共用 | 共用套件 |
| `packages/hawkr_ui_text/` | Dart 生成物＋手寫 `UiLocale`；兩支 App 共用；純 Dart | 共用套件 |
| `backend/internal/modules/seller/internal/text/{zh-TW.json,en.json,text_gen.go}` | seller 模組自己的伺服器句子（ADR D3、I3）；L′ 只有 D-L2 的原因句 | 模組 |

Go 的 package 叫 `uitext`，不叫 `copy`（L-brief Q6）。

### 1.2 key

照 ADR D2：不帶版本的 descriptor 路徑，`hawkr.` 前綴與版本段都去掉。

| 種類 | 文法 | 例 |
|---|---|---|
| enum 值的名詞 | `<pkg>.<型別路徑>.<VALUE>` | `seller.MeasureKind.MEASURE_KIND_WARNING`；巢狀：`identity.<Message>.DeletionState.<VALUE>` |
| 依值的句子（變體） | `<pkg>.<Enum 型別路徑>.<variant>.<VALUE>`，`variant` 是 lower_snake | `seller.KYCFlowStatus.gate_message.KYC_FLOW_STATUS_SUBMITTED` |
| 訊息句（不屬於單一 enum 值、只由契約欄位決定的句子） | `<pkg>.<Message 型別路徑>.<name>`，`name` 是 lower_snake | `seller.Measure.termination_notice` |
| 通用 | `common.unknown`、`common.error.<CODE>`（Connect code 的 proto 名） | `common.error.RESOURCE_EXHAUSTED` |

解析規則（生成器實作，不靠大小寫切段）：
1. 從 image 建出「去掉 `hawkr.` 與版本段的 package 名」→ package 的表；key 的開頭取**最長相符**的 package（`admin.seller` 優先於 `admin`）。
2. 其後的段逐一在該 package 的 descriptor 裡往下找巢狀型別，直到不再是型別名；最後找到的型別決定種類：enum → 剩下的是 `<VALUE>` 或 `<variant>.<VALUE>`；message → 剩下的是一段 `<name>`。
3. 剩下的段數或型別不合就失敗。

next-intl 的 key 不能含 `.`（是巢狀分隔），生成的 messages 是巢狀物件；同一個節點下值名（UPPER_SNAKE，buf lint 保證）、變體與訊息句（lower_snake）、巢狀型別（UpperCamel）不會撞名，生成器仍檢查同一節點沒有重複名稱。
生成器擋：兩個 package 去掉版本後同名（`v1alpha1` 與 `v1` 並存）、變體名或訊息句名不是 lower_snake、key 指向不存在的型別或值、key 指向值號 0。

### 1.3 來源檔格式

```json
{
  "default": {
    "seller.MeasureKind.MEASURE_KIND_WARNING": {
      "message": "警告",
      "description": "（-write 由 proto 註解產生）"
    },
    "seller.MeasureState.MEASURE_STATE_ACTIVE": {
      "message": "{kind, select, MEASURE_KIND_TERMINATION {合作已終止} other {停權中}}",
      "params": { "kind": { "enum": "seller.MeasureKind" } },
      "note": "kind 是同一筆措施的 MeasureKind。"
    },
    "seller.Measure.title_risk": {
      "message": "風險：{ground}",
      "params": { "ground": { "noun": "seller.RiskGround" } },
      "description": "依據是風險類型的措施的標題。ground 是風險類型的名詞（同語系、同受眾）。"
    },
    "seller.Measure.rejection_cooldown_warning": {
      "message": "{days, plural, other {若本次經駁回，將於 {days} 日內不得再提出解除請求}}",
      "params": { "days": { "type": "integer" } },
      "description": "…"
    },
    "common.unknown": { "message": "未知", "description": "不認得的值與 UNSPECIFIED 一律顯示這個字。" }
  },
  "staff": {
    "seller.KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED": { "message": "待審核" }
  },
  "variants": {
    "seller.KYCFlowStatus.action": {
      "omit": { "default": ["KYC_FLOW_STATUS_APPROVED"] },
      "description": "開店進度卡與認證入口引導賣家做哪一步。"
    },
    "seller.MeasureRequestState.with_kind": {
      "omit": { "default": ["MEASURE_REQUEST_STATE_CLOSED"], "staff": [] },
      "description": "請求的種類與狀態合成的狀態標籤。"
    }
  }
}
```

規則：

- `message`：ICU MF1 受限子集（§1.4）。
- `params`：只在來源語系宣告；其他語系的參數集合必須相同。型別：
  - `{"enum": "<型別路徑>"}`：只能用在 `select`，分支是該 enum 的值名＋`other`。
  - `{"noun": "<enum 型別路徑>"}`：**名詞參數**，訊息裡寫 `{ground}`；各端生成碼以**同一語系、同一受眾**的目錄名詞填入。參數值是 UNSPECIFIED 或不認得時，整個建構函式回「沒有文字」（TS `undefined`、Dart `null`、Go `("", false)`），由呼叫端用自己的後備 key。這不是拼接：整句的模式是一則訊息，譯者可以調整語序；名詞是完整的名詞片語，用在標籤位置（「風險：{ground}」「措施：{kind}」）。句中需要依值改寫法的，用 `select`，不用名詞參數（審查 D4）。
  - `{"type": "string"}`：已格式化的字串（日期、時間、使用者或管理員輸入的文字、client 的動作名稱）。
  - `{"type": "integer"}`：整數。plural 依它選分支；顯示一律寫 `{days}`，三端都輸出十進位 ASCII 數字、不分組（TS 的 `use-intl` 對簡單參數用 `String(value)`，由 golden 測試鎖住）。
- `description`：proto 有註解的 key（enum 值的名詞、變體、staff），`-write` 寫成 `descriptionOverride ?? proto 註解`，再接 `note`；`staff` 段附「受眾：後台員工」。沒有 proto 來源的 key（訊息句、`common.*`），`description` 手寫，`-write` 不動。`descriptionOverride` 與 `note` 都是手寫、不會被覆寫（ADR D2「說明從 proto 帶入，來源檔可覆寫」）。
- `staff`：只在 zh-TW（ADR：staff 只有 zh-TW）；`en.json` 出現 `staff` 段是錯誤。一個 enum 或變體只要有一個 `staff` key，該受眾的每個值都要有（L-brief Q1：以 enum 為單位）。沒有 `staff` 的 enum，admin 用預設受眾；這是生成器在編譯期決定的分支，不是執行期覆寫。
- `variants`：每個變體必須宣告；`omit` **依受眾**宣告（審查 D3）。完整性是「該受眾下，enum 所有非零值，扣掉該受眾的 `omit`」。`omit` 的值在生成的 API 回「沒有文字」。staff 沒有宣告 `omit` 時等於空集合。
- 不做依 client 覆寫、不做「不顯示」清單（labels-brief D-L1）。`omit` 只用在 L0 已核可「沒有文字」的位置（KycFlowCopy 的 APPROVED 沒有 action）與賣家端從不顯示的狀態（with_kind 的 CLOSED）。
- `en.json`：結構相同、只有 `message`；`-write` **不幫 en 補 key**（審查 D5）。給譯者的說明在 zh-TW 來源檔裡（TMS 以來源語系的 `description` 為準）。

### 1.4 ICU 受限子集

只允許：純文字、`{arg}`、`{arg, select, … other {…}}`、`{arg, plural, [one] {…} other {…}}`。巢狀最多兩層。
禁止：`selectordinal`、`{arg, number}`、`{arg, date}`、`=N` 精確分支、`#`（任何位置；數字一律寫 `{arg}`）、`<` 與 `>`（`use-intl` 會把 `<tag>` 當 rich text，Go 與 Dart 不會）（審查 D10）、拼接。
撇號：單獨的 `'` 是字面值；`'` 緊接 `{`、`}` 時報錯（ICU 的 apostrophe 規則在這種位置會改變語意，直接禁止就不會兩邊解讀不同）。
plural 分支依語系檢查：zh 只有 `other`；en 是 `one`、`other`（CLDR）。帶數量一律寫 `plural`，中文也是（ADR D2）。L′ 的目錄有一句帶數量（`seller.Measure.rejection_cooldown_warning`），三個 emitter 都要實作並由 golden 測試覆蓋。

### 1.5 L′ 的目錄內容

收錄規則照 L-brief Q3（只由契約值決定、兩個以上 client 使用）、Q4（以 enum 為單位完整），文字照 L0（已核可）與 §7（待核可）。

**enum 名詞（預設受眾，zh-TW）**

| enum | 值 | 來源與備註 |
|---|---|---|
| `seller.ViolationCategory` | 7 | 三端同字（L0 A） |
| `seller.ViolationSeverity` | 4 | 同上 |
| `seller.RiskGround` | 8 | 同上；取代 Go `riskGroundLabels` |
| `seller.MeasureKind` | 3 | 同上 |
| `seller.TerminationGround` | 11 | 同上；取代 Go `terminationGroundLabels` |
| `seller.MeasureState` | 5 | ACTIVE 用 `select`（L0 D.2，`other` 是「停權中」，已核可）；admin 同字，沒有 staff |
| `seller.MeasureRequestKind` | 2 | APPEAL「申覆」（L0 A3、Q5）、REMEDIATION「解除請求」 |
| `seller.MeasureRequestState` | 4 | 審核中／已通過／已駁回／已結案（L0 A3、D4） |
| `seller.KYCField` | 22 | L0 A1 的收斂字 |
| `seller.StoreField` | 5 | W `store-contact.ts:53-59` = S `kyc_fields.dart:162-169` |
| `seller.SellerRequirement` | 7 | STORE_NAME「店名」、LOGO「logo」、TOWN_CODE「所在地」已核可；另外 4 個見 §7(b) |
| `seller.KYCFlowStatus` | 11 | L0 A2 default 欄 |
| `seller.MeasureBasis` | 3 | CONTRACT「契約條款」；RISK「風險」、VIOLATION「違規認定」新增（L0 A5、D4） |
| `seller.RevokeReason` | 6 | 補四值，APPEAL_ACCEPTED「申覆成立」（L0 A5、D4）；賣家端只顯示哪幾個值是 client 的程式碼 |
| `seller.FoodRegistrationState` | 4 | 審核中／已通過／未通過／已結束（L0 A4、D4） |

**變體**

| 變體 | 值 | 文字與參數 |
|---|---|---|
| `seller.KYCFlowStatus.action` | 11（default omit APPROVED） | L0 D.1 |
| `seller.KYCFlowStatus.banner_title`、`banner_description`、`gate_heading` | 各 11 | L0 D.1；共用同一組文字的狀態各寫一條（§9 O7） |
| `seller.KYCFlowStatus.gate_message` | 11 | L0 D.1，`{subject}`（string，client 的動作名稱）；REJECTED／PAYUNI_REJECTED 是「認證未通過，請修改資料後重新送審。」 |
| `seller.MeasureRequestState.with_kind` | default 3（omit CLOSED）、staff 4 | `{kind, select, MEASURE_REQUEST_KIND_APPEAL {申覆審核中} MEASURE_REQUEST_KIND_REMEDIATION {解除請求審核中} other {…}}` 等；default 的「審核中／已通過／已駁回」、staff 的「待裁定／成立／駁回／已結案」（L0 A3）；`other` 分支見 §7(c) |
| `seller.MeasureRequestKind.submitted_note` | 2 | 「已提交申覆，請等候平台複核。」「已提交解除請求，請等候平台複核。」（L0 A3 的組合句，改成依 kind 的變體，不需要 select；審查 D20） |

**訊息句**（W 與 S 逐字相同、只由契約值加已格式化參數決定的句子；§9 O9。文字全部是現行字＋L0 B 的用語替換，已核可）

| key | 文字 | 參數 | W | S |
|---|---|---|---|---|
| `seller.Measure.title_fallback` | 措施紀錄 | — | `measureDisplay.ts:131,138,140` | `measure_display.dart:118,123,125` |
| `seller.Measure.title_risk` | 風險：{ground} | ground: noun `seller.RiskGround` | `:135` | `:121` |
| `seller.Measure.title_risk_fallback` | 風險措施 | — | `:135` | `:121` |
| `seller.Measure.meta_severity` | 嚴重程度：{severity} | severity: noun `seller.ViolationSeverity` | `:158` | `:140` |
| `seller.Measure.meta_kind` | 措施：{kind} | kind: noun `seller.MeasureKind` | `:159` | `:141` |
| `seller.Measure.meta_termination_ground` | 事由：{ground} | ground: noun `seller.TerminationGround` | `:160` | `:142` |
| `seller.Measure.termination_effective` | 終止生效：{start} | start: string（台北 `YYYY/MM/DD HH:mm`） | `:183` | `:291` |
| `seller.Measure.termination_effective_ended` | 終止生效：{start}，合作已於 {end} 結束 | start、end: string | `:182` | `:311` |
| `seller.Measure.termination_notice` | 平台已通知將於 {start} 終止合作。在此之前店家照常營業；如有異議，可提出申覆。 | start: string | `:351` | `:359` |
| `seller.Measure.appeal_deadline` | 申覆期限：{deadline} | deadline: string | `:366` | `:338` |
| `seller.Measure.rejection_cooldown_warning` | {days, plural, other {若本次經駁回，將於 {days} 日內不得再提出解除請求}} | days: integer | `:386` | `:267` |
| `seller.MeasureRequest.reply_note` | 平台回覆：{reason} | reason: string（裁定理由） | `:327` | `:216` |
| `seller.FoodRegistration.status_approved` | 已登錄：{number} | number: string | `FoodRegSupplement.tsx:148` | `food_registration_form.dart:101` |
| `seller.FoodRegistration.status_pending` | 審核中：{number}（{date} 送出） | number、date（台北 `YYYY/MM/DD`） | `:205-209` | `:109` |
| `seller.FoodRegistration.status_rejected` | 未通過：{reason}。請確認字號與食藥署登錄資料一致後重新送出。 | reason: string | `:238-239` | `:133` |
| `seller.FoodRegistration.status_revoked` | 已被平台撤銷：{number}。理由：{reason}。請確認字號與食藥署登錄資料一致後重新送出審核。 | number、reason | `:52-54` | `:125` |
| `seller.FoodRegistration.kyc_pending_note` | 平台會在實名認證通過後進行審核。 | — | `:48` | `:115` |

（W 行號：`website/features/seller/measures/measureDisplay.ts`、`website/features/seller-center/components/FoodRegSupplement.tsx`；S 行號：`seller-app/lib/features/seller/data/measure_display.dart`、`seller-app/lib/features/wallet/presentation/food_registration_form.dart`。實作者以 `rg -n` 重新定位，行號會因 L′-2 位移。）
`measureMeta` 以「 · 」接起各段、`measurePeriodText`／`measureEffect` 的停權句子、`requestNote` 的選擇邏輯留在 client：分隔符號是版面；停權句子兩端不同（§7(d)）。
不收的 W／S 共同文字：畫面固定的標題、副標、空狀態、按鈕（例 `SellerSuspendedNotice.tsx` 的 `TERMINATION_SUBTITLE`、onboarding 任務卡）——它們不是契約值的函式，由 I-sweep 抽成各端自己的資源；L′ 只照 L0 B 改用語。

**staff 受眾（zh-TW only）**：`seller.KYCFlowStatus`（11，L0 A2 staff 欄，NOT_STARTED「尚未申請」新增）、`seller.MeasureRequestState`（4：待裁定／成立／駁回／已結案）與 `with_kind` 的 staff（4）。

**通用**：`common.unknown`「未知」；`common.error.<CODE>` 15 個 code（Connect 共 16 個，CANCELED 除外），L0 D.4 原樣搬入。

**不收**：伺服器擁有文字的 enum（§2.3）；只有一個 client 顯示的 enum——`buyer.AddressLabel`（只有 buyer-app，L0 A5「這次不動」、ADR D2 分界）、admin 單端的 `AppType`、`KYCStatus`、`admin.seller.FoodRegistrationState` 等（L0 A5、Q3）；ReportStatus（CodeMeta 凍結）；Gender（字串 wire 值，L0 A5）；PayUni 狀態文案（範圍外，見 §8 範圍外）。

---

## 2. 生成器

### 2.1 語言、輸入、指令

- Go，讀 `buf build` 的 image（L-brief Q6 a；不 import Go 生成碼）。image 預設帶 imports 與 SourceCodeInfo，說明從這裡取。
- buf 版本檢查抽成 `api/check-buf-version.sh`（`api/generate.sh` 改成呼叫它，行為不變），Makefile 的 `uitext` 也呼叫它（審查 D13）：

  ```make
  uitext: ## 共用文案目錄與模組句子：重新生成 Go、TS、Dart，並把來源檔寫成正規形式
  	../api/check-buf-version.sh
  	buf build ../api -o - | GOWORK=off go run ./cmd/uitext-gen -image - -root .. -write
  ```

  `-write` 把兩份來源檔寫成正規形式（key 排序、`description` 依 §1.3 同步）；CI 跑同一條，再 `git diff --exit-code`。
- 輸入：image、`catalog/{config,zh-TW,en}.json`、每個 `internal/modules/*/internal/text/*.json`（目錄存在才讀）。

### 2.2 驗證（任一條失敗就不輸出、非零結束，訊息點名 key）

1. key 文法與解析（§1.2）；指向的型別、值存在；不是值號 0；同一節點沒有重複名稱。
2. enum 不是伺服器擁有的（§2.3）。
3. 完整性，以（受眾、語系）為單位：`required` 的每個語系，目錄裡每個 enum 的每個非零值都有名詞；每個變體扣掉該受眾的 `omit` 都有；有 `staff` 的 enum／變體每個值都有 staff。非 required 的語系只回報覆蓋率，不失敗（I4 第 4 點）。
4. 沒有多餘的 key：值或 enum 被刪掉、`omit` 列的值仍有文字、`en` 有 zh-TW 沒有的 key、`en` 出現 `staff` 段，都失敗。
5. ICU：可以解析、只用子集（§1.4）、select 分支是宣告的 enum 值名且有 `other`、plural 分支合乎該語系、每個語系的參數集合與來源相同、noun 參數指向目錄裡有的 enum。
6. 說明：每個 key 有 `description`（proto 帶入、`descriptionOverride` 或手寫）。
7. `common.error.*` 剛好是 Connect 的 15 個 code（16 個減去 CANCELED）；每個 required 語系都有 `common.unknown`。
8. D-L2 形狀守門（L′-2 起開啟）：名字以 `UnavailableReason` 結尾的 enum，只能出現在名為 `*Unavailability` 的 message 的 `reason` 欄位；出現在別處（例如 `repeated XxxUnavailableReason`）就失敗。
9. 模組句子（`text/`）：key 是 lower_snake 的點分路徑，其餘同 3–6。

### 2.3 伺服器擁有文字的 enum（結構規則，不是清單）

- `hawkr.common.v1alpha1.ErrorReason`（`ErrorInfo.reason` 的值域；唯一的具名規則）。
- 出現在任何 `*Unavailability` message 的 `reason` 欄位的 enum。L′-2 之後涵蓋 `MeasureActionUnavailableReason`、`AppealUnavailableReason`、`RemediationUnavailableReason`、`StoreOpeningUnavailableReason`。

生成器把這份集合寫進 manifest（§2.4），給 client 守門用（§5.2）。

### 2.4 輸出

| 輸出 | 路徑 |
|---|---|
| Go 目錄函式 | `backend/internal/platform/uitext/catalog_gen.go` |
| Go 模組句子 | `backend/internal/modules/seller/internal/text/text_gen.go` |
| 共用 golden | `backend/internal/platform/uitext/testdata/golden.gen.json`，同一份複製到 `packages/hawkr-ui-text-ts/tests/golden.gen.json`、`packages/hawkr_ui_text/test/golden.g.json` |
| next-intl messages（預設受眾，巢狀） | `packages/hawkr-ui-text-ts/src/messages/zh-TW.gen.json`、`en.gen.json`（**已與 zh-TW 合併**：en 缺的 key 填 zh-TW 文字） |
| TS key 與 message 建構函式 | `packages/hawkr-ui-text-ts/src/keys.gen.ts` |
| TS 的 zh-TW 編譯版（admin 用，含 staff） | `packages/hawkr-ui-text-ts/src/zh-tw.gen.ts` |
| Dart | `packages/hawkr_ui_text/lib/src/ui_text.g.dart` |
| 守門用 manifest | `packages/hawkr-ui-text-ts/src/enum-manifest.gen.json`、`packages/hawkr_ui_text/tool/enum_manifest.g.json`（每個 proto enum 的全名、TS 成員名、Dart 類別名、是否在目錄、是否伺服器擁有；Dart 版不含 `hawkr.admin.*`，對應 `packages/hawkr_api/buf.gen.yaml` 的 exclude） |

檔頭一律寫「由 backend/cmd/uitext-gen 生成，不得手改」與重新生成的指令。

**缺字的行為（三端一致，審查 D5）**：非 required 語系缺 key 時，一律輸出 zh-TW 的文字——Go 與 Dart 在生成時把 zh-TW 文字寫進該語系的分支；TS 的 `en.gen.json` 已合併 zh-TW。協商只會選到已發佈的語系（ADR D1），所以這條只在開發與 pseudo-locale 用得到；golden 含一筆「en 缺字 → zh-TW 文字」，三端各自斷言。

**golden（審查 D8）**：生成器用 `icu` 參考格式器，對每個 key 產生代表性參數的組合（每個 select 分支各一筆含 `other`、每個 plural 分支、noun 參數的每個值與不認得的值、受眾 default／staff、語系 zh-TW／en），寫成 `[{key, audience, locale, params, want}]`。四邊斷言同一份：Go 的 `catalog_gen.go`；Dart 的 `ui_text.g.dart`；TS 的 `use-intl` `createTranslator`（next-intl 用的同一個實作）與 `zh-tw.gen.ts` 的 `formatStaff`／預設受眾版。

### 2.5 API 草圖

四個例子貫穿三端：名詞 `seller.MeasureKind`、變體 `seller.KYCFlowStatus.gate_message`、select `seller.MeasureState`（「合作已終止」）、名詞參數 `seller.Measure.title_risk`，加上「未知」與 Connect 預設錯誤文字。

#### Go（`package uitext`）

```go
// uitext.go（手寫）
type Locale string
const (
	ZhTW Locale = "zh-TW"
	EN   Locale = "en"
)
const DefaultLocale = ZhTW // ADR D1

// WithLocale／LocaleFrom 是請求語系的縫：L′ 沒有人呼叫 WithLocale，LocaleFrom 一律回 DefaultLocale；
// I-server 的 interceptor 依 Accept-Language 協商後寫入。模組只能經這裡取語系，不讀 header。
func WithLocale(ctx context.Context, l Locale) context.Context
func LocaleFrom(ctx context.Context) Locale

type Text struct{ locale Locale; staff bool }
func For(l Locale) Text   // 預設受眾
func Staff() Text         // admin RPC：固定 zh-TW＋staff（ADR D1）
func (t Text) Locale() Locale
```

```go
// catalog_gen.go（生成）。v 是模組的值（去前綴的值名，同 rpc.EnumToProto）；不認得或 UNSPECIFIED 回 ("", false)，
// 伺服器不顯示「未知」。
func (t Text) SellerMeasureKind(v string) (string, bool)
func (t Text) SellerMeasureState(v string, kind string) (string, bool)
func (t Text) SellerKYCFlowStatusGateMessage(v string, subject string) (string, bool)
func (t Text) SellerMeasureTitleRisk(ground string) (string, bool) // ground 不認得時 ("", false)
func (t Text) CommonUnknown() string
```

呼叫：`uitext.Staff().SellerTerminationGround(string(t.Ground))`（`measure/create.go:108`，admin RPC）。
每個用到目錄的模組加一個資料集測試（L-brief Q6）：走訪 `model.XxxValues()`，斷言每個值在每個 required 語系都查得到。

#### TS（`@hawkr/ui-text`）

`package.json` exports（subpath、不做 barrel）：`./keys`、`./format`、`./messages/zh-TW`、`./messages/en`、`./zh-TW`、`./manifest`。
依賴：`@hawkr/api`（enum 型別，`workspace:*`）、`@connectrpc/connect`（`Code`，`catalog:`）。devDependencies：`typescript`、`@biomejs/biome`、`vitest`、`@types/node`（`catalog:`）；`tsx`（守門工具，`catalog:`）；`typescript-compiler-api`（`npm:@typescript/typescript6@6.0.2`，與 website 同版）；`use-intl`（`catalog:`，golden 對照）。`pnpm-workspace.yaml`：`packages` 加一行；`catalog` 加 `tsx`、`use-intl: 4.13.0`、`next-intl: 4.13.0`（website 改用 `catalog:`，保證 ui-text 測的與 website 跑的是同一版）；註解「typescript-compiler-api 只有 website 用」改成「website 與 @hawkr/ui-text 用」（審查 D19）。`@hawkr/api` 不依賴它（不成環）。

```ts
// src/message.ts（手寫）
export interface UiMessage<K extends UiTextKey = UiTextKey> {
	key: K;
	/** 名詞參數的值是另一則 UiMessage，格式化時以同一語系、同一受眾先解出。 */
	values?: Record<string, string | number | UiMessage>;
}
/** Connect code 的預設文字；Canceled 由呼叫端自己處理。 */
export type DefaultErrorText = (code: Exclude<Code, Code.Canceled>) => string;
```

```ts
// src/keys.gen.ts（生成）
export type UiTextKey = "seller.MeasureKind.MEASURE_KIND_WARNING" | … | "common.unknown" | `common.error.${…}`;

/** 名詞；UNSPECIFIED 與不認得的值回 common.unknown。 */
export function sellerMeasureKind(v: MeasureKind): UiMessage;
/** select 參數用 descriptor 的值名（不是 TS 成員名），由生成的表轉換；不認得的 kind 走 other。 */
export function sellerMeasureState(v: MeasureState, a: { kind: MeasureKind }): UiMessage;
/** 變體；該受眾 omit 的值回 undefined。default 與 staff 各一支（staff 只出現在 zh-tw.gen.ts）。 */
export function sellerKYCFlowStatusGateMessage(v: KYCFlowStatus, a: { subject: string }): UiMessage | undefined;
/** 名詞參數不認得時回 undefined，呼叫端用 sellerMeasureTitleRiskFallback()。 */
export function sellerMeasureTitleRisk(a: { ground: RiskGround }): UiMessage | undefined;
export function commonUnknown(): UiMessage;
export function commonError(code: Code): UiMessage | undefined; // Canceled 回 undefined
```

website（next-intl；L′ 的語系固定 zh-TW，I-web-0 接上 URL）：

```ts
// website/lib/intl/config.ts：目錄掛在 `uitext` namespace，不和 website 自己的 key 撞
import uiTextZhTW from "@hawkr/ui-text/messages/zh-TW";
export const defaultMessages = { ...websiteMessages, uitext: uiTextZhTW };

// website/lib/i18n/ui-text.ts
export type UiText = (m: UiMessage) => string;
export function useUiText(): UiText;                  // client component：useTranslations("uitext")＋先解名詞參數
export function uiTextFor(locale: AppLocale): UiText; // RSC、純函式、測試：createTranslator，明確帶 locale，不讀 header
export function useDefaultErrorText(): DefaultErrorText;
export function defaultErrorTextFor(locale: AppLocale): DefaultErrorText;
```

RSC 一律用 `uiTextFor(locale)`，不用 `getTranslations()`：沒有 `[locale]` 段時 `requestLocale` 會讀 header，把 ISR 的頁變成動態（§10 R2）。
整份 `uitext` namespace 會經 root layout 序列化進每一頁的 client payload（`app/layout.tsx:195`）；L′ 的量是幾 KB，記一筆，I-web-0 依路由切 namespace 時一起處理（審查 D21）。

admin（不裝 next-intl，吃 zh-TW 編譯版）：

```ts
// src/zh-tw.gen.ts（生成）：每個 key 編成函式；staff 有就用 staff，否則預設受眾；名詞參數也取 staff 優先
export function formatStaff(m: UiMessage): string;
export const defaultErrorTextZhTW: DefaultErrorText;
```

#### Dart（`hawkr_ui_text`，純 Dart）

依賴：`hawkr_api`（path）、`intl`（`>=0.20.2 <0.21.0`，包含兩支 App 解析到的 0.20.2；`plural` 呼叫 `Intl.pluralLogic`，ADR D2 b′）、`connectrpc`（`Code`）。dev：`analyzer`（守門工具）、`test`。

```dart
// lib/hawkr_ui_text.dart（手寫）
enum UiLocale {
  zhTW('zh-TW'), en('en');
  const UiLocale(this.tag);
  final String tag;
}
export 'src/ui_text.g.dart';
```

```dart
// lib/src/ui_text.g.dart（生成）。比對 value（整數），不比對名稱（omit_enum_names 會拿掉名稱）。
// protoc_plugin 的 enum 是 class，編譯器不檢查窮舉：完整性由生成器保證，_ 分支回「未知」。
extension SellerMeasureKindUiText on MeasureKind {
  String uiText(UiLocale l) => switch ((l, value)) {
        (UiLocale.zhTW, 1) => '警告',
        …
        _ => commonUnknown(l),
      };
}
extension SellerMeasureStateUiText on MeasureState {
  String uiText(UiLocale l, {required MeasureKind kind});
}
extension SellerKYCFlowStatusUiText on KYCFlowStatus {
  String uiText(UiLocale l);
  String? gateMessage(UiLocale l, {required String subject}); // omit 的值回 null
}
String? sellerMeasureTitleRisk(UiLocale l, {required RiskGround ground}); // ground 不認得回 null
String commonUnknown(UiLocale l);
String? commonErrorText(Code code, UiLocale l); // canceled 回 null
```

App 端（每支 App 一個手寫檔 `lib/core/l10n/ui_locale.dart`；已發佈語系是各介面自己的清單，ADR D1）：

```dart
const publishedUiLocales = [UiLocale.zhTW];
UiLocale resolveUiLocale(Locale? device); // zh* → zhTW；其他語言在 en 已發佈時 → en，否則 zhTW（ADR D1）
final uiLocaleProvider = Provider<UiLocale>((ref) => resolveUiLocale(PlatformDispatcher.instance.locale)); // 不用 @riverpod
```

`extension UiLocaleOf on BuildContext { UiLocale get uiLocale; }` 在**第一次有 widget 用到的 commit** 才建立（seller-app 在 L′-6；buyer-app 在 L′ 沒有 widget 用到，不建），避免 deadcode 擋下（審查 D17）。

### 2.6 模組句子（`text/`）的 Go 輸出

```go
// backend/internal/modules/seller/internal/text/text_gen.go（生成）
package text
func AppealUnavailableRevoked(l uitext.Locale) string
func RemediationUnavailableCooldown(l uitext.Locale, a RemediationUnavailableCooldownArgs) string
type RemediationUnavailableCooldownArgs struct{ Time string } // 已格式化的台北時間
```

---

## 3. D-L2：賣家端的原因 enum

### 3.1 契約（`api/hawkr/seller/v1alpha1/seller.proto`）

照 3a 的形狀與先例（新欄位號加 `reserved`）：

```proto
message AppealUnavailability {
  // 原因，給 client 分支用。伺服器一律設，不會是 UNSPECIFIED。
  AppealUnavailableReason reason = 1 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 給使用者看的完整一句話（locale 是實際用的語系），有主詞、不含 client 的畫面或選單名稱；
  // 與 SubmitAppeal 被拒絕時錯誤的 LocalizedMessage 是同一份文字（同一個 Go 函式）。
  google.rpc.LocalizedMessage message = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
}
message RemediationUnavailability { RemediationUnavailableReason reason = 1; google.rpc.LocalizedMessage message = 2; }
message StoreOpeningUnavailability { StoreOpeningUnavailableReason reason = 1; google.rpc.LocalizedMessage message = 2; }

message Measure {
  reserved 9, 14, 19, 22;
  reserved "appeal", "remediation_requests", "remediation_unavailable_reasons", "appeal_unavailable_reasons";
  repeated RemediationUnavailability remediation_unavailabilities = 23 [(google.api.field_behavior) = OUTPUT_ONLY];
  repeated AppealUnavailability appeal_unavailabilities = 24 [(google.api.field_behavior) = OUTPUT_ONLY];
}
message GetMyStoreOpeningResponse {
  reserved 2;
  reserved "unavailable_reasons";
  repeated StoreOpeningUnavailability unavailabilities = 3 [(google.api.field_behavior) = OUTPUT_ONLY];
}
```

同一個 commit 刪掉四個原因 enum 的「標籤「…」」與 `error_reason.proto:35-40` 的整句（labels-brief D-L2 第 5 點），註解只留語意；「依優先序、第一個是 NOT_RISK 或 NOT_ACTIVE 表示沒有入口」等分支規則保留。`next_remediation_time` 保留（D-L2 第 4 點：動態值另以結構化欄位給），client 不再拿它組字。

### 3.2 伺服器

- 句子：seller 模組 `internal/text/zh-TW.json`，`en.json` 是 `{}`。文字照 §7(a) 核可的最終句。
- model（`internal/model`）新增，錯誤路徑與可用性回應只經這幾個：

  ```go
  type AppealUnavailability struct { Reason AppealUnavailableReason; Message string }
  type RemediationUnavailability struct { Reason RemediationUnavailableReason; Message string }
  type OpeningUnavailability struct { Blocker OpeningBlocker; Message string }

  func (m Measure) AppealUnavailabilities(l uitext.Locale) []AppealUnavailability          // 依 AppealUnavailableReasons 的順序
  func (m Measure) RemediationUnavailabilities(l uitext.Locale) []RemediationUnavailability // NOT_ACTIVE 依 Status 選句；COOLDOWN 帶時間
  func (b OpeningBlocker) Text(l uitext.Locale) string
  ```

  `errAppealUnavailable(l, m)`、`errRemediationUnavailable(l, m)` 取第一筆的 `Message`：`apperr.FailedPrecondition("%s", msg)`。原因清單是空的（`allowed` 為 false 時不可能）回 `fmt.Errorf` 的內部錯誤，不再寫通用句（現行「這筆措施目前不能申訴」「補正請求目前不能提出」刪除）。`OpeningBlocker.Err(l)`：STORE_EXISTS 維持 `apperr.AlreadyExists`，TERMINATED／SUSPENDED 的 `WithReason` 不變。
- `StoreOpening.Unavailable` 改成 `[]OpeningUnavailability`；`Module.StoreOpening(ctx, …)`（`seller.go:424`）以 `uitext.LocaleFrom(ctx)` 組字；`onboarding.go:102` 的 `model.OpeningBlock(state)` 改成 `model.OpeningBlock(l, state)`。模組根套件（`governance.go`）轉出型別，handler 不 import `internal/`。
- COOLDOWN 的時間：伺服器以 Asia/Taipei 組好（D-L2 第 2 點）。`twtime` 新增 `MinuteCeil(t time.Time) time.Time`（秒或奈秒有餘數時進位到下一分鐘，與 website `formatTaipeiDateTimeCeilMinute`、seller-app `_cooldownReason` 同規則）；zh-TW 的版面 `2006/01/02 15:04`（`t.In(twtime.Taipei)`）寫在 `RemediationUnavailabilities` 組參數處，`twtime` 不認得語系。en 的版面在 I-server 的格式化一起做（§9 O3）。
- handler：`toMeasure` 的原因段改成走訪 `m.AppealUnavailabilities(l)`、`m.RemediationUnavailabilities(l)`，`Message: rpc.LocalizedMessage(l, msg)`；`GetMyStoreOpening` 同理。`l` 由 RPC 方法以 `uitext.LocaleFrom(ctx)` 取得後往下傳。
- `rpc.LocalizedMessage` 改成 `LocalizedMessage(l uitext.Locale, text string)`：`locale` 是實際用的語系（ADR D3）。admin 的 `toActionUnavailabilities` 傳 `uitext.ZhTW`。錯誤的 `errorLocale` 常數在 L′ 不動（`apperr` 的文字仍是 zh-TW 字面值；I-server 改）。

### 3.3 請求語系在 L′ 怎麼到模組

決定：**L′ 只做 context 縫，不做協商**。`uitext.WithLocale`／`LocaleFrom`；L′ 沒有 interceptor 寫入，所以永遠是 zh-TW。理由：ADR D1 規定未發佈的語系在解析時視為不存在；伺服器的已發佈語系只有 zh-TW，現在做 `Accept-Language` 協商結果永遠是 zh-TW，沒有可觀察的行為。I-server 只加 interceptor（含 `Vary: Accept-Language`、協商測試），模組與 handler 不再改；錯誤與可用性兩條路徑都從 `LocaleFrom(ctx)` 取語系，接上之後同時依請求。

### 3.4 client

規則（錯誤模型 D3 與 D-L2 第 5 點）：**顯示第一筆的 `message.message`；沒有 message 時顯示該動作的通用文字**（client 自己的字，照 L0 B3／B4 改用語）；`reason` 只用來分支。顯示位置與換掉的文字見 §7(a)。

- website
  - `lib/seller/measures.ts`：刪 `APPEAL_UNAVAILABLE_LABEL`（:187-194）、`APPEAL_ALREADY_APPEALED_NOTICE`、`APPEAL_REVOKED_NOTICE`（:174-175）、`appealBlockedReason`、`appealClosedNotice` 的整句分支（:200-222）、`remediationBlockedReason` 的手寫句與 COOLDOWN 組字（:229-247）；`appealClosedNotice`／`remediationClosedNotice` 改成「第一筆 message，否則 `APPEAL_UNAVAILABLE_NOTICE`／`REMEDIATION_UNAVAILABLE_NOTICE`」。
  - `features/seller/measures/measureDisplay.ts`：`appealEntry`、`remediationEntry` 的 `reason` 改讀 message；`remediationEntry` 依 `reason` 決定要不要顯示入口（NOT_RISK、NOT_ACTIVE 不顯示）不變。
  - `lib/seller/onboarding.ts`：刪 `STORE_OPENING_REFUSAL_TEXT`（:21-27）；`storeOpeningGate` 回 `{ blocked, refusal, message }`（`refusal` 只給 `storeOpeningRefusalHasAppealLink`）；不認得的 reason 但有 message 時顯示 message、不給申覆入口。`storeOpeningRefusal`（ApplyStore 的錯誤路徑）同樣回 `{ refusal, message }`，`message` 取 `factsFromError(e).localizedMessage`。
  - `features/seller/onboarding/SellerOnboardingClient.tsx:535` 顯示 `message`。
  - e2e mock `tests/e2e/helpers/mock-gateway.ts` 改新形狀並帶 `message`（website-e2e-verify 的 wire-shape 規則）。
- seller-app
  - `features/seller/data/measure_display.dart`：刪 `appealUnavailableReasonLabel`（:97-108）、`remediationAction` 的句子與 `_cooldownReason`（:229-258）；`appealAction`、`remediationAction` 的 `reason` 改讀 message。
  - `features/seller/data/measures.dart`：刪 `appealAlreadyAppealedNotice`、`appealRevokedNotice`（:29-30）；`appealClosedNotice`、`remediationClosedNotice`（:146-162）改成「第一筆 message，否則通用文字」。
  - `features/seller/data/seller_onboarding.dart`：`StoreOpeningRefusal` 拿掉 `text`（:25-46），保留 `offersAppeal`；`StoreOpeningBlock` 帶 `message`；`_failure`（:211-221）的 `OnboardingStoreOpeningRefused` 帶錯誤的 `LocalizedMessage`。`presentation/seller_info_screen.dart:202` 顯示 message。

---

## 4. client 的遷移

### 4.1 website

| 檔案 | 動作 |
|---|---|
| `package.json`（`@hawkr/ui-text: workspace:*`、`next-intl: catalog:`）、`next.config.ts`（`transpilePackages` 加 `@hawkr/ui-text`）、根目錄 `pnpm-lock.yaml` | 加依賴（L′-4） |
| `lib/intl/config.ts`、`lib/i18n/ui-text.ts`、next-intl 的 `AppConfig` 型別宣告（沒有就新增 `global.d.ts` 段） | §2.5 的接線；L′-4 只放錯誤文字需要的部分，L′-5 補齊 |
| `features/seller/measures/measureDisplay.ts:30-95,197-214,269-329` | 名詞表、`STATE_DISPLAY` 的文字（顏色留下）、合作已終止、請求的組合句改用目錄；`Partial<Record>` 全部消失 |
| `features/seller/measures/measureDisplay.ts` 的 `measureTitle`、`measureMeta`、`measurePeriodText` 的終止合作分支、`terminationNoticeText`、`appealEntry` 的期限、`rejectionCooldownWarning`、`requestNote` | 改用 §1.5 的訊息句；停權的期間句留在 client（§7(d)） |
| `features/seller-center/components/FoodRegSupplement.tsx:48-54,148,205-209,238-239` | 食品登錄的五句改用訊息句 |
| `lib/seller/kyc-flow.ts:62-197` | `KycFlowCopy` 的文字欄位改用目錄（label、action、banner、gate）；`tone`、`ctaLabel` 留在 client（L0 D.1） |
| `lib/seller/kyc.ts:434-544` | `FieldMeta` 拿掉 `label`、加 `control`（表單控制項 id；BANK_NAME 與 BANK_CODE 同一個 `bank` 控制項）；`kycFieldLabel` 改用目錄；`missingItemsOf` 依 `control` 去重，同一控制項列 proto 順序第一個欄位的名詞（銀行列「銀行名稱」）（§9 O8） |
| `lib/seller/store-contact.ts:53-59` | `STORE_FIELD_LABELS` 改用目錄 |
| `features/seller-center/dashboard/onboardingTasks.ts:55-60` | `SellerRequirement` 改用目錄 |
| `features/seller-center/components/SellerSuspendedNotice.tsx:54` 等「合作已終止」 | `sellerMeasureState(ACTIVE, {kind})` |
| L0 B2／B3 列的每一個檔與行 | 用語：措施、申覆、解除請求、修改、紀錄；導覽「措施與申覆」；發票的「補正期限」改「補件期限」；售後與發票的「申訴」不動（L0 B7-1） |
| 純函式 | 回傳字串的 `lib/seller/*.ts`、`measureDisplay.ts` 函式第一個參數改收 `ut: UiText`（§9 O6）；單元測試用 `uiTextFor("zh-TW")` |
| `package.json` 的 `lint` | 末端加守門（§5.2） |
| `website/AGENTS.md` | 開發指令段加一行 `@hawkr/ui-text`（比照 twaddr） |

### 4.2 admin

- 依賴 `@hawkr/ui-text`（`package.json`、`next.config` 的 `transpilePackages`），L′-4 加入。
- 改用目錄（staff）：`merchants/[id]/measure.ts:39-51,76-107,139-203,294-304`（RiskGround、MeasureState badge、MeasureBasis、ViolationCategory／Severity、MeasureKind、TerminationGround、RevokeReason）、`measures/measure-inbox.ts:71-104`（MeasureRequestKind、MeasureRequestState 與 `requestStateText` 的組合）、`merchants/store-row.ts:48-88`（KYCFlowStatus，補 NOT_STARTED「尚未申請」）、`merchants/[id]/kyc-application.ts:207-233`（KYCField，L0 A1 收斂字）。
- admin 單端的 enum 留在 admin（Q3），但一律 `Record<Exclude<E, E.UNSPECIFIED>, string>`，不認得與 UNSPECIFIED 走 `common.unknown`：`food-registration.ts:39-46`（刪「未知」）、`app-versions/page.tsx:31-35`（刪「未指定」）、`food-registration.ts:106-113`（KYCStatus；SUBMITTED「審核中」改「待審核」，§9 O12）。
- 用語：L0 B5；`ActiveMeasuresCard.tsx:55`「賣家停權中」留著（L0 D.2）。
- 範圍外（L-brief「知會」最後一項）：`store-row.ts:121-127`、`store-detail.ts:117-138` 等 admin 自己寫的「不能做」原因句。

### 4.3 seller-app

- `pubspec.yaml`（path 依賴 `../packages/hawkr_ui_text`）、`pubspec.lock`；`lib/core/l10n/ui_locale.dart`（L′-4 建 provider；L′-6 加 `BuildContext` extension）。
- 改用目錄：`features/seller/data/measure_display.dart:16-95,110-219,262-390`（名詞、標題、meta、終止合作的效果句、期限、預告、冷卻警告、請求句；停權的效果句留在 client）、`kyc_fields.dart:72-169`（`_kycFields` 拿掉 label、加 control；銀行是兩個控制項，各列一次；`missingItemsOf`:148-158 與 `KycMissingItem` 的 `==`:139-145 改依 control）、`kyc_flow.dart:38-129`（UNSPECIFIED 不再 `throw ArgumentError`，改「未知」）、`onboarding_tasks.dart:112-116`、`suspension_notice_screen.dart:86-130` 的「合作已終止」、`features/wallet/presentation/food_registration_form.dart:102-133`（日期改用台北：`packages/hawkr_api/lib/taipei_time.dart` 新增 `formatTaipeiDate`，刪 `_date`；這是 bug 修正，現況用裝置時區）。
- 顯示用的純函式加 `UiLocale l` 參數，widget 傳 `context.uiLocale`；repository／controller 用 `ref.read(uiLocaleProvider)`。
- 用語：L0 B2／B4 列的每一行（`seller_appeal_screen.dart:222`「處置時間」→「措施時間」）。
- 不在 L′：`app.dart:318-323` 的 `supportedLocales`／`zh_CN`（I-app-0；`app.dart` 是 10d 熱點）。

### 4.4 buyer-app

- 錯誤文字的呼叫點（3 檔：`auth_repository.dart`、`recipe_repository.dart`、`address_repository.dart`）與 `lib/core/l10n/ui_locale.dart`（只有 provider），在 L′-4。
- 品牌：`lib/shared/widgets/top_navigation.dart:312`「Hawkr」→「Hawkr」（術語表、L0 C3），併入 L′-4。
- AddressLabel 不動（只有 buyer-app 一端顯示；L0 A5）。`address_repository.dart` 以中文字反解 `_labelOf` 是 buyer-app 的設計缺陷，記成 buyer-app 的待辦（§8 範圍外），不在 L′。

### 4.5 錯誤 helper（`@hawkr/api`、`hawkr_api`）

方向：`@hawkr/ui-text` 依賴 `@hawkr/api`，所以 `@hawkr/api` 不能反過來依賴目錄；預設文字由呼叫端注入。**補預設文字只在 `submitError`／`userMessage` 一層做**（它們已經拿到 `defaults`），呼叫點不各自補（審查 D11）。

```ts
// @hawkr/api/connect/errors
export type DefaultErrorText = (code: Exclude<Code, Code.Canceled>) => string;
export function userMessage(err: unknown, defaults: DefaultErrorText): string | undefined; // Canceled 回 undefined
// FieldError.message 改成 string | undefined（= v.localizedMessage?.message，拿掉 ?? v.description）
// 刪 defaultErrorText
// @hawkr/api/connect/submit
export function submitError(err: unknown, defaults: DefaultErrorText, method?: DescMethod): SubmitError;
// fields 分支：f.message ?? defaults(Code.InvalidArgument)；SubmitError.fields 型別維持 Record<string, string>
// message 分支：d.message ?? defaults(e.code === Code.Canceled ? Code.Internal : e.code)（現行 Canceled 也顯示「系統錯誤」）
```

```dart
// hawkr_api/lib/errors.dart、submit.dart
typedef DefaultErrorText = String Function(Code code); // canceled 由 helper 先換成 internal
String? userMessage(Object error, {required DefaultErrorText defaults});
SubmitError submitError(Object error, {required DefaultErrorText defaults, Idempotency? idempotency, Object? request});
// FieldError.message 改成 String?；SubmitFields 在這一層補 defaults(Code.invalidArgument)；刪 defaultErrorText
```

- 呼叫端：website 用 `useDefaultErrorText()`／`defaultErrorTextFor(locale)`；admin 用 `defaultErrorTextZhTW`；App 用 `(c) => commonErrorText(c, ref.read(uiLocaleProvider))!`（生成碼保證非 canceled 有字）。
- 伺服器端「每個給使用者看的 `FieldViolation` 都帶 `localized_message`」：唯一轉換點已保證（`rpc/errors.go:145-170`）——**already protected**；`rpc_test.go:116` 已斷言非空，L′ 不另加。
- 套件測試：`packages/hawkr-api-ts/tests/errors.test.ts`、Dart 的情境表改成「注入的 defaults 被呼叫、description 不會出現」；預設文字本身的測試搬到 ui-text 的 golden。

### 4.6 kyc_status（L-brief Q4）

- `admin/seller/v1alpha1/seller.proto:832-833` 改成 `optional hawkr.seller.v1alpha1.KYCStatus kyc_status = 14`，註解「不設表示店家還沒有申請」。只有這一個欄位（`ProspectTaxIDMatch.kyc_status` 來自 inner join 一定有值；`ListStoresRequest.kyc_status` 是篩選）。
- Go `foodreg.go:122-124` 改成指標賦值；`make connect`。
- admin：`food-registrations/page.tsx:169`、`[id]/page.tsx:258` 改成 `kycStatus === undefined ? "尚未申請" : …`（admin 版面文字）；`KYC_STATUS_LABEL` 改 `Record<Exclude<…>>`。Dart 不生成 admin。

---

## 5. CI

### 5.1 生成物 drift：`.github/workflows/ui-text-generated-diff.yml`（L′-1）

- paths：`api/**`、`backend/cmd/uitext-gen/**`、`backend/internal/platform/uitext/**`、`backend/internal/modules/*/internal/text/**`、`packages/hawkr-ui-text-ts/src/**`、`packages/hawkr-ui-text-ts/tests/golden.gen.json`、`packages/hawkr_ui_text/lib/**`、`packages/hawkr_ui_text/tool/**`、`packages/hawkr_ui_text/test/golden.g.json`、`api/check-buf-version.sh`、本 workflow。
- 步驟：checkout → setup-go（`backend/.go-version`）→ buf（`api/.buf-version`，`bufbuild/buf-action` setup_only）→ `make -C backend uitext` → `git diff --exit-code`（輸出與兩份來源檔）。失敗訊息寫重新生成的指令。
- 生成器與 Go 執行期的單元測試由 `backend-monolith.yml` 的 `go test ./...` 跑。

### 5.2 守門：client 手寫的 proto enum 文字表

用 AST，不用 regex。名稱比對靠 manifest。

**規則（審查 O4 修正後）**
- 每個 client（website、admin、兩支 App、共用套件）一律擋：**在目錄的 enum** 與 **伺服器擁有的 enum** 的手寫文字表。沒有範圍參數、沒有例外清單。
- 跨端：同一個 proto enum 在**兩個以上 client** 都有手寫文字表就失敗（把 Q3 的「兩端以上進目錄」機械化）。單端的表（AddressLabel、admin 單端 enum）允許。

**TS**：`packages/hawkr-ui-text-ts/tools/check-enum-text.ts`（`typescript-compiler-api`）。對每個非測試、非生成、非 `node_modules`、非 `codemeta` 的 `.ts`／`.tsx`：
- 收集從 `@hawkr/api/gen/…` import 的識別字（處理 `as` 別名），對照 manifest。
- R1：object literal 的 computed key 是 `E.M`，值（含巢狀屬性、template、JSX）有漢字字串。
- R2：`case E.M:` 子句裡有漢字字串的 `return` 或賦值。
- `--report <file>`：輸出「此 client 有手寫表的 enum 全名」清單，給跨端檢查用。
- 已知漏網：`x === E.M ? "…" : …` 的三元式不擋；由 I-CI 的逐檔漢字 ratchet 補。

**Dart**：`packages/hawkr_ui_text/tool/check_enum_text.dart`（`package:analyzer` 的 `parseString`）：
- D1：switch 敘述與 switch 運算式的 case 樣式是 `E.M`（`E` 是 manifest 裡的 Dart 類別名），分支有漢字字串。
- D2：map literal 的 key 是 `E.M`，值（含 record）有漢字字串。
- 範圍：App 的 `lib/`，以及 `packages/hawkr_api/lib`（排除 `gen/`）、`packages/hawkr_auth/lib`、`packages/hawkr_app_update/lib`。排除 `*.g.dart`、`*.freezed.dart`、`core/codemeta/`。`--report <file>` 同 TS。

**跨端**：`scripts/check-enum-text-cross.sh`：對 website、admin 跑 TS 工具、對兩支 App 跑 Dart 工具（都帶 `--report`），再交集；有任何 enum 出現在兩個以上 client 就失敗並列出檔案。

**接在哪裡**

| 檢查 | 本機 | CI |
|---|---|---|
| website | `package.json` 的 `lint` 末端加 `pnpm --filter @hawkr/ui-text check-enum-text ../../website` | website-ci 的 lint |
| admin | `lint` 末端加 `… ../../admin` | admin-ci 的 lint |
| seller-app、buyer-app | 各 App `AGENTS.md` 的驗證指令段加 `(cd ../packages/hawkr_ui_text && dart pub get && dart run tool/check_enum_text.dart ../../<app>/lib)`；seller-app Makefile 加 `enum-text` target（buyer-app 沒有 Makefile） | `_flutter-quality.yml` 新增 step `enum_text`：`working-directory: packages/hawkr_ui_text`，先 `flutter pub get` 再 `dart run tool/check_enum_text.dart ../../${{ inputs.app }}/lib`；加進「Fail if any static check failed」的條件（審查 D7-3：hygiene step 在 App 目錄、ui_text 沒有 pub get，不接在那裡） |
| 跨端 | `bash scripts/check-enum-text-cross.sh` | 新 workflow `.github/workflows/ui-text-guard.yml`：Node＋pnpm＋Flutter；paths `website/**`、`admin/**`、`seller-app/lib/**`、`buyer-app/lib/**`、`packages/hawkr-ui-text-ts/**`、`packages/hawkr_ui_text/**`、`scripts/check-enum-text-cross.sh`、本 workflow |

### 5.3 既有 workflow 的 paths 與 matrix（每一條分配到 commit，審查 D6、D7）

| workflow | 改動 | commit |
|---|---|---|
| `ts-packages.yml` | matrix 加 `@hawkr/ui-text`；paths 加 `packages/hawkr-ui-text-ts/**`、`packages/hawkr-api-ts/**`（ui-text 依賴它的型別；`api/` 的改動一定伴隨它的生成物） | L′-1 |
| `dart-packages.yml` | matrix 加 `hawkr_ui_text`（paths 的 `packages/hawkr_*/**` 已涵蓋它與 `hawkr_api`）；format step 改成 `dart format --output=none --set-exit-if-changed lib test $( [ -d tool ] && echo tool )` | L′-1 |
| `website-ci.yml`、`admin-ci.yml` | paths 加 `packages/hawkr-ui-text-ts/**` | L′-4（兩個 web 第一次依賴） |
| `seller-ci.yml`、`buyer-ci.yml` | paths 加 `packages/hawkr_ui_text/**` | L′-4（兩支 App 第一次依賴） |
| `_flutter-quality.yml` | `enum_text` step | L′-6 |
| `ui-text-guard.yml`（新） | 跨端 | L′-1 |

### 5.4 不在 L′ 的 CI（I-CI）

逐檔漢字 ratchet、pseudo-locale、「沒人用的 key」、已發佈語系與目錄 `required` 的對齊（§9 O10）。

---

## 6. proto 與程式註解的用語

- 範圍：`api/` 的 98 行（處置 5、申訴 64、補正 29）＋ 55 行「標籤「…」」。規則照術語表退場表：對賣家的「處置」→「措施」；賣家對措施的請求「申訴」→「申覆」；對使用者顯示的「補正」→「解除請求」或「修改」。發票申訴、售後申訴、法律頁不動（L0 B7-1、B7-5）。
- 「標籤「X」：」整段刪除，只留語意（codemeta ADR D6）；文字已經在目錄。`MeasureState` ACTIVE 的「client 依 kind 呈現」一句刪除。
- 這些註解會進三端的生成碼，同一個 commit 跑 `make connect`、`pnpm --filter @hawkr/api gen:api`、`packages/hawkr_api` 的 `../../api/generate.sh buf.gen.yaml`，以及 `make uitext`（`description` 跟著變）。
- 程式註解：在各區的遷移 commit 裡改該區的註解；Go 識別字不改（術語表範圍）。
- 檢查：`rg -n '處置|補正' api/` 只剩術語表允許的語意；`rg -n '申訴' api/ | rg -v '消費者申訴|發票申訴|售後申訴'` 為空；`rg -c '標籤「' api/` 為空。

---

## 7. 待 Will 核可的文案

只列 L0 沒有核可、而 L′ 會讓使用者看到（或改變顯示位置）的文字。L′-1 之前一次核可。原則：伺服器句為準、套術語表、不寫 client 的導覽名稱、單句不加句號（與現行 Go 句一致，也方便 website／seller-app 以「 · 」與期限並列）、兩句以上照常加句號、時間是台北 `2006/01/02 15:04`、秒數進位到分鐘。

### (a) D-L2 伺服器原因句（14 列）

收斂後的顯示位置代號：
- **W-btn**：website 反灰按鈕旁的短提示（`AppealEntry.tsx:28` 的 note、`RemediationEntry.tsx:33-38`）
- **W-full**：website 請求頁不能送出時的說明（`SellerMeasureRequestPage.tsx` 的 `closedNotice`，`lib/seller/measures.ts:211-257`）
- **W-open**：website 開店頁（`SellerOnboardingClient.tsx:535`，GetMyStoreOpening 與 ApplyStore 錯誤兩條路徑）
- **S-btn**：seller-app 措施頁按鈕旁（`appealNoteText`／`remediationAction`，`seller_appeal_screen.dart:181`）
- **S-full**：seller-app 請求表單的說明（`seller_measure_request_form_screen.dart:69-70`、`measures.dart:317-318`）
- **S-open**：seller-app 開店頁（`seller_info_screen.dart:202`）
- **Go**：`SubmitAppeal`／`RequestRemediation`／`ApplyStore` 被擋下時錯誤的 `LocalizedMessage`（現行 client 對措施的 failed_precondition 會重讀並顯示 W-full／S-full，所以使用者看到的是同一句）

**AppealUnavailableReason**

| 值 | W 短提示 | W 整句 | S | Go 現行（L0 B6 已核可的替換） | 建議最終句 | 取代 | 建議 |
|---|---|---|---|---|---|---|---|
| REVOKED | 已撤銷 `measures.ts:190` | 這項處置已撤銷，不需要申訴 `:175` | 已撤銷 `measure_display.dart:100`；整句 `measures.dart:30` 同 W | 這筆措施已撤銷 `appeal.go:26` | **這筆措施已撤銷，不需要申覆** | W-btn、W-full、S-btn、S-full、Go | 採（合併 client 整句的資訊，用語照 B）；替代：B6 原句「這筆措施已撤銷」 |
| ALREADY_APPEALED | 已申訴過 `:191` | 這項處置已經提出申訴 `:174` | 已申訴過 `:102`；整句 `measures.dart:29` 同 W | 這筆措施已申訴過 → 已申覆過 `appeal.go:28` | **這筆措施已提出過申覆，每筆措施只能申覆一次** | 同上 | 採（說明為什麼，取自 proto 語意「同一措施只能申訴一次」）；替代：B6「這筆措施已申覆過」 |
| APPEAL_WINDOW_ENDED | 申訴期限已過 `:192` | （用短提示） | 申訴期限已過 `:104` | 終止合作的申訴期限已過 → 申覆期限已過 `appeal.go:30` | **終止合作的申覆期限已過** | 同上 | 採 B6 原句；不帶時間：期限另以「申覆期限：{時間}」在同一行顯示（`appealEntry`、`appealNoteText`） |
| REQUEST_PENDING | 有審核中的請求 `:193` | （用短提示） | 有審核中的請求 `:106` | 這筆措施有待處理的補正請求，審核完成後才能申訴 → 解除請求…申覆 `appeal.go:32` | **這筆措施有待處理的解除請求，審核完成後才能申覆** | 同上 | 採 B6 原句（已查證：排第一時一定是解除請求，§0） |

**RemediationUnavailableReason**

| 值 | W 短提示 | S | Go 現行（B6） | 建議最終句 | 取代 | 建議 |
|---|---|---|---|---|---|---|
| REMEDIATION_PENDING | 解除請求審核中 `measures.ts:237` | 解除請求審核中 `measure_display.dart:235` | 這筆措施已有待處理的請求 `remediation.go:24`（與 APPEAL_PENDING 同句） | **這筆措施的解除請求審核中，審核完成前不能再提出** | W-btn、W-full、S-btn、S-full、Go | 採（保留兩端現在區分的資訊；一個原因一個函式，沒有額外成本）；替代：B6 原句（兩個原因同一句「這筆措施已有待處理的請求」） |
| APPEAL_PENDING | 申訴審核中 `:239` | 申訴審核中 `:238` | 同上 | **這筆措施的申覆審核中，審核完成前不能請求解除** | 同上 | 同上 |
| COOLDOWN | `{時間} 後可再提出` `:242`（台北、進位到分） | `{時間} 後可再提出` `:257` | 補正請求目前在冷卻期內，不能再提出 → 解除請求… `remediation.go:26`（沒有時間） | **解除請求目前在冷卻期內，{time} 後可再提出**（time＝`2006/01/02 15:04` 台北，秒數進位） | 同上 | 採（新句：伺服器帶時間，審查問題 2） |
| NOT_RISK | 不顯示入口 | 不顯示入口 | 只有依據風險類型的措施可以請求補正後解除 → 請求解除 `remediation.go:28` | **只有依據風險類型的措施可以請求解除** | W-full、S-full、Go（入口仍不顯示） | 採 B6 原句 |
| NOT_ACTIVE（已撤銷） | 不顯示入口 | 不顯示入口 | 這筆措施已撤銷 `remediation.go:31` | **這筆措施已撤銷** | W-full、S-full、Go | 採（原句） |
| NOT_ACTIVE（未撤銷：已結束、警告、預定生效） | 不顯示入口 | 不顯示入口 | 這筆措施已不在生效中 `remediation.go:33` | **這筆措施目前不在生效中** | 同上 | 採（「已不」對從未生效的警告不正確）；替代：原句 |

W-full、S-full 現在對 NOT_RISK／NOT_ACTIVE 顯示通用的「這項處置目前不能請求解除」；收斂後改顯示伺服器句，這是顯示上的變化。

**StoreOpeningUnavailableReason**（W `lib/seller/onboarding.ts:23-26`、S `seller_onboarding.dart:28-39`、Go `model/model.go:120-126`，三端同字）

| 值 | 現行 | 建議最終句 | 取代 | 建議 |
|---|---|---|---|---|
| TERMINATED | 此帳號因終止合作（第 21.3 條）不能重新開店。如有異議，請至「處置與申訴」提出申訴。 | **此帳號因終止合作（第 21.3 條）不能重新開店。如有異議，可對該終止提出申覆。** | W-open、S-open、Go | 已核可（L0 B6），列出供對照；申覆入口由 client 依 `reason` 顯示 |
| SUSPENDED | 帳號停權中，解除停權前不能開店 | **帳號停權中，解除停權前不能開店** | 同上 | 採（原句） |
| ACCOUNT_DELETING | 帳號正在刪除，不能申請開店 | **帳號正在刪除，不能申請開店** | 同上 | 採（原句） |
| STORE_EXISTS | 您已經是賣家，無需重複申請 | **你已經是賣家，不需要重複申請** | 同上 | 採（賣家面用「你」：`rg -c` 你 166、您 24）；替代：原句 |

另外：Go 的通用句「這筆措施目前不能申訴」「補正請求目前不能提出」刪除（§3.2，原因清單空時是程式錯誤，不再對使用者顯示）。

### (b) SellerRequirement 另外 4 個值的名詞

現況兩端都沒有顯示這 4 個值（`onboardingTasks.ts:66-70`：交給公告頁或 KYC 任務；S `onboarding_tasks.dart:103-111` 同），名詞只在「其他公開條件」或之後的新位置用得到。已對照判斷函式 `model/exposure.go:94-110`：

| 值 | 判斷（code） | 審查建議 | 建議 | 說明 |
|---|---|---|---|---|
| STORE_ACTIVE | `s.Status != Closed` | 店家未關閉 | **店家未關閉** | 準確 |
| NOT_SUSPENDED | `!s.Suspended()`（帳號任一店家的生效中措施） | 帳號未停權 | **帳號未停權** | 準確（停權以帳號為單位） |
| KYC_APPROVED | `flow == FlowApproved`（含 PayUni 核准） | 商家認證已通過 | **商家認證已通過** | 與 KycFlowCopy APPROVED 的 banner「商家認證已通過」同字，涵蓋 PayUni |
| NOT_PENDING_DELETION | `s.Status != PendingDeletion`（店家隨帳號刪除中） | 店家未在刪除中 | **店家未在刪除中** | 準確；替代「帳號未在刪除中」較貼近賣家的認知（刪除是帳號申請的），但判斷的是店家狀態 |

### (c) select 的 `other` 後備文字（7 列）

`other` 只在 client 收到新的 `MeasureRequestKind` 值（舊版 App）時出現。

| key | 受眾 | other 文字 | 建議 |
|---|---|---|---|
| `seller.MeasureRequestState.with_kind.PENDING` | default | 請求審核中 | 採 |
| `….with_kind.ACCEPTED` | default | 請求已通過 | 採 |
| `….with_kind.REJECTED` | default | 請求已駁回 | 採 |
| `….with_kind.PENDING` | staff | 請求待裁定 | 採 |
| `….with_kind.ACCEPTED` | staff | 請求成立 | 採 |
| `….with_kind.REJECTED` | staff | 請求駁回 | 採 |
| `….with_kind.CLOSED` | staff | 請求已結案 | 採 |

`seller.MeasureState.ACTIVE` 的 `other {停權中}` 已由 L0 D.2 核可。`MeasureBasis` 的標題改用名詞參數（§1.3），不需要 `other` 文字（審查建議的「風險」不再需要）；ground 不認得時用「風險措施」（L0 B3 已核可的用語替換）。

### (d) 其他：兩端不同、L′ 不收斂的句子（O9「兩端字不同的列給 Will」）

| 情形 | W（`measurePeriodText`，期間行） | S（`measureEffect`，效果行，依狀態） | 建議 |
|---|---|---|---|
| 停權有結束時間 | 停權期間：{start} 至 {end} `measureDisplay.ts:186` | 停權中（至 {end}）（生效中）／已於 {end} 結束（已結束） `measure_display.dart:306,312` | **L′ 不收斂，各端保留**（只照 L0 B 改用語；這組本來就沒有舊詞）：兩端是不同的資訊設計（W 固定顯示期間、S 依狀態顯示影響），統一是版面決定；I-sweep 抽成各端自己的資源 |
| 停權沒有結束時間 | 停權期間：{start} 起，未定結束時間，平台定期複審 `:188` | 停權中（未定結束時間，平台定期複審） `:306` | 同上 |

L′ 沒有其他新文字：§1.5 的訊息句、變體與名詞都取自 L0 已核可或兩端現行逐字相同的字（套 L0 B 的用語）。行為上的修正（不是文案）：seller-app 食品登錄的送出日期改用台北時區（§4.3）。

---

## 8. 子 commit 計畫

共通：每個 commit 在本機跑完所列的檢查才提交；Conventional Commits、說明繁體中文、`git commit --only`；一次 push（Actions 額度，AGENTS.local）。驗收寫來源；變異檢查是「故意弄壞、確認會紅，再還原」。
Flutter 依 AGENTS.local：開發中跑 analyze、custom_lint、改到的測試；commit 前兩支 App 各跑一次完整測試與 `bash scripts/deadcode.sh`；有 `part` 生成檔的檔改動就跑 build_runner。website 的 build 與 e2e 帶 `API_BASE_URL=http://127.0.0.1:9/api/v1`（build 再加 `NEXT_PUBLIC_API_BASE_URL`）。buf 先確認 `buf --version` 等於 `api/.buf-version`。

順序：L′-1 → L′-2 →（L′-3、L′-4 可對調）→ L′-5 → L′-6 → L′-7 → L′-8，最後做「最終狀態檢查」（不是 commit）。v1 的 L′-6（buyer-app）併入 L′-4，後面的編號往前移。

前置（L′-1 開工前）：§7 全部核可。

### L′-1 `feat(uitext)`：生成器、目錄、兩個套件、drift 與跨端守門（沒有消費端）

- 檔案：
  - `backend/cmd/uitext-gen/**`；`backend/internal/platform/uitext/{uitext.go,plural.go,catalog_gen.go,uitext_test.go,catalog/{config,zh-TW,en}.json,testdata/golden.gen.json}`；`backend/internal/platform/uitext/icu/**`；`backend/Makefile`（`uitext`）；`api/check-buf-version.sh`（新）、`api/generate.sh`（改呼叫它）。
  - `packages/hawkr-ui-text-ts/{package.json,tsconfig.json,biome.json,vitest.config.ts,src/**,tests/**,tools/check-enum-text.ts}`（比照 `packages/hawkr-twaddr-ts`）。
  - `packages/hawkr_ui_text/{pubspec.yaml,pubspec.lock,.fvmrc,.gitignore,analysis_options.yaml,lib/**,test/**,tool/check_enum_text.dart,tool/enum_manifest.g.json}`（`.fvmrc`、`.gitignore` 比照 `packages/hawkr_twaddr`，審查 D6）。
  - `pnpm-workspace.yaml`（packages、catalog、註解）、`pnpm-lock.yaml`、`website/package.json`（`next-intl: catalog:`，版本不變）。
  - `scripts/check-enum-text-cross.sh`；`.github/workflows/{ui-text-generated-diff.yml,ui-text-guard.yml,ts-packages.yml,dart-packages.yml}`（§5.3）。
  - 根目錄 `AGENTS.md`：地圖加兩列（兩個套件）、契約表加「共用文案目錄」一列（唯一來源 `backend/internal/platform/uitext/catalog`、消費端）；`backend/AGENTS.md`：platform 清單加 `uitext`、指令段加 `make uitext`。
  - ADR 文末修訂（審查 D18）：「D2 的『窮舉由 Dart `switch` 保證』改為由生成器保證：protoc_plugin 的 enum 是 class，switch 不檢查窮舉，生成碼以 `_` 分支回『未知』」。
  - 目錄內容只放 §1.5 中 L′-1 就能定的部分（名詞、變體、訊息句、通用）；`text/` 的原因句在 L′-2。
- 驗收：
  - `GOWORK=off go test -race ./cmd/uitext-gen/... ./internal/platform/uitext/...`：parser（子集、`#`／`<` 被擋、撇號、plural 分支）、key 解析（巢狀 enum、`admin.seller` 最長相符、訊息句）、§2.2 第 1–7、9 條各一條失敗案例、golden 由參考格式器產生且 Go emitter 全數相符（ADR D2、審查 D8）。
  - `make -C backend uitext && git diff --exit-code`；`make lint-monolith`（只看新增）；`go test ./internal/arch/...`。
  - `pnpm --filter @hawkr/ui-text lint test`：golden 對 `use-intl` `createTranslator` 與 `formatStaff`；名詞參數巢狀解出；en 缺字回 zh-TW；數字參數輸出 `1234`（不分組）。
  - `cd packages/hawkr_ui_text && flutter pub get && dart analyze && dart format --output=none --set-exit-if-changed lib test tool && dart test`（golden 全數相符）。
  - 守門工具對測試夾具：含 `Record<MeasureKind,string>` 漢字值的檔報錯；單端 enum 的表不報錯；`--report` 正確。
  - `bash scripts/check-enum-text-cross.sh` 在 HEAD 為 0（L0 的掃描：共用的 enum 都已在目錄；若不為 0，停下回報，那是 §7 沒涵蓋的文案）。
- 變異：刪 zh-TW 一個值 → 生成失敗且點名 key；加 `seller.MeasureKind.MEASURE_KIND_UNSPECIFIED` → 失敗；加 `common.error.CANCELED` → 失敗；select 寫不存在的值名 → 失敗；訊息含 `#` 或 `<b>` → 失敗；en.json 加 `staff` 段 → 失敗；`omit.default` 列的值仍有 default 文字 → 失敗；手改 `catalog_gen.go` 一個字 → drift 紅；改 golden 一筆的 want → Go、TS、Dart 三邊都紅；夾具裡 website 與 seller-app 對同一個非目錄 enum 都有表 → 跨端紅。
- 審查：backend（生成器與 platform），website 與 app 各看一眼套件 API。

### L′-2 `feat(seller)`：賣家端三個原因 enum 改成 D-L2 形狀（契約＋四端同一個 commit）

- 檔案：
  - 契約：`api/hawkr/seller/v1alpha1/seller.proto`、`api/hawkr/common/v1alpha1/error_reason.proto`；三端生成物（`backend/internal/httpapi/gen/hawkr/{seller,common}/v1alpha1/*`、`packages/hawkr-api-ts/src/gen/hawkr/{seller,common}/v1alpha1/*`、`packages/hawkr_api/lib/gen/hawkr/{seller,common}/v1alpha1/*`）。
  - backend：`modules/seller/internal/text/{zh-TW.json,en.json,text_gen.go}`（新）；`modules/seller/internal/model/{measure.go,model.go,measure_test.go}`；`modules/seller/internal/measure/{appeal.go,remediation.go}`；`modules/seller/internal/onboarding/onboarding.go`；`modules/seller/{seller.go,governance.go}`；`platform/twtime/{twtime.go,twtime_test.go}`（`MinuteCeil`）；`httpapi/rpc/{errors.go,rpc_test.go}`；`httpapi/seller/{measures.go,app.go,governance_measures.go,measures_test.go,app_test.go,governance_test.go}`；integration：`modules/seller/{measure_integration_test.go,remediation_integration_test.go,termination_availability_integration_test.go,termination_lifecycle_integration_test.go}`；`cmd/uitext-gen` 開啟 §2.2 第 8 條；`make uitext` 的輸出。
  - website：`lib/seller/{measures.ts,onboarding.ts}`、`features/seller/measures/{measureDisplay.ts,AppealEntry.tsx,RemediationEntry.tsx,SellerMeasureRequestPage.tsx}`、`features/seller/onboarding/SellerOnboardingClient.tsx`；測試（`rg -l 'AppealUnavailableReason|RemediationUnavailableReason|StoreOpeningUnavailableReason|unavailableReasons' website -g '!**/gen/**'` 的結果，審查 D16）：`lib/seller/measures.unit.test.ts`、`features/seller/measures/{measureDisplay,SellerMeasureDetailPage,SellerMeasureRequestPage,SellerMeasuresPage}.unit.test.ts`、`features/seller/onboarding/SellerOnboardingClient.unit.test.ts`、`features/seller-center/components/SellerSuspendedNotice.unit.test.ts`、`tests/seller-e2e-fixtures.test.ts`、`tests/e2e/helpers/mock-gateway.ts`。
  - seller-app：`lib/features/seller/data/{measure_display.dart,measures.dart,seller_onboarding.dart}`、`lib/features/seller/presentation/seller_info_screen.dart`；測試（同式對 `seller-app`）：`test/features/seller/data/{measure_display_test,measures_provider_test,measures_test,seller_onboarding_test,termination_display_test}.dart`、`test/features/seller/presentation/{seller_appeal_screen_test,seller_info_screen_test,termination_ui_test}.dart`、`test/support/{measure_fixtures.dart,measure_fixtures_test.dart}`。
- 驗收：
  - Go 資料集測試（labels-brief D-L2 第 2 點）：對每種 measure 情境，`len(Unavailabilities) == 0 ⇔ can_*`；每個原因在每個 required 語系文字非空；錯誤的 `LocalizedMessage` 等於可用性回應第一筆的 `message`（同一筆 measure）。開店同理。COOLDOWN：`15:05:01` → 句子含 `15:06`，`15:05:00` → `15:05`（twtime 測試＋句子測試）。
  - handler 測試斷言 wire 形狀與 `locale == "zh-TW"`；STORE_EXISTS 仍是 `already_exists`。
  - `buf lint api`；`buf breaking`（v1alpha1 略過）；api-contract 的 TS、Dart drift；`backend-monolith`（`go test -race ./...`、gofmt、tidy）；`GOWORK=off go test -race -tags integration ./internal/modules/seller/...`。
  - website：`pnpm lint`、`pnpm test`、`pnpm build`、e2e 的賣家措施與開店 spec。seller-app：analyze、custom_lint、改到的測試；commit 前完整測試與 deadcode。
  - `make -C backend uitext && git diff --exit-code`。
- 變異：讓 `errAppealUnavailable` 用另一句字面值 → 「錯誤文字等於可用性文字」紅；把某個 `*UnavailableReason` 改回 `repeated enum` → 生成器第 8 條紅；e2e mock 拿掉 `message` → 對應 spec 紅（不紅就補斷言伺服器文字）；`MinuteCeil` 改成捨去 → COOLDOWN 測試紅。
- 審查：backend、website、app。

### L′-3 `refactor(seller)`：Go 改用目錄、伺服器句子的用語

- 檔案：`model/measure.go`（刪 `terminationGroundLabels`、`riskGroundLabels`、兩個 `Label()`；`ActionUnavailableText` 的「申訴」→「申覆」，L0 B6）、`model/measure_test.go`（刪 `TestRiskGroundLabelsEqualTheProtoLabels`，加 `TestCatalogCoversModuleValues`）、`measure/create.go:108`（`uitext.Staff().SellerTerminationGround`）、`measure/appeal.go:46,170,189,201`、`measure/remediation.go:85,91`、`measure/revoke.go:32`（L0 B6）、`internal/legacy/sellerreason/reason.go:76,89` 與測試（舊契約固定 zh-TW 預設受眾）、`httpapi/seller/declaration.go:88`、`cmd/hawkr/devseed.go:298`（L0 B1 附註）、seller 模組的程式註解與 `CARRYOVER.md` 的沿用盤點；斷言這些字串的測試。
- 驗收：`rg -n 'Label\(\)' backend/internal/modules/seller backend/internal/legacy/sellerreason` 為空；`GOWORK=off go test -race ./...`；seller 模組 integration；`make lint-monolith`。
- 變異：在目錄刪 `seller.TerminationGround` 一個值並重新生成 → 生成器先紅；跳過生成器讓 `catalog_gen.go` 少一個 case → `TestCatalogCoversModuleValues` 紅。
- 審查：backend。

### L′-4 `refactor(api)`：拿掉 `description` 退路、Connect 預設文字改由目錄（四端）、buyer-app 品牌

- 檔案：`packages/hawkr-api-ts/src/connect/{errors.ts,submit.ts}` 與測試；`packages/hawkr_api/lib/{errors.dart,submit.dart}` 與測試；website 與 admin 的 `userMessage`／`submitError` 呼叫點（`rg -l '\b(submitError|userMessage|defaultErrorText)\(' website admin`，21 檔）；website `package.json`、`next.config.ts`、`lib/intl/config.ts`、`lib/i18n/ui-text.ts`（只放 `useDefaultErrorText`、`defaultErrorTextFor`、`uiTextFor`）；admin `package.json`、`next.config`；`pnpm-lock.yaml`；seller-app 7 檔、buyer-app 3 檔的呼叫點；兩支 App 的 `lib/core/l10n/ui_locale.dart`（只有 `publishedUiLocales`、`resolveUiLocale`、`uiLocaleProvider`）、`pubspec.{yaml,lock}`；buyer-app `lib/shared/widgets/top_navigation.dart:312`；`.github/workflows/{website-ci,admin-ci,seller-ci,buyer-ci}.yml` 的 paths（§5.3）。
- 驗收：`rg -n 'description' packages/hawkr-api-ts/src/connect/errors.ts packages/hawkr_api/lib/errors.dart` 只剩註解；`rg -n '參數錯誤|系統錯誤' packages/hawkr-api-ts/src packages/hawkr_api/lib` 為空；api-contract 的兩個套件 job、website／admin 的 lint、test、build、兩支 App 的 `fvm flutter pub get`（驗 intl 範圍，§10 R3）、analyze、完整測試、deadcode。
- 變異：伺服器回 BadRequest 只有 `description` 的情境測試 → 欄位文字是「參數錯誤」；把 `?? v.description` 加回 → 該測試紅；`submitError` 的 fields 分支不補 defaults → 「欄位文字非空」測試紅。
- 審查：website、admin、app（各看自己的呼叫點）。

### L′-5 `refactor(website)`：website 改用目錄、用語

- 檔案：§4.1 全部（L′-2 已處理的原因不再碰）。
- 驗收：`pnpm lint`（含守門）、`pnpm test`、`pnpm build`（**路由表裡賣家中心與買家頁仍是靜態／ISR，沒有新出現的 ƒ**，§10 R2）、`pnpm test:e2e` 的賣家 spec、`pnpm design:check --apps-root ..`。L0 B2／B3 的每個檔逐一 `rg` 舊詞為空（測試檔同步改）。
- 變異：在 `measureDisplay.ts` 加回一個 `Partial<Record<MeasureKind,string>>` → lint 紅；`missingItemsOf` 改回依 label 去重，餵 BANK_NAME＋BANK_CODE → 單元測試紅（期望一列「銀行名稱」）。
- 審查：website（只換字，版面不變）。

### L′-6 `refactor(seller-app)`：seller-app 改用目錄、用語；接上 App 的守門

- 檔案：§4.3 全部（含 `packages/hawkr_api/lib/taipei_time.dart` 的 `formatTaipeiDate` 與測試）；`lib/core/l10n/ui_locale.dart` 加 `BuildContext` extension；`.github/workflows/_flutter-quality.yml` 的 `enum_text` step；`seller-app/Makefile` 的 `enum-text`；兩支 App `AGENTS.md` 的驗證指令。
- 驗收：analyze、custom_lint、守門對兩支 App 都是 0（buyer-app 在 L′-4 已無目錄 enum 的表）、完整測試、deadcode、build_runner 比對（若有）；L0 B4 的舊詞 `rg` 為空；食品登錄日期測試（UTC 16:30 的時間點在台北是隔天）。
- 變異：在 `measure_display.dart` 加回 `switch (kind) { MeasureKind.MEASURE_KIND_WARNING => '警告', … }` → 守門紅；`missingItemsOf` 依 label 去重 → 測試紅（期望兩列）。
- 審查：app。

### L′-7 `refactor(admin)`：admin 改用目錄（staff）、用語；`kyc_status` 改 optional

- 檔案：§4.2、§4.6：`api/hawkr/admin/seller/v1alpha1/seller.proto`；三端生成物（Dart 不生成 admin）；`backend/internal/httpapi/seller/foodreg.go`、新檔 `foodreg_test.go`、`admin_integration_test.go:816`；admin 的檔與測試；admin `lint` 接守門。
- 驗收：`buf lint`、生成 drift、`backend-monolith`、admin `pnpm lint`、`pnpm test`、`pnpm build`；desktop、mobile、keyboard smoke（admin AGENTS）。food-registrations 的清單與詳情：沒有 KYC 申請的店顯示「尚未申請」。
- 變異：`toAdminFoodRegistration` 在 nil 時設 `KycStatus = UNSPECIFIED` → `foodreg_test.go`（斷言 `KycStatus == nil`）與 integration 紅（審查 D12）；在 `measure.ts` 加回 `TERMINATION_GROUND_LABEL` → 守門紅。
- 審查：admin、backend（契約）。

### L′-8 `docs(api)`：proto 註解的用語與「標籤」

- 檔案：`api/**/*.proto`（§6）；三端生成物；`make uitext` 同步的 `description`。
- 驗收：§6 的三條 `rg`；`buf lint`；三端生成 drift；ui-text drift。
- 審查：backend（契約文字）。

### 最終狀態檢查（push 前，對整個 L′ 的 diff；不是 commit）

- `website/messages/zh-TW.json` 的 scaffold 不刪（I-web-0／I-sweep），只確認目錄掛在 `uitext` 不撞名。
- 沒有雙規則：`rg -n 'Partial<Record<' website/features/seller website/lib/seller` 為空；四端守門與跨端檢查 0；四個 `*UnavailableReason` 只出現在 `*Unavailability.reason`；`rg -n '處置與申訴|措施與申訴' website admin seller-app` 為空；§1.5 訊息句對應的 W／S 字面值 `rg` 為空（例 `rg -n '終止生效：|申覆期限：|若本次經駁回' website/features seller-app/lib -g '!**/*.test.*'`）。
- 看 CI 時看每個 job 自己的結論（含新的 `ui-text-generated-diff`、`ui-text-guard`、`ts-packages` 的 ui-text、`dart-packages` 的 ui_text）。

### 範圍外（記進 Pending）

- **PayUni 狀態文案**：website `lib/seller/payuni.ts:61-84` 與 seller-app `features/seller/data/payuni.dart` 由 client 從多個欄位加 `canResubmit` 推導狀態，違反 2026-10-15「伺服器推導」的前提。依 L-brief「知會」末項的先例另排一刀：伺服器給狀態，文字再依 D-L1／D-L2 規則進目錄或伺服器句。Pending：「PayUni 狀態改由伺服器推導（client 推導的狀態文案不進 L′）」。**已結案（2026-10-21，第 5a 刀）**：狀態放進 `KYCFlow.payuni`，文字進共用目錄與伺服器句（`step10e-5a-design.md`；計畫 10e 的 5a 完成紀錄）。
- **buyer-app 地址**：`address_repository.dart` 以中文字反解 `_labelOf`，`Address.label` 應存 `AddressLabel`。Pending：「buyer-app 的地址標籤改存 enum」；website 地址改用 proto enum 時，AddressLabel 才因兩端使用而進目錄。
- admin 自己寫的「不能做」原因句（§4.2）；`app.dart` 的 `supportedLocales`（I-app-0）。

---

## 9. 已定的工程決定（原 v1 的待決，依審查分流）

| # | 題目 | 結論 | 依據 |
|---|---|---|---|
| O1 | SellerRequirement 4 個值 | 補名詞，Will 核可（§7(b)）；不用變體只收 3 值（等於換名字的「不顯示清單」） | Q4、D-L1 |
| O2 | D-L2 句子放哪 | 現在就放 seller 模組 `internal/text/*.json`＋生成；admin 的 `ActionUnavailableText` 在 L′ 不搬、只改用語，I-server 一起處理 | ADR D3、D-L2 |
| O3 | COOLDOWN 的時間 | 伺服器以台北時間組進句子，L′ 只做 zh-TW 版面＋`twtime.MinuteCeil`；句子由 Will 核可（§7(a)） | labels D-L2 第 2 點 |
| O4 | 守門範圍 | 四端一律擋「在目錄」與「伺服器擁有」的 enum，加跨端檢查（兩個以上 client 有表就失敗）；AddressLabel 不進目錄 | L0 A5、ADR D2 分界、審查 |
| O5 | Dart 沒有 `BuildContext` 的地方 | 每支 App 一個 `uiLocaleProvider` | 工程細節 |
| O6 | website 回傳字串的純函式 | 第一個參數收 `ut: UiText` | 工程細節 |
| O7 | KycFlowCopy 多狀態共用文字 | 每個值一條，文字重複；不加別名語法、不另加「改一半」的 lint | 工程細節 |
| O8 | 缺項去重 | 依控制項：website 銀行是一個控制項 → 列「銀行名稱」；seller-app 是兩個控制項 → 各列一次（已查證，§0） | L0 A1 |
| O9 | W／S 共同的組合句 | W 與 S 逐字相同、只由契約值加已格式化參數（含日期）決定的都收（§1.5 訊息句）；兩端不同的列給 Will（§7(d)）；畫面固定文字留給 I-sweep | Q3、ADR D2 |
| O10 | `required` 與已發佈語系 | `catalog/config.json` 寫 `required: ["zh-TW"]`；各介面的已發佈清單由 I-web-0、I-app-0 建立，I-CI 檢查 ⊆ | ADR D1 |
| O11 | 說明怎麼給譯者 | `-write` 把 `description` 同步進 zh-TW 來源檔（`descriptionOverride` 可覆寫、`note` 補充），en 不放說明 | ADR D2、審查 D14 |
| O12 | admin KYCStatus SUBMITTED | 改「待審核」 | L0 A2「順手對齊」，Will 全照建議 |

---

## 10. 風險與 10d 的協調

風險：

- R1 生成器要讀 googleapis 的依賴（`buf build` 走 BSR）：離線時跑不了。CI 正常；本機離線照 wiki `local-runbook.md`「離線 buf drift」。
- R2 website 的 ISR：RSC 呼叫 `getTranslations()` 會讀 header、把頁變成動態。設計用帶明確 locale 的 `createTranslator`；L′-5 驗收看路由表。
- R3 Dart 的 `intl` 範圍要包含兩支 App 解析到的 0.20.2；L′-4 在兩支 App 跑 `fvm flutter pub get` 驗證，L′-1 先在套件內以同一個範圍解析。
- R4 純函式加參數會改到很多單元測試；e2e 斷言的文字也會隨 L0 用語與 §7 的新句改。同一個 commit 改測試，不放寬斷言。
- R5 `omit` 依受眾手寫：若某端其實有顯示被 omit 的值，會拿到「沒有文字」。L′-2、L′-5、L′-6 的驗收逐一對照現行行為（CLOSED 在賣家端不顯示是現況：W `latestDecidedRequest`、S `_statusRequest`）。
- R6 守門只用名稱比對：同名的非 proto 識別字會誤判（目前沒看到）；三元式漏網。
- R7 跨端檢查在 L′-1 對 HEAD 必須是 0；若不是，代表有 L0 沒盤到的共用 enum 文字，要停下補文案核可，不能加例外。

10d 的熱點（`local/10d3id1` 與最近的 10d commit）：`backend/services/social-service`、`backend/gateway/internal`、`website/features/seller-center/posts`、`website/lib/api`、`website/lib/seller`（36fb5d894 動過兩個檔）、`seller-app/lib/features/post`、`seller-app/lib/shared`、`buyer-app/lib/core`、`backend/api/openapi/openapi.yaml`、`scripts/legacy-endpoints-baseline.json`。10d 的貼文狀態是舊 REST（social），不是 proto enum，目前不受守門影響。

L′ 會碰、要協調的共用檔：

| 檔 | 處理 |
|---|---|
| `pnpm-lock.yaml`、`pnpm-workspace.yaml`、`website/package.json`、`admin/package.json`、兩支 App 的 `pubspec.{yaml,lock}` | 衝突以重新 install／pub get 解決，不手合 lockfile |
| `website/next.config.ts`（`transpilePackages`） | 一行；合併時保留雙方 |
| `website/lib/seller/*`、`SellerSuspendedNotice.tsx`、`SellerOnboardingClient.tsx`、`FoodRegSupplement.tsx` | 開工前 `git diff --name-only main...local/10d3id1 -- website/lib/seller website/features/seller-center/components` 確認 |
| `api/` 改動後的三端生成物 | 一律重新生成，不手合；L′-2、L′-7、L′-8 開工前 `git diff --name-only main...local/10d3id1 -- api/` 確認 10d 沒有同檔改動 |
| `.github/workflows/{_flutter-quality,website-ci,admin-ci,seller-ci,buyer-ci,ts-packages,dart-packages}.yml` | 只加行；合併時保留雙方 |
| `seller-app/lib/app.dart`、`main.dart`、`buyer-app/lib/main.dart` | L′ 不碰（b′ 的好處，I2） |

L′ 不改 `openapi.yaml`、legacy baseline、social、gateway。合併 main 時若 10d 帶進新的 proto enum 文字表，四端守門或跨端檢查會在合併後紅，要在合併 commit 一起搬進目錄（新文案先給 Will 核可）。

---

## 11. 相對 v1 的變更紀錄

| 審查項 | v2 的處理 | 位置 |
|---|---|---|
| D1 HIGH AddressLabel | 不進目錄；前提「W 與 B 同字」更正（website 用字串）；L′-6 併入 L′-4 只剩品牌；`_labelOf` 記 buyer-app 待辦 | §0、§1.5、§4.4、§8 |
| D2 HIGH 原因句沒列 | 新增 §7(a)：14 列，含現行各端文字、檔案行號、最終句、取代的顯示位置；Go 通用句刪除 | §3.2、§3.4、§7(a) |
| D3 omit 不能依受眾 | `omit` 改成依受眾宣告 | §1.3、§2.2 |
| D4 select other、ground 的形式 | 新增名詞參數（同語系同受眾的目錄名詞，不是 client 拼接），title／meta 用它；`other` 文字列 §7(c) | §1.3、§1.5、§7(c) |
| D5 en 矛盾、缺字行為 | en 只放有 message 的 key、`-write` 不補；缺字三端都回 zh-TW（生成時決定），golden 各斷言 | §1.3、§2.4 |
| D6 Dart 套件檔案、format | 檔案清單加 `.fvmrc`、`.gitignore`；format 涵蓋 `tool` | §5.3、§8 L′-1 |
| D7 CI 接線 | 每條 paths 分配到 commit；ts-packages 加 `packages/hawkr-api-ts/**`；App 守門改成獨立 step、先 pub get | §5.2、§5.3 |
| D8 跨 runtime 等價 | 生成共用 golden，Go、Dart、TS（use-intl 與 formatStaff）四邊斷言 | §2.4、§8 L′-1 |
| D9、O9 有日期的句子 | 收進訊息句（新增 key 種類 `<pkg>.<Message>.<name>`）；兩端不同的列給 Will | §1.2、§1.5、§7(d) |
| D10 `<`、`#` | 子集禁止 `<`、`>`、`#`；整數參數以 `{n}` 輸出、不分組 | §1.4 |
| D11 FieldError 預設文字 | 只在 `submitError`／`userMessage` 補；TS `submitError(err, defaults, method?)` 保留既有 `method` | §4.5 |
| D12 kyc_status 變異 | 加 Go `foodreg_test.go` 斷言 `KycStatus == nil`，並改 integration 斷言 | §8 L′-7 |
| D13 繞過 buf 版本檢查 | 抽出 `api/check-buf-version.sh`，generate.sh 與 Makefile 共用 | §2.1 |
| D14 說明不能覆寫 | 加 `descriptionOverride` | §1.3 |
| D15 巢狀 enum key | descriptor 解析（最長相符 package、逐段找型別） | §1.2 |
| D16 測試清單 | 附 `rg -l` 指令與實際檔案 | §8 L′-2 |
| D17 extension 沒人用 | `BuildContext` extension 延到 L′-6；buyer-app 不建 | §2.5、§8 |
| D18 ADR 修訂沒分配 | 放 L′-1 | §8 L′-1 |
| D19 TS devDeps | 列出 devDeps 與 catalog、註解修改；`next-intl`／`use-intl` 進 catalog 保證同版 | §2.5 |
| D20 submitted_note | 改成 `MeasureRequestKind` 變體；reply_note 改訊息句 | §1.5 |
| D21 payload | 記一筆，I-web-0 處理 | §2.5 |
| 小錯 intl | 更正為 buyer-app 經其他套件帶入 | §0 |
| O2–O12 | 依分流定案，§8 待決改成 §9 已定決定 | §9 |
| PayUni | 範圍外＋Pending | §8 範圍外 |
| 自行查到 | STORE_EXISTS 是 AlreadyExists，不改成 FailedPrecondition；REQUEST_PENDING 排第一必為解除請求；seller-app 食品登錄日期用裝置時區（修正）；`account.dart` 沒有錯誤 helper，從 L′-4 清單拿掉；buyer-app 沒有 Makefile；`rpc_test.go:116` 已斷言 FieldViolation 文字非空（already protected，不另加測試） | §0、§3.2、§4.3、§4.5、§5.2 |
| 編號 | v1 L′-7／8／9 → v2 L′-6／7／8 | §8 |

## 12. 實作時的修訂

L′-1 實作與第 1 輪審查後的修正，以此節為準，與前文不同處以這裡為準。

| 項目 | 修訂 | 原因 |
|---|---|---|
| 跨端檢查的 CI 啟用 | `scripts/check-enum-text-cross.sh` 與兩支守門工具在 L′-1 就存在並有單元測試，但跑跨端檢查的 `ui-text-guard.yml` 改到 L′-2 才加（連同 seller-app 與 website 的 `AppealUnavailableReason`、`RemediationUnavailableReason` 進目錄）。各 client 對目錄 enum 的規則照 §8 在各 client 的子 commit 接上 | L′-1 HEAD 狀態跑跨端檢查會因這兩個 enum 在 seller-app 與 website 同時手寫而紅，L′-1 不得讓 CI 變紅 |
| Dart 守門的 enum 認定 | 只算從 `package:hawkr_api` 的 `.pb.dart`、`.pbenum.dart` 引入的類別（以 manifest 的 `dartLibrary` 比對，尊重 `show`／`hide`）；TS 本來就只認 `@hawkr/api/gen/…` 的 import | 兩支 App 的 CodeMeta `ReportStatus` 與 proto enum 同名，只比類別名會誤報 |
| 守門工具的失敗語意 | 工具 exit 0 與 1 之外、缺 report、report 不是 JSON、少任何一個 client 的 report，跨端腳本都 exit 2；TS 工具未捕捉的例外回 2 | node 的未捕捉例外也是 exit 1，會和「有命中」混淆而讓檢查變綠 |
| golden 的預設受眾表 | TS 測試對 zh-TW 且沒有 staff 案例的 key，另以 `formatStaff` 比對 golden（走 `zh-tw.gen.ts` 的 `DEFAULT`） | §2.4 要求比對預設受眾版 |
| 生成物清單 | `tests/golden-calls.gen.ts`、`test/golden_calls.g.dart` 也是生成物，不得手改，drift 的觸發路徑涵蓋整個 `tests/`、`test/` | 原清單漏列 |
| Dart 套件的 `intl` | 釘 `0.20.2`（Flutter 3.41.2 的 `flutter_localizations` 所釘），測試跑兩支 App 實際用的版本 | 範圍約束會解析到 0.20.3 |
| `common.error.*` 的數量 | 15 個（Connect 共 16 個 code，扣掉 CANCELED），§2 與驗證清單的「16 個」更正 | 計算錯誤 |
| ADR 修訂 | 「Dart 與 TS 的窮舉都由生成器保證」：TS 生成碼用 `Partial<Record<number, …>>`，型別不檢查窮舉 | 原句誤稱 TS 的 `Record` 不變 |
| 發票「補正期限→補件期限」只改介面（B7-4） | §4.1 的「補正期限改補件期限」只適用使用者介面文字（如 `TaxExtensionSection`）。`selfIssueTerms.ts` 的 `SELF_ISSUE_DEADLINE_NOTICE`（「60 天是平台的資料補正期限…」）是版本化法律條款正文的逐字投影，維持「補正期限」；條款正文不在 L0 範圍 | 條款逐字顯示，改字等於改條款版本；Will 2026-10-17 決定（B7-4） |
| L′-5 表單標籤與狀態後備 | 認證表單控制項標籤取共用文案目錄的 KYCField 名詞（D1，主要商品類型、公司地址、出生日期、存摺封面、銀行名稱、銀行帳號、負責人身分證字號等；銀行選擇器同時涵蓋 BANK_NAME 與 BANK_CODE，標籤取欄位號最小者「銀行名稱」）；`kycFlowStatusLabel` 的 UNSPECIFIED 與不認得的狀態走 common「未知」（D.3）；PayUni 問題欄位與缺項清單共用 `fieldsByControl` 去重（O8）；介面名詞「記錄」一律「紀錄」（術語表） | 缺項清單與表單標籤須同名；兩端同一個值須顯示同一個字 |
| L′-6 seller-app 的表單控制項與 canceled | seller-app 的銀行名稱與銀行代碼是兩個輸入框，各自一個控制項、各自取目錄名詞（「銀行名稱」「銀行代碼」，表單標籤原本的「金融機構代碼」改「銀行代碼」）；個人與公司負責人共用的身分證字號輸入框是同一個控制項，缺項與 PayUni 問題依控制項去重、取欄位號小者（「身分證字號」），輸入框與確認頁的標籤依身分取 INDIVIDUAL／BUSINESS_REP 的名詞；確認頁的列標籤也取同一組名詞。請求被取消（目錄沒有 canceled 的文字，`userMessage` 回 null）而路徑一定要顯示文字時，統一用 `hawkr_api` 的 `errorText`（伺服器句子，沒有時 code 的預設文字，取消當 internal，與 `submitError` 同一條規則）；buyer-app 的 auth（登入路徑與 getProfile 等）、address、recipe repository 與 seller-app 的 auth repository 共五處都改用它，刪除寫死的「登入失敗，請稍後再試」「地址操作失敗」「無法載入食譜資料」等句子（這些句子帶的動作脈絡目錄預設文字沒有，依 D11 不保留）。聯絡人的確認頁列標籤與表單標籤都取目錄 STORE_FIELD_CONTACT_NAME「聯絡人姓名」；食品登錄審核中沒有送出時間時日期留空（同 website） |
| L′-6 未知請求種類 | seller-app 對不認得的請求種類顯示目錄 `other` 的文字（§7(c)，Will 已核可）。L′-9 起 website 也一樣：刪除 `isKnownRequestKind`／`knownRequests` 的過濾，不認得的種類（含 UNSPECIFIED）與 seller-app 同樣走 `with_kind` 的 `other` 分支（「請求審核中」「請求已駁回」…）；待處理請求的說明行用 `submitted_note` 變體，不認得的種類沒有說明行，兩端相同 | §7(c) 的 `other` 是給收到新值的舊版 client 用的；§12 L′-5 的原則是「兩端同一個值須顯示同一個字」 | 缺項清單與表單標籤須同名；§4.5 的 canceled 規則（`submitError` 的 message 分支把 Canceled 當 Internal） |
| L′-7 建立終止的生效時機由伺服器給 | 建立終止對話框第一句（預告「建立後 30 天生效（時間）」或「立即生效」）不進共用目錄（admin 單端、依事由與時間，不符 §1.5），也不在 client 以三元式或 `TERMINATION_NOTICE_DAYS` 組：後台 `GetStoreResponse.available_terminations`（取代 2c-3 的平行陣列 `available_termination_grounds`，field 8 與名稱保留不重用；每個可建立的事由一筆，空清單即不能建立）與 `LinkedAccount.termination_option`（`can_create_linked_termination` 為 true 時，單一值）各給 `TerminationOption`（`ground`、排程生效時的 `effective_time`、`explanation` 的 LocalizedMessage，zh-TW、受眾員工）。生效時間來自 `model.TerminationStart`，`CreateMeasure` 排定 `starts_at` 與效果清單都只呼叫它；兩句放在 seller 模組的 `internal/text/zh-TW.json`（`termination.effect.immediate`、`termination.effect.scheduled`，後者的天數取 `NoticePeriod`、時間為台北 `YYYY/MM/DD HH:mm`），文字與舊 client 句子逐字相同。效果以讀取當下為建立時間推算，實際生效時間仍以建立後的 `period.start_time` 為準。共同說明與關聯帳號的來源說明維持 client 常數（沒有分支、沒有計算） | 預告期是伺服器的業務規則，client 不得有副本；可用性與原因由伺服器推導、client 只呈現（Will 2026-10-15 前提、D-L2、2c-3 的 `can_create_termination` 先例）；審查指出三元式繞過守門 R2；`TestTerminationAvailabilityIsTheSameRuleAsCreateMeasure` 擴充：效果時間等於 `CreateMeasure` 排的 `StartsAt`；AIP 與同 repo 先例（`unavailabilities`）都是一個 repeated message 帶 reason／ground 加 LocalizedMessage，平行陣列「同序同長」只靠註解維持；httpapi integration 對 `GetStore` 與 `GetMeasure` 的線上形狀逐字斷言 |
| L′-7 admin 單端 enum 的未知值 | `measureBasisText` 在種類或依據不認得時整句回「未知」；`endCauseLabel`、`kycIssueCodeLabel` 的 UNSPECIFIED 與不認得的值回「未知」 | §4.2：admin 單端 enum 的 UNSPECIFIED 與不認得的值一律走 common「未知」 |
| L′-9 設計審查的修正（Will 2026-10-18 全部照辦） | (1) 傳輸層錯誤文字刪除 `errors.go` 的四個手寫常數與 `session.go` 的 `textUnavailable`，一律取生成的 `CommonError`（語系取自 ctx 的 `uitext.LocaleFrom`）；(2) TS 加 `errorText(err, defaults)`（`@hawkr/api/connect/errors`，與 Dart `errorText`、`submitError` 同一條規則），admin 的 `userMessage(…) ?? "…"` 全部換成 `lib/error-text.ts` 的 `errorMessage`，並在 `check-enum-text` 加 R3 守門：`userMessage(…) ?? 字串` 會失敗；(3) `termination.effect.scheduled` 的 `days` 改 integer 加 plural；(4) 名詞函式對 UNSPECIFIED 與不認得的值三端都回「沒有文字」（Go `("", false)`、TS `undefined`、Dart `null`），「未知」由呼叫端明確用 `commonUnknown`（刪 website `knownText`、seller-app `knownUiText`，admin 的 `unknownText` 比對也刪）；(5) 模組句子可宣告 staff 受眾（來源語系檔的 `staff` 段，只有 zh-TW，生成器擋下 en 的 staff 文字與 staff 段），`ActionUnavailableText` 的七句與 L′-7 的終止生效說明都搬進 staff；(6) website 的未知請求種類同 seller-app（上一列）；(7) `uitext.FormatDateTime(l, t)` 是伺服器文字唯一的時間版面；(8) `DefaultErrorText` 只在 `@hawkr/api/connect/errors` 定義，`@hawkr/ui-text` re-export；(9) 生成的模組句子與 `CommonError` 回 `uitext.Message{Text, Locale}`，`rpc.LocalizedMessage(m)` 的 locale 取自 `m.Locale`；(10) `resolveUiLocale(languageCode, published)` 移進 `hawkr_ui_text`，兩支 App 只宣告已發佈清單 | 審查的發現（B）：同一個來源兩份、用字串當哨兵、受眾沒有機械檢查、locale 與文字分開傳；根目錄 AGENTS.md「沿用也是決策」 | 設計 §2.5 的 API 草圖（名詞回 `UiMessage`／`String`、`LocalizedMessage(l, text)`、`resolveUiLocale(Locale?)`）以這列為準；apperr 的訊息仍標 `errorLocale`（zh-TW），隨 I-server |
| L′-9 canceled 維持 internal（審查發現未採納） | 取消當成 `Code.Internal`，顯示「系統錯誤」，不改。理由：兩支 App 沒有任何使用者發起、會讓 Connect 呼叫回 canceled 的路徑（`rg -n 'AbortSignal' seller-app/lib buyer-app/lib` 0 筆）；website 與 admin 引用 Connect 的檔案裡只有 TanStack Query 的 `queryFn: ({ signal })`（`rg -l '@connectrpc/(connect\|connect-query)' website/lib website/features website/app admin/src --glob '!**/gen/**' --glob '!*.test.*' \| xargs rg -n 'AbortController\|AbortSignal\|signal'`，唯一的呼叫端是 `SearchResultsClient.tsx`），該 library 對被取消的查詢不進錯誤狀態、不顯示文字。所以顯示 canceled 錯誤文字的路徑只可能來自伺服器或傳輸層，那時通用的「系統錯誤」是對的。觸發：新增使用者發起的取消路徑時，該呼叫端吞掉 canceled、什麼都不顯示。reserved 欄位號照留（標準的 proto 衛生，成本為零） | 審查 [A]：canceled 顯示「系統錯誤」的理由只剩「現行如此」 | 同上 |

## 修訂（2026-10-21，第 5c 刀）

設計 `step10e-5c-design.md` v1.2（Will 2026-10-21 F1 選 α）。本文以下幾處的前提不成立，原文保留：

- `:34`、`:209`、`:537` 的「AddressLabel 只有 buyer-app 一端顯示」不成立：buyer-app 的標籤選擇器與顯示在 2026-07-16 品牌改版就刪了（`d09f9fa61`），自那天起任何 client 都不顯示標籤，值全由 client 編造（5c 設計 H1、H10）。因此 AddressLabel 不進目錄的結論仍對，但理由是沒有 client 顯示，不是只有一端顯示。
- `:823` 的 Pending（buyer-app 的地址標籤改存 enum；website 地址改用 proto enum 時 AddressLabel 才進目錄）結案：地址標籤已整個刪除（`AddressLabel` 與 `Address.label` 從契約、單體、DB 移除，buyer-app 的 `_labelOf` 隨之刪除），沒有東西可以進目錄。

## 修訂（2026-10-22，第 5d 刀）

設計 `step10e-5d-design.md` v1.2（Will 2026-10-22 決定）。本文以下幾處的前提不成立，原文保留：

- `:209` 的「Gender（字串 wire 值，L0 A5）」不成立：wire 值自 `5315cda8d` 起就是 proto `Gender` enum，兩個以上 client 手寫文字表違反根 `AGENTS.md`。`buyer.Gender.GENDER_MALE`、`GENDER_FEMALE`、`GENDER_OTHER` 與句子 `buyer.Profile.gender_unset`（沒有填性別；也是選單裡清除性別的那一項）已進目錄，三端的 `Gender` 在 manifest 的 `inCatalog` 為真。`GENDER_UNSPECIFIED` 仍不收（§1.2：目錄不收值號 0）：契約改 `optional Gender`，沒有填由欄位有沒有設值表達。
- `:52` 的「L′ 是 `{}`（ADR：英文內容之後補）」自 5d 起不再成立：`en.json` 有四個 key（Male、Female、Other、Not set，Will 2026-10-22 核可），部分填寫照 §1.3（只放有 `message` 的 key）；生成器只對 required 語系（zh-TW）檢查完整，`make uitext` 印 `en：4/190`，缺的以 zh-TW 補。ADR 的「未發佈語系只回報覆蓋率」不變：兩個介面的已發佈語系仍只有 zh-TW。

## 修訂（2026-10-22，目錄 enum 文字守門第 2 刀）

設計 `enum-text-guard-design.md` v1.3、ADR D2 文末修訂「目錄分界」（Will 2026-10-22）。

- §7(d)：seller-app 的三句（停權中（至 {end}）等）自本刀起由目錄提供（`seller.Measure.suspension_active_until`、`suspension_active_open`、`suspension_ended`；ADR D2 修訂）；website 的期間句不變。
- §5.2 的 R1／R2／D1／D2 由規則 `E` 取代（該設計書，第 3 刀落地）；R6 的三元式漏網自第 3 刀起補上。
