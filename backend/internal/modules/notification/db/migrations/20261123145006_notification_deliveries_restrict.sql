-- Modify "deliveries" table
ALTER TABLE "notification"."deliveries" DROP CONSTRAINT "deliveries_intent_id_fkey", ADD CONSTRAINT "deliveries_intent_id_fkey" FOREIGN KEY ("intent_id") REFERENCES "notification"."intents" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT;
