package services

import (
	"context"
	"strings"
	"time"

	"crowdrise/backend/internal/domain"
	"crowdrise/backend/internal/repositories"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo      *repositories.Repository
	jwtSecret []byte
}

type AuthResponse struct {
	Token string      `json:"token"`
	User  domain.User `json:"user"`
}

type Claims struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

func New(repo *repositories.Repository, jwtSecret string) *Service {
	return &Service{repo: repo, jwtSecret: []byte(jwtSecret)}
}

func (s *Service) Register(ctx context.Context, email, password, displayName string, phone *string) (AuthResponse, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	displayName = strings.TrimSpace(displayName)
	if !validEmail(email) || len(password) < 8 || displayName == "" {
		return AuthResponse{}, domain.ErrValidation
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResponse{}, err
	}
	user, err := s.repo.CreateUser(ctx, email, string(hash), displayName, phone)
	if err != nil {
		return AuthResponse{}, err
	}
	token, err := s.tokenFor(user)
	if err != nil {
		return AuthResponse{}, err
	}
	return AuthResponse{Token: token, User: user}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (AuthResponse, error) {
	user, err := s.repo.GetAuthUserByEmail(ctx, strings.TrimSpace(strings.ToLower(email)))
	if err != nil {
		return AuthResponse{}, err
	}
	if user.IsBlocked {
		return AuthResponse{}, domain.ErrForbidden
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return AuthResponse{}, domain.ErrForbidden
	}
	token, err := s.tokenFor(user.User)
	if err != nil {
		return AuthResponse{}, err
	}
	return AuthResponse{Token: token, User: user.User}, nil
}

func (s *Service) ParseToken(tokenValue string) (Claims, error) {
	claims := Claims{}
	token, err := jwt.ParseWithClaims(tokenValue, &claims, func(token *jwt.Token) (any, error) {
		return s.jwtSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		return Claims{}, domain.ErrForbidden
	}
	return claims, nil
}

func (s *Service) Me(ctx context.Context, userID string) (domain.User, error) {
	return s.repo.GetUserByID(ctx, userID)
}

func (s *Service) Categories(ctx context.Context) ([]repositories.Category, error) {
	return s.repo.ListCategories(ctx)
}

func (s *Service) ListProjects(ctx context.Context, status, campaignType string, page, pageSize int) ([]domain.Project, int, error) {
	return s.repo.ListProjects(ctx, status, campaignType, page, pageSize)
}

func (s *Service) ProjectDetails(ctx context.Context, projectID string) (domain.ProjectDetails, error) {
	return s.repo.GetProjectDetails(ctx, projectID)
}

func (s *Service) ListBroadcasts(ctx context.Context, projectID string) ([]domain.Broadcast, error) {
	return s.repo.ListBroadcasts(ctx, projectID)
}

func (s *Service) CreateBroadcast(ctx context.Context, projectID, authorID, status string) (domain.Broadcast, error) {
	if status == "" {
		status = domain.BroadcastScheduled
	}
	if !validBroadcastStatus(status) {
		return domain.Broadcast{}, domain.ErrValidation
	}
	return s.repo.CreateBroadcast(ctx, projectID, authorID, status)
}

func (s *Service) UpdateBroadcastStatus(ctx context.Context, broadcastID, authorID, status string) (domain.Broadcast, error) {
	if !validBroadcastStatus(status) {
		return domain.Broadcast{}, domain.ErrValidation
	}
	return s.repo.UpdateBroadcastStatus(ctx, broadcastID, authorID, status)
}

func (s *Service) DeleteBroadcast(ctx context.Context, broadcastID, authorID string) error {
	return s.repo.DeleteBroadcast(ctx, broadcastID, authorID)
}

func (s *Service) AddBroadcastFile(ctx context.Context, broadcastID, authorID, fileURL string) (domain.BroadcastChatFile, error) {
	if strings.TrimSpace(fileURL) == "" {
		return domain.BroadcastChatFile{}, domain.ErrValidation
	}
	return s.repo.AddBroadcastFile(ctx, broadcastID, authorID, strings.TrimSpace(fileURL))
}

func (s *Service) ListBroadcastFiles(ctx context.Context, broadcastID string) ([]domain.BroadcastChatFile, error) {
	return s.repo.ListBroadcastFiles(ctx, broadcastID)
}

func (s *Service) CanJoinBroadcast(ctx context.Context, broadcastID, userID string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.IsBlocked {
		return domain.ErrForbidden
	}
	broadcast, err := s.repo.GetBroadcast(ctx, broadcastID)
	if err != nil {
		return err
	}
	if broadcast.Status == domain.BroadcastEnded {
		return domain.ErrInvalidStatus
	}
	return nil
}

func (s *Service) CreateProject(ctx context.Context, authorID string, input repositories.ProjectInput) (domain.Project, error) {
	if err := validateProjectInput(input); err != nil {
		return domain.Project{}, err
	}
	return s.repo.CreateProject(ctx, authorID, input)
}

func (s *Service) UpdateProject(ctx context.Context, projectID, authorID string, input repositories.ProjectInput) (domain.Project, error) {
	if err := validateProjectInput(input); err != nil {
		return domain.Project{}, err
	}
	return s.repo.UpdateProject(ctx, projectID, authorID, input)
}

func (s *Service) AddMedia(ctx context.Context, projectID, authorID, mediaType, url string, sortOrder int) (map[string]any, error) {
	if mediaType == "" || url == "" {
		return nil, domain.ErrValidation
	}
	return s.repo.AddMedia(ctx, projectID, authorID, mediaType, url, sortOrder)
}

func (s *Service) AddMilestone(ctx context.Context, projectID, authorID string, input repositories.MilestoneInput) (domain.Milestone, error) {
	if input.Title == "" || input.Description == "" || input.AmountLimit <= 0 || input.Position <= 0 || input.DueAt.IsZero() {
		return domain.Milestone{}, domain.ErrValidation
	}
	return s.repo.AddMilestone(ctx, projectID, authorID, input)
}

func (s *Service) UpdateMilestone(ctx context.Context, projectID, milestoneID, authorID string, input repositories.MilestoneInput) (domain.Milestone, error) {
	if input.Title == "" || input.Description == "" || input.AmountLimit <= 0 || input.Position <= 0 || input.DueAt.IsZero() {
		return domain.Milestone{}, domain.ErrValidation
	}
	return s.repo.UpdateMilestone(ctx, projectID, milestoneID, authorID, input)
}

func (s *Service) AddReward(ctx context.Context, projectID, authorID string, input repositories.RewardInput) (domain.Reward, error) {
	if input.Title == "" || input.Description == "" || input.MinAmount <= 0 {
		return domain.Reward{}, domain.ErrValidation
	}
	return s.repo.AddReward(ctx, projectID, authorID, input)
}

func (s *Service) SubmitProject(ctx context.Context, projectID, authorID string) (domain.Project, error) {
	return s.repo.SubmitProject(ctx, projectID, authorID)
}

func (s *Service) AdminProjectDecision(ctx context.Context, projectID, adminID, decision string) (domain.Project, error) {
	return s.repo.AdminProjectDecision(ctx, projectID, adminID, decision)
}

func (s *Service) CreatePledge(ctx context.Context, projectID, backerID string, rewardID *string, amount float64) (map[string]any, error) {
	if amount <= 0 {
		return nil, domain.ErrValidation
	}
	return s.repo.CreatePledgeAndPayment(ctx, projectID, backerID, rewardID, amount)
}

func (s *Service) MockCapture(ctx context.Context, paymentID string) error {
	payment, err := s.repo.PaymentByID(ctx, paymentID)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"provider":            "mock",
		"provider_event_id":   "evt_" + uuid.NewString(),
		"provider_payment_id": payment.ProviderPaymentID,
		"event_type":          "payment.captured",
		"amount":              payment.Amount,
	}
	return s.PaymentWebhook(ctx, payload)
}

func (s *Service) MockFail(ctx context.Context, paymentID string) error {
	payment, err := s.repo.PaymentByID(ctx, paymentID)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"provider":            "mock",
		"provider_event_id":   "evt_" + uuid.NewString(),
		"provider_payment_id": payment.ProviderPaymentID,
		"event_type":          "payment.failed",
		"amount":              payment.Amount,
	}
	return s.PaymentWebhook(ctx, payload)
}

