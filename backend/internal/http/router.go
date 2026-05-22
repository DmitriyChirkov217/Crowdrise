package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"crowdrise/backend/internal/config"
	"crowdrise/backend/internal/domain"
	"crowdrise/backend/internal/realtime"
	"crowdrise/backend/internal/repositories"
	"crowdrise/backend/internal/services"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"nhooyr.io/websocket"
)

type Server struct {
	app *services.Service
	cfg config.Config
	hub *realtime.Hub
}

type contextKey string

const claimsKey contextKey = "claims"

func NewRouter(app *services.Service, cfg config.Config) http.Handler {
	s := &Server{app: app, cfg: cfg, hub: realtime.NewHub()}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.logRequests)
	r.Use(s.cors)
	r.Use(middleware.Recoverer)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", s.register)
		r.Post("/auth/login", s.login)
		r.Get("/categories", s.categories)
		r.Get("/projects", s.listProjects)
		r.Get("/projects/{project_id}", s.projectDetails)
		r.Get("/projects/{project_id}/broadcasts", s.listBroadcasts)
		r.Get("/broadcasts/{broadcast_id}/files", s.listBroadcastFiles)
		r.Get("/broadcasts/{broadcast_id}/ws", s.broadcastVoice)
		r.Post("/integrations/payments/webhook", s.paymentWebhook)

		r.Group(func(r chi.Router) {
			r.Use(s.auth)
			r.Get("/users/me", s.me)
			r.Post("/projects", s.requireRole(domain.RoleAuthor, s.createProject))
			r.Put("/projects/{project_id}", s.requireRole(domain.RoleAuthor, s.updateProject))
			r.Post("/projects/{project_id}/media", s.requireRole(domain.RoleAuthor, s.addMedia))
			r.Post("/projects/{project_id}/broadcasts", s.requireRole(domain.RoleAuthor, s.createBroadcast))
			r.Post("/projects/{project_id}/submit", s.requireRole(domain.RoleAuthor, s.submitProject))
			r.Post("/projects/{project_id}/milestones", s.requireRole(domain.RoleAuthor, s.addMilestone))
			r.Put("/projects/{project_id}/milestones/{milestone_id}", s.requireRole(domain.RoleAuthor, s.updateMilestone))
			r.Post("/projects/{project_id}/rewards", s.requireRole(domain.RoleAuthor, s.addReward))
			r.Post("/projects/{project_id}/updates", s.requireRole(domain.RoleAuthor, s.addProjectUpdate))
			r.Put("/broadcasts/{broadcast_id}/status", s.requireRole(domain.RoleAuthor, s.updateBroadcastStatus))
			r.Delete("/broadcasts/{broadcast_id}", s.requireRole(domain.RoleAuthor, s.deleteBroadcast))
			r.Post("/broadcasts/{broadcast_id}/files", s.requireRole(domain.RoleAuthor, s.addBroadcastFile))
			r.Post("/projects/{project_id}/pledges", s.requireRole(domain.RoleBacker, s.createPledge))
			r.Post("/milestones/{milestone_id}/submit", s.requireRole(domain.RoleAuthor, s.submitMilestoneReport))
			r.Post("/pledges/{pledge_id}/refund", s.refundPledge)
			r.Post("/payments/{payment_id}/mock-capture", s.requireRole(domain.RoleAdmin, s.mockCapture))
			r.Post("/payments/{payment_id}/mock-fail", s.requireRole(domain.RoleAdmin, s.mockFail))

			r.Get("/admin/projects", s.requireRole(domain.RoleAdmin, s.adminProjects))
			r.Get("/admin/projects/{project_id}", s.requireRole(domain.RoleAdmin, s.projectDetails))
			r.Post("/admin/projects/{project_id}/decision", s.requireRole(domain.RoleAdmin, s.adminProjectDecision))
			r.Get("/admin/milestones", s.requireRole(domain.RoleAdmin, s.adminMilestones))
			r.Post("/admin/milestones/{milestone_id}/review", s.requireRole(domain.RoleAdmin, s.adminMilestoneReview))
			r.Post("/admin/users/{user_id}/block", s.requireRole(domain.RoleAdmin, s.blockUser))
			r.Post("/admin/users/{user_id}/unblock", s.requireRole(domain.RoleAdmin, s.unblockUser))
		})
	})
	return r
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := allowedOrigin(r.Header.Get("Origin"), s.cfg.CORSOrigin); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func allowedOrigin(requestOrigin, configuredOrigins string) string {
	if configuredOrigins == "*" {
		return "*"
	}
	for _, origin := range strings.Split(configuredOrigins, ",") {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin != "" && origin == requestOrigin {
			return origin
		}
	}
	return ""
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("request_id", middleware.GetReqID(r.Context())).
			Dur("duration", time.Since(start)).
			Msg("http request")
	})
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeError(w, domain.ErrForbidden)
			return
		}
		claims, err := s.app.ParseToken(token)
		if err != nil {
			writeError(w, err)
			return
		}
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requireRole(role string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !services.HasRole(claims(r).Roles, role) {
			writeError(w, domain.ErrForbidden)
			return
		}
		h(w, r)
	}
}

