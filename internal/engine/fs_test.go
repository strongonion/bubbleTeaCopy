package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDirMergeOverwritesFiles(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")

	mustWriteFileTest(t, filepath.Join(src, "same.txt"), "new")
	mustWriteFileTest(t, filepath.Join(src, "only-src.txt"), "src")

	mustWriteFileTest(t, filepath.Join(dst, "same.txt"), "old")
	mustWriteFileTest(t, filepath.Join(dst, "only-dst.txt"), "dst")

	if err := copyPath(src, dst, true); err != nil {
		t.Fatalf("copyPath() 返回错误 = %v", err)
	}

	if got := readFile(t, filepath.Join(dst, "same.txt")); got != "new" {
		t.Fatalf("same.txt = %q, 期望 %q", got, "new")
	}
	if got := readFile(t, filepath.Join(dst, "only-src.txt")); got != "src" {
		t.Fatalf("only-src.txt = %q, 期望 %q", got, "src")
	}
	if got := readFile(t, filepath.Join(dst, "only-dst.txt")); got != "dst" {
		t.Fatalf("only-dst.txt = %q, 期望 %q", got, "dst")
	}
}

func TestClearTargetPreservesDirectory(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "out")

	mustWriteFileTest(t, filepath.Join(target, "a.txt"), "a")
	mustWriteFileTest(t, filepath.Join(target, "nested", "b.txt"), "b")

	if err := clearTarget(target); err != nil {
		t.Fatalf("clearTarget() 返回错误 = %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("获取 target 状态失败: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("target 应保持为目录")
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatalf("读取 target 目录失败: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("期望 target 目录为空，实际 %d 个条目", len(entries))
	}
}

func TestMoveFileOverwritesExistingFile(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src.txt")
	dst := filepath.Join(root, "dst.txt")

	mustWriteFileTest(t, src, "new")
	mustWriteFileTest(t, dst, "old")

	if err := movePath(src, dst, false); err != nil {
		t.Fatalf("movePath() 返回错误 = %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source 应被删除，stat err = %v", err)
	}
	if got := readFile(t, dst); got != "new" {
		t.Fatalf("dst 内容 = %q, 期望 %q", got, "new")
	}
}

func TestMoveDirIntoExistingDirMergesAndRemovesSource(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")

	mustWriteFileTest(t, filepath.Join(src, "from-src.txt"), "src")
	mustWriteFileTest(t, filepath.Join(dst, "from-dst.txt"), "dst")

	if err := movePath(src, dst, true); err != nil {
		t.Fatalf("movePath() 返回错误 = %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source 目录应被删除，stat err = %v", err)
	}
	if got := readFile(t, filepath.Join(dst, "from-src.txt")); got != "src" {
		t.Fatalf("from-src.txt = %q, 期望 %q", got, "src")
	}
	if got := readFile(t, filepath.Join(dst, "from-dst.txt")); got != "dst" {
		t.Fatalf("from-dst.txt = %q, 期望 %q", got, "dst")
	}
}

func mustWriteFileTest(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建父目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	return string(data)
}
