package models

import (
	"time"
)

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type User struct {
	UserID   string `json:"user_id" db:"user_id"`
	Username string `json:"username" db:"username"`
	TeamName string `json:"team_name" db:"team_name"`
	IsActive bool   `json:"is_active" db:"is_active"`
}

type TeamMember struct {
	UserID   string `json:"user_id" db:"user_id"`
	Username string `json:"username" db:"username"`
	IsActive bool   `json:"is_active" db:"is_active"`
}

type Team struct {
	TeamName string       `json:"team_name"`
	Members  []TeamMember `json:"members"`
}

type PullRequestStatus string

const (
	StatusOpen   PullRequestStatus = "OPEN"
	StatusMerged PullRequestStatus = "MERGED"
)

type PullRequest struct {
	PullRequestID     string            `json:"pull_request_id" db:"pull_request_id"`
	PullRequestName   string            `json:"pull_request_name" db:"pull_request_name"`
	AuthorID          string            `json:"author_id" db:"author_id"`
	Status            PullRequestStatus `json:"status" db:"status"`
	AssignedReviewers []string          `json:"assigned_reviewers"`
	CreatedAt         *time.Time        `json:"createdAt,omitempty" db:"created_at"`
	MergedAt          *time.Time        `json:"mergedAt,omitempty" db:"merged_at"`
}

type PullRequestShort struct {
	PullRequestID   string            `json:"pull_request_id" db:"pull_request_id"`
	PullRequestName string            `json:"pull_request_name" db:"pull_request_name"`
	AuthorID        string            `json:"author_id" db:"author_id"`
	Status          PullRequestStatus `json:"status" db:"status"`
}

type ReviewStat struct {
	UserID      string `json:"user_id" db:"user_id"`
	Username    string `json:"username" db:"username"`
	ReviewCount int    `json:"review_count" db:"review_count"`
}

type BulkDeactivateRequest struct {
	UserIDs []string `json:"user_ids"`
}

type BulkDeactivateResponse struct {
	Message     string   `json:"message"`
	Deactivated []string `json:"deactivated_users"`
}
