// wisp-cli 是 Wisp 官方命令行示例客户端：演示 OpenAPI 的完整用法。
//
// 用法：
//
//	wisp-cli -url https://wisp.example.com -token wpat_xxx post "今天的心情 #日记"
//	wisp-cli -url https://wisp.example.com -token wpat_xxx list [-n 10]
//	wisp-cli -url https://wisp.example.com -token wpat_xxx delete <post-id>
//
// 令牌在 Web 端「访问令牌」页创建（需要 write 作用域）。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	baseURL = flag.String("url", "http://localhost:8080", "Wisp 站点地址")
	token   = flag.String("token", os.Getenv("WISP_TOKEN"), "PAT 令牌（或环境变量 WISP_TOKEN）")
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		usage()
	}

	switch args[0] {
	case "post":
		if len(args) < 2 {
			fatal("用法: wisp-cli post \"内容 #标签\"")
		}
		doPost(strings.Join(args[1:], " "))
	case "list":
		n := 10
		if len(args) >= 3 && args[1] == "-n" {
			_, _ = fmt.Sscanf(args[2], "%d", &n)
		}
		doList(n)
	case "delete":
		if len(args) < 2 {
			fatal("用法: wisp-cli delete <post-id>")
		}
		doDelete(args[1])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "wisp-cli — Wisp 官方命令行示例")
	fmt.Fprintln(os.Stderr, "  post <内容...>   发布说说")
	fmt.Fprintln(os.Stderr, "  list [-n N]      查看时间线")
	fmt.Fprintln(os.Stderr, "  delete <id>      删除说说")
	os.Exit(1)
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "错误:", msg)
	os.Exit(1)
}

type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func call(method, path string, body any) []byte {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, strings.TrimRight(*baseURL, "/")+path, rd)
	if err != nil {
		fatal(err.Error())
	}
	req.Header.Set("Authorization", "Bearer "+*token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		var ae apiError
		_ = json.Unmarshal(data, &ae)
		if ae.Error.Message != "" {
			fatal(ae.Error.Message)
		}
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, data))
	}
	return data
}

func doPost(content string) {
	data := call("POST", "/api/v1/posts", map[string]any{
		"content_md": content,
		"source":     "cli",
		"client":     "wisp-cli",
	})
	var p struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(data, &p)
	fmt.Println("已发布:", p.ID)
}

func doList(n int) {
	data := call("GET", fmt.Sprintf("/api/v1/posts?limit=%d", n), nil)
	var resp struct {
		Items []struct {
			ID      string   `json:"id"`
			Content string   `json:"content_md"`
			At      int64    `json:"published_at"`
			Tags    []string `json:"tags"`
		} `json:"items"`
	}
	_ = json.Unmarshal(data, &resp)
	for _, p := range resp.Items {
		first := strings.ReplaceAll(strings.SplitN(p.Content, "\n", 2)[0], "\r", "")
		if len([]rune(first)) > 50 {
			first = string([]rune(first)[:50]) + "…"
		}
		fmt.Printf("%s  %s  %s\n", time.Unix(p.At, 0).Format("01-02 15:04"), p.ID, first)
	}
}

func doDelete(id string) {
	call("DELETE", "/api/v1/posts/"+id, nil)
	fmt.Println("已删除:", id)
}
