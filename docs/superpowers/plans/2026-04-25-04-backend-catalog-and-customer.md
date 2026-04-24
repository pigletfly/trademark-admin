# Plan 4: Backend 字典（国家 + 尼斯分类）+ 客户档案 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把字典（countries + nice_categories）与客户档案的后端能力实现到可被前端消费的水平：建表、seed 数据、`/api/v1/catalog/*` 与 `/api/v1/customers` 路由、基于角色的权限，以及审计与集成测试。

**Architecture:** 沿用 Plan 1/2 的模块化单体骨架——每个子域一个 `internal/<name>` 包，内部再按 `model.go / repository.go / service.go / handler.go / dto.go / router.go` 分层。seed 走一个独立的 `pkg/seeder` 包，JSON 数据用 `embed.FS` 嵌入二进制，启动时 upsert（幂等），也通过 `cmd/seed` CLI 手动跑。审计中间件已存在，本计划把它扩到 `authed` 组（所有非 GET 都写 audit log）。前端留到 Plan 5。

**Tech Stack:** Go 1.25 + Gin v1.10 + GORM v1.25 + pgx v5 + PostgreSQL 16 + golang-migrate v4（已接入）+ `embed.FS`（seed JSON）+ testcontainers-go（集成测试）。权限用已存在的 `auth.RequireAuth / auth.RequireRole / auth.CSRF` + `audit.Middleware`。

---

## File Structure

### Create

**Migrations + seed data**
- `apps/api/migrations/000002_catalogs_and_customers.up.sql` — 新增 `countries` / `nice_categories` / `customers` 三张表
- `apps/api/migrations/000002_catalogs_and_customers.down.sql` — 反向 drop
- `apps/api/seed/nice_categories.json` — 全 45 尼斯分类（中英文）
- `apps/api/seed/countries.json` — 约 60 条代表性国家（含 Madrid 成员布尔位、默认审查天数/注册月数）

**Seed 基础设施**
- `apps/api/seed_embed.go` — `package api` 的 `//go:embed all:seed` FS
- `apps/api/pkg/seeder/seeder.go` — 加载 JSON + upsert（与 migrator 包同层级）
- `apps/api/pkg/seeder/seeder_test.go` — testcontainers 集成测试
- `apps/api/cmd/seed/main.go` — 独立 CLI

**Catalog 模块**
- `apps/api/internal/catalog/model.go` — `Country` / `NiceCategory` GORM 模型 + TableName
- `apps/api/internal/catalog/dto.go` — API 请求/响应 struct
- `apps/api/internal/catalog/repository.go` — 读 countries / 读 nice_categories / 更新 country
- `apps/api/internal/catalog/repository_test.go`
- `apps/api/internal/catalog/service.go` — 薄层，封装 repo + 领域校验
- `apps/api/internal/catalog/handler.go` — Gin handler
- `apps/api/internal/catalog/handler_test.go`
- `apps/api/internal/catalog/router.go` — 挂载路由到公共/管理员组

**Customer 模块**
- `apps/api/internal/customer/model.go` — `Customer` GORM 模型 + 软删
- `apps/api/internal/customer/dto.go`
- `apps/api/internal/customer/repository.go` — list/search/分页 + CRUD + 按 created_by 过滤
- `apps/api/internal/customer/repository_test.go`
- `apps/api/internal/customer/service.go` — 负责根据角色决定 owner scope
- `apps/api/internal/customer/handler.go`
- `apps/api/internal/customer/handler_test.go`
- `apps/api/internal/customer/router.go`

### Modify

- `apps/api/cmd/server/main.go` — 启动时跑 seeder + 装配 catalog / customer 路由 + 把 audit middleware 扩到 `authed` 组

---

## Task 1: Migration 000002

**Files:**
- Create: `apps/api/migrations/000002_catalogs_and_customers.up.sql`
- Create: `apps/api/migrations/000002_catalogs_and_customers.down.sql`

- [ ] **Step 1: 写 up migration**

Create `apps/api/migrations/000002_catalogs_and_customers.up.sql`:
```sql
-- Countries dictionary. Code is ISO 3166-1 alpha-2 (2 letters).
CREATE TABLE IF NOT EXISTS countries (
  id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code                        TEXT UNIQUE NOT NULL,
  name_zh                     TEXT NOT NULL,
  name_en                     TEXT NOT NULL,
  is_madrid_member            BOOLEAN NOT NULL DEFAULT FALSE,
  default_acceptance_days     INTEGER,
  default_registration_months INTEGER,
  requires_notarization       BOOLEAN NOT NULL DEFAULT FALSE,
  notes_zh                    TEXT,
  notes_en                    TEXT,
  sort_order                  INTEGER NOT NULL DEFAULT 0,
  enabled                     BOOLEAN NOT NULL DEFAULT TRUE,
  created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_countries_enabled_sort ON countries(enabled, sort_order);
CREATE INDEX IF NOT EXISTS idx_countries_madrid ON countries(is_madrid_member) WHERE enabled;

-- Nice (international trademark) classes. Codes are fixed 1..45.
CREATE TABLE IF NOT EXISTS nice_categories (
  code            INTEGER PRIMARY KEY CHECK (code BETWEEN 1 AND 45),
  name_zh         TEXT NOT NULL,
  name_en         TEXT NOT NULL,
  description_zh  TEXT,
  description_en  TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Customers. Unique (name) among non-deleted rows.
CREATE TABLE IF NOT EXISTS customers (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name            TEXT NOT NULL,
  industry        TEXT,
  is_returning    BOOLEAN NOT NULL DEFAULT FALSE,
  price_sensitive BOOLEAN NOT NULL DEFAULT FALSE,
  contact_name    TEXT,
  contact_phone   TEXT,
  contact_email   TEXT,
  notes           TEXT,
  created_by      UUID NOT NULL REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_customers_name_alive
  ON customers(name)
  WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customers_owner
  ON customers(created_by)
  WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customers_search
  ON customers USING GIN (to_tsvector('simple', coalesce(name, '') || ' ' || coalesce(industry, '')))
  WHERE deleted_at IS NULL;
```

- [ ] **Step 2: 写 down migration**

Create `apps/api/migrations/000002_catalogs_and_customers.down.sql`:
```sql
DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS nice_categories;
DROP TABLE IF EXISTS countries;
```

- [ ] **Step 3: 本地验证 migration 可跑**

```bash
cd /Users/adam/workspace/github/trademark-admin
docker compose up -d postgres
cd apps/api
go build -o /tmp/tm-migrate ./cmd/migrate
DATABASE_URL=postgres://trademark:trademark@localhost:5432/trademark?sslmode=disable /tmp/tm-migrate up
DATABASE_URL=postgres://trademark:trademark@localhost:5432/trademark?sslmode=disable /tmp/tm-migrate version
# Expected: version 2 clean
DATABASE_URL=postgres://trademark:trademark@localhost:5432/trademark?sslmode=disable /tmp/tm-migrate down
DATABASE_URL=postgres://trademark:trademark@localhost:5432/trademark?sslmode=disable /tmp/tm-migrate up
rm /tmp/tm-migrate
cd ../..
docker compose down -v
```
Expected: 无错误；`version` 显示 2 且 clean。

- [ ] **Step 4: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/migrations/000002_catalogs_and_customers.up.sql apps/api/migrations/000002_catalogs_and_customers.down.sql
git commit -m "$(cat <<'EOF'
feat(api): migration 000002 adds countries + nice_categories + customers tables

countries holds the ISO country dictionary (name_zh/en, Madrid membership,
default acceptance/registration timelines, sort order, enabled).
nice_categories is the 45-row trademark class dictionary.
customers is a soft-deleted tenant-lite table, unique on name among live
rows, with a GIN index for name/industry full-text search.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Seed JSON 数据文件

**Files:**
- Create: `apps/api/seed/nice_categories.json`
- Create: `apps/api/seed/countries.json`
- Create: `apps/api/seed_embed.go`

- [ ] **Step 1: 写 `apps/api/seed/nice_categories.json`（45 条完整）**

