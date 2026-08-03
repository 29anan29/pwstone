package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"pwstore/internal/config"
)

const defaultAPI = "https://api.github.com/repos/29anan29/pwstone/releases/latest"

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type Release struct {
	Tag    string  `json:"tag_name"`
	Name   string  `json:"name"`
	Notes  string  `json:"body"`
	Assets []Asset `json:"assets"`
}

func apiURL() string {
	if u := os.Getenv("PW_UPDATE_API"); u != "" {
		return u
	}
	return defaultAPI
}

func AssetNameFor(version string) string {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("pw_%s_%s_%s.%s", strings.TrimPrefix(version, "v"), runtime.GOOS, runtime.GOARCH, ext)
}

func IsNewer(current, latestTag string) bool {
	a := strings.TrimPrefix(current, "v")
	b := strings.TrimPrefix(latestTag, "v")
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	ia := make([]int, len(as))
	for i, s := range as {
		n, err := strconv.Atoi(s)
		if err != nil {
			return true
		}
		ia[i] = n
	}
	for i := 0; i < len(ia) && i < len(bs); i++ {
		n, err := strconv.Atoi(bs[i])
		if err != nil {
			return true
		}
		if ia[i] != n {
			return ia[i] < n
		}
	}
	return len(ia) < len(bs)
}

func newClient(proxyURL string) (*http.Client, error) {
	tr := &http.Transport{}
	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("无效代理地址 %q: %w", proxyURL, err)
		}
		switch u.Scheme {
		case "socks5", "socks5h":
			auth := &proxy.Auth{}
			if u.User != nil {
				auth.User = u.User.Username()
				auth.Password, _ = u.User.Password()
			}
			d, err := proxy.SOCKS5("tcp", u.Host, auth, proxy.Direct)
			if err != nil {
				return nil, err
			}
			tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return d.Dial(network, addr)
			}
		case "http", "https":
			tr.Proxy = http.ProxyURL(u)
		default:
			return nil, fmt.Errorf("不支持的代理协议 %q（支持 socks5 / http / https）", u.Scheme)
		}
	}
	return &http.Client{Transport: tr, Timeout: 120 * time.Second}, nil
}

func Latest(proxyURL string) (*Release, error) {
	client, err := newClient(proxyURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", apiURL(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "pw-updater/"+config.Version)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络请求失败: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusForbidden:
		return nil, errors.New("GitHub API 限流，请稍后再试或使用 --proxy")
	case resp.StatusCode == http.StatusNotFound:
		return nil, errors.New("未找到任何发布版本")
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var rel Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func Download(proxyURL, version, assetName string) (string, error) {
	client, err := newClient(proxyURL)
	if err != nil {
		return "", err
	}
	rel, err := Latest(proxyURL)
	if err != nil {
		return "", err
	}
	var asset *Asset
	var sums *Asset
	for i := range rel.Assets {
		switch rel.Assets[i].Name {
		case assetName:
			asset = &rel.Assets[i]
		case "SHA256SUMS":
			sums = &rel.Assets[i]
		}
	}
	if asset == nil {
		names := make([]string, 0, len(rel.Assets))
		for _, a := range rel.Assets {
			names = append(names, a.Name)
		}
		return "", fmt.Errorf("未找到当前平台安装包 %s（可用: %s）", assetName, strings.Join(names, ", "))
	}
	data, err := fetch(client, asset.URL)
	if err != nil {
		return "", err
	}
	if sums != nil {
		sumData, err := fetch(client, sums.URL)
		if err == nil {
			if err := verifySHA256(data, sumData, assetName); err != nil {
				return "", err
			}
		}
	}
	return extractBinary(data, strings.HasSuffix(assetName, ".zip"))
}

func fetch(client *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "pw-updater/"+config.Version)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 128<<20))
}

func verifySHA256(data, sumsData []byte, assetName string) error {
	for _, line := range strings.Split(string(sumsData), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != assetName {
			continue
		}
		sum, err := hex.DecodeString(fields[0])
		if err != nil {
			return err
		}
		got := sha256.Sum256(data)
		if !bytes.Equal(sum, got[:]) {
			return errors.New("SHA256 校验失败，安装包可能被篡改，已中止")
		}
		return nil
	}
	return nil
}

func extractBinary(data []byte, isZip bool) (string, error) {
	dir, err := os.MkdirTemp("", "pw-update-*")
	if err != nil {
		return "", err
	}
	var bin []byte
	if isZip {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return "", err
		}
		for _, f := range zr.File {
			if base := filepath.Base(f.Name); base == "pw.exe" || base == "pw" {
				rc, err := f.Open()
				if err != nil {
					return "", err
				}
				bin, err = io.ReadAll(io.LimitReader(rc, 128<<20))
				rc.Close()
				if err != nil {
					return "", err
				}
				break
			}
		}
	} else {
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return "", err
		}
		tr := tar.NewReader(gz)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", err
			}
			if base := filepath.Base(hdr.Name); base == "pw" || base == "pw.exe" {
				bin, err = io.ReadAll(io.LimitReader(tr, 128<<20))
				if err != nil {
					return "", err
				}
				break
			}
		}
	}
	if bin == nil {
		return "", errors.New("安装包内未找到 pw 可执行文件")
	}
	dst := filepath.Join(dir, "pw")
	if runtime.GOOS == "windows" {
		dst += ".exe"
	}
	if err := os.WriteFile(dst, bin, 0o755); err != nil {
		return "", err
	}
	return dst, nil
}

func Replace(newPath string) error {
	cur, err := os.Executable()
	if err != nil {
		return err
	}
	return replaceExec(cur, newPath)
}
