-- 賣家觸發的重讀先搶讀取名額（Stripe 計畫 D33，步驟 7e1）：seller_read_claimed_at 記下上次賣家觸發的讀取被受理的時刻（含失敗與並行的），
-- 條件式 UPDATE 搶名額，間隔內的呼叫不向 provider 讀。同步 job 不讀寫這一欄。手寫的 migration（單體規則 6）；定義與 schema.sql 相同。
ALTER TABLE payment.accounts ADD COLUMN seller_read_claimed_at TIMESTAMPTZ;
