import { expect, test } from "@playwright/test"
import { deflateSync } from "node:zlib"

/**
 * 移动视口冒烟：发布 → 时间线 → 详情 → 编辑 → 删除。
 * 前置：WISP_URL 指向一个已初始化（owner=liang/password123）的全新 wisp 实例。
 */
const USERNAME = "liang"
const PASSWORD = "password123"

let seeded = false

/** 生成尺寸随时间的 PNG，避免压缩后与历史上传秒传成同一条附件。 */
function uniquePng(): Buffer {
  const crc32 = (buf: Buffer) => {
    let c = 0xffffffff
    for (const byte of buf) {
      c ^= byte
      for (let k = 0; k < 8; k++) c = c & 1 ? (c >>> 1) ^ 0xedb88320 : c >>> 1
    }
    return (c ^ 0xffffffff) >>> 0
  }
  const chunk = (type: string, data: Buffer) => {
    const len = Buffer.alloc(4)
    len.writeUInt32BE(data.length)
    const body = Buffer.concat([Buffer.from(type), data])
    const crc = Buffer.alloc(4)
    crc.writeUInt32BE(crc32(body))
    return Buffer.concat([len, body, crc])
  }
  const w = 8 + (Date.now() % 24)
  const h = 8 + (Date.now() % 7)
  const ihdr = Buffer.alloc(13)
  ihdr.writeUInt32BE(w, 0)
  ihdr.writeUInt32BE(h, 4)
  ihdr[8] = 8 // bit depth
  ihdr[9] = 2 // truecolor
  const raw = Buffer.alloc((w * 3 + 1) * h)
  for (let y = 0; y < h; y++) {
    const off = y * (w * 3 + 1)
    raw[off] = 0
    for (let x = 0; x < w; x++) {
      raw[off + 1 + x * 3] = (x * 17 + y * 3) % 256
      raw[off + 2 + x * 3] = (x * 5 + y * 29) % 256
      raw[off + 3 + x * 3] = (x * 11 + y * 7) % 256
    }
  }
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk("IHDR", ihdr),
    chunk("IDAT", deflateSync(raw)),
    chunk("IEND", Buffer.alloc(0)),
  ])
}

async function ensureLogin(page: import("@playwright/test").Page) {
  await page.goto("/login")
  await page.waitForSelector("form")
  // 首启向导（零用户）或登录页，都能到达登录态
  const hasInit = await page
    .getByText("初始化 Wisp")
    .isVisible()
    .catch(() => false)
  const user = USERNAME
  const pass = PASSWORD
  if (hasInit && !seeded) {
    await page.getByLabel("用户名").fill(user)
    await page.getByLabel("密码（至少 8 位）").fill(pass)
    await page.getByRole("button", { name: "创建并登录" }).click()
  } else {
    await page.getByLabel("用户名").fill(user)
    await page.getByLabel("密码").fill(pass)
    await page.getByRole("button", { name: "登录", exact: true }).click()
  }
  await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 10_000 })
  seeded = true
}

test("发布 → 时间线 → 详情 → 编辑 → 删除 全链路", async ({ page }) => {
  await ensureLogin(page)

  // 发布
  const content = `E2E 冒烟 ${Date.now()}`
  const editor = page.locator("[contenteditable=true]").first()
  await editor.click()
  await page.keyboard.type(content)
  await page.getByRole("button", { name: "发布" }).click()
  await expect(page.getByText(content).first()).toBeVisible({ timeout: 10_000 })

  // 详情页（卡片正文即链接）
  const firstCard = page.locator("article").filter({ hasText: content }).first()
  await firstCard.locator("a").filter({ hasText: content }).first().click()
  await page.waitForURL((u) => u.pathname.startsWith("/p/"))
  await expect(page.getByText(content).first()).toBeVisible()

  // 编辑（追加文本）
  await page.getByRole("button", { name: "编辑" }).click()
  await page.locator("[contenteditable=true]").first().click()
  await page.keyboard.type("（已编辑）")
  await page.getByRole("button", { name: "保存" }).click()
  await expect(page.getByText("（已编辑）").first()).toBeVisible({ timeout: 10_000 })

  // 删除（确认弹窗）
  await page.getByRole("button", { name: "删除" }).first().click()
  await page.getByRole("dialog").getByRole("button", { name: "删除" }).click()
  await page.waitForURL((u) => !u.pathname.startsWith("/p/"), { timeout: 10_000 })

  // 回到时间线，内容不在
  await expect(page.getByText(content)).toHaveCount(0)
})

