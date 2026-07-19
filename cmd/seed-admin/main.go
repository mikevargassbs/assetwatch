// seed-admin creates the first Admin user directly, bypassing the normal
// "only Admins can create users" rule, since there is no admin yet on a
// fresh install.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"sbs-bsp-cctv/internal/auth"
	"sbs-bsp-cctv/internal/config"
	"sbs-bsp-cctv/internal/db"
	"sbs-bsp-cctv/internal/mailer"
	"sbs-bsp-cctv/internal/rbac"
)

func main() {
	email := flag.String("email", "", "admin email, must end in @sbs.com.pg")
	fullName := flag.String("name", "", "admin full name")
	password := flag.String("password", "", "admin password")
	flag.Parse()

	if *email == "" || *fullName == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "usage: seed-admin -email you@sbs.com.pg -name \"Your Name\" -password \"...\"")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	tokenIssuer := auth.NewTokenIssuer(cfg.JWTSecret)
	authSvc := auth.NewService(pool, tokenIssuer, cfg.AllowedEmailDomain, mailer.Config{})

	userID, err := authSvc.CreateUser(ctx, *email, *fullName, *password, []string{rbac.Admin})
	if err != nil {
		log.Fatalf("create admin: %v", err)
	}

	fmt.Printf("Created super user %s (%s)\n", *email, userID)
}
