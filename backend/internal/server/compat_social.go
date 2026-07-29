package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type compatPostRow struct {
	ID           int64
	UserID       int64
	PostType     int
	Content      string
	LikeCount    int64
	CommentCount int64
	CreatedAt    time.Time
	Nickname     string
	AvatarKey    string
	MediaKeys    string
	IsLiked      int
}

func (s *Server) compatSocial(w http.ResponseWriter, r *http.Request, service string) bool {
	switch service {
	case "Dynamic.getRecommendDynamics", "Dynamic.getNewDynamic":
		s.compatPostList(w, r, "", 0)
	case "Dynamic.getAttentionDynamic":
		s.compatPostList(w, r, "follow", 0)
	case "Dynamic.getHomeDynamic":
		s.compatPostList(w, r, "user", compatInt64(r.FormValue("touid")))
	case "Dynamic.getDynamic":
		s.compatGetPost(w, r)
	case "Dynamic.setDynamic":
		s.compatCreatePost(w, r)
	case "Dynamic.del":
		s.compatDeletePost(w, r)
	case "Dynamic.report":
		s.compatReportPost(w, r)
	case "Dynamic.addLike":
		s.compatTogglePostLike(w, r)
	case "Dynamic.getComments":
		s.compatGetComments(w, r)
	case "Dynamic.getReplys":
		s.compatGetReplies(w, r)
	case "Dynamic.setComment":
		s.compatCreateComment(w, r)
	case "Dynamic.delComments":
		s.compatDeleteComment(w, r)
	case "Message.GetList", "Message.atLists", "Message.praiseLists", "Message.commentLists", "Message.fansLists":
		s.compatNotifications(w, r, service)
	case "Video.getMyVideo":
		s.compatVideoList(w, r, false)
	case "Video.getLikeVideos":
		s.compatVideoList(w, r, true)
	case "Video.getVideo":
		s.compatGetVideo(w, r)
	default:
		return false
	}
	return true
}

