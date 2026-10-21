# P02-02 后台唤醒与补偿 PoC 实测

日期：2026-10-21。基线 SHA：`409101b`（P02-01 实现提交为 `c49acb3`）。实现 SHA 见本地提交。设备：一加 Ace 3 Pro（Android 16 / ColorOS V16.0.0），无线 adb，手机代理保持开启。

## 结论

**已接入最小 WorkManager 补偿并在真机完成后台/锁屏/Doze/force-stop 时序实测。** 四个关键结果：

1. **normal 优先级 data 提示在深度 Doze 中不投递**：一条消息发出 85 秒后仍未达，最终在观察窗（>6 分钟）内始终未补达且 `onDeletedMessages` 未触发——被服务端扣住的普通优先级消息**可能长期不可达**。
2. **HIGH 优先级可穿透深度 Doze**：受控 `force-idle` 下 118ms 送达（`prio=1/1`，到达时 `doze=true screenOn=false`），与前台延迟同量级。
3. **「推送失败重开仍同步」成立**：force-stop 后消息必然丢失送达机会，重新打开应用后 **110ms** 内 `app-open` 补偿任务运行并落盘 `last_sync`；被扣住的 401 消息随后在下一条实时消息激活通道时补达（累计延迟 181 秒）。
4. **回前台本身不冲刷积压消息**：应用前台可见 2 分钟，挂起消息未补达；直到新消息（501，137ms 送达）激活通道，401 才在 5 秒内冲出。**补偿设计应以「打开即同步」为主，不能依赖 FCM 积压冲刷。**

不增加常驻服务：manifest 仅含非前台的消息接收 Service；调度全部走 WorkManager（JobScheduler）。

## 范围与实现

在 P02-01 工程内新增（均在 `android/poc/fcm/**`）：

- `PoCApp`：进程初始化——`PoCLog` 文件持久化（修复 P02-01 遗留 2）、屏幕亮灭广播时序记录、周期补偿任务登记（30min + 网络约束 + 唯一 KEEP，T9.1 初始值）。
- `SyncWorker` + `SyncJobs`：模拟同步任务（记录触发来源与落盘 `last_sync.txt`）；唯一任务名 + KEEP 防堆积；`app-open` / `onDeletedMessages` 两个即时触发入口。
- `MainActivity`：onResume/onPause 记录前后台时序，每次前台打开登记即时补偿。
- `FcmPoCService.onMessageReceived`：增加 `recv` epoch、到达时 `screenOn/doze` 状态；`onDeletedMessages` 触发补偿。
- 依赖新增 `androidx.work:work-runtime-ktx:2.12.0`（2026-10-21 Google Maven 最新稳定版）。
- 发送桩修正 `AndroidConfig.Priority` 枚举为大写（HIGH/NORMAL）。
- 证据：`evidence/p02-02-timeline.png`（完整时序截图）。

## 实测数据（sent/recv 为 epoch 毫秒，延迟=recv-sent）

| # | 场景 | 消息 | 发送 | 延迟 | 到达时状态 |
| --- | --- | --- | --- | --- | --- |
| 201 | 灭屏约 10 分钟自然深度 Doze，normal | 15:42:47 | **187.5s** | 解锁回前台后 0.8s 补达 |
| 301 | 受控 `force-idle` 深度 Doze，normal | 15:47:52 | **未补达**（>6min，观察截止） | — |
| 302 | 同上 Doze，HIGH | 15:49:46.104 | **118ms** | doze=true screenOn=false，设备仍 IDLE |
| 401 | force-stop（进程已死），normal | 15:51:17 | **181.3s** | 15:51:59 重开；15:54:13 新消息激活通道后 5s 补达 |
| 501 | 前台 unlocked，normal（对照） | 15:54:13.194 | **137ms** | doze=false screenOn=true |

WorkManager 时序：15:41:43（锁屏后启动，Doze 中）登记的即时任务被推迟，15:45:53 解锁瞬间与 periodic 任务一并运行；15:51:59.630 重开进程，15:51:59.740 `app-open` SyncWork 运行（进程启动后 110ms），`last_sync.txt` 落盘。token 跨 force-stop/重启保持不变（`eNXTP-4t…(142)…RgdY`）。

操作命令：`dumpsys deviceidle force-idle/unforce`、`dumpsys battery unplug/reset`、`input keyevent KEYCODE_SLEEP/WAKEUP`、`am force-stop`、`am start`；发送同 P02-01 桩（`--priority high` 为 HIGH 对照）。测试期间无线 adb 在 Doze 下保持连接。

## 验收核对

- [x] 记录真实时间：全部事件带设备时间戳 + epoch sent/recv 差值。
- [x] 推送失败重开仍同步：force-stop 场景，重开后 110ms 内补偿运行并落盘。
- [x] 不增加常驻服务：无前台 Service（代码与 manifest 可核）。
- [x] 受影响检查：`:app:assembleDebug` 通过；真机五场景实测如上；无其他受影响面（PoC 隔离工程）。
- [x] 仅提交本任务变更；独立审计与集成完成后才可 `done`。

## 遗留问题

1. 301 长期未补达且无 `onDeletedMessages`：普通优先级消息在 Doze 中的服务端保留/丢弃策略未定论，若 P02-03 可复现需补测（可观察更长时间窗或 TTL 参数化）。
2. 「前台不冲刷、新消息激活冲刷」仅单次观察到（401/501），建议 P02-03 复核一次再写入门槛结论。
3. 周期任务 30 分钟的真实触发与漂移未在短窗内观察（仅记录到解锁窗口的机会性运行）；长窗行为可在 P02-03 顺带记录。
4. 高优先级穿透 Doze 的通道代价（配额、耗电）未量化——T9.1 约束「高优先级仅用于确需用户可见通知」维持不变，本卡数据支撑该边界而非放宽。
