-- =====================================================
-- Shipping Service 資料庫初始化腳本
-- =====================================================

-- updated_at 自動更新觸發器函數
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- -----------------------------------------------------
-- 1. 物流單主表 shipments
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS shipments (
    id                    BIGINT PRIMARY KEY,
    shipment_no           VARCHAR(64) NOT NULL,
    order_no              VARCHAR(64) NOT NULL,
    seller_account_id     BIGINT NOT NULL,
    buyer_account_id      BIGINT,                     -- 退貨物流時可為 NULL
    shipping_method       VARCHAR(30) NOT NULL,
    mer_id                VARCHAR(32),
    ship_trade_no         VARCHAR(64),
    uber_delivery_id      VARCHAR(128),
    status                VARCHAR(4) NOT NULL,
    provider_status       VARCHAR(20),                    -- PayUni ShipStatus / Uber status（遵循收斂規範 §4.1 SoT）
    provider_message      TEXT,                           -- 第三方狀態說明 / 錯誤訊息（原 provider_status_desc 改名）
    provider_status_time  TIMESTAMPTZ,                    -- PayUni 回報狀態時間（webhook StatusTime，§3.4 里程碑）
    ship_type             VARCHAR(10),
    lgs_type              VARCHAR(10),
    goods_type            VARCHAR(10),
    service_type          VARCHAR(10) NOT NULL,
    trade_amt             BIGINT NOT NULL,
    sender_name           VARCHAR(50) NOT NULL,
    sender_phone          VARCHAR(20) NOT NULL,
    sender_city           VARCHAR(20),
    sender_district       VARCHAR(20),
    sender_address        VARCHAR(200),
    sender_zip_code       VARCHAR(10),
    recipient_name        VARCHAR(50) NOT NULL,
    recipient_phone       VARCHAR(20) NOT NULL,
    recipient_address     VARCHAR(200),
    store_id              VARCHAR(20),
    store_name            VARCHAR(100),
    partner_id            VARCHAR(10),
    validation_no         VARCHAR(10),
    spec                  VARCHAR(10),
    delivery_time_tag     VARCHAR(10),
    -- 列印狀態機（LabelPrintStatus，§2.4.5.1）：4101 NONE / 4102 REQUESTED / 4103 SUCCEEDED / 4104 FAILED
    label_print_status        VARCHAR(4) NOT NULL DEFAULT '4101',
    label_print_requested_at  TIMESTAMPTZ,
    -- 黑貓叫車狀態機（TcatCallStatus，§2.4.5.2）：4201 NONE / 4202 REQUESTED / 4203 SUCCEEDED / 4204 FAILED
    tcat_call_status          VARCHAR(4) NOT NULL DEFAULT '4201',
    tcat_call_requested_at    TIMESTAMPTZ,
    tcat_call_time            TIMESTAMPTZ,
    label_object_key      VARCHAR(512),
    label_asset_id        BIGINT,
    label_url             VARCHAR(512),
    label_expire_at       TIMESTAMPTZ,
    prod_desc             VARCHAR(200),
    odno                  VARCHAR(20),
    receipt_confirmed     BOOLEAN NOT NULL DEFAULT FALSE,
    is_contact            VARCHAR(1) DEFAULT 'Y',
    is_trolley            VARCHAR(1) DEFAULT 'N',
    memo                  VARCHAR(100),
    sender_snapshot       JSONB,
    receiver_snapshot     JSONB,
    shipment_photos       JSONB,
    is_return             BOOLEAN NOT NULL DEFAULT FALSE,
    last_reconciled_at    TIMESTAMPTZ,
    processing_deadline   TIMESTAMPTZ,                      -- 賣家處理期限（UTC），逾期由排程自動取消。NULL = 無期限 / 已通知
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT chk_shipping_method CHECK (shipping_method IN ('3001', '3002', '3003')),
    CONSTRAINT chk_trade_amt_non_negative CHECK (trade_amt >= 0),
    CONSTRAINT chk_label_print_status CHECK (label_print_status IN ('4101', '4102', '4103', '4104')),
    CONSTRAINT chk_tcat_call_status CHECK (tcat_call_status IN ('4201', '4202', '4203', '4204'))
);

