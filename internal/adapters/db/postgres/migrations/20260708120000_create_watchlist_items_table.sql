-- +goose Up
-- +goose StatementBegin
CREATE TABLE watchlist_items (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    profile_id uuid NOT NULL REFERENCES user_profiles(id) ON DELETE CASCADE,
    content_id uuid NOT NULL REFERENCES content(id),
    priority text NOT NULL CHECK (priority IN ('must', 'high', 'medium', 'low')),
    notes text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'watched')),
    CONSTRAINT watchlist_items_profile_id_content_id_unique UNIQUE (profile_id, content_id)
);

CREATE INDEX idx_watchlist_items_profile_id ON watchlist_items(profile_id);
CREATE INDEX idx_watchlist_items_content_id ON watchlist_items(content_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS watchlist_items;
-- +goose StatementEnd
