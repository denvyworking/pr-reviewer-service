package service

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/denvyworking/pr-reviewer-service/internal/models"
	"github.com/denvyworking/pr-reviewer-service/internal/repository"
)

var (
	ErrTeamExists           = errors.New("team already exists")
	ErrPRExists             = errors.New("PR already exists")
	ErrPRMerged             = errors.New("PR is merged")
	ErrNotAssigned          = errors.New("reviewer is not assigned")
	ErrNoCandidate          = errors.New("no active replacement candidate")
	ErrNotFound             = errors.New("resource not found")
	ErrBulkDeactivateFailed = errors.New("bulk deactivate failed - some PRs cannot be reassigned")
)

type Service struct {
	repo repository.Repository
}

func NewService(repo repository.Repository) *Service {
	rand.Seed(time.Now().UnixNano())
	return &Service{repo: repo}
}

func (s *Service) CreateTeam(ctx context.Context, team *models.Team) error {
	exists, err := s.repo.TeamExists(ctx, team.TeamName)
	if err != nil {
		return err
	}
	if exists {
		return ErrTeamExists
	}

	return s.repo.CreateTeam(ctx, team)
}

func (s *Service) GetTeam(ctx context.Context, teamName string) (*models.Team, error) {
	return s.repo.GetTeam(ctx, teamName)
}

func (s *Service) SetUserActivity(ctx context.Context, userID string, isActive bool) (*models.User, error) {
	user, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}

	return s.repo.UpdateUserActivity(ctx, userID, isActive)
}

func (s *Service) selectReviewers(team *models.Team, authorID string) []string {
	var candidates []string

	for _, member := range team.Members {
		if member.IsActive && member.UserID != authorID {
			candidates = append(candidates, member.UserID)
		}
	}

	if len(candidates) <= 1 {
		return candidates
	}

	//Первый случайный индекс
	idx1 := rand.Intn(len(candidates))

	idx2 := rand.Intn(len(candidates) - 1)
	if idx2 == idx1 {
		idx2++
	}

	return []string{candidates[idx1], candidates[idx2]}
}

func (s *Service) CreatePR(ctx context.Context, prID, prName, authorID string) (*models.PullRequest, error) {
	exists, err := s.repo.PRExists(ctx, prID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrPRExists
	}

	author, err := s.repo.GetUser(ctx, authorID)
	if err != nil {
		return nil, err
	}
	if author == nil {
		return nil, ErrNotFound
	}

	team, err := s.repo.GetTeam(ctx, author.TeamName)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, ErrNotFound
	}

	reviewers := s.selectReviewers(team, authorID)

	now := time.Now()
	pr := &models.PullRequest{
		PullRequestID:     prID,
		PullRequestName:   prName,
		AuthorID:          authorID,
		Status:            models.StatusOpen,
		AssignedReviewers: reviewers,
		CreatedAt:         &now,
	}

	err = s.repo.CreatePR(ctx, pr)
	if err != nil {
		return nil, err
	}

	return pr, nil
}

func (s *Service) MergePR(ctx context.Context, prID string) (*models.PullRequest, error) {
	PullRequest, err := s.repo.GetPR(ctx, prID)
	if err != nil {
		return nil, err
	}
	if PullRequest == nil {
		return nil, ErrNotFound
	}
	// индемпотентность
	if PullRequest.Status == models.StatusMerged {
		return PullRequest, nil
	}
	time := time.Now()
	if err := s.repo.UpdatePRStatus(ctx, prID, models.StatusMerged, &time); err != nil {
		return nil, err
	}
	PullRequest.Status = models.StatusMerged
	PullRequest.MergedAt = &time
	return PullRequest, nil
}

