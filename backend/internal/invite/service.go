package invite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

var (
	teamPattern = regexp.MustCompile(`^[0-9a-z]{3}$`)
	codePattern = regexp.MustCompile(`^[0-9a-z]{3}-[0-9a-z]{4}$`)
)

type Code struct {
	TeamCode     string
	PersonalCode string
	FullCode     string
}

type BindResult struct {
	InviterUserID int64
	TeamID        int64
	InviteCode    string
	UserCode      Code
	AlreadyBound  bool
}

type Service struct {
	db     *sql.DB
	random io.Reader
}

func New(db *sql.DB) *Service {
	return &Service{db: db, random: rand.Reader}
}

func Normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) == 7 && !strings.Contains(value, "-") {
		value = value[:3] + "-" + value[3:]
	}
	return value
}

func ValidTeamCode(value string) bool {
	return teamPattern.MatchString(Normalize(value))
}

func ValidCode(value string) bool {
	return codePattern.MatchString(Normalize(value))
}

func GeneratePart(source io.Reader, length int) (string, error) {
	if length < 1 || length > 32 {
		return "", fmt.Errorf("invalid code length %d", length)
	}
	result := make([]byte, 0, length)
	var buffer [32]byte
	for len(result) < length {
		if _, err := io.ReadFull(source, buffer[:]); err != nil {
			return "", fmt.Errorf("read invite entropy: %w", err)
		}
		for _, value := range buffer {
			if value >= 252 {
				continue
			}
			result = append(result, alphabet[int(value)%len(alphabet)])
			if len(result) == length {
				break
			}
		}
	}
	return string(result), nil
}

func (s *Service) CreateTeam(ctx context.Context, name, requestedCode string, createdBy int64) (int64, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, "", errors.New("team name is required")
	}
	requestedCode = Normalize(requestedCode)
	if requestedCode != "" && !ValidTeamCode(requestedCode) {
		return 0, "", errors.New("team code must contain exactly three lowercase letters or digits")
	}
	for attempt := 0; attempt < 64; attempt++ {
		code := requestedCode
		if code == "" {
			var err error
			code, err = GeneratePart(s.random, 3)
			if err != nil {
				return 0, "", err
			}
		}
		result, err := s.db.ExecContext(ctx,
			"INSERT INTO teams(code,name,status,created_by) VALUES(?,?,1,?)", code, name, createdBy,
		)
		if err == nil {
			id, idErr := result.LastInsertId()
			return id, code, idErr
		}
		if !isDuplicate(err) || requestedCode != "" {
			return 0, "", err
		}
	}
	return 0, "", errors.New("team code space is temporarily unavailable")
}

