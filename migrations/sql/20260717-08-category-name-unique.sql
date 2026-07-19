-- +migrate Up
ALTER TABLE categories ADD UNIQUE INDEX uk_categories_name (name);

-- +migrate Down
ALTER TABLE categories DROP INDEX uk_categories_name;