func (s *Server) queryCompatPosts(
	ctx context.Context, viewerID int64, mode string, targetUserID int64, page string, videoOnly bool,
) ([]map[string]any, error) {
	limit, offset := compatPage(page)
	conditions := []string{"post.status=1", "post.visibility=1"}
	args := make([]any, 0, 8)
	joins := ""
	switch mode {
	case "follow":
		joins = " JOIN user_follows follow ON follow.target_user_id=post.user_id AND follow.user_id=? "
		args = append(args, viewerID)
	case "user":
		conditions = append(conditions, "post.user_id=?")
		args = append(args, targetUserID)
	case "liked":
		joins = " JOIN social_reactions liked ON liked.target_type=1 AND liked.target_id=post.id AND liked.user_id=? "
		args = append(args, viewerID)
	}
	if videoOnly {
		conditions = append(conditions, "post.post_type=2")
	}
	args = append(args, viewerID, limit, offset)
	query := `
		SELECT post.id,post.user_id,post.post_type,post.content,post.like_count,post.comment_count,
		       post.created_at,COALESCE(NULLIF(user.nickname,''),user.username),
		       COALESCE(avatar.object_key,''),
		       COALESCE((
		         SELECT GROUP_CONCAT(asset.object_key ORDER BY media.sort_order SEPARATOR ';')
		         FROM social_post_media media
		         JOIN media_assets asset ON asset.id=media.asset_id AND asset.status=1
		         WHERE media.post_id=post.id
		       ),''),
		       EXISTS(
		         SELECT 1 FROM social_reactions reaction
		         WHERE reaction.target_type=1 AND reaction.target_id=post.id AND reaction.user_id=?
		       )
		FROM social_posts post ` + joins + `
		JOIN users user ON user.id=post.user_id AND user.status=1
		LEFT JOIN media_assets avatar ON avatar.id=user.avatar_asset_id AND avatar.status=1
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY post.created_at DESC,post.id DESC LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var row compatPostRow
		if err = rows.Scan(
			&row.ID, &row.UserID, &row.PostType, &row.Content, &row.LikeCount, &row.CommentCount,
			&row.CreatedAt, &row.Nickname, &row.AvatarKey, &row.MediaKeys, &row.IsLiked,
		); err != nil {
			return nil, err
		}
		items = append(items, s.compatPostItem(row))
	}
	return items, rows.Err()
}

func (s *Server) compatPostItem(row compatPostRow) map[string]any {
	rawKeys := strings.Split(row.MediaKeys, ";")
	media := make([]string, 0, len(rawKeys))
	for _, key := range rawKeys {
		if url := s.mediaURL(key); url != "" {
			media = append(media, url)
		}
	}
	thumb := ""
	href := ""
	voice := ""
	switch row.PostType {
	case 1:
		thumb = strings.Join(media, ";")
	case 2:
		if len(media) > 0 {
			thumb = media[0]
		}
		if len(media) > 1 {
			href = media[1]
		}
	case 3:
		if len(media) > 0 {
			voice = media[0]
		}
	}
	user := map[string]any{
		"id": strconv.FormatInt(row.UserID, 10), "uid": strconv.FormatInt(row.UserID, 10),
		"user_nicename": row.Nickname, "user_nickname": row.Nickname,
		"avatar": s.mediaURL(row.AvatarKey), "avatar_thumb": s.mediaURL(row.AvatarKey),
	}
	return map[string]any{
		"id": strconv.FormatInt(row.ID, 10), "dynamicid": strconv.FormatInt(row.ID, 10),
		"videoid": strconv.FormatInt(row.ID, 10), "uid": strconv.FormatInt(row.UserID, 10),
		"type": strconv.Itoa(row.PostType), "title": row.Content, "content": row.Content,
		"thumb": thumb, "video_thumb": thumb, "href": href, "video_url": href,
		"voice": voice, "length": "0", "likes": strconv.FormatInt(row.LikeCount, 10),
		"comments": strconv.FormatInt(row.CommentCount, 10), "islike": strconv.Itoa(row.IsLiked),
		"datetime": row.CreatedAt.Format("2006-01-02 15:04"), "addtime": row.CreatedAt.Unix(),
		"userinfo": user,
	}
}

func (s *Server) loadCompatPost(ctx context.Context, viewerID, postID int64) (map[string]any, error) {
	var row compatPostRow
	err := s.db.QueryRowContext(ctx, `
		SELECT post.id,post.user_id,post.post_type,post.content,post.like_count,post.comment_count,
		       post.created_at,COALESCE(NULLIF(user.nickname,''),user.username),
		       COALESCE(avatar.object_key,''),
		       COALESCE((
		         SELECT GROUP_CONCAT(asset.object_key ORDER BY media.sort_order SEPARATOR ';')
		         FROM social_post_media media
		         JOIN media_assets asset ON asset.id=media.asset_id AND asset.status=1
		         WHERE media.post_id=post.id
		       ),''),
		       EXISTS(
		         SELECT 1 FROM social_reactions reaction
		         WHERE reaction.target_type=1 AND reaction.target_id=post.id AND reaction.user_id=?
		       )
		FROM social_posts post
		JOIN users user ON user.id=post.user_id AND user.status=1
		LEFT JOIN media_assets avatar ON avatar.id=user.avatar_asset_id AND avatar.status=1
		WHERE post.id=? AND post.status=1`,
		viewerID, postID,
	).Scan(
		&row.ID, &row.UserID, &row.PostType, &row.Content, &row.LikeCount, &row.CommentCount,
		&row.CreatedAt, &row.Nickname, &row.AvatarKey, &row.MediaKeys, &row.IsLiked,
	)
	if err != nil {
		return nil, err
	}
	return s.compatPostItem(row), nil
}

func (s *Server) compatPostList(w http.ResponseWriter, r *http.Request, mode string, targetUserID int64) {
	viewerID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	if mode == "user" && targetUserID < 1 {
		targetUserID = viewerID
	}
	items, err := s.queryCompatPosts(r.Context(), viewerID, mode, targetUserID, r.FormValue("p"), false)
	if err != nil {
		s.logger.Error("list social posts", "error", err)
		writeCompat(w, 500, "动态列表加载失败", nil)
		return
	}
	values := make([]any, len(items))
	for index := range items {
		values[index] = items[index]
	}
	writeCompatList(w, values)
}

func (s *Server) compatGetPost(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	item, err := s.loadCompatPost(r.Context(), viewerID, compatInt64(r.FormValue("dynamicid")))
	if errors.Is(err, sql.ErrNoRows) {
		writeCompat(w, 404, "动态不存在", nil)
		return
	}
	if err != nil {
		writeCompat(w, 500, "动态加载失败", nil)
		return
	}
	writeCompat(w, 0, "", item)
}

func (s *Server) compatCreatePost(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	postType := compatInt(r.FormValue("type"))
	content := boundedCompat(r.FormValue("title"), 5000)
	if postType < 0 || postType > 3 || (content == "" && postType == 0) {
		writeCompat(w, 400, "动态内容无效", nil)
		return
	}
	mediaValues := make([]string, 0, 9)
	switch postType {
	case 1:
		raw := strings.NewReplacer(",", ";", "|", ";").Replace(r.FormValue("thumb"))
		mediaValues = append(mediaValues, strings.Split(raw, ";")...)
	case 2:
		mediaValues = append(mediaValues, r.FormValue("video_thumb"), r.FormValue("href"))
	case 3:
		mediaValues = append(mediaValues, r.FormValue("voice"))
	}
	assetIDs := make([]int64, 0, len(mediaValues))
	for _, value := range mediaValues {
		if strings.TrimSpace(value) == "" {
			continue
		}
		assetID, err := s.assetIDForValue(r.Context(), userID, value)
		if err != nil {
			writeCompat(w, 400, "动态媒体文件无效", nil)
			return
		}
		assetIDs = append(assetIDs, assetID)
	}
	if postType != 0 && len(assetIDs) == 0 {
		writeCompat(w, 400, "请先上传动态媒体文件", nil)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeCompat(w, 500, "发布动态失败", nil)
		return
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO social_posts(user_id,post_type,content,visibility,status)
		VALUES(?,?,?,1,1)`, userID, postType, content)
	if err != nil {
		writeCompat(w, 500, "发布动态失败", nil)
		return
	}
	postID, _ := result.LastInsertId()
	for index, assetID := range assetIDs {
		if _, err = tx.ExecContext(r.Context(), `
			INSERT INTO social_post_media(post_id,asset_id,sort_order) VALUES(?,?,?)`,
			postID, assetID, index,
		); err != nil {
			writeCompat(w, 500, "保存动态媒体失败", nil)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		writeCompat(w, 500, "发布动态失败", nil)
		return
	}
	item, err := s.loadCompatPost(r.Context(), userID, postID)
	if err != nil {
		writeCompat(w, 500, "发布动态失败", nil)
		return
	}
	writeCompat(w, 0, "发布成功", item)
}

