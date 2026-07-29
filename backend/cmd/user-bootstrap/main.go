package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/config"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/invite"
)

var (
	errAppUserExists = errors.New("app user already exists")
	countryCodeRE    = regexp.MustCompile(`^[0-9]{1,8}$`)
	mobileRE         = regexp.MustCompile(`^[0-9]{5,20}$`)
)

type appUserInput struct {
	Username    string
	Password    string
	Nickname    string
	Email       string
	CountryCode string
}

type preparedAppUser struct {
	appUserInput
	PasswordHash string
}

type appUserResult struct {
	ID         int64
	Username   string
	Nickname   string
	Country    string
	InviteCode string
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	input, err := loadAppUserInput(os.Getenv)
	if err != nil {
		logger.Error("validate app user input", "error", err)
		os.Exit(1)
	}
	prepared, err := prepareAppUser(input)
	if err != nil {
		logger.Error("validate app user password", "error", err)
		os.Exit(1)
	}
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg.MySQLDSN)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	result, err := createAppUser(ctx, db, prepared)
	if errors.Is(err, errAppUserExists) {
		logger.Error(
			"app user already exists; bootstrap never overwrites credentials",
			"username", prepared.Username,
			"country_code", prepared.CountryCode,
		)
		os.Exit(1)
	}
	if err != nil {
		logger.Error("create app user", "error", err)
		os.Exit(1)
	}
	logger.Info(
		"app user created",
		"id", result.ID,
		"username", result.Username,
		"nickname", result.Nickname,
		"country_code", result.Country,
		"invite_code", result.InviteCode,
	)
}

func loadAppUserInput(getenv func(string) string) (appUserInput, error) {
	input := appUserInput{
		Username:    strings.TrimSpace(getenv("V2_APP_USER_USERNAME")),
		Password:    getenv("V2_APP_USER_PASSWORD"),
		Nickname:    strings.TrimSpace(getenv("V2_APP_USER_NICKNAME")),
		Email:       strings.ToLower(strings.TrimSpace(getenv("V2_APP_USER_EMAIL"))),
		CountryCode: strings.TrimSpace(getenv("V2_APP_USER_COUNTRY_CODE")),
	}
	if input.CountryCode == "" {
		input.CountryCode = "86"
	}
	if input.Nickname == "" {
		input.Nickname = input.Username
	}
	switch {
	case input.Username == "":
		return appUserInput{}, errors.New("V2_APP_USER_USERNAME is required")
	case !mobileRE.MatchString(input.Username):
		return appUserInput{}, errors.New("V2_APP_USER_USERNAME must contain 5 to 20 digits for H5 mobile login")
	case input.Password == "":
		return appUserInput{}, errors.New("V2_APP_USER_PASSWORD is required")
	case utf8.RuneCountInString(input.Nickname) > 100:
		return appUserInput{}, errors.New("V2_APP_USER_NICKNAME must not exceed 100 characters")
	case containsControl(input.Nickname):
		return appUserInput{}, errors.New("V2_APP_USER_NICKNAME must not contain control characters")
	case input.Email == "":
		return appUserInput{}, errors.New("V2_APP_USER_EMAIL is required")
	case len(input.Email) > 190:
		return appUserInput{}, errors.New("V2_APP_USER_EMAIL must not exceed 190 characters")
	case !validEmail(input.Email):
		return appUserInput{}, errors.New("V2_APP_USER_EMAIL is invalid")
	case !countryCodeRE.MatchString(input.CountryCode):
		return appUserInput{}, errors.New("V2_APP_USER_COUNTRY_CODE must contain 1 to 8 digits")
	}
	return input, nil
}

func prepareAppUser(input appUserInput) (preparedAppUser, error) {
	if err := validateStrongPassword(input); err != nil {
		return preparedAppUser{}, err
	}
	passwordHash, err := adminauth.HashPassword(input.Password)
	if err != nil {
		return preparedAppUser{}, err
	}
	return preparedAppUser{appUserInput: input, PasswordHash: passwordHash}, nil
}

