-- B-135: 加 messages 軟刪除 / 平台下架欄位（HIDE_TARGET 用）

ALTER TABLE messages
    ADD COLUMN deleted_at    TIMESTAMP WITH TIME ZONE,
    ADD COLUMN hidden_at     TIMESTAMP WITH TIME ZONE,
    ADD COLUMN hidden_reason TEXT;
