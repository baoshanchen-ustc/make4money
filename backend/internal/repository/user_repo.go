package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	dbgroup "github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/usagelog"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/userallowedgroup"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	"github.com/Wei-Shaw/sub2api/internal/pkg/oauth"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"

	entsql "entgo.io/ent/dialect/sql"
)

type userRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewUserRepository(client *dbent.Client, sqlDB *sql.DB) service.UserRepository {
	return &authIdentityUserRepository{userRepository: newUserRepositoryWithSQL(client, sqlDB)}
}

func newUserRepositoryWithSQL(client *dbent.Client, sqlq sqlExecutor) *userRepository {
	return &userRepository{client: client, sql: sqlq}
}

type authIdentityUserRepository struct {
	*userRepository
}

func (r *authIdentityUserRepository) CreatePendingAuthSession(ctx context.Context, input service.PendingAuthSessionInput) (*service.PendingAuthSessionRecord, error) {
	sessionToken, err := oauth.GenerateState()
	if err != nil {
		return nil, err
	}

	record, err := r.userRepository.CreatePendingAuthSession(ctx, CreatePendingAuthSessionInput{
		SessionToken:     sessionToken,
		Intent:           strings.TrimSpace(input.Intent),
		ProviderType:     strings.TrimSpace(input.ProviderType),
		ProviderKey:      strings.TrimSpace(input.ProviderKey),
		ProviderSubject:  strings.TrimSpace(input.ProviderSubject),
		TargetUserID:     input.TargetUserID,
		RedirectTo:       strings.TrimSpace(input.RedirectTo),
		Metadata:         cloneServiceMetadata(input.Metadata),
		UpstreamIdentity: cloneServiceMetadata(input.Metadata),
		ExpiresAt:        time.Now().Add(10 * time.Minute).UTC(),
	})
	if err != nil {
		return nil, err
	}
	return pendingAuthRecordToService(record), nil
}

