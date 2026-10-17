package media

import "context"

// StoredSettings 存储配置（settings KV 持久化的 JSON 结构）。
type StoredSettings struct {
	Backend     string     `json:"backend"` // local | s3 | lsky
	MirrorLocal bool       `json:"mirror_local"`
	S3          S3Config   `json:"s3"`
	Lsky        LskyConfig `json:"lsky"`
}

// DefaultSettings 本地驱动默认配置。
func DefaultSettings() StoredSettings {
	return StoredSettings{Backend: "local"}
}

// BuildStorage 依据配置构造主驱动；local 返回 (nil, nil) 由调用方用 LocalStore 兜底。
func BuildStorage(ctx context.Context, st StoredSettings) (Storage, error) {
	switch st.Backend {
	case "s3":
		return NewS3Storage(ctx, st.S3)
	case "lsky":
		return NewLskyStorage(st.Lsky)
	default:
		return nil, nil
	}
}
