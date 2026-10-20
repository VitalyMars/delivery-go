// Package transport contains shared HTTP limits and internal service authentication.
package transport

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func Env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func Brokers() []string {
	var brokers []string
	for _, broker := range strings.Split(Env("KAFKA_BROKERS", "localhost:9092"), ",") {
		if broker = strings.TrimSpace(broker); broker != "" {
			brokers = append(brokers, broker)
		}
	}
	return brokers
}

func ValidateInternalKey() error {
	if len(os.Getenv("INTERNAL_API_KEY")) < 32 {
		return errors.New("INTERNAL_API_KEY must contain at least 32 bytes")
	}
	return nil
}

func InternalKeyOK(key string) bool {
	return ValidateInternalKey() == nil && subtle.ConstantTimeCompare([]byte(key), []byte(os.Getenv("INTERNAL_API_KEY"))) == 1
}

func InternalServer(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	values := md.Get("x-internal-api-key")
	if len(values) != 1 || !InternalKeyOK(values[0]) {
		return nil, status.Error(codes.Unauthenticated, "service authentication required")
	}
	return handler(ctx, req)
}

func InternalClient(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "x-internal-api-key", os.Getenv("INTERNAL_API_KEY"))
	return invoker(ctx, method, req, reply, cc, opts...)
}

func Decode(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("body must contain exactly one JSON value")
	}
	return nil
}

func Server(address string, handler http.Handler) *http.Server {
	return &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
}

func StopGRPC(server *grpc.Server) {
	done := make(chan struct{})
	go func() { server.GracefulStop(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		server.Stop()
		<-done
	}
}
