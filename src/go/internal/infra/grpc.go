package infra

import (
	"context"
	"crypto/tls"
	"strings"

	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
)

// GRPCDial creates a gRPC client connection to the given target.
// If the target is an HTTPS Cloud Run URL, it automatically uses TLS credentials,
// attaches an OIDC identity token for the target audience (required by Cloud Run IAM),
// and connects on port 443. For local development (localhost), it uses insecure credentials.
func GRPCDial(target string) (*grpc.ClientConn, error) {
	if strings.HasPrefix(target, "https://") {
		// Cloud Run URL: strip scheme, use TLS on port 443
		host := strings.TrimPrefix(target, "https://")

		// Obtain an OIDC identity token for the target audience.
		// On Cloud Run, this uses the service account's metadata server automatically.
		tokenSource, err := idtoken.NewTokenSource(context.Background(), target)
		if err != nil {
			return nil, err
		}

		return grpc.NewClient(host+":443",
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
			grpc.WithPerRPCCredentials(&oauth.TokenSource{TokenSource: tokenSource}),
		)
	}
	// Local development: use insecure credentials
	return grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
}
