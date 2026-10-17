// Package config 从环境变量（前缀 WISP_）加载配置。
package config

import (
	"fmt"
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
	APIRateLimit  float64       `env:"API_RATE_LIMIT" envDefault:"20"` // 每秒请求数（按 IP）
	CORSOrigins   []string      `env:"CORS_ORIGINS" envSeparator:","`  // 允许跨域访问 API 的来源（默认同源，无需配置）
	MetingAPI     string        `env:"METING_API" envDefault:""`       // Meting 音乐解析端点（自托管，留空禁用音乐卡）
	GeocodeURL    string        `env:"GEOCODE_URL" envDefault:""`      // Nominatim 兼容端点；留空禁用地点搜索/反查
	ReadTimeout   time.Duration `env:"READ_TIMEOUT" envDefault:"15s"`
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
	return cfg, nil
}
