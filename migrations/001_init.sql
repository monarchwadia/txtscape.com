CREATE TABLE users (
    username      TEXT PRIMARY KEY,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tokens (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username      TEXT NOT NULL REFERENCES users(username),
    label         TEXT NOT NULL,
    token_hash    TEXT NOT NULL UNIQUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pages (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username    TEXT NOT NULL REFERENCES users(username),
    folder_path TEXT NOT NULL,
    file_name   TEXT NOT NULL,
    contents    TEXT NOT NULL,
    size_bytes  INT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (username, folder_path, file_name),

    CHECK (file_name ~ '^[a-z0-9_-]{1,245}\.txt$'),
    CHECK (folder_path ~ '^(/[a-z0-9_-]{1,10}){0,9}/$' OR folder_path = '/'),
    CHECK (size_bytes > 0 AND size_bytes <= 102400)
);

CREATE INDEX idx_pages_folder ON pages (username, folder_path);