func (s *Server) compatDeletePost(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	result, err := s.db.ExecContext(r.Context(), `
		UPDATE social_posts SET status=3,deleted_at=CURRENT_TIMESTAMP(3)
		WHERE id=? AND user_id=? AND status=1`,
		compatInt64(r.FormValue("dynamicid")), userID,
	)
	affected, _ := result.RowsAffected()
	if err != nil || affected != 1 {
		writeCompat(w, 404, "动态不存在或无权删除", nil)
		return
	}
	writeCompat(w, 0, "已删除", map[string]string{"deleted": "1"})
}

func (s *Server) compatReportPost(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	postID := compatInt64(r.FormValue("dynamicid"))
	if postID < 1 {
		writeCompat(w, 400, "举报对象无效", nil)
		return
	}
	_, err := s.db.ExecContext(r.Context(), `
		INSERT INTO user_reports(reporter_user_id,target_type,target_id,reason_code,description,evidence,status)
		VALUES(?,'social_post',?,'user_report',?,JSON_OBJECT(),0)`,
		userID, strconv.FormatInt(postID, 10), boundedCompat(r.FormValue("content"), 1000),
	)
	if err != nil {
		writeCompat(w, 500, "提交举报失败", nil)
		return
	}
	writeCompat(w, 0, "举报已提交", map[string]string{"reported": "1"})
}

