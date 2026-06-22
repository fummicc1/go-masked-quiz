package source

import "strings"

import "testing"

func TestNormalize_StripsHTMLComments(t *testing.T) {
	got := Normalize("before<!-- hidden\nmultiline -->after")
	if strings.Contains(got, "hidden") {
		t.Errorf("HTML comment not stripped: %q", got)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Errorf("surrounding text lost: %q", got)
	}
}

func TestNormalize_StripsTaskLists(t *testing.T) {
	in := "## Checklist\n- [ ] open item\n- [x] done item\nkeep this line\n"
	got := Normalize(in)
	if strings.Contains(got, "open item") || strings.Contains(got, "done item") {
		t.Errorf("task list not stripped: %q", got)
	}
	if !strings.Contains(got, "keep this line") {
		t.Errorf("non-task line lost: %q", got)
	}
}

func TestNormalize_CollapsesBlankRuns(t *testing.T) {
	got := Normalize("a\n\n\n\n\nb")
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("blank run not collapsed: %q", got)
	}
}

func TestNormalize_NormalisesCRLF(t *testing.T) {
	got := Normalize("a\r\nb\r\n")
	if strings.Contains(got, "\r") {
		t.Errorf("CR not stripped: %q", got)
	}
}

func TestNormalize_PreservesCodeFences(t *testing.T) {
	in := "Use `x` here.\n\n```go\nfunc f() {}\n```\n"
	got := Normalize(in)
	if !strings.Contains(got, "`x`") || !strings.Contains(got, "```go") {
		t.Errorf("code markup lost: %q", got)
	}
}
