// Package sliver provides a thin wrapper around the Sliver gRPC client that handles
// connection management using the standard Sliver operator config format.
package sliver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"

	"github.com/bishopfox/sliver/protobuf/rpcpb"
)

const (
	maxMsgSize     = (2*1024*1024*1024 - 1) // 2 GiB − 1 byte, matching upstream
	connectTimeout = 15 * time.Second
)

// Config is the Sliver operator config (JSON format written by `sliver-server operator`).
// It mirrors client/assets.ClientConfig so the same .cfg files can be used.
type Config struct {
	Operator      string `json:"operator"`
	LHost         string `json:"lhost"`
	LPort         int    `json:"lport"`
	Token         string `json:"token"`
	CACertificate string `json:"ca_certificate"`
	PrivateKey    string `json:"private_key"`
	Certificate   string `json:"certificate"`
}

// LoadConfig reads a Sliver operator .cfg file from disk.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading operator config %q: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing operator config: %w", err)
	}
	if cfg.LHost == "" || cfg.LPort == 0 {
		return nil, fmt.Errorf("operator config missing lhost/lport")
	}
	return &cfg, nil
}

// tokenAuth implements grpc.PerRPCCredentials to attach the operator bearer token.
type tokenAuth struct{ token string }

func (t tokenAuth) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{"Authorization": "Bearer " + t.token}, nil
}

func (tokenAuth) RequireTransportSecurity() bool { return true }

// Connect establishes a mTLS gRPC connection to a Sliver server using the given config.
// It blocks until the connection reaches READY state or connectTimeout expires.
func Connect(cfg *Config) (rpcpb.SliverRPCClient, *grpc.ClientConn, error) {
	tlsCfg, err := buildTLS(cfg)
	if err != nil {
		return nil, nil, err
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
		grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token}),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)),
	}

	addr := fmt.Sprintf("%s:%d", cfg.LHost, cfg.LPort)
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("creating gRPC client to %s: %w", addr, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	for {
		state := conn.GetState()
		if state == connectivity.Idle {
			conn.Connect()
		}
		if state == connectivity.Ready {
			break
		}
		if !conn.WaitForStateChange(ctx, state) {
			_ = conn.Close()
			return nil, nil, fmt.Errorf("connection to %s timed out: %w", addr, ctx.Err())
		}
	}

	return rpcpb.NewSliverRPCClient(conn), conn, nil
}

func buildTLS(cfg *Config) (*tls.Config, error) {
	cert, err := tls.X509KeyPair([]byte(cfg.Certificate), []byte(cfg.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("parsing client key pair: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(cfg.CACertificate)) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		// Hostname verification is skipped; we verify against the pinned CA instead.
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			roots := x509.NewCertPool()
			roots.AppendCertsFromPEM([]byte(cfg.CACertificate))
			parsed, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return fmt.Errorf("parsing server cert: %w", err)
			}
			if _, err := parsed.Verify(x509.VerifyOptions{Roots: roots}); err != nil {
				return fmt.Errorf("verifying server cert: %w", err)
			}
			return nil
		},
	}, nil
}