-- 索引
CREATE UNIQUE INDEX IF NOT EXISTS uk_shipments_shipment_no ON shipments(shipment_no);
CREATE UNIQUE INDEX IF NOT EXISTS idx_shipments_order_no ON shipments(order_no) WHERE is_return = FALSE;
CREATE INDEX IF NOT EXISTS idx_shipments_return ON shipments(order_no) WHERE is_return = TRUE;
CREATE INDEX IF NOT EXISTS idx_shipments_ship_trade_no ON shipments(ship_trade_no) WHERE ship_trade_no IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_shipments_uber_delivery_id ON shipments(uber_delivery_id) WHERE uber_delivery_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_shipments_seller ON shipments(seller_account_id, status);
CREATE INDEX IF NOT EXISTS idx_shipments_status ON shipments(status);
CREATE INDEX IF NOT EXISTS idx_shipments_created_at ON shipments(created_at);
-- §5.9 賣家處理期限掃描用部分索引：僅包含有期限的正向物流單
CREATE INDEX IF NOT EXISTS idx_shipments_processing_deadline
    ON shipments(processing_deadline)
    WHERE processing_deadline IS NOT NULL
      AND is_return = FALSE;

-- §5.19 物流貨態補收排程掃描用部分索引：終態與 Uber 不入索引
CREATE INDEX IF NOT EXISTS idx_shipments_reconcile
    ON shipments(last_reconciled_at NULLS FIRST, created_at)
    WHERE status NOT IN ('11', '41')
      AND shipping_method <> '3003'
      AND ship_trade_no IS NOT NULL;

-- §5.19.3.B 列印狀態機補收排程掃描：僅卡死 / 失敗待重試者入索引
CREATE INDEX IF NOT EXISTS idx_shipments_label_reconcile
    ON shipments(label_print_status, label_print_requested_at)
    WHERE label_print_status IN ('4102', '4104');

-- §5.19.3.C 黑貓 REQUESTED ops 告警掃描：僅 REQUESTED 卡死者入索引
CREATE INDEX IF NOT EXISTS idx_shipments_tcat_reconcile
    ON shipments(tcat_call_status, tcat_call_requested_at)
    WHERE tcat_call_status = '4202';

DROP TRIGGER IF EXISTS update_shipments_updated_at ON shipments;
CREATE TRIGGER update_shipments_updated_at
    BEFORE UPDATE ON shipments
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE shipments IS '物流單主表';
COMMENT ON COLUMN shipments.id IS 'TSID 主鍵';
COMMENT ON COLUMN shipments.shipment_no IS '內部物流單號（SHP-yyyyMMdd-seq6）';
COMMENT ON COLUMN shipments.status IS '內部物流狀態碼（ShippingStatus value）';
COMMENT ON COLUMN shipments.trade_amt IS '訂單金額（新台幣元）';

