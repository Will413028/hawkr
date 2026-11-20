# 10f-1 賣家端的詢問（R4）設計

來源：`step10e-reports-brief.md` R4、`step10e-3cb-design.md`（:542「10f 會改的」）；盤點對照 dev `4a80308d`。決定 F1–F7：Will 2026-11-20（照建議）。

## 決定

| # | 題目 | 結論 |
|---|---|---|
| F1 | 「從出現在賣家中心起算」 | 賣家畫面上線後，詢問建立即出現在賣家中心，起算點就是建立時間；`due_at` 的計算不改。不以「賣家第一次打開」起算（不打開就永不起算）。業界：Amazon Product Compliance Requests 從發出起算 |
| F2 | 逾期後回覆 | 可以，直到撤回或案件結案；逾期標記照舊給 admin（Zendesk Pending 的做法） |
| F3 | 回覆次數 | 一則詢問一次回覆；要補資料由 admin 另開詢問。資料模型不變 |
| F4 | 賣家看得到的詢問 | 該店全部詢問：開放中（可回覆）、已回覆（含 admin 代記的系統外回覆：管道、時間、內容）、已撤回（唯讀） |
| F5 | 新詢問的提示 | 賣家中心與 seller-app 導覽列顯示「待回覆數」徽章，數字由伺服器算；推播與 email 屬 A8 |
| F6 | 已關店的店 | 詢問開放中就能回覆；案件結案時自動撤回，不另加檢查 |
| F7 | 賣家讀自己上傳的附件 | 照申覆附件的既有做法；不寫 `evidence_accesses`（該表只記管理員讀取） |

## 改動

- 資料：`reply_channel` 加 `SELLER_CENTER`；加 `replied_by_account_id`，與 `reply_recorded_by_admin_id` 二選一（CHECK）。
- seller 模組：依 `store_id` 列出與讀取詢問、賣家回覆（文字＋附件，附件以 `mediaref.FinalizeEvidence` 用賣家帳號上傳，照 `measure/appeal.go`）、待回覆數；授權為帳號擁有該店。
- 契約：`api/hawkr/seller/v1alpha1/seller.proto` 加 `ListMyInquiries`、`GetMyInquiry`、`ReplyToInquiry`、待回覆數（可併入既有讀取）；文字走 uitext 目錄。
- client：website `app/[locale]/(seller)/seller/inquiries/`、seller-app `lib/features/seller/`，照 measures／appeal 的頁面與上傳；徽章接導覽列。
- admin：刪 `CaseInquiries.tsx:26-27`、`WithdrawInquiryDialog.tsx:17,56` 的「賣家端畫面上線前請另外聯絡賣家」；回覆顯示管道「賣家中心」與回覆的帳號。
- 不做：詢問事件與通知（A8）、通報人當詢問對象。

## 驗收

- Go 單元與 integration：別家店的帳號讀或回覆被拒；撤回後、已回覆後回覆被拒；逾期後可回覆；二選一 CHECK。
- mutation：拿掉擁有者檢查，跨店測試要失敗；拿掉「已回覆不可再回覆」，重複回覆測試要失敗。
- CI 等價：backend lint／test、`buf lint`/`breaking`、三端生成無 drift、website、admin、seller-app 的 CI 檢查；`make uitext` 無 drift。
