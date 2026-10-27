-- Add role column to cms_comment (1=buyer, 2=seller)
ALTER TABLE cms_comment ADD COLUMN role SMALLINT NOT NULL DEFAULT 1;
