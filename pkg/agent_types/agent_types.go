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

// Package agent_types provides types that can be returned by agents.
//
// These objects serve three purposes:
// - They behave as they were the type they're meant to be (e.g., string for text, image.Image for images)
// - They can be stringified: String() method to return a string defining the object
// - They integrate with Go's type system and can be serialized/deserialized
package agent_types

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AgentType is the interface that all agent return types must implement
type AgentType interface {
	// ToRaw returns the "raw" version of the object
	ToRaw() interface{}

	// ToString returns the stringified version of the object
	ToString() string

	// String implements the Stringer interface
	String() string
}

// AgentText represents text type returned by the agent. Behaves as a string.
type AgentText struct {
	value string
}

// NewAgentText creates a new AgentText instance
func NewAgentText(value string) *AgentText {
	return &AgentText{value: value}
}

// ToRaw implements AgentType
func (at *AgentText) ToRaw() interface{} {
	return at.value
}

// ToString implements AgentType
func (at *AgentText) ToString() string {
	return at.value
}

// String implements Stringer interface
func (at *AgentText) String() string {
	return at.ToString()
}

// Value returns the underlying string value
func (at *AgentText) Value() string {
	return at.value
}

// AgentImage represents image type returned by the agent. Behaves as an image.Image.
type AgentImage struct {
	value  interface{}
	path   string
	rawImg image.Image
	tensor interface{} // For tensor support if needed
}

// NewAgentImage creates a new AgentImage instance
func NewAgentImage(value interface{}) (*AgentImage, error) {
	ai := &AgentImage{value: value}

	switch v := value.(type) {
	case *AgentImage:
		ai.rawImg = v.rawImg
		ai.path = v.path
		ai.tensor = v.tensor
		return ai, nil

	case image.Image:
		ai.rawImg = v
		return ai, nil

	case []byte:
		img, _, err := image.Decode(bytes.NewReader(v))
		if err != nil {
			return nil, fmt.Errorf("failed to decode image from bytes: %w", err)
		}
		ai.rawImg = img
		return ai, nil

	case string:
		ai.path = v
		return ai, nil

	default:
		return nil, fmt.Errorf("unsupported type for AgentImage: %T", value)
	}
}

// ToRaw implements AgentType
func (ai *AgentImage) ToRaw() interface{} {
	if ai.rawImg != nil {
		return ai.rawImg
	}

	if ai.path != "" {
		file, err := os.Open(ai.path)
		if err != nil {
			log.Printf("Error opening image file %s: %v", ai.path, err)
			return nil
		}
		defer file.Close()

		img, _, err := image.Decode(file)
		if err != nil {
			log.Printf("Error decoding image file %s: %v", ai.path, err)
			return nil
		}

		ai.rawImg = img
		return ai.rawImg
	}

	if ai.tensor != nil {
		// Handle tensor conversion if needed
		// For now, we'll support basic float32/float64 arrays
		switch t := ai.tensor.(type) {
		case [][]float32:
			return ai.tensorToImage(t)
		case [][]float64:
			// Convert to float32
			float32Array := make([][]float32, len(t))
			for i := range t {
				float32Array[i] = make([]float32, len(t[i]))
				for j := range t[i] {
					float32Array[i][j] = float32(t[i][j])
				}
			}
			return ai.tensorToImage(float32Array)
		default:
			log.Printf("Unsupported tensor type for image conversion: %T", t)
			return nil
		}
	}

	return nil
}

// ToString implements AgentType
func (ai *AgentImage) ToString() string {
	if ai.path != "" {
		return ai.path
	}

	if ai.rawImg != nil {
		// Create temporary file
		tempDir := os.TempDir()
		filename := fmt.Sprintf("agent_image_%s.png", ai.generateID())
		ai.path = filepath.Join(tempDir, filename)

		file, err := os.Create(ai.path)
		if err != nil {
			log.Printf("Error creating temp image file: %v", err)
			return ""
		}
		defer file.Close()

		err = png.Encode(file, ai.rawImg)
		if err != nil {
			log.Printf("Error encoding image to PNG: %v", err)
			return ""
		}

		return ai.path
	}

	if ai.tensor != nil {
		// Convert tensor to image first
		rawImg := ai.ToRaw()
		if img, ok := rawImg.(image.Image); ok && img != nil {
			ai.rawImg = img
			return ai.ToString() // Recursive call to save the image
		}
		return ""
	}

	return ""
}

// String implements Stringer interface
func (ai *AgentImage) String() string {
	return ai.ToString()
}

// Save saves the image to the specified writer with the given format
func (ai *AgentImage) Save(w io.Writer, format string) error {
	img := ai.ToRaw()
	if img == nil {
		return fmt.Errorf("no image data available")
	}

	rawImg, ok := img.(image.Image)
	if !ok {
		return fmt.Errorf("invalid image data type")
	}

	switch strings.ToLower(format) {
	case "png":
		return png.Encode(w, rawImg)
	default:
		return fmt.Errorf("unsupported image format: %s", format)
	}
}

