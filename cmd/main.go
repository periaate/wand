package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/periaate/wand"

	gf "github.com/jessevdk/go-flags"
)

const (
	WANAddress  = ":443"
	LinkAddress = "localhost:6060"
)

var (
	cert string
	key  string
	opts *options

	domain string
)

func init() {
	domain = os.Getenv("WAND_DOMAIN")
}

type options struct {
	TLSDir  string   `short:"i" long:"tsldir" description:"Path to folder which contains the certificate and key files"`
	Domain  string   `short:"d" long:"domain" description:"Domain name for the server. Needs to be the same as the certificate's domain name."`
	Invalid []string `short:"x" long:"invalid" description:"Invalid target URLs"`
}

// main sets up the routes and starts the server.
func main() {
	opts = &options{}
	_, err := gf.Parse(opts)
	if err != nil {
		if gf.WroteHelp(err) {
			os.Exit(0)
		}
		log.Fatalln("Error parsing flags:", err)
	}

	if err := checkTLSDir(opts.TLSDir); err != nil {
		log.Fatalln(err)
	}

	if len(opts.Domain) != 0 {
		domain = opts.Domain
	}

	go wand.RunSessionWorker()

	go localServer()
	startServer(http.HandlerFunc(wand.SessionMW))
}

// localServer is a manager server for sessions and authentication links.
func localServer() {
	slog.Info("Starting link management server at", "address", "http://localhost:6060")
	opts.Invalid = append(opts.Invalid, LinkAddress)
	http.ListenAndServe(LinkAddress, wand.LinkHandler(domain, opts.Invalid...))
}

func startServer(h http.Handler) {
	slog.Info("Starting server", "address", WANAddress)
	slog.Info("Press Ctrl+C to stop gracefully...")

	// go shutdown()
	srv := &http.Server{
		Addr:         WANAddress,
		WriteTimeout: time.Second * 120,
		ReadTimeout:  time.Second * 120,
		IdleTimeout:  time.Second * 120,
		Handler:      h,
	}
	if err := srv.ListenAndServeTLS(cert, key); err != nil {
		fmt.Println("error occurred", err)
	}
}

func checkTLSDir(TLSDir string) error {
	if len(TLSDir) == 0 {
		TLSDir = os.Getenv("TLS_DIR")
	}
	if len(TLSDir) != 0 {
		if _, err := os.Stat(TLSDir); os.IsNotExist(err) {
			return fmt.Errorf("TLS directory does not exist: %s", TLSDir)
		}

		cert = filepath.Join(TLSDir, "cert.pem")
		key = filepath.Join(TLSDir, "key.pem")
		if _, err := os.Stat(cert); os.IsNotExist(err) {
			return fmt.Errorf("certificate file does not exist: %s", cert)
		}
		if _, err := os.Stat(key); os.IsNotExist(err) {
			return fmt.Errorf("key file does not exist: %s", key)
		}
		return nil
	}

	return nil
}
