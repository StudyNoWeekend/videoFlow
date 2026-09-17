package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gotd/td/session"
	"github.com/iyear/tdl/core/storage"
)

// sessionFileName 会话文件名，内容为包含授权密钥的 session.Data 序列化结果
const sessionFileName = "session.json"

// fileSessionStore 把 gotd 会话持久化到磁盘，实现 session.Storage。
// 会话数据等同于账号凭据：文件以 0600 权限写入，请勿提交到版本库或对外共享。
type fileSessionStore struct {
	path string

	mu sync.Mutex
}

// newFileSessionStore 只记录路径，目录在首次写入时按需创建
func newFileSessionStore(dir string) *fileSessionStore {
	return &fileSessionStore{path: filepath.Join(dir, sessionFileName)}
}

// LoadSession 读取已保存的会话，文件不存在时返回 session.ErrNotFound
func (s *fileSessionStore) LoadSession(context.Context) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, session.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("读取 Telegram 会话失败: %w", err)
	}
	return data, nil
}

// StoreSession 先写临时文件再原子重命名，避免写入中断导致会话文件损坏
func (s *fileSessionStore) StoreSession(_ context.Context, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建 Telegram 会话目录失败: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("写入 Telegram 会话失败: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("保存 Telegram 会话失败: %w", err)
	}
	return nil
}

// Exists 会话文件是否存在
func (s *fileSessionStore) Exists() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := os.Stat(s.path)
	return err == nil
}

// Remove 删除本地会话文件（退出登录）
func (s *fileSessionStore) Remove() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除 Telegram 会话失败: %w", err)
	}
	return nil
}

// memKV 实现 tdl storage.Storage 的内存 KV，用于缓存 peers 解析结果。
// 重启后重建缓存即可，无需持久化到磁盘。
type memKV struct {
	mu sync.RWMutex
	m  map[string][]byte
}

func newMemKV() *memKV {
	return &memKV{m: make(map[string][]byte)}
}

// Get 实现 storage.Storage
func (k *memKV) Get(_ context.Context, key string) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	value, ok := k.m[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return value, nil
}

// Set 实现 storage.Storage
func (k *memKV) Set(_ context.Context, key string, value []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.m[key] = value
	return nil
}

// Delete 实现 storage.Storage
func (k *memKV) Delete(_ context.Context, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	delete(k.m, key)
	return nil
}
