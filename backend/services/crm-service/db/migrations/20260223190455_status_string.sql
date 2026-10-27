-- Modify "crm_customers" table
-- USING 子句將既有的整數值（0/1/2）轉換為對應的字串
ALTER TABLE "crm_customers"
  ALTER COLUMN "status" TYPE character varying(20)
    USING CASE "status"::text
      WHEN '0' THEN 'normal'
      WHEN '1' THEN 'vip'
      WHEN '2' THEN 'blacklist'
      ELSE 'normal'
    END,
  ALTER COLUMN "status" SET DEFAULT 'normal';