test("深色模式切换", async ({ page }) => {
  await page.goto("/")
  await page.getByRole("button", { name: "切换深浅色" }).click()
  await expect
    .poll(() => page.evaluate(() => document.documentElement.classList.contains("dark")))
    .toBe(true)
  await page.getByRole("button", { name: "切换深浅色" }).click()
  await expect
    .poll(() => page.evaluate(() => document.documentElement.classList.contains("dark")))
    .toBe(false)
})

test("扩展卡片：位置不写入正文", async ({ page }) => {
  await ensureLogin(page)
  const place = "杭州 · 西湖"
  await page.getByRole("button", { name: "位置" }).click()
  await page.getByRole("textbox", { name: "地点名称" }).fill(place)
  await page.getByRole("button", { name: "添加", exact: true }).click()
  // 草稿应保留扩展卡片
  await page.waitForTimeout(700)
  await page.reload()
  await expect(page.getByText(place).first()).toBeVisible({ timeout: 10_000 })
  await page.getByRole("button", { name: "发布" }).click()
  const card = page.locator("article").first()
  await expect(card.getByText(place)).toBeVisible({ timeout: 10_000 })
  // 正文为空（卡片而非正文）
  await expect(card.locator(".post-content")).toHaveText("")
})

test("设置页切换子页面不重挂载左侧导航", async ({ page }) => {
  await ensureLogin(page)
  await page.goto("/settings")
  await page.waitForSelector("[data-settings-nav]")
  await page.evaluate(() => {
    ;(window as unknown as { __nav?: Element | null }).__nav =
      document.querySelector("[data-settings-nav]")
  })
  await page.getByRole("link", { name: "API 令牌" }).click()
  await expect(page.getByRole("heading", { name: "访问令牌" })).toBeVisible()
  const same = await page.evaluate(() => {
    const w = window as unknown as { __nav?: Element | null }
    return w.__nav === document.querySelector("[data-settings-nav]")
  })
  expect(same).toBe(true)
})

test("内容包导出接口可用", async ({ page }) => {
  await ensureLogin(page)
  const res = await page.request.get("/api/v1/admin/export/capsule?include_private=1")
  expect(res.status()).toBe(200)
  expect(res.headers()["content-type"]).toContain("application/zip")
  const body = await res.body()
  expect(body.length).toBeGreaterThan(100)
  expect(body.subarray(0, 2).toString()).toBe("PK")
})

test("媒体库：上传后可查看并删除确认", async ({ page }) => {
  await ensureLogin(page)
  const png = uniquePng()
  await page.locator('input[type="file"]').first().setInputFiles({
    name: "tiny.png",
    mimeType: "image/png",
    buffer: png,
  })
  await expect(page.getByRole("button", { name: "移除" }).first()).toBeVisible({ timeout: 10_000 })

  const content = `媒体库 E2E ${Date.now()}`
  await page.locator("[contenteditable=true]").first().click()
  await page.keyboard.type(content)
  await page.getByRole("button", { name: "发布" }).click()
  await expect(page.getByText(content).first()).toBeVisible({ timeout: 10_000 })

  await page.goto("/settings/data")
  const section = page
    .getByRole("heading", { name: "媒体库" })
    .locator("xpath=ancestor::section[1]")
  await expect(section).toBeVisible()
  await expect(section.getByText(/个文件/)).toBeVisible()
  await expect(section.getByText(content).first()).toBeVisible({ timeout: 10_000 })

  await section.getByRole("button", { name: "删除" }).first().click()
  const dialog = page.getByRole("dialog")
  await expect(dialog.getByText("删除文件")).toBeVisible()
  await expect(dialog.getByText(/仍被帖子引用/)).toBeVisible()
  await dialog.getByRole("button", { name: "取消" }).click()
  await expect(dialog).toHaveCount(0)
})

