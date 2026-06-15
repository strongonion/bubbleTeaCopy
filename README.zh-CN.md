# bubblecopy

[English README](README.md)

用于从 CSV 批量执行文件/文件夹复制和移动的本地工具。它现在提供了一个基于 tview 的纯终端 TUI，完全支持鼠标和键盘交互。

## CSV 格式

必须的表头：

```csv
source,target,op,clear_target,group
```

规则：
- `op`：`copy` 或 `move`
- `clear_target`：`true` 或 `false`
- `group`：留空时默认为 `ungrouped`
- CSV 编码：推荐使用 `UTF-8`，同时支持 `GB18030/GBK`

## 运行

```bash
go run ./cmd/bubblecopy -config ./tasks.example.csv -workers 4
```

默认情况下，程序会启动基于 tview 的终端 UI。

如果省略 `-config`，程序会自动顺序查找当前目录和程序所在目录下的 `tasks.csv`、`tasks.example.csv`。
对于打包的程序，只需把 CSV 放在程序同目录，或者在已包含 CSV 的目录下运行即可。

## TUI 交互

- 界面分为左侧的分组面板、右侧的任务面板，以及底部的操作按钮。
- 你可以完全使用 **鼠标** 操作：
  - 双击或单击分组，可以一键勾选/取消勾选该分组下的所有任务。
  - 双击或单击任务，可以切换该任务的勾选状态。
  - 点击底部按钮 (`Select All`, `Unselect All`, `Dry Run`, `Execute`, `Quit`) 进行流程控制。
- 或者使用 **键盘** 操作：
  - `Tab` / `Left` / `Right`：在面板和按钮之间切换焦点
  - `Up/Down`：移动光标
  - `Space` 或 `Enter`：在列表中切换勾选状态
  - `Enter`：触发当前聚焦的按钮
  - `q` 或 `Esc`：退出

## 发布

GitHub Actions 会在推送版本标签时自动构建并发布 GitHub Release，支持：
- Windows `amd64` 和 `arm64`
- macOS `amd64` 和 `arm64`
- Linux `amd64` 和 `arm64`

触发方式示例：

```bash
git tag v1.0.0
git push origin v1.0.0
```

每个 Release 产物都包含编译好的程序、`tasks.example.csv` 以及中英文 README。
