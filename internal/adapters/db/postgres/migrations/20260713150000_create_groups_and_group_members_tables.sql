-- +goose Up
-- +goose StatementBegin
CREATE TABLE groups (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    name text,
    invite_token text NOT NULL UNIQUE
);

CREATE TABLE group_members (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    group_id uuid NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    profile_id uuid NOT NULL REFERENCES user_profiles(id) ON DELETE CASCADE,
    CONSTRAINT group_members_group_id_profile_id_unique UNIQUE (group_id, profile_id)
);
CREATE INDEX idx_group_members_group_id ON group_members(group_id);
CREATE INDEX idx_group_members_profile_id ON group_members(profile_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS groups;
-- +goose StatementEnd