func validateStrongPassword(input appUserInput) error {
	if len(input.Password) < 12 {
		return errors.New("V2_APP_USER_PASSWORD must contain at least 12 characters")
	}
	if len(input.Password) > 128 {
		return errors.New("V2_APP_USER_PASSWORD must not exceed 128 characters")
	}
	if strings.EqualFold(input.Password, input.Username) ||
		strings.EqualFold(input.Password, input.Email) {
		return errors.New("V2_APP_USER_PASSWORD must not equal the username or email")
	}
	var lower, upper, digit, symbol bool
	for _, value := range input.Password {
		switch {
		case unicode.IsControl(value):
			return errors.New("V2_APP_USER_PASSWORD must not contain control characters")
		case unicode.IsLower(value):
			lower = true
		case unicode.IsUpper(value):
			upper = true
		case unicode.IsDigit(value):
			digit = true
		default:
			symbol = true
		}
	}
	categories := 0
	for _, present := range []bool{lower, upper, digit, symbol} {
		if present {
			categories++
		}
	}
	if categories < 3 {
		return errors.New(
			"V2_APP_USER_PASSWORD must use at least three of lowercase, uppercase, digits and symbols",
		)
	}
	return nil
}

func createAppUser(
	ctx context.Context,
	db *sql.DB,
	input preparedAppUser,
) (appUserResult, error) {
	var systemTeamID int64
	var systemTeamCode string
	if err := db.QueryRowContext(ctx, `
		SELECT id,code FROM teams WHERE code='sys' AND status=1`,
	).Scan(&systemTeamID, &systemTeamCode); errors.Is(err, sql.ErrNoRows) {
		return appUserResult{}, errors.New("active system team is missing; apply migrations first")
	} else if err != nil {
		return appUserResult{}, fmt.Errorf("read system team: %w", err)
	}
	if !invite.ValidTeamCode(systemTeamCode) {
		return appUserResult{}, errors.New("system team has an invalid invitation prefix")
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return appUserResult{}, fmt.Errorf("begin app user transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	insert, err := tx.ExecContext(ctx, `
		INSERT INTO users
			(username,country_code,mobile,email,password_hash,password_algo,nickname,
			 team_id,status,is_virtual)
		VALUES(?,?,?,?,?,'argon2id',?,?,1,0)`,
		input.Username,
		input.CountryCode,
		input.Username,
		input.Email,
		input.PasswordHash,
		input.Nickname,
		systemTeamID,
	)
	if err != nil {
		var mysqlErr *mysqlDriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return appUserResult{}, errAppUserExists
		}
		return appUserResult{}, fmt.Errorf("insert app user: %w", err)
	}
	userID, err := insert.LastInsertId()
	if err != nil {
		return appUserResult{}, fmt.Errorf("read app user id: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO wallet_accounts
			(user_id,currency,available,frozen,status)
		VALUES(?,'COIN',0,0,1)`,
		userID,
	); err != nil {
		return appUserResult{}, fmt.Errorf("create app user wallet: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO team_members
			(user_id,team_id,inviter_user_id,status)
		VALUES(?,?,0,1)`,
		userID, systemTeamID,
	); err != nil {
		return appUserResult{}, fmt.Errorf("join app user to system team: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return appUserResult{}, fmt.Errorf("commit app user: %w", err)
	}

	code, err := invite.New(db).EnsureUserCode(ctx, userID)
	if err != nil {
		cleanupErr := removeIncompleteAppUser(db, userID)
		if cleanupErr != nil {
			return appUserResult{}, errors.Join(
				fmt.Errorf("assign system team and invite code: %w", err),
				fmt.Errorf("remove incomplete app user: %w", cleanupErr),
			)
		}
		return appUserResult{}, fmt.Errorf("assign system team and invite code: %w", err)
	}
	return appUserResult{
		ID: userID, Username: input.Username, Nickname: input.Nickname,
		Country: input.CountryCode, InviteCode: code.FullCode,
	}, nil
}

func removeIncompleteAppUser(db *sql.DB, userID int64) error {
	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cleanup, err := db.BeginTx(cleanupContext, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer cleanup.Rollback() //nolint:errcheck
	for _, query := range []string{
		"DELETE FROM invite_codes WHERE user_id=?",
		"DELETE FROM team_members WHERE user_id=?",
		"DELETE FROM wallet_accounts WHERE user_id=?",
		"DELETE FROM users WHERE id=? AND is_virtual=0",
	} {
		if _, err = cleanup.ExecContext(cleanupContext, query, userID); err != nil {
			return err
		}
	}
	return cleanup.Commit()
}

func validEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && strings.EqualFold(address.Address, value)
}

func containsControl(value string) bool {
	for _, item := range value {
		if unicode.IsControl(item) {
			return true
		}
	}
	return false
}
