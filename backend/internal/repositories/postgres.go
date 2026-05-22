package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"crowdrise/backend/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type AuthUser struct {
	domain.User
	PasswordHash string
}

type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ProjectInput struct {
	Title            string
	ShortDescription string
	Description      string
	CategoryID       *int
	CampaignType     string
	Currency         string
	GoalAmount       float64
	StartAt          *time.Time
	EndAt            *time.Time
}

type MilestoneInput struct {
	Title       string
	Description string
	DueAt       time.Time
	AmountLimit float64
	Position    int
}

type RewardInput struct {
	Title            string
	Description      string
	MinAmount        float64
	LimitCount       *int
	DeliveryEstimate *time.Time
}

type PaymentRecord struct {
	ID                string
	PledgeID          string
	ProjectID         string
	BackerID          string
	ProviderPaymentID string
	Status            string
	Amount            float64
}

type OutboxItem struct {
	ID        string
	UserID    string
	EventType string
	Payload   []byte
}

func (r *Repository) CreateUser(ctx context.Context, email, passwordHash, displayName string, phone *string) (domain.User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback(ctx)

	id := uuid.NewString()
	_, err = tx.Exec(ctx, `insert into users (id, email, password_hash, display_name, phone) values ($1,$2,$3,$4,$5)`, id, email, passwordHash, displayName, phone)
	if err != nil {
		return domain.User{}, err
	}
	_, err = tx.Exec(ctx, `
		insert into user_roles (user_id, role_id)
		select $1, id from roles where code in ('backer','author')`, id)
	if err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	return r.GetUserByID(ctx, id)
}

func (r *Repository) GetAuthUserByEmail(ctx context.Context, email string) (AuthUser, error) {
	var user AuthUser
	err := r.db.QueryRow(ctx, `
		select id::text, email, password_hash, display_name, phone, is_blocked
		from users where lower(email) = lower($1)`, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Phone, &user.IsBlocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return AuthUser{}, domain.ErrNotFound
	}
	if err != nil {
		return AuthUser{}, err
	}
	roles, err := r.userRoles(ctx, user.ID)
	if err != nil {
		return AuthUser{}, err
	}
	user.Roles = roles
	return user, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	var user domain.User
	err := r.db.QueryRow(ctx, `
		select id::text, email, display_name, phone, is_blocked
		from users where id = $1`, id).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.Phone, &user.IsBlocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	user.Roles, err = r.userRoles(ctx, id)
	return user, err
}