func (r *authIdentityUserRepository) GetPendingAuthSessionByID(ctx context.Context, sessionID string) (*service.PendingAuthSessionRecord, error) {
	record, err := r.userRepository.GetPendingAuthSessionByToken(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	return pendingAuthRecordToService(record), nil
}

func (r *authIdentityUserRepository) UpdatePendingAuthSession(ctx context.Context, session *service.PendingAuthSessionRecord) error {
	if session == nil {
		return fmt.Errorf("pending auth session is required")
	}
	sessionToken := strings.TrimSpace(session.ID)
	if sessionToken == "" {
		sessionToken = strings.TrimSpace(session.Token)
	}
	if sessionToken == "" {
		return fmt.Errorf("pending auth session id is required")
	}

	return r.userRepository.UpdatePendingAuthSessionByToken(ctx, &PendingAuthSessionRecord{
		SessionToken:        sessionToken,
		Intent:              strings.TrimSpace(session.Intent),
		ProviderType:        strings.TrimSpace(session.ProviderType),
		ProviderKey:         strings.TrimSpace(session.ProviderKey),
		ProviderSubject:     strings.TrimSpace(session.ProviderSubject),
		TargetUserID:        session.TargetUserID,
		RedirectTo:          strings.TrimSpace(session.RedirectTo),
		Metadata:            cloneServiceMetadata(session.Metadata),
		ResolvedEmail:       strings.TrimSpace(session.ResolvedEmail),
		PendingPasswordHash: strings.TrimSpace(session.PendingPasswordHash),
		UpstreamIdentity:    cloneServiceMetadata(session.Metadata),
		EmailVerifiedAt:     session.EmailVerifiedAt,
		PasswordVerifiedAt:  session.PasswordVerifiedAt,
		TOTPVerifiedAt:      session.TOTPVerifiedAt,
		ExpiresAt:           session.ExpiresAt,
		ConsumedAt:          session.ConsumedAt,
	})
}

func pendingAuthRecordToService(record *PendingAuthSessionRecord) *service.PendingAuthSessionRecord {
	if record == nil {
		return nil
	}

	return &service.PendingAuthSessionRecord{
		ID:                  strings.TrimSpace(record.SessionToken),
		Token:               strings.TrimSpace(record.SessionToken),
		Intent:              strings.TrimSpace(record.Intent),
		ProviderType:        strings.TrimSpace(record.ProviderType),
		ProviderKey:         strings.TrimSpace(record.ProviderKey),
		ProviderSubject:     strings.TrimSpace(record.ProviderSubject),
		TargetUserID:        record.TargetUserID,
		ResolvedEmail:       strings.TrimSpace(record.ResolvedEmail),
		PendingPasswordHash: strings.TrimSpace(record.PendingPasswordHash),
		Metadata:            cloneServiceMetadata(record.Metadata),
		EmailVerifiedAt:     record.EmailVerifiedAt,
		PasswordVerifiedAt:  record.PasswordVerifiedAt,
		TOTPVerifiedAt:      record.TOTPVerifiedAt,
		ExpiresAt:           record.ExpiresAt,
		ConsumedAt:          record.ConsumedAt,
		RedirectTo:          strings.TrimSpace(record.RedirectTo),
		CreatedAt:           record.CreatedAt,
		UpdatedAt:           record.UpdatedAt,
	}
}

func cloneServiceMetadata(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (r *userRepository) Create(ctx context.Context, userIn *service.User) error {
	if userIn == nil {
		return nil
	}
	userIn.SignupSource = service.NormalizeSignupSource(userIn.SignupSource)

	// 统一使用 ent 的事务：保证用户与允许分组的更新原子化，
	// 并避免基于 *sql.Tx 手动构造 ent client 导致的 ExecQuerier 断言错误。
	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}

	var txClient *dbent.Client
	if err == nil {
		defer func() { _ = tx.Rollback() }()
		txClient = tx.Client()
	} else {
		// 已处于外部事务中（ErrTxStarted），复用当前 client 并由调用方负责提交/回滚。
		txClient = r.client
	}

	created, err := txClient.User.Create().
		SetEmail(userIn.Email).
		SetUsername(userIn.Username).
		SetNotes(userIn.Notes).
		SetPasswordHash(userIn.PasswordHash).
		SetRole(userIn.Role).
		SetBalance(userIn.Balance).
		SetConcurrency(userIn.Concurrency).
		SetStatus(userIn.Status).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, service.ErrEmailExists)
	}

	if err := r.syncUserAllowedGroupsWithClient(ctx, txClient, created.ID, userIn.AllowedGroups); err != nil {
		return err
	}

	if tx != nil {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	applyUserEntityToService(userIn, created)
	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (*service.User, error) {
	m, err := r.client.User.Query().Where(dbuser.IDEQ(id)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrUserNotFound, nil)
	}

	out := userEntityToService(m)
	groups, err := r.loadAllowedGroups(ctx, []int64{id})
	if err != nil {
		return nil, err
	}
	if v, ok := groups[id]; ok {
		out.AllowedGroups = v
	}
	r.attachAvatar(ctx, out)
	return out, nil
}

func (r *userRepository) GetByIDWithActivity(ctx context.Context, id int64) (*service.User, error) {
	out, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	timestamps, err := r.loadUserActivityTimestamps(ctx, []int64{id})
	if err != nil {
		return nil, err
	}
	if activity, ok := timestamps[id]; ok {
		out.LastLoginAt = activity.LastLoginAt
		out.LastUsedAt = activity.LastUsedAt
	}
	return out, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*service.User, error) {
	m, err := r.client.User.Query().Where(dbuser.EmailEQ(email)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrUserNotFound, nil)
	}

	out := userEntityToService(m)
	groups, err := r.loadAllowedGroups(ctx, []int64{m.ID})
	if err != nil {
		return nil, err
	}
	if v, ok := groups[m.ID]; ok {
		out.AllowedGroups = v
	}
	r.attachAvatar(ctx, out)
	return out, nil
}