// generateID generates a unique identifier
func (ai *AgentImage) generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("%x", bytes)
}

// tensorToImage converts a 2D float32 array to an image
func (ai *AgentImage) tensorToImage(tensor [][]float32) image.Image {
	if len(tensor) == 0 || len(tensor[0]) == 0 {
		return nil
	}

	height := len(tensor)
	width := len(tensor[0])

	// Create a new grayscale image
	img := image.NewGray(image.Rect(0, 0, width, height))

	// Convert tensor values to pixel values
	// Following Python implementation: (255 - array * 255)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Clamp value between 0 and 1
			val := tensor[y][x]
			if val < 0 {
				val = 0
			} else if val > 1 {
				val = 1
			}

			// Convert to uint8 (0-255)
			pixelValue := uint8((1 - val) * 255)
			img.SetGray(x, y, color.Gray{pixelValue})
		}
	}

	return img
}

// AgentAudio represents audio type returned by the agent
type AgentAudio struct {
	value      interface{}
	sampleRate int
	path       string
	tensor     interface{}
}

// NewAgentAudio creates a new AgentAudio instance
func NewAgentAudio(value interface{}, sampleRate ...int) (*AgentAudio, error) {
	defaultSampleRate := 16000
	if len(sampleRate) > 0 {
		defaultSampleRate = sampleRate[0]
	}

	aa := &AgentAudio{
		value:      value,
		sampleRate: defaultSampleRate,
	}

	switch v := value.(type) {
	case string:
		aa.path = v
		return aa, nil

	case []float32, []float64:
		aa.tensor = v
		return aa, nil

	case [2]interface{}: // tuple-like structure (samplerate, data)
		if rate, ok := v[0].(int); ok {
			aa.sampleRate = rate
		}
		aa.tensor = v[1]
		return aa, nil

	default:
		return nil, fmt.Errorf("unsupported type for AgentAudio: %T", value)
	}
}

// ToRaw implements AgentType
func (aa *AgentAudio) ToRaw() interface{} {
	if aa.tensor != nil {
		return aa.tensor
	}

	if aa.path != "" {
		// Load audio file - this would require additional audio processing libraries
		// For full implementation, we would need to:
		// 1. Read WAV/MP3/other audio formats
		// 2. Extract PCM samples
		// 3. Convert to tensor format
		log.Printf("Audio file loading not implemented for path: %s - would require audio processing libraries", aa.path)
		return nil
	}

	return nil
}

// ToString implements AgentType
func (aa *AgentAudio) ToString() string {
	if aa.path != "" {
		return aa.path
	}

	if aa.tensor != nil {
		// Create temporary file and save audio data
		tempDir := os.TempDir()
		filename := fmt.Sprintf("agent_audio_%s.wav", aa.generateID())
		aa.path = filepath.Join(tempDir, filename)

		// Audio encoding would require additional libraries like go-audio or similar
		// For full implementation, we would need to:
		// 1. Convert tensor data to PCM samples
		// 2. Write WAV header with proper sample rate
		// 3. Save to file
		log.Printf("Audio tensor to file conversion not implemented - would require audio processing libraries")
		return aa.path
	}

	return ""
}

// String implements Stringer interface
func (aa *AgentAudio) String() string {
	return aa.ToString()
}

// SampleRate returns the sample rate of the audio
func (aa *AgentAudio) SampleRate() int {
	return aa.sampleRate
}

// generateID generates a unique identifier
func (aa *AgentAudio) generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// HandleAgentInputTypes converts AgentTypes to their raw values in function arguments
func HandleAgentInputTypes(args []interface{}, kwargs map[string]interface{}) ([]interface{}, map[string]interface{}) {
	// Convert args
	newArgs := make([]interface{}, len(args))
	for i, arg := range args {
		if agentType, ok := arg.(AgentType); ok {
			newArgs[i] = agentType.ToRaw()
		} else {
			newArgs[i] = arg
		}
	}

	// Convert kwargs
	newKwargs := make(map[string]interface{})
	for k, v := range kwargs {
		if agentType, ok := v.(AgentType); ok {
			newKwargs[k] = agentType.ToRaw()
		} else {
			newKwargs[k] = v
		}
	}

	return newArgs, newKwargs
}

// HandleAgentOutputTypes converts outputs to appropriate AgentType based on output type
func HandleAgentOutputTypes(output interface{}, outputType string) interface{} {
	// Type mapping based on specified output type
	switch outputType {
	case "string":
		if str, ok := output.(string); ok {
			return NewAgentText(str)
		}
	case "image":
		if img, err := NewAgentImage(output); err == nil {
			return img
		}
	case "audio":
		if audio, err := NewAgentAudio(output); err == nil {
			return audio
		}
	}

	// Auto-detect type if no specific output type specified
	switch v := output.(type) {
	case string:
		return NewAgentText(v)
	case image.Image:
		if img, err := NewAgentImage(v); err == nil {
			return img
		}
	case []byte:
		// Try to decode as image first
		if img, err := NewAgentImage(v); err == nil {
			return img
		}
	}

	// Return as-is if no conversion applies
	return output
}