func (r *Repository) userRoles(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		select r.code from roles r
		join user_roles ur on ur.role_id = r.id
		where ur.user_id = $1 order by r.code`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *Repository) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.db.Query(ctx, `select id, name from categories order by name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Category
	for rows.Next() {
		var item Category
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) ListProjects(ctx context.Context, status, campaignType string, page, pageSize int) ([]domain.Project, int, error) {
	if status == "" {
		status = domain.ProjectPublished
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	where := "where p.status = $1"
	args := []any{status}
	if campaignType != "" {
		where += " and p.campaign_type = $2"
		args = append(args, campaignType)
	}
	var total int
	countSQL := fmt.Sprintf("select count(*) from projects p %s", where)
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		select p.id::text, p.author_id::text, p.title, p.short_description, p.description, p.category_id,
		       c.name, p.campaign_type, p.currency, p.goal_amount, p.status,
		       coalesce(f.total_collected,0), coalesce(f.total_refunded,0), coalesce(f.total_available,0), coalesce(f.total_reserved,0)
		from projects p
		left join categories c on c.id = p.category_id
		left join project_funds f on f.project_id = p.id
		%s
		order by p.created_at desc
		limit $%d offset $%d`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := scanProjects(rows)
	return items, total, err
}

func (r *Repository) GetProjectDetails(ctx context.Context, projectID string) (domain.ProjectDetails, error) {
	project, err := r.getProject(ctx, projectID)
	if err != nil {
		return domain.ProjectDetails{}, err
	}
	milestones, err := r.ListMilestones(ctx, projectID)
	if err != nil {
		return domain.ProjectDetails{}, err
	}
	rewards, err := r.ListRewards(ctx, projectID)
	if err != nil {
		return domain.ProjectDetails{}, err
	}
	updates, err := r.listUpdates(ctx, projectID)
	if err != nil {
		return domain.ProjectDetails{}, err
	}
	media, err := r.listMedia(ctx, projectID)
	if err != nil {
		return domain.ProjectDetails{}, err
	}
	broadcasts, err := r.ListBroadcasts(ctx, projectID)
	if err != nil {
		return domain.ProjectDetails{}, err
	}
	return domain.ProjectDetails{Project: project, Milestones: milestones, Rewards: rewards, Updates: updates, Media: media, Broadcasts: broadcasts}, nil
}

func (r *Repository) getProject(ctx context.Context, projectID string) (domain.Project, error) {
	rows, err := r.db.Query(ctx, `
		select p.id::text, p.author_id::text, p.title, p.short_description, p.description, p.category_id,
		       c.name, p.campaign_type, p.currency, p.goal_amount, p.status,
		       coalesce(f.total_collected,0), coalesce(f.total_refunded,0), coalesce(f.total_available,0), coalesce(f.total_reserved,0)
		from projects p
		left join categories c on c.id = p.category_id
		left join project_funds f on f.project_id = p.id
		where p.id = $1`, projectID)
	if err != nil {
		return domain.Project{}, err
	}
	defer rows.Close()
	projects, err := scanProjects(rows)
	if err != nil {
		return domain.Project{}, err
	}
	if len(projects) == 0 {
		return domain.Project{}, domain.ErrNotFound
	}
	return projects[0], nil
}

func scanProjects(rows pgx.Rows) ([]domain.Project, error) {
	var items []domain.Project
	for rows.Next() {
		var p domain.Project
		var categoryID sql.NullInt32
		var categoryName sql.NullString
		if err := rows.Scan(
			&p.ID, &p.AuthorID, &p.Title, &p.ShortDescription, &p.Description, &categoryID,
			&categoryName, &p.CampaignType, &p.Currency, &p.GoalAmount, &p.Status,
			&p.Funds.TotalCollected, &p.Funds.TotalRefunded, &p.Funds.TotalAvailable, &p.Funds.TotalReserved,
		); err != nil {
			return nil, err
		}
		if categoryID.Valid {
			v := int(categoryID.Int32)
			p.CategoryID = &v
		}
		if categoryName.Valid {
			v := categoryName.String
			p.CategoryName = &v
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (r *Repository) CreateProject(ctx context.Context, authorID string, input ProjectInput) (domain.Project, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Project{}, err
	}
	defer tx.Rollback(ctx)
	id := uuid.NewString()
	_, err = tx.Exec(ctx, `
		insert into projects (id, author_id, title, short_description, description, category_id, campaign_type, currency, goal_amount, start_at, end_at, status)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'draft')`,
		id, authorID, input.Title, input.ShortDescription, input.Description, input.CategoryID, input.CampaignType, input.Currency, input.GoalAmount, input.StartAt, input.EndAt)
	if err != nil {
		return domain.Project{}, err
	}
	if _, err := tx.Exec(ctx, `insert into project_funds (project_id) values ($1)`, id); err != nil {
		return domain.Project{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Project{}, err
	}
	return r.getProject(ctx, id)
}

func (r *Repository) UpdateProject(ctx context.Context, projectID, authorID string, input ProjectInput) (domain.Project, error) {
	project, err := r.getProject(ctx, projectID)
	if err != nil {
		return domain.Project{}, err
	}
	if project.AuthorID != authorID {
		return domain.Project{}, domain.ErrForbidden
	}
	if project.Status != domain.ProjectDraft && project.Status != domain.ProjectRejected {
		return domain.Project{}, domain.ErrInvalidStatus
	}
	_, err = r.db.Exec(ctx, `
		update projects set title=$1, short_description=$2, description=$3, category_id=$4, campaign_type=$5,
		    currency=$6, goal_amount=$7, start_at=$8, end_at=$9, updated_at=now()
		where id=$10`,
		input.Title, input.ShortDescription, input.Description, input.CategoryID, input.CampaignType,
		input.Currency, input.GoalAmount, input.StartAt, input.EndAt, projectID)
	if err != nil {
		return domain.Project{}, err
	}
	return r.getProject(ctx, projectID)
}

func (r *Repository) AddMedia(ctx context.Context, projectID, authorID, mediaType, url string, sortOrder int) (map[string]any, error) {
	project, err := r.getProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if project.AuthorID != authorID {
		return nil, domain.ErrForbidden
	}
	id := uuid.NewString()
	_, err = r.db.Exec(ctx, `insert into project_media (id, project_id, media_type, url, sort_order) values ($1,$2,$3,$4,$5)`, id, projectID, mediaType, url, sortOrder)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "project_id": projectID, "media_type": mediaType, "url": url, "sort_order": sortOrder}, nil
}

func (r *Repository) AddMilestone(ctx context.Context, projectID, authorID string, input MilestoneInput) (domain.Milestone, error) {
	project, err := r.getProject(ctx, projectID)
	if err != nil {
		return domain.Milestone{}, err
	}
	if project.AuthorID != authorID {
		return domain.Milestone{}, domain.ErrForbidden
	}
	if project.Status != domain.ProjectDraft && project.Status != domain.ProjectRejected {
		return domain.Milestone{}, domain.ErrInvalidStatus
	}
	id := uuid.NewString()
	_, err = r.db.Exec(ctx, `
		insert into milestones (id, project_id, title, description, due_at, amount_limit, position, status)
		values ($1,$2,$3,$4,$5,$6,$7,'planned')`, id, projectID, input.Title, input.Description, input.DueAt, input.AmountLimit, input.Position)
	if err != nil {
		return domain.Milestone{}, err
	}
	return r.GetMilestone(ctx, id)
}

func (r *Repository) UpdateMilestone(ctx context.Context, projectID, milestoneID, authorID string, input MilestoneInput) (domain.Milestone, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Milestone{}, err
	}
	defer tx.Rollback(ctx)

	var ownerID, status string
	err = tx.QueryRow(ctx, `
		select p.author_id::text, p.status
		from milestones m
		join projects p on p.id = m.project_id
		where m.id=$1 and m.project_id=$2
		for update`, milestoneID, projectID).Scan(&ownerID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Milestone{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Milestone{}, err
	}
	if ownerID != authorID {
		return domain.Milestone{}, domain.ErrForbidden
	}
	if status != domain.ProjectDraft && status != domain.ProjectRejected {
		return domain.Milestone{}, domain.ErrInvalidStatus
	}
	_, err = tx.Exec(ctx, `
		update milestones
		set title=$1, description=$2, due_at=$3, amount_limit=$4, position=$5, updated_at=now()
		where id=$6 and project_id=$7`,
		input.Title, input.Description, input.DueAt, input.AmountLimit, input.Position, milestoneID, projectID)
	if err != nil {
		return domain.Milestone{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Milestone{}, err
	}
	return r.GetMilestone(ctx, milestoneID)
}

func (r *Repository) ListMilestones(ctx context.Context, projectID string) ([]domain.Milestone, error) {
	rows, err := r.db.Query(ctx, `
		select id::text, project_id::text, title, description, due_at, amount_limit, position, status
		from milestones where project_id=$1 order by position`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.Milestone
	for rows.Next() {
		var item domain.Milestone
		var due time.Time
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Title, &item.Description, &due, &item.AmountLimit, &item.Position, &item.Status); err != nil {
			return nil, err
		}
		item.DueAt = due.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetMilestone(ctx context.Context, milestoneID string) (domain.Milestone, error) {
	rows, err := r.db.Query(ctx, `
		select id::text, project_id::text, title, description, due_at, amount_limit, position, status
		from milestones where id=$1`, milestoneID)
	if err != nil {
		return domain.Milestone{}, err
	}
	defer rows.Close()
	items, err := scanMilestones(rows)
	if err != nil {
		return domain.Milestone{}, err
	}
	if len(items) == 0 {
		return domain.Milestone{}, domain.ErrNotFound
	}
	return items[0], nil
}

func scanMilestones(rows pgx.Rows) ([]domain.Milestone, error) {
	var items []domain.Milestone
	for rows.Next() {
		var item domain.Milestone
		var due time.Time
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Title, &item.Description, &due, &item.AmountLimit, &item.Position, &item.Status); err != nil {
			return nil, err
		}
		item.DueAt = due.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) AddReward(ctx context.Context, projectID, authorID string, input RewardInput) (domain.Reward, error) {
	project, err := r.getProject(ctx, projectID)
	if err != nil {
		return domain.Reward{}, err
	}
	if project.AuthorID != authorID {
		return domain.Reward{}, domain.ErrForbidden
	}
	if project.CampaignType != domain.CampaignReward || (project.Status != domain.ProjectDraft && project.Status != domain.ProjectRejected) {
		return domain.Reward{}, domain.ErrInvalidStatus
	}
	id := uuid.NewString()
	_, err = r.db.Exec(ctx, `
		insert into rewards (id, project_id, title, description, min_amount, limit_count, delivery_estimate)
		values ($1,$2,$3,$4,$5,$6,$7)`, id, projectID, input.Title, input.Description, input.MinAmount, input.LimitCount, input.DeliveryEstimate)
	if err != nil {
		return domain.Reward{}, err
	}
	return r.GetReward(ctx, id)
}

func (r *Repository) GetReward(ctx context.Context, rewardID string) (domain.Reward, error) {
	var item domain.Reward
	var delivery sql.NullTime
	err := r.db.QueryRow(ctx, `select id::text, project_id::text, title, description, min_amount, limit_count, delivery_estimate from rewards where id=$1`, rewardID).
		Scan(&item.ID, &item.ProjectID, &item.Title, &item.Description, &item.MinAmount, &item.LimitCount, &delivery)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Reward{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Reward{}, err
	}
	if delivery.Valid {
		v := delivery.Time.Format("2006-01-02")
		item.DeliveryEstimate = &v
	}
	return item, nil
}

func (r *Repository) ListRewards(ctx context.Context, projectID string) ([]domain.Reward, error) {
	rows, err := r.db.Query(ctx, `select id::text, project_id::text, title, description, min_amount, limit_count, delivery_estimate from rewards where project_id=$1 order by min_amount`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.Reward
	for rows.Next() {
		var item domain.Reward
		var delivery sql.NullTime
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Title, &item.Description, &item.MinAmount, &item.LimitCount, &delivery); err != nil {
			return nil, err
		}
		if delivery.Valid {
			v := delivery.Time.Format("2006-01-02")
			item.DeliveryEstimate = &v
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) SubmitProject(ctx context.Context, projectID, authorID string) (domain.Project, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Project{}, err
	}
	defer tx.Rollback(ctx)

	var status, campaignType string
	var goal, milestoneSum float64
	var milestoneCount, rewardCount int
	var ownerID string
	err = tx.QueryRow(ctx, `select author_id::text, status, campaign_type, goal_amount from projects where id=$1 for update`, projectID).Scan(&ownerID, &status, &campaignType, &goal)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Project{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Project{}, err
	}
	if ownerID != authorID {
		return domain.Project{}, domain.ErrForbidden
	}
	err = tx.QueryRow(ctx, `select count(*), coalesce(sum(amount_limit),0) from milestones where project_id=$1`, projectID).Scan(&milestoneCount, &milestoneSum)
	if err != nil {
		return domain.Project{}, err
	}
	err = tx.QueryRow(ctx, `select count(*) from rewards where project_id=$1`, projectID).Scan(&rewardCount)
	if err != nil {
		return domain.Project{}, err
	}
	if err := domain.ValidateSubmit(status, campaignType, goal, milestoneCount, milestoneSum, rewardCount); err != nil {
		return domain.Project{}, err
	}
	_, err = tx.Exec(ctx, `update projects set status='on_review', updated_at=now() where id=$1`, projectID)
	if err != nil {
		return domain.Project{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Project{}, err
	}
	return r.getProject(ctx, projectID)
}

func (r *Repository) AdminProjectDecision(ctx context.Context, projectID, adminID, decision string) (domain.Project, error) {
	status := map[string]string{"approved": domain.ProjectPublished, "rejected": domain.ProjectRejected, "blocked": domain.ProjectBlocked}[decision]
	if status == "" {
		return domain.Project{}, domain.ErrValidation
	}
	project, err := r.getProject(ctx, projectID)
	if err != nil {
		return domain.Project{}, err
	}
	_, err = r.db.Exec(ctx, `update projects set status=$1, updated_at=now() where id=$2`, status, projectID)
	if err != nil {
		return domain.Project{}, err
	}
	event := "project_rejected"
	if status == domain.ProjectPublished {
		event = "project_published"
	}
	_ = r.CreateNotification(ctx, project.AuthorID, event, map[string]any{"project_id": projectID, "decision": decision})
	return r.getProject(ctx, projectID)
}

func (r *Repository) CreatePledgeAndPayment(ctx context.Context, projectID, backerID string, rewardID *string, amount float64) (map[string]any, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var status, campaignType string
	err = tx.QueryRow(ctx, `select status, campaign_type from projects where id=$1`, projectID).Scan(&status, &campaignType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != domain.ProjectPublished || amount <= 0 {
		return nil, domain.ErrInvalidStatus
	}
	if campaignType == domain.CampaignDonation && rewardID != nil {
		return nil, domain.ErrValidation
	}
	if rewardID != nil {
		var minAmount float64
		if err := tx.QueryRow(ctx, `select min_amount from rewards where id=$1 and project_id=$2`, *rewardID, projectID).Scan(&minAmount); err != nil {
			return nil, domain.ErrValidation
		}
		if amount < minAmount {
			return nil, domain.ErrValidation
		}
	}
	pledgeID := uuid.NewString()
	paymentID := uuid.NewString()
	providerPaymentID := "mock_" + paymentID
	_, err = tx.Exec(ctx, `
		insert into pledges (id, project_id, backer_id, reward_id, amount, status)
		values ($1,$2,$3,$4,$5,'payment_pending')`, pledgeID, projectID, backerID, rewardID, amount)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		insert into payments (id, pledge_id, provider, provider_payment_id, status, amount)
		values ($1,$2,'mock',$3,'pending',$4)`, paymentID, pledgeID, providerPaymentID, amount)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{
		"pledge_id":   pledgeID,
		"payment_id":  paymentID,
		"payment_url": "http://localhost:8080/mock-payments/" + paymentID,
		"status":      domain.PledgePaymentPending,
	}, nil
}

