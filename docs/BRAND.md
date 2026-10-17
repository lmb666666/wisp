# 品牌与素材

Wisp 的标记是**一个带尾巴的 W + 一点光**：W 代表“说说/写”，上扬的笔画与星火代表“一缕轻烟/一点灵感”，暖陶土色来自纸张与傍晚的光。

## 文件

| 文件 | 用途 |
|---|---|
| `web/public/icon.svg` | 网站 favicon、PWA any 图标 |
| `web/public/icon-maskable.svg` | Android maskable 源文件 |
| `web/public/icon-192.png` / `icon-512.png` | PWA / manifest |
| `web/public/icon-maskable-512.png` | Android 自适应图标 |
| `web/public/apple-touch-icon.png` | iOS 主屏（180px，满版底） |
| `web/public/favicon.ico` | 传统浏览器 favicon |
| `web/public/og.png` | 默认 OG / 社交媒体预览 |
| `docs/assets/logo.svg` | 圆角方形标记（文档/展示） |
| `docs/assets/logo-mark.svg` | 透明底标记 |
| `docs/assets/banner.png` | README 顶部横幅（1280×360） |
| `docs/assets/social-preview.png` | GitHub Social Preview（1280×640） |

## 颜色

| 名称 | 值 | 用途 |
|---|---|---|
| Terracotta | `#b85a2f` | 主色、图标底 |
| Ember | `#a9431d` | 渐变深端 |
| Apricot | `#d97b42` | 渐变亮端 |
| Cream | `#fff6e8` | 标记笔画 |
| Sand | `#ffe0ad` | 星火 |
| Paper | `#f8f0e5` | 横幅/卡片底色 |
| Ink | `#2b1d16` | 字体主色 |

## 使用

- 最小尺寸：图标 16px；小于 24px 时不加文字。
- 留白：标记四周至少保留图形宽度的 12%。
- 使用圆角方形底时，圆角约为边长的 22%。
- 不要把 W 拉伸、旋转、加描边或换成高饱和霓虹色。
- 不要把标记放进与暖色相近的低对比背景；深色背景下使用固定陶土底，不要改成主题浅橙。

## 重新生成

需要 `rsvg-convert` 与 ImageMagick：

```bash
rsvg-convert -w 192 -h 192 web/public/icon.svg -o web/public/icon-192.png
rsvg-convert -w 512 -h 512 web/public/icon.svg -o web/public/icon-512.png
rsvg-convert -w 512 -h 512 web/public/icon-maskable.svg -o web/public/icon-maskable-512.png
rsvg-convert -w 180 -h 180 web/public/icon-maskable.svg -o web/public/apple-touch-icon.png
for s in 16 32 48; do rsvg-convert -w $s -h $s web/public/icon.svg -o /tmp/fav-$s.png; done
magick /tmp/fav-16.png /tmp/fav-32.png /tmp/fav-48.png web/public/favicon.ico
rsvg-convert -w 1280 -h 360 docs/assets/banner.svg -o docs/assets/banner.png
rsvg-convert -w 1280 -h 640 docs/assets/social-preview.svg -o docs/assets/social-preview.png
```

截图使用 390×844 视口、2× DPR、演示数据；不要使用真实用户内容。
