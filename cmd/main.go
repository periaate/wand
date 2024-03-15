package main

import (
	"bufio"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/periaate/cf/dns"
	"github.com/periaate/clmux"
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
	TLSDir       string   `short:"t" long:"tlsdir" description:"Path to folder which contains the certificate and key files"`
	Domain       string   `short:"d" long:"domain" description:"Domain name for the server. Needs to be the same as the certificate's domain name."`
	Invalid      []string `short:"x" long:"invalid" description:"Invalid target URLs"`
	AutoFlareCfg bool     `long:"flare" description:"Automatically configure cloudflare records to point to current IP. Requires 'CF_API', 'CF_ZONE', 'CF_DOMAIN' env variables to be set."`
	API          bool     `long:"api" description:"Runs the web API for link creation"`
}

func checkDNS() {
	apiToken := os.Getenv("CF_API")
	zoneID := os.Getenv("CF_ZONE")
	domainName := os.Getenv("CF_DOMAIN")

	api, err := dns.InitCFAPI(apiToken)
	if err != nil {
		log.Fatalln("Failed to initialize Cloudflare API:", err)
	}

	if err := dns.EnsureDNS(api, zoneID, domainName); err != nil {
		log.Fatalln("error ensuring dns:", err)
	}

	fmt.Println("successfully updated DNS records")
}

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

	if opts.AutoFlareCfg {
		checkDNS()
	}

	link := clmux.MakeView("cmd", 0)
	proxy := clmux.MakeView("proxy", clmux.DefaultMaxEntries)

	mux := &clmux.Mux{
		Input:  os.Stdin,
		Output: os.Stdout,

		Views: make(map[string]clmux.Source),
	}

	mux.Src = link
	mux.Register(link, proxy)

	proxyLogger := proxy.Slogger()
	slog.SetDefault(proxyLogger)

	opts.Invalid = append(opts.Invalid, LinkAddress)
	fn := wand.MakeLinkFn(domain, opts.Invalid...)
	h := wand.LinkHandler(fn)

	go startCLI(link, mux, fn)
	go wand.RunSessionWorker()

	if opts.API {
		go localServer(h)
	}
	startServer(http.HandlerFunc(wand.SessionProxy))
}

func startCLI(clog clmux.Logger, mux *clmux.Mux, fn wand.LinkBuilder) {
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for {
			clog.Log("Enter command ('proxy' for http logs, 'link' to generate link):\n")
			fmt.Println()
			if !scanner.Scan() {
				continue
			}

			command := scanner.Text()
			switch command {
			case "proxy":
				mux.SetView("proxy")
			case "link":
				clog.Log("Target:\n")
				if !scanner.Scan() {
					continue
				}
				target := scanner.Text()
				ld, err := wand.BuildLinkData(target, false, "", "", "")
				if err != nil {
					clog.Log(fmt.Sprint("error:", err))
					continue
				}
				res, err := fn(ld)
				if err != nil {
					clog.Log(fmt.Sprint("error:", err))
					continue
				}

				clog.Log("Link succesfully built\n")
				clog.Log(res)
				clog.Log("\n")
				clog.Log("Press any button to continue\n")
				scanner.Scan()

			case "exit":
				os.Exit(0)
			default:
				mux.SetView("cmd")
			}
		}
	}()
}

func localServer(h http.HandlerFunc) {
	slog.Info("Starting link management server at", "address", "http://"+LinkAddress)
	http.ListenAndServe(LinkAddress, h)
}

func startServer(h http.Handler) {
	slog.Info("Starting server", "address", WANAddress)
	slog.Info("Press Ctrl+C to stop gracefully...")

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
		TLSDir = os.Getenv("WAND_TLS_DIR")
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
