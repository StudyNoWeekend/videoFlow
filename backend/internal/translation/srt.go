package translation

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Subtitle 表示一个字幕条目
type Subtitle struct {
	Index     int
	StartTime time.Duration
	EndTime   time.Duration
	Text      string
}

// ParseSRT 解析 SRT 格式字幕内容
func ParseSRT(content string) []Subtitle {
	var subtitles []Subtitle
	var currentSubtitle *Subtitle

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			// 空行表示字幕条目结束
			if currentSubtitle != nil {
				subtitles = append(subtitles, *currentSubtitle)
				currentSubtitle = nil
			}
			continue
		}

		// 尝试解析序号
		if currentSubtitle == nil {
			index, err := strconv.Atoi(line)
			if err == nil {
				currentSubtitle = &Subtitle{Index: index}
				continue
			}
		}

		// 尝试解析时间轴
		if currentSubtitle != nil && strings.Contains(line, "-->") {
			startTime, endTime := parseTimeLine(line)
			currentSubtitle.StartTime = startTime
			currentSubtitle.EndTime = endTime
			continue
		}

		// 解析文本内容
		if currentSubtitle != nil && currentSubtitle.StartTime > 0 {
			if currentSubtitle.Text != "" {
				currentSubtitle.Text += "\n"
			}
			currentSubtitle.Text += line
		}
	}

	// 处理最后一个字幕条目
	if currentSubtitle != nil {
		subtitles = append(subtitles, *currentSubtitle)
	}

	return subtitles
}

// GenerateSRT 生成 SRT 格式字幕内容
func GenerateSRT(subtitles []Subtitle) string {
	var builder strings.Builder

	for i, sub := range subtitles {
		if i > 0 {
			builder.WriteString("\n")
		}

		builder.WriteString(fmt.Sprintf("%d\n", sub.Index))
		builder.WriteString(fmt.Sprintf("%s --> %s\n",
			formatTime(sub.StartTime),
			formatTime(sub.EndTime)))
		builder.WriteString(sub.Text)
		builder.WriteString("\n")
	}

	return builder.String()
}

// parseTimeLine 解析时间轴行，格式：00:00:01,000 --> 00:00:04,000
func parseTimeLine(line string) (time.Duration, time.Duration) {
	// 使用正则表达式匹配时间格式
	re := regexp.MustCompile(`(\d{2}:\d{2}:\d{2}[,\.]\d{3})\s*-->\s*(\d{2}:\d{2}:\d{2}[,\.]\d{3})`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 3 {
		return 0, 0
	}

	startTime := parseTime(matches[1])
	endTime := parseTime(matches[2])

	return startTime, endTime
}

// parseTime 解析单个时间字符串，格式：00:00:01,000 或 00:00:01.000
func parseTime(timeStr string) time.Duration {
	// 将逗号替换为点号，统一格式
	timeStr = strings.Replace(timeStr, ",", ".", 1)

	parts := strings.Split(timeStr, ":")
	if len(parts) != 3 {
		return 0
	}

	hours, _ := strconv.Atoi(parts[0])
	minutes, _ := strconv.Atoi(parts[1])

	// 处理秒和毫秒部分
	secondParts := strings.Split(parts[2], ".")
	seconds, _ := strconv.Atoi(secondParts[0])
	var milliseconds int
	if len(secondParts) > 1 {
		milliseconds, _ = strconv.Atoi(secondParts[1])
	}

	return time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(milliseconds)*time.Millisecond
}

// formatTime 将时间格式化为 SRT 时间格式：00:00:01,000
func formatTime(duration time.Duration) string {
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60
	milliseconds := int(duration.Milliseconds()) % 1000

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, milliseconds)
}
