-- =====================================================================
-- V2：退款超額 DB 兜底約束（§5.5.5 第三層防護）
-- =====================================================================
-- 不變式：同一 trade_no 的退款總額（在途 + 成功，status IN ('0','1','2')）≤ 對應交易金額。
--
-- PostgreSQL CHECK 無法跨 row 聚合，改以 BEFORE INSERT/UPDATE trigger 實現。
-- 關鍵：trigger 內必須「先取序列化鎖再聚合」——否則 READ COMMITTED 下裸 SUM 再比對會
-- 重演 L1 的 TOCTOU（兩併發交易各讀到偏低的 SUM 都通過），L3 形同虛設。
-- 取鎖手段：SELECT ... FROM payment_transactions WHERE trade_no = NEW.trade_no FOR UPDATE
-- （trade_no 於 payment_transactions 為 UNIQUE，鎖對應交易 row，事務結束自動釋放）。
--
-- 即使 L1 行鎖 / L2 餘額計算因競態或未來新 caller 被繞過，L3 以自身序列化鎖獨立成立。
-- =====================================================================

CREATE OR REPLACE FUNCTION check_refund_total_not_exceed_transaction()
    RETURNS TRIGGER AS $$
DECLARE
    v_transaction_amount BIGINT;
    v_used_amount        BIGINT;
BEGIN
    -- 先取序列化鎖：鎖對應交易 row，序列化同 trade_no 的併發退款寫入。
    SELECT amount
      INTO v_transaction_amount
      FROM payment_transactions
     WHERE trade_no = NEW.trade_no
     FOR UPDATE;

    -- 找不到對應交易（理論上被 FK 保護，此處防禦性放行，交由應用層 / FK 處理）
    IF v_transaction_amount IS NULL THEN
        RETURN NEW;
    END IF;

    IF TG_OP = 'INSERT' THEN
        -- NEW row 尚未寫入表，聚合既有在途 + 成功退款，再顯式加上本次 NEW.amount（避免漏算本筆）。
        SELECT COALESCE(SUM(amount), 0)
          INTO v_used_amount
          FROM payment_refunds
         WHERE trade_no = NEW.trade_no
           AND status IN ('0', '1', '2');

        IF v_used_amount + NEW.amount > v_transaction_amount THEN
            RAISE EXCEPTION
                'refund total exceeds transaction amount: trade_no=%, used=%, new=%, transaction=%',
                NEW.trade_no, v_used_amount, NEW.amount, v_transaction_amount
                USING ERRCODE = 'check_violation';
        END IF;
    ELSE
        -- UPDATE：BEFORE 觸發時表內仍為 OLD 值，聚合時排除本 row，再依 NEW.status 決定是否計入本筆。
        -- 僅當本 row 在更新後仍屬在途 / 成功（'0','1','2'）才需納入不變式檢查；轉為 FAILED('3') 釋放額度不擋。
        SELECT COALESCE(SUM(amount), 0)
          INTO v_used_amount
          FROM payment_refunds
         WHERE trade_no = NEW.trade_no
           AND status IN ('0', '1', '2')
           AND id <> NEW.id;

        IF NEW.status IN ('0', '1', '2')
           AND v_used_amount + NEW.amount > v_transaction_amount THEN
            RAISE EXCEPTION
                'refund total exceeds transaction amount: trade_no=%, used=%, new=%, transaction=%',
                NEW.trade_no, v_used_amount, NEW.amount, v_transaction_amount
                USING ERRCODE = 'check_violation';
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_check_refund_total
    BEFORE INSERT OR UPDATE ON payment_refunds
    FOR EACH ROW
    EXECUTE FUNCTION check_refund_total_not_exceed_transaction();

COMMENT ON FUNCTION check_refund_total_not_exceed_transaction() IS
    '§5.5.5 L3 退款超額 DB 兜底：先 FOR UPDATE 鎖 payment_transactions 序列化，再校驗同 trade_no 退款總額（在途+成功）≤ 交易金額';
