# HPAHub 网站模板

基于 Svelte 和 PocketBase 的网站模板，提供管理员备份管理界面。

## 部署

安装 Docker 和 Docker Compose，然后运行：

```sh
docker compose up -d --build
```

访问 `http://localhost:8090`。设置 `PORT` 环境变量可更改宿主机端口。PocketBase 数据保存在 Docker 卷 `pb_data` 中，更新容器不会清除数据。

创建 PocketBase 超级管理员（将邮箱和密码替换为自己的值）：

```sh
docker compose exec app ./pocketbase superuser upsert admin@example.com '替换为强密码'
```

使用该账号登录网站管理界面，设置备份间隔和保留份数，也可手动创建和下载备份。自动备份默认关闭；保留份数仅限制自动备份，不删除手动备份。

## 备份接口

所有接口均需 PocketBase 超级管理员身份：

- `GET /api/backup-policy`：读取策略。
- `PUT /api/backup-policy`：设置 `{"intervalHours": 24, "retainCount": 5}`。间隔为 0 时关闭自动备份；范围为 0–8760 小时，保留份数为 1–100。
- `GET /api/backups`、`POST /api/backups`：列出和创建备份。
- `GET /api/backups/{name}?token={fileToken}`：下载备份；通过 `POST /api/files/token` 获取短期文件令牌。

自动备份保存在 PocketBase 备份存储中。策略更改后，从保存时起计算下次备份时间。

## 数据安全

备份包含敏感数据，请妥善保管并定期下载到独立存储。生产环境应在可信反向代理后启用 HTTPS。删除 Docker 卷会永久删除数据库及保存在卷中的备份。
