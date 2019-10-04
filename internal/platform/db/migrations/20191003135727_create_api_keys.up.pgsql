CREATE TABLE api_keys
(
    hashed_key BYTEA PRIMARY KEY,
    user_id INT REFERENCES users (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
)