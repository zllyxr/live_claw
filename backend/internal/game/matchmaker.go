package game

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	FishingTableCount = 300
	FishingSeatCount  = 4
)

var ErrVenueFull = errors.New("venue is full")

type Assignment struct {
	Table   int
	Seat    int
	Resumed bool
}

type Matchmaker struct {
	redis *redis.Client
	now   func() time.Time
}

func NewMatchmaker(redisClient *redis.Client) *Matchmaker {
	return &Matchmaker{redis: redisClient, now: time.Now}
}

var assignScript = redis.NewScript(`
local prefix = KEYS[1]
local user_key = prefix .. ':user:' .. ARGV[1]
local current = redis.call('GET', user_key)
if current then
  return {current, '1'}
end
local start_table = tonumber(ARGV[2])
local start_seat = tonumber(ARGV[3])
for table_offset = 0, 299 do
  local table_no = ((start_table - 1 + table_offset) % 300) + 1
  local seat_key = prefix .. ':table:' .. table_no
  for seat_offset = 0, 3 do
    local seat_no = ((start_seat - 1 + seat_offset) % 4) + 1
    if not redis.call('HGET', seat_key, tostring(seat_no)) then
      redis.call('HSET', seat_key, tostring(seat_no), ARGV[1])
      local assignment = tostring(table_no) .. ':' .. tostring(seat_no)
      redis.call('SET', user_key, assignment)
      redis.call('HSET', prefix .. ':heartbeat', ARGV[1], ARGV[4])
      return {assignment, '0'}
    end
  end
end
return {'', '0'}
`)

var releaseScript = redis.NewScript(`
local prefix = KEYS[1]
local user_key = prefix .. ':user:' .. ARGV[1]
local current = redis.call('GET', user_key)
if not current then
  return 0
end
local separator = string.find(current, ':')
local table_no = string.sub(current, 1, separator - 1)
local seat_no = string.sub(current, separator + 1)
local seat_key = prefix .. ':table:' .. table_no
if redis.call('HGET', seat_key, seat_no) == ARGV[1] then
  redis.call('HDEL', seat_key, seat_no)
end
redis.call('DEL', user_key)
redis.call('HDEL', prefix .. ':heartbeat', ARGV[1])
return 1
`)

var reserveScript = redis.NewScript(`
local prefix = KEYS[1]
local user_key = prefix .. ':user:' .. ARGV[1]
local expected = ARGV[2] .. ':' .. ARGV[3]
local current = redis.call('GET', user_key)
if current then
  if current == expected then
    redis.call('HSET', prefix .. ':heartbeat', ARGV[1], ARGV[4])
    return 1
  end
  return 0
end
local seat_key = prefix .. ':table:' .. ARGV[2]
local occupant = redis.call('HGET', seat_key, ARGV[3])
if occupant and occupant ~= ARGV[1] then
  return 0
end
redis.call('HSET', seat_key, ARGV[3], ARGV[1])
redis.call('SET', user_key, expected)
redis.call('HSET', prefix .. ':heartbeat', ARGV[1], ARGV[4])
return 1
`)

func (m *Matchmaker) AssignFishing(ctx context.Context, venueID, userID int64) (Assignment, error) {
	if m.redis == nil || venueID < 1 || userID < 1 {
		return Assignment{}, errors.New("invalid match request")
	}
	startTable, err := secureIndex(FishingTableCount)
	if err != nil {
		return Assignment{}, err
	}
	startSeat, err := secureIndex(FishingSeatCount)
	if err != nil {
		return Assignment{}, err
	}
	prefix := fmt.Sprintf("game:v2:fishing:venue:%d", venueID)
	raw, err := assignScript.Run(ctx, m.redis, []string{prefix},
		strconv.FormatInt(userID, 10),
		strconv.Itoa(startTable+1),
		strconv.Itoa(startSeat+1),
		strconv.FormatInt(m.now().UnixMilli(), 10),
	).Slice()
	if err != nil {
		return Assignment{}, err
	}
	if len(raw) != 2 || fmt.Sprint(raw[0]) == "" {
		return Assignment{}, ErrVenueFull
	}
	table, seat, err := parseAssignment(fmt.Sprint(raw[0]))
	if err != nil {
		return Assignment{}, err
	}
	return Assignment{Table: table, Seat: seat, Resumed: fmt.Sprint(raw[1]) == "1"}, nil
}

func (m *Matchmaker) ReleaseFishing(ctx context.Context, venueID, userID int64) error {
	if m.redis == nil || venueID < 1 || userID < 1 {
		return errors.New("invalid release request")
	}
	prefix := fmt.Sprintf("game:v2:fishing:venue:%d", venueID)
	return releaseScript.Run(ctx, m.redis, []string{prefix}, strconv.FormatInt(userID, 10)).Err()
}

func (m *Matchmaker) ReserveFishing(ctx context.Context, venueID, userID int64, table, seat int) error {
	if m.redis == nil || venueID < 1 || userID < 1 ||
		table < 1 || table > FishingTableCount || seat < 1 || seat > FishingSeatCount {
		return errors.New("invalid reserve request")
	}
	prefix := fmt.Sprintf("game:v2:fishing:venue:%d", venueID)
	result, err := reserveScript.Run(
		ctx,
		m.redis,
		[]string{prefix},
		strconv.FormatInt(userID, 10),
		strconv.Itoa(table),
		strconv.Itoa(seat),
		strconv.FormatInt(m.now().UnixMilli(), 10),
	).Int()
	if err != nil {
		return err
	}
	if result != 1 {
		return errors.New("fishing seat is occupied")
	}
	return nil
}

func secureIndex(limit int) (int, error) {
	if limit < 1 {
		return 0, errors.New("invalid random limit")
	}
	limit64 := uint64(limit)
	maximum := ^uint64(0) - (^uint64(0) % limit64)
	for {
		var buffer [8]byte
		if _, err := rand.Read(buffer[:]); err != nil {
			return 0, err
		}
		value := binary.BigEndian.Uint64(buffer[:])
		if value < maximum {
			return int(value % limit64), nil
		}
	}
}

func parseAssignment(value string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, errors.New("invalid assignment")
	}
	table, tableErr := strconv.Atoi(parts[0])
	seat, seatErr := strconv.Atoi(parts[1])
	if tableErr != nil || seatErr != nil || table < 1 || table > FishingTableCount || seat < 1 || seat > FishingSeatCount {
		return 0, 0, errors.New("invalid assignment")
	}
	return table, seat, nil
}
