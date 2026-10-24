#!/usr/bin/env python3
"""Wisp 全功能验收矩阵：覆盖全部端点与核心语义（57 项断言）。

用法：
  1. 起一个全新实例（必须空库）：
     WISP_PORT=18200 WISP_PUBLIC_URL=http://localhost:18200 \
     WISP_DATA_DIR=/tmp/wisp-matrix WISP_API_RATE_LIMIT=1000 \
     WISP_ENABLE_SWAGGER=true ./wisp
  2. python3 scripts/full_matrix.py

退出码 0 = 全部通过。
"""
import hashlib
import hmac
import io
import json
import os
import struct
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile
import zlib
from http.server import BaseHTTPRequestHandler, HTTPServer

BASE = os.environ.get("WISP_URL", "http://localhost:18200")
RESULTS = []
COOKIE = None  # 手动回放：urllib 的 CookieJar 会拒收无点域名(localhost)的 Cookie


def check(name, ok, detail=""):
    RESULTS.append((name, ok, detail))
    print(("✓" if ok else "✗"), name, detail if not ok and detail else "")


def req(method, path, body=None, jar=None, token=None, headers=None):
    """jar 语义：字符串（如 "owner"）= 携带会话 Cookie；None = 游客，绝不带 Cookie。"""
    global COOKIE
    data = None
    h = dict(headers or {})
    if body is not None and not isinstance(body, bytes):
        data = json.dumps(body).encode()
        h.setdefault("Content-Type", "application/json")
    elif isinstance(body, bytes):
        data = body
    if token:
        h["Authorization"] = "Bearer " + token
    if COOKIE and jar:
        h["Cookie"] = COOKIE
    r = urllib.request.Request(BASE + path, data=data, method=method, headers=h)
    opener = urllib.request.build_opener()
    try:
        resp = opener.open(r, timeout=15)
        code, b, hdrs = resp.status, resp.read(), dict(resp.headers)
    except urllib.error.HTTPError as e:
        code, b, hdrs = e.code, e.read(), dict(e.headers)
    sc = hdrs.get("Set-Cookie")
    if sc and jar and not token:
        COOKIE = sc.split(";")[0]
    return (code, b), hdrs


def jreq(method, path, body=None, jar=None, token=None, headers=None):
    (code, b), _ = req(method, path, body, jar, token, headers)
    try:
        return code, json.loads(b)
    except Exception:
        return code, b.decode(errors="replace")


# ===== 0. 基础 =====
owner = "owner"
code, b = jreq("GET", "/healthz", jar=owner)
check("healthz", code == 200 and b.get("status") == "ok")
code, b = jreq("GET", "/api/v1/ping")
check("ping", code == 200 and b.get("pong") == "wisp")
code, b = jreq("GET", "/api/v1/auth/status", jar=owner)
check("auth.status 未初始化", code == 200 and b.get("initialized") is False)

# ===== 1. 认证 =====
code, b = jreq("POST", "/api/v1/auth/init",
               {"username": "liang", "password": "password123", "display_name": "Liang"}, jar=owner)
check("auth.init", code == 201 and b.get("username") == "liang", f"{code} {b}")
code, b = jreq("POST", "/api/v1/auth/init", {"username": "evil", "password": "password123"})
check("auth.init 幂等(409)", code == 409)
code, b = jreq("POST", "/api/v1/auth/init", {"username": "x", "password": "password123"})
check("auth.init 用户名校验(400)", code == 400)
code, b = jreq("GET", "/api/v1/auth/status", jar=owner)
check("auth.status 已初始化", code == 200 and b.get("initialized") is True and b.get("username") == "liang")
code, b = jreq("GET", "/api/v1/auth/me", jar=owner)
check("auth.me", code == 200 and b.get("username") == "liang" and b.get("is_owner") is True)
code, b = jreq("POST", "/api/v1/auth/login", {"username": "liang", "password": "wrongpass1"})
check("auth.login 错误密码(401)", code == 401)
code, b = jreq("GET", "/api/v1/auth/status")
check("auth.status 匿名不泄露用户名", code == 200 and "username" not in b)


# ===== 2. 附件 =====
def png(w=300, h=200):
    sig = b"\x89PNG\r\n\x1a\n"

    def ch(t, d):
        return struct.pack(">I", len(d)) + t + d + struct.pack(">I", zlib.crc32(t + d) & 0xFFFFFFFF)

    row = b"\x00" + bytes([(i * 7) % 256 for i in range(w * 3)])
    return (sig + ch(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0))
            + ch(b"IDAT", zlib.compress(row * h)) + ch(b"IEND", b""))


