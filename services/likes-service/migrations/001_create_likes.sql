CREATE TABLE IF NOT EXISTS likes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    UNIQUE (user_id, entity_id)
);

CREATE INDEX idx_likes_entity_id ON likes (entity_id);
CREATE INDEX idx_likes_user_id ON likes (user_id);
