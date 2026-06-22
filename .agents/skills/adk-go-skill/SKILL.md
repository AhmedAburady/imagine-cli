---
name: adk-go-skill
description: |
  Build AI agents with Google's Agent Development Kit (ADK) for Go. Use for: creating LLM agents with Gemini, building custom tools, implementing multi-agent workflows (sequential, parallel, loop), MCP integration, A2A communication, REST APIs, human-in-the-loop confirmation, sessions, memory, and cloud deployment. Triggers: ADK, Agent Development Kit, Go agent, AI agent Go, Gemini agent, multi-agent Go, agent orchestration, agentic workflow Go.
---

# Google ADK-Go Agent Development

Build sophisticated AI agents in Go using Google's Agent Development Kit.

## Quick Start

```go
package main

import (
    "context"
    "log"
    "os"

    "google.golang.org/genai"
    "google.golang.org/adk/agent"
    "google.golang.org/adk/agent/llmagent"
    "google.golang.org/adk/cmd/launcher"
    "google.golang.org/adk/cmd/launcher/full"
    "google.golang.org/adk/model/gemini"
    "google.golang.org/adk/tool"
    "google.golang.org/adk/tool/geminitool"
)

func main() {
    ctx := context.Background()

    // Create Gemini model
    model, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{
        APIKey: os.Getenv("GOOGLE_API_KEY"),
    })
    if err != nil {
        log.Fatalf("Failed to create model: %v", err)
    }

    // Create agent
    a, err := llmagent.New(llmagent.Config{
        Name:        "my_agent",
        Model:       model,
        Description: "A helpful assistant.",
        Instruction: "You are a helpful assistant.",
        Tools: []tool.Tool{
            geminitool.GoogleSearch{},
        },
    })
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }

    // Launch agent
    config := &launcher.Config{
        AgentLoader: agent.NewSingleLoader(a),
    }
    l := full.NewLauncher()
    if err = l.Execute(ctx, config, os.Args[1:]); err != nil {
        log.Fatalf("Run failed: %v", err)
    }
}
```

**Installation**: `go get google.golang.org/adk`

**Environment**: Set `GOOGLE_API_KEY` for Gemini.

## Core Concepts

### Agent Interface

All agents implement this interface:

```go
type Agent interface {
    Name() string
    Description() string
    Run(InvocationContext) iter.Seq2[*session.Event, error]
    SubAgents() []Agent
}
```

### LLM Agent Config

```go
llmagent.Config{
    Name:                  string,           // Required: unique name
    Description:           string,           // For LLM routing decisions
    Model:                 model.LLM,        // Required: LLM instance
    Instruction:           string,           // System prompt (supports {var} placeholders)
    InstructionProvider:   func(ctx) string, // Dynamic instructions
    GlobalInstruction:     string,           // Shared across agent tree
    SubAgents:             []agent.Agent,    // Child agents for delegation
    Tools:                 []tool.Tool,      // Available tools
    Toolsets:              []tool.Toolset,   // Tool collections
    GenerateContentConfig: *genai.GenerateContentConfig, // Model settings
    InputSchema:           *genai.Schema,    // Input validation
    OutputSchema:          *genai.Schema,    // Structured output (disables tools)
    OutputKey:             string,           // Save output to state key
    IncludeContents:       IncludeContents,  // History mode

    // Callbacks
    BeforeAgentCallbacks:  []agent.BeforeAgentCallback,
    AfterAgentCallbacks:   []agent.AfterAgentCallback,
    BeforeModelCallbacks:  []BeforeModelCallback,
    AfterModelCallbacks:   []AfterModelCallback,
    OnModelErrorCallbacks: []OnModelErrorCallback,
    BeforeToolCallbacks:   []BeforeToolCallback,
    AfterToolCallbacks:    []AfterToolCallback,
    OnToolErrorCallbacks:  []OnToolErrorCallback,
}
```

## Creating Tools

### Function Tool

Wrap Go functions as agent tools:

```go
import "google.golang.org/adk/tool/functiontool"

type WeatherInput struct {
    City string `json:"city" jsonschema:"city name to check weather"`
}

type WeatherOutput struct {
    Temperature float64 `json:"temperature"`
    Conditions  string  `json:"conditions"`
}

func getWeather(ctx tool.Context, input WeatherInput) (WeatherOutput, error) {
    // Implementation
    return WeatherOutput{Temperature: 72.5, Conditions: "Sunny"}, nil
}

weatherTool, err := functiontool.New(functiontool.Config{
    Name:        "get_weather",
    Description: "Get weather for a city",
}, getWeather)
```

### Function Tool with Confirmation (HITL)

```go
// Static confirmation
tool, err := functiontool.New(functiontool.Config{
    Name:                "delete_file",
    Description:         "Delete a file",
    RequireConfirmation: true,
}, handler)

// Dynamic confirmation
tool, err := functiontool.New(functiontool.Config{
    Name:        "transfer_money",
    Description: "Transfer money",
    RequireConfirmationProvider: func(args TransferArgs) bool {
        return args.Amount > 1000
    },
}, handler)
```

