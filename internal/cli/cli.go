package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"

	"pwstore/internal/auth"
	"pwstore/internal/config"
	"pwstore/internal/model"
	"pwstore/internal/store"
	"pwstore/internal/tui"
	"pwstore/internal/updater"
)

type CLI struct {
	in   io.Reader
	out  io.Writer
	err  io.Writer
	scan *bufio.Scanner
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	c := &CLI{in: stdin, out: stdout, err: stderr}
	if err := config.InitDir(); err != nil {
		fmt.Fprintln(stderr, "初始化数据目录失败:", err)
		return 1
	}
	if len(args) == 0 {
		return c.runTUI()
	}
	switch args[0] {
	case "init":
		return c.dispatch(c.cmdInit(args[1:]))
	case "in":
		return c.dispatch(c.cmdIn(args[1:]))
	case "out":
		return c.dispatch(c.cmdOut(args[1:]))
	case "search":
		return c.dispatch(c.cmdSearch(args[1:]))
	case "list":
		return c.dispatch(c.cmdList(args[1:]))
	case "del":
		return c.dispatch(c.cmdDel(args[1:]))
	case "export":
		return c.dispatch(c.cmdExport(args[1:]))
	case "passwd":
		return c.dispatch(c.cmdPasswd(args[1:]))
	case "destroy":
		return c.dispatch(c.cmdDestroy(args[1:]))
	case "update":
		return c.dispatch(c.cmdUpdate(args[1:]))
	case "version", "-v", "--version":
		fmt.Fprintf(c.out, "pw %s\n", config.Version)
		return 0
	case "about":
		return c.dispatch(c.cmdAbout(args[1:]))
	case "help", "-h", "--help":
		c.printHelp()
		return 0
	default:
		fmt.Fprintf(c.err, "未知命令: %s\n\n", args[0])
		c.printHelp()
		return 2
	}
}

func (c *CLI) dispatch(err error) int {
	if err == nil {
		return 0
	}
	fmt.Fprintln(c.err, "✗ "+err.Error())
	if errors.Is(err, auth.ErrLocked) || errors.Is(err, store.ErrWrongPassword) {
		return 3
	}
	return 1
}

func (c *CLI) runTUI() int {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		c.printHelp()
		return 0
	}
	if err := tui.Run(); err != nil {
		fmt.Fprintln(c.err, "TUI 错误:", err)
		return 1
	}
	return 0
}

func (c *CLI) cmdInit(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("用法: pw init（init 不接受额外参数）")
	}
	if store.Exists(config.VaultPath()) {
		return errors.New("数据已存在，如需重置请先 destroy")
	}
	fmt.Fprintln(c.out, "首次使用 - 设置主密码")
	fmt.Fprintln(c.out, "要求: 至少 12 位，且包含大写、小写、数字、特殊符号中至少 3 类")
	for {
		master, err := c.readSecret("设置主密码: ")
		if err != nil {
			return err
		}
		if ok, msg := model.CheckMasterStrength(master); !ok {
			fmt.Fprintln(c.out, "✗ "+msg)
			continue
		}
		master2, err := c.readSecret("再次确认主密码: ")
		if err != nil {
			return err
		}
		if master != master2 {
			fmt.Fprintln(c.out, "✗ 两次输入不一致")
			continue
		}
		if _, err := store.Create(config.VaultPath(), master); err != nil {
			return err
		}
		fmt.Fprintln(c.out, "✓ 主密码已设置")
		return nil
	}
}

func (c *CLI) unlockLoop() (*store.Vault, error) {
	a := auth.New()
	if err := a.CheckLock(); err != nil {
		return nil, err
	}
	for {
		pw, err := c.readSecret("请输入主密码: ")
		if err != nil {
			return nil, err
		}
		v, err := a.Unlock(config.VaultPath(), pw)
		if err == nil {
			return v, nil
		}
		if errors.Is(err, auth.ErrLocked) {
			return nil, err
		}
		fmt.Fprintln(c.out, "✗ "+err.Error())
	}
}

