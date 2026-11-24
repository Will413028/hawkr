# B7-5 admin 券管理版面（Will 2026-11-24 同意）

沿用既有 admin 版面與元件，不新增元件；後端方法為 `api/hawkr/admin/coupon/v1alpha1/coupon.proto` 的 `CouponAdminService`。

- 側欄「行銷 / 營運」在「系統公告」之後加「優惠券」→ `/admin/coupons`。
- `/admin/coupons` 兩個分頁：
  - 活動：篩選狀態、發行者（平台／店家 store id）；表格欄位名稱、發行者、折扣／門檻、期間、狀態，列點進明細。「建立平台活動」對話框照 `CreateAnnouncementDialog` 寫法：名稱、折扣（現金／百分比）、門檻、對象（全部／會員／指定名單）、期間、發放上限（「不限」與數字分開，不限不等於 0）。
  - 買家持券查詢：輸入 buyer ID，列出券（狀態、來源、訂單號、時間）；`locked`／`available` 的列可「作廢」，對話框原因必填（`VoidVoucher`）。
- `/admin/coupons/[id]`（`DetailCardShell`）：規則卡、統計卡（可用／鎖定／已用／作廢）、指定名單卡（指定名單對象才顯示）。平台活動才顯示「發布」「提前結束」「指定發放」；店家活動唯讀（D6 修訂）。提前結束對話框填原因，說明「鎖定中的券照常完成，未使用的券會作廢」，送出後顯示作廢張數；指定發放貼 buyer ID，顯示新發與已持有數。
- `/admin/merchants/[id]` 加「優惠券配額」卡：顯示現值並可設定「同時進行中的活動上限」。現值由新增的唯讀方法 `GetSellerQuota` 提供（Will 2026-11-24 選擇補方法），在 B7-5 與畫面一起做（proto、handler 宣告權限、測試）。