func (s *Service) ReassignReviewer(ctx context.Context, prID, oldUserID string) (*models.PullRequest, string, error) {
	pr, err := s.repo.GetPR(ctx, prID)
	if err != nil {
		return nil, "", err
	}
	if pr == nil {
		return nil, "", ErrNotFound
	}

	// Проверка что PR не мержен!!!
	if pr.Status == models.StatusMerged {
		return nil, "", ErrPRMerged
	}

	if !s.contains(pr.AssignedReviewers, oldUserID) {
		return nil, "", ErrNotAssigned
	}

	oldReviewer, err := s.repo.GetUser(ctx, oldUserID)
	if err != nil {
		return nil, "", err
	}
	if oldReviewer == nil {
		return nil, "", ErrNotFound
	}

	team, err := s.repo.GetTeam(ctx, oldReviewer.TeamName)
	if err != nil {
		return nil, "", err
	}
	if team == nil {
		return nil, "", ErrNotFound
	}

	newReviewerID := s.selectReplacementReviewer(team, oldUserID, pr.AuthorID, pr.AssignedReviewers)
	if newReviewerID == "" {
		return nil, "", ErrNoCandidate
	}

	newReviewers := s.replaceReviewer(pr.AssignedReviewers, oldUserID, newReviewerID) // ← используем метод структуры

	err = s.repo.UpdatePRReviewers(ctx, prID, newReviewers)
	if err != nil {
		return nil, "", err
	}

	pr.AssignedReviewers = newReviewers
	return pr, newReviewerID, nil
}

func (s *Service) GetReviewStats(ctx context.Context) ([]models.ReviewStat, error) {
	return s.repo.GetReviewStats(ctx)
}

func (s *Service) BulkDeactivateUsers(ctx context.Context, userIDs []string) ([]string, error) {
	deactivated := make([]string, 0, len(userIDs))

	// Фаза 1: Проверка возможности переназначения всех PR
	for _, userID := range userIDs {
		user, err := s.repo.GetUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, ErrNotFound
		}

		prs, err := s.repo.GetPRsByReviewer(ctx, userID)
		if err != nil {
			return nil, err
		}

		for _, pr := range prs {
			if pr.Status == models.StatusOpen {
				newReviewer := s.selectReplacementReviewerForBulk(pr.PullRequestID, userID, user.TeamName)
				if newReviewer == "" {
					return nil, ErrBulkDeactivateFailed
				}
			}
		}
	}

	// Фаза 2: Выполнение переназначения и деактивации
	for _, userID := range userIDs {
		user, err := s.repo.GetUser(ctx, userID)
		if err != nil {
			return deactivated, err
		}

		prs, err := s.repo.GetPRsByReviewer(ctx, userID)
		if err != nil {
			return deactivated, err
		}

		for _, pr := range prs {
			if pr.Status == models.StatusOpen {
				fullPR, err := s.repo.GetPR(ctx, pr.PullRequestID)
				if err != nil {
					return deactivated, err
				}
				if fullPR == nil {
					continue
				}

				newReviewer := s.selectReplacementReviewerForBulk(pr.PullRequestID, userID, user.TeamName)
				if newReviewer != "" {
					newReviewers := s.replaceReviewer(fullPR.AssignedReviewers, userID, newReviewer)
					err = s.repo.UpdatePRReviewers(ctx, pr.PullRequestID, newReviewers)
					if err != nil {
						return deactivated, err
					}
				}
			}
		}
		_, err = s.repo.UpdateUserActivity(ctx, userID, false)
		if err != nil {
			return deactivated, err
		}

		deactivated = append(deactivated, userID)
	}

	return deactivated, nil
}

func (s *Service) selectReplacementReviewerForBulk(prID, oldUserID, teamName string) string {
	pr, err := s.repo.GetPR(context.Background(), prID)
	if err != nil || pr == nil {
		return ""
	}

	team, err := s.repo.GetTeam(context.Background(), teamName)
	if err != nil || team == nil {
		return ""
	}

	return s.selectReplacementReviewer(team, oldUserID, pr.AuthorID, pr.AssignedReviewers)
}

func (s *Service) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (s *Service) replaceReviewer(reviewers []string, oldID, newID string) []string {
	var result []string
	for _, reviewer := range reviewers {
		if reviewer == oldID {
			result = append(result, newID)
		} else {
			result = append(result, reviewer)
		}
	}
	return result
}

// выбрать замену для ревьювера
func (s *Service) selectReplacementReviewer(team *models.Team, oldUserID, authorID string, currentReviewers []string) string {
	var candidates []string

	for _, member := range team.Members {
		// Активный, не старый ревьювер, не автор PR, и не isActive=false
		// только такой нам подходит
		if member.IsActive &&
			member.UserID != oldUserID &&
			member.UserID != authorID &&
			!s.contains(currentReviewers, member.UserID) {
			candidates = append(candidates, member.UserID)
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	return candidates[rand.Intn(len(candidates))]
}

func (s *Service) GetReview(ctx context.Context, UserId string) []models.PullRequestShort {
	PRs, _ := s.repo.GetPRsByReviewer(ctx, UserId)
	return PRs
}
