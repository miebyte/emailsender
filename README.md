# emailsender

一个提供 gRPC 接口的轻量邮件发送代理，支持同步发送、Redis 异步队列和 MySQL 发送记录。邮件支持 UTF-8 纯文本和 HTML，通过 SMTP 直接 TLS 连接发送。

## 结构

```text
main.go                   配置、连接、依赖组装和服务启动
api/grpc/                 protobuf 协议、参数映射和 gRPC 错误映射
internal/config/          配置默认值与启动校验
internal/email/           发送流程、输入校验和所需的三个窄接口
internal/smtp/            SMTP 连接、MIME 编码、超时和取消
internal/queue/           Redis 任务序列化及入队、消费
internal/storage/         记录存储、独立表模型和生成的 GORM 查询
cmd/generate/             GORM 查询生成入口
```

这个项目没有复杂领域模型，使用服务与适配器组织代码。API 不接触数据库和 Redis，邮件服务不依赖具体客户端；状态由服务直接更新，不经过事件总线。只有一个记录表和单条写入，不额外建立仓储工厂或跨仓储事务层。

基础设施与启动使用 `goutils`，Redis 连接由 `main` 通过 `redisutils.NewRedisPool` 创建、启动验证并关闭，队列仅接收客户端。`redisutils` 负责 ACL、连接/读写超时、Context deadline 和重试配置。

`emailutils` 固定发送 HTML 且不支持 Context/超时，`queueutils.RedisQueue` 内部使用 Background Context，因此这两处保留小型适配器，分别处理支持正文格式选择的 SMTP 和带操作期限的 Redis 队列命令。

## 运行

需要 Go 1.27、MySQL、Redis，以及支持直接 TLS 的 SMTP 服务（通常为 465 端口）。先创建空的 `emailsender` 数据库；启动会通过 `models.AllModels()` 自动建表。升级后，AutoMigrate 会新增 `content_type` 列，旧记录默认使用 `text/plain`。

当前 Redis 配置扩展尚未发布库版本，`go.mod` 使用相对路径 `replace github.com/miebyte/goutils => ../goutils`，需要将修改后的 goutils 源码放在本项目同级目录。发布包含这些改动的新版本后，可升级依赖并移除该替换。

```sh
cp config.example.json config.json
# 填写 config.json 中的数据库和 SMTP 凭据
go run . --configFile config.json
```

内部 HTTP 默认监听 `28081`，健康检查为 `/health_check`；gRPC 默认监听 `28082`。HTTP 端口使用 `--port` 设置，gRPC 端口使用 `config.grpc_port` 设置。

未指定本地配置文件时，`flags/v2` 从 Consul 的 `configs/email_sender/config` 读取配置。其值为示例文件中 `config` 对象的内容，不含外层 `config`；连接通过 `CONSUL_HTTP_ADDR`、`CONSUL_HTTP_TOKEN` 等标准变量配置。当前基础设施在启动时读取配置快照，修改后需重启服务。

配置中 `queue_name` 默认 `emailsender:tasks`，SMTP 端口默认 `465`，SMTP 超时默认 `30` 秒。Redis 支持 ACL `username` 和 `password`；`timeout` 默认 `2` 秒，范围 2–60 秒，用于限制连接及单次队列操作。邮件队列要求显式设置 `context_timeout_enabled: true` 和 `max_retries: -1`，保证操作期限生效并关闭命令重试；Worker 的退出等待覆盖 Redis 操作和最终状态写入。

## 接口与发送行为

`internal/proto/emailsend.proto` 定义两个 RPC，均接收 `from`、`to`、`subject`、`body` 和可选的 `content_type`，响应为空：

- `EmailSender.Send`：等待 SMTP 接受邮件，再记录发送成功。
- `EmailSender.SendAsync`：创建待发送记录并入队后返回，Worker 发送后更新状态。

每次调用支持一个收件人。`from` 和 `to` 支持带显示名称的邮箱地址，各最多 320 个字符；主题最多 255 个字符，正文最多 65535 字节。请求必须为合法 UTF-8，邮件头禁止换行。SMTP 信封发件人使用配置中的账号，邮件展示的 From 使用请求值，SMTP 服务可能对此施加限制。

`content_type` 仅支持 `text/plain` 和 `text/html`；省略或传空字符串时默认使用 `text/plain`，兼容现有调用。传入其他值会返回 gRPC `InvalidArgument`。发送 HTML 时，将 HTML 源码放入 `body`，并设置 `content_type: "text/html"`；同步和异步发送均支持此参数。例如，已创建的 Go gRPC 客户端可这样调用：

```go
_, err := client.Send(ctx, &emailsendpb.SendEmailRequest{
	From:        "sender@example.com",
	To:          "receiver@example.com",
	Subject:     "HTML 邮件",
	Body:        "<h1>你好</h1><p>这是一封 HTML 邮件。</p>",
	ContentType: "text/html",
})
```

异步入队失败时会尝试把记录标为失败；失败补偿或最终状态更新失败会报告错误。发送失败记录为失败状态并继续消费下一封；队列故障或状态持久化失败会使 Worker 报错，由 `cores` 停止服务。进程退出时，已开始发送的任务会在有限时间内尝试写回结果。

队列使用 Redis List，出队即移除，没有消息确认、自动重试或跨 MySQL/Redis 事务。进程异常终止可能留下待发送记录；同步发送成功后写库失败时也无法撤销邮件。当前不提供恰好一次投递保证、记录查询接口或送达回执。

## 开发与验证

```sh
make test
make vet
make build
make generate-query
```

GORM 查询由锁定的 `gorm.io/gen v0.3.26` 从 `internal/storage/models` 生成，无需连接数据库。修改模型后运行 `make generate-query`，提交生成文件，不直接编辑 `internal/storage/query`。

重新生成 protobuf 需先安装 `protoc`，然后执行 `make tools` 和 `make generate-proto`。`make generate` 同时生成查询与协议代码。

测试使用 Fake 依赖、MySQL SQL mock 和本地 TLS SMTP 服务，无需真实数据库、Redis 或邮箱。支持 CGO 的开发环境还可执行 `CGO_ENABLED=1 go test -race ./...`。

Docker 镜像通过 `docker compose up --build` 启动，默认从 Consul 加载配置。构建使用命名上下文读取同级 `../goutils` 源码，需要 Docker Compose 2.17 或以上版本（[additional_contexts 文档](https://docs.docker.com/reference/compose-file/build/#additional_contexts)）。直接构建时使用 `docker build --build-context goutils=../goutils -t emailsender .`。Compose 使用 host 网络，适用于 Linux Docker 主机或已开启 host 网络支持的环境。
