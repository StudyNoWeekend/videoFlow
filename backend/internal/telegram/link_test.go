package telegram

import "testing"

func TestIsTelegramURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"标准消息链接", "https://t.me/telegram/193", true},
		{"私有频道链接", "https://t.me/c/1697797156/151", true},
		{"telegram.me 域名", "https://telegram.me/telegram/193", true},
		{"telegram.dog 域名", "https://telegram.dog/telegram/193", true},
		{"tx.me 域名", "https://tx.me/telegram/193", true},
		{"大写域名", "https://T.ME/telegram/193", true},
		{"tg 协议", "tg://resolve?domain=telegram&post=193", true},
		{"带查询参数", "https://t.me/opencfdchannel/4434?comment=360409", true},
		{"其他站点", "https://www.youtube.com/watch?v=abc", false},
		{"相似域名", "https://fake-t.me/telegram/193", false},
		{"空字符串", "", false},
		{"非链接文本", "not a url", false},
		{"缺少协议", "t.me/telegram/193", false},
		{"ftp 协议", "ftp://t.me/telegram/193", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTelegramURL(tt.url); got != tt.want {
				t.Errorf("IsTelegramURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
