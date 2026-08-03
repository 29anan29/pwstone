# pwstore

Go 编写的中文命令行密码管理器：本地存储、零外部服务、主密码加密、附带交互式 TUI。命令快捷指令为 `pw`。

## 特性

- Argon2id 密钥派生（64 MiB 内存 / 3 轮 / 4 线程），抗 GPU 离线爆破
- AES-256-GCM 逐记录加密，站点 / 账号 / 密码 / 备注全部加密，无元数据明文泄漏
- 版本化文件格式：magic + 版本 + KDF 参数内置在文件中，未来可平滑升级算法
- 持久化防暴力锁定：连续输错 5 次锁定 30 秒，状态落盘、跨进程生效
- 主密码永不落盘：命令行参数、进程列表、shell history 中均不出现密码
- 原子写入：临时文件 + fsync + rename，崩溃不损坏数据；每次保存自动生成 `.bak`
- 文件互斥锁（flock）：防止多进程并发写入导致数据损坏
- 权限加固：数据文件自动修正为 0600 / 0700，拒绝组与其他用户读取
- 完整性校验：GCM 认证标签对任何篡改 fail-closed（报告错误而非静默跳过）
- 交互式 TUI：ASCII 艺术标题、Unicode 表格边框、搜索 / 增改删 / 随机密码生成 / 复制到剪贴板 / 明文导出

## 构建

```bash
make build        # 输出 bin/pw
make test         # 运行单元测试
make install      # 安装到 $GOBIN
```

依赖 Go 1.24+。

## 使用

直接运行（无参数）进入 TUI；或用子命令（适合脚本 / 管道）：

```bash
pw                              # 交互式 TUI
pw init                         # 首次初始化主密码
pw in github.com -u alice -n 工作    # 新增/更新（密码隐藏输入）
pw out github.com -u alice      # 精确查询
pw search git                   # 模糊搜索
pw list                         # 列出全部（密码打码）
pw del github.com -u alice      # 删除
pw export -o backup.txt         # 导出明文备份
pw passwd                       # 修改主密码（全量重加密）
pw destroy                      # 自毁（输入 DESTROY 确认）
pw update                      # 检查并更新（显示更新内容，确认 y 后下载替换）
pw update check                # 仅检查版本，不更新
pw update --proxy socks5://127.0.0.1:1080   # 走代理更新
pw about                       # 版本与加密信息
```

更新采用 apt 风格：先展示当前/新版本与更新内容，输入 `y` 确认后下载、SHA256 校验并备份替换自身。国内网络可用 `--proxy socks5://...` 或 `http://...` 代理加速。

非 TTY 场景（管道）下密码从 stdin 逐行读取，例如：

```bash
printf '主密码\n站点密码\n' | pw in github.com -u alice
```

## TUI 快捷键

| 按键 | 功能 |
| --- | --- |
| `↑` `↓` / `j` `k` | 选择记录 |
| `/` | 搜索，`Enter`/`Esc` 退出搜索 |
| `空格` | 显示 / 隐藏密码 |
| `c` | 复制密码到剪贴板 |
| `a` `e` `d` | 新增 / 编辑 / 删除 |
| `g`（密码字段） | 生成随机强密码 |
| `x` | 导出明文 |
| `l` | 锁定（清除内存密钥） |
| `q` / `Ctrl+C` | 退出 |

## 文件格式（v1）

数据位于 `~/.local/share/.pwstore/vault.dat`（Linux）/ `~/Library/Application Support/.pwstore`（macOS）/ `%LOCALAPPDATA%\.pwstore`（Windows），权限 0600。

```
magic "PWSTORE1" (8B)
version u16 = 1 | kdf算法 u16 = 0(argon2id) | memory u32 | iter u32 | parallel u8 | salt(16B)
verifier: u32长度 + nonce||GCM("pwstore-verified")
记录数 u32
逐条: u32长度 + nonce||GCM(JSON{site,username,password,notes})
```

## 目录结构

```
cmd/pwstore/     入口
internal/config/ 路径与常量
internal/model/  数据模型、主密码强度、随机密码
internal/crypto/ Argon2id + AES-GCM + 版本化编解码
internal/store/  Vault CRUD、原子写、备份、导出、改密重加密、文件锁
internal/auth/   持久化锁定与解锁
internal/cli/    子命令路由、隐藏输入
internal/tui/    bubbletea TUI
```

## 安全边界

- 数据目录与所有文件权限 0600/0700，打开时自动修正过宽的权限
- 保险库文件互斥锁，多进程并发访问时后到者报错而非写坏数据
- 密钥仅存在于解锁进程内存，退出 / 锁定即清零
- 丢失主密码 = 数据无法恢复；建议定期 `export` 备份并妥善保管

## 发布

推 tag（`v*`）自动触发 CI，构建 linux / windows / mac 的 amd64 / arm64 安装包（tar.gz / zip / deb）并发布到 GitHub Releases：
