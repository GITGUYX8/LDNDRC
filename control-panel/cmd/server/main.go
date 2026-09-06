package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ldndrc/control-panel/internal/auth"
	"github.com/ldndrc/control-panel/internal/gateway"
	"github.com/ldndrc/control-panel/internal/httpapi"
	"github.com/ldndrc/control-panel/internal/nodes"
	"github.com/ldndrc/control-panel/internal/sessions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	nodeDB := os.Getenv("NODES_DB")
	if nodeDB == "" {
		nodeDB = "/var/lib/ldndrc/nodes.json"
	}
	nodeStore, err := nodes.NewStore(nodeDB)
	if err != nil {
		log.Fatalf("load nodes db: %v", err)
	}
	var (
		k8sClient kubernetes.Interface
		minter    nodes.Minter
	)
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
		config, err := rest.InClusterConfig()
		if err != nil {
			log.Fatalf("load in-cluster Kubernetes config: %v", err)
		}
		k8sClient, err = kubernetes.NewForConfig(config)
		if err != nil {
			log.Fatalf("create Kubernetes client: %v", err)
		}
		minter = nodes.NewBootstrapMinter(k8sClient)
		go nodes.NewWatcher(nodeStore, k8sClient, 5*time.Second).Run(context.Background())
	}
	apiHandler := httpapi.NewRouterWithNodes(authSvc, sessionStore, nodeStore, minter)
	gatewayHandler := gateway.NewSessionHandler(authSvc, sessionStore, apiHandler)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           gatewayHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("control-panel listening on :%s", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
