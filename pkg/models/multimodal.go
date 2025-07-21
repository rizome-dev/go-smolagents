// Copyright 2025 Rizome Labs, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package models - Multimodal support for vision and audio
package models

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// MediaType represents different types of media content
type MediaType string

const (
	MediaTypeText  MediaType = "text"
	MediaTypeImage MediaType = "image"
	MediaTypeAudio MediaType = "audio"
	MediaTypeVideo MediaType = "video"
)

// MediaContent represents multimodal content
type MediaContent struct {
	Type      MediaType   `json:"type"`
	Text      *string     `json:"text,omitempty"`
	ImageURL  *ImageURL   `json:"image_url,omitempty"`
	AudioData *AudioData  `json:"audio_data,omitempty"`
	VideoData *VideoData  `json:"video_data,omitempty"`
	Metadata  interface{} `json:"metadata,omitempty"`
}

// ImageURL represents an image reference
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "low", "high", "auto"
}

// AudioData represents audio content
type AudioData struct {
	Data   string `json:"data"`   // Base64 encoded audio data
	Format string `json:"format"` // "mp3", "wav", "ogg", etc.
	URL    string `json:"url,omitempty"`
}

// VideoData represents video content
type VideoData struct {
	Data   string `json:"data"`   // Base64 encoded video data
	Format string `json:"format"` // "mp4", "webm", "avi", etc.
	URL    string `json:"url,omitempty"`
}

// MultimodalMessage represents a message with mixed content types
type MultimodalMessage struct {
	Role    string          `json:"role"`
	Content []*MediaContent `json:"content"`
}

// MultimodalSupport provides utilities for handling multimodal content
type MultimodalSupport struct {
	MaxImageSize int64    `json:"max_image_size"` // Maximum image size in bytes
	MaxAudioSize int64    `json:"max_audio_size"` // Maximum audio size in bytes
	MaxVideoSize int64    `json:"max_video_size"` // Maximum video size in bytes
	ImageFormats []string `json:"image_formats"`  // Supported image formats
	AudioFormats []string `json:"audio_formats"`  // Supported audio formats
	VideoFormats []string `json:"video_formats"`  // Supported video formats
}

// NewMultimodalSupport creates a new multimodal support instance
func NewMultimodalSupport() *MultimodalSupport {
	return &MultimodalSupport{
		MaxImageSize: 20 * 1024 * 1024, // 20MB
		MaxAudioSize: 25 * 1024 * 1024, // 25MB
		MaxVideoSize: 50 * 1024 * 1024, // 50MB
		ImageFormats: []string{"jpg", "jpeg", "png", "gif", "webp", "bmp"},
		AudioFormats: []string{"mp3", "wav", "ogg", "m4a", "flac"},
		VideoFormats: []string{"mp4", "webm", "avi", "mov", "mkv"},
	}
}

// LoadImageFromFile loads an image from a file path
func (ms *MultimodalSupport) LoadImageFromFile(filePath string) (*MediaContent, error) {
	// Check file size
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.Size() > ms.MaxImageSize {
		return nil, fmt.Errorf("image file too large: %d bytes (max: %d)", info.Size(), ms.MaxImageSize)
	}

	// Check file extension
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filePath), "."))
	if !ms.isValidImageFormat(ext) {
		return nil, fmt.Errorf("unsupported image format: %s", ext)
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Encode to base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	// Determine MIME type
	mimeType := mime.TypeByExtension("." + ext)
	if mimeType == "" {
		mimeType = "image/" + ext
	}

	// Create data URL
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

	return &MediaContent{
		Type: MediaTypeImage,
		ImageURL: &ImageURL{
			URL:    dataURL,
			Detail: "auto",
		},
	}, nil
}

// LoadImageFromURL loads an image from a URL
func (ms *MultimodalSupport) LoadImageFromURL(url string, detail string) (*MediaContent, error) {
	if detail == "" {
		detail = "auto"
	}

	// Validate detail parameter
	if detail != "low" && detail != "high" && detail != "auto" {
		return nil, fmt.Errorf("invalid detail level: %s (must be 'low', 'high', or 'auto')", detail)
	}

	return &MediaContent{
		Type: MediaTypeImage,
		ImageURL: &ImageURL{
			URL:    url,
			Detail: detail,
		},
	}, nil
}

// LoadAudioFromFile loads audio from a file path
func (ms *MultimodalSupport) LoadAudioFromFile(filePath string) (*MediaContent, error) {
	// Check file size
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.Size() > ms.MaxAudioSize {
		return nil, fmt.Errorf("audio file too large: %d bytes (max: %d)", info.Size(), ms.MaxAudioSize)
	}

	// Check file extension
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filePath), "."))
	if !ms.isValidAudioFormat(ext) {
		return nil, fmt.Errorf("unsupported audio format: %s", ext)
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Encode to base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	return &MediaContent{
		Type: MediaTypeAudio,
		AudioData: &AudioData{
			Data:   base64Data,
			Format: ext,
		},
	}, nil
}

