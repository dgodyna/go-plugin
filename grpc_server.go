package plugin

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// DefaultGRPCServer can be used with the "GRPCServer" field for Server
// as a default factory method to create a gRPC server with no extra options.
func DefaultGRPCServer(opts []grpc.ServerOption) *grpc.Server {
	return grpc.NewServer(opts...)
}

// GRPCServer is a ServerType implementation that serves plugins over
// gRPC. This allows plugins to easily be written for other languages.
//
// The GRPCServer outputs a custom configuration as a base64-encoded
// JSON structure represented by the GRPCServerConfig config structure.
type GRPCServer struct {
	// Plugins are the list of plugins to serve.
	Plugins map[string]Plugin

	// Server is the actual server that will accept connections. This
	// will be used for plugin registration as well.
	Server func([]grpc.ServerOption) *grpc.Server

	// TLS should be the TLS configuration if available. If this is nil,
	// the connection will not have transport security.
	TLS *tls.Config

	// DoneCh is the channel that is closed when this server has exited.
	DoneCh chan struct{}

	// Stdout/StderrLis are the readers for stdout/stderr that will be copied
	// to the stdout/stderr connection that is output.
	Stdout io.Reader
	Stderr io.Reader

	config GRPCServerConfig
	server *grpc.Server
}

// ServerProtocol impl.
func (s *GRPCServer) Init() error {
	// TODO(mitchellh): I don't know why this is the case currently, but
	// I'm getting connection refused errors when trying to use TLS. Given
	// only one project uses this we should look into it later.
	if s.TLS != nil {
		//return fmt.Errorf("TLS is not currently supported with gRPC plugins")
	}

	// Create our server
	var opts []grpc.ServerOption
	if s.TLS != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(s.TLS)))
	}
	s.server = s.Server(opts)

	// Register all our plugins onto the gRPC server.
	for k, raw := range s.Plugins {
		p, ok := raw.(GRPCPlugin)
		if !ok {
			return fmt.Errorf("%q is not a GRPC-compatibile plugin", k)
		}

		if err := p.GRPCServer(s.server); err != nil {
			return fmt.Errorf("error registring %q: %s", k, err)
		}
	}

	return nil
}

// Config is the GRPCServerConfig encoded as JSON then base64.
func (s *GRPCServer) Config() string {
	// Create a buffer that will contain our final contents
	var buf bytes.Buffer

	// Wrap the base64 encoding with JSON encoding.
	if err := json.NewEncoder(&buf).Encode(s.config); err != nil {
		// We panic since ths shouldn't happen under any scenario. We
		// carefully control the structure being encoded here and it should
		// always be successful.
		panic(err)
	}

	return buf.String()
}

func (s *GRPCServer) Serve(lis net.Listener) {
	// Start serving in a goroutine
	go s.server.Serve(lis)

	// Wait until graceful completion
	<-s.DoneCh
}

// GRPCServerConfig is the extra configuration passed along for consumers
// to facilitate using GRPC plugins.
type GRPCServerConfig struct {
	StdoutAddr string `json:"stdout_addr"`
	StderrAddr string `json:"stderr_addr"`
}
