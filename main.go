package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tailscale.com/tsnet"
	"tailscale.com/envknob"
)

func main() {
	hostname := flag.String("hostname", "tsexpose", "Tailscale hostname for this node")
	localPort := flag.Int("local-port", 8000, "Local port to expose (required)")
	tsPort := flag.Int("ts-port", 8000, "Port to listen on in tailnet (defaults to local-port)")
	authKey := flag.String("auth-key", "", "Tailscale auth key (or set TS_AUTHKEY env)")
	stateDir := flag.String("state-dir", "", "Directory to store Tailscale state (default: ~/.tsexpose/<hostname>)")
	ephemeral := flag.Bool("ephemeral", false, "Remove node from tailnet on exit")
	tsLogs := flag.Bool("ts-logs", false, "Enable Tailscale logging to log.tailscale.net")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "tsexpose - Expose a local port on your Tailscale network using tsnet\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  tsexpose [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  tsexpose -local-port 3000\n")
		fmt.Fprintf(os.Stderr, "  tsexpose -local-port 8080 -ts-port 443 -hostname myapp\n")
		fmt.Fprintf(os.Stderr, "  tsexpose -local-port 5432 -hostname my-db -ephemeral\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if !*tsLogs {
		envknob.SetNoLogsNoSupport()
	}

	if *localPort == 0 {
		fmt.Fprintf(os.Stderr, "error: -local-port is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	if *tsPort == 0 {
		*tsPort = *localPort
	}

	if *authKey == "" {
		*authKey = os.Getenv("TS_AUTHKEY")
	}

	if *stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("failed to get home directory: %v", err)
		}
		*stateDir = fmt.Sprintf("%s/.tsexpose/%s", home, *hostname)
	}

	if err := os.MkdirAll(*stateDir, 0700); err != nil {
		log.Fatalf("failed to create state directory %s: %v", *stateDir, err)
	}

	srv := &tsnet.Server{
		Hostname:  *hostname,
		AuthKey:   *authKey,
		Dir:       *stateDir,
		Ephemeral: *ephemeral,
	}

	if !*tsLogs {
		srv.Logf = func(string, ...any) {}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("starting tailscale node %q...", *hostname)

	status, err := srv.Up(ctx)
	if err != nil {
		log.Fatalf("failed to start tsnet: %v", err)
	}

	log.Printf("connected to tailnet as %s", status.Self.DNSName)
	for _, ip := range status.TailscaleIPs {
		log.Printf("  tailscale IP: %s", ip)
	}

	listenAddr := fmt.Sprintf(":%d", *tsPort)
	ln, err := srv.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("failed to listen on tailnet port %d: %v", *tsPort, err)
	}
	defer ln.Close()

	localAddr := fmt.Sprintf("127.0.0.1:%d", *localPort)
	log.Printf("proxying %s tailnet:%d -> %s (http)", *hostname, *tsPort, localAddr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("received %s, shutting down...", sig)
		cancel()
		ln.Close()
		srv.Close()
	}()

	serveHTTP(ctx, ln, srv, localAddr)
}

func serveHTTP(ctx context.Context, ln net.Listener, tsSrv *tsnet.Server, localAddr string) {
	target, err := url.Parse(fmt.Sprintf("http://%s", localAddr))
	if err != nil {
		log.Fatalf("invalid local address: %v", err)
	}

	lc, err := tsSrv.LocalClient()
	if err != nil {
		log.Fatalf("failed to get local client: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		who, err := lc.WhoIs(req.Context(), req.RemoteAddr)
		if err == nil && who.UserProfile != nil {
			req.Header.Set("X-Tailscale-User-Login", who.UserProfile.LoginName)
			req.Header.Set("X-Tailscale-User-Name", who.UserProfile.DisplayName)
			req.Header.Set("X-Tailscale-User-Profile-Pic", who.UserProfile.ProfilePicURL)
		}
		if err == nil && who.Node != nil {
			req.Header.Set("X-Tailscale-Node-Name", who.Node.ComputedName)
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("proxy error: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "tsexpose: upstream unreachable: %v\n", err)
	}

	httpSrv := &http.Server{
		Handler:           proxy,
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		httpSrv.Shutdown(shutdownCtx)
	}()

	if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server error: %v", err)
	}
}
