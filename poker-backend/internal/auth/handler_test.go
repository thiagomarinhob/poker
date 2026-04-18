package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/thiagomarinho/poker-backend/internal/auth"
	"github.com/thiagomarinho/poker-backend/internal/user"
)

const testSecret = "super-secret-key-for-testing-only!!"

// ── fake querier ──────────────────────────────────────────────────────────────

type fakeQuerier struct {
	mu    sync.RWMutex
	users map[string]user.User // keyed by email
}

func newFakeQuerier() *fakeQuerier {
	return &fakeQuerier{users: make(map[string]user.User)}
}

func (f *fakeQuerier) insertUser(u user.User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users[u.Email] = u
}

func (f *fakeQuerier) CreateUser(_ context.Context, arg user.CreateUserParams) (user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.users[arg.Email]; exists {
		return user.User{}, &pgconn.PgError{Code: "23505"}
	}
	u := user.User{
		ID:           uuid.New(),
		Email:        arg.Email,
		PasswordHash: arg.PasswordHash,
		Role:         arg.Role,
	}
	f.users[arg.Email] = u
	return u, nil
}

func (f *fakeQuerier) GetUserByEmail(_ context.Context, email string) (user.User, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	u, ok := f.users[email]
	if !ok {
		return user.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (f *fakeQuerier) GetUserByID(_ context.Context, id uuid.UUID) (user.User, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return user.User{}, pgx.ErrNoRows
}

func (f *fakeQuerier) ApplyChipsDelta(_ context.Context, arg user.ApplyChipsDeltaParams) (user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for email, u := range f.users {
		if u.ID == arg.ID {
			bal := u.ChipsBalance + arg.Amount
			if bal < 0 {
				return user.User{}, &pgconn.PgError{Code: "P0001"}
			}
			u.ChipsBalance = bal
			_ = arg.HandID
			_ = arg.Metadata
			_ = arg.Type
			f.users[email] = u
			return u, nil
		}
	}
	return user.User{}, pgx.ErrNoRows
}

func (f *fakeQuerier) ListUsersForAdmin(_ context.Context, _ user.ListUsersForAdminParams) ([]user.ListUsersForAdminRow, error) {
	return nil, nil
}

func (f *fakeQuerier) SearchUsersForAdmin(_ context.Context, _ user.SearchUsersForAdminParams) ([]user.SearchUsersForAdminRow, error) {
	return nil, nil
}

// ── test env ──────────────────────────────────────────────────────────────────

type testEnv struct {
	router  *gin.Engine
	querier *fakeQuerier
	mini    *miniredis.Miniredis
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	fq := newFakeQuerier()
	svc := auth.NewService(fq, rdb, testSecret, auth.WithBcryptCost(bcrypt.MinCost))
	h := auth.NewHandler(svc)

	r := gin.New()
	api := r.Group("/api")
	h.RegisterRoutes(api)

	protected := api.Group("/protected")
	protected.Use(auth.AuthRequired(testSecret))
	protected.GET("/me", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": c.GetString(auth.CtxKeyUserID)})
	})
	protected.GET("/admin", auth.AdminOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	return &testEnv{router: r, querier: fq, mini: mr}
}

func post(t *testing.T, r *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatalf("encode body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func get(t *testing.T, r *gin.Engine, path, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeTokens(t *testing.T, w *httptest.ResponseRecorder) auth.AuthResponse {
	t.Helper()
	var resp auth.AuthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// ── register ──────────────────────────────────────────────────────────────────

func TestRegister_Success(t *testing.T) {
	env := newTestEnv(t)
	w := post(t, env.router, "/api/auth/register", map[string]string{
		"email": "alice@example.com", "password": "password123",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", w.Code, w.Body)
	}
	resp := decodeTokens(t, w)
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Error("expected both tokens in response")
	}
	if resp.ExpiresIn != 900 {
		t.Errorf("expected expires_in=900, got %d", resp.ExpiresIn)
	}
}

func TestRegister_InvalidEmail(t *testing.T) {
	env := newTestEnv(t)
	w := post(t, env.router, "/api/auth/register", map[string]string{
		"email": "not-an-email", "password": "password123",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	env := newTestEnv(t)
	w := post(t, env.router, "/api/auth/register", map[string]string{
		"email": "bob@example.com", "password": "short",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestRegister_MissingFields(t *testing.T) {
	env := newTestEnv(t)
	w := post(t, env.router, "/api/auth/register", map[string]string{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	env := newTestEnv(t)
	payload := map[string]string{"email": "dup@example.com", "password": "password123"}
	post(t, env.router, "/api/auth/register", payload)

	w := post(t, env.router, "/api/auth/register", payload)
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d: %s", w.Code, w.Body)
	}
}

// ── login ─────────────────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	env := newTestEnv(t)
	post(t, env.router, "/api/auth/register", map[string]string{
		"email": "login@example.com", "password": "mypassword",
	})

	w := post(t, env.router, "/api/auth/login", map[string]string{
		"email": "login@example.com", "password": "mypassword",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body)
	}
	resp := decodeTokens(t, w)
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Error("expected both tokens in login response")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	env := newTestEnv(t)
	post(t, env.router, "/api/auth/register", map[string]string{
		"email": "wrong@example.com", "password": "correctpassword",
	})

	w := post(t, env.router, "/api/auth/login", map[string]string{
		"email": "wrong@example.com", "password": "wrongpassword",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	env := newTestEnv(t)
	w := post(t, env.router, "/api/auth/login", map[string]string{
		"email": "ghost@example.com", "password": "password123",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestLogin_RateLimit(t *testing.T) {
	env := newTestEnv(t)
	payload := map[string]string{"email": "rate@example.com", "password": "wrongpassword"}

	const limit = 5 // must match ratelimit.go maxLoginAttempts
	for i := 0; i < limit; i++ {
		post(t, env.router, "/api/auth/login", payload)
	}

	w := post(t, env.router, "/api/auth/login", payload)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429 after %d attempts, got %d", limit, w.Code)
	}
}

// ── refresh ───────────────────────────────────────────────────────────────────

func TestRefresh_Success(t *testing.T) {
	env := newTestEnv(t)
	reg := decodeTokens(t, post(t, env.router, "/api/auth/register", map[string]string{
		"email": "refresh@example.com", "password": "password123",
	}))

	w := post(t, env.router, "/api/auth/refresh", map[string]string{
		"refresh_token": reg.RefreshToken,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body)
	}
	resp := decodeTokens(t, w)
	if resp.AccessToken == "" {
		t.Error("expected new access token")
	}
}

func TestRefresh_TokenRotation(t *testing.T) {
	env := newTestEnv(t)
	reg := decodeTokens(t, post(t, env.router, "/api/auth/register", map[string]string{
		"email": "rotate@example.com", "password": "password123",
	}))

	// Use the refresh token once — it should be consumed.
	post(t, env.router, "/api/auth/refresh", map[string]string{"refresh_token": reg.RefreshToken})

	// Reusing the same token must fail.
	w := post(t, env.router, "/api/auth/refresh", map[string]string{"refresh_token": reg.RefreshToken})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for reused token, got %d", w.Code)
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	env := newTestEnv(t)
	w := post(t, env.router, "/api/auth/refresh", map[string]string{
		"refresh_token": "00000000-0000-0000-0000-000000000000",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

// ── logout ────────────────────────────────────────────────────────────────────

func TestLogout_InvalidatesRefreshToken(t *testing.T) {
	env := newTestEnv(t)
	reg := decodeTokens(t, post(t, env.router, "/api/auth/register", map[string]string{
		"email": "logout@example.com", "password": "password123",
	}))

	w := post(t, env.router, "/api/auth/logout", map[string]string{
		"refresh_token": reg.RefreshToken,
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", w.Code)
	}

	// Token must be invalid after logout.
	w2 := post(t, env.router, "/api/auth/refresh", map[string]string{
		"refresh_token": reg.RefreshToken,
	})
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 after logout, got %d", w2.Code)
	}
}

func TestLogout_UnknownToken(t *testing.T) {
	env := newTestEnv(t)
	// Logout is idempotent — unknown token still returns 204.
	w := post(t, env.router, "/api/auth/logout", map[string]string{
		"refresh_token": "00000000-0000-0000-0000-000000000000",
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", w.Code)
	}
}

// ── AuthRequired middleware ───────────────────────────────────────────────────

func TestAuthRequired_ValidToken(t *testing.T) {
	env := newTestEnv(t)
	reg := decodeTokens(t, post(t, env.router, "/api/auth/register", map[string]string{
		"email": "mw@example.com", "password": "password123",
	}))

	w := get(t, env.router, "/api/protected/me", reg.AccessToken)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body)
	}
}

func TestAuthRequired_MissingHeader(t *testing.T) {
	env := newTestEnv(t)
	w := get(t, env.router, "/api/protected/me", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestAuthRequired_InvalidToken(t *testing.T) {
	env := newTestEnv(t)
	w := get(t, env.router, "/api/protected/me", "invalid.token.value")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestAuthRequired_WrongSecret(t *testing.T) {
	env := newTestEnv(t)
	// Token signed with a different secret must be rejected.
	otherSvc := auth.NewService(newFakeQuerier(),
		redis.NewClient(&redis.Options{Addr: env.mini.Addr()}),
		"a-completely-different-secret-key!!",
		auth.WithBcryptCost(bcrypt.MinCost),
	)
	otherH := auth.NewHandler(otherSvc)
	r2 := gin.New()
	otherH.RegisterRoutes(r2.Group("/api"))

	reg := decodeTokens(t, post(t, r2, "/api/auth/register", map[string]string{
		"email": "spy@example.com", "password": "password123",
	}))

	// Use token from other router against our router (different secret).
	w := get(t, env.router, "/api/protected/me", reg.AccessToken)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

// ── AdminOnly middleware ──────────────────────────────────────────────────────

func TestAdminOnly_PlayerForbidden(t *testing.T) {
	env := newTestEnv(t)
	reg := decodeTokens(t, post(t, env.router, "/api/auth/register", map[string]string{
		"email": "player@example.com", "password": "password123",
	}))

	w := get(t, env.router, "/api/protected/admin", reg.AccessToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
}

func TestAdminOnly_AdminAllowed(t *testing.T) {
	env := newTestEnv(t)

	hash, _ := bcrypt.GenerateFromPassword([]byte("adminpass"), bcrypt.MinCost)
	env.querier.insertUser(user.User{
		ID:           uuid.New(),
		Email:        "admin@example.com",
		PasswordHash: string(hash),
		Role:         "admin",
	})

	login := decodeTokens(t, post(t, env.router, "/api/auth/login", map[string]string{
		"email": "admin@example.com", "password": "adminpass",
	}))

	w := get(t, env.router, "/api/protected/admin", login.AccessToken)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body)
	}
}
