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

-- Materialized view for current post state (aggregated)
CREATE MATERIALIZED VIEW IF NOT EXISTS current_post_state
ENGINE = AggregatingMergeTree()
ORDER BY post_id
AS
SELECT
    post_id,
    countIf(event_type='like.created') as likes,
    countIf(event_type='comment.created') as comments,
    max(created_at) as last_update
FROM feed_events
GROUP BY post_id;
