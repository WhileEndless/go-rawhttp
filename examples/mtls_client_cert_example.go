// Example: Using client certificates for mutual TLS (mTLS) authentication
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/WhileEndless/go-rawhttp/v2"
)

func main() {
	// Example 1: Using PEM byte arrays
	exampleWithPEM()

	// Example 2: Using file paths
	exampleWithFiles()
}

func exampleWithPEM() {
	// Your client certificate and key in PEM format
	clientCertPEM := []byte(`-----BEGIN CERTIFICATE-----
MIICljCCAX4CCQCFcV9...
-----END CERTIFICATE-----`)

	clientKeyPEM := []byte(`-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhk...
-----END PRIVATE KEY-----`)

	sender := rawhttp.NewSender()

	req := []byte(`GET /api/secure HTTP/1.1
Host: mtls-server.example.com
User-Agent: go-rawhttp/1.1.6
Connection: close

`)

	opts := rawhttp.Options{
		Scheme: "https",
		Host:   "mtls-server.example.com",
		Port:   443,

		// Client certificate for mTLS
		ClientCertPEM: clientCertPEM,
		ClientKeyPEM:  clientKeyPEM,
	}

	resp, err := sender.Do(context.Background(), req, opts)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Body: %d bytes\n", resp.BodyBytes)
}

func exampleWithFiles() {
	sender := rawhttp.NewSender()

	req := []byte(`GET /api/secure HTTP/1.1
Host: mtls-server.example.com
User-Agent: go-rawhttp/1.1.6
Connection: close

`)

	opts := rawhttp.Options{
		Scheme: "https",
		Host:   "mtls-server.example.com",
		Port:   443,

		// Client certificate from files
		ClientCertFile: "/path/to/client.crt",
		ClientKeyFile:  "/path/to/client.key",
	}

	resp, err := sender.Do(context.Background(), req, opts)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Body: %d bytes\n", resp.BodyBytes)
}

// Example 3: mTLS with custom CA certificates (self-signed server)
func exampleWithCustomCA() {
	caCertPEM := []byte(`-----BEGIN CERTIFICATE-----
... your CA certificate ...
-----END CERTIFICATE-----`)

	clientCertPEM := []byte(`... your client cert ...`)
	clientKeyPEM := []byte(`... your client key ...`)

	sender := rawhttp.NewSender()

	req := []byte(`GET /api/secure HTTP/1.1
Host: self-signed-mtls.example.com
Connection: close

`)

	opts := rawhttp.Options{
		Scheme: "https",
		Host:   "self-signed-mtls.example.com",
		Port:   8443,

		// Custom CA to trust server's self-signed cert
		CustomCACerts: [][]byte{caCertPEM},

		// Client certificate for mTLS
		ClientCertPEM: clientCertPEM,
		ClientKeyPEM:  clientKeyPEM,
	}

	resp, err := sender.Do(context.Background(), req, opts)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Body: %d bytes\n", resp.BodyBytes)
}
