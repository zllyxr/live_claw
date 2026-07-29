package home

type Section[T any] struct {
	Status    string `json:"status"`
	Items     []T    `json:"items"`
	Error     string `json:"error,omitempty"`
	UpdatedAt int64  `json:"updated_at"`
}

type Dashboard struct {
	ServerTime int64                 `json:"server_time"`
	User       *UserSummary          `json:"user,omitempty"`
	Wallet     *WalletSummary        `json:"wallet,omitempty"`
	Banners    Section[Banner]       `json:"banners"`
	Live       Section[LiveRoom]     `json:"live"`
	Sports     Section[SportsMatch]  `json:"sports"`
	Lottery    Section[LotteryGame]  `json:"lottery"`
	Fishing    Section[FishingVenue] `json:"fishing"`
}

type UserSummary struct {
	ID       int64  `json:"id"`
	Nickname string `json:"nickname"`
}

type WalletSummary struct {
	Coin   int64 `json:"coin"`
	Frozen int64 `json:"frozen"`
}

type Banner struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Image       string `json:"image"`
	ActionType  string `json:"action_type"`
	ActionValue string `json:"action_value"`
}

type LiveRoom struct {
	ID             int64  `json:"id"`
	UID            int64  `json:"uid,string"`
	Stream         string `json:"stream"`
	Title          string `json:"title"`
	Thumb          string `json:"thumb"`
	Avatar         string `json:"avatar"`
	UserNickname   string `json:"user_nickname"`
	OnlineCount    int64  `json:"nums"`
	Provider       string `json:"provider"`
	ProviderRoomID string `json:"provider_room_id"`
}

type SportsOption struct {
	ID         int64  `json:"id"`
	OptionCode string `json:"option_code"`
	OptionName string `json:"option_name"`
	OddsScaled int64  `json:"odds_scaled"`
	Odds       string `json:"odds"`
}

type SportsMatch struct {
	ID              int64          `json:"id"`
	MatchID         string         `json:"match_id"`
	Competition     string         `json:"competition"`
	CompetitionType string         `json:"competition_type"`
	HomeName        string         `json:"home_name"`
	AwayName        string         `json:"away_name"`
	HomeLogo        string         `json:"home_logo"`
	AwayLogo        string         `json:"away_logo"`
	HomeScore       int            `json:"home_score"`
	AwayScore       int            `json:"away_score"`
	Status          string         `json:"status"`
	BetStatus       int            `json:"bet_status"`
	KickoffAt       int64          `json:"kickoff_at"`
	BetCloseAt      int64          `json:"bet_close_at"`
	Options         []SportsOption `json:"options"`
}

type LotteryIssue struct {
	ID          int64  `json:"id"`
	IssueNumber string `json:"issue_number"`
	CloseAt     int64  `json:"close_at"`
	DrawAt      int64  `json:"draw_at"`
	Status      int    `json:"status"`
}

type LotteryGame struct {
	ID           int64         `json:"id"`
	CategoryID   int64         `json:"category_id"`
	GameCode     string        `json:"game_code"`
	GameName     string        `json:"game_name"`
	Icon         string        `json:"icon"`
	CurrentIssue *LotteryIssue `json:"current_issue,omitempty"`
}

type FishingVenue struct {
	GameID        int64   `json:"game_id"`
	GameCode      string  `json:"game_code"`
	GameName      string  `json:"game_name"`
	EntryPath     string  `json:"entry_path"`
	VenueID       int64   `json:"venue_id"`
	VenueCode     string  `json:"venue_code"`
	VenueName     string  `json:"venue_name"`
	Multiplier    int     `json:"multiplier"`
	TableCount    int     `json:"table_count"`
	SeatsPerTable int     `json:"seats_per_table"`
	MinBalance    int64   `json:"min_balance"`
	EscrowAmount  int64   `json:"escrow_amount"`
	BetLevels     []int64 `json:"bet_levels"`
}