def upload(fname, content, ctype="image/png"):
    boundary = "----wispboundary123"
    body = (f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"{fname}\"\r\n"
            f"Content-Type: {ctype}\r\n\r\n").encode() + content + f"\r\n--{boundary}--\r\n".encode()
    h = {"Content-Type": f"multipart/form-data; boundary={boundary}"}
    if COOKIE:
        h["Cookie"] = COOKIE
    r = urllib.request.Request(BASE + "/api/v1/attachments", data=body, method="POST", headers=h)
    try:
        resp = urllib.request.build_opener().open(r, timeout=30)
        return resp.status, json.loads(resp.read())
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read())


code, att = upload("a.png", png())
check("附件上传（派生物）", code == 201 and att.get("kind") == "image"
      and att.get("width") == 300 and att.get("blurhash"), f"{code} {att}")
code, att2 = upload("b.png", png())
check("附件秒传", code == 201 and att2.get("id") == att.get("id"))
code, _ = jreq("GET", att["url"])
check("附件可访问", code == 200)
code, _ = jreq("GET", att["thumb_url"])
check("缩略图可访问", code == 200)
code, attf = upload("x.txt", b"hello wisp", "text/plain")
check("非图片附件(kind=file)", code == 201 and attf.get("kind") == "file")
code, _ = upload("big.png", b"\x00" * (33 << 20))
check("超限 413", code == 413)

# ===== 3. 帖子 CRUD + 可见性 + 游标 =====
code, p1 = jreq("POST", "/api/v1/posts",
                {"content_md": "第一条 **加粗** #hello #测试\n\n第二行", "attachment_ids": [att["id"]], "client": "matrix"},
                jar=owner)
check("发帖(渲染+标签+附件)", code == 201 and "<strong>" in p1.get("content_html", "")
      and sorted(p1.get("tags", [])) == sorted(["hello", "测试"]) and len(p1.get("attachments", [])) == 1)

# 媒体库：无主筛选 / 统计 / 删除保护
code, b = jreq("GET", "/api/v1/admin/media?orphan=1", jar=owner)
check("媒体库无主筛选", code == 200 and b.get("total") == 1
      and b["items"][0]["id"] == attf["id"], f"{code} {b}")
code, b = jreq("GET", "/api/v1/admin/media/stats", jar=owner)
check("媒体库统计", code == 200 and b.get("count", 0) >= 2 and b.get("orphan") == 1
      and b.get("bytes", 0) > 0, f"{code} {b}")
code, _ = jreq("DELETE", "/api/v1/admin/media/" + att["id"], jar=owner)
check("媒体库拒绝删除已绑定(409)", code == 409)
code, _ = jreq("DELETE", "/api/v1/admin/media/" + attf["id"], jar=owner)
check("媒体库删除无主(204)", code == 204)
code, p2 = jreq("POST", "/api/v1/posts", {"content_md": "时间线第二帖"}, jar=owner)
check("第二帖创建", code == 201)
code, p3 = jreq("POST", "/api/v1/posts", {"content_md": "补发的时间机器", "published_at": 1700000000}, jar=owner)
check("补发历史时间", code == 201 and p3.get("published_at") == 1700000000)
code, b = jreq("GET", "/api/v1/posts?limit=2", jar=owner)
check("时间线分页", code == 200 and len(b.get("items", [])) >= 2)
code, b = jreq("GET", "/api/v1/posts?limit=50")
check("游客时间线", code == 200 and all(i["visibility"] == "public" for i in b.get("items", [])))
code, _ = jreq("POST", "/api/v1/posts", {"content_md": "x", "visibility": "private"}, jar=owner)
check("私密发布已移除→400", code == 400)
code, b = jreq("PATCH", "/api/v1/posts/" + p1["id"], {"content_md": "编辑后 #新标签"}, jar=owner)
check("编辑+重建标签", code == 200 and "新标签" in b.get("tags", []))
code, _ = jreq("POST", "/api/v1/posts", {"content_md": "x"}, token="wpat_invalid")
check("无效 PAT→401", code == 401)

