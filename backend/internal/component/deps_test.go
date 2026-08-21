package component

import (
	"reflect"
	"testing"
)

func TestTaskTypeDependencies(t *testing.T) {
	tests := []struct {
		name     string
		taskType string
		want     []ComponentType
	}{
		{"字幕生成", "subtitle", []ComponentType{ComponentFFmpeg, ComponentWhisperASR}},
		{"字幕烧录", "subtitle_burn", []ComponentType{ComponentFFmpeg}},
		{"去马赛克", "deblur", []ComponentType{ComponentDocker, ComponentLada}},
		{"修复（deblur 别名）", "repair", []ComponentType{ComponentDocker, ComponentLada}},
		{"清晰度修复", "upscale", []ComponentType{ComponentDocker, ComponentVideo2X}},
		{"未知类型返回空", "unknown", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TaskTypeDependencies(tt.taskType)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TaskTypeDependencies(%q) = %v, want %v", tt.taskType, got, tt.want)
			}
		})
	}
}
