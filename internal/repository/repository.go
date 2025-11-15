package repository

import (
	"context"
	"time"

	"github.com/denvyworking/pr-reviewer-service/internal/models"
)

type TeamRepository interface {
	CreateTeam(ctx context.Context, team *models.Team) error
	GetTeam(ctx context.Context, teamName string) (*models.Team, error)
	TeamExists(ctx context.Context, teamName string) (bool, error)
}

type UserRepository interface {
	UpdateUserActivity(ctx context.Context, userID string, isActive bool) (*models.User, error)
	GetUser(ctx context.Context, userID string) (*models.User, error)
	GetUsersByTeam(ctx context.Context, teamName string) ([]*models.User, error)
}

type PRRepository interface {
	CreatePR(ctx context.Context, pr *models.PullRequest) error
	GetPR(ctx context.Context, prID string) (*models.PullRequest, error)
	UpdatePRStatus(ctx context.Context, prID string, status models.PullRequestStatus, mergedAt *time.Time) error
	UpdatePRReviewers(ctx context.Context, prID string, reviewers []string) error
	GetPRsByReviewer(ctx context.Context, userID string) ([]models.PullRequestShort, error)
	PRExists(ctx context.Context, prID string) (bool, error)
}

type ReviewStat interface {
	GetReviewStats(ctx context.Context) ([]models.ReviewStat, error)
}

type Repository interface {
	TeamRepository
	UserRepository
	PRRepository
	ReviewStat
}
