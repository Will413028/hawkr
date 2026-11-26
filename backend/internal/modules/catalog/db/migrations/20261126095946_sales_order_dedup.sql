-- Create "sales_orders" table
CREATE TABLE "catalog"."sales_orders" (
  "order_no" text NOT NULL,
  "recorded_at" timestamptz NOT NULL,
  PRIMARY KEY ("order_no"),
  CONSTRAINT "sales_orders_no_check" CHECK ((char_length(order_no) >= 1) AND (char_length(order_no) <= 200))
);