test("标签管理：重命名与合并", async ({ page }) => {
  await ensureLogin(page)
  const stamp = Date.now()
  const tagA = `标签甲${stamp}`
  const tagB = `标签乙${stamp}`
  const renamed = `标签丙${stamp}`
  const merged = `标签丁${stamp}`
  for (const tag of [tagA, tagB]) {
    const res = await page.request.post("/api/v1/posts", {
      data: { content_md: `标签测试 #${tag}` },
    })
    expect(res.status()).toBe(201)
  }

  await page.goto("/stats?tab=tags")
  await page.getByRole("button", { name: "管理标签" }).click()
  await expect(page.getByText(`#${tagA}`)).toBeVisible()
  await expect(page.getByText(`#${tagB}`)).toBeVisible()

  // 行内重命名
  await page.getByRole("button", { name: `重命名 ${tagA}` }).click()
  let dialog = page.getByRole("dialog")
  await dialog.getByLabel("标签名").fill(renamed)
  await dialog.getByRole("button", { name: "保存" }).click()
  await expect(page.getByText(`#${renamed}`)).toBeVisible({ timeout: 10_000 })
  await expect(page.getByText(`#${tagA}`)).toHaveCount(0)

  // 多选合并到新名称
  await page.getByRole("checkbox", { name: renamed }).click()
  await page.getByRole("checkbox", { name: tagB }).click()
  await page.getByLabel("合并到标签").fill(merged)
  await page.getByRole("button", { name: "合并", exact: true }).click()
  dialog = page.getByRole("dialog")
  await expect(dialog.getByText(new RegExp(`#${merged}`))).toBeVisible()
  await dialog.getByRole("button", { name: "合并", exact: true }).click()
  await expect(page.getByText(`#${merged}`)).toBeVisible({ timeout: 10_000 })
  await expect(page.getByText(`#${renamed}`)).toHaveCount(0)
  await expect(page.getByText(`#${tagB}`)).toHaveCount(0)
})

test("筛选：标签/可见性、URL 持久化与清除", async ({ page }) => {
  await ensureLogin(page)
  const stamp = Date.now()
  const tagA = `筛选甲${stamp}`
  const tagB = `筛选乙${stamp}`
  const publicText = `B1 公开 ${stamp}`
  const privateText = `B1 私密 ${stamp}`
  for (const input of [
    { content_md: `${publicText} #${tagA}`, visibility: "public" },
    { content_md: `${privateText} #${tagB}`, visibility: "private" },
  ]) {
    const res = await page.request.post("/api/v1/posts", { data: input })
    expect(res.status()).toBe(201)
  }

  await page.goto("/")
  await page.getByRole("button", { name: "筛选", exact: true }).click()
  let dialog = page.getByRole("dialog")
  await expect(dialog.getByText("筛选说说")).toBeVisible()
  await dialog.getByLabel("搜索标签").fill(tagA)
  await dialog.getByRole("checkbox", { name: tagA }).click()
  await dialog.getByRole("button", { name: "应用" }).click()

  await expect(page.getByRole("button", { name: `移除筛选：#${tagA}` })).toBeVisible()
  await expect(page.getByText(publicText).first()).toBeVisible()
  await expect(page.getByText(privateText)).toHaveCount(0)

  // 刷新后条件保留
  await page.reload()
  await expect(page.getByRole("button", { name: `移除筛选：#${tagA}` })).toBeVisible()
  await expect(page.getByText(publicText).first()).toBeVisible()

  // 快捷键打开筛选
  await page.keyboard.press("Control+k")
  dialog = page.getByRole("dialog")
  await expect(dialog.getByText("筛选说说")).toBeVisible()
  await page.keyboard.press("Escape")
  await expect(dialog).toHaveCount(0)

  // 清除后恢复全部（owner 能看到自己的私密帖）
  await page.getByRole("button", { name: "清除全部" }).click()
  await expect(page.getByRole("button", { name: `移除筛选：#${tagA}` })).toHaveCount(0)
  await expect(page.getByText(privateText).first()).toBeVisible()

  // 可见性筛选：只看私密
  await page.getByRole("button", { name: "筛选", exact: true }).click()
  dialog = page.getByRole("dialog")
  await dialog.getByRole("button", { name: "私密", exact: true }).click()
  await dialog.getByRole("button", { name: "应用" }).click()
  await expect(page.getByText(privateText).first()).toBeVisible()
  await expect(page.getByText(publicText)).toHaveCount(0)
  await page.getByRole("button", { name: "清除全部" }).click()
})

test("分享链接与表情弹层：外部/ Esc 关闭并归还焦点", async ({ page, context }) => {
  await ensureLogin(page)
  const text = `分享测试 ${Date.now()}`
  const res = await page.request.post("/api/v1/posts", { data: { content_md: text } })
  expect(res.status()).toBe(201)

  await context.grantPermissions(["clipboard-read", "clipboard-write"])
  await page.addInitScript(() => {
    Object.defineProperty(navigator, "share", { value: undefined, configurable: true })
  })
  await page.reload()

  const article = page.locator("article").filter({ hasText: text }).first()
  await expect(article).toBeVisible()
  await article.getByRole("button", { name: "分享" }).click()
  await expect(page.getByText("链接已复制")).toBeVisible()
  const clip = await page.evaluate(() => navigator.clipboard.readText())
  expect(clip).toContain("/p/")

  const trigger = article.getByRole("button", { name: "添加表情" })
  await trigger.click()
  await expect(article.getByRole("menuitem", { name: "👍" })).toBeVisible()
  await page.keyboard.press("Escape")
  await expect(article.getByRole("menuitem", { name: "👍" })).toHaveCount(0)
  await expect(trigger).toBeFocused()

  await trigger.click()
  await expect(article.getByRole("menuitem", { name: "👍" })).toBeVisible()
  await page.locator("h2").first().click()
  await expect(article.getByRole("menuitem", { name: "👍" })).toHaveCount(0)
})