func (r *Repository) PaymentByID(ctx context.Context, paymentID string) (PaymentRecord, error) {
	var p PaymentRecord
	err := r.db.QueryRow(ctx, `
		select pay.id::text, pay.pledge_id::text, pl.project_id::text, pl.backer_id::text, pay.provider_payment_id, pay.status, pay.amount
		from payments pay join pledges pl on pl.id = pay.pledge_id
		where pay.id=$1`, paymentID).
		Scan(&p.ID, &p.PledgeID, &p.ProjectID, &p.BackerID, &p.ProviderPaymentID, &p.Status, &p.Amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return PaymentRecord{}, domain.ErrNotFound
	}
	return p, err
}

func (r *Repository) HandlePaymentWebhook(ctx context.Context, provider, providerEventID, providerPaymentID, eventType string, amount float64, payload map[string]any) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	payloadJSON, _ := json.Marshal(payload)
	eventID := uuid.NewString()
	_, err = tx.Exec(ctx, `
		insert into payment_webhook_events (id, provider, provider_event_id, event_type, payload)
		values ($1,$2,$3,$4,$5)`, eventID, provider, providerEventID, eventType, payloadJSON)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDuplicateWebhookEvent
		}
		return err
	}

	var payment PaymentRecord
	err = tx.QueryRow(ctx, `
		select pay.id::text, pay.pledge_id::text, pl.project_id::text, pl.backer_id::text, pay.provider_payment_id, pay.status, pay.amount
		from payments pay join pledges pl on pl.id = pay.pledge_id
		where pay.provider_payment_id=$1 for update`, providerPaymentID).
		Scan(&payment.ID, &payment.PledgeID, &payment.ProjectID, &payment.BackerID, &payment.ProviderPaymentID, &payment.Status, &payment.Amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	_, _ = tx.Exec(ctx, `update payment_webhook_events set payment_id=$1 where id=$2`, payment.ID, eventID)
	if isFinalPaymentStatus(payment.Status) {
		return tx.Commit(ctx)
	}

	switch eventType {
	case "payment.captured":
		_, err = tx.Exec(ctx, `update payments set status='captured', updated_at=now() where id=$1`, payment.ID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `update pledges set status='paid', updated_at=now() where id=$1`, payment.PledgeID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			update project_funds
			set total_collected = total_collected + $1,
			    total_reserved = total_reserved + $1,
			    updated_at = now()
			where project_id=$2`, payment.Amount, payment.ProjectID)
		if err != nil {
			return err
		}
		if err := insertLedger(ctx, tx, payment.ProjectID, domain.OperationCollect, payment.Amount, "payment", payment.ID); err != nil {
			return err
		}
		if err := insertLedger(ctx, tx, payment.ProjectID, domain.OperationReserve, payment.Amount, "payment", payment.ID); err != nil {
			return err
		}
		if err := insertOutbox(ctx, tx, payment.BackerID, "pledge_paid", map[string]any{"project_id": payment.ProjectID, "pledge_id": payment.PledgeID}); err != nil {
			return err
		}
	case "payment.failed", "payment.canceled":
		status := domain.PaymentFailed
		if eventType == "payment.canceled" {
			status = domain.PaymentCanceled
		}
		_, err = tx.Exec(ctx, `update payments set status=$1, updated_at=now() where id=$2`, status, payment.ID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `update pledges set status='canceled', updated_at=now() where id=$1`, payment.PledgeID)
		if err != nil {
			return err
		}
		if err := insertOutbox(ctx, tx, payment.BackerID, "payment_failed", map[string]any{"project_id": payment.ProjectID, "pledge_id": payment.PledgeID}); err != nil {
			return err
		}
	default:
		return domain.ErrValidation
	}
	return tx.Commit(ctx)
}

func (r *Repository) RefundPledge(ctx context.Context, pledgeID, actorID, reason string, isAdmin bool) (map[string]any, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var paymentID, backerID, projectID, paymentStatus, pledgeStatus string
	var amount, reserved float64
	err = tx.QueryRow(ctx, `
		select pay.id::text, pl.backer_id::text, pl.project_id::text, pay.status, pl.status, pay.amount
		from pledges pl join payments pay on pay.pledge_id = pl.id
		where pl.id=$1 for update`, pledgeID).Scan(&paymentID, &backerID, &projectID, &paymentStatus, &pledgeStatus, &amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !isAdmin && backerID != actorID {
		return nil, domain.ErrForbidden
	}
	if paymentStatus != domain.PaymentCaptured || pledgeStatus != domain.PledgePaid {
		return nil, domain.ErrInvalidStatus
	}
	err = tx.QueryRow(ctx, `select total_reserved from project_funds where project_id=$1 for update`, projectID).Scan(&reserved)
	if err != nil {
		return nil, err
	}
	if reserved < amount {
		return nil, domain.ErrInsufficientReservedFunds
	}
	refundID := uuid.NewString()
	_, err = tx.Exec(ctx, `insert into refunds (id, payment_id, provider_refund_id, status, amount, reason) values ($1,$2,$3,'succeeded',$4,$5)`,
		refundID, paymentID, "mock_ref_"+refundID, amount, reason)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `update payments set status='refunded', updated_at=now() where id=$1`, paymentID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `update pledges set status='refunded', updated_at=now() where id=$1`, pledgeID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		update project_funds set total_reserved=total_reserved-$1, total_refunded=total_refunded+$1, updated_at=now()
		where project_id=$2`, amount, projectID)
	if err != nil {
		return nil, err
	}
	if err := insertLedger(ctx, tx, projectID, domain.OperationRefund, amount, "refund", refundID); err != nil {
		return nil, err
	}
	if err := insertOutbox(ctx, tx, backerID, "refund_succeeded", map[string]any{"project_id": projectID, "refund_id": refundID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"refund_id": refundID, "status": "succeeded"}, nil
}

func (r *Repository) SubmitMilestone(ctx context.Context, milestoneID, authorID, reportText string, files []map[string]string) (map[string]any, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var projectID, projectAuthorID, projectStatus, milestoneStatus string
	err = tx.QueryRow(ctx, `
		select m.project_id::text, p.author_id::text, p.status, m.status
		from milestones m join projects p on p.id=m.project_id
		where m.id=$1 for update`, milestoneID).Scan(&projectID, &projectAuthorID, &projectStatus, &milestoneStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if projectAuthorID != authorID {
		return nil, domain.ErrForbidden
	}
	if projectStatus != domain.ProjectPublished || (milestoneStatus != domain.MilestonePlanned && milestoneStatus != domain.MilestoneInProgress && milestoneStatus != domain.MilestoneRejected) {
		return nil, domain.ErrInvalidStatus
	}
	submissionID := uuid.NewString()
	_, err = tx.Exec(ctx, `insert into milestone_submissions (id, milestone_id, author_id, report_text) values ($1,$2,$3,$4)`, submissionID, milestoneID, authorID, reportText)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		_, err = tx.Exec(ctx, `insert into milestone_submission_files (id, submission_id, file_url, file_type) values ($1,$2,$3,$4)`, uuid.NewString(), submissionID, f["file_url"], f["file_type"])
		if err != nil {
			return nil, err
		}
	}
	_, err = tx.Exec(ctx, `update milestones set status='on_review', updated_at=now() where id=$1`, milestoneID)
	if err != nil {
		return nil, err
	}
	if err := insertOutbox(ctx, tx, authorID, "milestone_submitted", map[string]any{"project_id": projectID, "milestone_id": milestoneID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"submission_id": submissionID, "milestone_status": domain.MilestoneInReview}, nil
}

func (r *Repository) ReviewMilestone(ctx context.Context, milestoneID, submissionID, adminID, decision, comment string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var projectID, authorID, status string
	var amount, reserved float64
	err = tx.QueryRow(ctx, `
		select m.project_id::text, p.author_id::text, m.status, m.amount_limit
		from milestones m join projects p on p.id=m.project_id
		where m.id=$1 for update`, milestoneID).Scan(&projectID, &authorID, &status, &amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != domain.MilestoneInReview {
		return domain.ErrInvalidStatus
	}
	_, err = tx.Exec(ctx, `insert into milestone_reviews (id, submission_id, admin_id, decision, comment) values ($1,$2,$3,$4,$5)`, uuid.NewString(), submissionID, adminID, decision, comment)
	if err != nil {
		return err
	}
	switch decision {
	case "approved":
		err = tx.QueryRow(ctx, `select total_reserved from project_funds where project_id=$1 for update`, projectID).Scan(&reserved)
		if err != nil {
			return err
		}
		if reserved < amount {
			return domain.ErrInsufficientReservedFunds
		}
		_, err = tx.Exec(ctx, `update milestones set status='approved', updated_at=now() where id=$1`, milestoneID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `update project_funds set total_reserved=total_reserved-$1, total_available=total_available+$1, updated_at=now() where project_id=$2`, amount, projectID)
		if err != nil {
			return err
		}
		if err := insertLedger(ctx, tx, projectID, domain.OperationRelease, amount, "milestone", milestoneID); err != nil {
			return err
		}
		if err := insertOutbox(ctx, tx, authorID, "milestone_approved", map[string]any{"project_id": projectID, "milestone_id": milestoneID}); err != nil {
			return err
		}
	case "rejected":
		_, err = tx.Exec(ctx, `update milestones set status='rejected', updated_at=now() where id=$1`, milestoneID)
		if err != nil {
			return err
		}
		if err := insertOutbox(ctx, tx, authorID, "milestone_rejected", map[string]any{"project_id": projectID, "milestone_id": milestoneID}); err != nil {
			return err
		}
	default:
		return domain.ErrValidation
	}
	return tx.Commit(ctx)
}

func (r *Repository) AddProjectUpdate(ctx context.Context, projectID, authorID, title, content string) (map[string]any, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var ownerID string
	if err := tx.QueryRow(ctx, `select author_id::text from projects where id=$1`, projectID).Scan(&ownerID); err != nil {
		return nil, err
	}
	if ownerID != authorID {
		return nil, domain.ErrForbidden
	}
	id := uuid.NewString()
	_, err = tx.Exec(ctx, `insert into project_updates (id, project_id, author_id, title, content) values ($1,$2,$3,$4,$5)`, id, projectID, authorID, title, content)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `select distinct backer_id::text from pledges where project_id=$1 and status='paid'`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var backerID string
		if err := rows.Scan(&backerID); err != nil {
			return nil, err
		}
		if err := insertOutbox(ctx, tx, backerID, "project_update_created", map[string]any{"project_id": projectID, "update_id": id}); err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "project_id": projectID, "title": title, "content": content}, nil
}

func (r *Repository) listUpdates(ctx context.Context, projectID string) ([]any, error) {
	rows, err := r.db.Query(ctx, `select id::text, title, content, created_at from project_updates where project_id=$1 order by created_at desc`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []any
	for rows.Next() {
		var id, title, content string
		var created time.Time
		if err := rows.Scan(&id, &title, &content, &created); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"id": id, "title": title, "content": content, "created_at": created})
	}
	return items, rows.Err()
}

func (r *Repository) listMedia(ctx context.Context, projectID string) ([]any, error) {
	rows, err := r.db.Query(ctx, `select id::text, media_type, url, sort_order from project_media where project_id=$1 order by sort_order`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []any
	for rows.Next() {
		var id, mediaType, url string
		var sortOrder int
		if err := rows.Scan(&id, &mediaType, &url, &sortOrder); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"id": id, "media_type": mediaType, "url": url, "sort_order": sortOrder})
	}
	return items, rows.Err()
}

func (r *Repository) ListMilestoneSubmissions(ctx context.Context) ([]map[string]any, error) {
	rows, err := r.db.Query(ctx, `
		select s.id::text, s.milestone_id::text, s.author_id::text, s.report_text, s.submitted_at, m.title, p.title,
		       coalesce(jsonb_agg(jsonb_build_object('file_url', f.file_url, 'file_type', f.file_type) order by f.file_url) filter (where f.id is not null), '[]'::jsonb)
		from milestone_submissions s
		join milestones m on m.id=s.milestone_id
		join projects p on p.id=m.project_id
		left join milestone_submission_files f on f.submission_id=s.id
		where m.status='on_review'
		group by s.id, s.milestone_id, s.author_id, s.report_text, s.submitted_at, m.title, p.title
		order by s.submitted_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []map[string]any
	for rows.Next() {
		var id, milestoneID, authorID, report, milestoneTitle, projectTitle string
		var submittedAt time.Time
		var files []byte
		if err := rows.Scan(&id, &milestoneID, &authorID, &report, &submittedAt, &milestoneTitle, &projectTitle, &files); err != nil {
			return nil, err
		}
		var submissionFiles []map[string]string
		if err := json.Unmarshal(files, &submissionFiles); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id": id, "milestone_id": milestoneID, "author_id": authorID, "report_text": report,
			"submitted_at": submittedAt, "milestone_title": milestoneTitle, "project_title": projectTitle, "files": submissionFiles,
		})
	}
	return items, rows.Err()
}

func (r *Repository) SetUserBlocked(ctx context.Context, userID string, blocked bool) error {
	tag, err := r.db.Exec(ctx, `update users set is_blocked=$1, updated_at=now() where id=$2`, blocked, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) ListBroadcasts(ctx context.Context, projectID string) ([]domain.Broadcast, error) {
	rows, err := r.db.Query(ctx, `
		select id::text, project_id::text, status
		from broadcast
		where project_id=$1
		order by id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.Broadcast
	for rows.Next() {
		var item domain.Broadcast
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Status); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		files, err := r.ListBroadcastFiles(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].Files = files
	}
	return items, nil
}

func (r *Repository) CreateBroadcast(ctx context.Context, projectID, authorID, status string) (domain.Broadcast, error) {
	var ownerID string
	err := r.db.QueryRow(ctx, `select author_id::text from projects where id=$1`, projectID).Scan(&ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Broadcast{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Broadcast{}, err
	}
	if ownerID != authorID {
		return domain.Broadcast{}, domain.ErrForbidden
	}
	id := uuid.NewString()
	_, err = r.db.Exec(ctx, `insert into broadcast (id, project_id, status) values ($1,$2,$3)`, id, projectID, status)
	if err != nil {
		return domain.Broadcast{}, err
	}
	return r.GetBroadcast(ctx, id)
}

func (r *Repository) GetBroadcast(ctx context.Context, broadcastID string) (domain.Broadcast, error) {
	var item domain.Broadcast
	err := r.db.QueryRow(ctx, `
		select id::text, project_id::text, status
		from broadcast
		where id=$1`, broadcastID).Scan(&item.ID, &item.ProjectID, &item.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Broadcast{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Broadcast{}, err
	}
	files, err := r.ListBroadcastFiles(ctx, broadcastID)
	if err != nil {
		return domain.Broadcast{}, err
	}
	item.Files = files
	return item, nil
}

func (r *Repository) UpdateBroadcastStatus(ctx context.Context, broadcastID, authorID, status string) (domain.Broadcast, error) {
	tag, err := r.db.Exec(ctx, `
		update broadcast b
		set status=$1
		from projects p
		where b.project_id=p.id and b.id=$2 and p.author_id=$3`, status, broadcastID, authorID)
	if err != nil {
		return domain.Broadcast{}, err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		err := r.db.QueryRow(ctx, `select exists(select 1 from broadcast where id=$1)`, broadcastID).Scan(&exists)
		if err != nil {
			return domain.Broadcast{}, err
		}
		if exists {
			return domain.Broadcast{}, domain.ErrForbidden
		}
		return domain.Broadcast{}, domain.ErrNotFound
	}
	return r.GetBroadcast(ctx, broadcastID)
}

func (r *Repository) DeleteBroadcast(ctx context.Context, broadcastID, authorID string) error {
	tag, err := r.db.Exec(ctx, `
		delete from broadcast b
		using projects p
		where b.project_id=p.id and b.id=$1 and p.author_id=$2`, broadcastID, authorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		err := r.db.QueryRow(ctx, `select exists(select 1 from broadcast where id=$1)`, broadcastID).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			return domain.ErrForbidden
		}
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) AddBroadcastFile(ctx context.Context, broadcastID, authorID, fileURL string) (domain.BroadcastChatFile, error) {
	var ownerID string
	err := r.db.QueryRow(ctx, `
		select p.author_id::text
		from broadcast b
		join projects p on p.id=b.project_id
		where b.id=$1`, broadcastID).Scan(&ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.BroadcastChatFile{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.BroadcastChatFile{}, err
	}
	if ownerID != authorID {
		return domain.BroadcastChatFile{}, domain.ErrForbidden
	}
	id := uuid.NewString()
	_, err = r.db.Exec(ctx, `insert into broadcast_chat_files (id, broadcast_id, file_url) values ($1,$2,$3)`, id, broadcastID, fileURL)
	if err != nil {
		return domain.BroadcastChatFile{}, err
	}
	return domain.BroadcastChatFile{ID: id, BroadcastID: broadcastID, FileURL: fileURL}, nil
}

func (r *Repository) ListBroadcastFiles(ctx context.Context, broadcastID string) ([]domain.BroadcastChatFile, error) {
	rows, err := r.db.Query(ctx, `
		select id::text, broadcast_id::text, file_url
		from broadcast_chat_files
		where broadcast_id=$1
		order by id`, broadcastID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.BroadcastChatFile
	for rows.Next() {
		var item domain.BroadcastChatFile
		if err := rows.Scan(&item.ID, &item.BroadcastID, &item.FileURL); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) CreateNotification(ctx context.Context, userID, eventType string, payload map[string]any) error {
	return insertOutbox(ctx, r.db, userID, eventType, payload)
}

func (r *Repository) TakePendingOutbox(ctx context.Context, limit int) ([]OutboxItem, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `
		select id::text, user_id::text, event_type, payload
		from notification_outbox
		where status='pending'
		order by created_at
		limit $1
		for update skip locked`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []OutboxItem
	for rows.Next() {
		var item OutboxItem
		if err := rows.Scan(&item.ID, &item.UserID, &item.EventType, &item.Payload); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, item := range items {
		if _, err := tx.Exec(ctx, `update notification_outbox set status='processing', attempts=attempts+1, updated_at=now() where id=$1`, item.ID); err != nil {
			return nil, err
		}
	}
	return items, tx.Commit(ctx)
}

func (r *Repository) MarkOutboxSent(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `update notification_outbox set status='sent', updated_at=now() where id=$1`, id)
	return err
}

func insertLedger(ctx context.Context, tx pgx.Tx, projectID, operation string, amount float64, referenceType, referenceID string) error {
	_, err := tx.Exec(ctx, `
		insert into fund_ledger (id, project_id, operation_type, amount, reference_type, reference_id)
		values ($1,$2,$3,$4,$5,$6)`, uuid.NewString(), projectID, operation, amount, referenceType, referenceID)
	return err
}

type execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func insertOutbox(ctx context.Context, db execer, userID, eventType string, payload map[string]any) error {
	payloadJSON, _ := json.Marshal(payload)
	_, err := db.Exec(ctx, `
		insert into notification_outbox (id, user_id, event_type, payload, status)
		values ($1,$2,$3,$4,'pending')`, uuid.NewString(), userID, eventType, payloadJSON)
	return err
}

func isFinalPaymentStatus(status string) bool {
	return status == domain.PaymentCaptured || status == domain.PaymentFailed || status == domain.PaymentCanceled || status == domain.PaymentRefunded
}
