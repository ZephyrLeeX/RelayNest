# 设备管理与 Bot 生命周期工作区恢复

任务：WORKSPACE-RECOVERY-DEVICE-LIFECYCLE-01，仅恢复工作区，未实现业务、未执行业务测试。

## 原状态与保存证据

两原 artifact 仍标为 active，但对应 RelayNest 子目录不存在，Git worktree 元数据无两树；父目录仅余 .codex-worktree-name。原分支 codex/device-admin-http、codex/bot-lifecycle-plan 都停在基线 79c4601deb71d5865e3539ddc7da7f138737ee15，reflog 仅创建记录，不能据此判断无未提交改动。

原设备路径 SHA1 对应 refs/codex/snapshots/ccbd8882520141e0544c18c6daf66cc7d93a1f10，指向 03b95101c3bf0671118aa54b1c70f7b56d9389ef。此提交为 startup-cleanup 自动快照，其唯一父为上述基线，保存原未提交工作，不代表 P04-05-S03 已实现完成。相对基线有三个路径：修改 server/cmd/relaynest/main.go、新增 server/internal/devices/http.go、修改 server/internal/httpapi/router.go，共117行新增、1行删除。已逐一比对恢复文件 git hash-object 与快照树中的 blob，全部一致。

原 Bot 路径 SHA1 对应 refs/codex/snapshots/ba27a57becb6b7df633e441f0d4b7adb2bbe41a2，值恰为基线。结合原分支与 reflog，无已提交或已保存未提交变更证据；只能确定现存保存证据为基线，无法证明曾经没有未保存内容。

## 恢复方式与结果

两原 artifact restore 均返回 No archived worktree；原设备树 attach 返回 ENOENT。app create_worktree 创建 device-admin-recovered/bot-lifecycle-recovered 虽返回 created 且 registrationError:null，紧接检查均只有父目录，RelayNest 不存在、Git 无登记；artifact 却仍 active。未反复尝试或覆盖原目录。

改原生 git worktree add --detach 到新的非 managed 目录，并建立独立分支：

- /home/lijx/Projects/RelayNest-recovery-20261024/device-admin：codex/device-admin-recovery-20261024，恢复起点 03b95101c3bf0671118aa54b1c70f7b56d9389ef。
- /home/lijx/Projects/RelayNest-recovery-20261024/bot-lifecycle：codex/bot-lifecycle-recovery-20261024，HEAD 79c4601deb71d5865e3539ddc7da7f138737ee15。

两树实际可进入、Git status 干净（写本报告前）；Bot 树与基线 git diff --exit-code 通过。未修改原 refs、快照、原父目录，未操作主 index、台账、.directory。恢复报告仅在设备恢复树独立提交。

## 后续条件

两树可交回原任务继续，但需重新派发原任务范围与检查。设备快照属于未验收中间成果，业务测试未运行，Bot 原任务无新增成果证据。app managed 生命周期故障仍待排查；本次无需其恢复即可继续业务任务。未 push 或部署。

原实现后续审计 review base 必须保留 79c4601deb71d5865e3539ddc7da7f138737ee15，使全部03b9510草稿差异进入审查；本报告提交不能用作豁免草稿的业务基线。
