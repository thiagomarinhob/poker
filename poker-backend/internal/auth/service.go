package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/thiagomarinho/poker-backend/internal/user"
)

var (
	ErrEmailTaken   = errors.New("email already in use")
	ErrInvalidCreds = errors.New("invalid credentials")
	ErrRateLimited  = errors.New("too many login attempts, try again in 15 minutes")
	ErrInvalidToken = errors.New("invalid or expired token")
)

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type Service struct {
	queries    user.Querier
	redis      *redis.Client
	jwtSecret  string
	bcryptCost int
}

type Option func(*Service)

// WithBcryptCost overrides the default bcrypt cost of 12. Use bcrypt.MinCost in tests.
func WithBcryptCost(cost int) Option {
	return func(s *Service) { s.bcryptCost = cost }
}

func NewService(q user.Querier, rdb *redis.Client, jwtSecret string, opts ...Option) *Service {
	s := &Service{
		queries:    q,
		redis:      rdb,
		jwtSecret:  jwtSecret,
		bcryptCost: 12,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Service) Register(ctx context.Context, email, password string) (*AuthResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u, err := s.queries.CreateUser(ctx, user.CreateUserParams{
		Email:        email,
		PasswordHash: string(hash),
		Role:         "player",
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	return s.issueTokens(ctx, u.ID, u.Role)
}

func (s *Service) Login(ctx context.Context, email, password, ip string) (*AuthResponse, error) {
	if err := checkRateLimit(ctx, s.redis, ip); err != nil {
		return nil, err
	}

	u, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCreds
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCreds
	}

	return s.issueTokens(ctx, u.ID, u.Role)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	key := refreshKey(refreshToken)

	val, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, ErrInvalidToken
	}

	parts := strings.SplitN(val, "|", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}

	s.redis.Del(ctx, key) // rotate: old token is consumed immediately

	return s.issueTokens(ctx, userID, parts[1])
}

func (s *Service) Logout(ctx context.Context, refreshToken string) {
	s.redis.Del(ctx, refreshKey(refreshToken))
}

func (s *Service) issueTokens(ctx context.Context, userID uuid.UUID, role string) (*AuthResponse, error) {
	accessToken, err := generateAccessToken(userID, role, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	rt := newRefreshToken()
	val := userID.String() + "|" + role
	if err := s.redis.Set(ctx, refreshKey(rt), val, refreshTokenDuration).Err(); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: rt,
		TokenType:    "Bearer",
		ExpiresIn:    int(accessTokenDuration.Seconds()),
	}, nil
}
