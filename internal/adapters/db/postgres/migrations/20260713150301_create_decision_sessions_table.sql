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

-- session_votes/session_rankings/session_priority_snapshots reference the
-- *frozen* participant/candidate set via composite FKs to
-- session_participants/session_candidates, not directly to
-- user_profiles/contents — a vote, ranking, or snapshot for a profile_id or
-- content_id that isn't actually part of THIS session is a DB-level
-- impossibility, not just an application-layer check the resolvers have to
-- trust blindly.
CREATE TABLE session_votes (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    session_id uuid NOT NULL REFERENCES decision_sessions(id) ON DELETE CASCADE,
    profile_id uuid NOT NULL,
    content_id uuid NOT NULL,
    CONSTRAINT session_votes_session_id_profile_id_unique UNIQUE (session_id, profile_id),
    CONSTRAINT session_votes_participant_fk FOREIGN KEY (session_id, profile_id)
        REFERENCES session_participants(session_id, profile_id) ON DELETE CASCADE,
    CONSTRAINT session_votes_candidate_fk FOREIGN KEY (session_id, content_id)
        REFERENCES session_candidates(session_id, content_id) ON DELETE CASCADE
);

CREATE TABLE session_rankings (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    session_id uuid NOT NULL REFERENCES decision_sessions(id) ON DELETE CASCADE,
    profile_id uuid NOT NULL,
    content_id uuid NOT NULL,
    rank integer NOT NULL CHECK (rank > 0),
    CONSTRAINT session_rankings_session_id_profile_id_content_id_unique UNIQUE (session_id, profile_id, content_id),
    CONSTRAINT session_rankings_session_id_profile_id_rank_unique UNIQUE (session_id, profile_id, rank),
    CONSTRAINT session_rankings_participant_fk FOREIGN KEY (session_id, profile_id)
        REFERENCES session_participants(session_id, profile_id) ON DELETE CASCADE,
    CONSTRAINT session_rankings_candidate_fk FOREIGN KEY (session_id, content_id)
        REFERENCES session_candidates(session_id, content_id) ON DELETE CASCADE
);

CREATE TABLE session_priority_snapshots (
    id uuid DEFAULT uuidv7() NOT NULL PRIMARY KEY,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    session_id uuid NOT NULL REFERENCES decision_sessions(id) ON DELETE CASCADE,
    profile_id uuid NOT NULL,
    content_id uuid NOT NULL,
    priority text NOT NULL CHECK (priority IN ('must', 'high', 'medium', 'low')),
    CONSTRAINT session_priority_snapshots_session_id_profile_id_content_id_unique UNIQUE (session_id, profile_id, content_id),
    CONSTRAINT session_priority_snapshots_participant_fk FOREIGN KEY (session_id, profile_id)
        REFERENCES session_participants(session_id, profile_id) ON DELETE CASCADE,
    CONSTRAINT session_priority_snapshots_candidate_fk FOREIGN KEY (session_id, content_id)
        REFERENCES session_candidates(session_id, content_id) ON DELETE CASCADE
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
