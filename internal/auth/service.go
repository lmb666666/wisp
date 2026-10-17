package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/util"
)

const (
	// SessionCookie 会话 Cookie 名。
	SessionCookie = "wisp_session"
	sessionTTL    = 30 * 24 * time.Hour
	// TokenPrefix PAT 明文前缀，便于识别与扫描泄漏。
	TokenPrefix = "wpat_"
)

// Service 基于数据库会话表的认证服务（单作者模型，服务端会话可随时吊销）。
type Service struct {
	q repo.Querier
}

func NewService(q repo.Querier) *Service { return &Service{q: q} }

func hashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// Login 校验用户名密码，创建会话，返回明文会话令牌（写入 Cookie）。
func (s *Service) Login(ctx context.Context, username, password, userAgent string) (string, error) {
	user, err := s.q.GetUserByUsername(ctx, username)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	ok, err := VerifyPassword(password, user.PasswordHash)
	if err != nil || !ok {
		return "", ErrInvalidCredentials
	}
	plain, err := randomToken(32)
	if err != nil {
		return "", fmt.Errorf("generate session: %w", err)
	}
	now := time.Now()
	if err := s.q.CreateSession(ctx, repo.CreateSessionParams{
		ID:        hashToken(plain),
		UserID:    user.ID,
		UserAgent: userAgent,
		ExpiresAt: now.Add(sessionTTL).Unix(),
		CreatedAt: now.Unix(),
	}); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return plain, nil
}

// UserBySession 用明文会话令牌换取用户。
func (s *Service) UserBySession(ctx context.Context, plainToken string) (repo.User, error) {
	user, err := s.q.GetUserBySession(ctx, repo.GetUserBySessionParams{
		ID:        hashToken(plainToken),
		ExpiresAt: time.Now().Unix(),
	})
	if err != nil {
		return repo.User{}, ErrUnauthorized
	}
	return user, nil
}

// Logout 吊销会话。
func (s *Service) Logout(ctx context.Context, plainToken string) error {
	return s.q.DeleteSession(ctx, hashToken(plainToken))
}

// CreateToken 新建 PAT，返回明文（仅此一次）与落库记录。
func (s *Service) CreateToken(ctx context.Context, name, scopes string) (string, repo.ApiToken, error) {
	plain, err := randomToken(32)
	if err != nil {
		return "", repo.ApiToken{}, fmt.Errorf("generate token: %w", err)
	}
	if !ValidScopes(scopes) {
		scopes = "read"
	}
	row, err := s.q.CreateToken(ctx, repo.CreateTokenParams{
		ID:        util.NewULID(),
		Name:      name,
		TokenHash: hashToken(plain),
		Scopes:    scopes,
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		return "", repo.ApiToken{}, fmt.Errorf("create token: %w", err)
	}
	return TokenPrefix + plain, row, nil
}

// TokenByPlain 用明文 PAT 换取令牌记录（顺带记录 last_used_at）。
func (s *Service) TokenByPlain(ctx context.Context, plain string) (repo.ApiToken, error) {
	row, err := s.q.GetTokenByHash(ctx, hashToken(strings.TrimPrefix(plain, TokenPrefix)))
	if err != nil {
		return repo.ApiToken{}, ErrUnauthorized
	}
	_ = s.q.TouchToken(ctx, repo.TouchTokenParams{ID: row.ID, LastUsedAt: util.NullInt64(time.Now())})
	return row, nil
}

func (s *Service) ListTokens(ctx context.Context) ([]repo.ListTokensRow, error) {
	return s.q.ListTokens(ctx)
}

// Owner 返回站点所有者（单作者模型，PAT 归属于 owner）。
func (s *Service) Owner(ctx context.Context) (repo.User, error) {
	user, err := s.q.GetOwnerUser(ctx)
	if err != nil {
		return repo.User{}, ErrUnauthorized
	}
	return user, nil
}

func (s *Service) DeleteToken(ctx context.Context, id string) error {
	return s.q.DeleteToken(ctx, id)
}

// ValidScopes 只接受 read / write / "read,write"。
func ValidScopes(s string) bool {
	return s == "read" || s == "write" || s == "read,write"
}

// HasScope 判断令牌是否具有某权限。
func HasScope(scopes, want string) bool {
	for _, p := range strings.Split(scopes, ",") {
		if strings.TrimSpace(p) == want {
			return true
		}
	}
	return false
}
