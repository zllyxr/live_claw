package home

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type SQLLoader struct {
	db           *sql.DB
	mediaBaseURL string
}

func NewSQLLoader(db *sql.DB, mediaBaseURL string) *SQLLoader {
	return &SQLLoader{db: db, mediaBaseURL: strings.TrimRight(mediaBaseURL, "/")}
}

func (l *SQLLoader) Banners(ctx context.Context) ([]Banner, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT b.id,b.title,b.subtitle,COALESCE(a.bucket,''),COALESCE(a.object_key,''),
		       b.action_type,b.action_value
		FROM home_banners b
		LEFT JOIN media_assets a ON a.id=b.image_asset_id AND a.status=1
		WHERE b.status=1
		  AND (b.starts_at IS NULL OR b.starts_at<=CURRENT_TIMESTAMP(3))
		  AND (b.ends_at IS NULL OR b.ends_at>CURRENT_TIMESTAMP(3))
		ORDER BY b.sort_order DESC,b.id DESC
		LIMIT 10`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Banner, 0)
	for rows.Next() {
		var item Banner
		var bucket, objectKey string
		if err := rows.Scan(&item.ID, &item.Title, &item.Subtitle, &bucket, &objectKey, &item.ActionType, &item.ActionValue); err != nil {
			return nil, err
		}
		item.Image = l.assetURL(bucket, objectKey)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (l *SQLLoader) LiveRooms(ctx context.Context) ([]LiveRoom, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT room.id,room.host_user_id,room.room_no,room.title,
		       COALESCE(cover.bucket,''),COALESCE(cover.object_key,''),
		       COALESCE(avatar.bucket,''),COALESCE(avatar.object_key,''),
		       COALESCE(NULLIF(profile.nickname,''),NULLIF(u.nickname,''),u.username),
		       room.provider,room.provider_room_id,
		       COALESCE(profile.cover_url,''),COALESCE(profile.avatar_url,'')
		FROM live_rooms room
		JOIN users u ON u.id=room.host_user_id AND u.status=1
		LEFT JOIN douyin_room_profiles profile ON profile.live_room_id=room.id
		LEFT JOIN media_assets cover ON cover.id=room.cover_asset_id AND cover.status=1
		LEFT JOIN media_assets avatar ON avatar.id=u.avatar_asset_id AND avatar.status=1
		WHERE room.status=1 AND room.provider='douyin' AND profile.verify_status=1
		ORDER BY room.sort_order DESC,room.last_seen_at DESC,room.id DESC
		LIMIT 6`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]LiveRoom, 0)
	for rows.Next() {
		var item LiveRoom
		var coverBucket, coverKey, avatarBucket, avatarKey string
		var remoteCover, remoteAvatar string
		if err := rows.Scan(
			&item.ID, &item.UID, &item.Stream, &item.Title,
			&coverBucket, &coverKey, &avatarBucket, &avatarKey,
			&item.UserNickname, &item.Provider, &item.ProviderRoomID,
			&remoteCover, &remoteAvatar,
		); err != nil {
			return nil, err
		}
		item.Thumb = l.assetURL(coverBucket, coverKey)
		item.Avatar = l.assetURL(avatarBucket, avatarKey)
		if item.Thumb == "" && validRemoteAssetURL(remoteCover) {
			item.Thumb = remoteCover
		}
		if item.Avatar == "" && validRemoteAssetURL(remoteAvatar) {
			item.Avatar = remoteAvatar
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (l *SQLLoader) SportsMatches(ctx context.Context) ([]SportsMatch, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT id,public_match_id,competition,competition_type,home_name,away_name,
		       home_logo_url,away_logo_url,home_score,away_score,match_status,bet_status,
		       CAST(UNIX_TIMESTAMP(kickoff_at) AS UNSIGNED),
		       CAST(UNIX_TIMESTAMP(bet_close_at) AS UNSIGNED)
		FROM sports_matches
		WHERE match_status NOT IN ('FT','CANCELLED')
		  AND EXISTS (
			  SELECT 1
			  FROM sports_markets visible_market
			  JOIN sports_market_options visible_option
			    ON visible_option.market_id=visible_market.id
			   AND visible_option.status=1
			   AND visible_option.odds_scaled>1000000
			  WHERE visible_market.match_id=sports_matches.id
			    AND visible_market.status=1
		  )
		ORDER BY (match_status NOT IN ('1H','HT','2H','LIVE')) ASC,
		         (bet_status<>1) ASC,kickoff_at ASC,id ASC
		LIMIT 3`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]SportsMatch, 0, 3)
	for rows.Next() {
		var item SportsMatch
		if err := rows.Scan(
			&item.ID, &item.MatchID, &item.Competition, &item.CompetitionType,
			&item.HomeName, &item.AwayName, &item.HomeLogo, &item.AwayLogo,
			&item.HomeScore, &item.AwayScore, &item.Status, &item.BetStatus,
			&item.KickoffAt, &item.BetCloseAt,
		); err != nil {
			return nil, err
		}
		options, err := l.sportsOptions(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		item.Options = options
		items = append(items, item)
	}
	return items, rows.Err()
}

func (l *SQLLoader) sportsOptions(ctx context.Context, matchID int64) ([]SportsOption, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT option_item.id,option_item.option_code,option_item.name,option_item.odds_scaled
		FROM sports_markets market
		JOIN sports_market_options option_item ON option_item.market_id=market.id AND option_item.status=1
		WHERE market.match_id=? AND market.status=1
		ORDER BY (market.market_code NOT IN ('1x2','match_winner')) ASC,
		         market.sort_order DESC,option_item.id ASC
		LIMIT 3`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]SportsOption, 0, 3)
	for rows.Next() {
		var item SportsOption
		if err := rows.Scan(&item.ID, &item.OptionCode, &item.OptionName, &item.OddsScaled); err != nil {
			return nil, err
		}
		item.Odds = formatOdds(item.OddsScaled)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (l *SQLLoader) LotteryGames(ctx context.Context) ([]LotteryGame, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT game.id,game.category_id,game.game_code,game.name,
		       issue.id,issue.issue_no,
		       CAST(UNIX_TIMESTAMP(issue.sale_close_at) AS UNSIGNED),
		       CAST(UNIX_TIMESTAMP(issue.draw_at) AS UNSIGNED),issue.status
		FROM lottery_games game
		LEFT JOIN lottery_issues issue ON issue.id=(
		    SELECT current_issue.id FROM lottery_issues current_issue
		    WHERE current_issue.game_id=game.id AND current_issue.status IN (1,2)
		    ORDER BY (current_issue.status=1) DESC,
		             current_issue.sale_close_at ASC,current_issue.id ASC LIMIT 1
		)
		WHERE game.status=1
		ORDER BY game.sort_order DESC,game.id ASC
		LIMIT 4`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]LotteryGame, 0, 4)
	for rows.Next() {
		var item LotteryGame
		var issueID sql.NullInt64
		var issueNo sql.NullString
		var closeAt, drawAt, status sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.CategoryID, &item.GameCode, &item.GameName,
			&issueID, &issueNo, &closeAt, &drawAt, &status,
		); err != nil {
			return nil, err
		}
		item.Icon = staticLotteryIconURL(item.GameCode)
		if issueID.Valid {
			item.CurrentIssue = &LotteryIssue{
				ID: issueID.Int64, IssueNumber: issueNo.String,
				CloseAt: closeAt.Int64, DrawAt: drawAt.Int64, Status: int(status.Int64),
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (l *SQLLoader) FishingVenues(ctx context.Context) ([]FishingVenue, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT game.id,game.game_code,game.name,game.entry_path,
		       venue.id,venue.venue_code,venue.name,venue.multiplier,
		       venue.table_count,venue.seats_per_table,venue.min_balance,
		       venue.escrow_amount,venue.bet_levels
		FROM games game
		JOIN game_venues venue ON venue.game_id=game.id AND venue.status=1
		WHERE game.game_code='deepsea_hunter' AND game.status=1
		ORDER BY venue.multiplier ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]FishingVenue, 0, 3)
	for rows.Next() {
		var item FishingVenue
		var raw []byte
		if err := rows.Scan(
			&item.GameID, &item.GameCode, &item.GameName, &item.EntryPath,
			&item.VenueID, &item.VenueCode, &item.VenueName, &item.Multiplier,
			&item.TableCount, &item.SeatsPerTable, &item.MinBalance,
			&item.EscrowAmount, &raw,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &item.BetLevels); err != nil {
			return nil, fmt.Errorf("decode venue %s bet levels: %w", item.VenueCode, err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (l *SQLLoader) Wallet(ctx context.Context, userID int64) (WalletSummary, error) {
	var wallet WalletSummary
	err := l.db.QueryRowContext(ctx, `
		SELECT available,frozen FROM wallet_accounts
		WHERE user_id=? AND currency='COIN' AND status=1`, userID,
	).Scan(&wallet.Coin, &wallet.Frozen)
	if err == sql.ErrNoRows {
		return WalletSummary{}, nil
	}
	return wallet, err
}

func (l *SQLLoader) assetURL(bucket, objectKey string) string {
	bucket = strings.Trim(bucket, "/")
	objectKey = strings.Trim(objectKey, "/")
	if bucket == "" || objectKey == "" {
		return ""
	}
	segments := strings.Split(objectKey, "/")
	for index := range segments {
		segments[index] = url.PathEscape(segments[index])
	}
	base := l.mediaBaseURL
	escapedBucket := url.PathEscape(bucket)
	if strings.HasSuffix(base, "/"+escapedBucket) {
		return base + "/" + strings.Join(segments, "/")
	}
	return base + "/" + escapedBucket + "/" + strings.Join(segments, "/")
}

func staticLotteryIconURL(gameCode string) string {
	code := strings.ToUpper(strings.TrimSpace(gameCode))
	if code == "" {
		return ""
	}
	return "/lottery-icons/" + url.PathEscape(code) + ".png"
}

func validRemoteAssetURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Hostname() != ""
}

func formatOdds(scaled int64) string {
	whole := scaled / 1_000_000
	fraction := scaled % 1_000_000
	text := strconv.FormatInt(whole, 10) + "." + fmt.Sprintf("%06d", fraction)
	return strings.TrimRight(strings.TrimRight(text, "0"), ".")
}
