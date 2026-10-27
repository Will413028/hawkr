-- Modify "order_item_reviews" table
ALTER TABLE "order_item_reviews" ADD CONSTRAINT "order_item_reviews_reply_consistency" CHECK ((reply_content IS NULL) = (replied_at IS NULL)), ADD COLUMN "reply_content" text NULL, ADD COLUMN "replied_at" timestamptz NULL;
