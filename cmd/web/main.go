// Command web serves the go_muse dashboard: a Svelte single-page app (embedded
// into this binary) backed by a small JSON API. It reads the gomuse analysis
// database for audio attributes and accepts a PixelPlayer .pxpl backup upload
// for listening data, then visualizes the library and generates .m3u playlists
// from a recommendation engine.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/Ahdeyyy/go_muse/internal/webapi"
)

func main() {
	var (
		dbPath = flag.String("db", "gomuse.db", "gomuse SQLite database path")
		addr   = flag.String("addr", "127.0.0.1:8765", "listen address")
		open   = flag.Bool("open", true, "open the dashboard in a browser on start")
	)
	flag.Parse()

	if err := run(*dbPath, *addr, *open); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(dbPath, addr string, open bool) error {
	srv, err := webapi.New(dbPath)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	httpSrv := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	url := "http://" + ln.Addr().String()
	fmt.Printf("go_muse dashboard listening on %s\n", url)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	if open {
		go openBrowser(url)
	}

	select {
	case <-ctx.Done():
		fmt.Println("\nshutting down…")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutCtx)
	case err := <-errc:
		return err
	}
}

// openBrowser best-effort launches the default browser at url.
func openBrowser(url string) {
	time.Sleep(300 * time.Millisecond) // let the listener settle
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd, args = "open", []string{url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	_ = exec.Command(cmd, args...).Start()
}