func (c *CLI) cmdIn(args []string) error {
	pos, flags, err := parseArgs(args)
	if err != nil {
		return err
	}
	if len(pos) < 1 {
		return errors.New("用法: pw in <网站> [-u 账号] [-n 备注]")
	}
	site := pos[0]
	user := flags["user"]
	note := flags["note"]

	if !store.Exists(config.VaultPath()) {
		return errors.New("尚未初始化，请先运行: pw init")
	}
	v, err := c.unlockLoop()
	if err != nil {
		return err
	}
	defer v.Close()

	pass, err := c.readSecret("请输入该站点密码: ")
	if err != nil {
		return err
	}
	created, err := v.Upsert(model.Entry{Site: site, Username: user, Password: pass, Notes: note})
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintf(c.out, "✓ 已添加: %s / %s\n", site, user)
	} else {
		fmt.Fprintf(c.out, "✓ 已更新: %s / %s\n", site, user)
	}
	return nil
}

func (c *CLI) cmdOut(args []string) error {
	pos, flags, err := parseArgs(args)
	if err != nil {
		return err
	}
	if len(pos) < 1 {
		return errors.New("用法: pw out <网站> [-u 账号]")
	}
	v, err := c.unlockLoop()
	if err != nil {
		return err
	}
	defer v.Close()
	e, ok := v.Get(pos[0], flags["user"])
	if !ok {
		return fmt.Errorf("✗ 未找到: %s / %s", pos[0], flags["user"])
	}
	fmt.Fprintln(c.out, "━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintf(c.out, "  网站: %s\n", e.Site)
	fmt.Fprintf(c.out, "  账号: %s\n", e.Username)
	fmt.Fprintf(c.out, "  密码: %s\n", e.Password)
	if e.Notes != "" {
		fmt.Fprintf(c.out, "  备注: %s\n", e.Notes)
	}
	fmt.Fprintln(c.out, "━━━━━━━━━━━━━━━━━━━━━━")
	return nil
}

func (c *CLI) cmdSearch(args []string) error {
	if len(args) == 0 {
		return errors.New("用法: pw search <关键字>")
	}
	v, err := c.unlockLoop()
	if err != nil {
		return err
	}
	defer v.Close()
	results := v.Search(strings.Join(args, " "))
	if len(results) == 0 {
		fmt.Fprintf(c.out, "✗ 未找到匹配: %s\n", strings.Join(args, " "))
		return nil
	}
	for i, e := range results {
		fmt.Fprintln(c.out, "━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Fprintf(c.out, "  [%d] 网站: %s\n", i+1, e.Site)
		fmt.Fprintf(c.out, "      账号: %s\n", e.Username)
		fmt.Fprintf(c.out, "      密码: %s\n", e.Password)
		if e.Notes != "" {
			fmt.Fprintf(c.out, "      备注: %s\n", e.Notes)
		}
	}
	fmt.Fprintln(c.out, "━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintf(c.out, "共找到 %d 条记录\n", len(results))
	return nil
}

func (c *CLI) cmdList(args []string) error {
	v, err := c.unlockLoop()
	if err != nil {
		return err
	}
	defer v.Close()
	entries := v.Entries()
	if len(entries) == 0 {
		fmt.Fprintln(c.out, "-  暂无保存的记录")
		return nil
	}
	for i, e := range entries {
		fmt.Fprintf(c.out, "  [%d] %s / %s  %s\n", i+1, e.Site, e.Username, maskPassword(e.Password))
	}
	fmt.Fprintf(c.out, "\n共 %d 条记录\n", len(entries))
	return nil
}

func (c *CLI) cmdDel(args []string) error {
	pos, flags, err := parseArgs(args)
	if err != nil {
		return err
	}
	if len(pos) < 1 {
		return errors.New("用法: pw del <网站> [-u 账号]")
	}
	v, err := c.unlockLoop()
	if err != nil {
		return err
	}
	defer v.Close()
	if !v.Delete(pos[0], flags["user"]) {
		return fmt.Errorf("✗ 未找到: %s / %s", pos[0], flags["user"])
	}
	fmt.Fprintf(c.out, "✓ 已删除: %s / %s\n", pos[0], flags["user"])
	return nil
}

func (c *CLI) cmdExport(args []string) error {
	_, flags, err := parseArgs(args)
	if err != nil {
		return err
	}
	v, err := c.unlockLoop()
	if err != nil {
		return err
	}
	defer v.Close()
	if len(v.Entries()) == 0 {
		fmt.Fprintln(c.out, "-  无数据可导出")
		return nil
	}
	path := flags["output"]
	if path == "" {
		path = filepath.Join(".", "pw_export_"+time.Now().Format("20060102_150405")+".txt")
	}
	if err := v.Export(path); err != nil {
		return err
	}
	fmt.Fprintf(c.out, "✓ 已导出到: %s\n", path)
	return nil
}

func (c *CLI) cmdPasswd(args []string) error {
	v, err := c.unlockLoop()
	if err != nil {
		return err
	}
	defer v.Close()
	for {
		np, err := c.readSecret("设置新主密码: ")
		if err != nil {
			return err
		}
		if ok, msg := model.CheckMasterStrength(np); !ok {
			fmt.Fprintln(c.out, "✗ "+msg)
			continue
		}
		np2, err := c.readSecret("再次确认新主密码: ")
		if err != nil {
			return err
		}
		if np != np2 {
			fmt.Fprintln(c.out, "✗ 两次输入不一致")
			continue
		}
		if err := v.Rekey(np); err != nil {
			return err
		}
		fmt.Fprintln(c.out, "✓ 主密码已更新（所有数据已用新密钥重新加密）")
		return nil
	}
}

func (c *CLI) cmdDestroy(args []string) error {
	fmt.Fprint(c.out, "! 自毁将删除所有数据（不可恢复），确认输入 DESTROY: ")
	var confirm string
	if term.IsTerminal(int(os.Stdin.Fd())) {
		_, _ = fmt.Scanln(&confirm)
	} else {
		if c.scan == nil {
			c.scan = bufio.NewScanner(c.in)
		}
		if c.scan.Scan() {
			confirm = c.scan.Text()
		}
	}
	if strings.TrimSpace(confirm) != "DESTROY" {
		fmt.Fprintln(c.out, "已取消")
		return nil
	}
	removed := 0
	for _, f := range []string{config.VaultPath(), config.VaultPath() + ".bak", config.LockPath()} {
		if err := os.Remove(f); err == nil {
			removed++
		}
	}
	_ = os.Remove(config.AppDir())
	fmt.Fprintf(c.out, "自毁完成，已清除 %d 个文件\n", removed)
	return nil
}

func (c *CLI) cmdUpdate(args []string) error {
	pos, flags, err := parseArgs(args)
	if err != nil {
		return err
	}
	proxy := flags["proxy"]
	checkOnly := len(pos) > 0 && pos[0] == "check"

	fmt.Fprintln(c.out, "正在检查更新…")
	rel, err := updater.Latest(proxy)
	if err != nil {
		return err
	}
	cur := config.Version
	if !updater.IsNewer(cur, rel.Tag) {
		fmt.Fprintf(c.out, "已是最新版本 v%s\n", rel.Tag)
		return nil
	}

	fmt.Fprintln(c.out, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintf(c.out, "  当前版本: v%s\n", cur)
	fmt.Fprintf(c.out, "  新版本:   %s\n", rel.Tag)
	if rel.Name != "" {
		fmt.Fprintf(c.out, "  名称:     %s\n", rel.Name)
	}
	fmt.Fprintln(c.out, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintln(c.out, "更新内容:")
	notes := strings.TrimSpace(rel.Notes)
	if notes == "" {
		notes = "（无发布说明）"
	}
	for _, line := range strings.Split(notes, "\n") {
		fmt.Fprintln(c.out, "  "+line)
	}
	fmt.Fprintln(c.out, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if checkOnly {
		return nil
	}

	answer, err := c.readLine("确认更新? [y/N]: ")
	if err != nil {
		return err
	}
	if strings.ToLower(answer) != "y" && strings.ToLower(answer) != "yes" {
		fmt.Fprintln(c.out, "已取消更新")
		return nil
	}

	asset := updater.AssetNameFor(rel.Tag)
	fmt.Fprintf(c.out, "正在下载 %s …\n", asset)
	tmp, err := updater.Download(proxy, rel.Tag, asset)
	if err != nil {
		return err
	}
	defer os.Remove(filepath.Dir(tmp))
	fmt.Fprintln(c.out, "校验通过，正在替换…")
	if err := updater.Replace(tmp); err != nil {
		return fmt.Errorf("替换失败: %w", err)
	}
	fmt.Fprintln(c.out, "更新完成，请重新运行 pw")
	return nil
}

func (c *CLI) readLine(prompt string) (string, error) {
	fmt.Fprint(c.out, prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		sc := bufio.NewScanner(os.Stdin)
		if !sc.Scan() {
			return "", errors.New("无法读取输入")
		}
		return strings.TrimSpace(sc.Text()), nil
	}
	if c.scan == nil {
		c.scan = bufio.NewScanner(c.in)
	}
	if !c.scan.Scan() {
		return "", errors.New("无法读取输入")
	}
	return strings.TrimSpace(c.scan.Text()), nil
}

func (c *CLI) cmdAbout(args []string) error {
	fmt.Fprintln(c.out, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintln(c.out, "  pw 密码管理器 v"+config.Version)
	fmt.Fprintln(c.out, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintln(c.out, "  加密引擎: Argon2id (64 MiB, iter=3, 4 线程) + AES-256-GCM")
	fmt.Fprintln(c.out, "  数据存储: "+config.AppDir())
	fmt.Fprintln(c.out, "  保险库文件: "+config.VaultFile)
	fmt.Fprintln(c.out, "  备份文件:   "+config.BackupFile)
	fmt.Fprintln(c.out, "  暴力破解防护: 连续输错 5 次锁定 30 秒（持久化）")
	fmt.Fprintln(c.out, "  官网/仓库: https://github.com/29anan29/pwstone")
	fmt.Fprintln(c.out, "  协议/说明: 详见仓库 README.md")
	fmt.Fprintln(c.out, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintln(c.out, "  提示: 主密码丢失后数据无法恢复，请定期 export 备份")
	return nil
}

func (c *CLI) readSecret(prompt string) (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(c.out, prompt)
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(c.out)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	if c.scan == nil {
		c.scan = bufio.NewScanner(c.in)
	}
	if !c.scan.Scan() {
		return "", errors.New("无法读取输入")
	}
	return strings.TrimSpace(c.scan.Text()), nil
}

func (c *CLI) printHelp() {
	fmt.Fprintln(c.out, `━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  pw 密码管理器 (v`+config.Version+`)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  直接运行（无参数）进入交互式 TUI

  子命令:
    init                   首次初始化主密码
    in  <网站> [-u 账号] [-n 备注]   新增/更新（密码隐藏输入）
    out <网站> [-u 账号]    精确查询
    search <关键字>         模糊搜索
    list                   列出全部（密码打码）
    del <网站> [-u 账号]    删除
    export [-o 文件]        导出明文备份
    passwd                 修改主密码
    destroy                自毁（删除所有数据）
    update [check] [--proxy socks5://127.0.0.1:1080]  检查并更新
    about                  查看版本与加密信息
    version                显示版本
    help                   显示帮助

  安全说明:
    • Argon2id 密钥派生 (64MiB, iter=3)
    • AES-256-GCM 逐记录加密（网站/账号/密码全加密）
    • 主密码永不落盘、不出现在命令行参数
    • 连续输错 5 次锁定 30 秒（持久化，跨进程生效）
    • 文件互斥锁，防止多进程同时写坏数据
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`)
}

func maskPassword(pw string) string {
	if pw == "" {
		return ""
	}
	return strings.Repeat("*", len([]rune(pw)))
}

func parseArgs(args []string) (positional []string, flags map[string]string, err error) {
	flags = map[string]string{}
	aliases := map[string]string{
		"u": "user", "user": "user",
		"n": "note", "note": "note",
		"o": "output", "output": "output",
		"x": "proxy", "proxy": "proxy",
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case strings.HasPrefix(a, "--"):
			kv := strings.SplitN(a[2:], "=", 2)
			name, ok := aliases[kv[0]]
			if !ok {
				return nil, nil, fmt.Errorf("未知参数: --%s", kv[0])
			}
			if len(kv) == 2 {
				flags[name] = kv[1]
			} else {
				if i+1 >= len(args) {
					return nil, nil, fmt.Errorf("参数 --%s 缺少值", kv[0])
				}
				i++
				flags[name] = args[i]
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			name, ok := aliases[a[1:]]
			if !ok {
				return nil, nil, fmt.Errorf("未知参数: -%s", a[1:])
			}
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("参数 -%s 缺少值", a[1:])
			}
			i++
			flags[name] = args[i]
		default:
			positional = append(positional, a)
		}
	}
	return positional, flags, nil
}