-- -----------------------------------------------------
-- 2. 物流狀態歷史表 shipment_status_logs
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS shipment_status_logs (
    id                    BIGSERIAL PRIMARY KEY,
    shipment_id           BIGINT NOT NULL,
    from_status           VARCHAR(4),
    to_status             VARCHAR(4) NOT NULL,
    provider_status       VARCHAR(20),
    provider_message      TEXT,
    trigger_source        VARCHAR(30) NOT NULL,
    raw_payload           JSONB,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_status_logs_shipment ON shipment_status_logs(shipment_id, created_at);
CREATE INDEX IF NOT EXISTS idx_status_logs_created ON shipment_status_logs(created_at);

COMMENT ON TABLE shipment_status_logs IS '物流狀態歷史表';

-- -----------------------------------------------------
-- 3. Webhook 冪等表 shipping_webhook_dedup
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS shipping_webhook_dedup (
    id                    BIGSERIAL PRIMARY KEY,
    event_key             VARCHAR(255) NOT NULL,
    ship_trade_no         VARCHAR(64),
    process_status        VARCHAR(20) NOT NULL,
    processed_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uk_dedup_event_key UNIQUE (event_key)
);

CREATE INDEX IF NOT EXISTS idx_dedup_processed ON shipping_webhook_dedup(processed_at);

COMMENT ON TABLE shipping_webhook_dedup IS 'Webhook 冪等表';

-- -----------------------------------------------------
-- 4. 賣家物流建店設定 seller_shipment_settings
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS seller_shipment_settings (
    id                      BIGINT PRIMARY KEY,
    seller_account_id       BIGINT NOT NULL,
    shipping_type           VARCHAR(30) NOT NULL DEFAULT '3651',  -- ShippingType.CVS_C2C
    goods_type              VARCHAR(10),
    -- 寄件人/退貨聯絡人資訊（宅配+超商共用）
    sender_name             VARCHAR(50),           -- 寄件人姓名（超商=退貨聯絡人姓名）
    sender_phone            VARCHAR(20),           -- 寄件人電話（超商=退貨聯絡人電話）
    -- 宅配專用：寄件地址
    sender_city             VARCHAR(20),           -- 寄件城市
    sender_district         VARCHAR(20),           -- 寄件地區
    sender_address_detail   VARCHAR(200),          -- 寄件詳細地址
    sender_postal_code      VARCHAR(10),           -- 寄件郵遞區號（供運費路線推導）
    -- 宅配專用：退貨地址（目前與寄件地址共同維護，未來可拆開）
    return_name             VARCHAR(50),           -- 退貨收件人姓名
    return_phone            VARCHAR(20),           -- 退貨收件人電話
    return_city             VARCHAR(20),           -- 退貨城市
    return_district         VARCHAR(20),           -- 退貨地區
    return_address_detail   VARCHAR(200),          -- 退貨詳細地址
    return_postal_code      VARCHAR(10),           -- 退貨郵遞區號
    -- 超商專用：退貨門市
    return_store_code       VARCHAR(20),
    return_store_name       VARCHAR(100),
    return_store_address    VARCHAR(200),
    label_company_name      VARCHAR(20),
    product_category_code   VARCHAR(10),
    mer_id                  VARCHAR(32) NOT NULL,
    partner_id              VARCHAR(20),
    eship_id                VARCHAR(20),
    provider_status         VARCHAR(20),
    provider_message        TEXT,
    onboarding_status       VARCHAR(4) NOT NULL,
    enabled                 BOOLEAN NOT NULL DEFAULT TRUE,
    -- PayUni 建店送審狀態機（PayUniSubmitStatus，§2.4.5.3）：4301 NONE / 4302 REQUESTED / 4303 SUCCEEDED / 4304 FAILED
    payuni_submit_status        VARCHAR(4) NOT NULL DEFAULT '4301',
    payuni_submit_requested_at  TIMESTAMPTZ,
    payuni_submitted_at         TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT chk_shipping_type CHECK (shipping_type IN ('3651', '3652')),  -- ShippingType
    CONSTRAINT chk_onboarding_status CHECK (onboarding_status IN ('3601', '3602', '3603', '3604', '3605')),
    CONSTRAINT chk_payuni_submit_status CHECK (payuni_submit_status IN ('4301', '4302', '4303', '4304'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_settings_seller_type
    ON seller_shipment_settings(seller_account_id, shipping_type, COALESCE(goods_type, ''));
CREATE INDEX IF NOT EXISTS idx_settings_seller ON seller_shipment_settings(seller_account_id);
CREATE INDEX IF NOT EXISTS idx_settings_status ON seller_shipment_settings(onboarding_status);

-- §5.19.3.D ops 告警掃描：僅 REQUESTED 卡死者入索引
CREATE INDEX IF NOT EXISTS idx_settings_submit_reconcile
    ON seller_shipment_settings(payuni_submit_status, payuni_submit_requested_at)
    WHERE payuni_submit_status = '4302';

DROP TRIGGER IF EXISTS update_seller_shipment_settings_updated_at ON seller_shipment_settings;
CREATE TRIGGER update_seller_shipment_settings_updated_at
    BEFORE UPDATE ON seller_shipment_settings
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE seller_shipment_settings IS '賣家物流建店設定';
COMMENT ON COLUMN seller_shipment_settings.id IS 'TSID 主鍵';
COMMENT ON COLUMN seller_shipment_settings.shipping_type IS '物流類型 (ShippingType value): 3651=CVS_C2C / 3652=HOME_DELIVERY';
COMMENT ON COLUMN seller_shipment_settings.onboarding_status IS '建店狀態: PENDING_REVIEW / ACTIVE / REJECTED / CLOSED / PERMANENTLY_CLOSED';

-- -----------------------------------------------------
-- 5. 建店操作日誌 seller_shipment_onboarding_logs
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS seller_shipment_onboarding_logs (
    id                      BIGSERIAL PRIMARY KEY,
    setting_id              BIGINT NOT NULL,
    seller_account_id       BIGINT NOT NULL,
    action_type             VARCHAR(20) NOT NULL, -- OnboardingLogAction value（8701=PAYUNI_SUBMIT / 8702=LOCAL_UPDATE）
    request_snapshot        JSONB NOT NULL,
    response_snapshot       JSONB,
    result_status           VARCHAR(20) NOT NULL, -- OnboardingLogResult value（8801=SUCCESS / 8802=FAILED）
    error_message           VARCHAR(500),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_action_type CHECK (action_type IN ('8701', '8702')),
    CONSTRAINT chk_result_status CHECK (result_status IN ('8801', '8802'))
);

CREATE INDEX IF NOT EXISTS idx_onboarding_logs_setting ON seller_shipment_onboarding_logs(setting_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_onboarding_logs_seller ON seller_shipment_onboarding_logs(seller_account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_onboarding_logs_created ON seller_shipment_onboarding_logs(created_at);

COMMENT ON TABLE seller_shipment_onboarding_logs IS '建店操作日誌';

-- -----------------------------------------------------
-- 6. 配送方式設定 shipping_method_configs
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS shipping_method_configs (
    id                      BIGSERIAL    PRIMARY KEY,
    shipping_method         VARCHAR(50)  NOT NULL,
    delivery_temp           VARCHAR(20)  NOT NULL,
    max_dimension_sum_cm    INTEGER      NOT NULL DEFAULT 0,
    max_single_side_cm      INTEGER      NOT NULL DEFAULT 0,
    max_weight_grams        BIGINT       NOT NULL DEFAULT 0,
    estimated_days          INTEGER      NOT NULL DEFAULT 0,
    settlement_days         INTEGER      NOT NULL DEFAULT 0,
    commission_rate         NUMERIC(5,4) NOT NULL DEFAULT 0,
    commission_min          BIGINT       NOT NULL DEFAULT 0,
    enabled                 BOOLEAN      NOT NULL DEFAULT true,
    created_at              TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ    NOT NULL DEFAULT NOW(),

    CONSTRAINT uk_smc_method_temp UNIQUE (shipping_method, delivery_temp)
);

DROP TRIGGER IF EXISTS update_shipping_method_configs_updated_at ON shipping_method_configs;
CREATE TRIGGER update_shipping_method_configs_updated_at
    BEFORE UPDATE ON shipping_method_configs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE shipping_method_configs IS '配送方式設定（物流限制與營運參數）';
COMMENT ON COLUMN shipping_method_configs.shipping_method IS '配送方式代碼（TAQBIN / SEVEN_ELEVEN_STORE_PICKUP）';
COMMENT ON COLUMN shipping_method_configs.delivery_temp IS '配送溫層代碼（AMBIENT / CHILLED / FROZEN）';
COMMENT ON COLUMN shipping_method_configs.max_dimension_sum_cm IS '材積上限 L+W+H（cm），0 表示不限';
COMMENT ON COLUMN shipping_method_configs.max_single_side_cm IS '最長邊上限（cm），0 表示不限';
COMMENT ON COLUMN shipping_method_configs.max_weight_grams IS '重量上限（公克），0 表示不限';
COMMENT ON COLUMN shipping_method_configs.estimated_days IS '預計到貨天數';
COMMENT ON COLUMN shipping_method_configs.settlement_days IS '撥款天期 D+N 的 N';
COMMENT ON COLUMN shipping_method_configs.commission_rate IS '代收手續費率（0.0050 = 0.5%）';
COMMENT ON COLUMN shipping_method_configs.commission_min IS '代收手續費最低金額（新台幣元）';

-- 初始資料
INSERT INTO shipping_method_configs (shipping_method, delivery_temp, max_dimension_sum_cm, max_single_side_cm, max_weight_grams, estimated_days, settlement_days, commission_rate, commission_min) VALUES
('3002', '3201', 105, 45, 5000, 3, 7, 0.0050, 1),
('3002', '3203', 105, 45, 10000, 3, 7, 0.0050, 1),
('3001', '3201', 150, 0, 20000, 2, 15, 0.0100, 20),
('3001', '3202', 120, 0, 20000, 2, 15, 0.0100, 20),
('3001', '3203', 120, 0, 20000, 2, 15, 0.0100, 20);

-- -----------------------------------------------------
-- 7. 運費費率級距表 shipping_fee_tiers
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS shipping_fee_tiers (
    id                  BIGSERIAL    PRIMARY KEY,
    shipping_method     VARCHAR(50)  NOT NULL,
    delivery_temp       VARCHAR(20)  NOT NULL,
    delivery_zone       VARCHAR(30)  NOT NULL,
    max_dimension_cm    INTEGER      NOT NULL DEFAULT 0,
    fee                 BIGINT       NOT NULL,
    enabled             BOOLEAN      NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ    NOT NULL DEFAULT NOW(),

    CONSTRAINT uk_sft_method_temp_zone_dim UNIQUE (shipping_method, delivery_temp, delivery_zone, max_dimension_cm)
);

DROP TRIGGER IF EXISTS update_shipping_fee_tiers_updated_at ON shipping_fee_tiers;
CREATE TRIGGER update_shipping_fee_tiers_updated_at
    BEFORE UPDATE ON shipping_fee_tiers
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE shipping_fee_tiers IS '運費費率級距表，依配送方式×溫層×區域×材積查找運費';
COMMENT ON COLUMN shipping_fee_tiers.shipping_method IS '配送方式代碼';
COMMENT ON COLUMN shipping_fee_tiers.delivery_temp IS '配送溫層代碼';
COMMENT ON COLUMN shipping_fee_tiers.delivery_zone IS '配送區域（DeliveryZoneKey value：8101=DOMESTIC / 8102=OFFSHORE / 8103=INTER_ISLAND）';
COMMENT ON COLUMN shipping_fee_tiers.max_dimension_cm IS '材積上限 L+W+H（cm），0 表示統一價';
COMMENT ON COLUMN shipping_fee_tiers.fee IS '運費金額（新台幣元）';

-- 初始資料（共 32 筆）
-- 7-11 店到店（統一價，不分區域）
INSERT INTO shipping_fee_tiers (shipping_method, delivery_temp, delivery_zone, max_dimension_cm, fee) VALUES
('3002', '3201', '8101', 0, 65),
('3002', '3203', '8101', 0, 165),
-- 黑貓常溫（3 區域 × 4 級距 = 12 筆）
('3001', '3201', '8101', 60, 125),
('3001', '3201', '8101', 90, 165),
('3001', '3201', '8101', 120, 205),
('3001', '3201', '8101', 150, 245),
('3001', '3201', '8102', 60, 230),
('3001', '3201', '8102', 90, 290),
('3001', '3201', '8102', 120, 330),
('3001', '3201', '8102', 150, 370),
('3001', '3201', '8103', 60, 450),
('3001', '3201', '8103', 90, 570),
('3001', '3201', '8103', 120, 650),
('3001', '3201', '8103', 150, 730),
-- 黑貓冷藏（3 區域 × 3 級距 = 9 筆）
('3001', '3202', '8101', 60, 160),
('3001', '3202', '8101', 90, 225),
('3001', '3202', '8101', 120, 290),
('3001', '3202', '8102', 60, 270),
('3001', '3202', '8102', 90, 350),
('3001', '3202', '8102', 120, 410),
('3001', '3202', '8103', 60, 530),
('3001', '3202', '8103', 90, 690),
('3001', '3202', '8103', 120, 810),
-- 黑貓冷凍（3 區域 × 3 級距 = 9 筆，與冷藏同費率）
('3001', '3203', '8101', 60, 160),
('3001', '3203', '8101', 90, 225),
('3001', '3203', '8101', 120, 290),
('3001', '3203', '8102', 60, 270),
('3001', '3203', '8102', 90, 350),
('3001', '3203', '8102', 120, 410),
('3001', '3203', '8103', 60, 530),
('3001', '3203', '8103', 90, 690),
('3001', '3203', '8103', 120, 810);

-- -----------------------------------------------------
-- 8. 買家常用門市表 buyer_store_pickups
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS buyer_store_pickups (
    id                BIGINT PRIMARY KEY,
    account_id        BIGINT NOT NULL,
    store_code        VARCHAR(20) NOT NULL,
    store_name        VARCHAR(100) NOT NULL,
    store_address     VARCHAR(200) NOT NULL,
    receiver_name     VARCHAR(50) NOT NULL,
    receiver_phone    VARCHAR(20) NOT NULL,
    is_default        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bsp_account ON buyer_store_pickups(account_id);
CREATE INDEX IF NOT EXISTS idx_bsp_account_default ON buyer_store_pickups(account_id)
    WHERE is_default = TRUE;

DROP TRIGGER IF EXISTS update_buyer_store_pickups_updated_at ON buyer_store_pickups;
CREATE TRIGGER update_buyer_store_pickups_updated_at
    BEFORE UPDATE ON buyer_store_pickups
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE buyer_store_pickups IS '買家常用門市表';
COMMENT ON COLUMN buyer_store_pickups.id IS 'TSID 主鍵';
COMMENT ON COLUMN buyer_store_pickups.store_code IS '門市代碼（PayUni 門市地圖回傳，6 碼）';

-- -----------------------------------------------------
-- 9. 假日表 holidays（節假日管理）
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS holidays (
    id              BIGSERIAL PRIMARY KEY,
    holiday_date    DATE NOT NULL,
    name            VARCHAR(50) NOT NULL,
    year            SMALLINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_holidays_date ON holidays(holiday_date);
CREATE INDEX IF NOT EXISTS idx_holidays_year ON holidays(year);

COMMENT ON TABLE holidays IS '台灣國定假日與例假日（行政院人事行政總處 CSV 匯入）';
COMMENT ON COLUMN holidays.holiday_date IS '放假日期';
COMMENT ON COLUMN holidays.name IS '假日名稱（例假日/春節/補假 等）';
COMMENT ON COLUMN holidays.year IS '年份（方便按年匯入/清除）';
