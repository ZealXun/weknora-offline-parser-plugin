# ZealXun Offline Document Parser

面向 WeKnora 外部插件运行时的离线文档解析器。插件支持 Markdown、纯文本和 HTML，将原始文件转换为规范 Markdown 后交回 WeKnora，由主系统继续完成分块、向量化和入库。

插件清单声明 `network: false`。可选的联网探针会尝试访问固定的 `https://example.com/`，用于验证 plugin-runtime 是否真正拦截未授权出站请求并记录 `network_denied` 事件；探针结果不会中断正常文档解析。

## 数据流

```text
WeKnora 上传文件
  -> 通过 gRPC 流式发送 64 KiB 文件块
  -> 插件离线转换 Markdown/TXT/HTML
  -> 返回标题、格式、探针结果和 Markdown
  -> WeKnora 分块、向量化并写入知识库
```

插件不能直接访问 WeKnora 的 PostgreSQL、Redis 或文件目录。

## 目录

```text
.
├── plugin.yaml                         # 身份、能力、配置 Schema 与权限
├── main.go                             # SDK gRPC Server 入口
├── internal/offlineparser/config.go    # 配置默认值与校验
├── internal/offlineparser/handler.go   # 生命周期、流式协议与联网探针
├── internal/offlineparser/transform.go # 离线文档转换
├── Dockerfile                          # 最小只读运行镜像
└── .github/workflows/release.yml       # tag 触发的测试与 GHCR 发布
```

## 支持格式

| 文件类型 | 处理方式 |
| --- | --- |
| `.md` / `.markdown` | 保留 Markdown，可选择移除 YAML Front Matter |
| `.txt` | 规范化换行并作为 Markdown 文本返回 |
| `.html` / `.htm` | 转换标题、段落、列表、链接、强调、代码块和引用 |

单个输入文件最大 100 MiB。

## 配置

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `preserve_front_matter` | `true` | 是否保留 Markdown 开头的 YAML Front Matter |
| `html_title_as_heading` | `true` | HTML 正文没有一级标题时，是否将 `<title>` 写成一级标题 |
| `outbound_probe` | `false` | 是否执行固定地址的未授权联网探针 |

探针地址不可由用户配置，避免把测试能力变成任意 URL 请求入口。

## 本地开发

仓库默认与 WeKnora 主仓位于同一目录：

```text
~/WeKnora
~/weknora-offline-parser-plugin
```

安装依赖并运行测试：

```bash
go mod tidy
go test ./...
```

本地启动 gRPC 服务：

```bash
WEKNORA_PLUGIN_LISTEN=:9000 go run .
```

构建与清单一致的本地镜像：

```bash
docker buildx build \
  --load \
  --build-context weknora=../WeKnora \
  -t ghcr.io/zealxun/weknora-offline-parser-plugin:0.1.0 \
  .
```

如果不允许下载基础镜像，可使用本机 Go 工具链完全离线构建：

```bash
mkdir -p dist
CGO_ENABLED=0 GOPROXY=off go build -trimpath -o dist/weknora-offline-parser-plugin .
docker build --network=none -f Dockerfile.local \
  -t ghcr.io/zealxun/weknora-offline-parser-plugin:0.1.0 .
```

`Dockerfile.local` 从 `scratch` 开始，只复制本地二进制，不拉取任何基础镜像。

## 安装到 WeKnora

1. 确保 WeKnora 使用 plugins profile 启动，且 plugin-runtime 正常运行。
2. 在“设置 → 插件管理”中粘贴 `plugin.yaml`，检查权限后安装。
3. 启用 `ZealXun 离线文档解析器`。
4. 在“设置 → 解析引擎”中选择该插件并保存配置。
5. 上传受支持格式的文件，WeKnora 会通过外部解析器处理。

本地镜像已经存在且 runtime 使用 `if-not-present` 拉取策略时，不需要先发布 GHCR。

## 验证出站拦截

1. 在解析引擎配置中开启 `outbound_probe`。
2. 上传一个 `.md`、`.txt` 或 `.html` 文件并触发解析。
3. 文档应当正常解析，返回元数据 `network_probe=denied:http_403` 或 `blocked:proxy_denied`。
4. 在插件“运行日志”中查询 `network_denied`。
5. 在系统审计日志中查询 `plugin.network_denied`，目标插件应为 `io.zealxun.parser.offline`。

如果探针结果是 `unexpectedly_allowed:*`，说明运行时网络隔离失效，不能视为测试通过。

## 发布

推送与清单版本一致的 tag（例如 `v0.1.0`）后，GitHub Actions 会：

1. 检出插件仓库和 WeKnora SDK 分支。
2. 执行 `go test ./...`。
3. 构建 `linux/amd64`、`linux/arm64` 镜像。
4. 发布到 `ghcr.io/zealxun/weknora-offline-parser-plugin`。

发布前需确认 GHCR package 对目标 WeKnora 部署环境可见。
