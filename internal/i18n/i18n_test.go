package i18n

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestCatalogsMatch(t *testing.T) {
	english, chinese := catalogs["en"], catalogs["zh-Hans"]
	if len(english) != len(chinese) {
		t.Fatal("catalog keys do not match")
	}
	formats := regexp.MustCompile(`%[sdw]`)
	for key, value := range english {
		translation, ok := chinese[key]
		if !ok || value == "" || translation == "" {
			t.Errorf("missing translation: %q", key)
		}
		if strings.Join(formats.FindAllString(value, -1), ",") != strings.Join(formats.FindAllString(translation, -1), ",") {
			t.Errorf("format arguments differ: %q", key)
		}
	}
}

func TestLocaleResolutionAndSystemDefaults(t *testing.T) {
	for _, key := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		t.Setenv(key, "")
	}
	if SystemLanguage() != "en" {
		t.Fatal("missing locale must fall back to English")
	}
	t.Setenv("LANG", "zh_CN.UTF-8")
	if SystemLanguage() != "zh-Hans" {
		t.Fatal("Chinese system locale not recognized")
	}
	t.Setenv("LC_ALL", "en_US.UTF-8")
	if SystemLanguage() != "en" {
		t.Fatal("LC_ALL must take precedence over LANG")
	}
	t.Setenv("LANGUAGE", "zh-Hans:en")
	if SystemLanguage() != "zh-Hans" {
		t.Fatal("preferred language list not recognized")
	}
	if Resolve("fr-FR") != "en" || Resolve("zh-SG") != "zh-Hans" {
		t.Fatal("locale fallback failed")
	}
}

func TestDeferredMessagesAndErrors(t *testing.T) {
	message := Text("Found %d candidate models. Access and quota are checked when used.", 18)
	if !strings.Contains(message.String("zh-Hans"), "18 个") || !strings.Contains(message.String("en"), "18 candidate") {
		t.Fatal("message arguments were not translated")
	}
	err := Text("Enter a valid API key.")
	nested := Text("Could not save the language preference: %s", FromError(err))
	if strings.Contains(nested.String("zh-Hans"), "Enter a valid") {
		t.Fatal("nested error remained in English")
	}
	if FromError(errors.New("external error")).String("zh-Hans") != "external error" {
		t.Fatal("external error lost")
	}
	if T("unsupported", "unknown text") != "unknown text" {
		t.Fatal("missing-key fallback failed")
	}
}

// Catch newly added user-facing strings without catalog entries.
func TestSourceMessagesHaveCatalogEntries(t *testing.T) {
	for _, directory := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join("..", "..", directory), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (selector.Sel.Name != "tr" && selector.Sel.Name != "Text") {
					return true
				}
				literal, ok := call.Args[0].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return true
				}
				key, _ := strconv.Unquote(literal.Value)
				if _, ok := catalogs["en"][key]; !ok {
					t.Errorf("%s: missing catalog entry %q", path, key)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
