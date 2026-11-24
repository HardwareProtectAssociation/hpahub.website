# HPAHub 网站模板

基于 Svelte 和 PocketBase 的登录制网站模板。

网站支持简体中文、繁體中文、日本語和 English。使用导航栏语言选择器切换，选择保存在当前浏览器；首次访问根据浏览器语言选择，其他语言默认简体中文。用户提交的内容不会自动翻译。

## 部署

安装 Docker 和 Docker Compose，然后运行：

```sh
docker compose up -d --build --remove-orphans
```

构建时可通过 Docker 构建参数 `GOPROXY` 和 `ALPINE_MIRROR` 设置依赖镜像源。

访问 `http://localhost:18763`。设置 `PORT` 环境变量可更改宿主机端口。PocketBase 数据保存在 Docker 卷 `pb_data` 中，更新容器不会清除数据。

显卡数据、仓库项目与文件由 PocketBase 管理，保存在 `pb_data`；默认本地文件存储会随 PocketBase 备份一起保存。视频缩略图和预览依赖容器内的 FFmpeg、ffprobe。运行时 Compose 只挂载 `pb_data`，不挂载历史卷，也不执行启动兼容检查。

## 从旧版本迁移

先停止所有旧服务和写入，并备份和验证 `pb_data`、`gpu_data`、`warehouse_data` 三个卷。确认实际 Compose 项目卷名及新应用镜像；以下卷名和 `IMAGE` 均须替换为实际值。仅在已授权的隔离环境操作，不触碰生产数据。导入期间不能运行其他应用实例。

```sh
docker compose down --remove-orphans
docker compose build app
# 仅当旧显卡卷包含 gpu.db 时运行：
docker run --rm --network none --mount type=volume,src=实际项目_pb_data,dst=/app/pb_data --mount type=volume,src=实际项目_gpu_data,dst=/source,readonly --entrypoint /app/pocketbase IMAGE hpa-import-gpu --source /source/gpu.db
# 仅当旧仓库卷包含数据时运行：
docker run --rm --network none --mount type=volume,src=实际项目_pb_data,dst=/app/pb_data --mount type=volume,src=实际项目_warehouse_data,dst=/source,readonly --entrypoint /app/pocketbase IMAGE hpa-import-warehouse --source /source
docker compose up -d --build --remove-orphans
```

导入只执行一次，不保留长期兼容实现。保留旧卷，不要使用 `down -v`；失败时停止操作并保留备份和源卷。切换后检查显卡、项目、文件、预览和访问权限，并创建、验证新的 PocketBase 备份。新服务产生写入后，直接回退旧卷会丢失这些写入；恢复前先停写并确认回滚方案。若使用 PocketBase 外部对象存储，须单独备份和验证对象；不能假定 PocketBase ZIP 包含远程文件。

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

分别在两个 PowerShell 窗口中运行：

```powershell
.\scripts\dev-backend.ps1
.\scripts\dev-frontend.ps1
```

访问 `http://127.0.0.1:5173`，前端将 `/api` 请求代理到 `127.0.0.1:8090`。两个服务均只监听本机；按 Ctrl+C 停止各自的服务。后端数据保存在独立的 `backend/pb_data`，不与 Docker 数据卷共享，也不会自动创建测试管理员。需要管理员时，在第三个 PowerShell 窗口运行：

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
# Go 测试覆盖备份验证、数据导入与仓库文件安全。
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

备份包含敏感数据，请妥善保管并定期下载到独立存储。生产环境应在可信反向代理后启用 HTTPS。删除 Docker 卷会永久删除数据库及保存在卷中的备份。
