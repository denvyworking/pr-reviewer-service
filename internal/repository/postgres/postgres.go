package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/denvyworking/pr-reviewer-service/internal/models"
	"github.com/jmoiron/sqlx"
)

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func ConnectToDatabase(connectionString string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func (r *PostgresRepository) CreateTeam(ctx context.Context, team *models.Team) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	currentTime := time.Now()

	_, err = tx.ExecContext(ctx, `
        INSERT INTO teams (team_name, created_at) 
        VALUES ($1, $2) 
        ON CONFLICT (team_name) DO NOTHING
    `, team.TeamName, currentTime)
	if err != nil {
		return fmt.Errorf("failed to insert team: %w", err)
	}

	for _, member := range team.Members {
		_, err = tx.ExecContext(ctx, `
            INSERT INTO users (user_id, username, team_name, is_active, created_at) 
            VALUES ($1, $2, $3, $4, $5)
            ON CONFLICT (user_id) DO UPDATE SET
                username = EXCLUDED.username,
                team_name = EXCLUDED.team_name,
                is_active = EXCLUDED.is_active
        `, member.UserID, member.Username, team.TeamName, member.IsActive, currentTime)
		if err != nil {
			return fmt.Errorf("failed to insert user %s: %w", member.UserID, err)
		}
	}

	return tx.Commit()
}

func (r *PostgresRepository) GetTeam(ctx context.Context, teamName string) (*models.Team, error) {
	var team models.Team
	team.TeamName = teamName

	err := r.db.SelectContext(ctx, &team.Members, `
        SELECT user_id, username, is_active
        FROM users 
        WHERE team_name = $1
        ORDER BY user_id
    `, teamName)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}

	if len(team.Members) == 0 {
		return nil, nil
	}

	return &team, nil
}

func (r *PostgresRepository) GetUser(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	err := r.db.GetContext(ctx, &user, `
        SELECT user_id, username, team_name, is_active
        FROM users 
        WHERE user_id = $1
    `, userID)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

func (r *PostgresRepository) TeamExists(ctx context.Context, teamName string) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `
        SELECT EXISTS(SELECT 1 FROM teams WHERE team_name = $1)
    `, teamName)

	if err != nil {
		return false, fmt.Errorf("failed to check team existence: %w", err)
	}

	return exists, nil
}

func (r *PostgresRepository) UpdateUserActivity(ctx context.Context, userID string, isActive bool) (*models.User, error) {
	_, err := r.db.ExecContext(ctx, `
        UPDATE users 
        SET is_active = $1 
        WHERE user_id = $2
    `, isActive, userID)

	if err != nil {
		return nil, fmt.Errorf("failed to update user activity: %w", err)
	}

	return r.GetUser(ctx, userID)
}

func (r *PostgresRepository) CreatePR(ctx context.Context, pr *models.PullRequest) error {
	reviewersJSON, err := json.Marshal(pr.AssignedReviewers)
	if err != nil {
		return fmt.Errorf("failed to marshal reviewers: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
        INSERT INTO pull_requests 
        (pull_request_id, pull_request_name, author_id, status, assigned_reviewers, created_at) 
        VALUES ($1, $2, $3, $4, $5, $6)
    `, pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status, reviewersJSON, pr.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create PR: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetPR(ctx context.Context, prID string) (*models.PullRequest, error) {
	var pr models.PullRequest
	var reviewersJSON []byte

	err := r.db.QueryRowContext(ctx, `
        SELECT pull_request_id, pull_request_name, author_id, status, assigned_reviewers, created_at, merged_at
        FROM pull_requests 
        WHERE pull_request_id = $1
    `, prID).Scan(
		&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status,
		&reviewersJSON, &pr.CreatedAt, &pr.MergedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get PR: %w", err)
	}

	if err := json.Unmarshal(reviewersJSON, &pr.AssignedReviewers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reviewers: %w", err)
	}

	return &pr, nil
}

func (r *PostgresRepository) PRExists(ctx context.Context, prID string) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `
        SELECT EXISTS(SELECT 1 FROM pull_requests WHERE pull_request_id = $1)
    `, prID)

	if err != nil {
		return false, fmt.Errorf("failed to check PR existence: %w", err)
	}

	return exists, nil
}

func (r *PostgresRepository) UpdatePRStatus(ctx context.Context, prID string, status models.PullRequestStatus, mergedAt *time.Time) error {
	_, err := r.db.ExecContext(ctx, `
        UPDATE pull_requests 
        SET status = $1, merged_at = $2 
        WHERE pull_request_id = $3
    `, status, mergedAt, prID)

	if err != nil {
		return fmt.Errorf("failed to update PR status: %w", err)
	}

	return nil
}

func (r *PostgresRepository) UpdatePRReviewers(ctx context.Context, prID string, reviewers []string) error {
	reviewersJSON, err := json.Marshal(reviewers)
	if err != nil {
		return fmt.Errorf("failed to marshal reviewers: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
        UPDATE pull_requests 
        SET assigned_reviewers = $1 
        WHERE pull_request_id = $2
    `, reviewersJSON, prID)

	if err != nil {
		return fmt.Errorf("failed to update PR reviewers: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetPRsByReviewer(ctx context.Context, userID string) ([]models.PullRequestShort, error) {
	var prs []models.PullRequestShort

	err := r.db.SelectContext(ctx, &prs, `
        SELECT pull_request_id, pull_request_name, author_id, status
        FROM pull_requests 
        WHERE assigned_reviewers @> $1
        ORDER BY created_at DESC
    `, fmt.Sprintf(`["%s"]`, userID))

	if err != nil {
		if err == sql.ErrNoRows {
			return []models.PullRequestShort{}, nil
		}
		return nil, fmt.Errorf("failed to get PRs by reviewer: %w", err)
	}

	return prs, nil
}

// GetReviewStats получает статистику по количеству PR у каждого пользователя
func (r *PostgresRepository) GetReviewStats(ctx context.Context) ([]models.ReviewStat, error) {
	var stats []models.ReviewStat

	query := `
        SELECT 
            u.user_id, 
            u.username,
            (SELECT COUNT(*) 
             FROM pull_requests pr 
             WHERE pr.status = 'OPEN' 
             AND pr.assigned_reviewers @> jsonb_build_array(u.user_id)
            ) as review_count
        FROM users u
        ORDER BY review_count DESC
    `

	err := r.db.SelectContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get review stats: %w", err)
	}

	return stats, nil
}

func (r *PostgresRepository) GetUsersByTeam(ctx context.Context, teamName string) ([]*models.User, error) {
	var users []*models.User

	err := r.db.SelectContext(ctx, &users, `
        SELECT user_id, username, team_name, is_active
        FROM users 
        WHERE team_name = $1
        ORDER BY user_id
    `, teamName)

	if err != nil {
		return nil, fmt.Errorf("failed to get users by team: %w", err)
	}

	return users, nil
}
