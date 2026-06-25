package telegram

import (
	"fmt"
	"strconv"
)

type telegramResponse[T any] struct {
	OK          bool               `json:"ok"`
	Result      T                  `json:"result"`
	Description string             `json:"description"`
	Parameters  ResponseParameters `json:"parameters"`
}

type telegramErrorResponse struct {
	OK          bool               `json:"ok"`
	Description string             `json:"description"`
	Parameters  ResponseParameters `json:"parameters"`
}

type ResponseParameters struct {
	RetryAfter int `json:"retry_after"`
}

type Update struct {
	UpdateID    int      `json:"update_id"`
	Message     *Message `json:"message"`
	ChannelPost *Message `json:"channel_post"`
}

func (u Update) ProviderMessage() *Message {
	if u.Message != nil {
		return u.Message
	}
	return u.ChannelPost
}

type Message struct {
	MessageID            int            `json:"message_id"`
	Date                 int64          `json:"date"`
	Chat                 Chat           `json:"chat"`
	Caption              string         `json:"caption"`
	ForwardOrigin        *ForwardOrigin `json:"forward_origin"`
	ForwardFromChat      *Chat          `json:"forward_from_chat"`
	ForwardFromMessageID int            `json:"forward_from_message_id"`
	ForwardSenderName    string         `json:"forward_sender_name"`
	ForwardSignature     string         `json:"forward_signature"`
	ForwardDate          int64          `json:"forward_date"`
	Photo                []PhotoSize    `json:"photo"`
	Document             *Document      `json:"document"`
	Video                *Video         `json:"video"`
	Animation            *Document      `json:"animation"`
}

func (m Message) MediaRef() (MediaRef, bool) {
	if len(m.Photo) > 0 {
		largest := m.Photo[0]
		for _, photo := range m.Photo[1:] {
			if photo.Score() > largest.Score() {
				largest = photo
			}
		}
		return MediaRef{
			FileID:       largest.FileID,
			FileUniqueID: largest.FileUniqueID,
			MIMEType:     "image/jpeg",
			FileName:     fmt.Sprintf("telegram-%s-%d.jpg", m.Chat.IDString(), m.MessageID),
		}, true
	}
	if m.Document != nil {
		return MediaRef{
			FileID:       m.Document.FileID,
			FileUniqueID: m.Document.FileUniqueID,
			MIMEType:     m.Document.MIMEType,
			FileName:     m.Document.FileName,
		}, true
	}
	if m.Video != nil {
		return MediaRef{
			FileID:       m.Video.FileID,
			FileUniqueID: m.Video.FileUniqueID,
			MIMEType:     m.Video.MIMEType,
			FileName:     m.Video.FileName,
		}, true
	}
	if m.Animation != nil {
		return MediaRef{
			FileID:       m.Animation.FileID,
			FileUniqueID: m.Animation.FileUniqueID,
			MIMEType:     m.Animation.MIMEType,
			FileName:     m.Animation.FileName,
		}, true
	}
	return MediaRef{}, false
}

func (m Message) URL() string {
	if m.Chat.Username == "" {
		return ""
	}
	return fmt.Sprintf("https://t.me/%s/%d", m.Chat.Username, m.MessageID)
}

func (m Message) SourceRef() SourceRef {
	if m.ForwardOrigin != nil {
		if ref, ok := m.ForwardOrigin.SourceRef(); ok {
			return ref
		}
	}
	if m.ForwardFromChat != nil {
		ref := SourceRef{
			ChatID:    m.ForwardFromChat.IDString(),
			ChatName:  m.ForwardFromChat.DisplayName(),
			MessageID: m.ForwardFromMessageID,
			URL:       messageURL(*m.ForwardFromChat, m.ForwardFromMessageID),
		}
		if ref.MessageID > 0 {
			return ref
		}
		if m.ForwardSenderName != "" {
			ref.ChatName = m.ForwardSenderName
		}
		if m.ForwardSignature != "" {
			ref.ChatName = m.ForwardSignature
		}
	}
	if m.ForwardSenderName != "" {
		return SourceRef{
			ChatID:   m.Chat.IDString(),
			ChatName: m.ForwardSenderName,
		}
	}
	return SourceRef{
		ChatID:    m.Chat.IDString(),
		ChatName:  m.Chat.DisplayName(),
		MessageID: m.MessageID,
		URL:       m.URL(),
	}
}

type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (c Chat) IDString() string {
	return strconv.FormatInt(c.ID, 10)
}

func (c Chat) DisplayName() string {
	if c.Title != "" {
		return c.Title
	}
	if c.FirstName != "" || c.LastName != "" {
		if c.LastName == "" {
			return c.FirstName
		}
		if c.FirstName == "" {
			return c.LastName
		}
		return c.FirstName + " " + c.LastName
	}
	if c.Username != "" {
		return c.Username
	}
	return c.IDString()
}

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (u User) IDString() string {
	return strconv.FormatInt(u.ID, 10)
}

func (u User) DisplayName() string {
	if u.FirstName != "" || u.LastName != "" {
		if u.LastName == "" {
			return u.FirstName
		}
		if u.FirstName == "" {
			return u.LastName
		}
		return u.FirstName + " " + u.LastName
	}
	if u.Username != "" {
		return u.Username
	}
	return u.IDString()
}

type ForwardOrigin struct {
	Type            string `json:"type"`
	SenderUser      *User  `json:"sender_user"`
	SenderUserName  string `json:"sender_user_name"`
	Chat            *Chat  `json:"chat"`
	MessageID       int    `json:"message_id"`
	Date            int64  `json:"date"`
	AuthorSignature string `json:"author_signature"`
}

func (o ForwardOrigin) SourceRef() (SourceRef, bool) {
	if o.Chat != nil {
		ref := SourceRef{
			ChatID:    o.Chat.IDString(),
			ChatName:  o.Chat.DisplayName(),
			MessageID: o.MessageID,
			URL:       messageURL(*o.Chat, o.MessageID),
		}
		if o.AuthorSignature != "" {
			ref.Author = o.AuthorSignature
		}
		return ref, ref.ChatID != ""
	}
	if o.SenderUser != nil {
		return SourceRef{
			ChatID:   o.SenderUser.IDString(),
			ChatName: o.SenderUser.DisplayName(),
		}, true
	}
	if o.SenderUserName != "" {
		return SourceRef{ChatName: o.SenderUserName}, true
	}
	return SourceRef{}, false
}

type SourceRef struct {
	ChatID    string
	ChatName  string
	MessageID int
	URL       string
	Author    string
}

func messageURL(chat Chat, messageID int) string {
	if chat.Username == "" || messageID <= 0 {
		return ""
	}
	return fmt.Sprintf("https://t.me/%s/%d", chat.Username, messageID)
}

type PhotoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int    `json:"file_size"`
}

func (p PhotoSize) Score() int {
	if p.FileSize > 0 {
		return p.FileSize
	}
	return p.Width * p.Height
}

type Document struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileName     string `json:"file_name"`
	MIMEType     string `json:"mime_type"`
}

type Video struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileName     string `json:"file_name"`
	MIMEType     string `json:"mime_type"`
}

type MediaRef struct {
	FileID       string
	FileUniqueID string
	FileName     string
	MIMEType     string
}

func (m MediaRef) StableID() string {
	if m.FileUniqueID != "" {
		return m.FileUniqueID
	}
	return m.FileID
}

type File struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileSize     int    `json:"file_size"`
	FilePath     string `json:"file_path"`
}
