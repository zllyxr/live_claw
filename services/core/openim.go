package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type IMUser struct {
	UserID   string `json:"userID"`
	Nickname string `json:"nickname"`
	FaceURL  string `json:"faceURL"`
}

type IMSession struct {
	UserID            string `json:"userID"`
	Token             string `json:"token"`
	ExpireTimeSeconds int64  `json:"expireTimeSeconds"`
	APIAddress        string `json:"apiAddr"`
	WSAddress         string `json:"wsAddr"`
}

type OpenIMClient struct {
	apiURL          string
	gatewayURL      string
	secret          string
	adminUserID     string
	publicAPI       string
	publicWS        string
	httpClient      *http.Client
	logger          *slog.Logger
	mu              sync.Mutex
	adminToken      string
	adminTokenUntil time.Time
	mutedGroups     sync.Map
}

func NewOpenIMClient(cfg Config, logger *slog.Logger) *OpenIMClient {
	return &OpenIMClient{
		apiURL: cfg.OpenIMAPIURL, gatewayURL: cfg.OpenIMGatewayURL, secret: cfg.OpenIMSecret, adminUserID: cfg.OpenIMAdminUserID,
		publicAPI: cfg.OpenIMPublicAPIAddress, publicWS: cfg.OpenIMPublicWSAddress,
		httpClient: &http.Client{Timeout: 8 * time.Second}, logger: logger,
	}
}

func (c *OpenIMClient) Configured() bool {
	return c.apiURL != "" && c.secret != "" && c.adminUserID != ""
}

func loadIMUser(ctx context.Context, db *sql.DB, uid int64) (IMUser, error) {
	var user IMUser
	err := db.QueryRowContext(ctx,
		"SELECT CAST(id AS CHAR), COALESCE(NULLIF(user_nickname,''), NULLIF(user_login,''), CONCAT('用户',id)), COALESCE(NULLIF(avatar_thumb,''),avatar,'') FROM cmf_user WHERE id=? AND user_status=1",
		uid,
	).Scan(&user.UserID, &user.Nickname, &user.FaceURL)
	if errors.Is(err, sql.ErrNoRows) {
		return IMUser{}, appError(404, "用户不存在或已停用")
	}
	return user, err
}

func (c *OpenIMClient) EnsureUserSession(ctx context.Context, user IMUser, platformID int) (IMSession, error) {
	adminToken, err := c.getAdminToken(ctx)
	if err != nil {
		return IMSession{}, err
	}

	var queryResponse struct {
		ErrCode int `json:"errCode"`
		Data    struct {
			Users []IMUser `json:"usersInfo"`
		} `json:"data"`
	}
	err = c.post(ctx, "/user/get_users_info", adminToken, map[string]any{"userIDs": []string{user.UserID}}, &queryResponse)
	if err != nil || queryResponse.ErrCode != 0 || len(queryResponse.Data.Users) == 0 {
		var registerResponse openIMResponse
		if registerErr := c.post(ctx, "/user/user_register", adminToken, map[string]any{"users": []IMUser{user}}, &registerResponse); registerErr != nil {
			return IMSession{}, registerErr
		}
		if registerResponse.ErrCode != 0 {
			return IMSession{}, fmt.Errorf("openim register user: code=%d msg=%s", registerResponse.ErrCode, registerResponse.ErrMsg)
		}
	}

	var tokenResponse struct {
		openIMResponse
		Data struct {
			Token             string `json:"token"`
			ExpireTimeSeconds any    `json:"expireTimeSeconds"`
		} `json:"data"`
	}
	if err := c.post(ctx, "/auth/get_user_token", adminToken, map[string]any{
		"platformID": platformID,
		"userID":     user.UserID,
	}, &tokenResponse); err != nil {
		return IMSession{}, err
	}
	if tokenResponse.ErrCode != 0 || tokenResponse.Data.Token == "" {
		return IMSession{}, fmt.Errorf("openim user token: code=%d msg=%s", tokenResponse.ErrCode, tokenResponse.ErrMsg)
	}
	return IMSession{
		UserID: user.UserID, Token: tokenResponse.Data.Token,
		ExpireTimeSeconds: valueInt64(tokenResponse.Data.ExpireTimeSeconds),
		APIAddress:        c.publicAPI, WSAddress: c.publicWS,
	}, nil
}