// LoadAudioFromURL loads audio from a URL
func (ms *MultimodalSupport) LoadAudioFromURL(url string) (*MediaContent, error) {
	// Download the audio file
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download audio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download audio: HTTP %d", resp.StatusCode)
	}

	// Check content length
	if resp.ContentLength > ms.MaxAudioSize {
		return nil, fmt.Errorf("audio file too large: %d bytes (max: %d)", resp.ContentLength, ms.MaxAudioSize)
	}

	// Read data
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio data: %w", err)
	}

	// Determine format from content type or URL
	format := "unknown"
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		if strings.HasPrefix(contentType, "audio/") {
			format = strings.TrimPrefix(contentType, "audio/")
		}
	} else {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(url), "."))
		if ms.isValidAudioFormat(ext) {
			format = ext
		}
	}

	// Encode to base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	return &MediaContent{
		Type: MediaTypeAudio,
		AudioData: &AudioData{
			Data:   base64Data,
			Format: format,
			URL:    url,
		},
	}, nil
}

// LoadVideoFromFile loads video from a file path
func (ms *MultimodalSupport) LoadVideoFromFile(filePath string) (*MediaContent, error) {
	// Check file size
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.Size() > ms.MaxVideoSize {
		return nil, fmt.Errorf("video file too large: %d bytes (max: %d)", info.Size(), ms.MaxVideoSize)
	}

	// Check file extension
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filePath), "."))
	if !ms.isValidVideoFormat(ext) {
		return nil, fmt.Errorf("unsupported video format: %s", ext)
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Encode to base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	return &MediaContent{
		Type: MediaTypeVideo,
		VideoData: &VideoData{
			Data:   base64Data,
			Format: ext,
		},
	}, nil
}

// CreateTextContent creates text content
func (ms *MultimodalSupport) CreateTextContent(text string) *MediaContent {
	return &MediaContent{
		Type: MediaTypeText,
		Text: &text,
	}
}

// CreateMultimodalMessage creates a multimodal message
func (ms *MultimodalSupport) CreateMultimodalMessage(role string, contents ...*MediaContent) *MultimodalMessage {
	return &MultimodalMessage{
		Role:    role,
		Content: contents,
	}
}

// ConvertToStandardFormat converts multimodal messages to standard message format
func (ms *MultimodalSupport) ConvertToStandardFormat(messages []*MultimodalMessage) []map[string]interface{} {
	var result []map[string]interface{}

	for _, msg := range messages {
		standardMsg := map[string]interface{}{
			"role": msg.Role,
		}

		if len(msg.Content) == 1 && msg.Content[0].Type == MediaTypeText {
			// Simple text message
			standardMsg["content"] = *msg.Content[0].Text
		} else {
			// Complex multimodal message
			var contentArray []map[string]interface{}

			for _, content := range msg.Content {
				switch content.Type {
				case MediaTypeText:
					contentArray = append(contentArray, map[string]interface{}{
						"type": "text",
						"text": *content.Text,
					})
				case MediaTypeImage:
					contentArray = append(contentArray, map[string]interface{}{
						"type":      "image_url",
						"image_url": content.ImageURL,
					})
				case MediaTypeAudio:
					contentArray = append(contentArray, map[string]interface{}{
						"type":       "audio",
						"audio_data": content.AudioData,
					})
				case MediaTypeVideo:
					contentArray = append(contentArray, map[string]interface{}{
						"type":       "video",
						"video_data": content.VideoData,
					})
				}
			}

			standardMsg["content"] = contentArray
		}

		result = append(result, standardMsg)
	}

	return result
}

// isValidImageFormat checks if an image format is supported
func (ms *MultimodalSupport) isValidImageFormat(format string) bool {
	for _, supported := range ms.ImageFormats {
		if format == supported {
			return true
		}
	}
	return false
}

// isValidAudioFormat checks if an audio format is supported
func (ms *MultimodalSupport) isValidAudioFormat(format string) bool {
	for _, supported := range ms.AudioFormats {
		if format == supported {
			return true
		}
	}
	return false
}

// isValidVideoFormat checks if a video format is supported
func (ms *MultimodalSupport) isValidVideoFormat(format string) bool {
	for _, supported := range ms.VideoFormats {
		if format == supported {
			return true
		}
	}
	return false
}

