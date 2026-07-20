-- +goose Up
-- +goose StatementBegin
-- One live session per group, enforced at the index rather than an
-- app-level read-then-check, so two concurrent creates can't both win the
-- race (ADR-0006). completed/cancelled rows fall out of the partial index,
-- so history is unaffected and a group unblocks itself by finalizing.
CREATE UNIQUE INDEX one_live_session_per_group
    ON decision_sessions (group_id)
    WHERE status = 'voting';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS one_live_session_per_group;
-- +goose StatementEnd
