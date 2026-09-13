// Command server is the Kulhanek Points Tracker web application: it loads
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

	"github.com/wkulhanek/kulhanek-points-tracker/internal/accounts"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/auth"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/backup"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/config"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/db"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/email"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/email/factory"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/email/gmail"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/notifications"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/preferences"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
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

	var gmailOAuth *gmail.OAuth
	if cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "" {
		gmailOAuth = gmail.NewOAuth(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.BaseURL)
	}
	senderFactory := factory.New(emailSettings, gmailOAuth)

	deps := &web.Deps{
		AuthStore:     authStore,
		LoginLimiter:  auth.NewLoginLimiter(),
		Accounts:      accountsSvc,
		Preferences:   prefsStore,
		EmailSettings: emailSettings,
		GmailOAuth:    gmailOAuth,
		SenderFactory: senderFactory,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	notifScheduler := notifications.NewScheduler(accountsSvc, prefsStore, notificationsStore, senderFactory)
	go notifScheduler.Run(ctx)

	backupScheduler := backup.NewScheduler(conn, cfg.DataDir)
	go backupScheduler.Run(ctx)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           web.NewRouter(deps),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown", "error", err)
		}
	}()

	slog.Info("kulhanek-points-tracker listening", "port", cfg.Port, "data_dir", cfg.DataDir)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
