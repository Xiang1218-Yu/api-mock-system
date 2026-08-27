# API 接口聚合与 Mock 服务平台

面向前后端开发团队的 API 协作平台：接口定义管理、智能 Mock 数据生成、多接口聚合代理、自动 OpenAPI 文档。功能需求见 `system.md`。

## 快速开始

```bash
# 本地运行（自动建库 api_mock.db，无需外部服务）
make run
# 或直接
go run ./cmd/server

# 访问
open http://localhost:8080          # Web 管理界面
curl http://localhost:8080/healthz  # 健康检查
```

首次访问在 Web 界面注册账号 → 登录 → 创建项目 → 定义接口 → 发布 → 调用 Mock。

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `SERVER_PORT` | 服务端口 | `8080` |
| `DB_DSN` | 数据库连接串（SQLite 文件路径） | `api_mock.db` |
| `JWT_SECRET` | JWT 签名密钥（生产必须设置） | `dev-secret-change-me` |
| `JWT_EXPIRY` | Token 有效期（`24h` 或毫秒数） | `24h` |
| `MOCK_BASE_URL` | 聚合下游基础 URL | `http://localhost:8080/mock` |
| `AGGREGATE_TIMEOUT` | 聚合默认超时（毫秒） | `3000` |
| `RATE_RPS` | 接口限流速率（每秒每键令牌，`0` 关闭，§3.2） | `50` |
| `RATE_BURST` | 限流突发上限 | `100` |
| `LOG_LEVEL` | 日志级别 `debug/info/warn/error` | `info` |
| `APP_ENV` | 环境 `development/production` | `development` |

> 注：`system.md` 规定使用 PostgreSQL + Redis。本实现使用 **SQLite（纯 Go，无 CGO 即可，但 CGO 加速）** + **进程内缓存**，使二进制可零依赖运行。存储层均通过接口抽象，替换为 Postgres/Redis 仅需新增实现，无需改动业务层。

## 架构：分层 + 单一职责

每个包只承担一项职责，依赖单向流动：`models → repository → service → handler → router`。

```
cmd/server/         程序入口（仅 signal + app.Run）
internal/
  app/              组合根：唯一知道各层如何装配的地方
  config/           环境变量加载
  logger/           zap 日志构建
  auth/             JWT 签发/校验 + bcrypt 密码哈希
  cache/            进程内 TTL 缓存（Redis 替代）
  storage/          *gorm.DB 包装 + 自动迁移（唯一选择驱动的包）
  id/               UUID 生成
  httpx/            统一响应信封 + 绑定/分页/错误
  middleware/       CORS / 日志 / panic 恢复 / JWT 鉴权 / 限流
  pathmatch/        参数化路径匹配（/users/:id ↔ /users/42）
  models/           持久化实体（纯结构体，无逻辑）
  *repo/            数据访问：user/project/api/aggregate/mockdata/debug/calllog
  *service/         业务逻辑：user/project/api/mock/aggregate/doc/debug/dashboard
  mockengine/       JSON Schema → Mock 数据（确定性、智能类型生成器）
  aggregator/       串行/并行/条件聚合执行引擎（net/http + goroutine）
  openapi/          OpenAPI 3.0 文档构建 + JSON/YAML 序列化
  *handler/         HTTP 适配层（仅解析→调用服务→写响应）
  router/           路由装配表
  web/              嵌入式前端资源（vanilla SPA）
```

## 核心 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/register` | 注册 |
| POST | `/api/v1/auth/login` | 登录（返回 JWT） |
| GET | `/api/v1/projects` | 项目列表（按可见性/成员过滤） |
| POST | `/api/v1/projects/:projectId/apis` | 创建接口 |
| POST | `/api/v1/apis/:id/publish` | 发布接口（版本快照 +1） |
| POST | `/api/v1/apis/:id/rollback/:version` | 回滚到指定版本 |
| ANY | `/mock/:projectId/*path` | **Mock 调用**（公开） |
| POST | `/api/v1/apis/:id/mock/override` | 设置固定 Mock 覆盖 |
| POST | `/api/v1/projects/:projectId/aggregates` | 创建聚合接口 |
| ANY | `/aggregate/:projectId/*path` | **聚合调用**（鉴权） |
| GET | `/api/v1/projects/:projectId/docs/openapi.json` | 导出 OpenAPI |
| GET | `/api/v1/projects/:projectId/docs/preview` | 文档预览页（跳转 SPA Swagger 风格预览） |
| GET | `/api/v1/projects/:projectId/stats/trends?days=7` | 调用趋势 |
| GET | `/api/v1/projects/:projectId/stats/duration` | 耗时分布 |

完整接口见 `system.md` §6。

## Mock 引擎特性

- 基于 JSON Schema 生成：`object/array/string/integer/number/boolean/null`
- 智能生成器：姓名、邮箱、手机号、UUID、日期、枚举
- **确定性**：相同请求签名 → 相同 Mock 数据（满足”缓存与一致性”需求）。对象属性按名排序后遍历，保证相同 seed + schema 永远产出相同数据（消除 map 迭代序随机导致的非确定性）
- 字段名智能识别：`email` 字段生成邮箱、`*Id` 生成整数、`name` 生成姓名
- 响应延迟模拟（0–5000ms）、状态码模拟
- 手动覆盖：固定值替换随机值，按请求键（method+path+query+body）匹配

## 聚合引擎特性

- 三种模式：`serial`（串行）/ `parallel`（并行 goroutine）/ `conditional`（按请求参数选择下游）
- 字段映射：合并结果中字段重命名/转换
- 超时控制：下游任一超时返回超时标记 + 已完成数据
- 调用监控：返回每个下游的状态码、耗时、错误（耗时向上取整，不低于 1ms）

## 数据看板

- 项目统计：接口数、Mock/聚合调用次数、状态分布
- `GET /api/v1/projects/:projectId/stats/trends?days=7` — 按天 Mock/聚合调用趋势
- `GET /api/v1/projects/:projectId/stats/duration` — 响应耗时区间分布（0-10/10-50/50-100/100-500/500+ms）
- Mock/聚合运行时调用异步落库（`call_logs` 表），不阻塞响应

## 部署

```bash
# Docker
make docker-up        # 构建并启动容器
docker compose down   # 停止
```

SQLite 数据库持久化在 `api-mock-data` 卷中。切换到 Postgres/Redis 时：取消 `docker-compose.yml` 中注释，实现 `*repo` 接口的 Postgres 版本，并修改 `storage.Open`。

## 权限模型

- JWT 认证，24h 过期
- 项目级 RBAC：`admin`（全权）/ `editor`（改接口、聚合、Mock）/ `viewer`（只读）
- 私有项目仅成员可见；公开项目所有人可见
- Mock 运行时路由公开（供前端开发调用）；管理操作需鉴权 + 角色
- Mock 覆盖写入需 `editor+`，读取需 `viewer+`（在 service 层强制，非依赖 handler）
- 接口限流：管理 API 按用户限流，Mock 路由按项目限流（`RATE_RPS`/`RATE_BURST`，§3.2）
