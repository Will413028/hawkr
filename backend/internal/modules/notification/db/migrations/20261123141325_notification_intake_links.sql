-- Modify "inbox_items" table
ALTER TABLE "notification"."inbox_items" DROP COLUMN "intent_id", ADD COLUMN "occurred_at" timestamptz NOT NULL;
-- Modify "intents" table
ALTER TABLE "notification"."intents" ADD COLUMN "occurred_at" timestamptz NOT NULL;
-- Create "inbox_item_intents" table
CREATE TABLE "notification"."inbox_item_intents" (
  "intent_id" uuid NOT NULL,
  "inbox_item_id" uuid NOT NULL,
  PRIMARY KEY ("intent_id"),
  CONSTRAINT "inbox_item_intents_inbox_item_id_fkey" FOREIGN KEY ("inbox_item_id") REFERENCES "notification"."inbox_items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "inbox_item_intents_intent_id_fkey" FOREIGN KEY ("intent_id") REFERENCES "notification"."intents" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "inbox_item_intents_item" to table: "inbox_item_intents"
CREATE INDEX "inbox_item_intents_item" ON "notification"."inbox_item_intents" ("inbox_item_id");
