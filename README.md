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

## 旧账号导入

旧账号只能从一致的 SQLite 快照离线导入。停止应用服务后，将快照只读挂载到一次性容器：

```sh
docker compose run --rm --no-deps -T -v "$SNAPSHOT/users.db:/tmp/legacy-users.db:ro" app ./pocketbase hpa-import-accounts --source /tmp/legacy-users.db
```

导入保留受支持的旧密码和管理员身份。出现不支持的密码哈希或账号冲突时，整个导入回滚；保留快照并使用邀请码注册。

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
