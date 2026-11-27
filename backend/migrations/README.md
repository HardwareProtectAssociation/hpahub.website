# 共创平台数据库结构

当前数据归属、查询索引和升级恢复说明见 [database-structure.md](database-structure.md)；硬件动态参数模型见 [hardware-schema.md](../hardware-schema.md)。

依据 `项目共创平台_AI开发说明.md` v1.2（第 4—9、15、16 节）和 `项目共创平台_详细开发计划.xlsx`（数据库表、字段字典、身份字典、职业成长）建立。实现于迁移 `1760000008_co_creation_schema.go`，建表代码按模块拆分在 `co_creation_schema*.go`。

本迁移只负责数据结构、索引、约束和字典种子数据，不含接口、页面和业务权限实现。所有表均为 PocketBase collection，默认对 API 关闭（访问规则为空，仅超级管理员可读写），在对应业务接口实现后再按最小权限开放。

## 运行与验证

```sh
cd backend
go test ./migrations -run TestCoCreation -v   # 建表、部分唯一索引、经验入账幂等
go test ./...                                  # 全部后端测试
go run . migrate up --dir <临时数据目录>        # 在空库或已有库上实际执行迁移
```

迁移随后端启动自动执行。已有数据库升级时只新增集合/字段，并回填 `users.account_status`、`users.registration_source`；不会改动现有账号、硬件库、仓库和邀请码表。执行前仍应按 `README.md` 备份 `pb_data`。

名片展示设置由 `1760000025_profile_visibility.go` 增加 `users.hidden_profile_sections` 保存；空值保持原有公开行为，隐藏不会删除资料；细项以类型和记录 ID 保存，公开接口仅返回遮挡占位。`1760000045_profile_project_access.go` 增加 `users.profile_project_access`，只为单独隐藏的项目保存角色和成员白名单；整个项目栏目隐藏时白名单不生效。

## 集合总览

共创平台文档中的 40 张业务表与 1 张用户扩展表均已建立，集合名与文档表名一致。后续功能迁移可增加独立集合，例如 `1760000040` 新增的成员头衔记录 `user_titles`。

| 模块 | 集合 |
| --- | --- |
| 账号 | `users`（扩展）、`membership_invitation_codes`、`user_titles`、`membership_applications`（无邀请码入会申请） |
| 个人空间 | `collaboration_identities`、`user_identities`、`skills`、`user_skills`、`user_collaboration_preferences`、`user_showcases`、`user_skill_likes`（技能点赞扩展表） |
| 项目核心 | `projects`、`project_stages`、`project_review_records` |
| 成员权限 | `project_members`、`project_join_requests`、`project_permission_grants`、`project_permission_requests`、`project_participant_roles` |
| 内容互动 | `project_contents`、`media_assets`、`project_comments`、`project_ratings`、`project_follows`、`project_completion_ratings` |
| 赞助奖励 | `project_rewards`、`project_sponsorship_needs`、`project_sponsorships`、`project_agreements`、`project_reward_fulfillments` |
| 匹配 | `project_skill_needs` |
| 联合、接管、治理 | `project_unions`、`project_union_members`、`project_takeover_requests`、`project_deletion_requests`、`notifications`、`audit_events`、`reports` |
| 职业成长 | `project_tasks`、`project_task_contributions`、`task_xp_ballots`、`task_xp_votes`、`career_xp_ledger`、`career_level_thresholds` |

每个集合都带 `id`、`created`、`updated`，对应文档的 `created_at`、`updated_at`。

## PocketBase 适配与字段差异

文档字段字典以 PostgreSQL 为建议目标；本项目沿用现有 Go + PocketBase（SQLite）+ Svelte 技术栈，因此有以下可审计的差异：