```json
[
  {"code":1,"name_zh":"化学品","name_en":"Chemicals","description_zh":"工业、科学用化学品；未加工塑料；肥料","description_en":"Chemicals for industry, science; unprocessed plastics; fertilizers"},
  {"code":2,"name_zh":"颜料油漆","name_en":"Paints","description_zh":"颜料、清漆、防腐剂","description_en":"Paints, varnishes, preservatives"},
  {"code":3,"name_zh":"日化用品","name_en":"Cosmetics & Cleaning","description_zh":"化妆品、香水、清洁用品","description_en":"Cosmetics, perfumery, cleaning preparations"},
  {"code":4,"name_zh":"燃料油脂","name_en":"Fuels & Lubricants","description_zh":"工业用油和润滑剂;燃料;照明材料","description_en":"Industrial oils, greases, fuels, illuminants"},
  {"code":5,"name_zh":"医药","name_en":"Pharmaceuticals","description_zh":"医药及兽医用制剂;医用营养品","description_en":"Pharmaceutical and veterinary preparations; medical nutrition"},
  {"code":6,"name_zh":"普通金属","name_en":"Common Metals","description_zh":"普通金属及其合金、金属建材","description_en":"Common metals and alloys; metal building materials"},
  {"code":7,"name_zh":"机械设备","name_en":"Machinery","description_zh":"机器和机床;非手动工具","description_en":"Machines and machine tools; non-hand tools"},
  {"code":8,"name_zh":"手工器械","name_en":"Hand Tools","description_zh":"手工用具;刀叉餐具","description_en":"Hand tools; cutlery"},
  {"code":9,"name_zh":"科学仪器","name_en":"Scientific & Electronics","description_zh":"计算机、软件、科学仪器、电子产品","description_en":"Computers, software, scientific apparatus, electronics"},
  {"code":10,"name_zh":"医疗器械","name_en":"Medical Apparatus","description_zh":"医疗器械和外科器具","description_en":"Medical and surgical apparatus"},
  {"code":11,"name_zh":"家用电器","name_en":"Appliances","description_zh":"照明、加热、蒸汽、烹饪、制冷设备","description_en":"Lighting, heating, cooking, refrigerating apparatus"},
  {"code":12,"name_zh":"运输工具","name_en":"Vehicles","description_zh":"陆、空、水用运输工具","description_en":"Vehicles for locomotion by land, air, water"},
  {"code":13,"name_zh":"军火烟火","name_en":"Firearms & Fireworks","description_zh":"火器;弹药及射弹;爆炸物;烟火","description_en":"Firearms; ammunition; explosives; fireworks"},
  {"code":14,"name_zh":"珠宝钟表","name_en":"Jewelry & Watches","description_zh":"贵重金属及其合金;珠宝;钟表","description_en":"Precious metals; jewelry; horological instruments"},
  {"code":15,"name_zh":"乐器","name_en":"Musical Instruments","description_zh":"乐器及配件","description_en":"Musical instruments"},
  {"code":16,"name_zh":"印刷用品","name_en":"Paper & Printed Matter","description_zh":"纸;纸板;印刷品;文具","description_en":"Paper; cardboard; printed matter; stationery"},
  {"code":17,"name_zh":"橡胶塑料","name_en":"Rubber & Plastics","description_zh":"橡胶、塑料制品(未成品)","description_en":"Rubber, plastics in semi-worked form"},
  {"code":18,"name_zh":"皮革箱包","name_en":"Leather Goods","description_zh":"皮革制品;旅行箱;雨伞","description_en":"Leather goods; luggage; umbrellas"},
  {"code":19,"name_zh":"建筑材料","name_en":"Building Materials","description_zh":"非金属建筑材料","description_en":"Non-metallic building materials"},
  {"code":20,"name_zh":"家具","name_en":"Furniture","description_zh":"家具、镜子、相框","description_en":"Furniture, mirrors, picture frames"},
  {"code":21,"name_zh":"家居用具","name_en":"Household Utensils","description_zh":"家用或厨房器皿;玻璃器皿","description_en":"Household or kitchen utensils; glassware"},
  {"code":22,"name_zh":"绳网原料","name_en":"Ropes & Fibers","description_zh":"绳、绳索、网、帐篷;纺织用纤维原料","description_en":"Ropes, strings, nets, tents; raw textile fibers"},
  {"code":23,"name_zh":"纱线丝线","name_en":"Yarns & Threads","description_zh":"纺织用纱和线","description_en":"Yarns and threads for textile use"},
  {"code":24,"name_zh":"纺织布料","name_en":"Textiles","description_zh":"织物及家居纺织品","description_en":"Textiles and household linen"},
  {"code":25,"name_zh":"服装鞋帽","name_en":"Clothing & Footwear","description_zh":"服装、鞋、帽","description_en":"Clothing, footwear, headgear"},
  {"code":26,"name_zh":"花边饰品","name_en":"Lace & Trimmings","description_zh":"花边、丝带、饰扣","description_en":"Lace, ribbons, trimmings"},
  {"code":27,"name_zh":"地毯壁纸","name_en":"Carpets & Wallpaper","description_zh":"地毯、墙纸、地板覆盖物","description_en":"Carpets, wallpaper, floor coverings"},
  {"code":28,"name_zh":"玩具运动","name_en":"Toys & Sports","description_zh":"玩具、游戏;体育和健身器材;圣诞树装饰","description_en":"Games, toys; sporting and gymnastic articles; Christmas tree decorations"},
  {"code":29,"name_zh":"肉食","name_en":"Meat & Preserved Food","description_zh":"肉;鱼;家禽;加工水果蔬菜;蛋;奶","description_en":"Meat; fish; poultry; processed fruits, vegetables; eggs; dairy"},
  {"code":30,"name_zh":"粮食副食","name_en":"Staple Foods","description_zh":"咖啡、茶、可可;糖、大米;面粉;焙烤食品;调味品","description_en":"Coffee, tea, cocoa; sugar, rice; flour; baked goods; condiments"},
  {"code":31,"name_zh":"农产品","name_en":"Agriculture","description_zh":"未加工的农业产品;活动物;新鲜水果蔬菜","description_en":"Unprocessed agricultural products; live animals; fresh fruits and vegetables"},
  {"code":32,"name_zh":"啤酒饮料","name_en":"Beers & Soft Drinks","description_zh":"啤酒;矿泉水;果汁;不含酒精饮料","description_en":"Beers; mineral water; fruit juices; non-alcoholic beverages"},
  {"code":33,"name_zh":"酒类","name_en":"Alcoholic Beverages","description_zh":"含酒精饮料(啤酒除外)","description_en":"Alcoholic beverages (except beers)"},
  {"code":34,"name_zh":"烟草烟具","name_en":"Tobacco","description_zh":"烟草;烟具;火柴","description_en":"Tobacco; smokers' articles; matches"},
  {"code":35,"name_zh":"广告商业","name_en":"Advertising & Business","description_zh":"广告;商业经营;商业管理;办公事务","description_en":"Advertising; business management; business administration; office functions"},
  {"code":36,"name_zh":"金融保险","name_en":"Finance & Insurance","description_zh":"保险;金融服务;货币事务;不动产事务","description_en":"Insurance; financial affairs; monetary affairs; real estate affairs"},
  {"code":37,"name_zh":"建筑修理","name_en":"Construction & Repair","description_zh":"建筑物建造;修理;安装服务","description_en":"Building construction; repair; installation services"},
  {"code":38,"name_zh":"通讯电信","name_en":"Telecommunications","description_zh":"电信;通讯服务","description_en":"Telecommunications"},
  {"code":39,"name_zh":"运输仓储","name_en":"Transport & Storage","description_zh":"运输;商品包装和储藏;旅行安排","description_en":"Transport; packaging and storage of goods; travel arrangement"},
  {"code":40,"name_zh":"材料加工","name_en":"Material Treatment","description_zh":"材料处理;定制制造","description_en":"Treatment of materials; custom manufacturing"},
  {"code":41,"name_zh":"教育娱乐","name_en":"Education & Entertainment","description_zh":"教育;提供培训;娱乐;文体活动","description_en":"Education; providing of training; entertainment; sporting and cultural activities"},
  {"code":42,"name_zh":"科技研发","name_en":"Scientific & IT Services","description_zh":"科学、技术服务和研究;计算机软件设计与开发","description_en":"Scientific and technological services and research; computer software design and development"},
  {"code":43,"name_zh":"餐饮住宿","name_en":"Food, Drink & Lodging","description_zh":"提供食物和饮料服务;临时住宿","description_en":"Services providing food and drink; temporary accommodation"},
  {"code":44,"name_zh":"医疗美容","name_en":"Medical & Beauty","description_zh":"医疗服务;兽医服务;人或动物保健;美容服务","description_en":"Medical services; veterinary services; hygienic and beauty care"},
  {"code":45,"name_zh":"法律安保","name_en":"Legal & Security","description_zh":"法律服务;安全服务;社交陪伴服务","description_en":"Legal services; security services; personal and social services"}
]
```

- [ ] **Step 2: 写 `apps/api/seed/countries.json`（60 条代表性国家）**

范围覆盖 Madrid Union 主要成员国 + 全部 G20 经济体。不穷举全部 ~130 个 Madrid 成员（超出 MVP 所需），后续按真实业务需要补。每条记录含 ISO alpha-2、中英文名、Madrid 成员布尔、默认受理天数、注册周期月数（经验值，可后续微调）、是否需公证、排序。

Create `apps/api/seed/countries.json`:
```json
[
  {"code":"CN","name_zh":"中国","name_en":"China","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":12,"requires_notarization":false,"sort_order":1},
  {"code":"HK","name_zh":"中国香港","name_en":"Hong Kong","is_madrid_member":false,"default_acceptance_days":30,"default_registration_months":9,"requires_notarization":false,"sort_order":2},
  {"code":"TW","name_zh":"中国台湾","name_en":"Taiwan","is_madrid_member":false,"default_acceptance_days":30,"default_registration_months":8,"requires_notarization":false,"sort_order":3},
  {"code":"MO","name_zh":"中国澳门","name_en":"Macau","is_madrid_member":false,"default_acceptance_days":30,"default_registration_months":9,"requires_notarization":false,"sort_order":4},
  {"code":"US","name_zh":"美国","name_en":"United States","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":10,"requires_notarization":false,"sort_order":10},
  {"code":"GB","name_zh":"英国","name_en":"United Kingdom","is_madrid_member":true,"default_acceptance_days":20,"default_registration_months":4,"requires_notarization":false,"sort_order":11},
  {"code":"EU","name_zh":"欧盟","name_en":"European Union","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":5,"requires_notarization":false,"notes_zh":"EUIPO 单一申请覆盖 27 成员国","notes_en":"EUIPO single filing covers 27 member states","sort_order":12},
  {"code":"DE","name_zh":"德国","name_en":"Germany","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":7,"requires_notarization":false,"sort_order":13},
  {"code":"FR","name_zh":"法国","name_en":"France","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":5,"requires_notarization":false,"sort_order":14},
  {"code":"IT","name_zh":"意大利","name_en":"Italy","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":8,"requires_notarization":false,"sort_order":15},
  {"code":"ES","name_zh":"西班牙","name_en":"Spain","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":8,"requires_notarization":false,"sort_order":16},
  {"code":"NL","name_zh":"荷兰","name_en":"Netherlands","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":5,"requires_notarization":false,"sort_order":17},
  {"code":"CH","name_zh":"瑞士","name_en":"Switzerland","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":6,"requires_notarization":false,"sort_order":18},
  {"code":"SE","name_zh":"瑞典","name_en":"Sweden","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":8,"requires_notarization":false,"sort_order":19},
  {"code":"NO","name_zh":"挪威","name_en":"Norway","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":8,"requires_notarization":false,"sort_order":20},
  {"code":"DK","name_zh":"丹麦","name_en":"Denmark","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":6,"requires_notarization":false,"sort_order":21},
  {"code":"FI","name_zh":"芬兰","name_en":"Finland","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":8,"requires_notarization":false,"sort_order":22},
  {"code":"IE","name_zh":"爱尔兰","name_en":"Ireland","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":7,"requires_notarization":false,"sort_order":23},
  {"code":"PT","name_zh":"葡萄牙","name_en":"Portugal","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":6,"requires_notarization":false,"sort_order":24},
  {"code":"AT","name_zh":"奥地利","name_en":"Austria","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":7,"requires_notarization":false,"sort_order":25},
  {"code":"BE","name_zh":"比利时","name_en":"Belgium","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":5,"requires_notarization":false,"sort_order":26},
  {"code":"GR","name_zh":"希腊","name_en":"Greece","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":10,"requires_notarization":false,"sort_order":27},
  {"code":"PL","name_zh":"波兰","name_en":"Poland","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":9,"requires_notarization":false,"sort_order":28},
  {"code":"CZ","name_zh":"捷克","name_en":"Czech Republic","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":8,"requires_notarization":false,"sort_order":29},
  {"code":"HU","name_zh":"匈牙利","name_en":"Hungary","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":8,"requires_notarization":false,"sort_order":30},
  {"code":"RU","name_zh":"俄罗斯","name_en":"Russia","is_madrid_member":true,"default_acceptance_days":60,"default_registration_months":10,"requires_notarization":true,"sort_order":40},
  {"code":"TR","name_zh":"土耳其","name_en":"Türkiye","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":9,"requires_notarization":false,"sort_order":41},
  {"code":"JP","name_zh":"日本","name_en":"Japan","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":10,"requires_notarization":false,"sort_order":50},
  {"code":"KR","name_zh":"韩国","name_en":"South Korea","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":10,"requires_notarization":false,"sort_order":51},
  {"code":"SG","name_zh":"新加坡","name_en":"Singapore","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":8,"requires_notarization":false,"sort_order":52},
  {"code":"MY","name_zh":"马来西亚","name_en":"Malaysia","is_madrid_member":true,"default_acceptance_days":60,"default_registration_months":12,"requires_notarization":false,"sort_order":53},
  {"code":"TH","name_zh":"泰国","name_en":"Thailand","is_madrid_member":true,"default_acceptance_days":60,"default_registration_months":14,"requires_notarization":false,"sort_order":54},
  {"code":"ID","name_zh":"印度尼西亚","name_en":"Indonesia","is_madrid_member":true,"default_acceptance_days":60,"default_registration_months":18,"requires_notarization":false,"sort_order":55},
  {"code":"PH","name_zh":"菲律宾","name_en":"Philippines","is_madrid_member":true,"default_acceptance_days":60,"default_registration_months":12,"requires_notarization":false,"sort_order":56},
  {"code":"VN","name_zh":"越南","name_en":"Vietnam","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":15,"requires_notarization":false,"sort_order":57},
  {"code":"IN","name_zh":"印度","name_en":"India","is_madrid_member":true,"default_acceptance_days":90,"default_registration_months":24,"requires_notarization":false,"sort_order":58},
  {"code":"AE","name_zh":"阿联酋","name_en":"United Arab Emirates","is_madrid_member":true,"default_acceptance_days":60,"default_registration_months":10,"requires_notarization":true,"sort_order":60},
  {"code":"SA","name_zh":"沙特阿拉伯","name_en":"Saudi Arabia","is_madrid_member":true,"default_acceptance_days":60,"default_registration_months":12,"requires_notarization":true,"sort_order":61},
  {"code":"IL","name_zh":"以色列","name_en":"Israel","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":12,"requires_notarization":false,"sort_order":62},
  {"code":"EG","name_zh":"埃及","name_en":"Egypt","is_madrid_member":true,"default_acceptance_days":60,"default_registration_months":18,"requires_notarization":true,"sort_order":63},
  {"code":"MA","name_zh":"摩洛哥","name_en":"Morocco","is_madrid_member":true,"default_acceptance_days":60,"default_registration_months":12,"requires_notarization":false,"sort_order":64},
  {"code":"ZA","name_zh":"南非","name_en":"South Africa","is_madrid_member":false,"default_acceptance_days":60,"default_registration_months":24,"requires_notarization":false,"notes_zh":"非 Madrid 成员,需走单一申请","notes_en":"Not a Madrid member; single filing required","sort_order":65},
  {"code":"NG","name_zh":"尼日利亚","name_en":"Nigeria","is_madrid_member":false,"default_acceptance_days":90,"default_registration_months":24,"requires_notarization":true,"sort_order":66},
  {"code":"KE","name_zh":"肯尼亚","name_en":"Kenya","is_madrid_member":true,"default_acceptance_days":60,"default_registration_months":18,"requires_notarization":false,"sort_order":67},
  {"code":"CA","name_zh":"加拿大","name_en":"Canada","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":18,"requires_notarization":false,"sort_order":70},
  {"code":"MX","name_zh":"墨西哥","name_en":"Mexico","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":8,"requires_notarization":false,"sort_order":71},
  {"code":"BR","name_zh":"巴西","name_en":"Brazil","is_madrid_member":true,"default_acceptance_days":60,"default_registration_months":12,"requires_notarization":false,"sort_order":72},
  {"code":"AR","name_zh":"阿根廷","name_en":"Argentina","is_madrid_member":false,"default_acceptance_days":60,"default_registration_months":18,"requires_notarization":true,"sort_order":73},
  {"code":"CL","name_zh":"智利","name_en":"Chile","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":10,"requires_notarization":false,"sort_order":74},
  {"code":"CO","name_zh":"哥伦比亚","name_en":"Colombia","is_madrid_member":true,"default_acceptance_days":60,"default_registration_months":12,"requires_notarization":false,"sort_order":75},
  {"code":"PE","name_zh":"秘鲁","name_en":"Peru","is_madrid_member":false,"default_acceptance_days":60,"default_registration_months":10,"requires_notarization":true,"sort_order":76},
  {"code":"AU","name_zh":"澳大利亚","name_en":"Australia","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":7,"requires_notarization":false,"sort_order":80},
  {"code":"NZ","name_zh":"新西兰","name_en":"New Zealand","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":6,"requires_notarization":false,"sort_order":81},
  {"code":"RO","name_zh":"罗马尼亚","name_en":"Romania","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":7,"requires_notarization":false,"sort_order":90},
  {"code":"BG","name_zh":"保加利亚","name_en":"Bulgaria","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":9,"requires_notarization":false,"sort_order":91},
  {"code":"HR","name_zh":"克罗地亚","name_en":"Croatia","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":8,"requires_notarization":false,"sort_order":92},
  {"code":"SK","name_zh":"斯洛伐克","name_en":"Slovakia","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":8,"requires_notarization":false,"sort_order":93},
  {"code":"SI","name_zh":"斯洛文尼亚","name_en":"Slovenia","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":8,"requires_notarization":false,"sort_order":94},
  {"code":"UA","name_zh":"乌克兰","name_en":"Ukraine","is_madrid_member":true,"default_acceptance_days":60,"default_registration_months":12,"requires_notarization":false,"sort_order":95},
  {"code":"BY","name_zh":"白俄罗斯","name_en":"Belarus","is_madrid_member":true,"default_acceptance_days":60,"default_registration_months":12,"requires_notarization":true,"sort_order":96},
  {"code":"KZ","name_zh":"哈萨克斯坦","name_en":"Kazakhstan","is_madrid_member":true,"default_acceptance_days":60,"default_registration_months":12,"requires_notarization":true,"sort_order":97}
]
```

