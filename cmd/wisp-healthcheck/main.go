// wisp-healthcheck：容器 HEALTHCHECK 用的极简探针。
// distroless 运行镜像没有 shell/curl，主程序又拒绝一切参数，
// 因此单独提供一个只做 GET /healthz 的小二进制。
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	port := os.Getenv("WISP_PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/healthz", port))
	if err != nil {
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
