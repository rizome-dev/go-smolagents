// Package agent_types defines the types that can be returned by agents.
package agent_types

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// AgentType is the interface that all agent output types must implement.
// These objects serve three purposes:
// - They behave as they were the type they're meant to be (string, image, etc.)
// - They can be stringified to return a string defining the object
// - They should be displayed correctly when requested
type AgentType interface {
	// ToRaw returns the raw version of the object
	ToRaw() interface{}

	// ToString returns a string representation of the object
	ToString() string
}

// AgentText represents text returned by agents
type AgentText struct {
	value string
}

// NewAgentText creates a new AgentText
func NewAgentText(value string) *AgentText {
	return &AgentText{value: value}
}

// ToRaw returns the raw string value
func (a *AgentText) ToRaw() interface{} {
	return a.value
}

// ToString returns the string value
func (a *AgentText) ToString() string {
	return a.value
}

// String implements the Stringer interface
func (a *AgentText) String() string {
	return a.value
}

// AgentImage represents image data returned by agents
type AgentImage struct {
	raw       image.Image
	path      string
	imagePath string
}

// NewAgentImageFromImage creates a new AgentImage from an image.Image
func NewAgentImageFromImage(img image.Image) *AgentImage {
	return &AgentImage{
		raw: img,
	}
}

// NewAgentImageFromPath creates a new AgentImage from a file path
func NewAgentImageFromPath(path string) *AgentImage {
	return &AgentImage{
		path: path,
	}
}

// NewAgentImageFromBytes creates a new AgentImage from bytes
func NewAgentImageFromBytes(data []byte) (*AgentImage, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image data: %w", err)
	}
	return &AgentImage{raw: img}, nil
}

// ToRaw returns the raw image data
func (a *AgentImage) ToRaw() interface{} {
	if a.raw != nil {
		return a.raw
	}

	if a.path != "" {
		file, err := os.Open(a.path)
		if err != nil {
			log.Printf("Error opening image file: %v", err)
			return nil
		}
		defer file.Close()

		img, _, err := image.Decode(file)
		if err != nil {
			log.Printf("Error decoding image: %v", err)
			return nil
		}
		a.raw = img
		return img
	}

	return nil
}

// ToString returns a path to the serialized version of the image
func (a *AgentImage) ToString() string {
	if a.path != "" {
		return a.path
	}

	if a.imagePath != "" {
		return a.imagePath
	}

	if a.raw != nil {
		// Create temp directory
		tempDir := os.TempDir()
		uuid := uuid.NewString()
		imgPath := filepath.Join(tempDir, uuid+".png")

		file, err := os.Create(imgPath)
		if err != nil {
			log.Printf("Error creating image file: %v", err)
			return ""
		}
		defer file.Close()

		err = png.Encode(file, a.raw)
		if err != nil {
			log.Printf("Error encoding image: %v", err)
			return ""
		}

		a.imagePath = imgPath
		return imgPath
	}

	return ""
}

// Save saves the image to a file
func (a *AgentImage) Save(outputPath string, format string) error {
	img := a.ToRaw().(image.Image)
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("error creating image file: %w", err)
	}
	defer file.Close()

	switch format {
	case "png":
		return png.Encode(file, img)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// AgentAudio represents audio data returned by agents
type AgentAudio struct {
	data       []byte
	path       string
	sampleRate int
}

// NewAgentAudioFromBytes creates a new AgentAudio from bytes
func NewAgentAudioFromBytes(data []byte, sampleRate int) *AgentAudio {
	return &AgentAudio{
		data:       data,
		sampleRate: sampleRate,
	}
}

// NewAgentAudioFromPath creates a new AgentAudio from a file path
func NewAgentAudioFromPath(path string, sampleRate int) *AgentAudio {
	return &AgentAudio{
		path:       path,
		sampleRate: sampleRate,
	}
}

// ToRaw returns the raw audio data
func (a *AgentAudio) ToRaw() interface{} {
	if a.data != nil {
		return a.data
	}

	if a.path != "" {
		data, err := os.ReadFile(a.path)
		if err != nil {
			log.Printf("Error reading audio file: %v", err)
			return nil
		}
		a.data = data
		return data
	}

	return nil
}

// ToString returns a path to the serialized version of the audio
func (a *AgentAudio) ToString() string {
	if a.path != "" {
		return a.path
	}

	if a.data != nil {
		// Create temp directory
		tempDir := os.TempDir()
		uuid := uuid.NewString()
		audioPath := filepath.Join(tempDir, uuid+".wav")

		err := os.WriteFile(audioPath, a.data, 0644)
		if err != nil {
			log.Printf("Error writing audio file: %v", err)
			return ""
		}

		a.path = audioPath
		return audioPath
	}

	return ""
}

// GetSampleRate returns the sample rate
func (a *AgentAudio) GetSampleRate() int {
	return a.sampleRate
}

// HandleAgentOutputTypes converts output to the appropriate agent type
func HandleAgentOutputTypes(output interface{}, outputType string) interface{} {
	switch outputType {
	case "string":
		return NewAgentText(fmt.Sprintf("%v", output))
	case "image":
		switch v := output.(type) {
		case image.Image:
			return NewAgentImageFromImage(v)
		case string:
			return NewAgentImageFromPath(v)
		case []byte:
			img, err := NewAgentImageFromBytes(v)
			if err != nil {
				log.Printf("Error creating image from bytes: %v", err)
				return output
			}
			return img
		}
	}

	// If no specific conversion, return as is
	return output
}

// EncodeImageBase64 encodes an image to base64
func EncodeImageBase64(img image.Image) string {
	buf := new(bytes.Buffer)
	err := png.Encode(buf, img)
	if err != nil {
		log.Printf("Error encoding image to PNG: %v", err)
		return ""
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// MakeImageURL creates a data URL for an image
func MakeImageURL(base64Str string) string {
	return "data:image/png;base64," + base64Str
}

// DecodeImageBase64 decodes a base64 string to an image
func DecodeImageBase64(base64Str string) (image.Image, error) {
	data, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, fmt.Errorf("error decoding base64 string: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("error decoding image data: %w", err)
	}

	return img, nil
}

// HandleAgentInputTypes preprocesses arguments that are AgentType to their raw values
func HandleAgentInputTypes(args []interface{}, kwargs map[string]interface{}) ([]interface{}, map[string]interface{}) {
	processedArgs := make([]interface{}, len(args))
	for i, arg := range args {
		if at, ok := arg.(AgentType); ok {
			processedArgs[i] = at.ToRaw()
		} else {
			processedArgs[i] = arg
		}
	}

	processedKwargs := make(map[string]interface{})
	for k, v := range kwargs {
		if at, ok := v.(AgentType); ok {
			processedKwargs[k] = at.ToRaw()
		} else {
			processedKwargs[k] = v
		}
	}

	return processedArgs, processedKwargs
}
