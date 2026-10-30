package desktop

import (
	"strings"
	"testing"
)

func TestDesktopFilesEscapeExecutableAndPreserveCallbackField(t *testing.T) {
	path := `/home/user/game 100%/launch$tool`
	files := Files(path)
	if len(files) != 2 {
		t.Fatalf("expected two desktop entries, got %d", len(files))
	}
	for _, id := range []string{DesktopID, CallbackID} {
		content := string(files[id])
		if strings.Contains(content, "@EXEC@") || !strings.Contains(content, `"/home/user/game 100%%/launch\\$tool"`) {
			t.Errorf("%s does not contain the correctly quoted executable: %q", id, content)
		}
	}
	if !strings.Contains(string(files[CallbackID]), " callback %u\n") {
		t.Fatal("callback URI field code changed")
	}
}

func TestManagedBlockRemoval(t *testing.T) {
	block := ManagedBlock("/a directory/rules.lua")
	for _, tc := range []struct {
		name, source, want string
		wantError          bool
	}{
		{name: "unchanged", source: "other config\n", want: "other config\n"},
		{name: "remove", source: "before\n" + block + "after\n", want: "before\nafter\n"},
		{name: "missing end", source: BlockBegin + "\n", wantError: true},
		{name: "missing begin", source: BlockEnd + "\n", wantError: true},
		{name: "duplicate", source: block + block, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RemoveBlock(tc.source)
			if (err != nil) != tc.wantError || (!tc.wantError && got != tc.want) {
				t.Fatalf("got %q, %v; want %q, error=%t", got, err, tc.want, tc.wantError)
			}
		})
	}
}

func TestQuoteLuaEscapesControlCharacters(t *testing.T) {
	input := "path\\with\"quote\nnext\rline"
	want := `"path\\with\"quote\nnext\rline"`
	if got := QuoteLua(input); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRemoveMIMEReferencePreservesOtherAssociations(t *testing.T) {
	source := "[Default Applications]\ntext/html=browser.desktop;\n" + CallbackMIME + "=" + CallbackID + ";previous.desktop;\n" +
		"[Added Associations]\n" + CallbackMIME + "=" + CallbackID + ";\n"
	want := "[Default Applications]\ntext/html=browser.desktop;\n" + CallbackMIME + "=previous.desktop;\n[Added Associations]\n"
	if got := RemoveMIMEReference(source); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