func claims(r *http.Request) services.Claims {
	value, _ := r.Context().Value(claimsKey).(services.Claims)
	return value
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string  `json:"email"`
		Password    string  `json:"password"`
		DisplayName string  `json:"display_name"`
		Phone       *string `json:"phone"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := s.app.Register(r.Context(), req.Email, req.Password, req.DisplayName, req.Phone)
	writeResult(w, res, err, http.StatusCreated)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := s.app.Login(r.Context(), req.Email, req.Password)
	writeResult(w, res, err, http.StatusOK)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	res, err := s.app.Me(r.Context(), claims(r).UserID)
	writeResult(w, res, err, http.StatusOK)
}

func (s *Server) categories(w http.ResponseWriter, r *http.Request) {
	res, err := s.app.Categories(r.Context())
	writeResult(w, res, err, http.StatusOK)
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	items, total, err := s.app.ListProjects(r.Context(), r.URL.Query().Get("status"), r.URL.Query().Get("campaign_type"), page, pageSize)
	writeResult(w, map[string]any{"items": items, "page": max(page, 1), "page_size": pageSizeOrDefault(pageSize), "total": total}, err, http.StatusOK)
}

func (s *Server) adminProjects(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	status := r.URL.Query().Get("status")
	if status == "" {
		status = domain.ProjectOnReview
	}
	items, total, err := s.app.ListProjects(r.Context(), status, "", page, pageSize)
	writeResult(w, map[string]any{"items": items, "page": max(page, 1), "page_size": pageSizeOrDefault(pageSize), "total": total}, err, http.StatusOK)
}

func (s *Server) projectDetails(w http.ResponseWriter, r *http.Request) {
	res, err := s.app.ProjectDetails(r.Context(), chi.URLParam(r, "project_id"))
	writeResult(w, res, err, http.StatusOK)
}

func (s *Server) listBroadcasts(w http.ResponseWriter, r *http.Request) {
	res, err := s.app.ListBroadcasts(r.Context(), chi.URLParam(r, "project_id"))
	writeResult(w, map[string]any{"items": res}, err, http.StatusOK)
}

func (s *Server) createBroadcast(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := s.app.CreateBroadcast(r.Context(), chi.URLParam(r, "project_id"), claims(r).UserID, req.Status)
	writeResult(w, res, err, http.StatusCreated)
}

func (s *Server) updateBroadcastStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := s.app.UpdateBroadcastStatus(r.Context(), chi.URLParam(r, "broadcast_id"), claims(r).UserID, req.Status)
	writeResult(w, res, err, http.StatusOK)
}

func (s *Server) deleteBroadcast(w http.ResponseWriter, r *http.Request) {
	err := s.app.DeleteBroadcast(r.Context(), chi.URLParam(r, "broadcast_id"), claims(r).UserID)
	writeResult(w, map[string]string{"status": "deleted"}, err, http.StatusOK)
}

func (s *Server) addBroadcastFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FileURL string `json:"file_url"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := s.app.AddBroadcastFile(r.Context(), chi.URLParam(r, "broadcast_id"), claims(r).UserID, req.FileURL)
	writeResult(w, res, err, http.StatusCreated)
}

func (s *Server) listBroadcastFiles(w http.ResponseWriter, r *http.Request) {
	res, err := s.app.ListBroadcastFiles(r.Context(), chi.URLParam(r, "broadcast_id"))
	writeResult(w, map[string]any{"items": res}, err, http.StatusOK)
}

func (s *Server) broadcastVoice(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	claims, err := s.app.ParseToken(token)
	if err != nil {
		writeError(w, err)
		return
	}
	broadcastID := chi.URLParam(r, "broadcast_id")
	if err := s.app.CanJoinBroadcast(r.Context(), broadcastID, claims.UserID); err != nil {
		writeError(w, err)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		log.Error().Err(err).Msg("websocket accept")
		return
	}
	s.hub.Join(r.Context(), conn, broadcastID)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	input, ok := projectInputFromRequest(w, r)
	if !ok {
		return
	}
	res, err := s.app.CreateProject(r.Context(), claims(r).UserID, input)
	writeResult(w, res, err, http.StatusCreated)
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	input, ok := projectInputFromRequest(w, r)
	if !ok {
		return
	}
	res, err := s.app.UpdateProject(r.Context(), chi.URLParam(r, "project_id"), claims(r).UserID, input)
	writeResult(w, res, err, http.StatusOK)
}