### Manual Confirmation Flow

```go
func sensitiveOperation(ctx tool.Context, args Args) (Result, error) {
    confirmation := ctx.ToolConfirmation()
    if confirmation == nil {
        // Request confirmation
        ctx.RequestConfirmation("Please approve this action", payload)
        return Result{Status: "Pending approval"}, nil
    }

    if !confirmation.Confirmed {
        return Result{}, fmt.Errorf("operation rejected")
    }

    // Proceed with confirmed operation
    return doOperation(args)
}
```

### Built-in Tools

```go
import "google.golang.org/adk/tool/geminitool"

// Google Search (grounded search)
geminitool.GoogleSearch{}
```

### Agent as Tool

Use agents as callable tools:

```go
import "google.golang.org/adk/tool/agenttool"

searchAgent, _ := llmagent.New(llmagent.Config{
    Name:        "search_agent",
    Description: "Performs web searches",
    Tools:       []tool.Tool{geminitool.GoogleSearch{}},
    // ...
})

mainAgent, _ := llmagent.New(llmagent.Config{
    Name: "main_agent",
    Tools: []tool.Tool{
        agenttool.New(searchAgent, nil),
    },
})
```

## Workflow Agents

### Sequential Agent

Execute sub-agents in order:

```go
import "google.golang.org/adk/agent/workflowagents/sequentialagent"

seqAgent, err := sequentialagent.New(sequentialagent.Config{
    AgentConfig: agent.Config{
        Name:        "pipeline",
        Description: "Process data in sequence",
        SubAgents:   []agent.Agent{step1Agent, step2Agent, step3Agent},
    },
})
```

### Parallel Agent

Execute sub-agents concurrently:

```go
import "google.golang.org/adk/agent/workflowagents/parallelagent"

parallelAgent, err := parallelagent.New(parallelagent.Config{
    AgentConfig: agent.Config{
        Name:        "concurrent_processor",
        Description: "Process multiple tasks simultaneously",
        SubAgents:   []agent.Agent{taskA, taskB, taskC},
    },
})
```

### Loop Agent

Repeat until condition or max iterations:

```go
import "google.golang.org/adk/agent/workflowagents/loopagent"

loopAgent, err := loopagent.New(loopagent.Config{
    AgentConfig: agent.Config{
        Name:      "refiner",
        SubAgents: []agent.Agent{reviewAgent, reviseAgent},
    },
    MaxIterations: 3, // 0 = infinite until escalation
})
```

Exit loop via escalation in sub-agent or use `exitlooptool`.

## MCP Integration

### In-Memory MCP Server

```go
import (
    "github.com/modelcontextprotocol/go-sdk/mcp"
    "google.golang.org/adk/tool/mcptoolset"
)

// Create in-memory transport
clientTransport, serverTransport := mcp.NewInMemoryTransports()

// Setup MCP server
server := mcp.NewServer(&mcp.Implementation{Name: "my_server", Version: "v1.0.0"}, nil)
mcp.AddTool(server, &mcp.Tool{Name: "my_tool", Description: "..."}, handler)
server.Connect(ctx, serverTransport, nil)

// Create toolset from transport
mcpToolSet, err := mcptoolset.New(mcptoolset.Config{
    Transport: clientTransport,
})

// Use in agent
agent, err := llmagent.New(llmagent.Config{
    Toolsets: []tool.Toolset{mcpToolSet},
})
```

### Remote MCP Server

```go
import "golang.org/x/oauth2"

// HTTP transport with auth
transport := &mcp.StreamableClientTransport{
    Endpoint:   "https://api.example.com/mcp/",
    HTTPClient: oauth2.NewClient(ctx, tokenSource),
}

mcpToolSet, err := mcptoolset.New(mcptoolset.Config{
    Transport: transport,
})
```

## Session & State

### Session Service

```go
import "google.golang.org/adk/session"

// In-memory (development)
sessionService := session.InMemoryService()

// Create session
resp, err := sessionService.Create(ctx, &session.CreateRequest{
    AppName: "my_app",
    UserID:  "user123",
})
sessionID := resp.Session.ID()
```

### State Scopes

```go
// State key prefixes
session.KeyPrefixApp   // "app:"  - shared across all users/sessions
session.KeyPrefixUser  // "user:" - per user, across sessions
session.KeyPrefixTemp  // "temp:" - current invocation only

// Access in tools/callbacks
state := ctx.ReadonlyState()
val, err := state.Get("user:preference")

// Write state
ctx.State().Set("temp:working_data", data)
```

### Instruction Templating

```go
// Use {key} placeholders resolved from state
llmagent.Config{
    Instruction: "You are helping {user:name}. Their preference is {user:pref}.",
    // Use {var?} for optional variables
    Instruction: "Context: {context?}",
}
```

