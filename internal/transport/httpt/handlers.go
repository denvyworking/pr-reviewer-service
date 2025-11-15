package httpt

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/denvyworking/pr-reviewer-service/internal/models"
	"github.com/denvyworking/pr-reviewer-service/internal/service"
)

type Handlers struct {
	service *service.Service
}

func NewHandlers(service *service.Service) *Handlers {
	return &Handlers{service: service}
}

func (h *Handlers) CreateTeamHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var team models.Team
	if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
		writeError(w, "BAD_REQUEST", "invalid JSON", http.StatusBadRequest)
		return
	}

	if err := h.service.CreateTeam(r.Context(), &team); err != nil {
		switch err {
		case service.ErrTeamExists:
			writeError(w, "TEAM_EXISTS", err.Error(), http.StatusBadRequest)
		default:
			writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"team": team,
	})
}

func (h *Handlers) GetTeamHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		writeError(w, "BAD_REQUEST", "team_name is required", http.StatusBadRequest)
		return
	}

	team, err := h.service.GetTeam(r.Context(), teamName)
	if err != nil {
		switch err {
		case service.ErrNotFound:
			writeError(w, "NOT_FOUND", "team not found", http.StatusNotFound)
		default:
			writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(team)
}

func (h *Handlers) CreatePRHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		PullRequestID   string `json:"pull_request_id"`
		PullRequestName string `json:"pull_request_name"`
		AuthorID        string `json:"author_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, "BAD_REQUEST", "invalid JSON", http.StatusBadRequest)
		return
	}

	pr, err := h.service.CreatePR(r.Context(), request.PullRequestID, request.PullRequestName, request.AuthorID)
	if err != nil {
		switch err {
		case service.ErrPRExists:
			writeError(w, "PR_EXISTS", err.Error(), http.StatusConflict)
		case service.ErrNotFound:
			writeError(w, "NOT_FOUND", err.Error(), http.StatusNotFound)
		default:
			writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pr": pr,
	})
}

func (h *Handlers) SetIsActiveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.isAdminAuthorized(r) {
		writeError(w, "UNAUTHORIZED", "invalid admin token", http.StatusUnauthorized)
		return
	}
	var user struct {
		UserID   string `json:"user_id"`
		IsActive bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		writeError(w, "BAD_REQUEST", "invalid JSON", http.StatusBadRequest)
		return
	}

	updateUser, err := h.service.SetUserActivity(r.Context(), user.UserID, user.IsActive)
	if err != nil {
		switch err {
		case service.ErrNotFound:
			writeError(w, "NOT_FOUND", err.Error(), http.StatusNotFound)
		default:
			writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": updateUser,
	})

}

func (h *Handlers) MergePRHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var pull_request_id struct {
		PullRequestID string `json:"pull_request_id"`
	}
	// второй способ дешифровать json request
	// он хуже, поэтому далее так не будем)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.Unmarshal(body, &pull_request_id); err != nil {
		writeError(w, "BAD_REQUEST", "invalid JSON", http.StatusBadRequest)
		return
	}
	if pull_request_id.PullRequestID == "" {
		writeError(w, "BAD_REQUEST", "pull_request_id is required", http.StatusBadRequest)
		return
	}
	pr, err := h.service.MergePR(r.Context(), pull_request_id.PullRequestID)
	if err != nil {
		switch err {
		case service.ErrNotFound:
			writeError(w, "NOT_FOUND", err.Error(), http.StatusNotFound)
		default:
			writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"pr": pr,
	}

	resp, err := json.Marshal(response)
	if err != nil {
		writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(resp); err != nil {
		writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		return
	}

}

func (h *Handlers) ReassignPRHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	//pull_request_id, old_user_id
	var reassign struct {
		Pr_id      string `json:"pull_request_id"`
		OldUser_id string `json:"old_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reassign); err != nil {
		writeError(w, "BAD_REQUEST", "invalid JSON", http.StatusBadRequest)
		return
	}
	pr, newReviewer, err := h.service.ReassignReviewer(r.Context(), reassign.Pr_id, reassign.OldUser_id)
	if err != nil {
		switch err {
		case service.ErrNotFound:
			writeError(w, "NOT_FOUND", err.Error(), http.StatusNotFound)
		case service.ErrPRMerged:
			writeError(w, "PR_MERGED", err.Error(), http.StatusConflict)
		case service.ErrNotAssigned:
			writeError(w, "NOT_ASSIGNED", err.Error(), http.StatusConflict)
		case service.ErrNoCandidate:
			writeError(w, "NO_CANDIDATE", err.Error(), http.StatusConflict)
		default:
			writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pr":          pr,
		"replaced_by": newReviewer,
	})
}

func (h *Handlers) GetReviewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userId := r.URL.Query().Get("user_id")
	pr := h.service.GetReview(r.Context(), userId)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":       userId,
		"pull_requests": pr,
	})

}

// GetReviewStatsHandler возвращает статистику по ревьюверам
func (h *Handlers) GetReviewStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := h.service.GetReviewStats(r.Context())
	if err != nil {
		writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats": stats,
	})
}

func (h *Handlers) BulkDeactivateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.isAdminAuthorized(r) {
		writeError(w, "UNAUTHORIZED", "invalid admin token", http.StatusUnauthorized)
		return
	}

	var request models.BulkDeactivateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, "BAD_REQUEST", "invalid JSON", http.StatusBadRequest)
		return
	}

	deactivatedUsers, err := h.service.BulkDeactivateUsers(r.Context(), request.UserIDs)
	if err != nil {
		switch err {
		case service.ErrNotFound:
			writeError(w, "NOT_FOUND", err.Error(), http.StatusNotFound)
		case service.ErrBulkDeactivateFailed:
			writeError(w, "BULK_DEACTIVATE_FAILED", "cannot deactivate users - some PRs cannot be reassigned", http.StatusConflict)
		default:
			writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.BulkDeactivateResponse{
		Message:     "Users deactivated successfully",
		Deactivated: deactivatedUsers,
	})
}

func (h *Handlers) isAdminAuthorized(r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	return authHeader == "admin-token"
}

func writeError(w http.ResponseWriter, code, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResp := models.ErrorResponse{}
	errorResp.Error.Code = code
	errorResp.Error.Message = message

	json.NewEncoder(w).Encode(errorResp)
}
