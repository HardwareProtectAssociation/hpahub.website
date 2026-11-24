# HPAHub 网站模板

基于 Svelte 和 PocketBase 的登录制网站模板。

## 部署

安装 Docker 和 Docker Compose，然后运行：

```sh
docker compose up -d --build
```

访问 `http://localhost:18763`。设置 `PORT` 环境变量可更改宿主机端口。PocketBase 数据保存在 Docker 卷 `pb_data` 中，更新容器不会清除数据。

显卡库和仓库分别保存在 `gpu_data`、`warehouse_data` 卷中。PocketBase 备份仅包含 `pb_data`，不能恢复这两个独立数据卷；应分别备份显卡数据库和仓库文件。

创建 PocketBase 超级管理员（将邮箱和密码替换为自己的值）：

```sh
docker compose exec app ./pocketbase superuser upsert admin@example.com '替换为强密码'
```

管理员与普通用户使用同一登录入口，登录后进入内容首页。超级管理员和站点管理员可在管理控制台生成邀请码并查看消费记录；邀请码 24 小时内有效，注册成功后失效。邀请注册账号默认为成员。

为防止绕过邀请码注册，版本化迁移关闭 `users` 集合的原生创建、更新、删除和 OAuth2 登录。登录及注册分别有每分钟全站 60 次和 30 次的请求上限。

PocketBase 超级管理员还可在控制台设置备份间隔和保留份数、手动创建及下载备份。自动备份默认关闭；保留份数仅限制自动备份，不删除手动备份。

## 公开内容

成员可在设置中编辑个人资料和卡片，并自行控制资料是否公开。公开项目由站点管理员或超级管理员管理；仓库仅限内部成员访问。关闭公开后，网站不再提供对应资料和图片，但无法撤回访客已下载的副本。

## 旧账号导入

旧账号只能从一致的 SQLite 快照离线导入。停止应用服务后，将快照只读挂载到一次性容器：

```sh
docker compose run --rm --no-deps -T -v "$SNAPSHOT/users.db:/tmp/legacy-users.db:ro" app ./pocketbase hpa-import-accounts --source /tmp/legacy-users.db
```

导入保留受支持的旧密码和管理员身份。出现不支持的密码哈希或账号冲突时，整个导入回滚；保留快照并使用邀请码注册。

## 旧站迁移与切换

本节是操作计划，不代表验证或部署已经完成。计划批准只授权开发，不授权生产操作。只有在人工确认 PR 测试充分、PR 已合并，并另行授权部署后，才能操作生产。不要自行合并 PR 或开启自动合并。

### 隔离验证

先用合成数据验证，禁止将真实快照传入未获授权的开发环境。准备两个显卡条目，包含游戏记录、自定义字段和私有 `related_media`；仓库准备两个项目中的同名文件、含空格和中文的文件、隐藏元数据、配置及支持 Range 的媒体。账号快照使用合成密码哈希。

```sh
python3 scripts/hpa-legacy-snapshot.py \
  --gpu-db "$FIXTURE/gpu.db" \
  --warehouse-root "$FIXTURE/warehouse" \
  --auth-db "$FIXTURE/users.db" \
  --output "$SNAPSHOT"
```

以下步骤统一使用隔离项目和配置，不能改用现有 `hpahub-local` 数据卷：

```sh
export PORT=18765
docker compose -p hpahub-migration-check -f compose.yaml -f scripts/compose.local.yaml build
docker compose -p hpahub-migration-check -f compose.yaml -f scripts/compose.local.yaml create
```

先确认项目使用新建且为空的 `gpu_data`、`warehouse_data` 和 `pb_data` 卷。若已有项目卷含数据，立即停止，不覆盖、不清理。停止目标服务后，将合成快照装入已创建的容器并离线导入账号：

```sh
docker compose -p hpahub-migration-check -f compose.yaml -f scripts/compose.local.yaml stop
docker compose -p hpahub-migration-check -f compose.yaml -f scripts/compose.local.yaml cp "$SNAPSHOT/gpu.db" gpu:/app/data/gpu.db
docker compose -p hpahub-migration-check -f compose.yaml -f scripts/compose.local.yaml cp "$SNAPSHOT/warehouse/." warehouse:/app/data/
docker compose -p hpahub-migration-check -f compose.yaml -f scripts/compose.local.yaml run --rm --no-deps -T -v "$SNAPSHOT/users.db:/tmp/legacy-users.db:ro" app ./pocketbase hpa-import-accounts --source /tmp/legacy-users.db
docker compose -p hpahub-migration-check -f compose.yaml -f scripts/compose.local.yaml up -d
docker compose -p hpahub-migration-check -f compose.yaml -f scripts/compose.local.yaml exec -T app ./pocketbase superuser create admin@hpahub.local admin123
```

核对卷内容与 `manifest.json`，并执行 `cd backend && go test ./...`、`python3 scripts/test-local-restore.py`。检查公开显卡筛选和对比、私有媒体不泄露、成员仓库上传及同名文件隔离、Range 下载、媒体认证和附件类型。验证成员资料与卡片的公开及撤销、项目草稿和发布权限、登录退出，以及桌面和窄屏的直链、刷新和键盘操作。检查以下两种配置的 sidecar 均不发布端口，生产应用端口仅绑定本机：

```sh
docker compose config
docker compose -p hpahub-local -f compose.yaml -f scripts/compose.local.yaml config
```

在 PR 中记录实际验证结果和限制，包括私密资产尚未完成公开审核、旧仓库源盘点为空；切换前重新盘点，不以旧代码目录大小代替数据盘点。

### 生产快照与导入

