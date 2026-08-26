package scheduler

import (
	"path/filepath"
)

// canOverwriteSource 报告覆盖模式是否允许把产物原地替换处理源文件。
// 仅当处理源仍位于当前输出目录内时允许：历史任务的 source_path 可能指向
// 旧版"输入树同名子目录"（output_dir 未配置时期的产物位置），此时覆盖会把
// 产物写回输入目录，必须拒绝并降级为生成独立产物文件。
func canOverwriteSource(sourcePath, outputDir string) bool {
	if sourcePath == "" {
		return false
	}
	return filepath.Dir(filepath.Clean(sourcePath)) == filepath.Clean(outputDir)
}