func (s *Server) compatTogglePostLike(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	postID := compatInt64(r.FormValue("dynamicid"))
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeCompat(w, 500, "点赞失败", nil)
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var ownerID int64
	if err = tx.QueryRowContext(r.Context(), `
		SELECT user_id FROM social_posts WHERE id=? AND status=1 FOR UPDATE`, postID,
	).Scan(&ownerID); err != nil {
		writeCompat(w, 404, "动态不存在", nil)
		return
	}
	var liked int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT EXISTS(
			SELECT 1 FROM social_reactions WHERE target_type=1 AND target_id=? AND user_id=?
		)`, postID, userID,
	).Scan(&liked); err != nil {
		writeCompat(w, 500, "点赞失败", nil)
		return
	}
	if liked == 1 {
		_, err = tx.ExecContext(r.Context(), `
			DELETE FROM social_reactions WHERE target_type=1 AND target_id=? AND user_id=?`,
			postID, userID)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `
				UPDATE social_posts SET like_count=GREATEST(like_count-1,0) WHERE id=?`, postID)
		}
		liked = 0
	} else {
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO social_reactions(target_type,target_id,user_id,reaction) VALUES(1,?,?,1)`,
			postID, userID)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `
				UPDATE social_posts SET like_count=like_count+1 WHERE id=?`, postID)
		}
		if err == nil && ownerID != userID {
			_, err = tx.ExecContext(r.Context(), `
				INSERT INTO notifications(user_id,notification_type,actor_user_id,title,content,payload)
				VALUES(?,'like',?,'动态获赞','有用户赞了你的动态',JSON_OBJECT('dynamicid',?))`,
				ownerID, userID, postID)
		}
		liked = 1
	}
	var likeCount int64
	if err == nil {
		err = tx.QueryRowContext(r.Context(),
			`SELECT like_count FROM social_posts WHERE id=?`, postID).Scan(&likeCount)
	}
	if err != nil || tx.Commit() != nil {
		writeCompat(w, 500, "点赞失败", nil)
		return
	}
	writeCompat(w, 0, "", map[string]string{
		"islike": strconv.Itoa(liked), "likes": strconv.FormatInt(likeCount, 10),
	})
}

type compatCommentRow struct {
	ID            int64
	PostID        int64
	UserID        int64
	ParentID      int64
	ReplyToUserID int64
	Content       string
	LikeCount     int64
	CreatedAt     time.Time
	Nickname      string
	AvatarKey     string
	ReplyNickname string
}

func (s *Server) compatCommentItem(row compatCommentRow) map[string]any {
	user := map[string]any{
		"id": strconv.FormatInt(row.UserID, 10), "uid": strconv.FormatInt(row.UserID, 10),
		"user_nicename": row.Nickname, "user_nickname": row.Nickname,
		"avatar": s.mediaURL(row.AvatarKey), "avatar_thumb": s.mediaURL(row.AvatarKey),
	}
	targetUser := map[string]any{
		"id": strconv.FormatInt(row.ReplyToUserID, 10), "uid": strconv.FormatInt(row.ReplyToUserID, 10),
		"user_nicename": row.ReplyNickname, "user_nickname": row.ReplyNickname,
	}
	return map[string]any{
		"id": strconv.FormatInt(row.ID, 10), "commentid": strconv.FormatInt(row.ID, 10),
		"dynamicid": strconv.FormatInt(row.PostID, 10), "uid": strconv.FormatInt(row.UserID, 10),
		"touid": strconv.FormatInt(row.ReplyToUserID, 10), "content": row.Content,
		"datetime": row.CreatedAt.Format("2006-01-02 15:04"), "addtime": row.CreatedAt.Unix(),
		"likes": strconv.FormatInt(row.LikeCount, 10), "islike": "0",
		"replys": "0", "replylist": []any{}, "userinfo": user, "touserinfo": targetUser,
	}
}

