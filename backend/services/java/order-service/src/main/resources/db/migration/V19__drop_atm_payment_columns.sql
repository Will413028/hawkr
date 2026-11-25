-- ATM 付款與 PayUni 頁面版退款隨 PayUni 金流刪除：付款與退款只經 payment 模組（Stripe）。
-- 只有這兩條流程讀寫的欄位一併刪除。
ALTER TABLE orders
    DROP COLUMN atm_bank_code,
    DROP COLUMN atm_virtual_account,
    DROP COLUMN atm_expire_date,
    DROP COLUMN need_refund_page;
