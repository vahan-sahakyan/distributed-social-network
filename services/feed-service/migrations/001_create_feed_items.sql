CREATE TABLE IF NOT EXISTS feed_items (
    user_id TEXT NOT NULL,
    post_id TEXT NOT NULL,
    author_id TEXT NOT NULL,
    text TEXT NOT NULL,
    likes_count INTEGER DEFAULT 0,
    comments_count INTEGER DEFAULT 0,
    image_url TEXT,
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY (user_id, post_id)
);

CREATE INDEX idx_feed_items_user_created ON feed_items (user_id, created_at DESC);
CREATE INDEX idx_feed_items_author ON feed_items (author_id, created_at DESC);
