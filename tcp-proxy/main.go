package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		log.Println("Usage: tcp-proxy <container-ip:container-port> <external-port>")
		os.Exit(1)
	}

	containerConnect := os.Args[1]
	externalPort := os.Args[2]

	caCertEncoded := os.Getenv("TLS_CA_CERT")

	if caCertEncoded == "" {
		log.Fatalf("TLS_CA_CERT env var is required")
	}

	caCert, err := base64.StdEncoding.DecodeString(caCertEncoded)

	if err != nil {
		log.Fatalf("error decoding CA certificate: %v", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	serverCrtEncoded := os.Getenv("TLS_SERVER_CERT")

	if serverCrtEncoded == "" {
		log.Fatalf("TLS_SERVER_CERT env var is required")
	}

	serverCrt, err := base64.StdEncoding.DecodeString(serverCrtEncoded)

	if err != nil {
		log.Fatalf("error decoding server certificate: %v", err)
	}

	serverKeyEncoded := os.Getenv("TLS_SERVER_KEY")

	if serverKeyEncoded == "" {
		log.Fatalf("TLS_SERVER_KEY env var is required")
	}

	serverKey, err := base64.StdEncoding.DecodeString(serverKeyEncoded)

	if err != nil {
		log.Fatalf("error decoding server key: %v", err)
	}

	cer, err := tls.X509KeyPair(serverCrt, serverKey)
	if err != nil {
		log.Println(err)
		return
	}

	config := &tls.Config{
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cer},
	}

	conn, err := tls.Listen("tcp", fmt.Sprintf(":%s", externalPort), config)

	if err != nil {
		panic(err)
	}

	log.Printf("Listening on port %s\n", externalPort)

	for {
		client, err := conn.Accept()

		if err != nil {
			log.Printf("Error accepting connection: %s\n", err)
			continue
		}

		go handleClient(client, containerConnect)
	}
}

func handleClient(client net.Conn, containerConnect string) {
	defer client.Close()

	forwardService, err := net.Dial("tcp", containerConnect)

	if err != nil {
		log.Printf("Error connecting to forward service (%s): %s\n", containerConnect, err)
		return
	}

	defer forwardService.Close()

	go func() {
		_, err = io.Copy(forwardService, client)

		if err != nil {
			log.Printf("Error copying data to forward service (%s): %s\n", containerConnect, err)
		}
	}()

	_, err = io.Copy(client, forwardService)

	if err != nil {
		log.Printf("Error copying data to client (%s): %s\n", client.RemoteAddr(), err)
	}
}
