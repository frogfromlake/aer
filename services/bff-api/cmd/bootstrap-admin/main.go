// Command bootstrap-admin creates the first admin account (Phase 134 / ADR-040).
//
// Self-registration is closed (LICENSE §3.2), so the first admin cannot be
// invited by another admin — this one-shot tool seeds it. It connects under
// the `bff_auth` role, ensures an admin account exists for
// ADMIN_BOOTSTRAP_EMAIL, issues a single-use invite token, and prints the
// accept-invite link. The admin then sets their own password and records the
// responsible-use consent through the normal accept-invite flow — no password
// is ever seeded. Re-running issues a fresh link (e.g. if the first expired).
//
// Run via `make create-admin` (reads .env). It refuses to change the role of
// an existing non-admin account.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/viper"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bootstrap-admin: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	v := viper.New()
	v.AutomaticEnv()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig()
	v.SetDefault("POSTGRES_HOST", "localhost")
	v.SetDefault("POSTGRES_PORT", "5432")
	v.SetDefault("POSTGRES_DB", "aer_metadata")
	v.SetDefault("BFF_INVITE_TTL_SECONDS", 259200)

	email := v.GetString("ADMIN_BOOTSTRAP_EMAIL")
	authUser := v.GetString("BFF_AUTH_DB_USER")
	authPass := v.GetString("BFF_AUTH_DB_PASSWORD")
	if email == "" {
		return fmt.Errorf("ADMIN_BOOTSTRAP_EMAIL must be set")
	}
	if authUser == "" || authPass == "" {
		return fmt.Errorf("BFF_AUTH_DB_USER and BFF_AUTH_DB_PASSWORD must be set")
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		url.QueryEscape(authUser), url.QueryEscape(authPass),
		v.GetString("POSTGRES_HOST"), v.GetString("POSTGRES_PORT"), v.GetString("POSTGRES_DB"))
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}

	store := storage.NewAuthStore(db)

	existing, err := store.GetUserByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("look up admin: %w", err)
	}
	var userID string
	switch {
	case existing == nil:
		userID, err = store.CreateInvitedUser(ctx, email, string(auth.RoleAdmin))
		if err != nil {
			return fmt.Errorf("create admin: %w", err)
		}
		fmt.Printf("Created admin (invited): %s\n", email)
	case existing.Role != string(auth.RoleAdmin):
		return fmt.Errorf("user %s already exists with role %q — refusing to change it automatically", email, existing.Role)
	default:
		userID = existing.ID
		fmt.Printf("Admin %s already exists (status=%s); issuing a fresh invite link.\n", email, existing.Status)
	}

	raw, hash, err := auth.GenerateOpaqueToken()
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}
	ttl := time.Duration(v.GetInt("BFF_INVITE_TTL_SECONDS")) * time.Second
	if err := store.CreateToken(ctx, userID, "invite", hash, time.Now().Add(ttl)); err != nil {
		return fmt.Errorf("create invite token: %w", err)
	}

	link := v.GetString("BFF_PUBLIC_BASE_URL") + "/accept-invite?token=" + raw
	fmt.Println("\nAccept-invite link (deliver to the admin; sets their password + consent):")
	fmt.Println("  " + link)
	return nil
}