# ===== 4. PAT 作用域 =====
code, tok = jreq("POST", "/api/v1/auth/tokens", {"name": "reader", "scopes": "read"}, jar=owner)
check("PAT 创建", code == 201 and tok.get("token", "").startswith("wpat_"))
read_tok = tok["token"]
code, b = jreq("GET", "/api/v1/posts?limit=50", token=read_tok)
check("PAT read 可读", code == 200 and any("时间线第二帖" in i["content_md"] for i in b.get("items", [])))
code, _ = jreq("POST", "/api/v1/posts", {"content_md": "x"}, token=read_tok)
check("PAT read 不可写→403", code == 403)
code, tok2 = jreq("POST", "/api/v1/auth/tokens", {"name": "writer", "scopes": "write"}, jar=owner)
code, b = jreq("POST", "/api/v1/posts", {"content_md": "PAT 写入帖", "client": "matrix"}, token=tok2.get("token", ""))
check("PAT write 发帖(source=api)", code == 201 and b.get("source") == "api")
code, b = jreq("GET", "/api/v1/auth/tokens", jar=owner)
check("PAT 列表(脱敏)", code == 200 and all("token" not in i for i in b.get("items", [])))

# ===== 5. 幂等 =====
code1, i1 = jreq("POST", "/api/v1/posts", {"content_md": "幂等帖"}, jar=owner,
                 headers={"Idempotency-Key": "mx-1"})
code2, i2 = jreq("POST", "/api/v1/posts", {"content_md": "幂等帖"}, jar=owner,
                 headers={"Idempotency-Key": "mx-1"})
check("Idempotency-Key 重放", code1 == 201 and code2 == 200 and i1.get("id") == i2.get("id"))

# ===== 6. 搜索 =====
code, b = jreq("GET", "/api/v1/search?q=" + urllib.parse.quote("幂等帖"))
check("搜索 FTS5(3字)", code == 200 and len(b.get("items", [])) >= 1)
code, b = jreq("GET", "/api/v1/search?q=" + urllib.parse.quote("时间"))
check("搜索 LIKE(2字)", code == 200 and len(b.get("items", [])) >= 1)
code, b = jreq("GET", "/api/v1/search?q=" + urllib.parse.quote("不存在的词xyz"))
check("搜索无结果", code == 200 and len(b.get("items", [])) == 0)

# ===== 8. stats =====
year = time.localtime().tm_year
code, b = jreq("GET", f"/api/v1/stats/daily?year={year}", jar=owner)
check("stats 热力图", code == 200 and isinstance(b.get("items"), list))

# ===== 9. 标签 =====
code, b = jreq("GET", "/api/v1/tags")
names = [t["name"] for t in b.get("items", [])]
check("标签聚合", code == 200 and len(names) >= 1
      and all(isinstance(t.get("id"), int) and t.get("post_count", 0) > 0 for t in b.get("items", [])),
      f"{code} {names}")

# 标签管理：重命名 / 冲突 / 合并 / 只解绑删除
code, ptag = jreq("POST", "/api/v1/posts", {"content_md": "标签管理 #mgA #mgB"}, jar=owner)
tag_ids = {t["name"]: t["id"] for t in jreq("GET", "/api/v1/tags")[1].get("items", [])}
code, _ = jreq("PATCH", f"/api/v1/tags/{tag_ids['mgA']}", {"name": "mgC"}, jar=owner)
b = jreq("GET", "/api/v1/posts/" + ptag["id"], jar=owner)[1]
check("标签重命名", code == 200 and sorted(b.get("tags", [])) == ["mgB", "mgC"], f"{code} {b.get('tags')}")
code, _ = jreq("PATCH", f"/api/v1/tags/{tag_ids['mgA']}", {"name": "mgB"}, jar=owner)
check("标签重命名冲突→409", code == 409)
code, b = jreq("POST", "/api/v1/tags/merge", {"from_ids": [tag_ids["mgA"]], "into": "mgB"}, jar=owner)
b = jreq("GET", "/api/v1/posts/" + ptag["id"], jar=owner)[1]
check("标签合并去重", code == 200 and b.get("tags") == ["mgB"], f"{code} {b.get('tags')}")
code, _ = jreq("DELETE", f"/api/v1/tags/{tag_ids['mgB']}", jar=owner)
b = jreq("GET", "/api/v1/posts/" + ptag["id"], jar=owner)[1]
check("标签删除只解绑", code == 204 and b.get("tags") == [] and b.get("content_md", "") != "")

# 组合筛选：多标签 ALL × 日期范围
now = int(time.time())
code, pfil = jreq("POST", "/api/v1/posts",
                  {"content_md": "筛选帖 #fA #fB", "published_at": now - 1000}, jar=owner)
