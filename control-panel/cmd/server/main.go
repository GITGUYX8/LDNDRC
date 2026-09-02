package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ldndrc/control-panel/internal/auth"
	"github.com/ldndrc/control-panel/internal/httpapi"
	"github.com/ldndrc/control-panel/internal/sessions"
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
	sessionStore := sessions.NewStore()
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" || os.Getenv("KUBERNETES_IN_CLUSTER") == "true" {
		namespace := os.Getenv("SESSION_NAMESPACE")
		if namespace == "" {
			namespace = "ldndrc"
		}
		image := os.Getenv("SESSION_IMAGE")
		if image == "" {
			image = "ldndrc/ros2-gz:jazzy-harmonic-workspace"
		}
		provisioner, err := sessions.NewInClusterProvisioner(namespace, image)
		if err != nil {
			log.Fatalf("configure Kubernetes session provisioner: %v", err)
		}
		sessionStore = sessions.NewStoreWithProvisioner(provisioner)
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewRouter(authSvc, sessionStore),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("control-panel listening on :%s", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
