-- +goose Up
-- +goose StatementBegin
CREATE TABLE content (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    source text NOT NULL,
    source_id text NOT NULL,
    content_type text NOT NULL,
    title text NOT NULL,
    release_year integer,
    poster_url text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT content_source_source_id_content_type_unique UNIQUE (source, source_id, content_type)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS content;
-- +goose StatementEnd