- [ ] **Step 3: 创建 embed FS 入口**

Create `apps/api/seed_embed.go`:
```go
package api

import "embed"

// SeedFS exposes all seed JSON files as an embedded read-only file system.
// Consumers (pkg/seeder, cmd/seed) read from this to keep the binary
// hermetic and avoid path discovery at runtime.
//
//go:embed all:seed
var SeedFS embed.FS
```

- [ ] **Step 4: 编译确认 embed 无语法错**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go build ./...
```
Expected: succeed (no output).

- [ ] **Step 5: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/seed/ apps/api/seed_embed.go
git commit -m "$(cat <<'EOF'
feat(api): bilingual seed data for nice_categories (45) and countries (60)

JSON files embedded via apps/api/seed_embed.go so the binary is hermetic
and seed upserts do not rely on runtime file-system paths. Country data
covers the primary Madrid Union members plus the G20; it is not
exhaustive and can be appended to as business requirements grow.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Seeder 包（幂等 upsert）

**Files:**
- Create: `apps/api/pkg/seeder/seeder.go`
- Create: `apps/api/pkg/seeder/seeder_test.go`

- [ ] **Step 1: 写 seeder 包**

Create `apps/api/pkg/seeder/seeder.go`:
```go
// Package seeder loads JSON seed data from an embed.FS and upserts it into
// Postgres. Upserts are idempotent: ON CONFLICT DO UPDATE, so re-running
// is safe.
package seeder

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SeedCountry mirrors apps/api/seed/countries.json rows.
type SeedCountry struct {
	Code                      string  `json:"code"`
	NameZh                    string  `json:"name_zh"`
	NameEn                    string  `json:"name_en"`
	IsMadridMember            bool    `json:"is_madrid_member"`
	DefaultAcceptanceDays     *int    `json:"default_acceptance_days,omitempty"`
	DefaultRegistrationMonths *int    `json:"default_registration_months,omitempty"`
	RequiresNotarization      bool    `json:"requires_notarization"`
	NotesZh                   *string `json:"notes_zh,omitempty"`
	NotesEn                   *string `json:"notes_en,omitempty"`
	SortOrder                 int     `json:"sort_order"`
}

// SeedNiceCategory mirrors apps/api/seed/nice_categories.json rows.
type SeedNiceCategory struct {
	Code          int     `json:"code"`
	NameZh        string  `json:"name_zh"`
	NameEn        string  `json:"name_en"`
	DescriptionZh *string `json:"description_zh,omitempty"`
	DescriptionEn *string `json:"description_en,omitempty"`
}

// Run loads both seed files from seedFS and upserts them inside a single
// transaction. countriesPath and categoriesPath are paths relative to
// seedFS root (e.g. "seed/countries.json").
func Run(ctx context.Context, db *gorm.DB, seedFS fs.FS, countriesPath, categoriesPath string) error {
	countries, err := loadCountries(seedFS, countriesPath)
	if err != nil {
		return fmt.Errorf("load countries: %w", err)
	}
	categories, err := loadNiceCategories(seedFS, categoriesPath)
	if err != nil {
		return fmt.Errorf("load nice_categories: %w", err)
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := upsertCountries(tx, countries); err != nil {
			return fmt.Errorf("upsert countries: %w", err)
		}
		if err := upsertNiceCategories(tx, categories); err != nil {
			return fmt.Errorf("upsert nice_categories: %w", err)
		}
		return nil
	})
}

func loadCountries(seedFS fs.FS, path string) ([]SeedCountry, error) {
	raw, err := fs.ReadFile(seedFS, path)
	if err != nil {
		return nil, err
	}
	var rows []SeedCountry
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func loadNiceCategories(seedFS fs.FS, path string) ([]SeedNiceCategory, error) {
	raw, err := fs.ReadFile(seedFS, path)
	if err != nil {
		return nil, err
	}
	var rows []SeedNiceCategory
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func upsertCountries(tx *gorm.DB, rows []SeedCountry) error {
	if len(rows) == 0 {
		return nil
	}
	payload := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		payload = append(payload, map[string]any{
			"code":                        r.Code,
			"name_zh":                     r.NameZh,
			"name_en":                     r.NameEn,
			"is_madrid_member":            r.IsMadridMember,
			"default_acceptance_days":     r.DefaultAcceptanceDays,
			"default_registration_months": r.DefaultRegistrationMonths,
			"requires_notarization":       r.RequiresNotarization,
			"notes_zh":                    r.NotesZh,
			"notes_en":                    r.NotesEn,
			"sort_order":                  r.SortOrder,
		})
	}
	// NOTE: use clause.Assignments (not AssignmentColumns): the JSON payload
	// does not carry updated_at, so EXCLUDED.updated_at would be NULL and
	// break the NOT NULL constraint. We set updated_at = NOW() explicitly.
	return tx.Table("countries").Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "code"}},
		DoUpdates: clause.Assignments(map[string]any{
			"name_zh":                     gorm.Expr("EXCLUDED.name_zh"),
			"name_en":                     gorm.Expr("EXCLUDED.name_en"),
			"is_madrid_member":            gorm.Expr("EXCLUDED.is_madrid_member"),
			"default_acceptance_days":     gorm.Expr("EXCLUDED.default_acceptance_days"),
			"default_registration_months": gorm.Expr("EXCLUDED.default_registration_months"),
			"requires_notarization":       gorm.Expr("EXCLUDED.requires_notarization"),
			"notes_zh":                    gorm.Expr("EXCLUDED.notes_zh"),
			"notes_en":                    gorm.Expr("EXCLUDED.notes_en"),
			"sort_order":                  gorm.Expr("EXCLUDED.sort_order"),
			"updated_at":                  gorm.Expr("NOW()"),
		}),
	}).Create(&payload).Error
}

func upsertNiceCategories(tx *gorm.DB, rows []SeedNiceCategory) error {
	if len(rows) == 0 {
		return nil
	}
	payload := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		payload = append(payload, map[string]any{
			"code":           r.Code,
			"name_zh":        r.NameZh,
			"name_en":        r.NameEn,
			"description_zh": r.DescriptionZh,
			"description_en": r.DescriptionEn,
		})
	}
	return tx.Table("nice_categories").Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "code"}},
		DoUpdates: clause.Assignments(map[string]any{
			"name_zh":        gorm.Expr("EXCLUDED.name_zh"),
			"name_en":        gorm.Expr("EXCLUDED.name_en"),
			"description_zh": gorm.Expr("EXCLUDED.description_zh"),
			"description_en": gorm.Expr("EXCLUDED.description_en"),
			"updated_at":     gorm.Expr("NOW()"),
		}),
	}).Create(&payload).Error
}
```

- [ ] **Step 2: 写 seeder 集成测试**

Create `apps/api/pkg/seeder/seeder_test.go`:
```go
package seeder_test

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	gpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/seeder"
)

// newTestDB spins up a pg container, runs embedded migrations and returns a GORM handle.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("seedertest"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	mig, err := migrator.New(api.Migrations, "migrations", dsn)
	require.NoError(t, err)
	require.NoError(t, mig.Up())
	require.NoError(t, mig.Close())

	db, err := gorm.Open(gpostgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	return db
}

// minimalFS contains only the two seed files we feed to seeder.Run.
func minimalFS() fstest.MapFS {
	return fstest.MapFS{
		"seed/countries.json": &fstest.MapFile{Data: []byte(`[
			{"code":"CN","name_zh":"中国","name_en":"China","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":12,"requires_notarization":false,"sort_order":1},
			{"code":"US","name_zh":"美国","name_en":"United States","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":10,"requires_notarization":false,"sort_order":10}
		]`)},
		"seed/nice_categories.json": &fstest.MapFile{Data: []byte(`[
			{"code":1,"name_zh":"化学品","name_en":"Chemicals"},
			{"code":35,"name_zh":"广告商业","name_en":"Advertising"}
		]`)},
	}
}

