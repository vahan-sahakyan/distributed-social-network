CREATE KEYSPACE IF NOT EXISTS posts
    WITH replication = {'class': 'NetworkTopologyStrategy', 'replication_factor': 1};

CREATE TABLE IF NOT EXISTS posts.posts (
    id TEXT PRIMARY KEY,
    text TEXT,
    author_id TEXT,
    image_id TEXT,
    likes INT,
    comments INT,
    created_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_posts_author_id ON posts.posts (author_id);