## Callbacks

### Agent Callbacks

```go
// Before agent runs (can short-circuit)
func beforeAgent(ctx agent.CallbackContext) (*genai.Content, error) {
    // Return content to skip agent, nil to continue
    return nil, nil
}

// After agent completes
func afterAgent(ctx agent.CallbackContext) (*genai.Content, error) {
    // Can modify state, add response
    return nil, nil
}
```

### Model Callbacks

```go
// Inspect/modify request, implement caching
func beforeModel(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
    // Return response to skip model call
    return nil, nil
}

// Post-process response
func afterModel(ctx agent.CallbackContext, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
    // Log, modify, replace response
    return resp, nil
}
```

### Tool Callbacks

```go
// Before tool execution
func beforeTool(ctx tool.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
    // Modify args in place, return result to skip tool
    return nil, nil
}

// After tool execution
func afterTool(ctx tool.Context, t tool.Tool, args, result map[string]any, err error) (map[string]any, error) {
    // Log, modify result
    return result, nil
}
```

## Deployment

### REST API Server

```go
import (
    "net/http"
    "google.golang.org/adk/server/adkrest"
)

config := &launcher.Config{
    AgentLoader:    agent.NewSingleLoader(myAgent),
    SessionService: session.InMemoryService(),
}

apiHandler := adkrest.NewHandler(config, 120*time.Second)

mux := http.NewServeMux()
mux.Handle("/api/", http.StripPrefix("/api", apiHandler))
http.ListenAndServe(":8080", mux)
```

**REST Endpoints**:
- `POST /apps/{appName}/users/{userId}/sessions/{sessionId}` - Create session
- `POST /run` - Execute agent with message

### A2A (Agent-to-Agent)

```go
import (
    "google.golang.org/adk/server/adka2a"
    "google.golang.org/adk/agent/remoteagent"
    "github.com/a2aproject/a2a-go/a2a"
    "github.com/a2aproject/a2a-go/a2asrv"
)

// Expose agent via A2A
agentCard := &a2a.AgentCard{
    Name:               agent.Name(),
    Skills:             adka2a.BuildAgentSkills(agent),
    PreferredTransport: a2a.TransportProtocolJSONRPC,
    URL:                "http://localhost:8080/invoke",
    Capabilities:       a2a.AgentCapabilities{Streaming: true},
}

executor := adka2a.NewExecutor(adka2a.ExecutorConfig{
    RunnerConfig: runner.Config{
        AppName:        agent.Name(),
        Agent:          agent,
        SessionService: session.InMemoryService(),
    },
})

mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(agentCard))
mux.Handle("/invoke", a2asrv.NewJSONRPCHandler(a2asrv.NewHandler(executor)))
```

**Connect to Remote Agent**:

```go
remoteAgent, err := remoteagent.NewA2A(remoteagent.A2AConfig{
    Name:            "Remote Weather Agent",
    AgentCardSource: "http://remote-server:8080",
})
```

### Launcher Modes

```go
import "google.golang.org/adk/cmd/launcher/full"

// full.NewLauncher() supports:
// - console: Interactive CLI
// - restapi: REST API server
// - a2a: Agent-to-Agent protocol
// - webui: Development web interface

l := full.NewLauncher()
l.Execute(ctx, config, os.Args[1:])

// Run: go run main.go console
// Run: go run main.go restapi --port 8080
// Run: go run main.go webui
```

## Memory

```go
import "google.golang.org/adk/memory"

// In-memory implementation
memService := memory.NewInMemory()

// Add session to memory
memService.AddSession(ctx, session)

// Search across sessions
results, err := memService.Search(ctx, "previous conversation about weather")
```

## Artifacts

```go
// Save artifact
ctx.Artifacts().Save(ctx, "output.json", &genai.Part{Text: jsonData})

// List artifacts
list, _ := ctx.Artifacts().List(ctx)

// Load artifact
artifact, _ := ctx.Artifacts().Load(ctx, "output.json")

// Load specific version
artifact, _ := ctx.Artifacts().LoadVersion(ctx, "output.json", 2)
```

## Best Practices

1. **Tool Design**: Keep tools focused; use typed structs for inputs/outputs with jsonschema tags
2. **Error Handling**: Return descriptive errors; tools should fail gracefully
3. **Instruction Templating**: Use `{var}` for dynamic values, `{var?}` for optional
4. **Multi-Agent**: Use agenttool for agent-as-tool pattern; keep specialized agents focused
5. **State Scoping**: Use appropriate prefix (`app:`, `user:`, `temp:`) for data lifecycle
6. **Callbacks**: Use for logging, caching, validation; avoid heavy computation
7. **Confirmation**: Require HITL for destructive/expensive operations
8. **Testing**: Mock model with BeforeModelCallback; test tools independently

## Reference Files

- **patterns.md**: Common agent patterns and architectures
- **api_reference.md**: Complete API documentation