func (s *Server) queryCompatComments(
	ctx context.Context, postID, parentID int64, page string,
) ([]map[string]any, error) {
	limit, offset := compatPage(page)
	query := `
		SELECT comment.id,comment.post_id,comment.user_id,comment.parent_comment_id,
		       comment.reply_to_user_id,comment.content,comment.like_count,comment.created_at,
		       COALESCE(NULLIF(user.nickname,''),user.username),COALESCE(avatar.object_key,''),
		       COALESCE(NULLIF(reply_user.nickname,''),reply_user.username,'')
		FROM social_comments comment
		JOIN users user ON user.id=comment.user_id
		LEFT JOIN media_assets avatar ON avatar.id=user.avatar_asset_id AND avatar.status=1
		LEFT JOIN users reply_user ON reply_user.id=comment.reply_to_user_id
		WHERE comment.status=1 AND comment.post_id=? AND comment.parent_comment_id=?
		ORDER BY comment.created_at ASC,comment.id ASC LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, query, postID, parentID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var row compatCommentRow
		if err = rows.Scan(
			&row.ID, &row.PostID, &row.UserID, &row.ParentID, &row.ReplyToUserID,
			&row.Content, &row.LikeCount, &row.CreatedAt, &row.Nickname, &row.AvatarKey, &row.ReplyNickname,
		); err != nil {
			return nil, err
		}
		item := s.compatCommentItem(row)
		if parentID == 0 {
			var replyCount int64
			_ = s.db.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM social_comments
				WHERE parent_comment_id=? AND status=1`, row.ID).Scan(&replyCount)
			item["replys"] = strconv.FormatInt(replyCount, 10)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) compatGetComments(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireCompatUser(w, r); !ok {
		return
	}
	postID := compatInt64(r.FormValue("dynamicid"))
	items, err := s.queryCompatComments(r.Context(), postID, 0, r.FormValue("p"))
	if err != nil {
		writeCompat(w, 500, "评论加载失败", nil)
		return
	}
	var count int64
	_ = s.db.QueryRowContext(r.Context(),
		`SELECT comment_count FROM social_posts WHERE id=? AND status=1`, postID).Scan(&count)
	writeCompat(w, 0, "", map[string]any{"comments": strconv.FormatInt(count, 10), "commentlist": items})
}

func (s *Server) compatGetReplies(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireCompatUser(w, r); !ok {
		return
	}
	parentID := compatInt64(r.FormValue("commentid"))
	var postID int64
	if err := s.db.QueryRowContext(r.Context(),
		`SELECT post_id FROM social_comments WHERE id=? AND status=1`, parentID).Scan(&postID); err != nil {
		writeCompat(w, 404, "评论不存在", nil)
		return
	}
	items, err := s.queryCompatComments(r.Context(), postID, parentID, r.FormValue("p"))
	if err != nil {
		writeCompat(w, 500, "回复加载失败", nil)
		return
	}
	values := make([]any, len(items))
	for index := range items {
		values[index] = items[index]
	}
	writeCompatList(w, values)
}

func (s *Server) compatCreateComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	postID := compatInt64(r.FormValue("dynamicid"))
	parentID := compatInt64(r.FormValue("parentid"))
	if parentID < 1 {
		parentID = compatInt64(r.FormValue("commentid"))
	}
	replyToUserID := compatInt64(r.FormValue("touid"))
	content := boundedCompat(r.FormValue("content"), 2000)
	if postID < 1 || content == "" {
		writeCompat(w, 400, "评论内容不能为空", nil)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeCompat(w, 500, "发表评论失败", nil)
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var ownerID int64
	if err = tx.QueryRowContext(r.Context(),
		`SELECT user_id FROM social_posts WHERE id=? AND status=1 FOR UPDATE`, postID,
	).Scan(&ownerID); err != nil {
		writeCompat(w, 404, "动态不存在", nil)
		return
	}
	if parentID > 0 {
		var parentPostID int64
		if err = tx.QueryRowContext(r.Context(), `
			SELECT post_id FROM social_comments WHERE id=? AND status=1`, parentID,
		).Scan(&parentPostID); err != nil || parentPostID != postID {
			writeCompat(w, 400, "回复的评论无效", nil)
			return
		}
	}
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO social_comments(post_id,user_id,parent_comment_id,reply_to_user_id,content,status)
		VALUES(?,?,?,?,?,1)`, postID, userID, parentID, replyToUserID, content)
	if err == nil {
		_, err = tx.ExecContext(r.Context(),
			`UPDATE social_posts SET comment_count=comment_count+1 WHERE id=?`, postID)
	}
	notifyUserID := ownerID
	if replyToUserID > 0 {
		notifyUserID = replyToUserID
	}
	if err == nil && notifyUserID != userID {
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO notifications(user_id,notification_type,actor_user_id,title,content,payload)
			VALUES(?,'comment',?,'动态评论',?,JSON_OBJECT('dynamicid',?))`,
			notifyUserID, userID, content, postID)
	}
	if err != nil {
		writeCompat(w, 500, "发表评论失败", nil)
		return
	}
	commentID, _ := result.LastInsertId()
	if err = tx.Commit(); err != nil {
		writeCompat(w, 500, "发表评论失败", nil)
		return
	}
	items, err := s.queryCompatComments(r.Context(), postID, parentID, "1")
	if err != nil {
		writeCompat(w, 0, "评论成功", map[string]string{"commentid": strconv.FormatInt(commentID, 10)})
		return
	}
	for _, item := range items {
		if item["commentid"] == strconv.FormatInt(commentID, 10) {
			writeCompat(w, 0, "评论成功", item)
			return
		}
	}
	writeCompat(w, 0, "评论成功", map[string]string{"commentid": strconv.FormatInt(commentID, 10)})
}

