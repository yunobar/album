-- +goose Up
-- +goose StatementBegin
ALTER TABLE content RENAME TO contents;

ALTER TABLE groups ADD COLUMN round_robin_pointer integer NOT NULL DEFAULT 0;

CREATE TABLE decision_sessions (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    group_id uuid NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    method text NOT NULL CHECK (method IN ('majority', 'ranked', 'priority', 'round_robin', 'random')),
    status text NOT NULL DEFAULT 'voting' CHECK (status IN ('open', 'voting', 'completed', 'cancelled')),
    winner_content_id uuid REFERENCES contents(id),
    random_seed bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    finalized_at timestamp with time zone
);
CREATE INDEX idx_decision_sessions_group_id ON decision_sessions(group_id);

CREATE TABLE session_participants (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    session_id uuid NOT NULL REFERENCES decision_sessions(id) ON DELETE CASCADE,
    profile_id uuid NOT NULL REFERENCES user_profiles(id) ON DELETE CASCADE,
    CONSTRAINT session_participants_session_id_profile_id_unique UNIQUE (session_id, profile_id)
);

CREATE TABLE session_candidates (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    session_id uuid NOT NULL REFERENCES decision_sessions(id) ON DELETE CASCADE,
    content_id uuid NOT NULL REFERENCES contents(id),
    CONSTRAINT session_candidates_session_id_content_id_unique UNIQUE (session_id, content_id)
);

CREATE TABLE session_votes (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    session_id uuid NOT NULL REFERENCES decision_sessions(id) ON DELETE CASCADE,
    profile_id uuid NOT NULL REFERENCES user_profiles(id) ON DELETE CASCADE,
    content_id uuid NOT NULL REFERENCES contents(id),
    CONSTRAINT session_votes_session_id_profile_id_unique UNIQUE (session_id, profile_id)
);

CREATE TABLE session_rankings (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    session_id uuid NOT NULL REFERENCES decision_sessions(id) ON DELETE CASCADE,
    profile_id uuid NOT NULL REFERENCES user_profiles(id) ON DELETE CASCADE,
    content_id uuid NOT NULL REFERENCES contents(id),
    rank integer NOT NULL CHECK (rank > 0),
    CONSTRAINT session_rankings_session_id_profile_id_content_id_unique UNIQUE (session_id, profile_id, content_id)
);

CREATE TABLE session_priority_snapshots (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    session_id uuid NOT NULL REFERENCES decision_sessions(id) ON DELETE CASCADE,
    profile_id uuid NOT NULL REFERENCES user_profiles(id) ON DELETE CASCADE,
    content_id uuid NOT NULL REFERENCES contents(id),
    priority text NOT NULL CHECK (priority IN ('must', 'high', 'medium', 'low')),
    CONSTRAINT session_priority_snapshots_session_id_profile_id_content_id_unique UNIQUE (session_id, profile_id, content_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS session_priority_snapshots;
DROP TABLE IF EXISTS session_rankings;
DROP TABLE IF EXISTS session_votes;
DROP TABLE IF EXISTS session_candidates;
DROP TABLE IF EXISTS session_participants;
DROP TABLE IF EXISTS decision_sessions;
ALTER TABLE groups DROP COLUMN round_robin_pointer;
ALTER TABLE contents RENAME TO content;
-- +goose StatementEnd
