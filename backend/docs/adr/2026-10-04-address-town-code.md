# 台灣地址以內政部鄉鎮市區代碼表示，client 的對照表由同一份資料生成

| 項目 | 值 |
|---|---|
| 狀態 | accepted（儲存：Will 2026-10-01，W-D；共用訊息與 client 的對照：Will 2026-10-04） |
| 日期 | 2026-10-04 |
| 影響範圍 | `internal/platform/twaddr`、`internal/modules/buyer`（收貨地址）、`internal/modules/seller`（店家、KYC 地址）、`api/hawkr/common`、`internal/legacy/buyerserver`、website、admin、buyer-app、seller-app、`packages/hawkr-twaddr-ts` |
| 計畫 | `docs/plans/2026-09-29-a3-buyer-seller.md` 步驟 7a、8j |

## 脈絡

舊服務的地址存中文的縣市與行政區名稱：buyer 的 `city`、`district` 是自由文字，seller 以 `pkg/twaddr` 的名稱對照查郵遞區號並另存 `postal_code`，行政區不驗證。四個 client 各有一份手抄的名稱表（以中文名為鍵、用「台」），彼此與伺服器都可能不一致。

7a 搬 seller 時決定改存內政部的 8 碼鄉鎮市區代碼（前 5 碼是縣市），表放在 `internal/platform/twaddr`（國土測繪中心代碼服務，2026-10-01 下載，22 個縣市、368 個鄉鎮市區，加上以名稱對照的 3 碼郵遞區號）；這個決定只記在計畫與 seller `CARRYOVER.md`。8j 把 buyer 的收貨地址改過來時，要決定地址訊息怎麼共用、client 怎麼把代碼轉成名稱。

約束：
- 外部：Java order-service（結帳時把地址轉給 shipping 做 Uber 報價）與 Go crm-service 讀 buyer 的 legacy `GetAddress`／`GetAddressList`，期待中文名稱；crm 在 A6 才搬進單體，order 在 B 階段改寫。
- 外部：App 的更新要經商店審核，內建的資料可能比伺服器舊。代碼極少變動，兩支 App 有版本門檻可以強制更新（`packages/hawkr_app_update`）。
- 繼承：7a 的「顯示文字在呈現層」（計畫步驟 7a）；現在仍成立：名稱是呈現，API 只傳識別。

## 考慮過的方案

**儲存**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：地址元件以權威機關的代碼表示與驗證，可推導的值不另存（ISO 3166-2 的行政區代碼；Google libaddressinput 以代碼驗證行政區） | 舊資料與 legacy 的消費端要轉換 | ✓ 只存 `town_code`，縣市取前 5 碼，郵遞區號由 twaddr 推導 |
| 存中文名稱，寫入時對照驗證 | 「臺／台」等異體字、改名時要改資料；郵遞區號另存會不一致 | ✗ |

**API 的形狀**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：一起修改的欄位做成一個子訊息，跨模組共用的訊息放在共用 package（Google AIP 的 `google.type.PostalAddress`；API 規範 §一、§七） | seller 的平面欄位要一起改，mask 的路徑變成子訊息 | ✓ `hawkr.common.v1alpha1` 的地址訊息（`town_code`、`address_detail`），buyer 與 seller 共用 |
| 維持平面欄位、只統一欄位名稱 | 明細離開鄉鎮就不成立，平面欄位讓 mask 可以只改其中一個 | ✗ |

只有所在地、沒有明細的欄位（例如公開店家的 `town_code`）不是地址，維持平面。

**client 怎麼把代碼轉成名稱與郵遞區號**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：小而少變的參考資料由單一來源生成到各平台（libphonenumber 以一份 metadata 生成各語言版本） | App 內建的表可能比伺服器舊 | ✓ 從 `twaddr` 的 CSV 生成 TS（website）與 Dart（`packages/hawkr_twaddr`，兩支 App 共用），CI 比對漂移 |
| 唯讀的 `ListTowns`，client 啟動時取得 | 多一支 RPC、離線與快取的處理；為極少發生的變動付每次啟動的成本 | ✗（重新評估的終局） |
| 回應帶 `OUTPUT_ONLY` 的縣市、鄉鎮名稱與郵遞區號 | 違反「顯示文字在呈現層」，每個回應都重複 | ✗ |

名稱用內政部的寫法（「臺」）：權威來源如此，PayUni 已經送「臺」，Uber 那端由 shipping 自己換字。

## 決策

- 台灣地址只存內政部 8 碼鄉鎮市區代碼與明細；縣市、郵遞區號由 `internal/platform/twaddr` 推導。寫入時以 twaddr 驗證，DB 以 CHECK 擋格式。
- 地址訊息放在 `hawkr.common.v1alpha1`，buyer 與 seller 共用；明細最多 255 字，收貨地址必填，店家可以只填鄉鎮市區。
- legacy 邊界轉成中文名稱給還沒搬的服務；查不到代碼代表不變量被破壞，回 Internal。
- client 的對照表由 twaddr 的同一份 CSV 生成，取代手抄表。

## 後果

- 上線前沒有正式資料：既有的中文地址列由 migration 刪除，seed 改成代碼。
- 內政部代碼異動時要更新 twaddr、重新生成、發 App；沒更新的 App 在版本門檻強制更新前會顯示不出新代碼。
- 重新評估：第一次遇到內政部代碼異動，或 App 版本落差造成顯示問題時，改成伺服器提供 `ListTowns`。

## 修訂：退場的代碼（2026-10-04，Will）

8j 的 design review 指出：內政部廢止或合併代碼後，如果直接從 twaddr 刪掉，已經存在 buyer、seller、KYC 的舊代碼會查不到，legacy 邊界回 Internal，Java 結帳與 crm 的地址清單整份失敗。原決策段保留歷史。

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：代碼表只增不刪，棄用的代碼保留並標明取代關係（ISO 3166 的 formerly used codes、Unicode CLDR 的 deprecated 與 replacement） | 表要多一個退場標記與取代代碼 | ✓ |
| 每次異動附 data migration 改寫舊代碼 | 一分為二等無法自動對應的情況要人工處理，每次異動都要寫轉換 | ✗ |

決策：twaddr 只增不刪。退場的代碼保留，拒絕新的寫入，讀取時仍轉得出名稱與郵遞區號；client 的對照表同樣保留，編輯時要求重選。退場標記在第一次遇到內政部代碼異動時實作（現在沒有退場的代碼，不預先加欄位）。

## 修訂：TS 對照表改成共用套件（2026-10-09，A3b 10e）

admin 的店家與 KYC 審核也要把代碼轉成名稱。TS 的生成物與查詢從 website 搬到 `packages/hawkr-twaddr-ts`（`@hawkr/twaddr`），website 與 admin 共用，和 Dart 的 `packages/hawkr_twaddr` 對稱；不採 admin 複製一份或直接顯示代碼。原決策段保留歷史，基準不變（單一來源生成到各平台）。
