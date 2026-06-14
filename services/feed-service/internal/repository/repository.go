package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vahan/distributed-social-network/feed-service/internal/model"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) InsertFeedItem(ctx context.Context, item *model.FeedItem) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO feed_items (user_id, post_id, author_id, text, likes_count, comments_count, image_url, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		item.UserID, item.PostID, item.AuthorID, item.Text, item.LikesCount, item.CommentsCount, item.ImageURL, item.CreatedAt,
	)
	return err
}

func (r *Repository) GetHomeFeed(ctx context.Context, userID string, limit int) ([]model.FeedItem, error) {
	rows, err := r.db.Query(ctx,
		`SELECT user_id, post_id, author_id, text, likes_count, comments_count, image_url, created_at
		 FROM feed_items WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.FeedItem
	for rows.Next() {
		var item model.FeedItem
		if err := rows.Scan(&item.UserID, &item.PostID, &item.AuthorID, &item.Text,
			&item.LikesCount, &item.CommentsCount, &item.ImageURL, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *Repository) GetUserFeed(ctx context.Context, authorID string, limit int) ([]model.FeedItem, error) {
	rows, err := r.db.Query(ctx,
		`SELECT user_id, post_id, author_id, text, likes_count, comments_count, image_url, created_at
		 FROM feed_items WHERE author_id = $1 ORDER BY created_at DESC LIMIT $2`,
		authorID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.FeedItem
	for rows.Next() {
		var item model.FeedItem
		if err := rows.Scan(&item.UserID, &item.PostID, &item.AuthorID, &item.Text,
			&item.LikesCount, &item.CommentsCount, &item.ImageURL, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