仅在获授权的维护窗口操作。使用已合并的 `main`，并先备份 Caddy 配置、旧服务及源数据，保留旧站文件。生产项目使用独立的 Compose 项目 `hpahub` 和新卷；检查项目卷是否已存在且含数据，若含数据则停止，不覆盖、不清理。应用监听 `127.0.0.1:18763`，不要直接将旧在线目录挂载为可写卷。

最终快照前停止 `gpudb.service`、`warehouse.service` 和 `auth-gateway.service`，保留 SSH 只读导出能力。在旧机上指定一个不存在的受保护目标目录：

```sh
python3 scripts/hpa-legacy-snapshot.py \
  --gpu-db /opt/gpu-db/data/gpu.db \
  --warehouse-root /data \
  --auth-db /opt/auth-gateway/users.db \
  --output "$SNAPSHOT"
```

不要使用其他位置的 `.bak`。快照包含一致的 `gpu.db`、`users.db`、仓库文件和 `manifest.json`；`projects` 空目录也必须保留。依据 manifest 核对源和快照的各 GPU 表行数、账号总数、仓库文件相对路径、长度与 SHA-256；也要核对隐藏元数据、预览、原文件和配置。缺少配置时确认 manifest 标记并使用旧服务默认值。源与目标不符时不得切流；没有进一步批准时恢复旧服务。

数据所有者须在公开前审核显卡备注和自定义字段中的私密内容。发现私密内容时不得上线，直到所有者处理。账号哈希不受支持时保留旧账号库备份，本次不导入任何旧账号，由管理员发新邀请码；不得猜密码或设置通用临时密码。

确认新卷后，在同一项目和配置前缀下构建、创建并停止目标服务，再导入快照：

```sh
export HOST_BIND=127.0.0.1 PORT=18763
docker compose -p hpahub -f compose.yaml build
docker compose -p hpahub -f compose.yaml create
docker compose -p hpahub -f compose.yaml stop
docker compose -p hpahub -f compose.yaml cp "$SNAPSHOT/gpu.db" gpu:/app/data/gpu.db
docker compose -p hpahub -f compose.yaml cp "$SNAPSHOT/warehouse/." warehouse:/app/data/
docker compose -p hpahub -f compose.yaml run --rm --no-deps -T -v "$SNAPSHOT/users.db:/tmp/legacy-users.db:ro" app ./pocketbase hpa-import-accounts --source /tmp/legacy-users.db
docker compose -p hpahub -f compose.yaml up -d
```

导入命令执行数据库迁移；账号快照只读挂载，不进入镜像或应用卷。安装首位生产超级管理员时使用上面的 PocketBase CLI 和强密码，不使用本地测试账号。PocketBase 备份只覆盖 `pb_data`，不能恢复 `gpu_data` 或 `warehouse_data`；分别备份这两个卷并保护账号快照和备份。

### Caddy 切流与回退

仅修改现有 `hardwareprotection.top, hpa.volwave.cc` 站点配置：默认请求反代新站，保留 `/webhook*`、`/generate_204`、旧粘贴板 `/dashboard`、`/api/pastes*` 及其旧认证路由。不修改其他 vhost。切换后仅为旧 portal 重启旧认证服务；旧账号簿不与新站共享。`:3443` 和 `:8443` 仅将页面 GET/HEAD 分别重定向到 `/gpu` 和 `/warehouse`，拒绝其他方法，不再开放旧写入口；保持显卡库和仓库旧服务停止。

修改后先执行 `docker exec caddy-docker-caddy-1 caddy validate --config /etc/caddy/Caddyfile`。上线健康检查仅检查服务状态、公开首页、GPU 列表和未登录仓库访问被拒绝，不创建测试数据。先仅放行新站 GET/HEAD，暂停邀请注册和登录；检查完成后再开放新写入。

放行新写入前失败时，恢复旧 Caddy 配置和三个旧服务。放行写入后，先封写、备份新卷并人工核对增量；不得直接切回旧快照，否则会丢失新写入。保留源数据及备份，除非另获清理授权。

## 本地测试

需要 Docker Compose 2.24.4 或更新版本、Python 3。以下命令使用独立的 `hpahub-local` 项目和数据卷，不影响普通部署：

```sh
./scripts/local-up.sh                      # 构建、启动并打开页面
./scripts/local-reset.sh                   # 确认后删除本地测试数据库及备份
./scripts/local-restore.sh /path/backup.zip # 确认后从 PocketBase ZIP 恢复
```

测试地址默认为 `http://localhost:18764`，可通过 `PORT` 覆盖。首次创建数据卷时，测试管理员账号为 `admin`，密码为 `admin123`。已有或恢复的数据不会重置管理员。

恢复会停止测试服务，并将原数据保留在测试卷的 `backups/local-restore-*` 目录。恢复后运行启动命令。本地使用 Caddy 反向代理，仅监听本机；应用运行在隔离网络中。生产备份仍包含敏感数据和生产账户，请妥善保管。

## 备份接口

所有接口均需 PocketBase 超级管理员身份：

- `GET /api/backup-policy`：读取策略。
- `PUT /api/backup-policy`：设置 `{"intervalHours": 24, "retainCount": 5}`。间隔为 0 时关闭自动备份；范围为 0–8760 小时，保留份数为 1–100。
- `GET /api/backups`、`POST /api/backups`：列出和创建备份。
- `GET /api/backups/{name}?token={fileToken}`：下载备份；通过 `POST /api/files/token` 获取短期文件令牌。

自动备份保存在 PocketBase 备份存储中。策略更改后，从保存时起计算下次备份时间。

## 数据安全

备份包含敏感数据，请妥善保管并定期下载到独立存储。生产环境应在可信反向代理后启用 HTTPS。删除 Docker 卷会永久删除数据库及保存在卷中的备份。
