package telegram

import (
	"context"
	"errors"
	"fmt"

	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"github.com/iyear/tdl/core/tmedia"
	"github.com/iyear/tdl/core/util/tutil"
)

// 媒体类别
const (
	MediaTypeVideo    = "video"
	MediaTypePhoto    = "photo"
	MediaTypeAudio    = "audio"
	MediaTypeDocument = "document"
)

var (
	// ErrMessageDeleted 消息不存在或已被删除
	ErrMessageDeleted = errors.New("消息不存在或已被删除")
	// ErrNoMedia 消息中不包含可下载的媒体
	ErrNoMedia = errors.New("该消息不包含可下载的媒体")
	// ErrNotVideoMedia 消息中的媒体不是视频
	ErrNotVideoMedia = errors.New("仅支持下载视频类媒体")
	// ErrNotLoggedIn 尚未登录 Telegram 账号
	ErrNotLoggedIn = errors.New("Telegram 未登录，请先扫码登录")
)

// MediaInfo 消息中媒体的元信息
type MediaInfo struct {
	// Name 建议的文件名
	Name string
	// Size 文件大小（字节）
	Size int64
	// MIME 媒体类型
	MIME string
	// Kind 媒体类别
	Kind string
	// Duration 视频时长（秒）
	Duration int64
	// DC 文件所在的 DC，下载时必须使用该 DC 的连接
	DC int
	// Location 下载位置
	Location tg.InputFileLocationClass
	// MessageID 消息 ID
	MessageID int
}

// IsVideo 是否为视频类媒体
func (m *MediaInfo) IsVideo() bool {
	return m.Kind == MediaTypeVideo
}

// kindLabel 媒体类别的中文描述，用于错误提示
func kindLabel(kind string) string {
	switch kind {
	case MediaTypeVideo:
		return "视频"
	case MediaTypePhoto:
		return "图片"
	case MediaTypeAudio:
		return "音频"
	default:
		return "文件"
	}
}

// fetchMessage 解析链接并取回消息，链接解析与消息抓取均复用 tdl 的 tutil
func fetchMessage(ctx context.Context, cur *conn, rawURL string) (*tg.Message, error) {
	peer, msgID, err := tutil.ParseMessageLink(ctx, cur.manager, rawURL)
	if err != nil {
		return nil, fmt.Errorf("解析 Telegram 链接失败: %w", err)
	}

	msg, err := tutil.GetSingleMessage(ctx, cur.pool.Default(ctx), peer.InputPeer(), msgID)
	if err != nil {
		switch {
		case errors.Is(err, tutil.ErrMessageDeleted):
			return nil, ErrMessageDeleted
		case tgerr.Is(err, "CHANNEL_PRIVATE", "CHANNEL_INVALID", "PEER_ID_INVALID"):
			return nil, errors.New("无权访问该频道，请确认当前账号已加入")
		default:
			return nil, fmt.Errorf("获取消息失败: %w", err)
		}
	}
	return msg, nil
}

// classifyMedia 提取消息中的媒体信息并判定类别。
// 下载位置、文件名、大小等由 tdl 的 tmedia 提取，这里补充业务上关心的「是否视频」判定。
func classifyMedia(msg *tg.Message) (*MediaInfo, error) {
	switch msg.Media.(type) {
	case nil:
		return nil, ErrNoMedia
	case *tg.MessageMediaPhoto:
		// 图片不在支持范围内，单独给出可理解的提示
		return nil, fmt.Errorf("%w（该消息是图片）", ErrNotVideoMedia)
	}

	media, ok := tmedia.GetMedia(msg)
	if !ok {
		return nil, ErrNoMedia
	}

	info := &MediaInfo{
		Name:      media.Name,
		Size:      media.Size,
		DC:        media.DC,
		Location:  media.InputFileLoc,
		MessageID: msg.ID,
		Kind:      MediaTypeDocument,
	}

	if doc, ok := media2Document(msg); ok {
		info.MIME = doc.MimeType
		for _, attr := range doc.Attributes {
			switch a := attr.(type) {
			case *tg.DocumentAttributeVideo:
				// 视频属性优先级最高，明确标记为视频
				info.Kind = MediaTypeVideo
				info.Duration = int64(a.Duration)
			case *tg.DocumentAttributeAnimated:
				// 动图（GIF / 无声 MP4）按视频处理
				if info.Kind == MediaTypeDocument {
					info.Kind = MediaTypeVideo
				}
			case *tg.DocumentAttributeAudio:
				if info.Kind == MediaTypeDocument {
					info.Kind = MediaTypeAudio
				}
			case *tg.DocumentAttributeFilename:
				if info.Name == "" {
					info.Name = a.FileName
				}
			}
		}
	}

	if info.Name == "" {
		info.Name = fmt.Sprintf("telegram_%d", msg.ID)
	}
	return info, nil
}

// media2Document 从消息中取出文档本体
func media2Document(msg *tg.Message) (*tg.Document, bool) {
	media, ok := msg.Media.(*tg.MessageMediaDocument)
	if !ok {
		return nil, false
	}
	doc, ok := media.Document.(*tg.Document)
	return doc, ok
}
