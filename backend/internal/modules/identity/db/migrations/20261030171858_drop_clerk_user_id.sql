-- Modify "accounts" table
ALTER TABLE "identity"."accounts" DROP COLUMN "clerk_user_id";
-- 已退場的 River job `identity.clerk_user` 不再有 worker：清掉佇列裡的殘留（River 的表由 jobs.Migrate 先建立，沒有時略過）。
DO $$
BEGIN
  IF to_regclass('river.river_job') IS NOT NULL THEN
    DELETE FROM river.river_job WHERE kind = 'identity.clerk_user';
  END IF;
END
$$;
