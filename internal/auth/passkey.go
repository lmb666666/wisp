package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/lmb666666/wisp/internal/repo"
)

// wispUser 适配 go-webauthn 的 User 接口（单作者模型，ID 稳定派生自数据库主键）。
type wispUser struct {
	id    int64
	name  string
	creds []webauthn.Credential
}

func (u *wispUser) WebAuthnID() []byte                         { return []byte(fmt.Sprintf("wisp-user-%d", u.id)) }
func (u *wispUser) WebAuthnName() string                       { return u.name }
func (u *wispUser) WebAuthnDisplayName() string                { return u.name }
func (u *wispUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }
func (u *wispUser) WebAuthnIcon() string                       { return "" }

// WebAuthnService Passkey（发现式登录，无需输入用户名）。
// SessionData 存进程内存：重启后未完成的仪式作废（重新发起即可，无持久化必要）。
type WebAuthnService struct {
	wa       *webauthn.WebAuthn
	q        repo.Querier
	sessions sync.Map // sessionID -> ceremonyEntry
}

// ceremonyEntry 仪式数据 + 过期时间：begin 不需要认证，必须有 TTL 防内存堆积。
type ceremonyEntry struct {
	data *webauthn.SessionData
	exp  time.Time
}

// ceremonyTTL 未完成仪式的存活时间。
const ceremonyTTL = 5 * time.Minute

func NewWebAuthnService(q repo.Querier, publicURL string) (*WebAuthnService, error) {
	u, err := url.Parse(publicURL)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("PublicURL 无法派生 RPID: %w", err)
	}
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Wisp",
		RPID:          u.Hostname(),
		RPOrigins:     []string{strings.TrimRight(publicURL, "/")},
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn init: %w", err)
	}
	return &WebAuthnService{wa: wa, q: q}, nil
}

func (s *WebAuthnService) storeSession(sdata *webauthn.SessionData) string {
	id, err := randomToken(16)
	if err != nil {
		// 熵源失败时不能静默用空 id（会与他人碰撞/可预测）
		return ""
	}
	now := time.Now()
	s.sessions.Range(func(key, value any) bool {
		if e, ok := value.(*ceremonyEntry); ok && now.After(e.exp) {
			s.sessions.Delete(key)
		}
		return true
	})
	s.sessions.Store(id, &ceremonyEntry{data: sdata, exp: now.Add(ceremonyTTL)})
	return id
}

func (s *WebAuthnService) loadSession(id string) (webauthn.SessionData, error) {
	v, ok := s.sessions.LoadAndDelete(id)
	if !ok {
		return webauthn.SessionData{}, fmt.Errorf("仪式已过期，请重新发起")
	}
	e, _ := v.(*ceremonyEntry)
	if e == nil || e.data == nil {
		return webauthn.SessionData{}, fmt.Errorf("仪式数据无效")
	}
	return *e.data, nil
}

// loadWispUser 从数据库装配 webauthn.User（含全部已注册凭证）。
func (s *WebAuthnService) loadWispUser(ctx context.Context, user repo.User) (*wispUser, error) {
	rows, err := s.q.ListPasskeysByUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	creds := make([]webauthn.Credential, 0, len(rows))
	for _, r := range rows {
		cred := webauthn.Credential{
			ID:              []byte(r.ID),
			PublicKey:       r.PublicKey,
			AttestationType: r.Attestation,
			Authenticator:   webauthn.Authenticator{SignCount: uint32(r.SignCount)},
		}
		for _, t := range strings.Split(r.Transports, ",") {
			if t != "" {
				cred.Transport = append(cred.Transport, protocol.AuthenticatorTransport(t))
			}
		}
		creds = append(creds, cred)
	}
	return &wispUser{id: user.ID, name: user.Username, creds: creds}, nil
}

// BeginRegistration 已登录用户注册新 Passkey。返回给前端的创建参数 + 仪式 ID。
func (s *WebAuthnService) BeginRegistration(ctx context.Context, user repo.User) (any, string, error) {
	wu, err := s.loadWispUser(ctx, user)
	if err != nil {
		return nil, "", err
	}
	options, sdata, err := s.wa.BeginRegistration(wu,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired, // 发现式登录的前提
			UserVerification: protocol.VerificationPreferred,
		}),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
	)
	if err != nil {
		return nil, "", err
	}
	return options, s.storeSession(sdata), nil
}