func TestRun_InsertsThenUpdatesIdempotently(t *testing.T) {
	db := newTestDB(t)
	fs := minimalFS()

	// First run: insert.
	require.NoError(t, seeder.Run(context.Background(), db, fs, "seed/countries.json", "seed/nice_categories.json"))

	var countCountries, countCategories int64
	require.NoError(t, db.Table("countries").Count(&countCountries).Error)
	require.NoError(t, db.Table("nice_categories").Count(&countCategories).Error)
	require.EqualValues(t, 2, countCountries)
	require.EqualValues(t, 2, countCategories)

	// Second run with same data: no duplicates.
	require.NoError(t, seeder.Run(context.Background(), db, fs, "seed/countries.json", "seed/nice_categories.json"))
	require.NoError(t, db.Table("countries").Count(&countCountries).Error)
	require.EqualValues(t, 2, countCountries)

	// Mutate seed data and re-run: existing row updates.
	modified := fstest.MapFS{
		"seed/countries.json": &fstest.MapFile{Data: []byte(`[
			{"code":"CN","name_zh":"中国-Updated","name_en":"China","is_madrid_member":true,"default_acceptance_days":45,"default_registration_months":15,"requires_notarization":false,"sort_order":1},
			{"code":"US","name_zh":"美国","name_en":"United States","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":10,"requires_notarization":false,"sort_order":10}
		]`)},
		"seed/nice_categories.json": &fstest.MapFile{Data: []byte(`[
			{"code":1,"name_zh":"化学品","name_en":"Chemicals"},
			{"code":35,"name_zh":"广告商业","name_en":"Advertising"}
		]`)},
	}
	require.NoError(t, seeder.Run(context.Background(), db, modified, "seed/countries.json", "seed/nice_categories.json"))

	var cn struct {
		NameZh                string
		DefaultAcceptanceDays int
	}
	require.NoError(t, db.Table("countries").Select("name_zh, default_acceptance_days").Where("code = ?", "CN").Scan(&cn).Error)
	require.Equal(t, "中国-Updated", cn.NameZh)
	require.EqualValues(t, 45, cn.DefaultAcceptanceDays)
}

func TestRun_WithRealEmbeddedFS(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, seeder.Run(context.Background(), db, api.SeedFS, "seed/countries.json", "seed/nice_categories.json"))

	var countryCount, catCount int64
	require.NoError(t, db.Table("countries").Count(&countryCount).Error)
	require.NoError(t, db.Table("nice_categories").Count(&catCount).Error)
	require.GreaterOrEqual(t, countryCount, int64(60))
	require.EqualValues(t, 45, catCount)
}
```

注意：`gorm.io/driver/postgres` 和 `testcontainers-go/modules/postgres` 同名。用别名 `gpostgres "gorm.io/driver/postgres"` 避免冲突；`postgres` 就留给 testcontainers 包使用。

- [ ] **Step 3: 跑测试**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./pkg/seeder/...
```
Expected: 2 PASS; 用时 ~15-30s(因 docker 启动)。

如果失败：
- `cannot find package "api/pkg/seeder"` —— 检查 import 路径是否用 `github.com/pigletfly/trademark-admin/apps/api/pkg/seeder`
- docker 未启动 —— 先 `open -a Docker`
- `column "updated_at" violates not-null constraint` —— 确认 OnConflict DoUpdates 用的是 `gorm.Expr("NOW()")` 赋值而非 AssignmentColumns

- [ ] **Step 4: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/pkg/seeder/seeder.go apps/api/pkg/seeder/seeder_test.go
git commit -m "$(cat <<'EOF'
feat(api): seeder pkg loads countries + nice_categories JSON and upserts

Run() wraps both upserts in a single transaction. ON CONFLICT sets
updated_at = NOW() explicitly because EXCLUDED.updated_at would be NULL
for rows without that field in the JSON payload. Covered by a
testcontainers integration test that asserts insert + idempotency +
row update on repeated runs, plus a sanity test against the real
embedded SeedFS.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: cmd/seed CLI + 启动时自动 seed

**Files:**
- Create: `apps/api/cmd/seed/main.go`
- Modify: `apps/api/cmd/server/main.go`

- [ ] **Step 1: 写独立 CLI**

Create `apps/api/cmd/seed/main.go`:
```go
// Command seed upserts the embedded catalog JSON into the configured database.
// Idempotent; safe to run repeatedly. Used for manual re-seeds after an
// operator updates the JSON files in-repo.
package main

import (
	"context"
	"os"
	"time"

	gpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/config"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/logger"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/seeder"
)

func main() {
	log := logger.New()

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "error", err)
		os.Exit(1)
	}

	db, err := gorm.Open(gpostgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Error("open db", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := seeder.Run(ctx, db, api.SeedFS, "seed/countries.json", "seed/nice_categories.json"); err != nil {
		log.Error("seed", "error", err)
		os.Exit(1)
	}
	log.Info("seed complete")
}
```

- [ ] **Step 2: 修 server/main.go —— migrate 之后自动 seed**

Open `apps/api/cmd/server/main.go` and find the block after `log.Info("migrations applied")` (around line 44). Insert seed after migrations, before opening the `db` for GORM handle. But seed needs a GORM handle too. So: open `db`, then seed, then proceed with auth/bootstrap.

Current sequence (verified by reading the file):
```go
if err := mig.Up(); err != nil { ... }
_ = mig.Close()
log.Info("migrations applied")

db, err := database.Open(cfg.DatabaseURL)
...
defer func() { _ = database.Close(db) }()

// Build auth service.
authRepo := auth.NewRepository(db)
```

Insert between `defer database.Close` and `Build auth service.`:
```go
	// Seed catalog dictionaries (idempotent).
	{
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := seeder.Run(ctx, db, api.SeedFS, "seed/countries.json", "seed/nice_categories.json")
		cancel()
		if err != nil {
			log.Error("seed catalog", "error", err)
			os.Exit(1)
		}
		log.Info("catalog seeded")
	}

```

Add the import at the top of `main.go` (inside the existing import block):
```go
	"github.com/pigletfly/trademark-admin/apps/api/pkg/seeder"
```

注意 `api` 别名已在 import block 存在（`api "github.com/pigletfly/trademark-admin/apps/api"`），`api.SeedFS` 直接可用。

- [ ] **Step 3: 构建+跑**

```bash
cd /Users/adam/workspace/github/trademark-admin
docker compose up -d postgres
cd apps/api
go build ./...
go build -o /tmp/tm-seed ./cmd/seed
DATABASE_URL=postgres://trademark:trademark@localhost:5432/trademark?sslmode=disable /tmp/tm-seed
# Expected logs include "seed complete"
rm /tmp/tm-seed
cd ../..
docker compose down -v
```
Expected: 无错误；日志有 "seed complete"。

- [ ] **Step 4: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/cmd/seed/main.go apps/api/cmd/server/main.go
git commit -m "$(cat <<'EOF'
feat(api): auto-seed catalogs on server start + dedicated cmd/seed CLI

Server main now seeds countries + nice_categories right after migrations
are applied and before the auth service comes up; both operations are
idempotent so a cold server restart is safe. The new cmd/seed allows
operators to re-run the upsert after editing the embedded JSON without
restarting the API server.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Catalog model + repository

**Files:**
- Create: `apps/api/internal/catalog/model.go`
- Create: `apps/api/internal/catalog/repository.go`
- Create: `apps/api/internal/catalog/repository_test.go`

- [ ] **Step 1: 写 model**

Create `apps/api/internal/catalog/model.go`:
```go
package catalog

import (
	"time"

	"github.com/google/uuid"
)

// Country mirrors the countries table.
type Country struct {
	ID                        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code                      string    `gorm:"uniqueIndex;not null"`
	NameZh                    string    `gorm:"not null"`
	NameEn                    string    `gorm:"not null"`
	IsMadridMember            bool      `gorm:"not null;default:false"`
	DefaultAcceptanceDays     *int
	DefaultRegistrationMonths *int
	RequiresNotarization      bool    `gorm:"not null;default:false"`
	NotesZh                   *string
	NotesEn                   *string
	SortOrder                 int  `gorm:"not null;default:0"`
	Enabled                   bool `gorm:"not null;default:true"`
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

// TableName pins the GORM mapping.
func (Country) TableName() string { return "countries" }

// NiceCategory mirrors the nice_categories table.
type NiceCategory struct {
	Code          int    `gorm:"primaryKey"`
	NameZh        string `gorm:"not null"`
	NameEn        string `gorm:"not null"`
	DescriptionZh *string
	DescriptionEn *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (NiceCategory) TableName() string { return "nice_categories" }
```

- [ ] **Step 2: 写 repository**

Create `apps/api/internal/catalog/repository.go`:
```go
package catalog

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrNotFound is returned when a country or category does not exist.
var ErrNotFound = errors.New("catalog: not found")

// Repository wraps DB access for catalog dictionaries.
type Repository struct{ db *gorm.DB }

// NewRepository builds a Repository.
func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// ListCountries returns all enabled countries sorted by sort_order then code.
// onlyEnabled=false returns everything (for admin view).
func (r *Repository) ListCountries(ctx context.Context, onlyEnabled bool) ([]Country, error) {
	var rows []Country
	q := r.db.WithContext(ctx).Order("sort_order ASC, code ASC")
	if onlyEnabled {
		q = q.Where("enabled = TRUE")
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetCountry fetches a country by id.
func (r *Repository) GetCountry(ctx context.Context, id uuid.UUID) (*Country, error) {
	var row Country
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// UpdateCountry applies the admin-settable fields. Only fields present in
// patch (non-nil) are updated.
type CountryPatch struct {
	NameZh                    *string
	NameEn                    *string
	IsMadridMember            *bool
	DefaultAcceptanceDays     *int
	DefaultRegistrationMonths *int
	RequiresNotarization      *bool
	NotesZh                   *string
	NotesEn                   *string
	SortOrder                 *int
	Enabled                   *bool
}

func (r *Repository) UpdateCountry(ctx context.Context, id uuid.UUID, patch CountryPatch) (*Country, error) {
	updates := map[string]any{}
	if patch.NameZh != nil {
		updates["name_zh"] = *patch.NameZh
	}
	if patch.NameEn != nil {
		updates["name_en"] = *patch.NameEn
	}
	if patch.IsMadridMember != nil {
		updates["is_madrid_member"] = *patch.IsMadridMember
	}
	if patch.DefaultAcceptanceDays != nil {
		updates["default_acceptance_days"] = *patch.DefaultAcceptanceDays
	}
	if patch.DefaultRegistrationMonths != nil {
		updates["default_registration_months"] = *patch.DefaultRegistrationMonths
	}
	if patch.RequiresNotarization != nil {
		updates["requires_notarization"] = *patch.RequiresNotarization
	}
	if patch.NotesZh != nil {
		updates["notes_zh"] = *patch.NotesZh
	}
	if patch.NotesEn != nil {
		updates["notes_en"] = *patch.NotesEn
	}
	if patch.SortOrder != nil {
		updates["sort_order"] = *patch.SortOrder
	}
	if patch.Enabled != nil {
		updates["enabled"] = *patch.Enabled
	}
	if len(updates) == 0 {
		return r.GetCountry(ctx, id)
	}
	updates["updated_at"] = gorm.Expr("NOW()")

	res := r.db.WithContext(ctx).Model(&Country{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.GetCountry(ctx, id)
}

// ListNiceCategories returns all 45 categories, ordered by code.
func (r *Repository) ListNiceCategories(ctx context.Context) ([]NiceCategory, error) {
	var rows []NiceCategory
	if err := r.db.WithContext(ctx).Order("code ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
```

- [ ] **Step 3: 写 repository 集成测试**

Create `apps/api/internal/catalog/repository_test.go`:
```go
package catalog_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	gpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/catalog"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/seeder"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("catalogtest"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	mig, err := migrator.New(api.Migrations, "migrations", dsn)
	require.NoError(t, err)
	require.NoError(t, mig.Up())
	require.NoError(t, mig.Close())

	db, err := gorm.Open(gpostgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, seeder.Run(ctx, db, api.SeedFS, "seed/countries.json", "seed/nice_categories.json"))
	return db
}

func TestRepository_ListCountriesOrdered(t *testing.T) {
	db := newDB(t)
	repo := catalog.NewRepository(db)

	rows, err := repo.ListCountries(context.Background(), true)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rows), 60)
	require.Equal(t, "CN", rows[0].Code) // sort_order=1
}

func TestRepository_UpdateCountry_PartialPatch(t *testing.T) {
	db := newDB(t)
	repo := catalog.NewRepository(db)

	rows, err := repo.ListCountries(context.Background(), true)
	require.NoError(t, err)

	cn := rows[0]
	require.Equal(t, "CN", cn.Code)

	newDays := 45
	updated, err := repo.UpdateCountry(context.Background(), cn.ID, catalog.CountryPatch{
		DefaultAcceptanceDays: &newDays,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.DefaultAcceptanceDays)
	require.Equal(t, 45, *updated.DefaultAcceptanceDays)
	require.Equal(t, "中国", updated.NameZh) // untouched
}

func TestRepository_UpdateCountry_NotFound(t *testing.T) {
	db := newDB(t)
	repo := catalog.NewRepository(db)

	_, err := repo.UpdateCountry(context.Background(), uuid.New(), catalog.CountryPatch{})
	require.ErrorIs(t, err, catalog.ErrNotFound)
}

func TestRepository_ListNiceCategoriesAll45(t *testing.T) {
	db := newDB(t)
	repo := catalog.NewRepository(db)

	rows, err := repo.ListNiceCategories(context.Background())
	require.NoError(t, err)
	require.Equal(t, 45, len(rows))
	require.Equal(t, 1, rows[0].Code)
	require.Equal(t, 45, rows[44].Code)
}
```

