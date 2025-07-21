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

package agent_types

import (
	"image"
	"testing"
)

func TestAgentImage_TensorToImage(t *testing.T) {
	tests := []struct {
		name   string
		tensor interface{}
		want   bool // whether conversion should succeed
	}{
		{
			name: "float32 tensor",
			tensor: [][]float32{
				{0.0, 0.5, 1.0},
				{0.25, 0.75, 0.5},
				{1.0, 0.5, 0.0},
			},
			want: true,
		},
		{
			name: "float64 tensor",
			tensor: [][]float64{
				{0.0, 0.5, 1.0},
				{0.25, 0.75, 0.5},
				{1.0, 0.5, 0.0},
			},
			want: true,
		},
		{
			name:   "unsupported tensor type",
			tensor: [][]int{{1, 2, 3}, {4, 5, 6}},
			want:   false,
		},
		{
			name:   "nil tensor",
			tensor: nil,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ai := &AgentImage{tensor: tt.tensor}
			img := ai.ToRaw()

			if tt.want {
				if img == nil {
					t.Error("Expected image but got nil")
				} else if _, ok := img.(image.Image); !ok {
					t.Errorf("Expected image.Image but got %T", img)
				}
			} else {
				if img != nil {
					t.Errorf("Expected nil but got %T", img)
				}
			}
		})
	}
}

func TestAgentImage_TensorToImage_PixelValues(t *testing.T) {
	// Test that pixel values are converted correctly
	// Following Python implementation: (255 - array * 255)
	tensor := [][]float32{
		{0.0, 1.0},  // Should become 255, 0
		{0.5, 0.25}, // Should become 127, 191
	}

	ai := &AgentImage{tensor: tensor}
	rawImg := ai.ToRaw()

	img, ok := rawImg.(*image.Gray)
	if !ok {
		t.Fatalf("Expected *image.Gray but got %T", rawImg)
	}

	// Check dimensions
	bounds := img.Bounds()
	if bounds.Dx() != 2 || bounds.Dy() != 2 {
		t.Errorf("Expected 2x2 image but got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// Check pixel values
	testCases := []struct {
		x, y     int
		expected uint8
	}{
		{0, 0, 255}, // 0.0 -> 255
		{1, 0, 0},   // 1.0 -> 0
		{0, 1, 127}, // 0.5 -> 127 (approximately)
		{1, 1, 191}, // 0.25 -> 191 (approximately)
	}

	for _, tc := range testCases {
		actual := img.GrayAt(tc.x, tc.y).Y
		// Allow small rounding differences
		diff := int(actual) - int(tc.expected)
		if diff < -1 || diff > 1 {
			t.Errorf("Pixel at (%d,%d): expected %d, got %d", tc.x, tc.y, tc.expected, actual)
		}
	}
}

func TestAgentImage_ToString(t *testing.T) {
	// Test tensor to string conversion
	tensor := [][]float32{{0.5}}
	ai := &AgentImage{tensor: tensor}

	path := ai.ToString()
	if path == "" {
		t.Error("Expected non-empty path")
	}

	// Should have converted tensor to image and saved it
	if ai.rawImg == nil {
		t.Error("Expected rawImg to be set after ToString()")
	}
}

func TestAgentAudio_Creation(t *testing.T) {
	// Test audio creation with different inputs
	tests := []struct {
		name    string
		value   interface{}
		wantErr bool
	}{
		{
			name:    "from path",
			value:   "/path/to/audio.wav",
			wantErr: false,
		},
		{
			name:    "from tensor with sample rate",
			value:   [2]interface{}{44100, []float32{0.1, 0.2, 0.3}},
			wantErr: false,
		},
		{
			name:    "unsupported type",
			value:   123,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aa, err := NewAgentAudio(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAgentAudio() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && aa == nil {
				t.Error("Expected non-nil AgentAudio")
			}
		})
	}
}