code, b = jreq("GET", f"/api/v1/posts?tags=fA,fB&to={now}", jar=owner)
check("组合筛选(双标签+日期)", code == 200 and any(i["id"] == pfil["id"] for i in b.get("items", [])),
      f"{code} {len(b.get('items', []))}")
code, b = jreq("GET", "/api/v1/posts?tags=%5B%22fA%22%2C%22fB%22%5D", jar=owner)
check("组合筛选(JSON 标签)", code == 200 and any(i["id"] == pfil["id"] for i in b.get("items", [])),
      f"{code} {len(b.get('items', []))}")
code, b = jreq("GET", "/api/v1/posts?tags=fA,no-such-tag", jar=owner)
check("多标签 ALL 语义", code == 200 and b.get("items") == [])
code, b = jreq("GET", f"/api/v1/posts?from={now + 60}", jar=owner)
check("日期下界(未来为空)", code == 200 and b.get("items") == [])
code, _ = jreq("GET", "/api/v1/posts?from=not-a-number", jar=owner)
check("非法筛选参数→400", code == 400)

# ===== 10. 导入（memos JSON + 幂等）=====
boundary = "----wispboundary456"
memos_body = json.dumps([{"content": "从 Memos 迁移 #迁移", "createdTs": 1754128800,
                          "visibility": "PUBLIC"}]).encode()
body = (f"--{boundary}\r\nContent-Disposition: form-data; name=\"source\"\r\n\r\nmemos\r\n"
        f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"m.json\"\r\n"
        f"Content-Type: application/json\r\n\r\n").encode() + memos_body + f"\r\n--{boundary}--\r\n".encode()
imp_headers = {"Content-Type": f"multipart/form-data; boundary={boundary}", "Cookie": COOKIE}

r = urllib.request.Request(BASE + "/api/v1/admin/import", data=body, method="POST", headers=imp_headers)
imp = json.loads(urllib.request.build_opener().open(r, timeout=60).read())
check("导入 memos", imp.get("imported") == 1, str(imp))
r = urllib.request.Request(BASE + "/api/v1/admin/import", data=body, method="POST", headers=imp_headers)
imp2 = json.loads(urllib.request.build_opener().open(r, timeout=60).read())
check("导入幂等", imp2.get("imported") == 0 and imp2.get("skipped") == 1, str(imp2))

# ===== 11. 存储设置 =====
code, b = jreq("GET", "/api/v1/admin/settings/storage", jar=owner)
check("存储设置 GET", code == 200 and b.get("backend") == "local")
code, _ = jreq("PUT", "/api/v1/admin/settings/storage",
               {"backend": "s3", "mirror_local": False,
                "s3": {"endpoint": "invalid.example", "bucket": "nope",
                       "access_key": "a", "secret_key": "b"}, "lsky": {}}, jar=owner)
check("非法 S3 配置→400", code == 400)
code, b = jreq("PUT", "/api/v1/admin/settings/storage",
               {"backend": "local", "mirror_local": False, "s3": {}, "lsky": {}}, jar=owner)
check("存储设置 PUT 回写", code == 200 and b.get("backend") == "local")

# ===== 13. Webhook（本地接收器 + HMAC 校验）=====
received = {}


class Hook(BaseHTTPRequestHandler):
    def do_POST(self):
        ln = int(self.headers.get("Content-Length", 0))
        received["body"] = self.rfile.read(ln)
        received["headers"] = dict(self.headers)
        self.send_response(200)
        self.end_headers()

    def log_message(self, *a):
        pass


hook_srv = HTTPServer(("127.0.0.1", 18201), Hook)
threading.Thread(target=hook_srv.serve_forever, daemon=True).start()
code, wh = jreq("POST", "/api/v1/admin/webhooks",
                {"url": "http://127.0.0.1:18201/hook", "events": "post.created,post.deleted"}, jar=owner)
check("webhook 注册", code == 201 and wh.get("secret", "").startswith("wsec_"))
jreq("POST", "/api/v1/posts", {"content_md": "webhook 触发帖"}, jar=owner)
deadline = time.time() + 5
while time.time() < deadline and "body" not in received:
    time.sleep(0.1)
sig_ok = False
if "body" in received:
    mac = hmac.new(wh["secret"].encode(), received["body"], hashlib.sha256)
    sig_ok = (received["headers"].get("X-Wisp-Signature") == "sha256=" + mac.hexdigest()
              and received["headers"].get("X-Wisp-Event") == "post.created")
