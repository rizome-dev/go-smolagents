// Package agents provides type definitions for agent inputs and outputs
package agents

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// AgentType is the interface for all agent data types
type AgentType interface {
	// GetType returns the type of the agent data
	GetType() string

	// ToRaw converts the agent data to a raw value
	ToRaw() interface{}

	// String returns a string representation
	String() string
}

// AgentText represents text data
type AgentText struct {
	Text string
}

// NewAgentText creates a new AgentText
func NewAgentText(text string) *AgentText {
	return &AgentText{Text: text}
}

// GetType returns the type of the agent data
func (a *AgentText) GetType() string {
	return "text"
}

// ToRaw converts the agent data to a raw value
func (a *AgentText) ToRaw() interface{} {
	return a.Text
}

// String returns a string representation
func (a *AgentText) String() string {
	return a.Text
}

// AgentImage represents image data
type AgentImage struct {
	Image image.Image
	Path  string
	Data  []byte
}

// NewAgentImageFromImage creates a new AgentImage from an image
func NewAgentImageFromImage(img image.Image) *AgentImage {
	return &AgentImage{Image: img}
}

// NewAgentImageFromPath creates a new AgentImage from a file path
func NewAgentImageFromPath(path string) (*AgentImage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read image file: %w", err)
	}

	return &AgentImage{
		Path: path,
		Data: data,
	}, nil
}

// NewAgentImageFromBytes creates a new AgentImage from byte data
func NewAgentImageFromBytes(data []byte) (*AgentImage, error) {
	return &AgentImage{
		Data: data,
	}, nil
}

// GetType returns the type of the agent data
func (a *AgentImage) GetType() string {
	return "image"
}

// ToRaw converts the agent data to a raw value
func (a *AgentImage) ToRaw() interface{} {
	if a.Image != nil {
		// Encode as base64
		return "data:image/png;base64," + EncodeImageBase64(a.Image)
	}

	if a.Path != "" {
		return "file://" + a.Path
	}

	if a.Data != nil {
		return fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(a.Data))
	}

	return nil
}

// String returns a string representation
func (a *AgentImage) String() string {
	return fmt.Sprintf("<Image:%p>", a)
}

// AgentAudio represents audio data
type AgentAudio struct {
	Path string
	Data []byte
}

// NewAgentAudioFromPath creates a new AgentAudio from a file path
func NewAgentAudioFromPath(path string) (*AgentAudio, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio file: %w", err)
	}

	return &AgentAudio{
		Path: path,
		Data: data,
	}, nil
}

// NewAgentAudioFromBytes creates a new AgentAudio from byte data
func NewAgentAudioFromBytes(data []byte) (*AgentAudio, error) {
	return &AgentAudio{
		Data: data,
	}, nil
}

// GetType returns the type of the agent data
func (a *AgentAudio) GetType() string {
	return "audio"
}

// ToRaw converts the agent data to a raw value
func (a *AgentAudio) ToRaw() interface{} {
	if a.Path != "" {
		return "file://" + a.Path
	}

	if a.Data != nil {
		ext := "mp3" // default
		if a.Path != "" {
			ext = strings.TrimPrefix(filepath.Ext(a.Path), ".")
		}
		return fmt.Sprintf("data:audio/%s;base64,%s", ext, base64.StdEncoding.EncodeToString(a.Data))
	}

	return nil
}

// String returns a string representation
func (a *AgentAudio) String() string {
	return fmt.Sprintf("<Audio:%p>", a)
}

// EncodeImageBase64 encodes an image to a base64 string
func EncodeImageBase64(img image.Image) string {
	var buf strings.Builder
	encoder := base64.NewEncoder(base64.StdEncoding, &buf)

	// Encode as PNG for better quality
	if err := png.Encode(encoder, img); err != nil {
		return ""
	}

	encoder.Close()
	return buf.String()
}

// MakeImageURL creates a data URL for an image
func MakeImageURL(img image.Image) string {
	return "data:image/png;base64," + EncodeImageBase64(img)
}

// DecodeImageBase64 decodes a base64 string to an image
func DecodeImageBase64(data string) (image.Image, error) {
	// Remove the data URL prefix if present
	if strings.HasPrefix(data, "data:") {
		parts := strings.Split(data, ",")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid data URL format")
		}
		data = parts[1]
	}

	// Decode the base64 data
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 data: %w", err)
	}

	// Decode the image
	img, err := png.Decode(strings.NewReader(string(decoded)))
	if err != nil {
		// Try as JPEG if PNG fails
		img, err = jpeg.Decode(strings.NewReader(string(decoded)))
		if err != nil {
			return nil, fmt.Errorf("failed to decode image data: %w", err)
		}
	}

	return img, nil
}

// HandleAgentInputTypes converts input types based on their type
func HandleAgentInputTypes(inputs map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range inputs {
		if agentType, ok := v.(AgentType); ok {
			result[k] = agentType.ToRaw()
		} else {
			result[k] = v
		}
	}

	return result
}

// HandleAgentOutputTypes converts output types based on their type
func HandleAgentOutputTypes(output interface{}, outputType string) interface{} {
	if output == nil {
		return nil
	}

	switch outputType {
	case "string":
		if s, ok := output.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", output)
	case "boolean":
		if b, ok := output.(bool); ok {
			return b
		}
		return false
	case "integer":
		// Try to convert to integer
		switch v := output.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return v
		case float32:
			return int(v)
		case float64:
			return int(v)
		default:
			return 0
		}
	case "number":
		// Try to convert to number
		switch v := output.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			return v
		default:
			return 0.0
		}
	case "image":
		// Handle image output
		if img, ok := output.(*AgentImage); ok {
			return img.ToRaw()
		}
		return nil
	case "audio":
		// Handle audio output
		if audio, ok := output.(*AgentAudio); ok {
			return audio.ToRaw()
		}
		return nil
	case "array":
		// Try to convert to array
		if arr, ok := output.([]interface{}); ok {
			return arr
		}
		return []interface{}{output}
	case "object":
		// Try to convert to object
		if obj, ok := output.(map[string]interface{}); ok {
			return obj
		}
		return map[string]interface{}{"value": output}
	default:
		// Any other type
		return output
	}
}
