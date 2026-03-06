package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	pb "github.com/RedHatInsights/http-test-services/api/gen"
	"github.com/RedHatInsights/http-test-services/internal"
	"github.com/RedHatInsights/http-test-services/internal/handlers"
	"google.golang.org/grpc"
)

type clowderConfig struct {
	WebPort int `json:"webPort"`
}

type pingServer struct {
	pb.UnimplementedPingServiceServer
}

func (s *pingServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingReply, error) {
	log.Printf("gRPC Ping: %s", req.GetMessage())
	return &pb.PingReply{Message: req.GetMessage()}, nil
}

func main() {
	httpPort := resolveHTTPPort()
	docsDir := resolveDocsDir()

	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux, docsDir)

	timeout := resolveHTTPTimeout()
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", httpPort),
		Handler:      mux,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
		IdleTimeout:  timeout,
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPingServiceServer(grpcServer, &pingServer{})

	// Start HTTP server.
	go func() {
		log.Printf("HTTP server listening on :%d", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Start gRPC server.
	go func() {
		lis, err := net.Listen("tcp", ":50051")
		if err != nil {
			log.Fatalf("gRPC listen error: %v", err)
		}
		log.Println("gRPC server listening on :50051")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	// Wait for interrupt signal.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("Shutting down...")

	grpcServer.GracefulStop()
	httpServer.Shutdown(context.Background())
}

func resolveHTTPPort() int {
	if acgPath := os.Getenv(internal.EnvACGConfig); acgPath != "" {
		data, err := os.ReadFile(acgPath)
		if err == nil {
			var cfg clowderConfig
			if err := json.Unmarshal(data, &cfg); err == nil && cfg.WebPort > 0 {
				return cfg.WebPort
			}
		}
	}
	if portEnv := os.Getenv(internal.EnvHTTPPort); portEnv != "" {
		if port, err := strconv.Atoi(portEnv); err == nil && port > 0 {
			return port
		}
	}
	return 9092
}

func resolveHTTPTimeout() time.Duration {
	if s := os.Getenv(internal.EnvHTTPTimeout); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 30 * time.Second
}

func resolveDocsDir() string {
	// In container, docs are at /docs; locally, use ./docs.
	if _, err := os.Stat("/docs"); err == nil {
		return "/docs"
	}
	return "docs"
}
