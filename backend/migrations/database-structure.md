# 当前数据库结构与优化

数据库使用 PocketBase / SQLite。此文记录当前代码的数据归属和迁移 `1760000054`—`1760000056` 的优化，不能作为生产环境已部署版本的证明。生产升级前应核对实际版本及备份中的迁移记录。

## 数据归属

| 领域 | 主记录 | 关联记录 | 保存方式 |
| --- | --- | --- | --- |
| 账号与个人资料 | `users` | `user_identities`、`user_skills`、`user_titles`、`user_collaboration_preferences` | 身份、技能和头衔通过关系关联；获授头衔与佩戴头衔分开保存 |
| 个人卡片 | `member_cards` | `users` | 卡片归属用户；布局 JSON 保存位置、大小、旋转、字体和背景 |
| 项目信息 | `projects` | `project_versions`、`project_stages` | 当前信息用于查询；版本快照用于追溯修改者和修改内容 |
| 项目参与和权限 | `project_members` | `project_join_requests`、`project_permission_grants`、`project_member_departure_requests` | 成员关系、申请、权限、退出/请离流程分开保存 |
| 重复项目 | `projects` | `project_completion_requests` | 项目可持续进行；每次通过的完成申请分别保存用户、次数和审核者 |
| 硬件报告 | `hardware` | `hw_types`、`hw_fields`、`hw_tables` | 分类定义可填写参数；报告的动态参数和测试行保存在 `data` JSON 中 |
| 硬件文章 | `hardware_articles`、`hardware_reviews` | `hardware`、`hardware_review_likes` | 专栏文章和成员评测分别关联产品；图片与正文保存关联 |
| 互动 | `project_follows`、`user_skill_likes`、`member_relations` | 用户、项目、技能 | 每次互动有具体用户关系记录；人数从关系记录统计 |
| 消息与成长 | `messages`、`career_xp_ledger` | 项目、用户、职业、任务 | 消息与经验账本保留独立的事件记录 |

`projects.sponsor` 是展示用的赞助者姓名/组织文本；实际赞助承诺、履约和奖励数据仍属于 `project_sponsorships` 等关系表。两者用途不同，不用名称文字替代用户关系。

`warehouse_*`、`showcase_projects` 等已有集合的保留和下线应按具体功能单独处理；本次优化不删除这些集合或其中的记录。

## 字段与约束

- `projects.difficulty`、`owner_proposed_difficulty` 沿用现有字段名，表示项目级别及发起人自评级别，取值为 D—S。
- `projects.danger_level` 表示危险性，取值为空、`low`、`medium`、`high`、`extreme`。
- `projects.task_difficulty` 表示执行难度，取值为空、`easy`、`normal`、`hard`、`extreme`。两项在 `1760000055` 中由文本转为单选，接口仍使用相同的字符串值。
- `hw_fields(type, key)` 和 `hw_tables(type, key)` 分别唯一。不同分类可以复用参数名，同一分类的不同分组不能重复使用同一个字段 key。
- 硬件的动态参数保留 JSON，避免新增分辨率、刷新率等测试项时修改产品表结构；固定用于筛选、排序、权限的数据继续保留为独立字段。
- 关注、认可、评分继续保留逐人关系记录；本次未新增容易与这些记录不同步的持久计数字段。

## 查询索引

迁移 `1760000056` 增加以下复合索引，保留现有唯一约束和其他索引：

| 集合 | 索引列 | 对应查询 |
| --- | --- | --- |
| `projects` | `review_status, deleted_at, published_at DESC, updated DESC, id DESC` | 公开项目分页，查询时先过滤软删除 |
| `projects` | `owner_user_id, deleted_at, updated DESC` | 我的发起项目 |
| `project_members` | `project_id, status, joined_at` | 项目有效参与者 |
| `project_follows` | `user_id, is_active` | 我的关注项目 |
| `project_completion_requests` | `project_id, status, created`；`user_id, status, created` | 项目完成申请、个人完成历史 |
| `project_join_requests` | `status, created, id` | 管理员待审核目录 |
| `hardware` | `is_public, sort_order, name`；`kind, is_public, sort_order, name`；`submitted_by` | 报告公开列表、按分类筛选、本人未公开报告 |
| `hw_fields`、`hw_tables` | `type, sort_order, id` | 按分类加载配置 |
| `messages` | `recipient_user_id, created DESC, id DESC` | 收件箱分页；已有未读索引继续使用 |
| `user_titles` | `user_id, revoked_at, created, id` | 按用户读取仍有效的头衔 |

项目列表和详情的关注统计使用数据库 `COUNT/GROUP BY`，只查询所需项目。首页热门项目由数据库按有效关注人数选出前八个，不再只从最近一百个项目中挑选。所有统计都从现有关系记录得出。

## 升级、失败和恢复

1. 三个迁移可重复执行。硬件 key 有重复或项目分级存在未知值时，迁移报错并回滚当前迁移；保留原记录，不能通过自动删除、随意改名或清空字段消除冲突。
2. PocketBase 不支持直接把文本字段改为单选。`1760000055` 在事务中暂存原列、创建单选列、原样复制已验证的值，再删除临时副本；接口字段名和现有值不变。任何一步失败均回滚。
3. 升级前按仓库根目录的备份说明保留完整数据目录和所用源码版本。若需回到旧版本，停止服务后在获授权的隔离环境恢复对应版本的备份，再使用该版本启动；不能修改生产迁移记录或对已运行的新库强行执行旧迁移。
4. 自动测试使用临时数据库，覆盖空库初始化、旧结构有数据升级、幂等、冲突回滚、并发重复写入、旧数据目录恢复后启动迁移、分页完整性和查询计划。恢复测试使用合成数据目录，不代表所有历史生产备份都已验证。