1. **主键与时间**：`id` 为 PocketBase 15 位文本，不是 UUID；`created`/`updated` 为 autodate，对应 `created_at`/`updated_at`。业务时间字段（如 `submitted_at`、`reviewed_at`）使用 DateField，未设置为空字符串，统一 UTC 存储。
2. **关系字段**：`*_id` 字段为 PocketBase relation，写入时校验目标记录存在，但**不是数据库级外键**。删除保护依赖软删除和服务层校验，不建级联删除，避免误删历史。
3. **状态枚举**：使用 SelectField 限制可选值；PocketBase 不提供数据库 CHECK 约束，合法状态转换仍需服务端校验。
4. **金额**：SQLite 没有精确小数，金额改为整数最小单位（分）：`target_amount_cents`、`pledged_amount_cents`、`delivered_amount_cents`，币种保留在 `currency`。接口层必须按分换算，禁止用浮点参与金额计算。
5. **数组**：`help_types` 为 JSON 数组；`expected_reward_types` 为多选 SelectField（money/product/title/credit/learning/none/other），与奖励类型字典一致。
6. **JSON 字段**：`description`、`body`、`terms_snapshot`、`rules_snapshot`、`allocation_snapshot`、`reward_snapshot`、`handover_snapshot`、`payload`、`before_data`、`after_data` 为 JSONField。
7. **自引用**：`parent_content_id`、`parent_comment_id`、`supersedes_id`、`reverses_entry_id` 在集合创建后追加（PocketBase 要求被引用集合已经存在）。
8. **可空数值**：NumberField 没有 NULL，未设置为 0。约定 0 表示“未填写/未知”；对 0 有业务含义的字段用伴随字段区分，详见下一节。
9. **默认值**：PocketBase 字段没有默认值，布尔列固定默认 false。文档中默认为 true 的字段（如 `enabled`、`xp_enabled`、`accepts_sponsorship`、`is_active`、`receive_updates`）以及其余默认值由服务层写入，见“服务层必须补齐的规则”。
10. **平台角色**：复用现有 `users.role`（`member`/`user` 视为普通会员，`admin` 为平台管理员），超级管理员为 PocketBase `_superusers`，即文档中的 `super_admin`。没有新增 `platform_role` 字段，避免两套角色双写。`display_name` 为新增可选字段，现存量账号未自动生成昵称，服务层应在完善资料时要求填写。
11. **`users` 其他映射**：`avatar` 文件字段对应 `avatar_url`，`avatar_thumb` 保存头像展示缩略图，`avatar_cropped` 保存旧头像迁移生成的裁切图，原 `avatar` 保留作为回退，`bio` 沿用现有字段，`display_name` 为昵称；`gender`、`personality`、`age`、`contacts`、`activity_areas` 为个人资料扩展字段；`user_identities.featured` 标记“想要展示的职业”，服务层限制最多 3 个；`user_skills.featured` 标记“在公开主页技能展示中列出”，数量不限；`email`/`password` 由 PocketBase 认证管理；`users.createRule/updateRule/deleteRule` 仍为空（仅超级管理员），未在本次迁移放宽。
12. **项目难度**：`projects.difficulty` 取值为 `D`、`C`、`B`、`A`、`S`，由管理员在“项目管理”页判定；`1760000013` 为已有项目回填 `C`，公开项目接口返回该等级。
13. **管理控制台与邀请码**：管理控制台显示成员目录、待审批项目加入申请和全部项目；项目成员申请通过后写入 `project_members`。邀请码从控制台移到个人中心，普通成员只能查看和生成自己的邀请码；`hpahub_invitations` 增加 `created_by`，启动时对旧表自动补列。
14. **健康评估（已取消）**：功能入口和后端接口已移除；`users.current_status`、`daily_health_enabled` 与 `health_checkins` 表结构保留，避免删除历史数据。
15. **已完成项目改进**：`project_join_requests.source` 增加 `improvement`；已完成项目的详情页只提供改进申请，普通加入申请会返回 400；管理控制台按来源显示“申请改进”或“申请加入”。
16. **硬件新闻**：`hardware_news` 保存成员发布的标题、正文和原文链接；`hardware_news_reactions` 每个成员对每条新闻最多一个反应，取值为 `like` 或 `useless`，作者不能给自己投票。
17. **成员互动关系**：`member_relations` 保存有向关系 `admire`（关注）/`watch`（监视）/`investigate`（调查），同一对成员只能有一条关系；公开成员页显示被关注、被监视、被调查数量及当前访问者的关系。
18. **硬件通用模板**：删除原显卡/CPU/设备三套字段模板，改为单一 `general` 通用模板（名称、品牌、型号、分类、备注）；已有硬件记录统一转为 `general`，不会删除记录。
19. **难度投票**：`projects` 新增 `owner_proposed_difficulty`（发起人自评）和 `owner_difficulty_confirmed`（发起人确认）；`project_difficulty_votes` 记录管理员投票（同意/反对某难度等级）。共识难度需 ≥3 票同意、0 票反对、发起人确认；项目完成后难度锁定。
20. **站内消息**：`messages` 表支持用户间消息和系统通知（项目审核通过/拒绝）；消息带分类（user/system/project_approval）、已读状态和关联项目。
21. **硬件评测**：`hardware_reviews` 保存成员对硬件记录的评测（标题、正文、作者）；`hardware_review_likes` 每个成员对每篇评测最多点赞一次，作者不能给自己的评测点赞；首页“热门硬件”按评测点赞总数排序，同数按 `hardware.sort_order`。
12. **0 与未知**：`weekly_hours`、`expected_weekly_hours`、`estimated_duration_days`、`people_needed`、`quantity_limit`、`target_amount_cents`、`target_quantity`、`pledged_*`、`delivered_*`、`allocation_basis_points`、`eligible_count`、`valid_vote_count`、`final_xp` 的 0 都按“未填写”解释。`task_xp_votes.xp_value` 为 0 且有 `voted_at` 时表示投了 0 经验；`task_xp_ballots.final_xp` 为 0 且 `closed_at` 非空时表示有效结果为 0；贡献份额 0 且状态为 `confirmed` 表示确认的 0 份额。`career_level_thresholds.minimum_total_xp` 的 0 是等级 1 的合法门槛，`eligible_count` 为 0 表示合法但无人具备资格的边界；两者因 NumberField 不能表达 NULL 而保持可选。这样不额外增加可空标记字段。

