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

// Package smolagents provides a Go implementation of an AI agent framework.
//
// This library offers a comprehensive framework for building AI agents with
// advanced reasoning capabilities, leveraging Go's type system and conventions.
//
// The library provides:
// - Multi-step AI agents using the ReAct framework
// - Tool calling agents for function-based interactions
// - Code execution agents with sandboxed Go interpreter
// - Comprehensive tool ecosystem
// - Memory management for conversation history
// - Model abstractions for different LLM backends
// - Built-in monitoring and logging
package smolagents

// Re-export core types and interfaces from subpackages
import (
	"github.com/rizome-dev/go-smolagents/pkg/agent_types"
	"github.com/rizome-dev/go-smolagents/pkg/agents"
	"github.com/rizome-dev/go-smolagents/pkg/memory"
	"github.com/rizome-dev/go-smolagents/pkg/models"
	"github.com/rizome-dev/go-smolagents/pkg/monitoring"
	"github.com/rizome-dev/go-smolagents/pkg/tools"
	"github.com/rizome-dev/go-smolagents/pkg/utils"
)

const Version = "1.18.0"

// Agent types
type (
	AgentType  = agent_types.AgentType
	AgentText  = agent_types.AgentText
	AgentImage = agent_types.AgentImage
	AgentAudio = agent_types.AgentAudio
)

// Core agent interfaces and types
type (
	MultiStepAgent        = agents.MultiStepAgent
	BaseMultiStepAgent    = agents.BaseMultiStepAgent
	ReactCodeAgent        = agents.ReactCodeAgent
	ReactCodeAgentOptions = agents.ReactCodeAgentOptions
	RunResult             = agents.RunResult
	RunOptions            = agents.RunOptions
	PromptTemplates       = agents.PromptTemplates
	FinalOutput           = agents.FinalOutput
	AgentConfig           = agents.AgentConfig
	StreamStepResult      = agents.StreamStepResult
	StepCallback          = agents.StepCallback
)

// Tool system
type (
	Tool           = tools.Tool
	BaseTool       = tools.BaseTool
	ToolCollection = tools.ToolCollection
	ToolInput      = tools.ToolInput
)

// Default tools - types will be added when default_tools package is implemented
// type (
//	FinalAnswerTool       = default_tools.FinalAnswerTool
//	WebSearchTool         = default_tools.WebSearchTool
//	VisitWebpageTool      = default_tools.VisitWebpageTool
//	WikipediaSearchTool   = default_tools.WikipediaSearchTool
// )

// Memory system
type (
	AgentMemory      = memory.AgentMemory
	MemoryStep       = memory.MemoryStep
	ActionStep       = memory.ActionStep
	PlanningStep     = memory.PlanningStep
	TaskStep         = memory.TaskStep
	SystemPromptStep = memory.SystemPromptStep
	FinalAnswerStep  = memory.FinalAnswerStep
	Message          = memory.Message
	ToolCall         = memory.ToolCall
)

// Model system
type (
	Model                  = models.Model
	ChatMessage            = models.ChatMessage
	ChatMessageStreamDelta = models.ChatMessageStreamDelta
	ChatMessageToolCall    = models.ChatMessageToolCall
	MessageRole            = models.MessageRole
	GenerateOptions        = models.GenerateOptions
	ModelType              = models.ModelType
	InferenceClientModel   = models.InferenceClientModel
	OpenAIServerModel      = models.OpenAIServerModel
	AzureOpenAIServerModel = models.AzureOpenAIServerModel
	LiteLLMModel           = models.LiteLLMModel
	AmazonBedrockModel     = models.AmazonBedrockModel
	TransformersModel      = models.TransformersModel

	// Structured generation types
	ResponseFormat      = models.ResponseFormat
	JSONSchema          = models.JSONSchema
	StructuredOutput    = models.StructuredOutput
	SchemaValidator     = models.SchemaValidator
	StructuredGenerator = models.StructuredGenerator

	// Multimodal types
	MediaType         = models.MediaType
	MediaContent      = models.MediaContent
	ImageURL          = models.ImageURL
	AudioData         = models.AudioData
	VideoData         = models.VideoData
	MultimodalMessage = models.MultimodalMessage
	MultimodalSupport = models.MultimodalSupport
)

