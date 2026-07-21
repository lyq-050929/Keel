# Smart CS Agent Go

一个用 Go 写的智能客服多 Agent 原型项目，支持：

- 意图路由
- 知识检索回复
- 工单创建
- 合规检查
- 短期会话记忆
- 工作记忆
- MCP 工具注册表
- 基础追踪和指标

这个仓库目前的定位是：

- 可以直接跑起来
- 可以作为简历项目展示
- 可以继续慢慢演进为生产系统

## 架构

请求进入 `Gin API` 后，会进入 `SupervisorAgent`：

1. 先做意图识别
2. 再路由到知识检索或工单处理
3. 然后做合规检查
4. 最后拼装回复

记忆层包含：

- 短期记忆：会话消息，支持 Redis 和本地文件兜底
- 工作记忆：当前会话的状态和历史，支持本地文件持久化
- 长期记忆：内存知识库，适合演示 RAG 流程

## 启动

### 环境变量

- `PORT`：HTTP 端口，默认 `8090`
- `DATA_DIR`：数据目录，默认 `./data`
- `REDIS_URL`：可选，用于短期记忆 Redis 持久化
- `API_KEY`：可选，设置后会要求请求携带 `X-API-Key` 或 `Authorization: Bearer`
- `APP_ENV`：默认 `dev`
- `REQUEST_TIMEOUT`：默认 `30s`
- `SHUTDOWN_TIMEOUT`：默认 `10s`

### 本地运行

```bash
go mod download
go run .
```

### 常用命令

```bash
make fmt
make test
make build
make run
```

### Docker

```bash
docker build -t smart-cs-agent .
docker run -p 8090:8090 -e DATA_DIR=/data -v $(pwd)/data:/data smart-cs-agent
```

## 接口

- `POST /api/chat`
- `GET /api/history/:sessionId`
- `GET /api/tools`
- `GET /api/metrics`
- `GET /health`
- `GET /ready`

### 示例

```bash
curl -X POST http://localhost:8090/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"我想退款","user_id":"u1"}'
```

## 当前亮点

- 不是单文件 demo，而是一个分层 Agent 架构
- 支持会话和工作记忆持久化
- 有基础 MCP 工具注册表
- 有 request id、健康检查、就绪检查和指标
- 有基础测试覆盖
- 有 Dockerfile、Makefile、`.env.example`，适合放到 GitHub 展示

## 验证

```bash
go fmt ./...
go test ./...
go build ./...
```

## 后续可以继续优化的方向

- 接入真正的向量检索
- 接入真实工单系统
- 增加更完整的认证和权限
- 补充 Prometheus / OpenTelemetry 导出
- 增加更完整的端到端测试
