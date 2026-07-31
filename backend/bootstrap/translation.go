package bootstrap

import (
	"fmt"

	"video-captions/internal/translation"
	"video-captions/utils/logger"

	"go.uber.org/zap"
)

// TranslationExecutor 全局翻译执行器
var TranslationExecutor *translation.OllamaExecutor

// InitTranslation 初始化翻译执行器
func InitTranslation() error {
	if Config == nil {
		return fmt.Errorf("config not initialized")
	}

	cfg := Config.Translation
	if cfg.OllamaURL == "" {
		logger.Logger.Warn("Translation Ollama URL is empty, translation feature will be disabled")
		return nil
	}

	// 创建翻译执行器
	TranslationExecutor = translation.NewOllamaExecutor(translation.OllamaConfig{
		URL:            cfg.OllamaURL,
		Model:          cfg.Model,
		PromptTemplate: cfg.PromptTemplate,
	})

	logger.Logger.Info("Translation executor initialized successfully",
		zap.String("model", cfg.Model))
	return nil
}
