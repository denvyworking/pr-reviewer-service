package main

import (
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"

	"github.com/denvyworking/pr-reviewer-service/internal/repository/postgres"
	"github.com/denvyworking/pr-reviewer-service/internal/service"
	"github.com/denvyworking/pr-reviewer-service/internal/transport/httpt"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://user:password@localhost:5432/pr_reviewer?sslmode=disable"
	}

	db, err := postgres.ConnectToDatabase(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Connected to PostgreSQL database")
	repo := postgres.NewPostgresRepository(db)

	service := service.NewService(repo)

	handlers := httpt.NewHandlers(service)

	http.HandleFunc("/team/add", handlers.CreateTeamHandler)
	http.HandleFunc("/team/get", handlers.GetTeamHandler)
	http.HandleFunc("/pullRequest/create", handlers.CreatePRHandler)
	http.HandleFunc("/pullRequest/merge", handlers.MergePRHandler)
	http.HandleFunc("/pullRequest/reassign", handlers.ReassignPRHandler)
	http.HandleFunc("/users/setIsActive", handlers.SetIsActiveHandler)
	http.HandleFunc("/users/getReview", handlers.GetReviewHandler)
	http.HandleFunc("/stats/review-counts", handlers.GetReviewStatsHandler)
	http.HandleFunc("/users/bulkDeactivate", handlers.BulkDeactivateHandler)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Println("PR Reviewer Service starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
