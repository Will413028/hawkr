-- 店家可曝光的讀取模型：由 seller.exposure.changed 事件餵入，公開讀取面以它判斷買家看不看得到店家的商品。
-- 沒有列等同不可曝光。version 是 seller 端每家店單調遞增的版本，舊版本與重送的事件不改任何東西。
CREATE TABLE store_exposure (
  store_id    BIGINT      PRIMARY KEY,
  exposable   BOOLEAN     NOT NULL,
  version     BIGINT      NOT NULL CHECK (version > 0),
  updated_at  TIMESTAMPTZ NOT NULL
);

COMMENT ON TABLE store_exposure IS
  '店家可曝光的讀取模型（seller.exposure.changed）；沒有列 = 不可曝光；version 小於或等於現有的事件丟棄';

-- 停權下架改為讀取時過濾（store_exposure），不再以快照下架與還原。
DROP TABLE product_suspension_snapshot;
