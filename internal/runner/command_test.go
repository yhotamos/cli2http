package runner

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestSplitCommandLine(t *testing.T) {
	for _, test := range []struct {
		input string
		want  []string
	}{
		{`tool -resize 800x600`, []string{"tool", "-resize", "800x600"}},
		{`"with space/tool" "" "a b" "a\"b"`, []string{"with space/tool", "", "a b", `a"b`}},
		{`tool $HOME $(echo x) | > && #comment *`, []string{"tool", "$HOME", "$(echo", "x)", "|", ">", "&&", "#comment", "*"}},
	} {
		t.Run(test.input, func(t *testing.T) {
			got, err := splitCommandLine(test.input)
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("args=%q error=%v want=%q", got, err, test.want)
			}
		})
	}
	if runtime.GOOS == "windows" {
		for _, test := range []struct {
			input string
			want  []string
		}{
			{`C:\images\tool.exe C:\images\input.jpg`, []string{`C:\images\tool.exe`, `C:\images\input.jpg`}},
			{`"C:\ima mges\tool.exe"`, []string{`C:\ima mges\tool.exe`}},
			{`"\\server\share name\tool.exe" ""`, []string{`\\server\share name\tool.exe`, ""}},
			{`tool C:\images\ "C:\images\\"`, []string{"tool", `C:\images\`, `C:\images\`}},
			{`tool "unfinished`, []string{"tool", "unfinished"}},
			{`tool 'a b'`, []string{"tool", "'a", "b'"}},
		} {
			got, err := splitCommandLine(test.input)
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("input=%q args=%q error=%v want=%q", test.input, got, err, test.want)
			}
		}
	} else {
		got, err := splitCommandLine(`tool 'a b' 'C:\images\' '\\server\share'`)
		if err != nil || !reflect.DeepEqual(got, []string{"tool", "a b", `C:\images\`, `\\server\share`}) {
			t.Fatalf("args=%q error=%v", got, err)
		}
		for _, input := range []string{`tool "unfinished`, `tool 'unfinished`, `tool trailing\`} {
			if _, err := splitCommandLine(input); err == nil {
				t.Fatalf("expected parsing error: %q", input)
			}
		}
	}
}

func TestResolveCommand(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "with space")
	if err := os.Mkdir(directory, 0755); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(directory, "tool.exe")
	if err := os.WriteFile(executable, []byte("fixture"), 0755); err != nil {
		t.Fatal(err)
	}
	target, err := ResolveCommand(`"` + executable + `" "" "a b" --flag`)
	if err != nil {
		t.Fatal(err)
	}
	if target.Executable != executable || !reflect.DeepEqual(target.FixedArgs, []string{"", "a b", "--flag"}) {
		t.Fatalf("target=%+v", target)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"", "  ", "\r\n", `"" argument`, "tool a\x00b", "sh -c anything", "cmd /C anything", "cli2http", `"` + self + `"`} {
		if _, err := ResolveCommand(input); err == nil {
			t.Fatalf("expected rejection: %q", input)
		}
	}
}
