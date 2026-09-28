CREATE TABLE IF NOT EXISTS feed_events
(
    event_id UUID,
    event_type String,
    post_id String,
    user_id String,
    likes_delta Int32,
    comments_delta Int32,
    created_at DateTime
)
ENGINE = MergeTree()
ORDER BY (post_id, created_at);

-- post state is aggregated from feed_events at query time; drop the unused view older volumes still have
DROP VIEW IF EXISTS current_post_state;
