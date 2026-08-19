package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ldndrc/control-panel/internal/auth"
	"github.com/ldndrc/control-panel/internal/httpapi"
)

func main() {
	port := os.Getenv("CONTROL_PANEL_PORT")
	if port == "" {
		port = "8082"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET must be set")
	}

	authSvc := auth.NewService([]byte(jwtSecret), 24*time.Hour)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewRouter(authSvc),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("control-panel listening on :%s", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
