package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/shavandi/store/backend/internal/auth"
	"github.com/shavandi/store/backend/internal/config"
	"github.com/shavandi/store/backend/internal/postgres"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Admin provisioning failed:", err)
		os.Exit(1)
	}
}

func run(arguments []string, input io.Reader, output io.Writer) error {
	flags := flag.NewFlagSet("adminctl", flag.ContinueOnError)
	phone := flags.String("phone", "", "Sajad's Iranian mobile number")
	fromStdin := flags.Bool("password-stdin", false, "read the password from standard input; never pass it as an argument")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if !*fromStdin || *phone == "" || flags.NArg() != 0 {
		return errors.New("usage: adminctl --phone <mobile> --password-stdin")
	}
	data, err := io.ReadAll(io.LimitReader(input, 1027))
	if err != nil {
		return errors.New("read password from standard input")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if len(password) > 1024 {
		return auth.ErrInvalid
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	authConfig, err := config.LoadAuthentication()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := postgres.Open(ctx, cfg.Database)
	if err != nil {
		return errors.New("connect to PostgreSQL")
	}
	defer pool.Close()
	service, err := auth.NewService(postgres.NewAuthStore(pool), auth.UnavailableSMS{}, authConfig.Secret)
	if err != nil {
		return err
	}
	user, err := service.ProvisionAdmin(ctx, *phone, password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalid) || errors.Is(err, auth.ErrAdminExists) {
			return err
		}
		return errors.New("database provisioning transaction failed")
	}
	_, err = fmt.Fprintln(output, "Administrator provisioned:", user.ID)
	return err
}
