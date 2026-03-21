package tui

import (
	"testing"
	"unicode/utf8"

	"bubblecopy/internal/model"
)

func TestBuildGroupsPreservesFirstAppearanceOrder(t *testing.T) {
	tasks := []model.Task{
		{Index: 0, Group: "group-b"},
		{Index: 1, Group: "group-a"},
		{Index: 2, Group: "group-b"},
		{Index: 3, Group: ""},
	}

	groups := buildGroups(tasks, map[int]bool{})
	if len(groups) != 3 {
		t.Fatalf("groups 长度 = %d, 期望 3", len(groups))
	}
	if groups[0].Name != "group-b" {
		t.Fatalf("groups[0] = %q, 期望 group-b", groups[0].Name)
	}
	if groups[1].Name != "group-a" {
		t.Fatalf("groups[1] = %q, 期望 group-a", groups[1].Name)
	}
	if groups[2].Name != model.DefaultGroup {
		t.Fatalf("groups[2] = %q, 期望 %q", groups[2].Name, model.DefaultGroup)
	}
}

func TestGroupSpaceTogglesWholeGroup(t *testing.T) {
	tasks := []model.Task{
		{Index: 0, Group: "docs"},
		{Index: 1, Group: "docs"},
		{Index: 2, Group: "media"},
	}

	ui := newModel(tasks, 2)
	ui.focus = focusGroups
	ui.groupCursor = 0
	ui.toggleSelection()

	if !ui.selected[0] || !ui.selected[1] {
		t.Fatalf("期望 docs 分组下全部任务被选中")
	}
	if ui.selected[2] {
		t.Fatalf("期望 media 分组任务保持不变")
	}

	ui.toggleSelection()
	if ui.selected[0] || ui.selected[1] {
		t.Fatalf("期望 docs 分组被取消选择")
	}
}

func TestTruncateKeepsUTF8Boundary(t *testing.T) {
	input := "路径: C:\\源\\文件\\非常长的文件名.txt"
	got := truncate(input, 10)
	if !utf8.ValidString(got) {
		t.Fatalf("truncate 结果不是合法 UTF-8: %q", got)
	}
	if got != "路径: C:\\..." {
		t.Fatalf("truncate 结果 = %q, 期望 %q", got, "路径: C:\\...")
	}
}
