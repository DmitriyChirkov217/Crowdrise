package domain

import "errors"

const (
	RoleBacker = "backer"
	RoleAuthor = "author"
	RoleAdmin  = "admin"

	ProjectDraft     = "draft"
	ProjectOnReview  = "on_review"
	ProjectRejected  = "rejected"
	ProjectPublished = "published"
	ProjectCompleted = "completed"
	ProjectBlocked   = "blocked"
	ProjectCanceled  = "canceled"

	CampaignReward   = "reward"
	CampaignDonation = "donation"

	MilestonePlanned    = "planned"
	MilestoneInReview   = "on_review"
	MilestoneApproved   = "approved"
	MilestoneRejected   = "rejected"
	MilestoneInProgress = "in_progress"

	PledgePaymentPending = "payment_pending"
	PledgePaid           = "paid"
	PledgeCanceled       = "canceled"
	PledgeRefunded       = "refunded"

	PaymentPending  = "pending"
	PaymentCaptured = "captured"
	PaymentFailed   = "failed"
	PaymentCanceled = "canceled"
	PaymentRefunded = "refunded"

	OperationCollect = "collect"
	OperationReserve = "reserve"
	OperationRelease = "release"
	OperationRefund  = "refund"

	OutboxPending    = "pending"
	OutboxProcessing = "processing"
	OutboxSent       = "sent"
	OutboxFailed     = "failed"
)

var (
	ErrForbidden                 = errors.New("forbidden")
	ErrNotFound                  = errors.New("not found")
	ErrValidation                = errors.New("validation failed")
	ErrInvalidStatus             = errors.New("invalid status")
	ErrInsufficientReservedFunds = errors.New("insufficient reserved funds")
	ErrDuplicateWebhookEvent     = errors.New("duplicate webhook event")
)

type User struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	Phone       *string  `json:"phone,omitempty"`
	IsBlocked   bool     `json:"is_blocked"`
	Roles       []string `json:"roles"`
}

type Funds struct {
	TotalCollected float64 `json:"total_collected"`
	TotalRefunded  float64 `json:"total_refunded"`
	TotalAvailable float64 `json:"total_available"`
	TotalReserved  float64 `json:"total_reserved"`
}

type Project struct {
	ID               string  `json:"id"`
	AuthorID         string  `json:"author_id"`
	Title            string  `json:"title"`
	ShortDescription string  `json:"short_description"`
	Description      string  `json:"description"`
	CategoryID       *int    `json:"category_id,omitempty"`
	CategoryName     *string `json:"category_name,omitempty"`
	CampaignType     string  `json:"campaign_type"`
	Currency         string  `json:"currency"`
	GoalAmount       float64 `json:"goal_amount"`
	Status           string  `json:"status"`
	Funds            Funds   `json:"funds"`
}

type Milestone struct {
	ID          string  `json:"id"`
	ProjectID   string  `json:"project_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	DueAt       string  `json:"due_at"`
	AmountLimit float64 `json:"amount_limit"`
	Position    int     `json:"position"`
	Status      string  `json:"status"`
}

type Reward struct {
	ID               string  `json:"id"`
	ProjectID        string  `json:"project_id"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	MinAmount        float64 `json:"min_amount"`
	LimitCount       *int    `json:"limit_count,omitempty"`
	DeliveryEstimate *string `json:"delivery_estimate,omitempty"`
}

type ProjectDetails struct {
	Project    Project     `json:"project"`
	Media      []any       `json:"media"`
	Milestones []Milestone `json:"milestones"`
	Rewards    []Reward    `json:"rewards"`
	Updates    []any       `json:"updates"`
}