- [ ] **Step 4: 跑测试**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/catalog/...
```
Expected: 4 PASS.

- [ ] **Step 5: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/internal/catalog/model.go apps/api/internal/catalog/repository.go apps/api/internal/catalog/repository_test.go
git commit -m "$(cat <<'EOF'
feat(api): catalog repository with GORM models + partial-patch country update

Country / NiceCategory GORM structs with TableName pinned. Repository
methods: list countries (filterable by enabled), get country by id,
partial patch country by field (only non-nil fields are SET), list all
nice categories. Testcontainers suite covers ordering, partial update,
not-found, and the 45-category constant.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Catalog service + handler + router（读 + PATCH）

**Files:**
- Create: `apps/api/internal/catalog/dto.go`
- Create: `apps/api/internal/catalog/service.go`
- Create: `apps/api/internal/catalog/handler.go`
- Create: `apps/api/internal/catalog/handler_test.go`
- Create: `apps/api/internal/catalog/router.go`

- [ ] **Step 1: 写 DTO**

Create `apps/api/internal/catalog/dto.go`:
```go
package catalog

import "github.com/google/uuid"

// CountryDTO is the wire representation of a country row.
type CountryDTO struct {
	ID                        uuid.UUID `json:"id"`
	Code                      string    `json:"code"`
	NameZh                    string    `json:"name_zh"`
	NameEn                    string    `json:"name_en"`
	IsMadridMember            bool      `json:"is_madrid_member"`
	DefaultAcceptanceDays     *int      `json:"default_acceptance_days,omitempty"`
	DefaultRegistrationMonths *int      `json:"default_registration_months,omitempty"`
	RequiresNotarization      bool      `json:"requires_notarization"`
	NotesZh                   *string   `json:"notes_zh,omitempty"`
	NotesEn                   *string   `json:"notes_en,omitempty"`
	SortOrder                 int       `json:"sort_order"`
	Enabled                   bool      `json:"enabled"`
}

// NiceCategoryDTO is the wire representation of a nice category.
type NiceCategoryDTO struct {
	Code          int     `json:"code"`
	NameZh        string  `json:"name_zh"`
	NameEn        string  `json:"name_en"`
	DescriptionZh *string `json:"description_zh,omitempty"`
	DescriptionEn *string `json:"description_en,omitempty"`
}

// UpdateCountryRequest — all fields optional; only present (non-nil) fields are applied.
type UpdateCountryRequest struct {
	NameZh                    *string `json:"name_zh,omitempty"`
	NameEn                    *string `json:"name_en,omitempty"`
	IsMadridMember            *bool   `json:"is_madrid_member,omitempty"`
	DefaultAcceptanceDays     *int    `json:"default_acceptance_days,omitempty"`
	DefaultRegistrationMonths *int    `json:"default_registration_months,omitempty"`
	RequiresNotarization      *bool   `json:"requires_notarization,omitempty"`
	NotesZh                   *string `json:"notes_zh,omitempty"`
	NotesEn                   *string `json:"notes_en,omitempty"`
	SortOrder                 *int    `json:"sort_order,omitempty"`
	Enabled                   *bool   `json:"enabled,omitempty"`
}

func toCountryDTO(c Country) CountryDTO {
	return CountryDTO{
		ID:                        c.ID,
		Code:                      c.Code,
		NameZh:                    c.NameZh,
		NameEn:                    c.NameEn,
		IsMadridMember:            c.IsMadridMember,
		DefaultAcceptanceDays:     c.DefaultAcceptanceDays,
		DefaultRegistrationMonths: c.DefaultRegistrationMonths,
		RequiresNotarization:      c.RequiresNotarization,
		NotesZh:                   c.NotesZh,
		NotesEn:                   c.NotesEn,
		SortOrder:                 c.SortOrder,
		Enabled:                   c.Enabled,
	}
}

func toNiceCategoryDTO(n NiceCategory) NiceCategoryDTO {
	return NiceCategoryDTO{
		Code:          n.Code,
		NameZh:        n.NameZh,
		NameEn:        n.NameEn,
		DescriptionZh: n.DescriptionZh,
		DescriptionEn: n.DescriptionEn,
	}
}
```

- [ ] **Step 2: 写 service**

Create `apps/api/internal/catalog/service.go`:
```go
package catalog

import (
	"context"

	"github.com/google/uuid"
)

// Service is a thin orchestration layer around Repository. Exists so the
// handler can be unit-tested with a mock service (not yet needed in MVP).
type Service struct{ repo *Repository }

// NewService wires a Service with its Repository.
func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// ListCountries returns countries for API consumers (enabled only).
func (s *Service) ListCountries(ctx context.Context, includeDisabled bool) ([]CountryDTO, error) {
	rows, err := s.repo.ListCountries(ctx, !includeDisabled)
	if err != nil {
		return nil, err
	}
	out := make([]CountryDTO, len(rows))
	for i, r := range rows {
		out[i] = toCountryDTO(r)
	}
	return out, nil
}

// UpdateCountry applies admin-provided patch and returns the new state.
func (s *Service) UpdateCountry(ctx context.Context, id uuid.UUID, req UpdateCountryRequest) (*CountryDTO, error) {
	patch := CountryPatch{
		NameZh:                    req.NameZh,
		NameEn:                    req.NameEn,
		IsMadridMember:            req.IsMadridMember,
		DefaultAcceptanceDays:     req.DefaultAcceptanceDays,
		DefaultRegistrationMonths: req.DefaultRegistrationMonths,
		RequiresNotarization:      req.RequiresNotarization,
		NotesZh:                   req.NotesZh,
		NotesEn:                   req.NotesEn,
		SortOrder:                 req.SortOrder,
		Enabled:                   req.Enabled,
	}
	row, err := s.repo.UpdateCountry(ctx, id, patch)
	if err != nil {
		return nil, err
	}
	dto := toCountryDTO(*row)
	return &dto, nil
}

// ListNiceCategories returns all 45 nice categories.
func (s *Service) ListNiceCategories(ctx context.Context) ([]NiceCategoryDTO, error) {
	rows, err := s.repo.ListNiceCategories(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]NiceCategoryDTO, len(rows))
	for i, r := range rows {
		out[i] = toNiceCategoryDTO(r)
	}
	return out, nil
}
```

- [ ] **Step 3: 写 handler**

Create `apps/api/internal/catalog/handler.go`:
```go
package catalog

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler exposes the catalog HTTP endpoints.
type Handler struct{ svc *Service }

// NewHandler wires a Handler with its Service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// GetCountries GET /catalog/countries[?include_disabled=true]
func (h *Handler) GetCountries(c *gin.Context) {
	includeDisabled := c.Query("include_disabled") == "true"
	rows, err := h.svc.ListCountries(c.Request.Context(), includeDisabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": "failed to list countries"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

// GetNiceCategories GET /catalog/nice-categories
func (h *Handler) GetNiceCategories(c *gin.Context) {
	rows, err := h.svc.ListNiceCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": "failed to list nice categories"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

// PatchCountry PATCH /catalog/countries/:id (admin).
func (h *Handler) PatchCountry(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_ID", "message": "invalid uuid"})
		return
	}
	var req UpdateCountryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	dto, err := h.svc.UpdateCountry(c.Request.Context(), id, req)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND", "message": "country not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": "failed to update country"})
		return
	}
	c.JSON(http.StatusOK, dto)
}
```

- [ ] **Step 4: 写 router**

Create `apps/api/internal/catalog/router.go`:
```go
package catalog

import "github.com/gin-gonic/gin"

// RegisterReadRoutes mounts read endpoints on an authenticated group.
// Any role is allowed — the frontend decides who sees the menu.
func RegisterReadRoutes(authed *gin.RouterGroup, h *Handler) {
	g := authed.Group("/catalog")
	g.GET("/countries", h.GetCountries)
	g.GET("/nice-categories", h.GetNiceCategories)
}

// RegisterAdminRoutes mounts write endpoints on an admin-only group.
func RegisterAdminRoutes(admin *gin.RouterGroup, h *Handler) {
	g := admin.Group("/catalog")
	g.PATCH("/countries/:id", h.PatchCountry)
}
```

- [ ] **Step 5: 写 handler HTTP 集成测试**

Create `apps/api/internal/catalog/handler_test.go`:
```go
package catalog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	gpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/catalog"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/seeder"
)

