CREATE TABLE conversations (
    id BIGINT PRIMARY KEY,
    room_id VARCHAR(64) NOT NULL,
    buyer_id BIGINT NOT NULL DEFAULT 0,
    store_id BIGINT NOT NULL DEFAULT 0,
    last_message_id BIGINT DEFAULT 0,
    last_message_content TEXT DEFAULT '',
    last_message_type INTEGER NOT NULL DEFAULT 1,
    -- messages.preview_text 的對話層鏡射，供未來若要在對話列表也採用同一份權威
    -- preview 時直接讀（本次不改列表 UI，只先落地欄位）。
    last_message_preview TEXT,
    buyer_unread_count INTEGER NOT NULL DEFAULT 0,
    seller_unread_count INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    -- 停權賣家 gating 判準（14 號 §5.1 / §6.5）：對話關聯的訂單編號，僅伺服器端推導寫入。
    order_no VARCHAR(50),
    -- 訂單三方一致驗證通過時間；NULL = 未綁定訂單。
    order_binding_verified_at TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX idx_conversations_room_id ON conversations(room_id);
CREATE INDEX idx_conversations_updated_at ON conversations(updated_at DESC);
CREATE INDEX idx_conversations_buyer ON conversations(buyer_id);
CREATE INDEX idx_conversations_seller ON conversations(store_id);

CREATE TABLE messages (
    id BIGINT PRIMARY KEY,
    room_id VARCHAR(64) NOT NULL,
    sender_id BIGINT NOT NULL,
    receiver_id BIGINT NOT NULL,
    type INTEGER NOT NULL DEFAULT 1, -- 1=Text, 2=Image
    -- Bound content length (chars) to match WS maxMsgSize / gateway binding / usecase guard.
    content TEXT NOT NULL CONSTRAINT messages_content_length CHECK (char_length(content) <= 4096),
    -- 訊息的一行式摘要（推播 body / 未來任何列表使用）。card 訊息（product/order）由
    -- client 端 extra["preview"] 提供；其餘由 im-service 在寫入當下算好存入
    -- （usecase.computePreview）。NULL = 舊資料（不回填），讀取路徑 fallback「傳送了一張卡片」。
    preview_text TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    is_read BOOLEAN DEFAULT FALSE,
    seq_id BIGINT NOT NULL DEFAULT 0,
    deleted_at TIMESTAMP WITH TIME ZONE,
    hidden_at TIMESTAMP WITH TIME ZONE,
    hidden_reason TEXT
);

CREATE INDEX idx_messages_room_id ON messages(room_id);
CREATE INDEX idx_messages_site_room_created ON messages(room_id, created_at DESC);
CREATE INDEX idx_messages_sender_id ON messages(sender_id);
CREATE INDEX idx_messages_receiver_id ON messages(receiver_id);
CREATE UNIQUE INDEX idx_messages_room_seq ON messages(room_id, seq_id);
