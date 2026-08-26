package component

import (
	"context"

	"video-captions/internal/model"
)

// defaultDetector 包级默认检测器，供任务创建时的组件就绪校验复用
var defaultDetector = NewDetector()

// TaskTypeDependencies 返回指定任务类型所依赖的组件列表。
// 与前端 TASK_REQUIRED_COMPONENTS 保持一致，新增任务类型时需同步维护两边。
func TaskTypeDependencies(taskType string) []ComponentType {
	switch taskType {
	case model.TaskTypeSubtitle:
		return []ComponentType{ComponentFFmpeg, ComponentWhisperASR}
	case model.TaskTypeSubtitleBurn:
		return []ComponentType{ComponentFFmpeg}
	case model.TaskTypeDeblur, model.TaskTypeRepair:
		return []ComponentType{ComponentDocker, ComponentLada}
	case model.TaskTypeUpscale:
		return []ComponentType{ComponentDocker, ComponentVideo2X}
	case model.TaskTypeDownload:
		return []ComponentType{ComponentYtDlp}
	default:
		return nil
	}
}

// CheckTaskDependencies 实时检测指定任务类型依赖的组件是否全部就绪，
// 返回未就绪（missing/installing/error）的组件信息；全部就绪时返回空切片。
// 注意：docker 类组件的检测结果会反映 daemon 不可达的情况，
// whisper_asr 的检测结果会反映 ASR URL 未配置或服务不可达的情况。
func CheckTaskDependencies(ctx context.Context, taskType string) []ComponentInfo {
	var missing []ComponentInfo
	for _, ct := range TaskTypeDependencies(taskType) {
		info := defaultDetector.GetComponentStatus(ctx, ct)
		if info.Status != StatusInstalled {
			missing = append(missing, info)
		}
	}
	return missing
}
