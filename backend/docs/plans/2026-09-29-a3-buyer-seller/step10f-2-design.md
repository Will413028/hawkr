# 10f-2 檢舉表單與刪除匿名食安入口，設計

來源：`step10e-reports-brief.md` R5、R7 與 §4；`step10d-decision-brief.md` D4；ADR `docs/adr/2026-10-27-general-ecommerce-scope-cut.md` D3；盤點對照 dev `5ad73cbe`。決定 G1–G2：Will 2026-11-21（照建議）。

## 前提改變

R5 落在表單上的三項（「同意平台轉報時提供我的聯絡方式」、送出當下的聯絡方式快照、食安表單寫出 1919）都只服務 FDA 轉報。範圍裁減 D3 已刪掉轉報（`case_referrals`、`GetFDAExport`，commit `6da894bf`、`36914575`），食安法的轉報義務在一般電商不成立，三項不做。

盤點發現：website 與 buyer-app 都沒有店家、商品的「⋯ → 檢舉」表單；10d D4 在 2026-10-30 移到 10f 後沒有步驟承接，`CreateReport` 沒有任何 client 呼叫［量測：`rg -l "CreateReport|createReport" website buyer-app admin --glob '!*gen*'` 無結果］。

## 決定

| # | 題目 | 結論 |
|---|---|---|
| G1 | 10f-2 的範圍 | 不做同意勾選、快照、1919；改做 website 與 buyer-app 的店面、商品頁「⋯ → 檢舉」表單，接 `CreateReport`（類別、說明、商品、訂單編號、佐證）。10f-3 的「我的通報」以它為資料來源 |
| G2 | 匿名的 `SubmitFoodSafetyReport` | 刪除。一般電商的基準（Shopee、momo、Amazon）是登入才能檢舉；匿名入口的理由是食安法。`FOOD_SAFETY` 類別與 4 小時分案期限保留給登入的檢舉 |

## 切分

- **10f-2a 刪除匿名入口（backend、契約、admin）**：刪 `SellerReportPublicService`／`SubmitFoodSafetyReport`、限流與 viewer 的對應分支（`backend/internal/httpapi/ratelimit.go`、`viewer.go`、`effects.go`、`server.go`、`seller/reports.go`）；`seller.reports` 刪 `reporter_phone`、`reporter_email`、`reporter_name`、`contact_erased_at`、`discarded_submissions`、`reports_contact_cap`，`reporter_buyer_id` 改 NOT NULL（migration 先刪既有匿名列：系統從未上線）；刪聯絡方式清除 job（`governance/jobs.go`）與 365 天上限；admin `Report.contact`、`contact_erase_time` 與 `ReporterContact` 改 reserved，`report-view.ts` 的匿名分支刪除；文件：governance ADR 文末修訂、`backend/docs/modules/seller.md`、seller `CARRYOVER.md`、`AGENTS.md`。不動：`backend/services/` 舊服務。
- **10f-2b 檢舉表單（website、buyer-app）**：店面與商品頁的「⋯」選單加「檢舉」，訪客先登入；類別清單＝`PolicyCategory` 除 `UNSPECIFIED`、`PLATFORM_ABUSE`，標籤走 `@hawkr/ui-text`／`hawkr_ui_text`；佐證照既有 staged upload；`supplemented` 時顯示伺服器的 `notice`；`resource_exhausted` 走既有重試分類。版面與文案先給 Will 同意（共通規則「UI 改動」）。

## 消費端（重跑）

```
rg -l "SubmitFoodSafetyReport|SellerReportPublicService|reporter_phone|reporter_email|contact_erased_at|contactEraseTime|ReporterContact" --glob '!**/gen/**' --glob '!*.pb.go' --glob '!node_modules' --glob '!backend/services/**' .
```

## 驗收

- 10f-2a：上面的搜尋只剩計畫與 ADR 的歷史段；backend lint／test、integration（`reports_reporter_check` 改後的 CHECK、無匿名列可插入）；`buf lint`、三端生成無 drift（`buf breaking` 依上線前規則允許，reserved 欄位號）；admin CI；`maxRESTOperations` 不變。mutation：放回允許 `reporter_buyer_id IS NULL` 的 CHECK，插入匿名列的測試要失敗。
- 10f-2b：website、buyer-app CI；單元測試鎖類別清單不含 `PLATFORM_ABUSE`、訪客點檢舉導向登入、`supplemented` 顯示伺服器句子。mutation：把 `PLATFORM_ABUSE` 加回清單，測試要失敗。website e2e 推給 CI。

## 風險

- 10f-5 權利人通報是「不必登入的表單」：它的儲存在 10f-5 的決定簡報裡定（著作權法定流程與一般通報不同），本刀刪匿名欄位不預先替它保留。