func (s *Service) AssignUserCode(ctx context.Context, userID, teamID int64, teamCode string) (Code, error) {
	teamCode = Normalize(teamCode)
	if userID < 1 || teamID < 1 || !ValidTeamCode(teamCode) {
		return Code{}, errors.New("invalid invite assignment")
	}
	var existing Code
	err := s.db.QueryRowContext(ctx, `
		SELECT team_code,personal_code,full_code FROM invite_codes WHERE user_id=?`, userID,
	).Scan(&existing.TeamCode, &existing.PersonalCode, &existing.FullCode)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Code{}, err
	}

	for attempt := 0; attempt < 64; attempt++ {
		personal, err := GeneratePart(s.random, 4)
		if err != nil {
			return Code{}, err
		}
		code := Code{TeamCode: teamCode, PersonalCode: personal, FullCode: teamCode + "-" + personal}
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO invite_codes(user_id,team_id,team_code,personal_code,full_code,status)
			VALUES(?,?,?,?,?,1)`,
			userID, teamID, code.TeamCode, code.PersonalCode, code.FullCode,
		)
		if err == nil {
			return code, nil
		}
		if !isDuplicate(err) {
			return Code{}, err
		}
		// A concurrent request may have assigned this user while this request
		// collided on a personal code.
		if queryErr := s.db.QueryRowContext(ctx, `
			SELECT team_code,personal_code,full_code FROM invite_codes WHERE user_id=?`, userID,
		).Scan(&existing.TeamCode, &existing.PersonalCode, &existing.FullCode); queryErr == nil {
			return existing, nil
		}
	}
	return Code{}, errors.New("personal invite code space is temporarily unavailable")
}

func (s *Service) EnsureUserCode(ctx context.Context, userID int64) (Code, error) {
	if userID < 1 {
		return Code{}, errors.New("invalid user id")
	}
	var existing Code
	err := s.db.QueryRowContext(ctx, `
		SELECT team_code,personal_code,full_code FROM invite_codes WHERE user_id=?`,
		userID,
	).Scan(&existing.TeamCode, &existing.PersonalCode, &existing.FullCode)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Code{}, err
	}
	var teamID int64
	var teamCode string
	err = s.db.QueryRowContext(ctx, `
		SELECT team.id,team.code
		FROM users user
		JOIN teams team ON team.id=user.team_id AND team.status=1
		WHERE user.id=?`,
		userID,
	).Scan(&teamID, &teamCode)
	if errors.Is(err, sql.ErrNoRows) {
		err = s.db.QueryRowContext(ctx, `
			SELECT id,code FROM teams WHERE code='sys' AND status=1`,
		).Scan(&teamID, &teamCode)
		if err != nil {
			return Code{}, err
		}
		if _, err = s.db.ExecContext(ctx, `
			UPDATE users SET team_id=? WHERE id=?;
			INSERT INTO team_members(user_id,team_id,inviter_user_id,status)
			VALUES(?,?,0,1)
			ON DUPLICATE KEY UPDATE team_id=VALUES(team_id),status=1,left_at=NULL`,
			teamID, userID, userID, teamID,
		); err != nil {
			return Code{}, err
		}
	} else if err != nil {
		return Code{}, err
	}
	return s.AssignUserCode(ctx, userID, teamID, teamCode)
}

func (s *Service) Bind(ctx context.Context, inviteeUserID int64, rawCode, source string) (BindResult, error) {
	code := Normalize(rawCode)
	if inviteeUserID < 1 || !ValidCode(code) {
		return BindResult{}, errors.New("invalid invite code")
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "manual"
	}
	if len(source) > 30 {
		source = source[:30]
	}

	for attempt := 0; attempt < 64; attempt++ {
		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if err != nil {
			return BindResult{}, err
		}
		result, retry, bindErr := s.bindTx(ctx, tx, inviteeUserID, code, source)
		if bindErr != nil {
			_ = tx.Rollback()
			if retry {
				continue
			}
			return BindResult{}, bindErr
		}
		if err = tx.Commit(); err != nil {
			return BindResult{}, err
		}
		return result, nil
	}
	return BindResult{}, errors.New("personal invite code space is temporarily unavailable")
}

func (s *Service) bindTx(
	ctx context.Context,
	tx *sql.Tx,
	inviteeUserID int64,
	code, source string,
) (BindResult, bool, error) {
	var inviterUserID, teamID int64
	var inviterCode, teamCode string
	err := tx.QueryRowContext(ctx, `
		SELECT invite.user_id,invite.team_id,invite.full_code,invite.team_code
		FROM invite_codes invite
		WHERE invite.full_code=? AND invite.status=1`,
		code,
	).Scan(&inviterUserID, &teamID, &inviterCode, &teamCode)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
			SELECT alias.user_id,invite.team_id,invite.full_code,invite.team_code
			FROM invite_code_aliases alias
			JOIN invite_codes invite ON invite.user_id=alias.user_id AND invite.status=1
			WHERE alias.alias_code=? AND alias.expires_at>CURRENT_TIMESTAMP(3)`,
			code,
		).Scan(&inviterUserID, &teamID, &inviterCode, &teamCode)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return BindResult{}, false, errors.New("invite code does not exist")
	}
	if err != nil {
		return BindResult{}, false, err
	}
	if inviterUserID == inviteeUserID {
		return BindResult{}, false, errors.New("cannot bind your own invite code")
	}
	var currentTeamID int64
	if err = tx.QueryRowContext(ctx, `
		SELECT team_id FROM users WHERE id=? FOR UPDATE`,
		inviteeUserID,
	).Scan(&currentTeamID); errors.Is(err, sql.ErrNoRows) {
		return BindResult{}, false, errors.New("invitee user does not exist")
	} else if err != nil {
		return BindResult{}, false, err
	}

	var boundInviter, boundTeam int64
	var boundCode string
	err = tx.QueryRowContext(ctx, `
		SELECT inviter_user_id,team_id,invite_code
		FROM invite_relations WHERE invitee_user_id=?`,
		inviteeUserID,
	).Scan(&boundInviter, &boundTeam, &boundCode)
	if err == nil {
		var userCode Code
		_ = tx.QueryRowContext(ctx, `
			SELECT team_code,personal_code,full_code FROM invite_codes WHERE user_id=?`,
			inviteeUserID,
		).Scan(&userCode.TeamCode, &userCode.PersonalCode, &userCode.FullCode)
		return BindResult{
			InviterUserID: boundInviter, TeamID: boundTeam, InviteCode: boundCode,
			UserCode: userCode, AlreadyBound: true,
		}, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return BindResult{}, false, err
	}

	var oldCode string
	_ = tx.QueryRowContext(ctx, `
		SELECT full_code FROM invite_codes WHERE user_id=? FOR UPDATE`,
		inviteeUserID,
	).Scan(&oldCode)
	if oldCode != "" {
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO invite_code_aliases(alias_code,user_id,expires_at)
			VALUES(?,?,CURRENT_TIMESTAMP(3)+INTERVAL 180 DAY)
			ON DUPLICATE KEY UPDATE user_id=VALUES(user_id),expires_at=VALUES(expires_at)`,
			oldCode, inviteeUserID,
		); err != nil {
			return BindResult{}, false, err
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM invite_codes WHERE user_id=?", inviteeUserID); err != nil {
			return BindResult{}, false, err
		}
	}

	personal, err := GeneratePart(s.random, 4)
	if err != nil {
		return BindResult{}, false, err
	}
	userCode := Code{
		TeamCode: teamCode, PersonalCode: personal, FullCode: teamCode + "-" + personal,
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO invite_codes(user_id,team_id,team_code,personal_code,full_code,status)
		VALUES(?,?,?,?,?,1)`,
		inviteeUserID, teamID, userCode.TeamCode, userCode.PersonalCode, userCode.FullCode,
	); err != nil {
		if isDuplicate(err) {
			return BindResult{}, true, err
		}
		return BindResult{}, false, err
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO team_members(user_id,team_id,inviter_user_id,status)
		VALUES(?,?,?,1)
		ON DUPLICATE KEY UPDATE
			team_id=VALUES(team_id),inviter_user_id=VALUES(inviter_user_id),status=1,left_at=NULL`,
		inviteeUserID, teamID, inviterUserID,
	); err != nil {
		return BindResult{}, false, err
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO invite_relations
			(invitee_user_id,inviter_user_id,team_id,invite_code,source,confidence)
		VALUES(?,?,?,?,?,100)`,
		inviteeUserID, inviterUserID, teamID, inviterCode, source,
	); err != nil {
		return BindResult{}, false, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE users SET team_id=? WHERE id=?", teamID, inviteeUserID); err != nil {
		return BindResult{}, false, err
	}
	return BindResult{
		InviterUserID: inviterUserID, TeamID: teamID, InviteCode: inviterCode, UserCode: userCode,
	}, false, nil
}

func isDuplicate(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