test("自动备份：立即备份与保存设置", async ({ page }) => {
  await ensureLogin(page)
  await page.goto("/settings/data")
  const section = page
    .getByRole("heading", { name: "备份数据库" })
    .locator("xpath=ancestor::section[1]")
  await expect(section).toBeVisible()
  const interval = section.getByRole("spinbutton").first()
  const target = (await interval.inputValue()) === "12" ? "6" : "12"

  await section.getByRole("button", { name: "立即备份" }).click()
  await expect(page.getByText(/已生成 scheduled-/)).toBeVisible({ timeout: 10_000 })
  await expect(section.getByText(/最新：scheduled-/)).toBeVisible()

  await interval.fill(target)
  await section.getByRole("button", { name: "保存设置" }).click()
  await expect(page.getByText("备份设置已保存")).toBeVisible()
  await page.reload()
  const sectionAfter = page
    .getByRole("heading", { name: "备份数据库" })
    .locator("xpath=ancestor::section[1]")
  await expect(sectionAfter.getByRole("spinbutton").first()).toHaveValue(target)

  // 服务端归一化后表单同步显示实际保存值
  const intervalAfter = sectionAfter.getByRole("spinbutton").first()
  await intervalAfter.fill("999")
  await sectionAfter.getByRole("button", { name: "保存设置" }).click()
  await expect(page.getByText("备份设置已保存")).toBeVisible()
  await expect(intervalAfter).toHaveValue("24")
})

test("概览：指标卡、备份与版本信息", async ({ page }) => {
  await ensureLogin(page)
  await page.goto("/stats?tab=overview")
  await expect(page.getByRole("tab", { name: "概览" })).toHaveAttribute("aria-selected", "true")
  for (const label of ["帖子总数", "今日发布", "媒体占用", "标签"]) {
    await expect(page.getByText(label, { exact: true }).first()).toBeVisible()
  }
  await expect(page.getByText(/数据库 \d/).first()).toBeVisible()
  await expect(page.getByText(/上次备份/).first()).toBeVisible()
  await expect(page.getByText(/^v\d/).first()).toBeVisible()
  await expect(page.getByText(/运行时长/).first()).toBeVisible()
})

test("扩展卡片统一外壳：GitHub 与位置", async ({ page }) => {
  await ensureLogin(page)
  const text = `外壳测试 ${Date.now()}`
  const res = await page.request.post("/api/v1/posts", {
    data: {
      content_md: text,
      blocks: [
        { type: "github", url: "https://github.com/oklog/ulid" },
        { type: "location", name: "上海 · 徐汇" },
      ],
    },
  })
  expect(res.status()).toBe(201)
  await page.goto("/")
  const article = page.locator("article").filter({ hasText: text }).first()
  await expect(article.getByText("GitHub", { exact: true })).toBeVisible()
  await expect(article.getByText("https://github.com/oklog/ulid")).toBeVisible()
  await expect(article.getByText("位置", { exact: true })).toBeVisible()
  await expect(article.getByText("上海 · 徐汇")).toBeVisible()
  await expect(article.getByRole("link", { name: "OpenStreetMap" })).toBeVisible()
})

test("位置卡：坐标展示与点击加载地图", async ({ page }) => {
  await ensureLogin(page)
  const text = `坐标测试 ${Date.now()}`
  const res = await page.request.post("/api/v1/posts", {
    data: {
      content_md: text,
      blocks: [{ type: "location", name: "上海 · 徐汇", lat: 31.18, lng: 121.43 }],
    },
  })
  expect(res.status()).toBe(201)
  await page.goto("/")
  const article = page.locator("article").filter({ hasText: text }).first()
  await expect(article.getByText("31.1800,121.4300")).toBeVisible()
  const mapLink = article.getByRole("link", { name: "OpenStreetMap" })
  await expect(mapLink).toHaveAttribute("href", /mlat=31\.18&mlon=121\.43/)

  const toggle = article.getByRole("button", { name: "展开地图" })
  await toggle.click()
  const frame = article.locator("iframe")
  await expect(frame).toBeVisible()
  await expect(frame).toHaveAttribute("src", /openstreetmap\.org\/export\/embed\.html/)
  await expect(frame).toHaveAttribute("src", /marker=31\.18%2C121\.43/)
  await page.keyboard.press("Escape")
  await expect(frame).toHaveCount(0)
  await expect(article.getByRole("button", { name: "展开地图" })).toBeFocused()
})