// Monitoring and logging
type (
	TokenUsage  = monitoring.TokenUsage
	Timing      = monitoring.Timing
	AgentLogger = monitoring.AgentLogger
	Monitor     = monitoring.Monitor
	LogLevel    = monitoring.LogLevel
)

// Error types
type (
	AgentError              = utils.AgentError
	AgentParsingError       = utils.AgentParsingError
	AgentExecutionError     = utils.AgentExecutionError
	AgentMaxStepsError      = utils.AgentMaxStepsError
	AgentToolCallError      = utils.AgentToolCallError
	AgentToolExecutionError = utils.AgentToolExecutionError
	AgentGenerationError    = utils.AgentGenerationError
)

// Constructor functions
var (
	// Agent constructors
	NewBaseMultiStepAgent        = agents.NewBaseMultiStepAgent
	NewReactCodeAgent            = agents.NewReactCodeAgent
	DefaultReactCodeAgentOptions = agents.DefaultReactCodeAgentOptions
	CreateAgent                  = agents.CreateAgent
	DefaultAgentConfig           = agents.DefaultAgentConfig

	// Tool constructors - will be added when tools package is complete
	// NewBaseTool            = tools.NewBaseTool
	// NewToolCollection      = tools.NewToolCollection

	// Default tool constructors - will be added when default_tools package is implemented
	// NewFinalAnswer         = default_tools.NewFinalAnswerTool
	// NewWebSearch           = default_tools.NewWebSearchTool
	// NewVisitWebpage        = default_tools.NewVisitWebpageTool
	// NewWikipediaSearch     = default_tools.NewWikipediaSearchTool

	// Memory constructors - will be added when memory package is complete
	// NewAgentMemory = memory.NewAgentMemory
	// NewMessage     = memory.NewMessage

	// Model constructors
	NewInferenceClientModel    = models.NewInferenceClientModel
	NewOpenAIServerModel       = models.NewOpenAIServerModel
	NewAzureOpenAIServerModel  = models.NewAzureOpenAIServerModel
	NewLiteLLMModel            = models.NewLiteLLMModel
	NewAmazonBedrockModel      = models.NewAmazonBedrockModel
	NewTransformersModel       = models.NewTransformersModelImpl
	CreateModel                = models.CreateModel
	AutoDetectModelType        = models.AutoDetectModelType
	ValidateModelConfiguration = models.ValidateModelConfiguration
	GetModelInfo               = models.GetModelInfo

	// Structured generation constructors
	NewSchemaValidator       = models.NewSchemaValidator
	NewStructuredGenerator   = models.NewStructuredGenerator
	CreateJSONSchema         = models.CreateJSONSchema
	CreateToolCallSchema     = models.CreateToolCallSchema
	CreateFunctionCallSchema = models.CreateFunctionCallSchema
	ParseStructuredOutput    = models.ParseStructuredOutput
	GenerateStructuredPrompt = models.GenerateStructuredPrompt

	// Multimodal constructors
	NewMultimodalSupport = models.NewMultimodalSupport
	LoadImage            = models.LoadImage
	LoadImageURL         = models.LoadImageURL
	LoadAudio            = models.LoadAudio
	LoadVideo            = models.LoadVideo
	CreateText           = models.CreateText
	CreateMessage        = models.CreateMessage

	// Monitoring constructors - will be added when monitoring package is complete
	// NewAgentLogger = monitoring.NewAgentLogger
	// NewMonitor     = monitoring.NewMonitor

	// Agent type constructors - will be added when agent_types package is complete
	// NewAgentText  = agent_types.NewAgentText
	// NewAgentImage = agent_types.NewAgentImage
	// NewAgentAudio = agent_types.NewAgentAudio
)

// Utility functions - will be added when packages are complete
// var (
//	HandleAgentInputTypes  = agent_types.HandleAgentInputTypes
//	HandleAgentOutputTypes = agent_types.HandleAgentOutputTypes
//	ParseCodeBlobs         = utils.ParseCodeBlobs
//	ExtractCodeFromText    = utils.ExtractCodeFromText
//	TruncateContent        = utils.TruncateContent
//	IsValidName            = utils.IsValidName
// )
