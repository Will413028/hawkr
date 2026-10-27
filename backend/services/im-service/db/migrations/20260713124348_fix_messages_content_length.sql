-- Modify "messages" table
ALTER TABLE "messages" ADD CONSTRAINT "messages_content_length" CHECK (char_length(content) <= 4096);