func (c *OpenIMClient) EnsureUsers(ctx context.Context, users []IMUser) error {
	adminToken, err := c.getAdminToken(ctx)
	if err != nil {
		return err
	}
	for _, user := range users {
		if err := c.ensureUser(ctx, adminToken, user); err != nil {
			return fmt.Errorf("ensure openim user %s: %w", user.UserID, err)
		}
	}
	return nil
}

func (c *OpenIMClient) EnsureLiveGroup(ctx context.Context, liveID, liveName string, user IMUser) (string, error) {
	liveID = strings.TrimSpace(liveID)
	if liveID == "" || len(liveID) > 120 {
		return "", fmt.Errorf("invalid live id")
	}
	adminToken, err := c.getAdminToken(ctx)
	if err != nil {
		return "", err
	}
	bots := []IMUser{
		{UserID: "claw_live_bot", Nickname: "直播助手"},
		{UserID: "claw_moderator_bot", Nickname: "直播管理"},
	}
	for _, bot := range bots {
		if err := c.ensureUser(ctx, adminToken, bot); err != nil {
			return "", err
		}
	}
	groupID := liveGroupID(liveID)
	var groupResponse struct {
		openIMResponse
		Data struct {
			Groups []map[string]any `json:"groupInfos"`
		} `json:"data"`
	}
	err = c.post(ctx, "/group/get_groups_info", adminToken, map[string]any{"groupIDs": []string{groupID}}, &groupResponse)
	if err != nil {
		return "", err
	}
	if groupResponse.ErrCode != 0 || len(groupResponse.Data.Groups) == 0 {
		if strings.TrimSpace(liveName) == "" {
			liveName = "直播间 " + liveID
		}
		var createResponse openIMResponse
		err = c.post(ctx, "/group/create_group", adminToken, map[string]any{
			"ownerUserID":   bots[0].UserID,
			"adminUserIDs":  []string{bots[1].UserID},
			"memberUserIDs": []string{user.UserID},
			"groupInfo": map[string]any{
				"groupID": groupID, "groupName": liveName, "groupType": 2,
				"needVerification": 2, "lookMemberInfo": 0, "applyMemberFriend": 1,
				"ex": `{"kind":"claw_live"}`,
			},
		}, &createResponse)
		if err != nil {
			return "", err
		}
		if createResponse.ErrCode != 0 {
			return "", fmt.Errorf("openim create live group: code=%d msg=%s", createResponse.ErrCode, createResponse.ErrMsg)
		}
		if err := c.ensureLiveGroupMuted(ctx, adminToken, groupID); err != nil {
			return "", err
		}
		return groupID, nil
	}

	var memberResponse struct {
		openIMResponse
		Data struct {
			Members []map[string]any `json:"members"`
		} `json:"data"`
	}
	err = c.post(ctx, "/group/get_group_members_info", adminToken, map[string]any{
		"groupID": groupID, "userIDs": []string{user.UserID},
	}, &memberResponse)
	if err == nil && memberResponse.ErrCode == 0 && len(memberResponse.Data.Members) > 0 {
		if err := c.ensureLiveGroupMuted(ctx, adminToken, groupID); err != nil {
			return "", err
		}
		return groupID, nil
	}
	var joinResponse openIMResponse
	err = c.post(ctx, "/group/invite_user_to_group", adminToken, map[string]any{
		"groupID": groupID, "invitedUserIDs": []string{user.UserID}, "reason": "进入直播间",
	}, &joinResponse)
	if err != nil {
		return "", err
	}
	if joinResponse.ErrCode != 0 {
		return "", fmt.Errorf("openim join live group: code=%d msg=%s", joinResponse.ErrCode, joinResponse.ErrMsg)
	}
	if err := c.ensureLiveGroupMuted(ctx, adminToken, groupID); err != nil {
		return "", err
	}
	return groupID, nil
}

func liveGroupID(liveID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(liveID)))
	return fmt.Sprintf("claw_live_%x", sum[:12])
}

