package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const baseURL = "http://localhost:8080"

func TestFullFlow(t *testing.T) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	timestamp := time.Now().Unix()
	teamName := fmt.Sprintf("test-team-%d", timestamp)
	userID1 := fmt.Sprintf("e2e-user-1-%d", timestamp)
	userID2 := fmt.Sprintf("e2e-user-2-%d", timestamp)
	userID3 := fmt.Sprintf("e2e-user-3-%d", timestamp)
	prID := fmt.Sprintf("e2e-pr-%d", timestamp)

	// создание команды
	team := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": userID1, "username": "E2E User 1", "is_active": true},
			{"user_id": userID2, "username": "E2E User 2", "is_active": true},
			{"user_id": userID3, "username": "E2E User 3", "is_active": true},
		},
	}

	teamJSON, _ := json.Marshal(team)
	resp, err := http.Post(baseURL+"/team/add", "application/json", bytes.NewBuffer(teamJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Создание команды должно быть успешным")

	// создание pr
	prCreate := map[string]interface{}{
		"pull_request_id":   prID,
		"pull_request_name": "E2E Test Feature",
		"author_id":         userID1,
	}

	prJSON, _ := json.Marshal(prCreate)
	resp, err = http.Post(baseURL+"/pullRequest/create", "application/json", bytes.NewBuffer(prJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Создание PR должно быть успешным")

	// Парсим ответ и проверяем что ревьюверы назначились
	var prResponse map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&prResponse)
	pr := prResponse["pr"].(map[string]interface{})

	reviewers := pr["assigned_reviewers"].([]interface{})
	assert.Len(t, reviewers, 2, "Должны быть назначены 2 ревьювера")
	assert.NotContains(t, reviewers, userID1, "Автор не должен быть в списке ревьюверов")

	// users PRs
	resp, err = http.Get(baseURL + "/users/getReview?user_id=" + userID2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Получение PR пользователя должно быть успешным")

	var reviewResponse map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&reviewResponse)
	pullRequests := reviewResponse["pull_requests"].([]interface{})
	assert.Len(t, pullRequests, 1, "Пользователь должен видеть 1 PR")

	// merge
	mergeRequest := map[string]interface{}{
		"pull_request_id": prID,
	}

	mergeJSON, _ := json.Marshal(mergeRequest)
	resp, err = http.Post(baseURL+"/pullRequest/merge", "application/json", bytes.NewBuffer(mergeJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Мерж PR должен быть успешным")

	// Пробуем мержить еще раз (идемпотентность)
	resp, err = http.Post(baseURL+"/pullRequest/merge", "application/json", bytes.NewBuffer(mergeJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Повторный мерж должен быть успешным (идемпотентность)")

	// Нельзя менять ревьюера после merge
	reassignRequest := map[string]interface{}{
		"pull_request_id": prID,
		"old_user_id":     userID2,
	}

	reassignJSON, _ := json.Marshal(reassignRequest)
	resp, err = http.Post(baseURL+"/pullRequest/reassign", "application/json", bytes.NewBuffer(reassignJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode, "После мержа нельзя менять ревьюверов")

	// Проверяем что ошибка правильная
	var errorResponse map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&errorResponse)
	errorObj := errorResponse["error"].(map[string]interface{})
	assert.Equal(t, "PR_MERGED", errorObj["code"], "Должна быть ошибка PR_MERGED")

	t.Run("BulkDeactivateUsers", func(t *testing.T) {
		bulkDeactivateReq := map[string]interface{}{
			"user_ids": []string{userID2, userID3},
		}

		bulkJSON, _ := json.Marshal(bulkDeactivateReq)
		req, _ := http.NewRequest("POST", baseURL+"/users/bulkDeactivate", bytes.NewBuffer(bulkJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "admin-token")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Массовая деактивация должна быть успешной")

		var bulkResponse map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&bulkResponse)

		deactivatedUsers := bulkResponse["deactivated_users"].([]interface{})
		assert.Len(t, deactivatedUsers, 2, "Должны быть деактивированы 2 пользователя")
		assert.Contains(t, deactivatedUsers, userID2)
		assert.Contains(t, deactivatedUsers, userID3)

		// Проверяем что пользователи действительно деактивированы
		resp, err = http.Get(baseURL + "/team/get?team_name=" + teamName)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var teamResponse map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&teamResponse)

		members := teamResponse["members"].([]interface{})
		for _, member := range members {
			m := member.(map[string]interface{})
			if m["user_id"] == userID2 || m["user_id"] == userID3 {
				assert.False(t, m["is_active"].(bool), "Пользователь должен быть неактивен")
			}
		}
	})
	t.Run("ReviewStats", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/stats/review-counts")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var statsResponse map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&statsResponse)

		stats := statsResponse["stats"].([]interface{})
		assert.Greater(t, len(stats), 0, "Должна возвращаться статистика")
	})
	t.Run("SetUserActivity", func(t *testing.T) {
		setActiveReq := map[string]interface{}{
			"user_id":   userID2,
			"is_active": true,
		}

		activeJSON, _ := json.Marshal(setActiveReq)
		req, _ := http.NewRequest("POST", baseURL+"/users/setIsActive", bytes.NewBuffer(activeJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "admin-token")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	slog.Info("ВСЕ E2E ТЕСТЫ ПРОЙДЕНЫ УСПЕШНО!")
}