check("webhook 送达+签名", sig_ok)

# ===== 14. 备份 =====
r = urllib.request.Request(BASE + "/api/v1/admin/backup", headers={"Cookie": COOKIE})
resp = urllib.request.build_opener().open(r, timeout=30)
zf = zipfile.ZipFile(io.BytesIO(resp.read()))
check("备份 zip", resp.status == 200 and "wisp.db" in zf.namelist())

# 自动备份：默认设置 / 保存 / 立即运行 / 列表
code, b = jreq("GET", "/api/v1/admin/settings/backup", jar=owner)
check("备份设置默认值", code == 200 and b["settings"]["enabled"] is True
      and b["settings"]["interval_hours"] == 24 and b["settings"]["retention"] == 7,
      f"{code} {b}")
code, b = jreq("PUT", "/api/v1/admin/settings/backup",
               {"enabled": True, "interval_hours": 12, "retention": 3}, jar=owner)
check("备份设置保存", code == 200 and b["interval_hours"] == 12 and b["retention"] == 3,
      f"{code} {b}")
code, b = jreq("POST", "/api/v1/admin/backup/run", None, jar=owner)
check("立即备份", code == 200 and b.get("name", "").startswith("scheduled-")
      and b.get("size", 0) > 0, f"{code} {b}")
code, b = jreq("GET", "/api/v1/admin/settings/backup", jar=owner)
check("备份列表/上次时间", code == 200 and b["settings"]["last_run_at"] > 0
      and len(b.get("backups", [])) >= 1, f"{code} {b}")

# ===== 14b. 站点概览 =====
code, b = jreq("GET", "/api/v1/admin/overview", jar=owner)
code2, b2 = jreq("GET", "/api/v1/admin/media/stats", jar=owner)
check("概览统计", code == 200 and b["posts"]["total"] >= 1
      and b["attachments"]["bytes"] == b2.get("bytes") and b["db_bytes"] > 0
      and b["version"], f"{code} {b}")
code, _ = jreq("GET", "/api/v1/admin/overview")
check("概览游客→401", code == 401)

# ===== 15. OpenAPI / Swagger =====
code, _ = jreq("GET", "/swagger/openapi.yaml")
check("OpenAPI YAML", code == 200)
code, _ = jreq("GET", "/swagger")
check("Swagger UI 壳", code == 200)

# ===== 16. 前端交付 =====
code, b = jreq("GET", "/")
check("SPA index", code == 200 and "assets/index-" in b)
import re as _re
m = _re.search(r"/assets/[A-Za-z0-9_-]+\.css", b)
(code, _), css_hdrs = req("GET", m.group(0), jar=owner) if m else ((0, b""), {})
check("静态 CSS", code == 200 and "text/css" in css_hdrs.get("Content-Type", ""),
      str(css_hdrs.get("Content-Type")))

# ===== 17. RSS / OG =====
code, b = jreq("GET", "/feed.xml")
check("RSS", code == 200 and "<rss" in b and "webhook 触发帖" in b)
code, b = jreq("GET", "/p/" + p1["id"])
check("OG 注入", code == 200 and 'property="og:title"' in b)

# ===== 18. CSRF / 删除 / 登出 =====
r = urllib.request.Request(BASE + "/api/v1/posts",
                           data=json.dumps({"content_md": "csrf"}).encode(), method="POST",
                           headers={"Content-Type": "application/json",
                                    "Origin": "http://evil.example", "Cookie": COOKIE})
try:
    code = urllib.request.build_opener().open(r, timeout=15).status
except urllib.error.HTTPError as e:
    code = e.code
check("CSRF 跨站 Origin→403", code == 403)
code, _ = jreq("DELETE", "/api/v1/posts/" + p1["id"], jar=owner)
check("删除 204", code == 204)
code, _ = jreq("GET", "/api/v1/posts/" + p1["id"], jar=owner)
check("删后 404", code == 404)
code, _ = jreq("POST", "/api/v1/auth/logout", jar=owner)
check("登出", code in (200, 204))

fails = [r for r in RESULTS if not r[1]]
print(f"\n===== 矩阵结果: {len(RESULTS) - len(fails)}/{len(RESULTS)} 通过 =====")
if fails:
    for name, _, detail in fails:
        print("  ✗", name, detail)
    sys.exit(1)
