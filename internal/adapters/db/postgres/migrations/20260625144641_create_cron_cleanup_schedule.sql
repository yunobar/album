-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    PERFORM cron.unschedule('authkit-token-cleanup');
    PERFORM cron.schedule(
        'authkit-token-cleanup',
        '0 3 * * 0',
        'DELETE FROM refresh_tokens WHERE expires_at < now(); DELETE FROM sessions s WHERE NOT EXISTS (SELECT 1 FROM refresh_tokens rt WHERE rt.session_id = s.id) AND s.last_used_at < now() - interval ''7 days'';'
    );
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'pg_cron not available, skipping cron schedule: %', SQLERRM;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    PERFORM cron.unschedule('authkit-token-cleanup');
    PERFORM cron.schedule(
        'authkit-token-cleanup',
        '0 3 * * 0',
        'DELETE FROM refresh_tokens WHERE expires_at < now(); DELETE FROM sessions s WHERE NOT EXISTS (SELECT 1 FROM refresh_tokens rt WHERE rt.session_id = s.id) AND s.last_active_at < now() - interval ''7 days'';'
    );
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'pg_cron not available, skipping cron schedule: %', SQLERRM;
END $$;
-- +goose StatementEnd
