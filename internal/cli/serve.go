package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"relwatch/internal/api"
	"relwatch/internal/config"
	"relwatch/internal/notify"
	"relwatch/internal/service"
	"relwatch/internal/update"
	"relwatch/internal/web"
)

const defaultHTTPAddr = "127.0.0.1:8080"

func newSender(cfg config.Config) (notify.Sender, []string, error) {
	if !cfg.SMTP.Enabled {
		return nil, nil, nil
	}
	sender, err := notify.NewSMTP(cfg.SMTP)
	if err != nil {
		return nil, nil, err
	}
	return sender, cfg.SMTP.To, nil
}

// resolveSchedule picks the poll schedule in precedence order:
//
//	explicit flag  >  RELWATCH_SCHEDULE env  >  settings.conf schedule:  >  default.
//
// An empty string means "not set" and falls through to the next source.
func resolveSchedule(flag, env, config, def string) string {
	for _, v := range []string{flag, env, config, def} {
		if v != "" {
			return v
		}
	}
	return def
}

// resolveHTTP decides whether to start the read-only API and on which address,
// in precedence order:
//
//	--http flag  >  RELWATCH_HTTP_ADDR env  >  settings http: (enabled + listen).
//
// A flag/env address also enables the API; otherwise http.enabled must be true.
// The listen address defaults to loopback (defaultHTTPAddr).
func resolveHTTP(flagAddr, envAddr string, cfg config.HTTPConfig) (bool, string) {
	if flagAddr != "" {
		return true, flagAddr
	}
	if envAddr != "" {
		return true, envAddr
	}
	addr := cfg.Listen
	if addr == "" {
		addr = defaultHTTPAddr
	}
	return cfg.Enabled, addr
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func newServeCmd(opts *options) *cobra.Command {
	var schedule string
	var httpAddr string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Poll providers for releases on a schedule",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := opts.cfg
			sender, recipients, err := newSender(cfg)
			if err != nil {
				return err
			}

			// One long-lived store, shared by the poll loop and the API server.
			st, err := opts.openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			version := cmd.Root().Version
			updater := update.NewClient(updateServer(opts), nil)
			var updateMu sync.Mutex
			var updateStatus *update.Status

			notifyRun := func(ctx context.Context) {
				if st, err := updater.Check(ctx, version); err == nil {
					updateMu.Lock()
					updateStatus = &st
					updateMu.Unlock()
					if st.Available {
						fmt.Fprintf(os.Stderr, "relwatch: a newer version is available: %s (running %s; run 'relwatch upgrade')\n", st.Latest, st.Current)
					}
				}

				includePrereleases, err := resolvePrereleases(false, opts.cfg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "relwatch: %v\n", err)
					return
				}
				newReleases, err := checkAll(ctx, opts, st, "", "", includePrereleases, os.Stderr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "relwatch: check failed: %v\n", err)
					return
				}
				if sender == nil || len(newReleases) == 0 {
					return
				}
				msg := notify.Digest(newReleases)
				updateMu.Lock()
				ust := updateStatus
				updateMu.Unlock()
				if ust != nil && ust.Available {
					msg.Body += fmt.Sprintf("\n\nA newer relwatch version is available: %s (running %s).", ust.Latest, ust.Current)
				}
				msg.To = recipients
				if err := sender.Send(ctx, msg); err != nil {
					fmt.Fprintf(os.Stderr, "relwatch: notification failed: %v\n", err)
					return
				}
				fmt.Fprintf(os.Stderr, "relwatch: sent %d new-release notification(s)\n", len(newReleases))
			}

			notifyRun(cmd.Context())

			schedule = resolveSchedule(schedule, envOr("RELWATCH_SCHEDULE", ""), cfg.Schedule, "0 * * * *")

			c := cron.New()
			if _, err := c.AddFunc(schedule, func() {
				notifyRun(context.Background())
			}); err != nil {
				return err
			}
			c.Start()
			fmt.Printf("relwatch serving, poll schedule %q (Ctrl-C to stop)\n", schedule)

			var httpSrv *http.Server
			if enabled, addr := resolveHTTP(httpAddr, os.Getenv("RELWATCH_HTTP_ADDR"), cfg.HTTP); enabled {
				svc := service.New(st)
				webUI, err := web.New(svc, version)
				if err != nil {
					return fmt.Errorf("web: %w", err)
				}
				apiSrv := api.New(svc, version, cfg.HTTP.Token)
				apiSrv.SetUpdateStatus(func() *update.Status {
					updateMu.Lock()
					defer updateMu.Unlock()
					return updateStatus
				})
				root := http.NewServeMux()
				root.Handle("/", webUI.Routes())
				root.Handle("/api/v1/", apiSrv.Handler())

				ln, err := net.Listen("tcp", addr)
				if err != nil {
					return fmt.Errorf("http: %w", err)
				}
				httpSrv = &http.Server{Handler: root}
				go func() {
					if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
						fmt.Fprintf(os.Stderr, "relwatch: http server: %v\n", err)
					}
				}()
				fmt.Printf("http listening on http://%s (dashboard at /, API at /api/v1)\n", ln.Addr())
				if cfg.HTTP.Token == "" && !isLoopbackAddr(ln.Addr().String()) {
					fmt.Fprintf(os.Stderr, "relwatch: warning: HTTP bound to %s without a token; set http.token or bind to loopback\n", addr)
				}
			}

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
			<-sig

			if httpSrv != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = httpSrv.Shutdown(ctx)
			}
			c.Stop()
			return nil
		},
	}

	cmd.Flags().StringVar(&schedule, "schedule", "", "cron schedule for polling (flag > env > config schedule: > default)")
	cmd.Flags().StringVar(&httpAddr, "http", "", "serve the web dashboard (/) and JSON API (/api/v1) on ADDR, e.g. 127.0.0.1:8080 (implies http.enabled; flag > env RELWATCH_HTTP_ADDR > settings http.listen)")
	return cmd
}