test("显式网站/视频块：编辑器校验与点击播放", async ({ page }) => {
  await ensureLogin(page)
  const videoText = `D3 视频 ${Date.now()}`

  await page.getByRole("button", { name: "视频", exact: true }).click()
  const dialog = page.getByRole("dialog")
  await expect(dialog.getByText("添加视频")).toBeVisible()
  await dialog.getByLabel("链接").fill("https://example.com/not-video")
  await dialog.getByRole("button", { name: "添加", exact: true }).click()
  await expect(dialog.getByText("仅支持 B 站/YouTube 视频链接")).toBeVisible()
  await dialog.getByLabel("链接").fill("https://www.bilibili.com/video/BV1xx411c7mD")
  await dialog.getByRole("button", { name: "添加", exact: true }).click()
  await expect(dialog).toHaveCount(0)

  await page.locator("[contenteditable=true]").first().click()
  await page.keyboard.type(videoText)
  await page.getByRole("button", { name: "发布" }).click()
  const videoArticle = page.locator("article").filter({ hasText: videoText }).first()
  await expect(videoArticle.getByText("视频", { exact: true })).toBeVisible()
  await videoArticle.getByRole("button", { name: "播放视频" }).click()
  const frame = videoArticle.locator("iframe")
  await expect(frame).toBeVisible()
  await expect(frame).toHaveAttribute(
    "src",
    /player\.bilibili\.com\/player\.html\?bvid=BV1xx411c7mD/,
  )

  // 网站卡片（API 创建）：标题抓取失败时回退域名
  const siteText = `D3 网站 ${Date.now()}`
  const res = await page.request.post("/api/v1/posts", {
    data: {
      content_md: siteText,
      blocks: [{ type: "website", url: "https://example.com/article" }],
    },
  })
  expect(res.status()).toBe(201)
  await page.reload()
  const siteArticle = page.locator("article").filter({ hasText: siteText }).first()
  await expect(siteArticle.getByText("网站", { exact: true })).toBeVisible()
  await expect(siteArticle.getByText("example.com", { exact: true })).toBeVisible()
  await expect(siteArticle.getByRole("link", { name: "打开" }).first()).toHaveAttribute(
    "href",
    "https://example.com/article",
  )
})

test("草稿：发布后清空、分享页不覆盖草稿", async ({ page }) => {
  await ensureLogin(page)
  await page.goto("/")
  const editor = page.locator("[contenteditable=true]").first()
  const draftText = `草稿保留 ${Date.now()}`
  await editor.click()
  await page.keyboard.type(draftText)
  await page.waitForTimeout(700) // 等 autosave

  await page.goto("/share?text=%E5%88%86%E4%BA%AB%E5%86%85%E5%AE%B9")
  await page.waitForTimeout(700)
  await page.goto("/")
  const editor2 = page.locator("[contenteditable=true]").first()
  await expect(editor2).toContainText(draftText)

  await editor2.click()
  await page.keyboard.press("Control+a")
  await page.keyboard.press("Backspace")
  const published = `一次性发布 ${Date.now()}`
  await page.keyboard.type(published)
  await page.getByRole("button", { name: "发布" }).click()
  await expect(page.getByText(published).first()).toBeVisible()
  await expect(editor2).not.toContainText(published)
  await page.reload()
  await expect(page.locator("[contenteditable=true]").first()).not.toContainText(published)
})

test("扩展卡片弹窗：占位文案与入口去重", async ({ page }) => {
  await ensureLogin(page)
  await page.goto("/")
  await expect(page.getByRole("button", { name: "仅链接", exact: true })).toHaveCount(0)
  await expect(page.getByRole("button", { name: "链接", exact: true })).toHaveCount(0)

  await page.getByRole("button", { name: "网站", exact: true }).click()
  const dialog = page.getByRole("dialog")
  await expect(dialog.getByLabel("链接")).toHaveAttribute(
    "placeholder",
    "https://example.com/article",
  )
  await page.keyboard.press("Escape")
  await expect(dialog).toHaveCount(0)
})
