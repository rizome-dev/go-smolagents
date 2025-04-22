// Package smolagentsgo is a Go implementation of the Python smolagents library,
// providing a simple framework for building powerful AI agents.
//
// This library offers:
// - Simplicity: the logic for agents fits in a minimal number of lines of code
// - Support for any LLM: works with various LLM backends
// - First-class support for Code Agents
// - Clean abstractions: minimal abstractions above raw Go code
package smolagentsgo

import (
	"github.com/rizome-dev/smolagentsgo/agent_types"
	"github.com/rizome-dev/smolagentsgo/agents"
	"github.com/rizome-dev/smolagentsgo/memory"
	"github.com/rizome-dev/smolagentsgo/models"
	"github.com/rizome-dev/smolagentsgo/tools"
	"github.com/rizome-dev/smolagentsgo/utils"
)

// Version of the smolagentsgo library
const Version = "0.1.0"

// Export agent types
type AgentType = agent_types.AgentType
type AgentText = agent_types.AgentText
type AgentImage = agent_types.AgentImage
type AgentAudio = agent_types.AgentAudio

// Export agent functions
var HandleAgentOutputTypes = agent_types.HandleAgentOutputTypes
var HandleAgentInputTypes = agent_types.HandleAgentInputTypes
var EncodeImageBase64 = agent_types.EncodeImageBase64
var MakeImageURL = agent_types.MakeImageURL
var DecodeImageBase64 = agent_types.DecodeImageBase64

// Export agent constructors
var NewAgentText = agent_types.NewAgentText
var NewAgentImageFromImage = agent_types.NewAgentImageFromImage
var NewAgentImageFromPath = agent_types.NewAgentImageFromPath
var NewAgentImageFromBytes = agent_types.NewAgentImageFromBytes
var NewAgentAudioFromBytes = agent_types.NewAgentAudioFromBytes
var NewAgentAudioFromPath = agent_types.NewAgentAudioFromPath

// Export agents
type MultiStepAgent = agents.MultiStepAgent
type BaseMultiStepAgent = agents.BaseMultiStepAgent
type ToolCallingAgent = agents.ToolCallingAgent
type CodeAgent = agents.CodeAgent
type ManagedAgent = agents.ManagedAgent
type ManagedAgentImpl = agents.ManagedAgentImpl
type PythonExecutor = agents.PythonExecutor

// Export agent constructors
var NewBaseMultiStepAgent = agents.NewBaseMultiStepAgent
var NewToolCallingAgent = agents.NewToolCallingAgent
var NewCodeAgent = agents.NewCodeAgent
var NewManagedAgent = agents.NewManagedAgent

// Export prompt templates
type PromptTemplates = agents.PromptTemplates

var EmptyPromptTemplates = agents.EmptyPromptTemplates
var PopulateTemplate = agents.PopulateTemplate

// Export callback types
type ModelFunc = agents.ModelFunc
type RunCallback = agents.RunCallback
type FinalAnswerCheck = agents.FinalAnswerCheck

// Export tools
type Tool = tools.Tool
type BaseTool = tools.BaseTool
type FinalAnswerTool = tools.FinalAnswerTool
type InputProperty = tools.InputProperty

// Export tool constructors
var NewBaseTool = tools.NewBaseTool
var NewFinalAnswerTool = tools.NewFinalAnswerTool
var GetToolJSONSchema = tools.GetToolJSONSchema

// Export memory types
type AgentMemory = memory.AgentMemory
type ActionStep = memory.ActionStep
type PlanningStep = memory.PlanningStep
type TaskStep = memory.TaskStep
type SystemPromptStep = memory.SystemPromptStep
type FinalAnswerStep = memory.FinalAnswerStep
type ToolCall = memory.ToolCall
type MemoryStep = memory.MemoryStep

// Export memory constructors
var NewAgentMemory = memory.NewAgentMemory

// Export models
type Model = models.Model
type ChatMessage = models.ChatMessage
type MessageRole = models.MessageRole
type Message = models.Message
type MessageContent = models.MessageContent
type ChatMessageToolCall = models.ChatMessageToolCall
type ChatMessageToolCallDefinition = models.ChatMessageToolCallDefinition

// Export model functions
var RemoveStopSequences = models.RemoveStopSequences
var GetCleanMessageList = models.GetCleanMessageList
var GetToolCallFromText = models.GetToolCallFromText
var ParseJSONIfNeeded = models.ParseJSONIfNeeded
var SupportsStopParameter = models.SupportsStopParameter

// Export utils
var IsValidName = utils.IsValidName
var MakeJSONSerializable = utils.MakeJSONSerializable
var ParseCodeBlobs = utils.ParseCodeBlobs
var TruncateContent = utils.TruncateContent
var ParseJSONBlob = utils.ParseJSONBlob

// Export error types
type AgentError = utils.AgentError
type AgentGenerationError = utils.AgentGenerationError
type AgentExecutionError = utils.AgentExecutionError
type AgentParsingError = utils.AgentParsingError
type AgentToolCallError = utils.AgentToolCallError
type AgentToolExecutionError = utils.AgentToolExecutionError
type AgentMaxStepsError = utils.AgentMaxStepsError

// Export error constructors
var NewAgentGenerationError = utils.NewAgentGenerationError
var NewAgentExecutionError = utils.NewAgentExecutionError
var NewAgentParsingError = utils.NewAgentParsingError
var NewAgentToolCallError = utils.NewAgentToolCallError
var NewAgentToolExecutionError = utils.NewAgentToolExecutionError
var NewAgentMaxStepsError = utils.NewAgentMaxStepsError
