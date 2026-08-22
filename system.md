```markdown
# 需求文档：API接口聚合与 Mock 服务平台

## 1. 项目概述

### 1.1 项目名称
API接口聚合与 Mock 服务平台（API Aggregation & Mock Platform）

### 1.2 项目目标
构建一个面向前后端开发团队的API协作工具，提供接口定义管理、智能Mock数据生成、多接口聚合代理和自动文档生成能力，解决前后端并行开发时的接口依赖问题，提升团队协作效率。

### 1.3 目标用户
- 后端开发：定义接口规范、管理接口变更
- 前端开发：查看接口文档、使用Mock数据进行开发调试
- 测试工程师：快速生成测试数据、进行接口集成测试
- 技术负责人：统一管理团队API资产

### 1.4 核心业务流程
```
创建项目 → 定义接口 → 配置响应Schema → 自动生成Mock数据 → 前端调用Mock接口开发
                    ↓
             配置聚合规则 → 代理多个下游接口 → 返回聚合结果
                    ↓
             发布文档 → 生成OpenAPI → 对外提供接口文档
```

---

## 2. 功能模块

### 2.1 用户与项目模块
- 用户注册/登录（邮箱+密码，JWT认证）
- 项目创建（名称、描述、基础路径、团队可见性）
- 项目成员管理（邀请/移除，角色：管理员/编辑/只读）
- 项目列表与搜索

### 2.2 接口定义模块
- 接口基础信息：名称、描述、请求方法（GET/POST/PUT/DELETE/PATCH）
- 请求路径（支持路径参数，如 /users/:id）
- 请求参数：Query参数、Path参数、Header参数、Body参数
- Body参数支持JSON Schema定义（对象嵌套、数组、枚举）
- 响应定义：HTTP状态码、响应体JSON Schema、响应示例
- 接口标签与分组（便于分类管理）
- 接口版本管理（每次修改保存历史，支持回滚）
- 接口状态：设计中/已发布/已废弃

### 2.3 Mock数据引擎模块
- 基于JSON Schema自动生成符合结构的模拟数据
- 内置数据类型生成器：string/number/boolean/array/object/null
- 智能Mock规则：支持随机姓名、手机号、邮箱、日期、UUID、枚举值
- 数据关联支持：字段间引用（如 userId → 对应的 user 对象）
- 响应延迟模拟：可配置每个接口的响应延迟（0-5000ms）
- 响应状态码模拟：可配置返回200/400/404/500等
- Mock数据手动编辑覆盖（固定值替换随机值）
- Mock数据缓存与一致性（相同请求返回相同Mock数据）

### 2.4 API聚合代理模块
- 选择多个下游接口（来自同项目或其他项目）
- 配置聚合模式：
  - 串行聚合：按顺序依次调用，结果合并
  - 并行聚合：同时调用多个接口，结果合并
  - 条件聚合：根据请求参数决定调用哪些接口
- 字段映射：将下游接口的返回字段重命名/转换
- 数据合并：将多个接口的返回数据合并为一个JSON对象
- 聚合接口可设置超时时间（下游任一接口超时即返回超时错误）
- 聚合接口的Mock数据自动生成（基于各下游接口响应Schema合并）
- 聚合接口调用监控（各下游接口的调用状态与耗时）

### 2.5 接口文档模块
- 自动生成OpenAPI 3.0格式文档
- 支持导出为JSON/YAML格式
- 内置文档预览页面（Swagger UI风格）
- 文档版本发布（发布后生成固定版本号）
- 接口变更通知（订阅者在接口变更时收到通知）

### 2.6 接口调试模块
- 在线调试接口（输入参数，查看实时响应）
- 调试历史记录（保存最近调试记录）
- 请求/响应体格式化展示（JSON高亮/折叠）
- 支持环境变量（开发/测试/生产环境切换）

### 2.7 数据看板模块
- 项目统计：接口数、Mock调用次数、聚合调用次数
- 接口调用趋势图（按天/周）
- 接口响应耗时分布
- 接口状态分布（设计中/已发布/已废弃）

---

## 3. 非功能性需求

### 3.1 性能要求
- Mock接口响应时间 < 100ms（不含延迟配置）
- 串行聚合响应时间 = 各下游接口耗时之和 + 处理时间
- 并行聚合响应时间 = Max(各下游接口耗时) + 处理时间
- 文档页面加载 < 2秒
- 支持并发调用数：初期200 QPS

### 3.2 安全要求
- JWT Token过期时间24小时
- 项目级权限隔离（用户只能看到被授权的项目）
- 敏感请求参数可选择在日志中脱敏
- 接口调用频率限制（每用户/每项目）

### 3.3 可用性要求
- 服务可用率 ≥ 99.5%
- Mock服务与聚合服务支持独立部署
- 下游接口超时时返回部分数据+超时提示
- 系统启动时自动加载所有已发布的接口

### 3.4 扩展性要求
- 支持自定义Mock数据生成函数（用户上传JavaScript脚本）
- 支持接入外部认证系统（OAuth2/LDAP）
- 支持对接API网关（自动同步接口到网关）
- 支持接入监控系统（Prometheus指标暴露）

---

## 4. 技术栈

| 层级 | 技术选型 | 说明 |
|------|----------|------|
| 后端框架 | Gin | HTTP路由与中间件 |
| 数据库 | PostgreSQL | 主数据存储 |
| 缓存 | Redis | Session缓存、接口定义缓存 |
| 前端框架 | React + TypeScript + Ant Design | Web管理界面 |
| 接口文档预览 | Swagger UI | OpenAPI文档展示 |
| JSON Schema处理 | go-jsonschema | Schema校验与Mock生成 |
| 聚合请求客户端 | net/http + goroutine | 并发聚合请求 |
| 认证授权 | JWT | 用户认证与权限 |
| 日志 | Zap | 结构化日志 |
| 配置管理 | Viper | 配置加载 |
| 指标监控 | Prometheus | 接口调用指标（可选） |
| 定时任务 | gocron | 数据清理任务 |

---

## 5. 数据库设计（核心表）

### 5.1 users（用户表）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| email | VARCHAR(255) | 邮箱（唯一） |
| password_hash | VARCHAR(255) | 密码哈希 |
| name | VARCHAR(100) | 用户名 |
| created_at | TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | 更新时间 |

### 5.2 projects（项目表）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| name | VARCHAR(100) | 项目名称 |
| description | TEXT | 描述 |
| base_path | VARCHAR(100) | 基础路径（如 /api/v1） |
| owner_id | UUID | 创建人ID |
| visibility | VARCHAR(20) | public/private |
| created_at | TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | 更新时间 |

### 5.3 project_members（项目成员表）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| project_id | UUID | 项目ID |
| user_id | UUID | 用户ID |
| role | VARCHAR(20) | admin/editor/viewer |
| joined_at | TIMESTAMP | 加入时间 |

### 5.4 apis（接口定义表）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| project_id | UUID | 所属项目ID |
| name | VARCHAR(100) | 接口名称 |
| description | TEXT | 描述 |
| method | VARCHAR(10) | GET/POST/PUT/DELETE/PATCH |
| path | VARCHAR(200) | 接口路径 |
| status | VARCHAR(20) | designing/published/deprecated |
| request_schema | JSON | 请求参数Schema |
| response_schema | JSON | 响应Schema |
| response_example | JSON | 响应示例 |
| mock_delay | INT | Mock延迟（毫秒，默认0） |
| mock_status_code | INT | Mock状态码（默认200） |
| group_id | UUID | 所属分组ID（可选） |
| tags | VARCHAR[] | 标签列表 |
| version | INT | 当前版本号 |
| created_at | TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | 更新时间 |

### 5.5 api_versions（接口版本历史表）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| api_id | UUID | 接口ID |
| version | INT | 版本号 |
| snapshot | JSON | 完整接口定义快照 |
| change_comment | TEXT | 变更说明 |
| created_by | UUID | 变更人 |
| created_at | TIMESTAMP | 变更时间 |

### 5.6 aggregates（聚合接口表）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| project_id | UUID | 所属项目ID |
| name | VARCHAR(100) | 聚合接口名称 |
| description | TEXT | 描述 |
| path | VARCHAR(200) | 聚合接口路径 |
| mode | VARCHAR(20) | serial/parallel/conditional |
| timeout | INT | 超时时间（毫秒） |
| downstream_apis | JSON | 下游接口配置列表 |
| field_mappings | JSON | 字段映射规则 |
| created_at | TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | 更新时间 |

### 5.7 mock_data（Mock数据覆盖表）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| api_id | UUID | 接口ID |
| key | VARCHAR(100) | 数据键（用于请求参数匹配） |
| value | JSON | 固定的Mock响应数据 |
| enabled | BOOLEAN | 是否启用 |
| created_at | TIMESTAMP | 创建时间 |

### 5.8 debug_logs（调试日志表）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| user_id | UUID | 用户ID |
| api_id | UUID | 接口ID（可为空） |
| aggregate_id | UUID | 聚合接口ID（可为空） |
| request | JSON | 请求内容 |
| response | JSON | 响应内容 |
| status_code | INT | 响应状态码 |
| duration | INT | 耗时（毫秒） |
| created_at | TIMESTAMP | 创建时间 |

---

## 6. API接口设计（RESTful）

### 6.1 项目管理
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/projects | 创建项目 |
| GET | /api/v1/projects | 获取项目列表 |
| GET | /api/v1/projects/:id | 获取项目详情 |
| PUT | /api/v1/projects/:id | 更新项目 |
| DELETE | /api/v1/projects/:id | 删除项目 |
| POST | /api/v1/projects/:id/members | 邀请成员 |
| DELETE | /api/v1/projects/:id/members/:userId | 移除成员 |

### 6.2 接口管理
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/projects/:projectId/apis | 创建接口 |
| GET | /api/v1/projects/:projectId/apis | 获取接口列表 |
| GET | /api/v1/apis/:id | 获取接口详情 |
| PUT | /api/v1/apis/:id | 更新接口 |
| DELETE | /api/v1/apis/:id | 删除接口 |
| POST | /api/v1/apis/:id/publish | 发布接口（版本+1） |
| GET | /api/v1/apis/:id/versions | 获取版本历史 |
| POST | /api/v1/apis/:id/rollback/:version | 回滚到指定版本 |

### 6.3 Mock服务
| 方法 | 路径 | 说明 |
|------|------|------|
| ANY | /mock/:projectId/*path | Mock接口调用 |
| POST | /api/v1/apis/:id/mock/override | 设置固定Mock数据 |
| DELETE | /api/v1/apis/:id/mock/override | 清除Mock覆盖 |

### 6.4 聚合代理
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/projects/:projectId/aggregates | 创建聚合接口 |
| GET | /api/v1/projects/:projectId/aggregates | 获取聚合列表 |
| PUT | /api/v1/aggregates/:id | 更新聚合配置 |
| DELETE | /api/v1/aggregates/:id | 删除聚合接口 |
| ANY | /aggregate/:projectId/*path | 聚合接口调用 |

### 6.5 文档与调试
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/projects/:projectId/docs/openapi.json | 导出OpenAPI（JSON） |
| GET | /api/v1/projects/:projectId/docs/openapi.yaml | 导出OpenAPI（YAML） |
| GET | /api/v1/projects/:projectId/docs/preview | 文档预览页面 |
| POST | /api/v1/apis/:id/debug | 接口调试调用 |
| GET | /api/v1/apis/:id/debug/history | 获取调试历史 |

---

## 7. 前端页面清单

| 页面 | 路径 | 说明 |
|------|------|------|
| 登录/注册 | /login, /register | 用户认证 |
| 项目列表 | /projects | 展示所有项目 |
| 项目详情 | /projects/:id | 项目概览与统计 |
| 接口列表 | /projects/:id/apis | 项目下所有接口 |
| 接口定义 | /projects/:id/apis/:apiId | 接口编辑与定义 |
| Mock预览 | /projects/:id/apis/:apiId/mock | 查看Mock数据 |
| 聚合管理 | /projects/:id/aggregates | 聚合接口管理 |
| 接口调试 | /projects/:id/apis/:apiId/debug | 在线调试 |
| 文档预览 | /projects/:id/docs | OpenAPI文档 |
| 项目设置 | /projects/:id/settings | 成员管理与项目配置 |

---

## 8. 部署与运行

### 8.1 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| SERVER_PORT | 服务端口 | 8080 |
| DB_DSN | PostgreSQL连接串 | postgres://user:pass@localhost:5432/api_mock |
| REDIS_ADDR | Redis地址 | localhost:6379 |
| JWT_SECRET | JWT密钥 | （必须设置） |
| MOCK_BASE_URL | Mock服务基础URL | http://localhost:8080/mock |
| AGGREGATE_TIMEOUT | 聚合默认超时 | 3000ms |

### 8.2 启动命令
```bash
# 构建
go build -o api-mock ./cmd/server

# 运行
./api-mock

# Docker运行
docker-compose up -d
```

### 8.3 docker-compose.yml
```yaml
version: '3'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: api_mock
      POSTGRES_USER: api_mock
      POSTGRES_PASSWORD: api_mock123
    ports:
      - "5432:5432"
  redis:
    image: redis:7
    ports:
      - "6379:6379"
  api-mock:
    build: .
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - redis
    environment:
      DB_DSN: postgres://api_mock:api_mock123@postgres:5432/api_mock
      REDIS_ADDR: redis:6379
```