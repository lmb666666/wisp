package auth

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	wispdb "github.com/lmb666666/wisp/internal/db"
	"github.com/lmb666666/wisp/internal/repo"
)

func newTestAuthService(t *testing.T) (*Service, repo.Querier) {
	t.Helper()
	dir := t.TempDir()
	database, err := wispdb.Open(filepath.Join(dir, "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := wispdb.Migrate(database, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}
	q := repo.New(database)
	return NewService(q), q
}

func createTestUser(t *testing.T, q repo.Querier, username, password string, owner bool) repo.User {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	isOwner := int64(0)
	if owner {
		isOwner = 1
	}
	user, err := q.CreateUser(context.Background(), repo.CreateUserParams{
		Username: username, PasswordHash: hash, DisplayName: username,
		IsOwner: isOwner, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash = %q", hash)
	}
	if ok, err := VerifyPassword("correct horse battery staple", hash); err != nil || !ok {
		t.Fatalf("正确密码校验失败: %v %v", ok, err)
	}
	if ok, err := VerifyPassword("wrong", hash); err != nil || ok {
		t.Fatalf("错误密码被接受: %v %v", ok, err)
	}

	// 随机 salt：同一密码两次哈希必须不同。
	again, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if again == hash {
		t.Fatal("同一密码生成了相同哈希（salt 未随机化）")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	for _, encoded := range []string{"", "plaintext", "$argon2i$v=19$m=1,t=1,p=1$x$y", "$argon2id$v=99$m=1,t=1,p=1$x$y"} {
		if ok, err := VerifyPassword("pw", encoded); err == nil || ok {
			t.Fatalf("格式非法应报错: %q => %v %v", encoded, ok, err)
		}
	}
}

func TestLoginSessionLifecycle(t *testing.T) {
	svc, q := newTestAuthService(t)
	ctx := context.Background()
	user := createTestUser(t, q, "owner", "password123", true)

	if _, err := svc.Login(ctx, "owner", "wrong", "ua"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("错误密码 = %v", err)
	}
	if _, err := svc.Login(ctx, "nobody", "password123", "ua"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("不存在用户 = %v", err)
	}

	token, err := svc.Login(ctx, "owner", "password123", "ua")
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.UserBySession(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != user.ID {
		t.Fatalf("session 用户 = %d, want %d", got.ID, user.ID)
	}

	if err := svc.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UserBySession(ctx, token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("登出后会话 = %v, want ErrUnauthorized", err)
	}
}

func TestCreateTokenNormalizesScopesAndResolvesPlaintext(t *testing.T) {
	svc, _ := newTestAuthService(t)
	ctx := context.Background()

	plain, row, err := svc.CreateToken(ctx, "cli", "admin,write")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(plain, TokenPrefix) {
		t.Fatalf("token = %q, want %s 前缀", plain, TokenPrefix)
	}
	if row.Scopes != "read" {
		t.Fatalf("非法 scope 未回退 read: %q", row.Scopes)
	}

	got, err := svc.TokenByPlain(ctx, plain)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != row.ID {
		t.Fatalf("解析 token = %s, want %s", got.ID, row.ID)
	}
	if _, err := svc.TokenByPlain(ctx, "wpat_does-not-exist"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("无效 token = %v", err)
	}
}

func TestScopes(t *testing.T) {
	for _, scopes := range []string{"read", "write", "read,write"} {
		if !ValidScopes(scopes) {
			t.Fatalf("%q 应为合法 scope", scopes)
		}
	}
	for _, scopes := range []string{"", "admin", "read, admin"} {
		if ValidScopes(scopes) {
			t.Fatalf("%q 应为非法 scope", scopes)
		}
	}
	if !HasScope("read,write", "write") || !HasScope("read, write", "write") {
		t.Fatal("HasScope 应支持逗号与空格")
	}
	if HasScope("read", "write") {
		t.Fatal("HasScope 误判")
	}
}

func TestOwner(t *testing.T) {
	svc, q := newTestAuthService(t)
	ctx := context.Background()
	user := createTestUser(t, q, "owner", "password123", true)

	got, err := svc.Owner(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != user.ID || got.IsOwner != 1 {
		t.Fatalf("owner = %+v", got)
	}
}