## 唯一约束与部分唯一索引

| 集合 | 索引 | 含义 |
| --- | --- | --- |
| `collaboration_identities` | UNIQUE(category, name) | 同一分类下身份名唯一 |
| `skills` | UNIQUE(name) | 技能词唯一 |
| `user_identities` | UNIQUE(user_id, identity_id) | 身份多选不重复 |
| `user_skills` | UNIQUE(user_id, skill_id) | 技能不重复 |
| `user_collaboration_preferences` | UNIQUE(user_id) | 一人一份合作偏好 |
| `membership_invitation_codes` | UNIQUE(code_hash)；UNIQUE(invitee_user_id) WHERE invitee_user_id != '' | 一码一人；未用码允许并存 |
| `project_members` | UNIQUE(project_id, user_id) | 一个项目一条成员关系 |
| `project_join_requests` | UNIQUE(project_id, user_id) WHERE status = 'pending' | 同项目同用户最多一条待处理申请 |
| `project_permission_grants` | UNIQUE(project_id, user_id, permission_code) | 同一权限只有一条有效/撤销记录 |
| `project_permission_requests` | UNIQUE(project_id, user_id, permission_code) WHERE status = 'pending' | 同一权限最多一条待审批申请 |
| `project_ratings` | UNIQUE(content_id, user_id) | 一人一内容一条评分 |
| `project_follows` | UNIQUE(project_id, user_id) | 关注关系不重复 |
| `hardware_review_likes` | UNIQUE(review_id, user_id) | 一篇评测每人最多点一次赞 |
| `project_union_members` | UNIQUE(union_id, project_id) | 一个项目在一个联合中只出现一次 |
| `project_takeover_requests` | UNIQUE(project_id, applicant_user_id) WHERE status = 'pending' | 同项目同申请人最多一条待处理接管 |
| `project_deletion_requests` | UNIQUE(project_id) WHERE status = 'pending' | 同项目最多一条待处理删除 |
| `project_participant_roles` | UNIQUE(project_id, user_id, identity_id) | 同项目多角色可并存且不重复 |
| `project_completion_ratings` | UNIQUE(project_id, user_id) | 一人一项目一条整体评分，与内容评分分开 |
| `project_task_contributions` | UNIQUE(task_id, user_id, career_identity_id) | 同任务同职业一条贡献 |
| `task_xp_ballots` | UNIQUE(task_id, round_number) | 任务投票轮次唯一 |
| `task_xp_votes` | UNIQUE(ballot_id, user_id) | 一人一轮一票 |
| `career_xp_ledger` | UNIQUE(business_key)；UNIQUE(contribution_id) WHERE entry_kind = 'award' | 结算幂等；同一贡献只有一次奖励入账，撤销账保留 |
| `career_level_thresholds` | UNIQUE(career_identity_id, level_number) | 每个职业每级一条门槛 |
| `notifications` | UNIQUE(recipient_user_id, event_key) | 通知事件去重 |