func (s *Service) PaymentWebhook(ctx context.Context, payload map[string]any) error {
	provider, _ := payload["provider"].(string)
	eventID, _ := payload["provider_event_id"].(string)
	providerPaymentID, _ := payload["provider_payment_id"].(string)
	eventType, _ := payload["event_type"].(string)
	amount, _ := payload["amount"].(float64)
	if provider == "" || eventID == "" || providerPaymentID == "" || eventType == "" {
		return domain.ErrValidation
	}
	err := s.repo.HandlePaymentWebhook(ctx, provider, eventID, providerPaymentID, eventType, amount, payload)
	if err == domain.ErrDuplicateWebhookEvent {
		return nil
	}
	return err
}

func (s *Service) RefundPledge(ctx context.Context, pledgeID, actorID, reason string, isAdmin bool) (map[string]any, error) {
	return s.repo.RefundPledge(ctx, pledgeID, actorID, reason, isAdmin)
}

func (s *Service) SubmitMilestone(ctx context.Context, milestoneID, authorID, reportText string, files []map[string]string) (map[string]any, error) {
	if strings.TrimSpace(reportText) == "" {
		return nil, domain.ErrValidation
	}
	return s.repo.SubmitMilestone(ctx, milestoneID, authorID, reportText, files)
}

func (s *Service) ReviewMilestone(ctx context.Context, milestoneID, submissionID, adminID, decision, comment string) error {
	return s.repo.ReviewMilestone(ctx, milestoneID, submissionID, adminID, decision, comment)
}

func (s *Service) AddProjectUpdate(ctx context.Context, projectID, authorID, title, content string) (map[string]any, error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
		return nil, domain.ErrValidation
	}
	return s.repo.AddProjectUpdate(ctx, projectID, authorID, title, content)
}

func (s *Service) ListMilestoneSubmissions(ctx context.Context) ([]map[string]any, error) {
	return s.repo.ListMilestoneSubmissions(ctx)
}

func (s *Service) SetUserBlocked(ctx context.Context, userID string, blocked bool) error {
	return s.repo.SetUserBlocked(ctx, userID, blocked)
}

func (s *Service) tokenFor(user domain.User) (string, error) {
	claims := Claims{
		UserID: user.ID,
		Roles:  user.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

func validEmail(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".") && !strings.Contains(email, " ")
}

func validateProjectInput(input repositories.ProjectInput) error {
	if strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.ShortDescription) == "" || strings.TrimSpace(input.Description) == "" {
		return domain.ErrValidation
	}
	if input.GoalAmount <= 0 {
		return domain.ErrValidation
	}
	if input.CampaignType != domain.CampaignReward && input.CampaignType != domain.CampaignDonation {
		return domain.ErrValidation
	}
	if len(input.Currency) != 3 {
		return domain.ErrValidation
	}
	return nil
}

func HasRole(roles []string, role string) bool {
	for _, item := range roles {
		if item == role {
			return true
		}
	}
	return false
}

func validBroadcastStatus(status string) bool {
	return status == domain.BroadcastScheduled || status == domain.BroadcastLive || status == domain.BroadcastEnded
}
