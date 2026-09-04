package model

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// ClassifyArtifactName 根据文件名和视频原名判断输出文件的类型，返回
// subtitle / subtitled_video / repaired_video / upscaled_video，未匹配返回 unknown。
// 分类规则从 logic 层迁移而来，供输出文件列表与任务状态产物探测共用。
func ClassifyArtifactName(fileName, videoBaseName string) string {
	name := strings.ToLower(fileName)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	// 字幕文件：<video>.srt
	if ext == ".srt" && base == strings.ToLower(videoBaseName) {
		return "subtitle"
	}

	// 清晰度修复输出视频：含 _upscaled_ 特征，优先级高于 repaired 避免链式文件误分类
	if IsVideoFile(fileName) && (strings.Contains(base, "_upscaled")) {
		return "upscaled_video"
	}

	// 字幕合成视频：<video>_subtitled.<ext> 或 <video>_subtitled_<nonce>.<ext>
	if IsVideoFile(fileName) && strings.Contains(base, "_subtitled") {
		return "subtitled_video"
	}

	// 去马赛克输出视频：含 repaired / denoised / restored / enhanced / deblurred / deblur 等特征
	if IsVideoFile(fileName) {
		if containsRepairedKeyword(base) {
			return "repaired_video"
		}
	}

	return "unknown"
}

// containsRepairedKeyword 判断去扩展名的小写文件名是否含去马赛克产物标记。
// 外部去马赛克程序的产物命名不受本项目控制，只能按关键词启发式识别。
func containsRepairedKeyword(nameNoExt string) bool {
	return strings.Contains(nameNoExt, "repaired") ||
		strings.Contains(nameNoExt, "denoised") ||
		strings.Contains(nameNoExt, "restored") ||
		strings.Contains(nameNoExt, "enhanced") ||
		strings.Contains(nameNoExt, "deblurred") ||
		strings.Contains(nameNoExt, "deblur") ||
		strings.Contains(nameNoExt, "_fixed_") ||
		strings.HasPrefix(nameNoExt, "repaired_") ||
		strings.HasPrefix(nameNoExt, "fixed_")
}

// VideoArtifactStatuses 探测视频输出目录中各任务类型的产物文件是否存在，
// 返回 taskType -> 是否存在 的映射。用于任务记录缺失时（如记录被删除但产物保留）
// 的状态兜底判定。已知限制：覆盖模式的产物即处理源文件本身，无法与本底文件区分，
// 该场景探测不到。
func VideoArtifactStatuses(v *Video) map[string]bool {
	result := map[string]bool{
		TaskTypeSubtitle:     false,
		TaskTypeSubtitleBurn: false,
		TaskTypeDeblur:       false,
		TaskTypeUpscale:      false,
	}
	if v == nil || v.Path == "" {
		return result
	}

	outputDir := VideoOutputDir(context.Background(), v)
	baseLower := strings.ToLower(VideoBaseName(v))

	// 字幕产物：<base>.srt，确定性命名
	if _, err := os.Stat(filepath.Join(outputDir, VideoBaseName(v)+".srt")); err == nil {
		result[TaskTypeSubtitle] = true
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return result
	}
	for _, e := range entries {
		if !IsVideoFile(e.Name()) {
			continue
		}
		// 跳过与本底同名的文件：无法区分"覆盖模式的产物"和"未处理的拷贝"
		if e.Name() == v.Name {
			continue
		}
		nameLower := strings.ToLower(e.Name())
		nameNoExt := strings.TrimSuffix(nameLower, filepath.Ext(nameLower))
		switch {
		case strings.HasPrefix(nameNoExt, baseLower+"_upscaled"):
			result[TaskTypeUpscale] = true
		case strings.HasPrefix(nameNoExt, baseLower+"_subtitled"):
			result[TaskTypeSubtitleBurn] = true
		case containsRepairedKeyword(nameNoExt):
			result[TaskTypeDeblur] = true
		}
	}
	return result
}