func (s *Server) addMedia(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MediaType string `json:"media_type"`
		URL       string `json:"url"`
		SortOrder int    `json:"sort_order"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := s.app.AddMedia(r.Context(), chi.URLParam(r, "project_id"), claims(r).UserID, req.MediaType, req.URL, req.SortOrder)
	writeResult(w, res, err, http.StatusCreated)
}

func (s *Server) submitProject(w http.ResponseWriter, r *http.Request) {
	res, err := s.app.SubmitProject(r.Context(), chi.URLParam(r, "project_id"), claims(r).UserID)
	writeResult(w, map[string]any{"id": res.ID, "status": res.Status}, err, http.StatusOK)
}

func (s *Server) addMilestone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		DueAt       string  `json:"due_at"`
		AmountLimit float64 `json:"amount_limit"`
		Position    int     `json:"position"`
	}
	if !decode(w, r, &req) {
		return
	}
	due, err := time.Parse(time.RFC3339, req.DueAt)
	if err != nil {
		writeError(w, domain.ErrValidation)
		return
	}
	res, err := s.app.AddMilestone(r.Context(), chi.URLParam(r, "project_id"), claims(r).UserID, repositories.MilestoneInput{
		Title: req.Title, Description: req.Description, DueAt: due, AmountLimit: req.AmountLimit, Position: req.Position,
	})
	writeResult(w, res, err, http.StatusCreated)
}

func (s *Server) updateMilestone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		DueAt       string  `json:"due_at"`
		AmountLimit float64 `json:"amount_limit"`
		Position    int     `json:"position"`
	}
	if !decode(w, r, &req) {
		return
	}
	due, err := time.Parse(time.RFC3339, req.DueAt)
	if err != nil {
		writeError(w, domain.ErrValidation)
		return
	}
	res, err := s.app.UpdateMilestone(r.Context(), chi.URLParam(r, "project_id"), chi.URLParam(r, "milestone_id"), claims(r).UserID, repositories.MilestoneInput{
		Title: req.Title, Description: req.Description, DueAt: due, AmountLimit: req.AmountLimit, Position: req.Position,
	})
	writeResult(w, res, err, http.StatusOK)
}

func (s *Server) addReward(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title            string  `json:"title"`
		Description      string  `json:"description"`
		MinAmount        float64 `json:"min_amount"`
		LimitCount       *int    `json:"limit_count"`
		DeliveryEstimate *string `json:"delivery_estimate"`
	}
	if !decode(w, r, &req) {
		return
	}
	var delivery *time.Time
	if req.DeliveryEstimate != nil && *req.DeliveryEstimate != "" {
		parsed, err := time.Parse("2006-01-02", *req.DeliveryEstimate)
		if err != nil {
			writeError(w, domain.ErrValidation)
			return
		}
		delivery = &parsed
	}
	res, err := s.app.AddReward(r.Context(), chi.URLParam(r, "project_id"), claims(r).UserID, repositories.RewardInput{
		Title: req.Title, Description: req.Description, MinAmount: req.MinAmount, LimitCount: req.LimitCount, DeliveryEstimate: delivery,
	})
	writeResult(w, res, err, http.StatusCreated)
}

func (s *Server) createPledge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Amount   float64 `json:"amount"`
		RewardID *string `json:"reward_id"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := s.app.CreatePledge(r.Context(), chi.URLParam(r, "project_id"), claims(r).UserID, req.RewardID, req.Amount)
	writeResult(w, res, err, http.StatusCreated)
}

func (s *Server) mockCapture(w http.ResponseWriter, r *http.Request) {
	err := s.app.MockCapture(r.Context(), chi.URLParam(r, "payment_id"))
	writeResult(w, map[string]string{"status": "captured"}, err, http.StatusOK)
}

func (s *Server) mockFail(w http.ResponseWriter, r *http.Request) {
	err := s.app.MockFail(r.Context(), chi.URLParam(r, "payment_id"))
	writeResult(w, map[string]string{"status": "failed"}, err, http.StatusOK)
}

func (s *Server) paymentWebhook(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if !decode(w, r, &payload) {
		return
	}
	err := s.app.PaymentWebhook(r.Context(), payload)
	writeResult(w, map[string]string{"status": "ok"}, err, http.StatusOK)
}

func (s *Server) refundPledge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &req) {
		return
	}
	c := claims(r)
	res, err := s.app.RefundPledge(r.Context(), chi.URLParam(r, "pledge_id"), c.UserID, req.Reason, services.HasRole(c.Roles, domain.RoleAdmin))
	writeResult(w, res, err, http.StatusOK)
}