其余为查询索引：项目状态、阶段排序、内容时间线、评论、通知已读、审计、任务状态、经验账本按用户/职业等。

## 服务层必须补齐的规则

数据库只能表达唯一性和字段类型，以下规则需在 M0—M6 的服务端实现，不能只靠前端：

- **项目默认值与可见性**：`review_status=draft`、`execution_status=ongoing`、`collaboration_mode=independent`、`join_mode=approval`、`accepts_sponsorship=true`、`version=1`、`adventure_style=project`；公开项目必须 `review_status=approved` 且有 `responsible_admin_id`；`deleted_at` 非空的项目统一从详情、搜索、推荐、附件访问中过滤。
- **状态转换**：审核 `draft→pending→approved/rejected`、执行状态四种、加入/权限/接管/删除申请的状态流转，均在事务中校验并写 `audit_events`；`version` 用于并发修改和接管锁。
- **同项目一致性**：`current_stage_id` 必须属于本项目；内容父节点、阶段、赞助、任务部件等引用必须同项目；评论父级必须在同一内容内；任务贡献引用的角色必须同项目、同用户、同职业。
- **举报与展示**：`reports` 的 project/content/comment 三个目标恰好选一个；`user_showcases` 的 `project_id` 与 `external_url` 至少一个有效。
- **数值边界**：评分 1—5；每周时间 0—168；金额、数量、人数非负；`allocation_basis_points` 0—10000 且同任务确认份额合计 10000（D17）。
- **默认值**：`users.account_status=active`、新注册 `registration_source=invitation`、`display_name` 在完善资料时必填；`collaboration_identities` 的 `enabled`/`xp_enabled` 默认 true；观看关系 `is_active`/`receive_updates` 默认 true；`project_members.joined_at`、`granted_at`、`followed_at`、`reviewed_at`、`opens_at`、`closes_at`、`eligible_at` 由服务填写当前时间。
- **邀请注册**：注册事务内锁定邀请码、创建账号、写 `invitee_user_id`/`used_at`、绑定邀请人；失败整体回滚。已用码不可转绑，账号停用不删除邀请边。
- **经验结算**：只有 `status=confirmed` 的贡献和有效关闭的投票可结算；`career_xp_ledger` 的 `business_key` 使用贡献/结算业务键，不能使用请求随机值；逆转用 `entry_kind=reversal` 并保留原账。

## 种子数据

- `collaboration_identities`：32 条，来自计划第 4 节（14 个主力职业、18 个辅助职业）。名称为建议字典，可后台增删/停用。
- `skills`：31 条起步技能词典，用于搜索与匹配，可继续补充。
- `career_level_thresholds`：32 个职业 × 100 级（迁移 `1760000009` 把示例曲线扩展到上限 100 级），曲线为 `xp(L) = 50 × L × (L-1)`，即 0/100/300/600/1000/1500…，100 级为 495,000。门槛保存在数据库，后续可以调整曲线或继续加 101 级以上的新迁移。
- 职业徽章：前端每 10 级显示一个占位小徽章（Lv.10—Lv.100），具体样式待设计；当前徽章仍由职业等级派生，没有新增独立徽章表。
- 未生成演示用户、项目、赞助和任务数据。根会员初始化（D21）和演示数据属于 M0/M6 的后续任务；现有站点账号在升级后视为根会员来源（`registration_source=seed`）。

