# DEV-29 通用 Metrics 基础能力 — 实现进度

## 概览

本 THREAD 记录 DEV-29（通用 metrics 基础能力与 Skill 市场计数方案 v1）的实现进度和联调结论。

---

## Stage 1: 基础框架 ✅

### DEV-30: metrics 框架基础（已完成，已 review 通过）

分支：`feat/DEV-29-metrics-foundation`

实现内容：
- `resource_metrics` 表 migration（`20260717-03-resource-metrics.sql`）
- Redis 封装 `internal/redis/client.go`（TrackView/TrackDownload/TrackInstall + SADD dirty）
- Resolver 接口框架 + Skill Resolver（`internal/service/metrics/resolver.go`, `skill_resolver.go`）
- Metrics Service（`internal/service/metrics/service.go`）
- HTTP 接口 `POST /api/v1/metrics/track`（handler + 路由注册）
- 单测覆盖：resolver 注册/查找、SkillResolver 可见性、参数校验、Redis 失败仍返回 204

### DEV-32: Flush Worker（已完成，已 review 通过）

分支：`feat/DEV-29-metrics-foundation`（共用分支）

实现内容：
- Flush Worker（`internal/service/metrics/flush_worker.go`）
  - 默认 30s 周期，SPOP 500/批
  - 分布式锁：`SET metrics:flush:lock <instance_id> NX EX 120`
  - GETSET 原子取增量 + SET 0
  - DB UPSERT ON DUPLICATE KEY UPDATE 累加
  - 失败重试 3 次，最终 SADD 回 dirty
  - 锁释放 Lua 脚本校验 value
  - Graceful shutdown：独立 2s context 释放锁（review 修复）
- Metrics Repository（`internal/repository/metrics/repo.go`）
- Config 暴露：`METRICS_FLUSH_INTERVAL`、`METRICS_FLUSH_BATCH`、`METRICS_FLUSH_LOCK_TTL`
- 单测：miniredis + sqlmock 覆盖核心路径

---

## Stage 2: 业务接入 ✅

### DEV-33: Skill 后端业务接入（已完成，已 review 通过）

分支：`feat/DEV-33-skill-metrics-integration`

实现内容：
- Skill 下载计数：下载 URL 生成成功后调用 `metrics.TrackDownload("skill", id)`
- Skill list/get LEFT JOIN resource_metrics 返回 `view_count` / `download_count`
- GET /api/v1/skills sort 参数支持：`comprehensive`（默认）、`latest`、`downloads`、`views`
- 综合排序公式：`download_count * 5 + view_count * 1 + 20 / POW(age_days + 2, 1.2)`
- latest 用 cursor 分页，其余用 offset 分页（limit 上限 50）
- handler 层下载 metrics 失败不阻断响应
- 单测：sort_test.go、download_metrics_test.go

### DEV-34: Skill 前端接入（已完成，已 review 通过）

分支：`feat/DEV-29-skill-market-metrics-ui`

实现内容：
- SkillCard 展示 `viewCount` / `downloadCount`（Eye/Download 图标）
- SkillDetailModal 打开后 fire-and-forget 调用 `POST /metrics/track`
- 排序下拉：综合/最新/下载量/浏览量
- 分页：cursor（latest/mine）和 offset（其余）
- trackView 失败静默忽略，不弹 toast

---

## Stage 3: 集成联调 & 冒烟验证 ✅

### 1. 后端联调验证

**View 链路**：
```
详情页打开 → POST /api/v1/metrics/track {skill, id, view}
  → Redis INCR metrics:skill:{id}:view + SADD metrics:dirty
  → flush worker 30s 后 GETSET 取增量
  → UPSERT resource_metrics.view_count += delta
```
集成测试 `TestIntegration_ViewTrackFlushDB` 验证完整链路通过。

**Download 链路**：
```
GET /api/v1/skills/{id}/download 成功
  → 内部调用 TrackDownload
  → Redis INCR metrics:skill:{id}:download + SADD dirty
  → flush worker → UPSERT resource_metrics.download_count += delta
```
集成测试 `TestIntegration_DownloadTrackFlushDB` 验证通过。

### 2. 监控指标

