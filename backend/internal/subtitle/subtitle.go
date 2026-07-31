package subtitle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// 支持的导出格式
const (
	FormatSRT = "srt"
	FormatVTT = "vtt"
	FormatASS = "ass"
)

// Segment 表示一条字幕片段，时间单位为秒
// Start 为开始时间，End 为结束时间，Text 为字幕文本
type Segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// ValidFormat 判断给定的格式是否为支持的导出格式
func ValidFormat(format string) bool {
	switch format {
	case FormatSRT, FormatVTT, FormatASS:
		return true
	default:
		return false
	}
}

// ParseSegments 从 JSON 字符串中解析字幕片段列表
func ParseSegments(data string) ([]Segment, error) {
	if strings.TrimSpace(data) == "" {
		return []Segment{}, nil
	}

	segments := make([]Segment, 0)
	if err := json.Unmarshal([]byte(data), &segments); err != nil {
		return nil, fmt.Errorf("解析字幕片段失败: %w", err)
	}
	return segments, nil
}

// ToSRT 将字幕片段列表转换为 SRT 格式字符串
func ToSRT(segments []Segment) string {
	if len(segments) == 0 {
		return ""
	}

	var b strings.Builder
	for i, seg := range segments {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("%d\n", i+1))
		b.WriteString(fmt.Sprintf("%s --> %s\n", formatSRTTime(seg.Start), formatSRTTime(seg.End)))
		b.WriteString(seg.Text)
		b.WriteString("\n")
	}
	return b.String()
}

// ToVTT 将字幕片段列表转换为 WebVTT 格式字符串
func ToVTT(segments []Segment) string {
	if len(segments) == 0 {
		return "WEBVTT\n"
	}

	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for i, seg := range segments {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("%s --> %s\n", formatVTTTime(seg.Start), formatVTTTime(seg.End)))
		b.WriteString(seg.Text)
		b.WriteString("\n")
	}
	return b.String()
}

// ToASS 将字幕片段列表转换为 ASS 格式字符串（仅包含基础样式）
func ToASS(segments []Segment) string {
	var b strings.Builder
	b.WriteString("[Script Info]\n")
	b.WriteString("Title: Exported Subtitle\n")
	b.WriteString("ScriptType: v4.00+\n")
	b.WriteString("Collisions: Normal\n")
	b.WriteString("PlayDepth: 0\n")
	b.WriteString("\n")

	b.WriteString("[V4+ Styles]\n")
	b.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	b.WriteString("Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\n")
	b.WriteString("\n")

	b.WriteString("[Events]\n")
	b.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")

	for _, seg := range segments {
		text := normalizeASSText(seg.Text)
		b.WriteString(fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n", formatASSTime(seg.Start), formatASSTime(seg.End), text))
	}
	return b.String()
}

// formatSRTTime 将秒数转换为 SRT 时间格式 HH:MM:SS,mmm
func formatSRTTime(seconds float64) string {
	totalMs := int64(math.Round(seconds * 1000))
	ms := totalMs % 1000
	totalSec := totalMs / 1000
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

// formatVTTTime 将秒数转换为 WebVTT 时间格式
// 时长不足 1 小时时返回 MM:SS.mmm，否则返回 HH:MM:SS.mmm
func formatVTTTime(seconds float64) string {
	totalMs := int64(math.Round(seconds * 1000))
	ms := totalMs % 1000
	totalSec := totalMs / 1000
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
	}
	return fmt.Sprintf("%02d:%02d.%03d", m, s, ms)
}

// formatASSTime 将秒数转换为 ASS 时间格式 H:MM:SS.cc
func formatASSTime(seconds float64) string {
	totalCs := int64(math.Round(seconds * 100))
	cs := totalCs % 100
	totalSec := totalCs / 100
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, cs)
}

// normalizeASSText 将 ASS 字幕文本中的换行符转换为 ASS 支持的 \N
func normalizeASSText(text string) string {
	replacer := strings.NewReplacer("\r\n", "\\N", "\n", "\\N", "\r", "\\N")
	return replacer.Replace(text)
}

// srtTimeRegex 匹配 SRT 时间轴中的时间字符串
var srtTimeRegex = regexp.MustCompile(`(\d{2}):(\d{2}):(\d{2})[,\.](\d{3})\s*-->\s*(\d{2}):(\d{2}):(\d{2})[,\.](\d{3})`)

// ParseSRT 解析 SRT 格式字符串，返回字幕片段列表
func ParseSRT(content string) []Segment {
	var segments []Segment
	var current *Segment
	var hasTime bool

	scanner := bufio.NewScanner(strings.NewReader(content))
	// SRT 行可能较长，提高缓冲区上限
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			if current != nil && hasTime {
				segments = append(segments, *current)
			}
			current = nil
			hasTime = false
			continue
		}

		// 尝试匹配时间轴行
		// FindStringSubmatch 返回 [完整匹配, 组1, 组2, ..., 组8]，共 9 个元素
		if matches := srtTimeRegex.FindStringSubmatch(line); len(matches) == 9 {
			if current == nil {
				current = &Segment{}
			}
			current.Start = parseSRTTime(matches[1], matches[2], matches[3], matches[4])
			current.End = parseSRTTime(matches[5], matches[6], matches[7], matches[8])
			hasTime = true
			continue
		}

		// 尝试解析序号行（纯数字）
		if current == nil {
			if _, err := strconv.Atoi(line); err == nil {
				current = &Segment{}
				continue
			}
		}

		// 文本内容
		if current != nil && hasTime {
			if current.Text != "" {
				current.Text += "\n"
			}
			current.Text += line
		}
	}

	// 处理最后一个条目
	if current != nil && hasTime {
		segments = append(segments, *current)
	}

	return segments
}

// parseSRTTime 将拆分的时、分、秒、毫秒字符串转换为秒数（float64）
func parseSRTTime(h, m, s, ms string) float64 {
	hours, _ := strconv.Atoi(h)
	minutes, _ := strconv.Atoi(m)
	seconds, _ := strconv.Atoi(s)
	millis, _ := strconv.Atoi(ms)
	return float64(hours)*3600 + float64(minutes)*60 + float64(seconds) + float64(millis)/1000
}
