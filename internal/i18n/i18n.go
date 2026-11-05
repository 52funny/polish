// Package i18n provides embedded catalogs and deferred, locale-independent messages.
package i18n

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

//go:embed locales/*.json
var files embed.FS

var catalogs = loadCatalogs()

func loadCatalogs() map[string]map[string]string {
	result := make(map[string]map[string]string)
	for _, tag := range []string{"en", "zh-Hans"} {
		data, err := files.ReadFile("locales/" + tag + ".json")
		if err != nil {
			panic(err)
		}
		var catalog map[string]string
		if err := json.Unmarshal(data, &catalog); err != nil {
			panic(err)
		}
		result[tag] = catalog
	}
	return result
}

// Resolve maps locale variants to a supported language, with English as fallback.
func Resolve(tag string) string {
	tag = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tag), "_", "-"))
	if tag == "zh" || strings.HasPrefix(tag, "zh-") {
		return "zh-Hans"
	}
	return "en"
}

// SystemLanguage follows the Linux preference order also used by MyGo.
func SystemLanguage() string {
	for _, key := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := os.Getenv(key); value != "" {
			return Resolve(strings.Split(value, ":")[0])
		}
	}
	return "en"
}

// Message keeps arguments separate so existing status and error text can be
// translated again when the interface language changes. English source text is
// the catalog key and the fallback for missing translations.
type Message struct {
	Source string
	Args   []any
}

func Text(source string, args ...any) Message { return Message{source, args} }

func (m Message) String(locale string) string {
	format := catalogs[Resolve(locale)][m.Source]
	if format == "" {
		format = m.Source
	}
	args := append([]any(nil), m.Args...)
	for i, arg := range args {
		if message, ok := arg.(Message); ok {
			args[i] = message.String(locale)
		}
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// Error returns English text for logs, command-line output, and external callers.
func (m Message) Error() string { return m.String("en") }

func FromError(err error) Message {
	if err == nil {
		return Message{}
	}
	var message Message
	if errors.As(err, &message) {
		return message
	}
	return Text("%s", err.Error())
}

func T(locale, source string, args ...any) string { return Text(source, args...).String(locale) }
