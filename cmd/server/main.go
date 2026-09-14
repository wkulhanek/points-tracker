// Command server is the Points Tracker web application: it loads
// configuration, opens the SQLite database, wires up every domain package,
// and runs the HTTP server alongside the notification and backup
// schedulers until it receives a shutdown signal.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wkulhanek/points-tracker/internal/accounts"
	"github.com/wkulhanek/points-tracker/internal/auth"
	"github.com/wkulhanek/points-tracker/internal/backup"
	"github.com/wkulhanek/points-tracker/internal/config"
	"github.com/wkulhanek/points-tracker/internal/db"
	"github.com/wkulhanek/points-tracker/internal/email"
	"github.com/wkulhanek/points-tracker/internal/email/factory"
	"github.com/wkulhanek/points-tracker/internal/notifications"
	"github.com/wkulhanek/points-tracker/internal/preferences"
	"github.com/wkulhanek/points-tracker/internal/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Everything this process creates (the SQLite file, WAL/SHM files, and
	// daily backup snapshots) holds session hashes and email credentials,
	// so default every new file/directory to owner-only regardless of the
	// host's umask. Without this, e.g. backup.go's post-creation
	// os.Chmod(0o600) on each snapshot still leaves a brief window right
	// after VACUUM INTO creates the file where a permissive host umask
	// (022) would make it group/world-readable.
	syscall.Umask(0o077)

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return err
	}

	conn, err := db.Open(cfg.DataDir)
	if err != nil {
		return err
	}
	defer conn.Close()

	authStore := auth.NewStore(conn)
	if err := auth.BootstrapAdmin(authStore, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		return err
	}

	accountsSvc := accounts.NewService(accounts.NewStore(conn))
	prefsStore := preferences.NewStore(conn)
	emailSettings := email.NewSettingsStore(conn)
	notificationsStore := notifications.NewStore(conn)

	senderFactory := factory.New(emailSettings)

	deps := &web.Deps{
		AuthStore:     authStore,
		LoginLimiter:  auth.NewLoginLimiter(),
		Accounts:      accountsSvc,
		Preferences:   prefsStore,
		EmailSettings: emailSettings,
		SenderFactory: senderFactory,
		TrustProxy:    cfg.TrustProxy,
		BaseURL:       cfg.BaseURL,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	notifScheduler := notifications.NewScheduler(accountsSvc, prefsStore, notificationsStore, senderFactory)
	go notifScheduler.Run(ctx)

	backupScheduler := backup.NewScheduler(conn, cfg.DataDir)
	go backupScheduler.Run(ctx)

	// Periodically drop expired sessions and stale rate-limiter entries so
	// neither grows without bound over a long uptime.
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := authStore.DeleteExpiredSessions(); err != nil {
					slog.Error("prune expired sessions", "error", err)
				}
				deps.LoginLimiter.Cleanup()
			}
		}
	}()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           web.NewRouter(deps),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown", "error", err)
		}
	}()

	slog.Info("points-tracker listening", "port", cfg.Port, "data_dir", cfg.DataDir)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
