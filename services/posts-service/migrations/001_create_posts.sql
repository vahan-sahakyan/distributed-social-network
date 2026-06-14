CREATE TABLE IF NOT EXISTS posts (
    id TEXT PRIMARY KEY,
    text TEXT NOT NULL,
    author_id TEXT NOT NULL,
    image_id TEXT,
    likes INTEGER DEFAULT 0,
    comments INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_posts_author_id ON posts (author_id);
CREATE INDEX idx_posts_created_at ON posts (created_at DESC);
