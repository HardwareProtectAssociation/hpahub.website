# HPA Hub 网站

基于 Svelte 和 PocketBase 的登录制网站。

网站支持简体中文、繁體中文、日本語和 English。使用导航栏语言选择器切换，选择保存在当前浏览器；首次访问根据浏览器语言选择，其他语言默认简体中文。用户提交的内容不会自动翻译。

## 部署

安装 Docker 和 Docker Compose，然后运行：

```sh
docker compose up -d --build --remove-orphans
```

构建时可通过 Docker 构建参数 `GOPROXY` 和 `ALPINE_MIRROR` 设置依赖镜像源。

访问 `http://localhost:18763`。设置 `PORT` 环境变量可更改宿主机端口。PocketBase 数据保存在 Docker 卷 `pb_data` 中，更新容器不会清除数据。

显卡数据、仓库项目与文件由 PocketBase 管理，保存在 `pb_data`；默认本地文件存储会随 PocketBase 备份一起保存。视频缩略图和预览依赖容器内的 FFmpeg、ffprobe。

创建 PocketBase 超级管理员（将邮箱和密码替换为自己的值）：

```sh
docker compose exec app ./pocketbase superuser upsert admin@example.com '替换为强密码'
```

PocketBase 超级管理员还可在控制台设置备份间隔和保留份数、手动创建及下载备份。自动备份默认关闭；保留份数仅限制自动备份，不删除手动备份。

## 本地测试

需要 Docker Compose 2.24.4 或更新版本。以下命令使用独立的 `hpahub-local` 项目和数据卷，不影响普通部署；无需宿主机 Python 或 Go：

```sh
sh scripts/local-up.sh                      # 构建、启动并打开页面
sh scripts/local-reset.sh                   # 确认后删除本地测试数据库及备份
sh scripts/local-restore.sh /path/backup.zip # 确认后从 PocketBase ZIP 恢复
```

测试地址默认为 `http://localhost:18764`，可通过 `PORT` 覆盖。首次创建数据卷时，测试管理员账号为 `admin`，密码为 `admin123`。已有或恢复的数据不会重置管理员。

Linux 开发和测试仅使用 Docker。Windows 优先使用 WSL 2 和 Docker Desktop，并在 WSL 中运行上述命令。

### Windows PowerShell 开发服务器

仅在无法使用 WSL 和 Docker 时使用。安装 Go 1.24 或更新版本和 Node.js 22，并在 PowerShell 中安装前端依赖：

```powershell
cd frontend
npm ci
cd ..
```

如果 PowerShell 提示「禁止运行脚本」，先设置执行策略以允许直接运行 `.ps1`（`Bypass` 会无提示执行所有脚本，仅建议开发机使用）：

```powershell
Set-ExecutionPolicy -Scope CurrentUser -Force Bypass
# 对所有用户生效时，在管理员 PowerShell 中运行：
Set-ExecutionPolicy -Scope LocalMachine -Force Bypass
```

注意 `CurrentUser` 优先级高于 `LocalMachine`，已有 `CurrentUser` 设置的账户需一并更改。

在仓库根目录的 PowerShell 中运行：

```powershell
.\dev.ps1
```

开发服务器就绪后会自动在浏览器打开前端页面 `http://127.0.0.1:5173`，前端将 `/api` 请求代理到 `127.0.0.1:8090`。两个服务均只监听本机；按 Ctrl+C 或关闭该窗口即可停止两个服务及其子进程；任一服务意外退出时，脚本也会停止另一个服务。后端数据保存在独立的 `backend/pb_data`，不与 Docker 数据卷共享，也不会自动创建测试管理员。需要管理员时，在第三个 PowerShell 窗口运行：

```powershell
cd backend
go run . superuser create admin@hpahub.local '替换为测试密码' --dir (Join-Path (Get-Location) 'pb_data')
```

如果 8090 或 5173 端口被占用，先检查占用进程；如果是上一次未正确关闭的本项目开发服务器，停止该服务器及其子进程后重启。不要终止其他项目的进程。视频预览还需要在 Windows 上安装 FFmpeg 并加入 PATH。

其他检查应在隔离容器中执行：

```sh
# 在相应容器内执行：
go test ./...
npm ci && npm run build
# Go 测试覆盖备份验证与仓库文件安全。
```

恢复会停止测试服务，并将原数据保留在测试卷的 `backups/local-restore-*` 目录。恢复后运行启动命令。本地使用 Caddy 反向代理，仅监听本机；应用运行在隔离网络中。生产备份仍包含敏感数据和生产账户，请妥善保管。

## 备份接口

所有接口均需 PocketBase 超级管理员身份：

- `GET /api/backup-policy`：读取策略。
- `PUT /api/backup-policy`：设置 `{"intervalHours": 24, "retainCount": 5}`。间隔为 0 时关闭自动备份；范围为 0–8760 小时，保留份数为 1–100。
- `GET /api/backups`、`POST /api/backups`：列出和创建备份。
- `GET /api/backups/{name}?token={fileToken}`：下载备份；通过 `POST /api/files/token` 获取短期文件令牌。

自动备份保存在 PocketBase 备份存储中。策略更改后，从保存时起计算下次备份时间。

## 数据安全

备份包含敏感数据，请妥善保管并定期下载到独立存储。生产环境应在可信反向代理后启用 HTTPS。默认的 Docker 卷 `pb_data` 被删除时，卷内数据库、文件和备份也会被删除。建议在部署前将 `/app/pb_data` 绑定挂载到受保护的宿主机外部目录，并定期将备份复制到独立存储；删除 Docker 卷不会删除该目录，但删除目录或磁盘故障仍会造成数据丢失。已有部署改用绑定挂载前，应停写、备份并验证恢复，再完整复制现有数据并核验后切换，避免启动空数据库。

例如，在部署目录创建 `compose.override.yaml`（将路径替换为宿主机上已准备好且有适当权限的绝对路径）：

```yaml
services:
  app:
    volumes: !override
      - /srv/hpahub/pb_data:/app/pb_data
```

此配置需要 Docker Compose 2.24.4 或更新版本。外部对象存储中的文件需要单独备份和验证。

旧账号清理迁移会删除旧账号映射对应的用户资料及其旧密码哈希；升级前须备份并验证恢复。PocketBase 超级管理员不会因此删除。恢复旧备份会带回旧账号数据，恢复后需重新执行清理迁移。
