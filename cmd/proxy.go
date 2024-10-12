package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
	"io"
	"net"
	"os"
	"os/signal"
)

var forwardCmd = &cobra.Command{
	Use:   "forward [service] [port]",
	Short: "Forward a external port to localhost",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.CreateConfig(configFile)

		if err != nil {
			return err
		}

		client, err := docker.CreateClientFromConfig(cfg)

		if err != nil {
			return err
		}

		defer client.Close()

		containerId, err := docker.FindProjectContainer(cmd.Context(), client, cfg.Name, args[0])

		if err != nil {
			return err
		}

		proxy, err := docker.CreateTCPProxy(cmd.Context(), client, cfg.Server.Address, containerId, args[1])

		if err != nil {
			return err
		}

		cleanUp := func() {
			if err := client.ContainerKill(cmd.Context(), proxy.ProxyContainerId, "SIGKILL"); err != nil {
				log.Printf("Failed to kill proxy container: %s", err)
			}
		}

		defer cleanUp()

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)

		go func() {
			<-c
			cleanUp()
			os.Exit(1)
		}()

		cert, err := tls.X509KeyPair(proxy.Keys.ClientCert, proxy.Keys.ClientKey)

		if err != nil {
			return fmt.Errorf("error loading key pair: %s", err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(proxy.Keys.CaCert)

		tlsConfig := &tls.Config{
			RootCAs:      caCertPool,
			Certificates: []tls.Certificate{cert},
		}

		localPort, _ := cmd.Flags().GetString("local-port")

		localServer, err := net.Listen("tcp", localPort)

		if err != nil {
			return err
		}

		port := localServer.Addr().(*net.TCPAddr).Port

		log.Printf("Forwarded to local port: %d\n", port)

		for {
			client, err := localServer.Accept()

			if err != nil {
				log.Printf("Error accepting connection: %s\n", err)
				continue
			}

			go func() {
				defer client.Close()

				forwardService, err := tls.Dial("tcp", fmt.Sprintf("%s:%s", cfg.Server.Address, proxy.ListenPort), tlsConfig)

				if err != nil {
					log.Printf("Error connecting to forward service: %s\n", err)
					return
				}

				defer forwardService.Close()

				go func() {
					_, err = io.Copy(forwardService, client)

					if err != nil {
						log.Printf("Error copying data to forward service: %s\n", err)
					}
				}()

				_, err = io.Copy(client, forwardService)
			}()
		}
	},
}

func init() {
	rootCmd.AddCommand(forwardCmd)
	forwardCmd.Flags().String("local-port", ":61705", "Local port to forward to")
}