func (r *userRepository) attachAvatar(ctx context.Context, user *service.User) {
	if user == nil || user.ID == 0 {
		return
	}
	avatar, err := r.GetAvatar(ctx, user.ID)
	if err != nil {
		return
	}
	user.Avatar = avatar
	user.AvatarURL = service.ResolvePreferredUserAvatarURL(avatar)
	user.HasCustomAvatar = user.AvatarURL != ""
	if avatar != nil {
		user.AvatarUpdatedAt = &avatar.UpdatedAt
	}
}

func (r *userRepository) Update(ctx context.Context, userIn *service.User) error {
	if userIn == nil {
		return nil
	}
	userIn.SignupSource = service.NormalizeSignupSource(userIn.SignupSource)

	// 使用 ent 事务包裹用户更新与 allowed_groups 同步，避免跨层事务不一致。
	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}

	var txClient *dbent.Client
	if err == nil {
		defer func() { _ = tx.Rollback() }()
		txClient = tx.Client()
	} else {
		// 已处于外部事务中（ErrTxStarted），复用当前 client 并由调用方负责提交/回滚。
		txClient = r.client
	}

	updateOp := txClient.User.UpdateOneID(userIn.ID).
		SetEmail(userIn.Email).
		SetUsername(userIn.Username).
		SetNotes(userIn.Notes).
		SetPasswordHash(userIn.PasswordHash).
		SetRole(userIn.Role).
		SetBalance(userIn.Balance).
		SetConcurrency(userIn.Concurrency).
		SetStatus(userIn.Status).
		SetBalanceNotifyEnabled(userIn.BalanceNotifyEnabled).
		SetBalanceNotifyThresholdType(userIn.BalanceNotifyThresholdType).
		SetNillableBalanceNotifyThreshold(userIn.BalanceNotifyThreshold).
		SetBalanceNotifyExtraEmails(marshalExtraEmails(userIn.BalanceNotifyExtraEmails)).
		SetTotalRecharged(userIn.TotalRecharged)
	if userIn.BalanceNotifyThreshold == nil {
		updateOp = updateOp.ClearBalanceNotifyThreshold()
	}
	updated, err := updateOp.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrUserNotFound, service.ErrEmailExists)
	}

	if err := r.syncUserAllowedGroupsWithClient(ctx, txClient, updated.ID, userIn.AllowedGroups); err != nil {
		return err
	}

	if tx != nil {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	userIn.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *userRepository) Delete(ctx context.Context, id int64) error {
	affected, err := r.client.User.Delete().Where(dbuser.IDEQ(id)).Exec(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	if affected == 0 {
		return service.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) List(ctx context.Context, params pagination.PaginationParams) ([]service.User, *pagination.PaginationResult, error) {
	return r.ListWithFilters(ctx, params, service.UserListFilters{})
}

func (r *userRepository) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters service.UserListFilters) ([]service.User, *pagination.PaginationResult, error) {
	q := r.client.User.Query()

	if filters.Status != "" {
		q = q.Where(dbuser.StatusEQ(filters.Status))
	}
	if filters.Role != "" {
		q = q.Where(dbuser.RoleEQ(filters.Role))
	}
	if filters.Search != "" {
		q = q.Where(
			dbuser.Or(
				dbuser.EmailContainsFold(filters.Search),
				dbuser.UsernameContainsFold(filters.Search),
				dbuser.NotesContainsFold(filters.Search),
				dbuser.HasAPIKeysWith(apikey.KeyContainsFold(filters.Search)),
			),
		)
	}

	if filters.GroupName != "" {
		q = q.Where(dbuser.HasAllowedGroupsWith(
			dbgroup.NameContainsFold(filters.GroupName),
		))
	}

	// If attribute filters are specified, we need to filter by user IDs first
	var allowedUserIDs []int64
	if len(filters.Attributes) > 0 {
		var attrErr error
		allowedUserIDs, attrErr = r.filterUsersByAttributes(ctx, filters.Attributes)
		if attrErr != nil {
			return nil, nil, attrErr
		}
		if len(allowedUserIDs) == 0 {
			// No users match the attribute filters
			return []service.User{}, paginationResultFromTotal(0, params), nil
		}
		q = q.Where(dbuser.IDIn(allowedUserIDs...))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	usersQuery := q.
		Offset(params.Offset()).
		Limit(params.Limit())
	for _, order := range userListOrder(params) {
		usersQuery = usersQuery.Order(order)
	}

	users, err := usersQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}

	outUsers := make([]service.User, 0, len(users))
	if len(users) == 0 {
		return outUsers, paginationResultFromTotal(int64(total), params), nil
	}

	userIDs := make([]int64, 0, len(users))
	userMap := make(map[int64]*service.User, len(users))
	for i := range users {
		userIDs = append(userIDs, users[i].ID)
		u := userEntityToService(users[i])
		outUsers = append(outUsers, *u)
		userMap[u.ID] = &outUsers[len(outUsers)-1]
	}

	shouldLoadSubscriptions := filters.IncludeSubscriptions == nil || *filters.IncludeSubscriptions
	if shouldLoadSubscriptions {
		// Batch load active subscriptions with groups to avoid N+1.
		subs, err := r.client.UserSubscription.Query().
			Where(
				usersubscription.UserIDIn(userIDs...),
				usersubscription.StatusEQ(service.SubscriptionStatusActive),
			).
			WithGroup().
			All(ctx)
		if err != nil {
			return nil, nil, err
		}

		for i := range subs {
			if u, ok := userMap[subs[i].UserID]; ok {
				u.Subscriptions = append(u.Subscriptions, *userSubscriptionEntityToService(subs[i]))
			}
		}
	}

	allowedGroupsByUser, err := r.loadAllowedGroups(ctx, userIDs)
	if err != nil {
		return nil, nil, err
	}
	for id, u := range userMap {
		if groups, ok := allowedGroupsByUser[id]; ok {
			u.AllowedGroups = groups
		}
	}

	activityByUser, err := r.loadUserActivityTimestamps(ctx, userIDs)
	if err != nil {
		return nil, nil, err
	}
	for id, u := range userMap {
		if activity, ok := activityByUser[id]; ok {
			u.LastLoginAt = activity.LastLoginAt
			u.LastUsedAt = activity.LastUsedAt
		}
	}

	return outUsers, paginationResultFromTotal(int64(total), params), nil
}

type userActivityTimestamps struct {
	LastLoginAt *time.Time
	LastUsedAt  *time.Time
}

func (r *userRepository) loadUserActivityTimestamps(ctx context.Context, userIDs []int64) (map[int64]userActivityTimestamps, error) {
	result := make(map[int64]userActivityTimestamps, len(userIDs))
	if len(userIDs) == 0 || r == nil || r.client == nil || r.sql == nil {
		return result, nil
	}

	idArgs := make([]driver.Value, 0, len(userIDs))
	for _, id := range userIDs {
		idArgs = append(idArgs, id)
	}
	dialect := r.client.Driver().Dialect()
	builder := entsql.Dialect(dialect)

	loginQuery, loginArgs := builder.
		Select("id", "last_login_at").
		From(builder.Table("users")).
		Where(entsql.InValues("id", idArgs...)).
		Query()
	rows, err := r.sql.QueryContext(ctx, loginQuery, loginArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			userID      int64
			lastLoginAt sql.NullTime
		)
		if err := rows.Scan(&userID, &lastLoginAt); err != nil {
			return nil, err
		}
		activity := result[userID]
		if lastLoginAt.Valid {
			ts := lastLoginAt.Time
			activity.LastLoginAt = &ts
		}
		result[userID] = activity
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	usageQuery, usageArgs := builder.
		Select(usagelog.FieldUserID, "MAX("+usagelog.FieldCreatedAt+") AS last_used_at").
		From(builder.Table(usagelog.Table)).
		Where(entsql.InValues(usagelog.FieldUserID, idArgs...)).
		GroupBy(usagelog.FieldUserID).
		Query()
	usageRows, err := r.sql.QueryContext(ctx, usageQuery, usageArgs...)
	if err != nil {
		return nil, err
	}
	defer usageRows.Close()

	for usageRows.Next() {
		var (
			userID     int64
			lastUsedAt sql.NullTime
		)
		if err := usageRows.Scan(&userID, &lastUsedAt); err != nil {
			return nil, err
		}
		if !lastUsedAt.Valid {
			continue
		}
		activity := result[userID]
		ts := lastUsedAt.Time
		activity.LastUsedAt = &ts
		result[userID] = activity
	}
	if err := usageRows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func userListOrder(params pagination.PaginationParams) []func(*entsql.Selector) {
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	sortOrder := params.NormalizedSortOrder(pagination.SortOrderDesc)

	switch sortBy {
	case "last_login_at":
		return userTimestampOrder("last_login_at", sortOrder)
	case "last_used_at":
		return userLastUsedAtOrder(sortOrder)
	}

	var field string
	defaultField := true
	switch sortBy {
	case "email":
		field = dbuser.FieldEmail
		defaultField = false
	case "username":
		field = dbuser.FieldUsername
		defaultField = false
	case "role":
		field = dbuser.FieldRole
		defaultField = false
	case "balance":
		field = dbuser.FieldBalance
		defaultField = false
	case "concurrency":
		field = dbuser.FieldConcurrency
		defaultField = false
	case "status":
		field = dbuser.FieldStatus
		defaultField = false
	case "created_at":
		field = dbuser.FieldCreatedAt
		defaultField = false
	default:
		field = dbuser.FieldID
	}

	if sortOrder == pagination.SortOrderAsc {
		if defaultField && field == dbuser.FieldID {
			return []func(*entsql.Selector){dbent.Asc(dbuser.FieldID)}
		}
		return []func(*entsql.Selector){dbent.Asc(field), dbent.Asc(dbuser.FieldID)}
	}
	if defaultField && field == dbuser.FieldID {
		return []func(*entsql.Selector){dbent.Desc(dbuser.FieldID)}
	}
	return []func(*entsql.Selector){dbent.Desc(field), dbent.Desc(dbuser.FieldID)}
}

func userTimestampOrder(column string, sortOrder string) []func(*entsql.Selector) {
	direction := "DESC"
	tie := dbent.Desc(dbuser.FieldID)
	if sortOrder == pagination.SortOrderAsc {
		direction = "ASC"
		tie = dbent.Asc(dbuser.FieldID)
	}
	return []func(*entsql.Selector){
		func(s *entsql.Selector) {
			s.OrderExpr(entsql.Raw(s.C(column) + " " + direction + " NULLS LAST"))
		},
		tie,
	}
}

func userLastUsedAtOrder(sortOrder string) []func(*entsql.Selector) {
	const usageAlias = "user_last_used_activity"
	direction := "DESC"
	tie := dbent.Desc(dbuser.FieldID)
	if sortOrder == pagination.SortOrderAsc {
		direction = "ASC"
		tie = dbent.Asc(dbuser.FieldID)
	}

	return []func(*entsql.Selector){
		func(s *entsql.Selector) {
			usageSummary := entsql.
				Select(usagelog.FieldUserID).
				AppendSelectExprAs(entsql.Raw("MAX("+usagelog.FieldCreatedAt+")"), "last_used_at").
				From(entsql.Table(usagelog.Table)).
				GroupBy(usagelog.FieldUserID).
				As(usageAlias)
			s.LeftJoin(usageSummary).On(s.C(dbuser.FieldID), usageSummary.C(usagelog.FieldUserID))
			s.OrderExpr(entsql.Raw(usageAlias + ".last_used_at " + direction + " NULLS LAST"))
		},
		tie,
	}
}

// filterUsersByAttributes returns user IDs that match ALL the given attribute filters
func (r *userRepository) filterUsersByAttributes(ctx context.Context, attrs map[int64]string) ([]int64, error) {
	if len(attrs) == 0 {
		return nil, nil
	}

	if r.sql == nil {
		return nil, fmt.Errorf("sql executor is not configured")
	}

	clauses := make([]string, 0, len(attrs))
	args := make([]any, 0, len(attrs)*2+1)
	argIndex := 1
	for attrID, value := range attrs {
		clauses = append(clauses, fmt.Sprintf("(attribute_id = $%d AND value ILIKE $%d)", argIndex, argIndex+1))
		args = append(args, attrID, "%"+value+"%")
		argIndex += 2
	}

	query := fmt.Sprintf(
		`SELECT user_id
		 FROM user_attribute_values
		 WHERE %s
		 GROUP BY user_id
		 HAVING COUNT(DISTINCT attribute_id) = $%d`,
		strings.Join(clauses, " OR "),
		argIndex,
	)
	args = append(args, len(attrs))

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]int64, 0)
	for rows.Next() {
		var userID int64
		if scanErr := rows.Scan(&userID); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, userID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *userRepository) UpdateBalance(ctx context.Context, id int64, amount float64) error {
	client := clientFromContext(ctx, r.client)
	update := client.User.Update().Where(dbuser.IDEQ(id)).AddBalance(amount)
	// Track cumulative recharge amount for percentage-based notifications
	if amount > 0 && service.ShouldTrackTotalRecharged(ctx) {
		update = update.AddTotalRecharged(amount)
	}
	n, err := update.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	if n == 0 {
		return service.ErrUserNotFound
	}
	return nil
}

// DeductBalance 扣除用户余额
// 透支策略：允许余额变为负数，确保当前请求能够完成
// 中间件会阻止余额 <= 0 的用户发起后续请求
func (r *userRepository) DeductBalance(ctx context.Context, id int64, amount float64) error {
	client := clientFromContext(ctx, r.client)
	n, err := client.User.Update().
		Where(dbuser.IDEQ(id)).
		AddBalance(-amount).
		Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) UpdateConcurrency(ctx context.Context, id int64, amount int) error {
	client := clientFromContext(ctx, r.client)
	n, err := client.User.Update().Where(dbuser.IDEQ(id)).AddConcurrency(amount).Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	if n == 0 {
		return service.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	return r.client.User.Query().Where(dbuser.EmailEQ(email)).Exist(ctx)
}

func (r *userRepository) AddGroupToAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	client := clientFromContext(ctx, r.client)
	return client.UserAllowedGroup.Create().
		SetUserID(userID).
		SetGroupID(groupID).
		OnConflictColumns(userallowedgroup.FieldUserID, userallowedgroup.FieldGroupID).
		DoNothing().
		Exec(ctx)
}

func (r *userRepository) RemoveGroupFromAllowedGroups(ctx context.Context, groupID int64) (int64, error) {
	// 仅操作 user_allowed_groups 联接表，legacy users.allowed_groups 列已弃用。
	affected, err := r.client.UserAllowedGroup.Delete().
		Where(userallowedgroup.GroupIDEQ(groupID)).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	return int64(affected), nil
}

// RemoveGroupFromUserAllowedGroups 移除单个用户的指定分组权限
func (r *userRepository) RemoveGroupFromUserAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.UserAllowedGroup.Delete().
		Where(userallowedgroup.UserIDEQ(userID), userallowedgroup.GroupIDEQ(groupID)).
		Exec(ctx)
	return err
}