func (c *OpenIMClient) ensureLiveGroupMuted(ctx context.Context, adminToken, groupID string) error {
	if _, loaded := c.mutedGroups.Load(groupID); loaded {
		return nil
	}
	var response openIMResponse
	if err := c.post(ctx, "/group/mute_group", adminToken, map[string]any{"groupID": groupID}, &response); err != nil {
		return err
	}
	if response.ErrCode != 0 {
		return fmt.Errorf("openim mute live group: code=%d msg=%s", response.ErrCode, response.ErrMsg)
	}
	c.mutedGroups.Store(groupID, struct{}{})
	return nil
}

func (c *OpenIMClient) SendLiveCustomMessage(ctx context.Context, groupID string, payload any, system bool) (string, error) {
	adminToken, err := c.getAdminToken(ctx)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sendID := "claw_live_bot"
	if system {
		sendID = "claw_moderator_bot"
	}
	if err := c.ensureUser(ctx, adminToken, IMUser{UserID: sendID, Nickname: "直播助手"}); err != nil {
		return "", err
	}
	var response struct {
		openIMResponse
		Data struct {
			ServerMsgID string `json:"serverMsgID"`
		} `json:"data"`
	}
	err = c.post(ctx, "/msg/send_msg", adminToken, map[string]any{
		"sendID":  sendID,
		"recvID":  groupID,
		"groupID": groupID,
		"content": map[string]any{
			"data":        string(body),
			"description": "Claw live event",
			"extension":   "claw.live.v1",
		},
		"contentType":    110,
		"sessionType":    3,
		"isOnlineOnly":   true,
		"notOfflinePush": true,
		"sendTime":       time.Now().UnixMilli(),
	}, &response)
	if err != nil {
		return "", err
	}
	if response.ErrCode != 0 {
		return "", fmt.Errorf("openim send live message: code=%d msg=%s", response.ErrCode, response.ErrMsg)
	}
	return response.Data.ServerMsgID, nil
}

func (c *OpenIMClient) ensureUser(ctx context.Context, adminToken string, user IMUser) error {
	var queryResponse struct {
		openIMResponse
		Data struct {
			Users []IMUser `json:"usersInfo"`
		} `json:"data"`
	}
	if err := c.post(ctx, "/user/get_users_info", adminToken, map[string]any{"userIDs": []string{user.UserID}}, &queryResponse); err == nil && queryResponse.ErrCode == 0 && len(queryResponse.Data.Users) > 0 {
		return nil
	}
	var registerResponse openIMResponse
	if err := c.post(ctx, "/user/user_register", adminToken, map[string]any{"users": []IMUser{user}}, &registerResponse); err != nil {
		return err
	}
	if registerResponse.ErrCode != 0 {
		return fmt.Errorf("openim register system user: code=%d msg=%s", registerResponse.ErrCode, registerResponse.ErrMsg)
	}
	return nil
}

type openIMResponse struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
	ErrDlt  string `json:"errDlt"`
}

func (c *OpenIMClient) getAdminToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.adminToken != "" && time.Now().Before(c.adminTokenUntil) {
		return c.adminToken, nil
	}

	var response struct {
		openIMResponse
		Data struct {
			Token             string `json:"token"`
			ExpireTimeSeconds any    `json:"expireTimeSeconds"`
		} `json:"data"`
	}
	if err := c.post(ctx, "/auth/get_admin_token", "", map[string]any{
		"secret": c.secret,
		"userID": c.adminUserID,
	}, &response); err != nil {
		return "", err
	}
	if response.ErrCode != 0 || response.Data.Token == "" {
		return "", fmt.Errorf("openim admin token: code=%d msg=%s", response.ErrCode, response.ErrMsg)
	}
	expires := valueInt64(response.Data.ExpireTimeSeconds)
	if expires < 300 {
		expires = 3600
	}
	c.adminToken = response.Data.Token
	c.adminTokenUntil = time.Now().Add(time.Duration(expires-120) * time.Second)
	return c.adminToken, nil
}

func (c *OpenIMClient) post(ctx context.Context, path, token string, body any, target any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("operationID", strconv.FormatInt(time.Now().UnixNano(), 10))
	if token != "" {
		req.Header.Set("token", token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("openim %s returned %d: %s", path, resp.StatusCode, string(responseBody))
	}
	if err := json.Unmarshal(responseBody, target); err != nil {
		return fmt.Errorf("decode openim %s: %w", path, err)
	}
	return nil
}