结构化日志中可见（项目未接 Prometheus，使用 log）：
- `metrics_flush_runs_total` — 每轮 flush 记录 `result=success|partial_failure`
- `metrics_flush_db_fail_total` — flush 日志中 `db_failures=N`
- `metrics_track_redis_fail_total` — `[metrics] WARN: redis TrackView/Download failed`
- flush_duration — 每轮 flush 记录 `duration=Xms`
- dirty set size — 每轮 flush 开头记录 `dirty_set_size=N`

告警规则（建议在监控系统配置）：
- `flush_db_fail` 5分钟 > 10 次告警
- `dirty_set_size` 连续 3 周期增长告警

### 3. Redis 故障验证

集成测试 `TestIntegration_RedisDown_TrackDoesNotBlock` 和 `TestIntegration_RedisDown_FlushSkips` 验证：
- Redis 不可用时 track 操作返回错误但不阻塞/panic
- Redis 不可用时 flush worker 打日志跳过，不崩溃
- 主流程（Skill 详情页渲染、下载接口）不受影响（handler 吞掉 Redis 错误返回 204）

### 4. 多 Worker 锁验证

集成测试 `TestIntegration_MultiWorkerLock` 和 `TestIntegration_MultiWorkerLock_ValueProtection` 验证：
- 两个 worker 并发 flush，只有一个拿到锁并执行
- 释放锁时 Lua 脚本校验 value == instanceID，不误删别人的锁
- 上一轮 review 修复：shutdown 时使用独立 2s context 释放锁（`TestFlushWorker_LockRelease_AfterContextCancel`）

### 5. 综合排序验证

集成测试 `TestIntegration_ComprehensiveSort_Formula` 和 repo 层 sort_test.go 验证：
- `comprehensive`：downloads * 5 + views + recency bonus，老的高下载 skill 排前面
- `downloads`：按 download_count DESC
- `views`：按 view_count DESC
- `latest`：按 created_at DESC（cursor 分页）
- 新 skill 有 recency bonus（~8.7 分），但不会超过有 2+ 下载的 skill
- offset 分页 + limit 上限 50

### 6. 前端联调

代码审查确认：
- SkillCard 展示 `viewCount` / `downloadCount`
- 排序下拉切换触发 API 重新请求（带 sort 参数）
- SkillDetailModal `useEffect` 中 fire-and-forget `trackView(skillId)`
- `trackedRef` 防止同一次打开重复 track
- trackView 失败 `.catch(() => {})` 静默处理，不影响页面

### 7. 已知限制

- v1 不防刷（同用户多次 view/download 均计入）
- v1 不做 UV（不去重唯一访客）
- flush 周期内数据有 30-60s 延迟
- `download_count` 是下载 URL 生成次数（下载意图），不是 CDN 真实文件下载次数
- v1 不接入 MCP（后续加 resolver 和触发点即可）
- best-effort 丢失：DB 持续失败或进程崩溃时，已 GETSET 清零的 delta 不可恢复（不做 WAL）

---

## 剩余风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| flush 进程崩溃（GETSET 后、DB 写入前） | 已 GETSET 清零的当前批次 delta **不可恢复**，丢失最多一个 flush 周期的增量（30s） | best-effort 设计，日志告警；计数场景可接受短期不一致 |
| Redis 持续不可用 | 所有计数停止积累（INCR/SADD 失败） | 主流程不受影响；Redis 恢复后新流量正常累积 |
| MySQL/DB 持续失败 | 已 GETSET 的当前批次 delta **丢失**（GETSET 已将 Redis counter 清零，DB 写入失败后 delta 无法恢复）；SADD 只保留 dirty member 以便后续**新增流量**有重试机会，但本批 delta 不假装可恢复 | 进程内重试 3 次（间隔 100ms）；最终失败打 error log + metrics_flush_db_fail_total；dirty_set_size 连续增长告警提示运维介入 |
| 并发极高时 SPOP 竞争 | 理论上安全（SPOP 原子操作） | 已测试 batch 处理 |

> **关于 best-effort 丢失语义**（与 DEV-29 v5 方案契约一致）：
> Flush 使用 `GETSET key "0"` 原子取值并清零。如果后续 DB upsert 最终失败（重试耗尽），
> 该批 delta 不可恢复——这是 v1 的显式设计选择，不做 WAL/补偿。
> `SADD` 回写 dirty member 的作用仅是：确保该 resource 后续的**新增流量**仍会被 flush
> 发现并处理，而非恢复已丢失的 delta。
