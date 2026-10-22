# P03-12 插件静态检查 CI 实施报告

- 角色：developer；结果：implemented，待独立审计及集成。
- 基线：`4cf1304839d7dfdb55dd211f7f4deabb13846349`。
- 工作树：`/home/lijx/.codex/worktrees/moviepilot-ci/RelayNest`。
- 分支：`codex/moviepilot-ci-run3`。
- 变更：`.github/workflows/moviepilot-check.yml` 及本报告；没有修改插件、MP 核心、PoC 行为、其它 CI 或台账。

## 实现

实际源码在 `integrations/moviepilot/poc/`，复用现有 `verify_probe.py` 和 `verify_patch.py`，没有为卡片名移动目录或增加 helper。新增工作流在 push / pull_request 的插件目录、该工作流及 `mise.toml` 路径变化时触发，也允许手动运行。

使用 Ubuntu 24.04、Bash、固定 CPython 3.13.15；本地 `mise.toml` 声明 `3.13`，现有安装解析到 3.13.15。版本步骤同时断言精确 Python 版本和 mise 声明，后续升级需显式同步。全部 9 个现有 Python 文件使用 `compile()` 检查，不导入连接实例的操作脚本、不生成字节码。两个本地检查使用 `python -I -B`，不传真实源码或容器参数，不安装 MP 依赖，也不复制 MP 源码。

两个官方 action 的完整 SHA 经公开 Git tag 验证：

```text
git ls-remote https://github.com/actions/checkout.git refs/tags/v6.1.0
d23441a48e516b6c34aea4fa41551a30e30af803 refs/tags/v6.1.0
git ls-remote https://github.com/actions/setup-python.git refs/tags/v6.1.0
83679a892e2d95755f2dac6acb0bfd1e9ac5d548 refs/tags/v6.1.0
```

只授予 `contents: read`，checkout 禁止持久化凭据；无发布、上传、缓存、服务账号、Firebase、签名或 release secrets，无 `pull_request_target`、无秘密表达式。fork PR 使用同一套本地检查。各步骤使用默认失败传播，没有 `continue-on-error` 或掩盖退出码。

## 实际验证

环境：WSL Linux、CPython 3.13.15；`actionlint v1.7.7`（已有工具，Go 1.26.0 编译）。所有执行均在本地，无 MP/Hub 实例。当前 PATH 没有 Node，因此既有助手测试的可选 JavaScript 处理器解析分支未运行；11 项 Python 夹具测试均运行。GitHub runner 若有 Node，既有测试会额外解析处理器；本卡不把这个可选分支计为 Python 验收前提。

| 命令/检查 | 结果 |
| --- | --- |
| `/tmp/relaynest-ci-tools/actionlint -shellcheck= .github/workflows/moviepilot-check.yml` | PASS，退出 0；未运行 ShellCheck，另用 `bash -n` 验证全部运行脚本 |
| 将工作流 `run` 三个入口分别交给 `bash -n` | PASS，三个入口退出 0 |
| 工作流原版本/语法 heredoc，使用 Python 3.13.15 和 `bash -e -o pipefail` | PASS，9 个文件 |
| `python -I -B integrations/moviepilot/poc/verify_probe.py` | PASS，宿主桩、loopback HTTP 和拒绝边界 |
| `python -I -B integrations/moviepilot/poc/verify_patch.py` | PASS，11 项本地临时夹具测试 |
| 临时副本加入非法语法后运行同一语法入口 | PASS（预期失败），`SyntaxError`，退出 1 |
| 临时副本分别在两个校验入口注入 `AssertionError` | PASS（预期失败），两入口各退出 1 |
| `git diff --check` | PASS |

本地验证通过向 PATH 前置 `/home/lijx/.local/share/mise/installs/python/3.13.15/bin` 选择解释器；脚本环境未启用优化，断言有效。失败负例只写 `TemporaryDirectory`，不改工作树中的插件或校验实现。可复现验证代码如下，解释器和 actionlint 路径按本机安装调整：

```python
from pathlib import Path
import shutil
import subprocess
import tempfile

workflow = Path('.github/workflows/moviepilot-check.yml').read_text()
syntax = workflow.split('        run: |\n', 1)[1].split('\n      #', 1)[0]
syntax = '\n'.join(line[10:] for line in syntax.splitlines()) + '\n'
commands = [syntax, 'python -I -B integrations/moviepilot/poc/verify_probe.py',
            'python -I -B integrations/moviepilot/poc/verify_patch.py']
for command in commands:
    subprocess.run(['bash', '-n'], input=command, text=True, check=True)
    subprocess.run(['bash', '-e', '-o', 'pipefail', '-c', command], check=True)
for index, command in enumerate(commands):
    with tempfile.TemporaryDirectory(prefix='relaynest-p0312-negative-') as folder:
        root = Path(folder)
        shutil.copytree('integrations/moviepilot', root / 'integrations/moviepilot')
        shutil.copyfile('mise.toml', root / 'mise.toml')
        if index == 0:
            (root / 'integrations/moviepilot/broken.py').write_text('def broken(:\n')
        else:
            name = 'verify_probe.py' if index == 1 else 'verify_patch.py'
            (root / 'integrations/moviepilot/poc' / name).write_text(
                "raise AssertionError('P03-12 injected failure')\n")
        result = subprocess.run(['bash', '-e', '-o', 'pipefail', '-c', command],
                                cwd=root, text=True, capture_output=True)
        assert result.returncode != 0
        print(index + 1, result.returncode, result.stderr.splitlines()[-1])
```

## 验收与边界

- 固定兼容 Python：PASS，仅本地 CPython 3.13.15 的静态及夹具验证，不推定 MP 运行时兼容。
- 无 MP 源码复制：PASS；本地助手测试只用其既有合成夹具，不传 `--source-root`。
- 失败正确传播：PASS；三个步骤的负例均返回 1，工作流没有吞错配置。
- 受影响检查及独立提交：已运行上述本地检查；最终 SHA 由提交后的交接记录，不在报告内预写自身 SHA。
- 远端 GitHub Actions runner、实际 fork PR：NOT_RUN，未推送或触发远程运行。
- 真实 MP、Firebase/正式 Hub/Android、市场发布：NOT_RUN；本任务无访问、部署、重建、升级、挂载或业务/MCP 执行。

沿用 P01-06 用户验收结论：P1 最小 PoC 与 G-MP 放行不等于正式 P10/AC-07 通过。方案 A 仅支持固定 v3.1.5 指纹及助手显式授权应用；v3.0.0 仍是未实测的最低目标，不能宣称全部 >=3.0.0 兼容。原 NOT_RUN 项保留，完整上下文与自动编辑仍可选；本 CI 不重设 P1 硬门槛，也不把宿主/协议桩当真实 MP 验收。
