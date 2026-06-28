// author: fooyii & hehuan
// date: 2026-06-20
// description: config 包的单元测试，包含 LoadedPath 回写和 PID 文件路径获取测试

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetPIDFilePath(t *testing.T) {
	// 验证获取 PID 文件路径功能
	pidPath := GetPIDFilePath()
	if pidPath == "" {
		t.Fatal("expected non-empty PID file path")
	}

	// 确认路径包含 "chat2responses" 关键字
	base := filepath.Base(pidPath)
	if !strings.HasSuffix(base, ".pid") {
		t.Errorf("expected PID file to end with .pid, got: %s", base)
	}
}

func TestConfigLoadedPath(t *testing.T) {
	// 创建临时测试配置文件
	tmpDir, err := os.MkdirTemp("", "c2r-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configContent := `{
		"upstream": {
			"base_url": "https://api.openai.com/v1",
			"api_key": "sk-dummy"
		},
		"server": {
			"host": "127.0.0.1",
			"port": 57321
		},
		"model": {
			"default_model": "gpt-4o"
		}
	}`

	cfgFile := filepath.Join(tmpDir, "custom_config.json")
	if err := os.WriteFile(cfgFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 加载配置
	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatal(err)
	}

	// 此时 LoadedPath 应该等于 cfgFile 的绝对路径
	absExpected, _ := filepath.Abs(cfgFile)
	if cfg.LoadedPath != absExpected {
		t.Errorf("expected LoadedPath to be %s, got: %s", absExpected, cfg.LoadedPath)
	}
}