func (s *Server) compatDeleteComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	commentID := compatInt64(r.FormValue("commentid"))
	postID := compatInt64(r.FormValue("dynamicid"))
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeCompat(w, 500, "删除评论失败", nil)
		return
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(r.Context(), `
		UPDATE social_comments SET status=3,deleted_at=CURRENT_TIMESTAMP(3)
		WHERE id=? AND post_id=? AND user_id=? AND status=1`, commentID, postID, userID)
	affected, _ := result.RowsAffected()
	if err != nil || affected != 1 {
		writeCompat(w, 404, "评论不存在或无权删除", nil)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE social_posts SET comment_count=GREATEST(comment_count-1,0) WHERE id=?`, postID,
	); err != nil || tx.Commit() != nil {
		writeCompat(w, 500, "删除评论失败", nil)
		return
	}
	writeCompat(w, 0, "已删除", map[string]string{"deleted": "1"})
}

func (s *Server) compatNotifications(w http.ResponseWriter, r *http.Request, service string) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	notificationType := "system"
	switch service {
	case "Message.atLists":
		notificationType = "mention"
	case "Message.praiseLists":
		notificationType = "like"
	case "Message.commentLists":
		notificationType = "comment"
	case "Message.fansLists":
		notificationType = "follow"
	}
	limit, offset := compatPage(r.FormValue("p"))
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT notification.id,notification.title,notification.content,notification.created_at,
		       notification.actor_user_id,
		       COALESCE(NULLIF(actor.nickname,''),actor.username,''),
		       COALESCE(asset.object_key,'')
		FROM notifications notification
		LEFT JOIN users actor ON actor.id=notification.actor_user_id
		LEFT JOIN media_assets asset ON asset.id=actor.avatar_asset_id AND asset.status=1
		WHERE notification.user_id=? AND notification.notification_type=?
		ORDER BY notification.created_at DESC,notification.id DESC LIMIT ? OFFSET ?`,
		userID, notificationType, limit, offset,
	)
	if err != nil {
		writeCompat(w, 500, "消息加载失败", nil)
		return
	}
	defer rows.Close()
	items := make([]any, 0, limit)
	for rows.Next() {
		var id, actorID int64
		var title, content, nickname, avatarKey string
		var createdAt time.Time
		if err = rows.Scan(&id, &title, &content, &createdAt, &actorID, &nickname, &avatarKey); err != nil {
			break
		}
		items = append(items, map[string]any{
			"id": strconv.FormatInt(id, 10), "title": title, "content": content,
			"addtime": createdAt.Unix(), "datetime": createdAt.Format("2006-01-02 15:04"),
			"uid": strconv.FormatInt(actorID, 10), "user_nicename": nickname,
			"avatar": s.mediaURL(avatarKey), "avatar_thumb": s.mediaURL(avatarKey),
		})
	}
	if err != nil || rows.Err() != nil {
		writeCompat(w, 500, "消息加载失败", nil)
		return
	}
	writeCompatList(w, items)
}

func (s *Server) compatVideoList(w http.ResponseWriter, r *http.Request, liked bool) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	mode := "user"
	if liked {
		mode = "liked"
	}
	items, err := s.queryCompatPosts(r.Context(), userID, mode, userID, r.FormValue("p"), true)
	if err != nil {
		writeCompat(w, 500, "视频加载失败", nil)
		return
	}
	values := make([]any, len(items))
	for index := range items {
		values[index] = items[index]
	}
	writeCompatList(w, values)
}

func (s *Server) compatGetVideo(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	item, err := s.loadCompatPost(r.Context(), viewerID, compatInt64(r.FormValue("videoid")))
	if err != nil || item["type"] != "2" {
		writeCompat(w, 404, "视频不存在", nil)
		return
	}
	writeCompat(w, 0, "", item)
}