// FinishRegistration 完成注册并落库。
func (s *WebAuthnService) FinishRegistration(ctx context.Context, user repo.User, sessionID string, r *http.Request) error {
	wu, err := s.loadWispUser(ctx, user)
	if err != nil {
		return err
	}
	sdata, err := s.loadSession(sessionID)
	if err != nil {
		return err
	}
	cred, err := s.wa.FinishRegistration(wu, sdata, r)
	if err != nil {
		return fmt.Errorf("注册校验失败: %w", err)
	}
	transports := make([]string, 0, len(cred.Transport))
	for _, t := range cred.Transport {
		transports = append(transports, string(t))
	}
	return s.q.CreatePasskey(ctx, repo.CreatePasskeyParams{
		ID:          base64.RawURLEncoding.EncodeToString(cred.ID),
		UserID:      user.ID,
		PublicKey:   cred.PublicKey,
		Attestation: cred.AttestationType,
		Transports:  strings.Join(transports, ","),
		SignCount:   int64(cred.Authenticator.SignCount),
		CreatedAt:   time.Now().Unix(),
	})
}

// BeginLogin 发起发现式登录（无需用户名）。
func (s *WebAuthnService) BeginLogin() (any, string, error) {
	options, sdata, err := s.wa.BeginDiscoverableLogin(
		webauthn.WithUserVerification(protocol.VerificationPreferred),
	)
	if err != nil {
		return nil, "", err
	}
	return options, s.storeSession(sdata), nil
}

// FinishLogin 完成发现式登录：按凭证找到用户，校验签名，返回会话令牌。
func (s *WebAuthnService) FinishLogin(sessionID string, r *http.Request) (repo.User, string, error) {
	sdata, err := s.loadSession(sessionID)
	if err != nil {
		return repo.User{}, "", err
	}
	_, cred, err := s.wa.FinishPasskeyLogin(s.discoverableHandler, sdata, r)
	if err != nil {
		return repo.User{}, "", fmt.Errorf("登录校验失败: %w", err)
	}
	credID := base64.RawURLEncoding.EncodeToString(cred.ID)
	_ = s.q.UpdatePasskeySignCount(context.Background(), repo.UpdatePasskeySignCountParams{
		SignCount: int64(cred.Authenticator.SignCount), ID: credID,
	})
	// 凭证 → 用户
	user, err := s.userByPasskeyID(credID)
	if err != nil {
		return repo.User{}, "", err
	}
	plain, err := s.createSessionFor(user, r.UserAgent())
	if err != nil {
		return repo.User{}, "", err
	}
	return user, plain, nil
}

func (s *WebAuthnService) discoverableHandler(rawID, userHandle []byte) (webauthn.User, error) {
	user, err := s.userByPasskeyID(base64.RawURLEncoding.EncodeToString(rawID))
	if err != nil {
		return nil, err
	}
	wu, err := s.loadWispUser(context.Background(), user)
	if err != nil {
		return nil, err
	}
	// 只返回被断言的那把凭证，避免签名计数误判
	filtered := &wispUser{id: wu.id, name: wu.name}
	for _, c := range wu.creds {
		if strings.EqualFold(base64.RawURLEncoding.EncodeToString(c.ID), base64.RawURLEncoding.EncodeToString(rawID)) {
			filtered.creds = append(filtered.creds, c)
		}
	}
	return filtered, nil
}

func (s *WebAuthnService) userByPasskeyID(credID string) (repo.User, error) {
	row, err := s.q.GetPasskey(context.Background(), credID)
	if err != nil {
		return repo.User{}, fmt.Errorf("未知的 Passkey")
	}
	user, err := s.q.GetUser(context.Background(), row.UserID)
	if err != nil {
		return repo.User{}, ErrUnauthorized
	}
	return user, nil
}

func (s *WebAuthnService) createSessionFor(user repo.User, userAgent string) (string, error) {
	plain, err := randomToken(32)
	if err != nil {
		return "", err
	}
	now := time.Now()
	if err := s.q.CreateSession(context.Background(), repo.CreateSessionParams{
		ID:        hashToken(plain),
		UserID:    user.ID,
		UserAgent: userAgent,
		ExpiresAt: now.Add(sessionTTL).Unix(),
		CreatedAt: now.Unix(),
	}); err != nil {
		return "", err
	}
	return plain, nil
}
