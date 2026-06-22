# ADK-Go API Reference

Complete API documentation for Google's Agent Development Kit for Go.

## Table of Contents

1. [Package Structure](#package-structure)
2. [agent Package](#agent-package)
3. [llmagent Package](#llmagent-package)
4. [tool Package](#tool-package)
5. [functiontool Package](#functiontool-package)
6. [session Package](#session-package)
7. [runner Package](#runner-package)
8. [model Package](#model-package)
9. [Workflow Agents](#workflow-agents)
10. [Server Packages](#server-packages)

## Package Structure

```
google.golang.org/adk/
├── agent/                    # Core agent interfaces
│   ├── llmagent/            # LLM-powered agents
│   ├── remoteagent/         # Remote agent connections
│   └── workflowagents/      # Orchestration agents
│       ├── sequentialagent/
│       ├── parallelagent/
│       └── loopagent/
├── tool/                     # Tool interfaces
│   ├── functiontool/        # Function wrappers
│   ├── geminitool/          # Gemini built-in tools
│   ├── agenttool/           # Agent-as-tool
│   ├── mcptoolset/          # MCP integration
│   ├── exitlooptool/        # Loop control
│   ├── loadartifactstool/   # Artifact loading
│   └── toolconfirmation/    # HITL confirmation
├── model/                    # LLM interfaces
│   └── gemini/              # Gemini implementation
├── session/                  # Session management
│   ├── inmemory.go
│   ├── database/
│   └── vertexai/
├── memory/                   # Cross-session memory
├── artifact/                 # Artifact storage
├── runner/                   # Agent execution
├── server/                   # HTTP servers
│   ├── adkrest/             # REST API
│   └── adka2a/              # A2A protocol
└── cmd/launcher/            # CLI launchers
    ├── full/
    └── prod/
```

## agent Package

`google.golang.org/adk/agent`

### Types

#### Agent Interface

```go
type Agent interface {
    // Name returns the unique identifier of the agent
    Name() string

    // Description returns a brief description used for routing
    Description() string

    // Run executes the agent with the given context
    // Returns an iterator yielding events and errors
    Run(InvocationContext) iter.Seq2[*session.Event, error]

    // SubAgents returns child agents for delegation
    SubAgents() []Agent
}
```

#### Config

```go
type Config struct {
    // Name must be unique within agent tree, cannot be "user"
    Name string

    // Description for LLM routing decisions (one line preferred)
    Description string

    // SubAgents for task delegation
    SubAgents []Agent

    // BeforeAgentCallbacks execute before agent run
    // Return non-nil content/error to skip agent
    BeforeAgentCallbacks []BeforeAgentCallback

    // Run defines custom agent behavior
    Run func(InvocationContext) iter.Seq2[*session.Event, error]

    // AfterAgentCallbacks execute after agent run
    AfterAgentCallbacks []AfterAgentCallback
}
```

#### InvocationContext Interface

```go
type InvocationContext interface {
    context.Context

    // Agent returns the current agent
    Agent() Agent

    // Artifacts provides artifact storage access
    Artifacts() Artifacts

    // Memory provides cross-session memory access
    Memory() Memory

    // Session returns the current session
    Session() session.Session

    // InvocationID returns unique invocation identifier
    InvocationID() string

    // Branch returns the current branch path (agent_1.agent_2.agent_3)
    Branch() string

    // UserContent returns the user's input content
    UserContent() *genai.Content

    // RunConfig returns the run configuration
    RunConfig() *RunConfig

    // EndInvocation signals to end the invocation
    EndInvocation()

    // Ended returns whether invocation has ended
    Ended() bool
}
```

#### CallbackContext Interface

```go
type CallbackContext interface {
    context.Context

    // AgentName returns current agent's name
    AgentName() string

    // ReadonlyState provides read access to session state
    ReadonlyState() session.ReadonlyState

    // State provides read/write access to session state
    State() session.State

    // Artifacts provides artifact access
    Artifacts() Artifacts

    // InvocationID returns invocation identifier
    InvocationID() string

    // UserContent returns user input
    UserContent() *genai.Content

    // AppName returns application name
    AppName() string

    // Branch returns branch path
    Branch() string

    // SessionID returns session identifier
    SessionID() string

    // UserID returns user identifier
    UserID() string
}
```

#### RunConfig

```go
type RunConfig struct {
    // StreamingMode controls response streaming
    StreamingMode StreamingMode

    // SaveInputBlobsAsArtifacts saves input blobs as artifacts
    SaveInputBlobsAsArtifacts bool
}

type StreamingMode string

const (
    StreamingModeNone StreamingMode = "none"
    StreamingModeSSE  StreamingMode = "sse"
)
```

### Functions

```go
// New creates a custom agent
func New(cfg Config) (Agent, error)

// NewSingleLoader creates a loader for a single agent
func NewSingleLoader(agent Agent) Loader

// NewMultiLoader creates a loader for multiple agents
func NewMultiLoader(agents ...Agent) (Loader, error)
```

### Callback Types

```go
// BeforeAgentCallback executes before agent run
// Return non-nil content to skip agent
type BeforeAgentCallback func(CallbackContext) (*genai.Content, error)

// AfterAgentCallback executes after agent run
type AfterAgentCallback func(CallbackContext) (*genai.Content, error)
```

## llmagent Package

`google.golang.org/adk/agent/llmagent`

### Config

```go
type Config struct {
    // Required fields
    Name  string
    Model model.LLM

    // Description for routing
    Description string

    // Sub-agents
    SubAgents []agent.Agent

    // Instructions
    Instruction           string              // Static instruction (supports {var} placeholders)
    InstructionProvider   InstructionProvider // Dynamic instruction
    GlobalInstruction     string              // Tree-wide instruction (root only)
    GlobalInstructionProvider InstructionProvider

    // Tools
    Tools    []tool.Tool
    Toolsets []tool.Toolset

    // Model configuration
    GenerateContentConfig *genai.GenerateContentConfig

    // Schema constraints
    InputSchema  *genai.Schema // Input validation when used as tool
    OutputSchema *genai.Schema // Structured output (disables tools)

    // State
    OutputKey       string          // Save output to this state key
    IncludeContents IncludeContents // History inclusion mode

    // Transfer controls
    DisallowTransferToParent bool
    DisallowTransferToPeers  bool

    // Agent callbacks
    BeforeAgentCallbacks []agent.BeforeAgentCallback
    AfterAgentCallbacks  []agent.AfterAgentCallback

    // Model callbacks
    BeforeModelCallbacks  []BeforeModelCallback
    AfterModelCallbacks   []AfterModelCallback
    OnModelErrorCallbacks []OnModelErrorCallback

    // Tool callbacks
    BeforeToolCallbacks  []BeforeToolCallback
    AfterToolCallbacks   []AfterToolCallback
    OnToolErrorCallbacks []OnToolErrorCallback
}
```

### Callback Types

```go
// BeforeModelCallback - return response to skip model call
type BeforeModelCallback func(
    ctx agent.CallbackContext,
    req *model.LLMRequest,
) (*model.LLMResponse, error)

// AfterModelCallback - modify response
type AfterModelCallback func(
    ctx agent.CallbackContext,
    resp *model.LLMResponse,
    err error,
) (*model.LLMResponse, error)

// OnModelErrorCallback - handle model errors
type OnModelErrorCallback func(
    ctx agent.CallbackContext,
    req *model.LLMRequest,
    err error,
) (*model.LLMResponse, error)

// BeforeToolCallback - modify args or skip tool
type BeforeToolCallback func(
    ctx tool.Context,
    t tool.Tool,
    args map[string]any,
) (map[string]any, error)

// AfterToolCallback - modify result
type AfterToolCallback func(
    ctx tool.Context,
    t tool.Tool,
    args, result map[string]any,
    err error,
) (map[string]any, error)

// OnToolErrorCallback - handle tool errors
type OnToolErrorCallback func(
    ctx tool.Context,
    t tool.Tool,
    args map[string]any,
    err error,
) (map[string]any, error)

// InstructionProvider - dynamic instructions
type InstructionProvider func(ctx agent.ReadonlyContext) (string, error)
```

### IncludeContents

```go
type IncludeContents string

const (
    IncludeContentsNone    IncludeContents = "none"    // No history
    IncludeContentsDefault IncludeContents = "default" // Include history
)
```

### Functions

```go
// New creates an LLM agent
func New(cfg Config) (agent.Agent, error)
```

## tool Package

`google.golang.org/adk/tool`

### Types

#### Tool Interface

```go
type Tool interface {
    // Name returns tool identifier
    Name() string

    // Description returns human-readable description
    Description() string

    // IsLongRunning indicates async operations
    IsLongRunning() bool
}
```

#### Context Interface

```go
type Context interface {
    agent.CallbackContext

    // FunctionCallID returns unique call identifier
    FunctionCallID() string

    // Actions returns event actions for state/transfer
    Actions() *session.EventActions

    // SearchMemory performs semantic memory search
    SearchMemory(context.Context, string) (*memory.SearchResponse, error)

    // ToolConfirmation returns confirmation status
    ToolConfirmation() *toolconfirmation.ToolConfirmation

    // RequestConfirmation initiates HITL approval
    RequestConfirmation(hint string, payload any) error
}
```

#### Toolset Interface

```go
type Toolset interface {
    // Name returns toolset identifier
    Name() string

    // Tools returns available tools based on context
    Tools(ctx agent.ReadonlyContext) ([]Tool, error)
}
```

### Functions

```go
// Predicate filters tools
type Predicate func(ctx agent.ReadonlyContext, tool Tool) bool

// StringPredicate creates predicate from allowed names
func StringPredicate(allowedTools []string) Predicate

// FilterToolset wraps toolset with predicate
func FilterToolset(toolset Toolset, predicate Predicate) Toolset
```

## functiontool Package

`google.golang.org/adk/tool/functiontool`

### Types

#### Config

```go
type Config struct {
    // Name of the tool
    Name string

    // Description for LLM
    Description string

    // InputSchema - optional, auto-inferred from handler
    InputSchema *jsonschema.Schema

    // OutputSchema - optional, auto-inferred from handler
    OutputSchema *jsonschema.Schema

    // IsLongRunning marks async operations
    IsLongRunning bool

    // RequireConfirmation always requires HITL
    RequireConfirmation bool

    // RequireConfirmationProvider for conditional HITL
    // Signature: func(input TArgs) bool
    RequireConfirmationProvider any
}
```

#### Func Type

```go
// Func is a Go function that can be wrapped as a tool
type Func[TArgs, TResults any] func(tool.Context, TArgs) (TResults, error)
```

### Functions

```go
// New creates a function tool
func New[TArgs, TResults any](cfg Config, handler Func[TArgs, TResults]) (tool.Tool, error)
```

### Input/Output Requirements

- Input type must be a struct or map (or pointer to these)
- JSON schema automatically inferred from Go types
- Use `json` and `jsonschema` struct tags for customization:

```go
type Input struct {
    Name     string  `json:"name" jsonschema:"required,description=The name to use"`
    Count    int     `json:"count" jsonschema:"minimum=1,maximum=100"`
    Optional string  `json:"optional,omitempty"`
}
```

## session Package

`google.golang.org/adk/session`

### Types

#### Session Interface

```go
type Session interface {
    // ID returns unique session identifier
    ID() string

    // AppName returns application name
    AppName() string

    // UserID returns user identifier
    UserID() string

    // State returns session state
    State() State

    // Events returns session events
    Events() Events

    // LastUpdateTime returns last update timestamp
    LastUpdateTime() time.Time
}
```

#### State Interface

```go
type State interface {
    // Get retrieves value by key
    // Returns ErrStateKeyNotExist if not found
    Get(string) (any, error)

    // Set stores value by key
    Set(string, any) error

    // All returns iterator over all key-value pairs
    All() iter.Seq2[string, any]
}

type ReadonlyState interface {
    Get(string) (any, error)
    All() iter.Seq2[string, any]
}
```

#### Event

```go
type Event struct {
    model.LLMResponse

    // Set by storage
    ID        string
    Timestamp time.Time

    // Set by context
    InvocationID string
    Branch       string // "agent_1.agent_2.agent_3"
    Author       string

    // Actions taken
    Actions EventActions

    // Long-running tool call IDs
    LongRunningToolIDs []string
}

func (e *Event) IsFinalResponse() bool
func NewEvent(invocationID string) *Event
```

#### EventActions

```go
type EventActions struct {
    // State changes
    StateDelta map[string]any

    // Artifact updates (filename -> version)
    ArtifactDelta map[string]int64

    // HITL confirmations requested
    RequestedToolConfirmations map[string]toolconfirmation.ToolConfirmation

    // Skip model summarization
    SkipSummarization bool

    // Transfer to another agent
    TransferToAgent string

    // Escalate to parent
    Escalate bool
}
```

### State Key Prefixes

```go
const (
    KeyPrefixApp  string = "app:"  // Shared across all users/sessions
    KeyPrefixUser string = "user:" // Per-user, across sessions
    KeyPrefixTemp string = "temp:" // Current invocation only
)
```

### Service Interface

```go
type Service interface {
    Create(ctx context.Context, req *CreateRequest) (*CreateResponse, error)
    Get(ctx context.Context, req *GetRequest) (*GetResponse, error)
    List(ctx context.Context, req *ListRequest) (*ListResponse, error)
    Delete(ctx context.Context, req *DeleteRequest) error
    AppendEvent(ctx context.Context, req *AppendEventRequest) (*AppendEventResponse, error)
}
```

### Functions

```go
// InMemoryService creates in-memory session service
func InMemoryService() Service
```

## runner Package

`google.golang.org/adk/runner`

### Config

```go
type Config struct {
    AppName        string
    Agent          agent.Agent
    SessionService session.Service
    ArtifactService artifact.Service
    MemoryService   memory.Service
}
```

### Runner

```go
type Runner struct {
    // ...
}

// New creates a runner
func New(cfg Config) (*Runner, error)

// Run executes agent with user content
func (r *Runner) Run(
    ctx context.Context,
    userID, sessionID string,
    content *genai.Content,
    runConfig agent.RunConfig,
) iter.Seq2[*session.Event, error]
```

## model Package

`google.golang.org/adk/model`

### LLM Interface

```go
type LLM interface {
    // Name returns model/provider name
    Name() string

    // GenerateContent generates response
    GenerateContent(
        ctx context.Context,
        req *LLMRequest,
        streaming bool,
    ) iter.Seq[*LLMResponse]
}
```

### LLMRequest

```go
type LLMRequest struct {
    Model    string
    Contents []*genai.Content
    Config   *genai.GenerateContentConfig
    Tools    map[string]any `json:"-"`
}
```

### LLMResponse

```go
type LLMResponse struct {
    Content         *genai.Content
    Citations       []*genai.CitationMetadata
    GroundingChunks []*genai.GroundingChunk
    UsageMetadata   *genai.UsageMetadata

    // Streaming
    Partial      bool
    TurnComplete bool

    // Error handling
    ErrorCode    int
    ErrorMessage string

    // Model behavior
    FinishReason genai.FinishReason
    AvgLogprobs  *float64
    Interrupted  bool
}
```

### Gemini Package

`google.golang.org/adk/model/gemini`

```go
// NewModel creates a Gemini model
func NewModel(
    ctx context.Context,
    modelName string,
    config *genai.ClientConfig,
) (model.LLM, error)
```

## Workflow Agents

### sequentialagent

`google.golang.org/adk/agent/workflowagents/sequentialagent`

```go
type Config struct {
    AgentConfig agent.Config
}

// New creates sequential agent (runs sub-agents in order)
func New(cfg Config) (agent.Agent, error)
```

### parallelagent

`google.golang.org/adk/agent/workflowagents/parallelagent`

```go
type Config struct {
    AgentConfig agent.Config
}

// New creates parallel agent (runs sub-agents concurrently)
func New(cfg Config) (agent.Agent, error)
```

### loopagent

`google.golang.org/adk/agent/workflowagents/loopagent`

```go
type Config struct {
    AgentConfig   agent.Config
    MaxIterations uint // 0 = infinite until escalation
}

// New creates loop agent (repeats sub-agents)
func New(cfg Config) (agent.Agent, error)
```

### remoteagent

`google.golang.org/adk/agent/remoteagent`

```go
type A2AConfig struct {
    Name            string
    AgentCardSource string // URL or path to agent card
}

// NewA2A creates agent connected to remote A2A server
func NewA2A(cfg A2AConfig) (agent.Agent, error)
```

## Server Packages

### adkrest

`google.golang.org/adk/server/adkrest`

```go
// NewHandler creates REST API handler
func NewHandler(config *launcher.Config, timeout time.Duration) http.Handler
```

**Endpoints**:
- `POST /apps/{appName}/users/{userId}/sessions/{sessionId}` - Create session
- `POST /run` - Run agent

### adka2a

`google.golang.org/adk/server/adka2a`

```go
type ExecutorConfig struct {
    RunnerConfig runner.Config
}

// NewExecutor creates A2A request executor
func NewExecutor(cfg ExecutorConfig) *Executor

// BuildAgentSkills extracts skills from agent for AgentCard
func BuildAgentSkills(agent agent.Agent) []a2a.Skill
```

## Additional Packages

### agenttool

`google.golang.org/adk/tool/agenttool`

```go
// New wraps an agent as a tool
func New(agent agent.Agent, config *Config) tool.Tool
```

### geminitool

`google.golang.org/adk/tool/geminitool`

```go
// GoogleSearch provides grounded search
type GoogleSearch struct{}
```

### mcptoolset

`google.golang.org/adk/tool/mcptoolset`

```go
type Config struct {
    Transport mcp.Transport
}

// New creates toolset from MCP transport
func New(cfg Config) (tool.Toolset, error)
```

### exitlooptool

`google.golang.org/adk/tool/exitlooptool`

```go
// New creates tool to exit loop agent
func New() tool.Tool
```

### toolconfirmation

`google.golang.org/adk/tool/toolconfirmation`

```go
type ToolConfirmation struct {
    Confirmed bool
    Payload   any
}

const FunctionCallName = "adk_request_confirmation"

// OriginalCallFrom extracts original call from confirmation
func OriginalCallFrom(fc *genai.FunctionCall) (*genai.FunctionCall, error)
```

### memory

`google.golang.org/adk/memory`

```go
type Service interface {
    AddSession(context.Context, session.Session) error
    Search(ctx context.Context, query string) (*SearchResponse, error)
}

// NewInMemory creates in-memory memory service
func NewInMemory() Service
```

### artifact

`google.golang.org/adk/artifact`

```go
type Service interface {
    Save(ctx context.Context, req *SaveRequest) (*SaveResponse, error)
    List(ctx context.Context, req *ListRequest) (*ListResponse, error)
    Load(ctx context.Context, req *LoadRequest) (*LoadResponse, error)
    LoadVersion(ctx context.Context, req *LoadVersionRequest) (*LoadResponse, error)
}
```

### launcher

`google.golang.org/adk/cmd/launcher`

```go
type Config struct {
    AgentLoader    agent.Loader
    SessionService session.Service
    ArtifactService artifact.Service
    MemoryService   memory.Service
}
```

```go
// full.NewLauncher() - all modes (console, restapi, a2a, webui)
// prod.NewLauncher() - production modes only (restapi, a2a)

l := full.NewLauncher()
l.Execute(ctx, config, os.Args[1:])
l.CommandLineSyntax() string // Returns usage help
```
