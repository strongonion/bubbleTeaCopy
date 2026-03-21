package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bubblecopy/internal/model"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func TestLoadCSVParsesByHeaderName(t *testing.T) {
	path := writeTempCSV(t, strings.Join([]string{
		"group,source,clear_target,target,op",
		"docs,./a.txt,true,./out/file.txt,copy",
	}, "\n"))

	tasks, err := LoadCSV(path)
	if err != nil {
		t.Fatalf("LoadCSV() 返回错误 = %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("期望 1 个任务，实际 %d", len(tasks))
	}
	got := tasks[0]
	if got.Group != "docs" {
		t.Fatalf("期望分组为 docs，实际 %q", got.Group)
	}
	if got.Op != model.OpCopy {
		t.Fatalf("期望操作为 copy，实际 %q", got.Op)
	}
	if !got.ClearTarget {
		t.Fatalf("期望 clear_target=true")
	}
}

func TestLoadCSVMissingHeader(t *testing.T) {
	path := writeTempCSV(t, strings.Join([]string{
		"source,target,op,group",
		"./a.txt,./b.txt,copy,docs",
	}, "\n"))

	_, err := LoadCSV(path)
	if err == nil {
		t.Fatalf("期望缺失表头时报错")
	}
	if !strings.Contains(err.Error(), "missing required header: clear_target") {
		t.Fatalf("错误信息不符合预期: %v", err)
	}
}

func TestLoadCSVInvalidOp(t *testing.T) {
	path := writeTempCSV(t, strings.Join([]string{
		"source,target,op,clear_target,group",
		"./a.txt,./b.txt,cp,true,docs",
	}, "\n"))

	_, err := LoadCSV(path)
	if err == nil {
		t.Fatalf("期望非法 op 报错")
	}
	if !strings.Contains(err.Error(), "invalid op") {
		t.Fatalf("错误信息不符合预期: %v", err)
	}
}

func TestLoadCSVInvalidClearTarget(t *testing.T) {
	path := writeTempCSV(t, strings.Join([]string{
		"source,target,op,clear_target,group",
		"./a.txt,./b.txt,copy,yes,docs",
	}, "\n"))

	_, err := LoadCSV(path)
	if err == nil {
		t.Fatalf("期望非法 clear_target 报错")
	}
	if !strings.Contains(err.Error(), "invalid clear_target") {
		t.Fatalf("错误信息不符合预期: %v", err)
	}
}

func TestLoadCSVDefaultGroup(t *testing.T) {
	path := writeTempCSV(t, strings.Join([]string{
		"source,target,op,clear_target,group",
		"./a.txt,./b.txt,copy,false,",
	}, "\n"))

	tasks, err := LoadCSV(path)
	if err != nil {
		t.Fatalf("LoadCSV() 返回错误 = %v", err)
	}
	if got, want := tasks[0].Group, model.DefaultGroup; got != want {
		t.Fatalf("group = %q, 期望 %q", got, want)
	}
}

func TestLoadCSVGB18030ChinesePath(t *testing.T) {
	content := strings.Join([]string{
		"source,target,op,clear_target,group",
		`C:\源\文件\test.txt,C:\目标\文件\test.txt,copy,true,docs`,
	}, "\n")

	encoded, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte(content))
	if err != nil {
		t.Fatalf("GB18030 编码失败: %v", err)
	}

	path := writeTempCSVBytes(t, encoded)
	tasks, err := LoadCSV(path)
	if err != nil {
		t.Fatalf("LoadCSV() 返回错误 = %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("期望 1 个任务，实际 %d", len(tasks))
	}
	if got, want := tasks[0].Source, `C:\源\文件\test.txt`; got != want {
		t.Fatalf("source = %q, 期望 %q", got, want)
	}
	if got, want := tasks[0].Target, `C:\目标\文件\test.txt`; got != want {
		t.Fatalf("target = %q, 期望 %q", got, want)
	}
}

func TestLoadCSVUTF8BOMHeader(t *testing.T) {
	content := strings.Join([]string{
		"source,target,op,clear_target,group",
		"./a.txt,./b.txt,copy,false,docs",
	}, "\n")
	withBOM := append([]byte{0xEF, 0xBB, 0xBF}, []byte(content)...)

	path := writeTempCSVBytes(t, withBOM)
	tasks, err := LoadCSV(path)
	if err != nil {
		t.Fatalf("LoadCSV() 返回错误 = %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("期望 1 个任务，实际 %d", len(tasks))
	}
}

func writeTempCSV(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.csv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入 csv 失败: %v", err)
	}
	return path
}

func writeTempCSVBytes(t *testing.T, content []byte) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.csv")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("写入 csv 失败: %v", err)
	}
	return path
}
