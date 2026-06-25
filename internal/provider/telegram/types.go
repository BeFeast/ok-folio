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
	MessageID int         `json:"message_id"`
	Date      int64       `json:"date"`
	Chat      Chat        `json:"chat"`
	Caption   string      `json:"caption"`
	Photo     []PhotoSize `json:"photo"`
	Document  *Document   `json:"document"`
	Video     *Video      `json:"video"`
	Animation *Document   `json:"animation"`
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