func setup(t *testing.T) (*gin.Engine, *catalog.Service, *gorm.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("catalogh"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	mig, err := migrator.New(api.Migrations, "migrations", dsn)
	require.NoError(t, err)
	require.NoError(t, mig.Up())
	require.NoError(t, mig.Close())

	db, err := gorm.Open(gpostgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, seeder.Run(ctx, db, api.SeedFS, "seed/countries.json", "seed/nice_categories.json"))

	repo := catalog.NewRepository(db)
	svc := catalog.NewService(repo)
	h := catalog.NewHandler(svc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	// For tests we skip auth/CSRF and mount both groups on the root router.
	v1 := router.Group("/api/v1")
	catalog.RegisterReadRoutes(v1, h)
	catalog.RegisterAdminRoutes(v1, h)
	return router, svc, db
}

func TestGET_Countries_ReturnsSeeded(t *testing.T) {
	router, _, _ := setup(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/countries", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct{ Items []catalog.CountryDTO }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.GreaterOrEqual(t, len(body.Items), 60)
	require.Equal(t, "CN", body.Items[0].Code)
}

func TestGET_NiceCategories_45Rows(t *testing.T) {
	router, _, _ := setup(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/nice-categories", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct{ Items []catalog.NiceCategoryDTO }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, 45, len(body.Items))
}

func TestPATCH_Country_PartialUpdate(t *testing.T) {
	router, svc, _ := setup(t)
	countries, err := svc.ListCountries(context.Background(), false)
	require.NoError(t, err)
	cn := countries[0]

	newDays := 99
	body, _ := json.Marshal(catalog.UpdateCountryRequest{DefaultAcceptanceDays: &newDays})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/catalog/countries/"+cn.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var got catalog.CountryDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.NotNil(t, got.DefaultAcceptanceDays)
	require.Equal(t, 99, *got.DefaultAcceptanceDays)
	require.Equal(t, cn.NameZh, got.NameZh)
}

func TestPATCH_Country_NotFound(t *testing.T) {
	router, _, _ := setup(t)
	body, _ := json.Marshal(catalog.UpdateCountryRequest{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/catalog/countries/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestPATCH_Country_BadUUID(t *testing.T) {
	router, _, _ := setup(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/catalog/countries/not-a-uuid", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}
```

- [ ] **Step 6: 跑测试**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/catalog/...
```
Expected: repository_test.go 的 4 条 + handler_test.go 的 5 条 = 9 PASS。

- [ ] **Step 7: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/internal/catalog/dto.go apps/api/internal/catalog/service.go apps/api/internal/catalog/handler.go apps/api/internal/catalog/handler_test.go apps/api/internal/catalog/router.go
git commit -m "$(cat <<'EOF'
feat(api): catalog HTTP handlers + router for countries + nice_categories

GET /catalog/countries and GET /catalog/nice-categories are authenticated
reads (any role) with include_disabled query toggle for admin views.
PATCH /catalog/countries/:id is admin-only (gated via the admin router
group in main.go). Five-case handler suite covers the golden path and
not-found / bad-uuid branches.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Customer model + repository

**Files:**
- Create: `apps/api/internal/customer/model.go`
- Create: `apps/api/internal/customer/repository.go`
- Create: `apps/api/internal/customer/repository_test.go`

- [ ] **Step 1: 写 model**

Create `apps/api/internal/customer/model.go`:
```go
package customer

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Customer mirrors the customers table. Soft-delete via deleted_at.
type Customer struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey"`
	Name           string         `gorm:"not null"`
	Industry       *string
	IsReturning    bool `gorm:"not null;default:false"`
	PriceSensitive bool `gorm:"not null;default:false"`
	ContactName    *string
	ContactPhone   *string
	ContactEmail   *string
	Notes          *string
	CreatedBy      uuid.UUID `gorm:"type:uuid;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

// TableName pins the GORM mapping.
func (Customer) TableName() string { return "customers" }
```

- [ ] **Step 2: 写 repository**

Create `apps/api/internal/customer/repository.go`:
```go
package customer

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrNotFound indicates no live row matches the query.
var ErrNotFound = errors.New("customer: not found")

// ErrDuplicateName indicates an attempted insert/update violates the name uniqueness constraint.
var ErrDuplicateName = errors.New("customer: duplicate name")

// Repository wraps DB access for customer rows.
type Repository struct{ db *gorm.DB }

// NewRepository builds a Repository.
func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// ListFilter groups list parameters (pagination + search + owner scope).
type ListFilter struct {
	Query       string     // ILIKE on name + industry
	OwnerID     *uuid.UUID // nil = no scope filter (admin/reviewer)
	Page        int        // 1-based
	PageSize    int
}

// ListResult is the paginated list envelope.
type ListResult struct {
	Items    []Customer
	Page     int
	PageSize int
	Total    int64
}

// List returns customers matching filter with pagination.
func (r *Repository) List(ctx context.Context, f ListFilter) (ListResult, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 200 {
		f.PageSize = 20
	}

	q := r.db.WithContext(ctx).Model(&Customer{})
	if f.OwnerID != nil {
		q = q.Where("created_by = ?", *f.OwnerID)
	}
	if trimmed := strings.TrimSpace(f.Query); trimmed != "" {
		// Use parameterized ILIKE. Escape %/_ to prevent wildcard injection.
		esc := escapeLike(trimmed)
		like := "%" + esc + "%"
		q = q.Where("name ILIKE ? OR coalesce(industry,'') ILIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return ListResult{}, err
	}

	var rows []Customer
	err := q.Order("created_at DESC").
		Offset((f.Page - 1) * f.PageSize).
		Limit(f.PageSize).
		Find(&rows).Error
	if err != nil {
		return ListResult{}, err
	}

	return ListResult{Items: rows, Page: f.Page, PageSize: f.PageSize, Total: total}, nil
}

// escapeLike turns user input into a SQL ILIKE-safe pattern fragment.
// Escapes \ first, then % and _, using \ as the escape character.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// Get fetches a customer by id. If ownerID is non-nil, the row must belong to that owner.
func (r *Repository) Get(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*Customer, error) {
	q := r.db.WithContext(ctx).Where("id = ?", id)
	if ownerID != nil {
		q = q.Where("created_by = ?", *ownerID)
	}
	var row Customer
	err := q.Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Create inserts a new customer. Returns ErrDuplicateName on unique violation.
func (r *Repository) Create(ctx context.Context, in *Customer) error {
	if in.ID == uuid.Nil {
		in.ID = uuid.New()
	}
	err := r.db.WithContext(ctx).Create(in).Error
	if isUniqueViolation(err) {
		return ErrDuplicateName
	}
	return err
}

// Patch describes the optional field updates. Only non-nil fields are applied.
type Patch struct {
	Name           *string
	Industry       *string
	IsReturning    *bool
	PriceSensitive *bool
	ContactName    *string
	ContactPhone   *string
	ContactEmail   *string
	Notes          *string
}

// Update applies the patch to the row. If ownerID is non-nil, the update is
// scoped to that owner (rows owned by others remain untouched and ErrNotFound
// is returned).
func (r *Repository) Update(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID, p Patch) (*Customer, error) {
	updates := map[string]any{}
	if p.Name != nil {
		updates["name"] = *p.Name
	}
	if p.Industry != nil {
		updates["industry"] = *p.Industry
	}
	if p.IsReturning != nil {
		updates["is_returning"] = *p.IsReturning
	}
	if p.PriceSensitive != nil {
		updates["price_sensitive"] = *p.PriceSensitive
	}
	if p.ContactName != nil {
		updates["contact_name"] = *p.ContactName
	}
	if p.ContactPhone != nil {
		updates["contact_phone"] = *p.ContactPhone
	}
	if p.ContactEmail != nil {
		updates["contact_email"] = *p.ContactEmail
	}
	if p.Notes != nil {
		updates["notes"] = *p.Notes
	}
	if len(updates) == 0 {
		return r.Get(ctx, id, ownerID)
	}
	updates["updated_at"] = gorm.Expr("NOW()")

	q := r.db.WithContext(ctx).Model(&Customer{}).Where("id = ?", id)
	if ownerID != nil {
		q = q.Where("created_by = ?", *ownerID)
	}
	res := q.Updates(updates)
	if isUniqueViolation(res.Error) {
		return nil, ErrDuplicateName
	}
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, id, ownerID)
}

// isUniqueViolation checks for Postgres 23505 SQLSTATE in the error chain.
// GORM wraps pgconn errors; stringify + substring is pragmatic and the only
// thing that works across drivers without pulling in a hard pgx dependency.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key")
}
```

- [ ] **Step 3: 写 repository 集成测试**

Create `apps/api/internal/customer/repository_test.go`:
```go
package customer_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	gpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
)

// bootstrap spins up pg, migrates, seeds a salesperson role + user, and
// returns the *gorm.DB together with the owner uuid.
func bootstrap(t *testing.T) (*gorm.DB, uuid.UUID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("custtest"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	mig, err := migrator.New(api.Migrations, "migrations", dsn)
	require.NoError(t, err)
	require.NoError(t, mig.Up())
	require.NoError(t, mig.Close())

	db, err := gorm.Open(gpostgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	// Insert a dummy owner user linked to the seeded salesperson role.
	var roleID uuid.UUID
	require.NoError(t, db.Raw("SELECT id FROM roles WHERE code = ?", "salesperson").Scan(&roleID).Error)
	owner := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		owner, "Test Owner", "owner-"+owner.String()+"@test.local", "hash", roleID,
	).Error)

	return db, owner
}

func TestRepository_CreateAndGet(t *testing.T) {
	db, owner := bootstrap(t)
	repo := customer.NewRepository(db)

	c := &customer.Customer{Name: "Acme", CreatedBy: owner}
	require.NoError(t, repo.Create(context.Background(), c))
	require.NotEqual(t, uuid.Nil, c.ID)

	got, err := repo.Get(context.Background(), c.ID, nil)
	require.NoError(t, err)
	require.Equal(t, "Acme", got.Name)
}

func TestRepository_Create_DuplicateName(t *testing.T) {
	db, owner := bootstrap(t)
	repo := customer.NewRepository(db)

	require.NoError(t, repo.Create(context.Background(), &customer.Customer{Name: "Acme", CreatedBy: owner}))
	err := repo.Create(context.Background(), &customer.Customer{Name: "Acme", CreatedBy: owner})
	require.ErrorIs(t, err, customer.ErrDuplicateName)
}

func TestRepository_List_OwnerScoped(t *testing.T) {
	db, owner := bootstrap(t)
	repo := customer.NewRepository(db)

	// Create another owner to seed foreign rows.
	var roleID uuid.UUID
	require.NoError(t, db.Raw("SELECT id FROM roles WHERE code = ?", "salesperson").Scan(&roleID).Error)
	otherOwner := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		otherOwner, "Another", "other-"+otherOwner.String()+"@test.local", "h", roleID,
	).Error)

	require.NoError(t, repo.Create(context.Background(), &customer.Customer{Name: "A-mine", CreatedBy: owner}))
	require.NoError(t, repo.Create(context.Background(), &customer.Customer{Name: "B-mine", CreatedBy: owner}))
	require.NoError(t, repo.Create(context.Background(), &customer.Customer{Name: "C-other", CreatedBy: otherOwner}))

	// Scoped to owner: 2 rows.
	res, err := repo.List(context.Background(), customer.ListFilter{OwnerID: &owner})
	require.NoError(t, err)
	require.EqualValues(t, 2, res.Total)
	require.Len(t, res.Items, 2)

	// Unscoped: 3 rows.
	resAll, err := repo.List(context.Background(), customer.ListFilter{})
	require.NoError(t, err)
	require.EqualValues(t, 3, resAll.Total)
}

func TestRepository_List_QueryIlike(t *testing.T) {
	db, owner := bootstrap(t)
	repo := customer.NewRepository(db)

	indFinance := "Finance"
	indHealth := "Healthcare"
	require.NoError(t, repo.Create(context.Background(), &customer.Customer{Name: "Globex", Industry: &indFinance, CreatedBy: owner}))
	require.NoError(t, repo.Create(context.Background(), &customer.Customer{Name: "Initech", Industry: &indHealth, CreatedBy: owner}))

	res, err := repo.List(context.Background(), customer.ListFilter{Query: "glob"})
	require.NoError(t, err)
	require.EqualValues(t, 1, res.Total)
	require.Equal(t, "Globex", res.Items[0].Name)

	res, err = repo.List(context.Background(), customer.ListFilter{Query: "HEALTH"})
	require.NoError(t, err)
	require.EqualValues(t, 1, res.Total)
	require.Equal(t, "Initech", res.Items[0].Name)
}

func TestRepository_Update_OwnerGuard(t *testing.T) {
	db, owner := bootstrap(t)
	repo := customer.NewRepository(db)

	c := &customer.Customer{Name: "Acme", CreatedBy: owner}
	require.NoError(t, repo.Create(context.Background(), c))

	// Guarded update by the correct owner.
	newName := "Acme Inc"
	got, err := repo.Update(context.Background(), c.ID, &owner, customer.Patch{Name: &newName})
	require.NoError(t, err)
	require.Equal(t, "Acme Inc", got.Name)

	// Guarded update by a different owner: not found.
	other := uuid.New()
	_, err = repo.Update(context.Background(), c.ID, &other, customer.Patch{Name: &newName})
	require.ErrorIs(t, err, customer.ErrNotFound)
}

func TestRepository_Update_DuplicateName(t *testing.T) {
	db, owner := bootstrap(t)
	repo := customer.NewRepository(db)

	a := &customer.Customer{Name: "Alpha", CreatedBy: owner}
	b := &customer.Customer{Name: "Bravo", CreatedBy: owner}
	require.NoError(t, repo.Create(context.Background(), a))
	require.NoError(t, repo.Create(context.Background(), b))

	dup := "Alpha"
	_, err := repo.Update(context.Background(), b.ID, nil, customer.Patch{Name: &dup})
	require.ErrorIs(t, err, customer.ErrDuplicateName)
}
```

- [ ] **Step 4: 跑测试**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/customer/...
```
Expected: 6 PASS.

- [ ] **Step 5: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/internal/customer/model.go apps/api/internal/customer/repository.go apps/api/internal/customer/repository_test.go
git commit -m "$(cat <<'EOF'
feat(api): customer repository with owner-scoped list/update + soft delete

Customer model uses gorm.DeletedAt for soft delete. Repository.List
supports pagination (page/page_size), ILIKE search on name + industry
(with wildcard escaping), and optional owner scoping so salesperson
callers only see their own rows while reviewer/admin pass nil and see
all rows. Update() honours the same owner filter, returning ErrNotFound
when a salesperson tries to patch another owner's row. ErrDuplicateName
wraps the Postgres 23505 SQLSTATE so handlers can map to HTTP 409.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Customer service + handler + router

**Files:**
- Create: `apps/api/internal/customer/dto.go`
- Create: `apps/api/internal/customer/service.go`
- Create: `apps/api/internal/customer/handler.go`
- Create: `apps/api/internal/customer/handler_test.go`
- Create: `apps/api/internal/customer/router.go`

- [ ] **Step 1: 写 DTO**

Create `apps/api/internal/customer/dto.go`:
```go
package customer

import (
	"time"

	"github.com/google/uuid"
)

// CustomerDTO is the wire representation of a customer row.
type CustomerDTO struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Industry       *string   `json:"industry,omitempty"`
	IsReturning    bool      `json:"is_returning"`
	PriceSensitive bool      `json:"price_sensitive"`
	ContactName    *string   `json:"contact_name,omitempty"`
	ContactPhone   *string   `json:"contact_phone,omitempty"`
	ContactEmail   *string   `json:"contact_email,omitempty"`
	Notes          *string   `json:"notes,omitempty"`
	CreatedBy      uuid.UUID `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ListResponse is the paginated list envelope returned to clients.
type ListResponse struct {
	Items    []CustomerDTO `json:"items"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Total    int64         `json:"total"`
}

// CreateRequest — client-submitted body for POST /customers.
type CreateRequest struct {
	Name           string  `json:"name" binding:"required,min=1,max=200"`
	Industry       *string `json:"industry,omitempty"`
	IsReturning    bool    `json:"is_returning"`
	PriceSensitive bool    `json:"price_sensitive"`
	ContactName    *string `json:"contact_name,omitempty"`
	ContactPhone   *string `json:"contact_phone,omitempty"`
	ContactEmail   *string `json:"contact_email,omitempty"`
	Notes          *string `json:"notes,omitempty"`
}

// UpdateRequest — client-submitted body for PATCH /customers/:id.
// All fields optional; only present (non-nil) fields are applied.
type UpdateRequest struct {
	Name           *string `json:"name,omitempty" binding:"omitempty,min=1,max=200"`
	Industry       *string `json:"industry,omitempty"`
	IsReturning    *bool   `json:"is_returning,omitempty"`
	PriceSensitive *bool   `json:"price_sensitive,omitempty"`
	ContactName    *string `json:"contact_name,omitempty"`
	ContactPhone   *string `json:"contact_phone,omitempty"`
	ContactEmail   *string `json:"contact_email,omitempty"`
	Notes          *string `json:"notes,omitempty"`
}

func toDTO(c Customer) CustomerDTO {
	return CustomerDTO{
		ID:             c.ID,
		Name:           c.Name,
		Industry:       c.Industry,
		IsReturning:    c.IsReturning,
		PriceSensitive: c.PriceSensitive,
		ContactName:    c.ContactName,
		ContactPhone:   c.ContactPhone,
		ContactEmail:   c.ContactEmail,
		Notes:          c.Notes,
		CreatedBy:      c.CreatedBy,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
}
```

- [ ] **Step 2: 写 service**

Create `apps/api/internal/customer/service.go`:
```go
package customer

import (
	"context"

	"github.com/google/uuid"
)

// Role codes used to decide owner scoping. Kept as constants rather than
// imported from internal/auth to avoid a package-level circular dependency
// (auth → customer handler later would re-enter here).
const (
	RoleSalesperson = "salesperson"
	RoleReviewer    = "reviewer"
	RoleAdmin       = "admin"
)

// Service orchestrates owner scoping on top of Repository.
type Service struct{ repo *Repository }

// NewService wires a Service.
func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// ownerScope returns a pointer to the caller UUID if the role is salesperson
// (they only see their own rows), else nil (reviewer/admin see all).
func ownerScope(callerID uuid.UUID, role string) *uuid.UUID {
	if role == RoleSalesperson {
		c := callerID
		return &c
	}
	return nil
}

// List returns a paginated list, scoped to the caller's role.
func (s *Service) List(ctx context.Context, callerID uuid.UUID, role string, q string, page, pageSize int) (ListResponse, error) {
	res, err := s.repo.List(ctx, ListFilter{
		Query:    q,
		OwnerID:  ownerScope(callerID, role),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return ListResponse{}, err
	}
	items := make([]CustomerDTO, len(res.Items))
	for i, r := range res.Items {
		items[i] = toDTO(r)
	}
	return ListResponse{Items: items, Page: res.Page, PageSize: res.PageSize, Total: res.Total}, nil
}

// Get fetches a single customer respecting the caller's owner scope.
func (s *Service) Get(ctx context.Context, callerID uuid.UUID, role string, id uuid.UUID) (*CustomerDTO, error) {
	row, err := s.repo.Get(ctx, id, ownerScope(callerID, role))
	if err != nil {
		return nil, err
	}
	d := toDTO(*row)
	return &d, nil
}

// Create inserts a new customer owned by the caller.
func (s *Service) Create(ctx context.Context, callerID uuid.UUID, req CreateRequest) (*CustomerDTO, error) {
	row := &Customer{
		Name:           req.Name,
		Industry:       req.Industry,
		IsReturning:    req.IsReturning,
		PriceSensitive: req.PriceSensitive,
		ContactName:    req.ContactName,
		ContactPhone:   req.ContactPhone,
		ContactEmail:   req.ContactEmail,
		Notes:          req.Notes,
		CreatedBy:      callerID,
	}
	if err := s.repo.Create(ctx, row); err != nil {
		return nil, err
	}
	d := toDTO(*row)
	return &d, nil
}

// Update applies a patch; owner scope applies.
func (s *Service) Update(ctx context.Context, callerID uuid.UUID, role string, id uuid.UUID, req UpdateRequest) (*CustomerDTO, error) {
	row, err := s.repo.Update(ctx, id, ownerScope(callerID, role), Patch{
		Name:           req.Name,
		Industry:       req.Industry,
		IsReturning:    req.IsReturning,
		PriceSensitive: req.PriceSensitive,
		ContactName:    req.ContactName,
		ContactPhone:   req.ContactPhone,
		ContactEmail:   req.ContactEmail,
		Notes:          req.Notes,
	})
	if err != nil {
		return nil, err
	}
	d := toDTO(*row)
	return &d, nil
}
```

- [ ] **Step 3: 写 handler**

Create `apps/api/internal/customer/handler.go`:
```go
package customer

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

// Handler exposes customer HTTP endpoints.
type Handler struct{ svc *Service }

// NewHandler wires a Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// List GET /customers[?q=&page=&page_size=]
func (h *Handler) List(c *gin.Context) {
	caller := auth.CurrentUser(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.svc.List(c.Request.Context(), caller.ID, caller.Role, c.Query("q"), page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Get GET /customers/:id
func (h *Handler) Get(c *gin.Context) {
	caller := auth.CurrentUser(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_ID", "message": "invalid uuid"})
		return
	}
	dto, err := h.svc.Get(c.Request.Context(), caller.ID, caller.Role, id)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND", "message": "customer not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto)
}

// Create POST /customers
func (h *Handler) Create(c *gin.Context) {
	caller := auth.CurrentUser(c)
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	dto, err := h.svc.Create(c.Request.Context(), caller.ID, req)
	if errors.Is(err, ErrDuplicateName) {
		c.JSON(http.StatusConflict, gin.H{"code": "ERR_DUPLICATE_NAME", "message": "a customer with this name already exists"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, dto)
}

// Patch PATCH /customers/:id
func (h *Handler) Patch(c *gin.Context) {
	caller := auth.CurrentUser(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_ID", "message": "invalid uuid"})
		return
	}
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	dto, err := h.svc.Update(c.Request.Context(), caller.ID, caller.Role, id, req)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND", "message": "customer not found"})
		return
	}
	if errors.Is(err, ErrDuplicateName) {
		c.JSON(http.StatusConflict, gin.H{"code": "ERR_DUPLICATE_NAME", "message": "name already exists"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto)
}
```

- [ ] **Step 4: 写 router**

Create `apps/api/internal/customer/router.go`:
```go
package customer

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts /customers on an authenticated group.
// Owner scoping is handled inside the service based on role.
func RegisterRoutes(authed *gin.RouterGroup, h *Handler) {
	g := authed.Group("/customers")
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.PATCH("/:id", h.Patch)
}
```

- [ ] **Step 5: 写 handler HTTP 集成测试**

Create `apps/api/internal/customer/handler_test.go`:
```go
package customer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	gpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
)

// setupHandler brings up pg + migrations, seeds a salesperson user, mounts
// customer routes with a fake auth middleware that injects the test user
// into the Gin context (bypassing JWT/CSRF for unit-level testing).
func setupHandler(t *testing.T, role string) (*gin.Engine, *gorm.DB, uuid.UUID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("custh"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	mig, err := migrator.New(api.Migrations, "migrations", dsn)
	require.NoError(t, err)
	require.NoError(t, mig.Up())
	require.NoError(t, mig.Close())

	db, err := gorm.Open(gpostgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	var roleID uuid.UUID
	require.NoError(t, db.Raw("SELECT id FROM roles WHERE code = ?", role).Scan(&roleID).Error)
	user := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		user, "Test Owner", "u-"+user.String()+"@test.local", "hash", roleID,
	).Error)

	repo := customer.NewRepository(db)
	svc := customer.NewService(repo)
	h := customer.NewHandler(svc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	// Fake auth middleware — real CSRF/JWT tested elsewhere.
	v1.Use(func(c *gin.Context) {
		c.Set("auth.currentUser", auth.CurrentUserSummary{ID: user, Role: role})
		c.Next()
	})
	customer.RegisterRoutes(v1, h)

	return router, db, user
}

func do(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCreate_Then_Get(t *testing.T) {
	r, _, _ := setupHandler(t, customer.RoleSalesperson)

	w := do(t, r, http.MethodPost, "/api/v1/customers", customer.CreateRequest{Name: "Acme"})
	require.Equal(t, http.StatusCreated, w.Code)
	var created customer.CustomerDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Equal(t, "Acme", created.Name)

	w = do(t, r, http.MethodGet, "/api/v1/customers/"+created.ID.String(), nil)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestCreate_DuplicateName_409(t *testing.T) {
	r, _, _ := setupHandler(t, customer.RoleSalesperson)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/api/v1/customers", customer.CreateRequest{Name: "Acme"}).Code)
	w := do(t, r, http.MethodPost, "/api/v1/customers", customer.CreateRequest{Name: "Acme"})
	require.Equal(t, http.StatusConflict, w.Code)
}

func TestPatch_OwnerGuard_ForbidsCrossOwner(t *testing.T) {
	// Owner 1 creates a row.
	r1, db, _ := setupHandler(t, customer.RoleSalesperson)
	w := do(t, r1, http.MethodPost, "/api/v1/customers", customer.CreateRequest{Name: "Alpha"})
	require.Equal(t, http.StatusCreated, w.Code)
	var created customer.CustomerDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	// Owner 2 on same DB tries to patch Owner 1's row → 404.
	// Build a second router/engine pointing at the same DB but with a
	// different user injected.
	var roleID uuid.UUID
	require.NoError(t, db.Raw("SELECT id FROM roles WHERE code = ?", customer.RoleSalesperson).Scan(&roleID).Error)
	otherUser := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		otherUser, "Other", "other-"+otherUser.String()+"@test.local", "h", roleID,
	).Error)
	repo := customer.NewRepository(db)
	h := customer.NewHandler(customer.NewService(repo))
	r2 := gin.New()
	v1 := r2.Group("/api/v1")
	v1.Use(func(c *gin.Context) {
		c.Set("auth.currentUser", auth.CurrentUserSummary{ID: otherUser, Role: customer.RoleSalesperson})
		c.Next()
	})
	customer.RegisterRoutes(v1, h)

	newName := "Beta"
	w = do(t, r2, http.MethodPatch, "/api/v1/customers/"+created.ID.String(), customer.UpdateRequest{Name: &newName})
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestList_ReviewerSeesAll(t *testing.T) {
	rSales, db, _ := setupHandler(t, customer.RoleSalesperson)
	// Salesperson creates a customer.
	require.Equal(t, http.StatusCreated,
		do(t, rSales, http.MethodPost, "/api/v1/customers", customer.CreateRequest{Name: "X"}).Code)

	// Reviewer router bound to the same DB.
	var roleID uuid.UUID
	require.NoError(t, db.Raw("SELECT id FROM roles WHERE code = ?", customer.RoleReviewer).Scan(&roleID).Error)
	rev := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		rev, "Rev", "rev-"+rev.String()+"@test.local", "h", roleID,
	).Error)
	repo := customer.NewRepository(db)
	h := customer.NewHandler(customer.NewService(repo))
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	v1.Use(func(c *gin.Context) {
		c.Set("auth.currentUser", auth.CurrentUserSummary{ID: rev, Role: customer.RoleReviewer})
		c.Next()
	})
	customer.RegisterRoutes(v1, h)

	w := do(t, engine, http.MethodGet, "/api/v1/customers", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var resp customer.ListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.EqualValues(t, 1, resp.Total)
}

func TestCreate_ValidatesName(t *testing.T) {
	r, _, _ := setupHandler(t, customer.RoleSalesperson)
	w := do(t, r, http.MethodPost, "/api/v1/customers", customer.CreateRequest{Name: ""})
	require.Equal(t, http.StatusBadRequest, w.Code)
}
```

注意 `c.Set("auth.currentUser", ...)` 中的 key `"auth.currentUser"` 必须和 `internal/auth/middleware.go` 中的 `currentUserKey` 常量一致——目前常量是 unexported 的字符串 `"auth.currentUser"`（见代码）。如果源包里的 key 改了，上面 test 也要同步改。

- [ ] **Step 6: 跑测试**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go test ./internal/customer/...
```
Expected: 6 repository + 5 handler = 11 PASS.

- [ ] **Step 7: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/internal/customer/dto.go apps/api/internal/customer/service.go apps/api/internal/customer/handler.go apps/api/internal/customer/handler_test.go apps/api/internal/customer/router.go
git commit -m "$(cat <<'EOF'
feat(api): customer HTTP handlers + service with role-driven owner scoping

Service injects an owner filter when the caller role is salesperson and
nil otherwise, so reviewers/admins see everything. Handler maps repo
errors to the agreed HTTP codes: 404 not-found, 409 duplicate name,
400 invalid body/uuid. Five-case handler suite exercises create→get,
duplicate→409, cross-owner PATCH rejected, reviewer sees all, and name
validation.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: main.go 装配 catalog + customer 路由 + 扩展 audit

**Files:**
- Modify: `apps/api/cmd/server/main.go`

- [ ] **Step 1: 添加 import**

Open `apps/api/cmd/server/main.go` and add to the import block:
```go
	"github.com/pigletfly/trademark-admin/apps/api/internal/catalog"
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
```

- [ ] **Step 2: 把 audit middleware 扩到 authed 组，并装配 catalog + customer**

Find the block where `authed` and `adminGroup` are set up. Currently:
```go
	v1 := router.Group("/api/v1")
	public := v1.Group("")
	authed := v1.Group("")
	authed.Use(auth.RequireAuth([]byte(cfg.JWTAccessSecret)), auth.CSRF())

	auth.RegisterRoutes(public, authed, authHandler)

	// Audit plumbing
	auditRepo := audit.NewRepository(db)
	auditMW := audit.Middleware(auditRepo, func(c *gin.Context) (uuid.UUID, bool) {
		u := auth.CurrentUser(c)
		if u.ID == uuid.Nil {
			return uuid.Nil, false
		}
		return u.ID, true
	}, log)

	// Admin routes require auth + role=admin + CSRF + audit middleware
	adminGroup := v1.Group("")
	adminGroup.Use(auth.RequireAuth([]byte(cfg.JWTAccessSecret)),
		auth.RequireRole("admin"),
		auth.CSRF(),
		auditMW,
	)
	adminUserHandler := auth.NewAdminHandler(auth.NewAdminService(authRepo))
	auth.RegisterAdminRoutes(adminGroup, adminUserHandler)
	audit.RegisterAdminRoutes(adminGroup, audit.NewAdminHandler(auditRepo))
```

Rewrite it so that audit attaches to `authed` too (all authenticated mutations get audited), and wire catalog + customer:
```go
	v1 := router.Group("/api/v1")
	public := v1.Group("")

	// Build audit middleware first so both authed and adminGroup can chain it.
	auditRepo := audit.NewRepository(db)
	auditMW := audit.Middleware(auditRepo, func(c *gin.Context) (uuid.UUID, bool) {
		u := auth.CurrentUser(c)
		if u.ID == uuid.Nil {
			return uuid.Nil, false
		}
		return u.ID, true
	}, log)

	// Authenticated routes for any logged-in user.
	authed := v1.Group("")
	authed.Use(auth.RequireAuth([]byte(cfg.JWTAccessSecret)), auth.CSRF(), auditMW)
	auth.RegisterRoutes(public, authed, authHandler)

	// Catalog: read endpoints on authed; write endpoints on adminGroup below.
	catalogRepo := catalog.NewRepository(db)
	catalogSvc := catalog.NewService(catalogRepo)
	catalogHandler := catalog.NewHandler(catalogSvc)
	catalog.RegisterReadRoutes(authed, catalogHandler)

	// Customers — owner scoping is handled inside the service.
	custRepo := customer.NewRepository(db)
	custSvc := customer.NewService(custRepo)
	custHandler := customer.NewHandler(custSvc)
	customer.RegisterRoutes(authed, custHandler)

	// Admin-only routes: auth + role=admin + CSRF + audit.
	adminGroup := v1.Group("")
	adminGroup.Use(auth.RequireAuth([]byte(cfg.JWTAccessSecret)),
		auth.RequireRole("admin"),
		auth.CSRF(),
		auditMW,
	)
	adminUserHandler := auth.NewAdminHandler(auth.NewAdminService(authRepo))
	auth.RegisterAdminRoutes(adminGroup, adminUserHandler)
	audit.RegisterAdminRoutes(adminGroup, audit.NewAdminHandler(auditRepo))
	catalog.RegisterAdminRoutes(adminGroup, catalogHandler)
```

注意：`auth.RegisterRoutes(public, authed, ...)` 的签名不变，但我们把它挪到了 `authed` 的 `auditMW` 之后。这样 `/auth/login`、`/auth/refresh`、`/auth/logout` 中的 logout 会走 audit（POST `/auth/logout`）——接受，作为记录有用。`/auth/login` 和 `/auth/refresh` 仍在 `public` 组，不会被 audit（刚好符合 spec 的 "除 /auth/refresh"）；唯一的新增是 `/auth/logout` 变得会被 audit，这是合理的。

如果我们不想把 login 也审计，再检查下 `auth.RegisterRoutes` 内部：login/refresh 放 public，logout/me 放 authed。详见 `internal/auth/router.go`——如果 logout/me 也在 public，就无影响；如果在 authed，logout 会记录——这符合预期。

- [ ] **Step 3: 编译 + 跑所有后端测试**

```bash
cd /Users/adam/workspace/github/trademark-admin/apps/api
go build ./...
go test ./...
```
Expected: 全部 PASS（含 Plan 2 的 auth 测试 + Plan 4 的 catalog / customer / seeder 测试）。

- [ ] **Step 4: 端到端人工验证（可选但建议）**

```bash
cd /Users/adam/workspace/github/trademark-admin
docker compose up -d postgres
cd apps/api
go build -o /tmp/tm-api ./cmd/server
DATABASE_URL=postgres://trademark:trademark@localhost:5432/trademark?sslmode=disable \
JWT_ACCESS_SECRET=dev-access \
JWT_REFRESH_SECRET=dev-refresh \
BOOTSTRAP_ADMIN_EMAIL=admin@example.com \
BOOTSTRAP_ADMIN_PASSWORD=change-me-on-first-login \
APP_ENV=development \
  /tmp/tm-api &
API_PID=$!
sleep 2

# 登录拿 cookies
curl -c /tmp/tm-cookies -b /tmp/tm-cookies \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"change-me-on-first-login"}' \
  http://localhost:8080/api/v1/auth/login

# 读字典（应返回 60+ 条）
curl -b /tmp/tm-cookies http://localhost:8080/api/v1/catalog/countries | jq '.items | length'
# Expected: >=60

# 读尼斯分类（应是 45）
curl -b /tmp/tm-cookies http://localhost:8080/api/v1/catalog/nice-categories | jq '.items | length'
# Expected: 45

# 建客户 —— 需要带 CSRF
CSRF=$(awk '/tm_csrf_token/ {print $NF}' /tmp/tm-cookies)
curl -b /tmp/tm-cookies \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF" \
  -d '{"name":"Acme Corp","industry":"Software"}' \
  http://localhost:8080/api/v1/customers | jq .

# 列客户
curl -b /tmp/tm-cookies http://localhost:8080/api/v1/customers | jq '.items | length'

kill $API_PID
rm -f /tmp/tm-api /tmp/tm-cookies
cd ../..
docker compose down -v
```

Expected: 登录 200 Set-Cookie; 字典列表正常；客户创建返回 201 + 列表含 1 条。

- [ ] **Step 5: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/api/cmd/server/main.go
git commit -m "$(cat <<'EOF'
feat(api): wire catalog + customer routes and extend audit to all authed writes

The audit middleware now attaches to the authed group so any non-GET
request by any authenticated user gets logged. /auth/login and
/auth/refresh remain in the unauthenticated public group and are not
audited, matching spec ("all non-GET except /auth/refresh"). Catalog
read routes are mounted on authed; catalog write routes (PATCH country)
and admin-only routes remain on adminGroup. Customers CRUD runs on
authed with role-driven owner scoping handled inside the service.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Plan 4 Definition of Done

1. ✅ `apps/api/migrations/000002_catalogs_and_customers.{up,down}.sql` 合法可 up / down / up 循环
2. ✅ `apps/api/seed/{countries,nice_categories}.json` 至少 60 + 45 条，内容符合结构
3. ✅ `seeder.Run` 幂等；首次 insert，二次 update，覆盖 testcontainers 集成测试
4. ✅ 服务器启动自动跑 seed；`cmd/seed` CLI 可独立重新 seed
5. ✅ `GET /api/v1/catalog/countries` 返回已 seed 的国家，按 `sort_order` 升序
6. ✅ `GET /api/v1/catalog/nice-categories` 返回恰好 45 条
7. ✅ `PATCH /api/v1/catalog/countries/:id` 仅 admin 可调，且只更新 patch 中出现的字段；非法 uuid → 400，不存在 → 404
8. ✅ `GET /api/v1/customers` 按角色过滤：salesperson 只看自建；reviewer/admin 看全量
9. ✅ `GET /api/v1/customers?q=` 支持姓名+行业 ILIKE 搜索，`%`/`_` 被正确转义
10. ✅ `POST /api/v1/customers` 写入 `created_by = callerID`；同名返回 409
11. ✅ `PATCH /api/v1/customers/:id` salesperson 跨 owner 改动返回 404；重名返回 409
12. ✅ 所有非 GET 请求（除 `/auth/login` `/auth/refresh`）在 audit_logs 表有记录
13. ✅ `go build ./...` 成功；`go test ./...` 全绿
14. ✅ 人工 curl 脚本（Task 9 Step 4）的四个请求都成功

## 下一步

Plan 4 完成后进入 **Plan 5（前端字典 + 客户档案视图）**：
- `/_authenticated/catalog/countries.tsx` (admin-only edit drawer) + `/_authenticated/catalog/nice-categories.tsx` (read-only)
- `/_authenticated/customers/index.tsx` (TanStack Table + 搜索 + 分页) + `/_authenticated/customers/$id.tsx` (详情 + 编辑)
- sidebar 导航项（字典 admin-only）
- 基于 MSW 的客户创建 → 列表的集成测试