// ExtractText extracts all text content from a multimodal message
func (ms *MultimodalSupport) ExtractText(message *MultimodalMessage) string {
	var textParts []string

	for _, content := range message.Content {
		if content.Type == MediaTypeText && content.Text != nil {
			textParts = append(textParts, *content.Text)
		}
	}

	return strings.Join(textParts, " ")
}

// GetMediaCount returns the count of different media types in a message
func (ms *MultimodalSupport) GetMediaCount(message *MultimodalMessage) map[MediaType]int {
	counts := make(map[MediaType]int)

	for _, content := range message.Content {
		counts[content.Type]++
	}

	return counts
}

// ValidateMessage validates a multimodal message
func (ms *MultimodalSupport) ValidateMessage(message *MultimodalMessage) error {
	if message.Role == "" {
		return fmt.Errorf("message role cannot be empty")
	}

	if len(message.Content) == 0 {
		return fmt.Errorf("message content cannot be empty")
	}

	for i, content := range message.Content {
		if err := ms.validateContent(content); err != nil {
			return fmt.Errorf("content[%d]: %w", i, err)
		}
	}

	return nil
}

// validateContent validates a single content item
func (ms *MultimodalSupport) validateContent(content *MediaContent) error {
	switch content.Type {
	case MediaTypeText:
		if content.Text == nil {
			return fmt.Errorf("text content must have text field")
		}
	case MediaTypeImage:
		if content.ImageURL == nil {
			return fmt.Errorf("image content must have image_url field")
		}
		if content.ImageURL.URL == "" {
			return fmt.Errorf("image URL cannot be empty")
		}
	case MediaTypeAudio:
		if content.AudioData == nil {
			return fmt.Errorf("audio content must have audio_data field")
		}
		if content.AudioData.Data == "" && content.AudioData.URL == "" {
			return fmt.Errorf("audio content must have either data or URL")
		}
	case MediaTypeVideo:
		if content.VideoData == nil {
			return fmt.Errorf("video content must have video_data field")
		}
		if content.VideoData.Data == "" && content.VideoData.URL == "" {
			return fmt.Errorf("video content must have either data or URL")
		}
	default:
		return fmt.Errorf("unsupported content type: %s", content.Type)
	}

	return nil
}

// CompressImage compresses an image for better performance (placeholder implementation)
func (ms *MultimodalSupport) CompressImage(content *MediaContent, quality int) (*MediaContent, error) {
	if content.Type != MediaTypeImage {
		return nil, fmt.Errorf("content is not an image")
	}

	// This is a placeholder implementation
	// In a real implementation, this would:
	// 1. Decode the base64 image data
	// 2. Compress the image using libraries like "image/jpeg" or third-party libraries
	// 3. Re-encode to base64
	// 4. Return the compressed version

	return content, nil // Return original for now
}

// GenerateImageThumbnail generates a thumbnail for an image (placeholder implementation)
func (ms *MultimodalSupport) GenerateImageThumbnail(content *MediaContent, width, height int) (*MediaContent, error) {
	if content.Type != MediaTypeImage {
		return nil, fmt.Errorf("content is not an image")
	}

	// This is a placeholder implementation
	// In a real implementation, this would:
	// 1. Decode the base64 image data
	// 2. Resize the image to the specified dimensions
	// 3. Re-encode to base64
	// 4. Return the thumbnail

	return content, nil // Return original for now
}

// DefaultMultimodalSupport is the default multimodal support instance
var DefaultMultimodalSupport = NewMultimodalSupport()

// LoadImage is a convenience function to load an image using the default instance
func LoadImage(filePath string) (*MediaContent, error) {
	return DefaultMultimodalSupport.LoadImageFromFile(filePath)
}

// LoadImageURL is a convenience function to load an image from URL using the default instance
func LoadImageURL(url string, detail string) (*MediaContent, error) {
	return DefaultMultimodalSupport.LoadImageFromURL(url, detail)
}

// LoadAudio is a convenience function to load audio using the default instance
func LoadAudio(filePath string) (*MediaContent, error) {
	return DefaultMultimodalSupport.LoadAudioFromFile(filePath)
}

// LoadVideo is a convenience function to load video using the default instance
func LoadVideo(filePath string) (*MediaContent, error) {
	return DefaultMultimodalSupport.LoadVideoFromFile(filePath)
}

// CreateText is a convenience function to create text content using the default instance
func CreateText(text string) *MediaContent {
	return DefaultMultimodalSupport.CreateTextContent(text)
}

// CreateMessage is a convenience function to create a multimodal message using the default instance
func CreateMessage(role string, contents ...*MediaContent) *MultimodalMessage {
	return DefaultMultimodalSupport.CreateMultimodalMessage(role, contents...)
}