## 与现有站点的关系

- 现有 `hpahub_invitations` 是运行时创建的明文邀请码表，服务于旧注册流程；本次新增的 `membership_invitation_codes` 是 v1.2 邀请链模型。两者暂不合并，等 M0 注册服务统一时再切换并保留旧表数据。
- `users` 新增字段不影响现有登录、角色判断和公开资料查询；现有 `role` 取值不变。
- 硬件库（`hw_*`、`hardware`）、仓库（`warehouse_*`）和公共内容集合不受影响。
- 所有新集合的 API 规则为空，登录用户暂时无法通过 PocketBase REST 读写；后续业务接口应通过 Go 路由实现服务端权限，或逐项开放最小规则。

## 已知限制与后续事项

- 没有数据库级外键和 CHECK 约束，跨字段/跨表规则依赖服务层；删除一律走软删除或管理员操作。
- 金额以分为单位，接口和前端展示需要统一转换；任何汇总必须使用整数。
- 0 表示未填写，无法区分“真实的 0 小时”和“未填”；`xp_value`/`final_xp`/贡献份额通过伴随字段区分。
- 评分分制（D05/D20）、经验投票计票（D16）、贡献分配（D17）、等级曲线（D18）、接管审批（D01—D03）、邀请码配额（D22）、角色确认（D23）等仍待决策；结构已留字段，不把建议值写成已确认规则。
- 迁移文件已到 24 个；新增 `1760000010`（技能点赞表）、`1760000011`（技能图片与链接）、`1760000012`（个人资料字段与展示职业标记）、`1760000013`（项目难度 D-S）、`1760000014`（当前状态、每日健康评估开关与记录）、`1760000015`（已完成项目的改进申请来源）、`1760000016`（硬件新闻与新闻反应）、`1760000017`（成员互动关系）、`1760000018`（硬件数据库通用空白模板）、`1760000019`（项目难度投票机制）、`1760000020`（站内消息系统）、`1760000021`（用户硬件配置）、`1760000022`（加入申请增加职业时间能力字段）、`1760000023`（硬件评测与评测点赞）、`1760000024`（技能公开展示标记）。现有迁移已在环境中应用，暂保留历史；确需建立新基线时应先核对唯一生产部署和所需备份的迁移记录。后续结构变更应新增迁移，不要修改已部署的 `1760000008` 及之后的迁移。

- **长期项目**：`1760000026` 为项目新增 `is_long_term` 标记；未设置周期的旧项目保持非长期状态。

- **项目性质**：`1760000027` 为项目新增 `project_nature`；已有项目保持未指定性质，旧冒险风格数据保留。

- **项目安排**：`1760000028` 添加项目管理设置、保密协议和“未开始”状态；保密协议不限制项目公开访问。

- **长期项目状态**：`1760000029` 为项目执行状态增加 `long_term`，保留已有项目状态及周期标记。

- **项目需求**：`1760000030` 为项目新增多选 `project_needs`（资金、技术、设备、人手、宣传、情绪支持），已有项目保持空值。

- **需求和提供说明**：`1760000035` 为每个已选项目需求和发起人提供项增加可选说明；旧项目保持空值。
- **项目提供**：`1760000031` 为项目新增多选 `provider_offers`（技术、资金、设备、场地、人手、宣传），并给 `project_needs` 补充“场地”；已有项目保持空值。

- **项目头衔认证和成员头衔**：`1760000040` 为项目提供项增加“头衔认证”（自定义名称最多 10 字），新增 `user_titles` 保存管理员授予的官方头衔、授予者与撤销时间；撤销保留记录，成员资料只展示有效头衔。
- **项目完成后授予头衔**：`1760000046` 为头衔记录增加来源项目，并将唯一范围调整为成员、头衔名和来源项目；项目发起人只能在项目审核通过且状态为已完成后，向有效参与者授予该项目配置的非官方头衔，记录授予者。

