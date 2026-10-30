-- Drop "apple_tokens" table
DROP TABLE "identity"."apple_tokens";
-- 已退場的 Apple 授權撤銷 job 不再有 worker：清掉佇列裡的殘留（River 的表由 jobs.Migrate 先建立，沒有時略過）。
DO $$
BEGIN
  IF to_regclass('river.river_job') IS NOT NULL THEN
    DELETE FROM river.river_job
    WHERE kind IN ('identity.exchange_apple_code', 'identity.save_apple_token', 'identity.revoke_apple_token');
  END IF;
END
$$;