func (r *userRepository) GetFirstAdmin(ctx context.Context) (*service.User, error) {
	m, err := r.client.User.Query().
		Where(
			dbuser.RoleEQ(service.RoleAdmin),
			dbuser.StatusEQ(service.StatusActive),
		).
		Order(dbent.Asc(dbuser.FieldID)).
		First(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrUserNotFound, nil)
	}

	out := userEntityToService(m)
	groups, err := r.loadAllowedGroups(ctx, []int64{m.ID})
	if err != nil {
		return nil, err
	}
	if v, ok := groups[m.ID]; ok {
		out.AllowedGroups = v
	}
	return out, nil
}

func (r *userRepository) loadAllowedGroups(ctx context.Context, userIDs []int64) (map[int64][]int64, error) {
	out := make(map[int64][]int64, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}

	rows, err := r.client.UserAllowedGroup.Query().
		Where(userallowedgroup.UserIDIn(userIDs...)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	for i := range rows {
		out[rows[i].UserID] = append(out[rows[i].UserID], rows[i].GroupID)
	}

	for userID := range out {
		sort.Slice(out[userID], func(i, j int) bool { return out[userID][i] < out[userID][j] })
	}

	return out, nil
}

// syncUserAllowedGroupsWithClient 在 ent client/事务内同步用户允许分组：
// 仅操作 user_allowed_groups 联接表，legacy users.allowed_groups 列已弃用。
func (r *userRepository) syncUserAllowedGroupsWithClient(ctx context.Context, client *dbent.Client, userID int64, groupIDs []int64) error {
	if client == nil {
		return nil
	}

	// Keep join table as the source of truth for reads.
	if _, err := client.UserAllowedGroup.Delete().Where(userallowedgroup.UserIDEQ(userID)).Exec(ctx); err != nil {
		return err
	}

	unique := make(map[int64]struct{}, len(groupIDs))
	for _, id := range groupIDs {
		if id <= 0 {
			continue
		}
		unique[id] = struct{}{}
	}

	if len(unique) > 0 {
		creates := make([]*dbent.UserAllowedGroupCreate, 0, len(unique))
		for groupID := range unique {
			creates = append(creates, client.UserAllowedGroup.Create().SetUserID(userID).SetGroupID(groupID))
		}
		if err := client.UserAllowedGroup.
			CreateBulk(creates...).
			OnConflictColumns(userallowedgroup.FieldUserID, userallowedgroup.FieldGroupID).
			DoNothing().
			Exec(ctx); err != nil {
			return err
		}
	}

	return nil
}

func applyUserEntityToService(dst *service.User, src *dbent.User) {
	if dst == nil || src == nil {
		return
	}
	dst.ID = src.ID
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}

// marshalExtraEmails serializes notify email entries to JSON for storage.
func marshalExtraEmails(entries []service.NotifyEmailEntry) string {
	return service.MarshalNotifyEmails(entries)
}

// UpdateSignupSource is a compatibility hook for the auth identity migration.
// The concrete users.signup_source column is added by a separate schema/migration
// worker; until that lands in this worktree, we only validate the target user
// exists and normalize the requested value so auth flow changes can compile
// against a stable repository contract.
func (r *userRepository) UpdateSignupSource(ctx context.Context, userID int64, signupSource string) error {
	if userID <= 0 {
		return service.ErrUserNotFound
	}
	if _, err := r.client.User.Query().Where(dbuser.IDEQ(userID)).Only(ctx); err != nil {
		return translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	_ = service.NormalizeSignupSource(signupSource)
	return nil
}

// UpdateTotpSecret 更新用户的 TOTP 加密密钥
func (r *userRepository) UpdateTotpSecret(ctx context.Context, userID int64, encryptedSecret *string) error {
	client := clientFromContext(ctx, r.client)
	update := client.User.UpdateOneID(userID)
	if encryptedSecret == nil {
		update = update.ClearTotpSecretEncrypted()
	} else {
		update = update.SetTotpSecretEncrypted(*encryptedSecret)
	}
	_, err := update.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	return nil
}

// EnableTotp 启用用户的 TOTP 双因素认证
func (r *userRepository) EnableTotp(ctx context.Context, userID int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.User.UpdateOneID(userID).
		SetTotpEnabled(true).
		SetTotpEnabledAt(time.Now()).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	return nil
}

// DisableTotp 禁用用户的 TOTP 双因素认证
func (r *userRepository) DisableTotp(ctx context.Context, userID int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.User.UpdateOneID(userID).
		SetTotpEnabled(false).
		ClearTotpEnabledAt().
		ClearTotpSecretEncrypted().
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	return nil
}
