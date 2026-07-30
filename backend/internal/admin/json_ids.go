package admin

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

// apiDecimalID keeps database identifiers exact when they cross the JSON/JavaScript
// boundary. JavaScript numbers cannot represent every 64-bit integer.
func apiDecimalID(id int64) string {
	return strconv.FormatInt(id, 10)
}

func nonNegativeDecimalID(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "0" {
		return 0, nil
	}
	id, err := positiveDecimalID(value)
	if err != nil {
		return 0, errors.New("invalid non-negative decimal id")
	}
	return id, nil
}

// decimalIDInput accepts both legacy JSON numbers and the preferred decimal
// strings without ever routing the value through a floating-point number.
type decimalIDInput int64

func (id *decimalIDInput) UnmarshalJSON(data []byte) error {
	raw := string(data)
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
	}
	value, err := nonNegativeDecimalID(raw)
	if err != nil {
		return err
	}
	*id = decimalIDInput(value)
	return nil
}

func (id decimalIDInput) MarshalJSON() ([]byte, error) {
	return json.Marshal(apiDecimalID(id.Int64()))
}

func (id decimalIDInput) Int64() int64 {
	return int64(id)
}

// decimalIDListInput is the array counterpart to decimalIDInput. It keeps
// compatibility with older clients that sent JSON numbers while allowing the
// browser to send exact decimal strings for BIGINT identifiers.
type decimalIDListInput []decimalIDInput

func (ids decimalIDListInput) Int64s() []int64 {
	values := make([]int64, 0, len(ids))
	for _, id := range ids {
		values = append(values, id.Int64())
	}
	return values
}

func adminIdentityForAPI(admin adminauth.Admin) map[string]any {
	return map[string]any{
		"id":           apiDecimalID(admin.ID),
		"username":     admin.Username,
		"display_name": admin.DisplayName,
		"permissions":  admin.Permissions,
	}
}

func walletEntryForAPI(entry wallet.Entry) map[string]any {
	return map[string]any{
		"entry_no":        entry.EntryNo,
		"user_id":         apiDecimalID(entry.UserID),
		"available":       entry.Available,
		"frozen":          entry.Frozen,
		"delta_available": entry.DeltaAvailable,
		"delta_frozen":    entry.DeltaFrozen,
		"business_type":   entry.BusinessType,
		"business_id":     entry.BusinessID,
		"game_code":       entry.GameCode,
		"venue_code":      entry.VenueCode,
		"table_no":        entry.TableNo,
		"round_no":        entry.RoundNo,
	}
}
