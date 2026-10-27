-- Create recipe tables (migrated from MySQL)
-- gin_trgm_ops below needs pg_trgm; moved the CREATE EXTENSION here (originally only in the later
-- 20260430000001 migration) so replaying history on a genuinely fresh database doesn't fail here.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS recipes (
    id            BIGSERIAL PRIMARY KEY,
    title         VARCHAR(255)  NOT NULL,
    image_url     TEXT          NOT NULL DEFAULT '',
    source_url    TEXT          NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recipes_title ON recipes USING gin (title gin_trgm_ops);

CREATE TABLE IF NOT EXISTS recipe_ingredients (
    id         BIGSERIAL PRIMARY KEY,
    recipe_id  BIGINT       NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    name       VARCHAR(255) NOT NULL,
    amount     VARCHAR(255) NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_recipe_ingredients_recipe_id ON recipe_ingredients (recipe_id);
CREATE INDEX IF NOT EXISTS idx_recipe_ingredients_name ON recipe_ingredients USING gin (name gin_trgm_ops);

CREATE TABLE IF NOT EXISTS recipe_steps (
    id          BIGSERIAL PRIMARY KEY,
    recipe_id   BIGINT  NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    step_order  INT     NOT NULL DEFAULT 0,
    description TEXT    NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_recipe_steps_recipe_id ON recipe_steps (recipe_id, step_order);

CREATE TABLE IF NOT EXISTS ingredient_product_mappings (
    id                 BIGSERIAL PRIMARY KEY,
    ingredient_keyword VARCHAR(255) NOT NULL,
    product_keyword    VARCHAR(255) NOT NULL
);
