package memory

import (
	"testing"
	"time"
	
	"github.com/rizome-dev/go-smolagents/pkg/models"
)

func TestActionStep_ObservationImages(t *testing.T) {
	// Create test images
	img1, _ := models.LoadImageURL("https://example.com/image1.jpg", "auto")
	img2, _ := models.LoadImageURL("https://example.com/image2.jpg", "auto")
	images := []*models.MediaContent{img1, img2}
	
	// Create action step with images
	step := NewActionStepWithImages(1, time.Now(), images)
	
	// Verify images are stored
	if len(step.ObservationImages) != 2 {
		t.Errorf("Expected 2 images, got %d", len(step.ObservationImages))
	}
	
	// Test ToMessages with images
	messages, err := step.ToMessages(false)
	if err != nil {
		t.Fatalf("ToMessages failed: %v", err)
	}
	
	// Should have at least one message for images
	hasImageMessage := false
	for _, msg := range messages {
		if msg.Role == models.RoleUser {
			// Check if content has image data
			for _, content := range msg.Content {
				if content["type"] == "image_url" {
					hasImageMessage = true
					break
				}
			}
		}
	}
	
	if !hasImageMessage {
		t.Error("Expected image message in ToMessages output")
	}
}

func TestTaskStep_TaskImages(t *testing.T) {
	// Create test images
	img1, _ := models.LoadImageURL("https://example.com/task1.jpg", "auto")
	img2, _ := models.LoadImageURL("https://example.com/task2.jpg", "auto")
	images := []*models.MediaContent{img1, img2}
	
	// Create task step with images
	step := NewTaskStep("Analyze these images", images)
	
	// Verify images are stored
	if len(step.TaskImages) != 2 {
		t.Errorf("Expected 2 images, got %d", len(step.TaskImages))
	}
	
	// Test ToMessages with images
	messages, err := step.ToMessages(false)
	if err != nil {
		t.Fatalf("ToMessages failed: %v", err)
	}
	
	// Should have exactly one message with multimodal content
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	
	msg := messages[0]
	if msg.Role != models.RoleUser {
		t.Errorf("Expected user role, got %s", msg.Role)
	}
	
	// Check content structure
	hasTextContent := false
	imageCount := 0
	for _, content := range msg.Content {
		if content["type"] == "text" {
			hasTextContent = true
		} else if content["type"] == "image_url" {
			imageCount++
		}
	}
	
	if !hasTextContent {
		t.Error("Expected text content in message")
	}
	if imageCount != 2 {
		t.Errorf("Expected 2 image contents, got %d", imageCount)
	}
}

func TestTaskStep_NoImages(t *testing.T) {
	// Create task step without images
	step := NewTaskStep("Simple text task")
	
	// Verify no images
	if len(step.TaskImages) != 0 {
		t.Errorf("Expected 0 images, got %d", len(step.TaskImages))
	}
	
	// Test ToMessages without images
	messages, err := step.ToMessages(false)
	if err != nil {
		t.Fatalf("ToMessages failed: %v", err)
	}
	
	// Should have exactly one message with simple text content
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	
	msg := messages[0]
	if len(msg.Content) == 0 {
		t.Error("Expected non-empty content")
	}
}