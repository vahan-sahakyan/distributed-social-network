CREATE TABLE IF NOT EXISTS comments (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    text TEXT NOT NULL,
    likes INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_comments_entity_id ON comments (entity_id);
CREATE INDEX idx_comments_created_at ON comments (created_at DESC);