func (s *Server) submitMilestoneReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReportText string              `json:"report_text"`
		Files      []map[string]string `json:"files"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := s.app.SubmitMilestone(r.Context(), chi.URLParam(r, "milestone_id"), claims(r).UserID, req.ReportText, req.Files)
	writeResult(w, res, err, http.StatusCreated)
}

func (s *Server) adminProjectDecision(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Decision string `json:"decision"`
		Comment  string `json:"comment"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := s.app.AdminProjectDecision(r.Context(), chi.URLParam(r, "project_id"), claims(r).UserID, req.Decision)
	writeResult(w, res, err, http.StatusOK)
}

func (s *Server) adminMilestones(w http.ResponseWriter, r *http.Request) {
	res, err := s.app.ListMilestoneSubmissions(r.Context())
	writeResult(w, map[string]any{"items": res}, err, http.StatusOK)
}

func (s *Server) adminMilestoneReview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SubmissionID string `json:"submission_id"`
		Decision     string `json:"decision"`
		Comment      string `json:"comment"`
	}
	if !decode(w, r, &req) {
		return
	}
	err := s.app.ReviewMilestone(r.Context(), chi.URLParam(r, "milestone_id"), req.SubmissionID, claims(r).UserID, req.Decision, req.Comment)
	writeResult(w, map[string]string{"status": req.Decision}, err, http.StatusOK)
}

func (s *Server) addProjectUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := s.app.AddProjectUpdate(r.Context(), chi.URLParam(r, "project_id"), claims(r).UserID, req.Title, req.Content)
	writeResult(w, res, err, http.StatusCreated)
}

func (s *Server) blockUser(w http.ResponseWriter, r *http.Request) {
	err := s.app.SetUserBlocked(r.Context(), chi.URLParam(r, "user_id"), true)
	writeResult(w, map[string]bool{"is_blocked": true}, err, http.StatusOK)
}

func (s *Server) unblockUser(w http.ResponseWriter, r *http.Request) {
	err := s.app.SetUserBlocked(r.Context(), chi.URLParam(r, "user_id"), false)
	writeResult(w, map[string]bool{"is_blocked": false}, err, http.StatusOK)
}

func projectInputFromRequest(w http.ResponseWriter, r *http.Request) (repositories.ProjectInput, bool) {
	var req struct {
		Title            string  `json:"title"`
		ShortDescription string  `json:"short_description"`
		Description      string  `json:"description"`
		CategoryID       *int    `json:"category_id"`
		CampaignType     string  `json:"campaign_type"`
		Currency         string  `json:"currency"`
		GoalAmount       float64 `json:"goal_amount"`
		StartAt          *string `json:"start_at"`
		EndAt            *string `json:"end_at"`
	}
	if !decode(w, r, &req) {
		return repositories.ProjectInput{}, false
	}
	if req.Currency == "" {
		req.Currency = "RUB"
	}
	input := repositories.ProjectInput{
		Title: req.Title, ShortDescription: req.ShortDescription, Description: req.Description,
		CategoryID: req.CategoryID, CampaignType: req.CampaignType, Currency: req.Currency, GoalAmount: req.GoalAmount,
	}
	if req.StartAt != nil && *req.StartAt != "" {
		t, err := time.Parse(time.RFC3339, *req.StartAt)
		if err != nil {
			writeError(w, domain.ErrValidation)
			return repositories.ProjectInput{}, false
		}
		input.StartAt = &t
	}
	if req.EndAt != nil && *req.EndAt != "" {
		t, err := time.Parse(time.RFC3339, *req.EndAt)
		if err != nil {
			writeError(w, domain.ErrValidation)
			return repositories.ProjectInput{}, false
		}
		input.EndAt = &t
	}
	return input, true
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, domain.ErrValidation)
		return false
	}
	return true
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(header, "Bearer ")
}

func writeResult(w http.ResponseWriter, data any, err error, status int) {
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	switch {
	case errors.Is(err, domain.ErrValidation):
		status = http.StatusBadRequest
		code = "validation_error"
	case errors.Is(err, domain.ErrForbidden):
		status = http.StatusForbidden
		code = "forbidden"
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		code = "not_found"
	case errors.Is(err, domain.ErrInvalidStatus):
		status = http.StatusConflict
		code = "invalid_status"
	case errors.Is(err, domain.ErrInsufficientReservedFunds):
		status = http.StatusConflict
		code = "insufficient_reserved_funds"
	}
	log.Error().Err(err).Str("code", code).Msg("api error")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": code, "message": err.Error(), "request_id": uuid.NewString()})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func pageSizeOrDefault(value int) int {
	if value < 1 {
		return 20
	}
	return value
}
