// Package config 从环境变量（前缀 WISP_）加载配置。
package config

import (
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Port          int           `env:"PORT" envDefault:"8080"`
	DataDir       string        `env:"DATA_DIR" envDefault:"./data"`
	DBPath        string        `env:"DB_PATH" envDefault:""`
	LogLevel      string        `env:"LOG_LEVEL" envDefault:"info"`
	EnableSwagger bool          `env:"ENABLE_SWAGGER" envDefault:"false"`
	PublicURL     string        `env:"PUBLIC_URL" envDefault:"http://localhost:8080"`
	APIRateLimit  float64       `env:"API_RATE_LIMIT" envDefault:"20"`   // 每秒请求数（按 IP）
	AuthRateLimit float64       `env:"AUTH_RATE_LIMIT" envDefault:"1"`   // 登录/首启每秒尝试数（按 IP），Argon2 内存开销高
	CORSOrigins   []string      `env:"CORS_ORIGINS" envSeparator:","`    // 允许跨域访问 API 的来源（默认同源，无需配置）
	MetingAPI     string        `env:"METING_API" envDefault:""`         // Meting 音乐解析端点（自托管，留空禁用音乐卡）
	GeocodeURL    string        `env:"GEOCODE_URL" envDefault:""`        // Nominatim 兼容端点；留空禁用地点搜索/反查
	TrustProxy    bool          `env:"TRUST_PROXIES" envDefault:"false"` // 置 true 时从 X-Forwarded-For 取客户端 IP（仅在可信反代后开启）
	ReadTimeout   time.Duration `env:"READ_TIMEOUT" envDefault:"10m"`    // 覆盖整个请求体；上传/导入需要足够时间
	WriteTimeout  time.Duration `env:"WRITE_TIMEOUT" envDefault:"5m"`
}

func Load() (Config, error) {
	var cfg Config
	if err := env.ParseWithOptions(&cfg, env.Options{Prefix: "WISP_"}); err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}
	if cfg.DBPath == "" {
		cfg.DBPath = filepath.Join(cfg.DataDir, "wisp.db")
	}
	u, err := url.Parse(cfg.PublicURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return Config{}, fmt.Errorf("WISP_PUBLIC_URL 必须是完整的 http(s) 地址，当前为 %q", cfg.PublicURL)
	}
	return cfg, nil
}
