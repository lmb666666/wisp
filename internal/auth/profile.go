package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lmb666666/wisp/internal/repo"
)

// Profile 个人资料（设置页展示用）。
type Profile struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	CreatedAt   int64  `json:"created_at"`
	Passkeys    int64  `json:"passkeys"`
}

var ErrCurrentPassword = errors.New("当前密码不正确")

func (s *Service) Profile(ctx context.Context, userID int64) (Profile, error) {
	u, err := s.q.GetUser(ctx, userID)
	if err != nil {
		return Profile{}, ErrUnauthorized
	}
	n, _ := s.q.CountPasskeysByUser(ctx, userID)
	return Profile{Username: u.Username, DisplayName: u.DisplayName, CreatedAt: u.CreatedAt, Passkeys: n}, nil
}

// UpdateDisplayName 保存昵称（空则回退为用户名展示）。
func (s *Service) UpdateDisplayName(ctx context.Context, userID int64, name string) (Profile, error) {
	name = strings.TrimSpace(name)
	if len([]rune(name)) > 64 {
		return Profile{}, errors.New("昵称最多 64 个字符")
	}
	if _, err := s.q.UpdateUserDisplayName(ctx, repo.UpdateUserDisplayNameParams{
		DisplayName: name, UpdatedAt: time.Now().Unix(), ID: userID,
	}); err != nil {
		return Profile{}, fmt.Errorf("update display name: %w", err)
	}
	return s.Profile(ctx, userID)
}

// ChangePassword 校验当前密码后更新，并吊销该用户除 keepSession 外的全部会话
// （被盗 Cookie 在改密后立即失效，当前设备保持登录）。
func (s *Service) ChangePassword(ctx context.Context, userID int64, current, next, keepSession string) error {
	if len(next) < 8 {
		return errors.New("新密码至少 8 位")
	}
	u, err := s.q.GetUser(ctx, userID)
	if err != nil {
		return ErrUnauthorized
	}
	ok, err := VerifyPassword(current, u.PasswordHash)
	if err != nil || !ok {
		return ErrCurrentPassword
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	if err := s.q.UpdateUserPasswordHash(ctx, repo.UpdateUserPasswordHashParams{
		PasswordHash: hash, UpdatedAt: time.Now().Unix(), ID: userID,
	}); err != nil {
		return err
	}
	if keepSession != "" {
		return s.q.DeleteSessionsByUserKeep(ctx, repo.DeleteSessionsByUserKeepParams{
			UserID: userID, KeepID: hashToken(keepSession),
		})
	}
	return nil
}
