package telegram

import (
	"net/url"
	"strings"
)

// telegramHosts 视为 Telegram 链接的域名
var telegramHosts = map[string]struct{}{
	"t.me":         {},
	"telegram.me":  {},
	"telegram.dog": {},
	"tx.me":        {},
}

// IsTelegramURL 判断是否为 Telegram 链接（含官方 tg:// 协议）。
// 具体的消息链接解析交给 tdl 的 tutil.ParseMessageLink。
func IsTelegramURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}

	switch strings.ToLower(u.Scheme) {
	case "tg":
		return true
	case "http", "https":
	default:
		return false
	}

	_, ok := telegramHosts[strings.ToLower(u.Hostname())]
	return ok
}
