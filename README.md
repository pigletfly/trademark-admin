# 商标管理平台 (trademark-admin)

国际商标智能报价与国际业务管理平台，提供商标申请、报价、客户管理、订单跟踪等一体化功能。

![alt text](public/images/shadcn-admin.png)

> 本项目的 UI 基础改编自 [Shadcn Admin Dashboard](https://github.com/satnaing/shadcn-admin)，在其之上构建业务模块。

## 主要特性

- 浅色 / 深色主题切换
- 响应式布局，适配桌面与移动端
- 符合无障碍（A11y）规范
- 内置侧边栏组件与全局搜索
- 已内置 10+ 业务页面骨架
- 支持 RTL（从右至左）语言方向
- 前后端分离，接口规范统一

<details>
<summary>定制过的 UI 组件（点击展开）</summary>

本项目基于 Shadcn UI，但部分组件针对 RTL 与业务需求做了小幅改造，与官方版本存在差异。

如需通过 Shadcn CLI 更新组件（例如 `npx shadcn@latest add <component>`），未被修改的组件可直接更新；对下方列出的组件，建议手动合并改动，避免覆盖掉 RTL 适配或其它定制逻辑。

> 如果不需要 RTL 支持，可以安全地通过 Shadcn CLI 更新 "RTL 适配组件"，因为它们的变动主要与 RTL 相关。"已修改组件" 则可能包含其它业务层定制，需谨慎处理。

### 已修改组件

- scroll-area
- sonner
- separator

### RTL 适配组件

- alert-dialog
- calendar
- command
- dialog
- dropdown-menu
- select
- table
- sheet
- sidebar
- switch

**说明：**

- **已修改组件**：含有通用修改，可能包含 RTL 调整。
- **RTL 适配组件**：主要针对 RTL 语言方向的布局与位置修改。
- 具体实现请查看 `src/components/ui/` 下的源码。
- 其余 Shadcn UI 组件均为标准版本，可通过 CLI 安全更新。

</details>

## 技术栈

**前端 UI：** [ShadcnUI](https://ui.shadcn.com)（TailwindCSS + RadixUI）

**构建工具：** [Vite](https://vitejs.dev/)

**路由：** [TanStack Router](https://tanstack.com/router/latest)

**类型系统：** [TypeScript](https://www.typescriptlang.org/)

**代码检查 / 格式化：** [ESLint](https://eslint.org/) 与 [Prettier](https://prettier.io/)

**图标：** [Lucide Icons](https://lucide.dev/icons/)、[Tabler Icons](https://tabler.io/icons)（仅品牌图标）

**后端：** Go 1.25 + Gin + GORM + PostgreSQL

**认证（部分）：** [Clerk](https://go.clerk.com/GttUAaK)

## 本地运行

本仓库是一个 pnpm monorepo：

- `apps/web` — React 19 前端（Vite + TanStack Router + Shadcn）
- `apps/api` — Go 1.25 后端（Gin + GORM + PostgreSQL）
- `packages/shared` — 共享类型定义与 OpenAPI schema（占位）

### 环境要求

- Node 22+、pnpm 10+
- Go 1.25+
- Docker Desktop（通过 `docker compose` 运行完整栈时需要）

### 一键启动开发环境

```bash
make up          # 构建镜像并启动 postgres + api + web
curl localhost:8080/health
open http://localhost:5173
make down        # 停止服务
```

### 热重载开发（推荐）

```bash
docker compose up -d postgres   # 只起 postgres
make api                        # 本地运行后端，不带 watch，需手动重启
make dev                        # vite 开发服务器（支持 HMR）
```

### 测试

```bash
make test
cd apps/api && go test -tags=integration ./...   # 需要 Docker
```

### 登录流程冒烟测试（手动）

```bash
make up
sleep 15   # 等待数据库迁移与默认管理员初始化完成

# 登录，cookie 会写入 tm_access_token / tm_refresh_token / tm_csrf_token
curl -sS -c /tmp/tm-cookies.txt \
     -H 'Content-Type: application/json' \
     -d '{"email":"admin@example.com","password":"change-me-on-first-login"}' \
     http://localhost:8080/api/v1/auth/login

# 查看当前用户
curl -sS -b /tmp/tm-cookies.txt http://localhost:8080/api/v1/auth/me

# 管理员：列出用户（非 GET 请求需要 CSRF，此处仅演示 GET）
curl -sS -b /tmp/tm-cookies.txt 'http://localhost:8080/api/v1/admin/users'

make down
rm /tmp/tm-cookies.txt
```

## 鸣谢

- UI 模板来自 [@satnaing](https://github.com/satnaing) 的 [Shadcn Admin Dashboard](https://github.com/satnaing/shadcn-admin)
- 认证服务由 [Clerk](https://go.clerk.com/GttUAaK) 提供

## 开源协议

本项目基于 [MIT License](https://choosealicense.com/licenses/mit/) 开源。