- **项目成员退出与请离**：`1760000041` 新增 `project_member_departure_requests`，记录退出/请离原因、48 小时处理期限、处理状态和审核人。发起人或参与者拒绝、或 48 小时未处理时转交管理员；批准退出仅更新成员状态并保留原参与记录。后台调度每分钟检查超时申请。
- **未结申请唯一索引**：`1760000042` 将项目成员申请唯一索引限定到非空 `open_request_key`，已结案记录清空键但继续保留历史。

- **卡片画布背景图**：`1760000043` 将会员卡片图片上限扩展为 5 张，供画布背景和前景元素共同使用；背景图片索引保存在卡片 `layout.backgroundImage` 中。

- **个人生日与佩戴头衔**：`1760000044` 为用户新增可选生日和头衔选择字段；生日按年月日保存，头衔字段保存已获得头衔的记录 ID，不能自行创建头衔。
- **个人资料修改周期**：`1760000047` 为用户记录添加上次修改个人资料的时间戳，资料与头像通过个人资料接口每 7 天最多成功保存一次。

- **项目封面**：`1760000032` 为项目新增单文件字段 `cover`（JPEG/PNG/WebP/GIF，最多 2 MiB），已有项目保持无封面。

- **项目版本**：`1760000033` 新增 `project_versions`（项目、版本号、编辑者、快照 JSON）；每次编辑项目写入一个新版本，`projects.version` 同步递增。
- **项目资料修改周期**：`1760000048` 为项目记录添加上次信息修改时间戳；项目信息每 24 小时最多成功修改一次，封面更新使用独立接口。
- **项目编号**：`1760000049` 为所有已有项目补充分配唯一的 `HPA-` 加五位数字编号，并为后续项目创建设置唯一索引；编号由服务端分配，不随项目资料版本变更。
- **可重复项目完成记录**：`1760000050` 为项目增加每名参与者的重复完成上限，并新增逐次完成申请、审核和历史记录；每次通过作为一条个人完成记录保存，不会结束项目。
- **无邀请码入会申请**：`1760000051` 新增公开入会申请记录，保存申请人邮箱、昵称、理由和审核结果；管理员通过后生成一个 24 小时有效的一次性邀请码。
- **项目赞助者信息**：`1760000052` 为项目增加可选的赞助者名称或组织字段，随项目版本保存并在项目信息中展示。
- **硬件模板 key 唯一约束**：`1760000054` 分别约束 `hw_fields(type, key)`、`hw_tables(type, key)`；已有重复记录会使本次迁移停止并回滚，不自动去重或删除数据。
- **项目分级约束**：`1760000055` 在事务中将危险性和执行难度改为可选单选字段；保留字段值，未知值会阻止迁移。
- **业务查询索引**：`1760000056` 为项目、成员、关注、完成申请、硬件、配置、消息和头衔增加与实际筛选和排序对应的查询索引。
- **项目危险性与执行难度**：`1760000053` 为项目增加危险性和难度两个可选分级字段，随项目版本保存并在项目信息中展示。

- **状态原因**：`1760000034` 为项目新增 `status_reason`；暂停或中止时必须填写原因，恢复其他状态时清空。


## 硬件数据库

硬件分类、动态字段与测试表格由 `hw_types`、`hw_fields`、`hw_tables` 配置，单件硬件的结构化实测数据保存在 `hardware.data`。`1760000037` 新增 `hardware_articles`，与硬件报告分开保存图文专栏正文和图片；`1760000038` 为动态字段补充类型、单位、必填、选项、帮助文字，并为硬件记录增加公开状态和封面；`1760000039` 记录报告提交成员。所有成员可发布硬件报告与专栏文章，成员仅能编辑自己的报告；字段配置仍由管理员维护。字段解释及扩展方式见 [`hardware-schema.md`](../hardware-schema.md)。
