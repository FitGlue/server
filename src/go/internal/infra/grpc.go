package infra

import (
	"crypto/tls"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCDial creates a gRPC client connection to the given target.
// If the target is an HTTPS Cloud Run URL, it automatically uses TLS credentials
// and connects on port 443. For local development (localhost), it uses insecure credentials.
func GRPCDial(target string) (*grpc.ClientConn, error) {
	if strings.HasPrefix(target, "https://") {
		// Cloud Run URL: strip scheme, use TLS on port 443
		host := strings.TrimPrefix(target, "https://")
		return grpc.NewClient(host+":443", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	}
	// Local development: use insecure credentials
	return grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
}
