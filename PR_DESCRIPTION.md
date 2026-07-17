# feat(metrics): DEV-35 Stage 3 integration tests + monitoring verification

## DEV-35: Stage 3 集成联调 + 监控告警 + 冒烟验证

### 概述

本 PR 完成 DEV-29（通用 metrics 基础能力）的 Stage 3 集成联调、监控告警和冒烟验证，包含 Stage 1-3 全部后端变更。

### 监控告警规则

项目当前使用结构化日志（未接入 Prometheus），可观测字段：

| 指标 | 日志字段 | 告警建议 |
|------|----------|----------|
| flush 运行计数 | `[flush-worker] flush complete: result=success\|partial_failure` | — |
| DB 写入失败 | `db_failures=N` (N>0) | 5 分钟 > 10 次 |
| Redis track 失败 | `[metrics] WARN: redis TrackView/Download failed` | 1 分钟 > 50 次 |
| flush 执行时长 | `duration=Xs` | > 30s |
| dirty set 大小 | `dirty_set_size=N` | 连续 3 周期增长 |

### GETSET + SPOP best-effort 语义

- Flush 使用 `GETSET key "0"` 原子取值并清零 — 取值和重置是同一命令，无窗口丢失
- `SPOP` 从 dirty set 中取出 key — 如果后续 DB 失败，`SADD` 回写仅保留 dirty member（**不恢复已 GETSET 清零的 delta**）
- **Best-effort 丢失场景**：DB 持续失败或进程崩溃时，已 GETSET 清零的当前批 delta 不可恢复（与 DEV-29 v5 契约一致：「不假装可恢复」）
- SADD 回写的作用：确保后续新增流量仍有重试机会，而非恢复已丢失 delta

### 已知限制

- v1 不防刷：同用户多次 view/download 均计入
- v1 不做 UV：不去重唯一访客
- flush 周期内数据有 30-60s 延迟（可配置 `METRICS_FLUSH_INTERVAL`）
- `download_count` 语义：下载 URL 成功生成次数（下载意图），不是 CDN 真实文件下载次数
- v1 不接入 MCP：后续加 resolver 和触发点即可

### 变更列表

#### Stage 1: 基础框架 (DEV-30 + DEV-32)
- `migrations/sql/20260717-03-resource-metrics.sql` — resource_metrics 表
- `internal/redis/client.go` — TrackView/TrackDownload/TrackInstall + SADD dirty
- `internal/service/metrics/` — Resolver 框架、Skill Resolver、Metrics Service、Flush Worker
- `internal/repository/metrics/repo.go` — UpsertCounts
- `internal/api/handler/metrics/` — POST /api/v1/metrics/track
- `internal/config/config.go` — METRICS_FLUSH_INTERVAL/BATCH/LOCK_TTL

#### Stage 2: 业务接入 (DEV-33)
- Skill 下载计数：下载 URL 生成成功后调用 `metrics.TrackDownload("skill", id)`
- Skill list/get LEFT JOIN resource_metrics 返回 view_count/download_count
- GET /api/v1/skills sort 参数：comprehensive(默认)/latest/downloads/views
- 综合排序公式：`download_count * 5 + view_count * 1 + 20 / POW(age_days + 2, 1.2)`

#### Stage 3: 集成测试 + 文档 (DEV-35)
- `internal/service/metrics/integration_test.go` — 14 个集成/冒烟测试
- `THREAD.md` — 完整实现进度和剩余风险

### 测试

```
go test ./... — 30 packages PASS, 0 failures
```

集成测试覆盖：
1. View 链路 Track→Redis→Flush→DB
2. Download 链路 Track→Redis→Flush→DB
3. Redis 故障不阻断主流程
4. 多 worker 分布式锁互斥
5. 综合排序公式权重
6. DB 失败后 delta 丢失（best-effort 验证）
7. DB 恢复后新流量正常捕获
8. Worker 优雅关闭

### 关联 Issue

- Parent: DEV-29 (通用 metrics 基础能力)
- DEV-30: metrics 框架基础 ✅
- DEV-32: flush worker ✅
- DEV-33: Skill 业务接入 ✅
- DEV-34: 前端接入 ✅ (octo-web)
- **DEV-35**: 本 PR — 集成联调验证
