# ADK-Go Agent Patterns

Common patterns and architectures for building agents with ADK-Go.

## Table of Contents

1. [Single Agent Patterns](#single-agent-patterns)
2. [Multi-Agent Patterns](#multi-agent-patterns)
3. [Tool Patterns](#tool-patterns)
4. [State Management Patterns](#state-management-patterns)
5. [Error Handling Patterns](#error-handling-patterns)
6. [Testing Patterns](#testing-patterns)
7. [Production Patterns](#production-patterns)

## Single Agent Patterns

### Basic Conversational Agent

```go
agent, err := llmagent.New(llmagent.Config{
    Name:        "assistant",
    Model:       model,
    Description: "General purpose assistant",
    Instruction: `You are a helpful assistant. Be concise and accurate.
Answer questions directly. If unsure, say so.`,
})
```

### Specialized Domain Agent

```go
agent, err := llmagent.New(llmagent.Config{
    Name:        "code_reviewer",
    Model:       model,
    Description: "Reviews code for bugs, style, and best practices",
    Instruction: `You are a senior software engineer reviewing code.
Focus on:
- Logic errors and bugs
- Security vulnerabilities
- Performance issues
- Code style and readability
- Best practices

Provide specific, actionable feedback with code examples when appropriate.`,
    GenerateContentConfig: &genai.GenerateContentConfig{
        Temperature: genai.Ptr(float32(0.3)), // Lower for consistency
    },
})
```

### Structured Output Agent

```go
type AnalysisResult struct {
    Summary     string   `json:"summary"`
    KeyPoints   []string `json:"key_points"`
    Sentiment   string   `json:"sentiment"`
    Confidence  float64  `json:"confidence"`
}

agent, err := llmagent.New(llmagent.Config{
    Name:        "analyzer",
    Model:       model,
    Description: "Analyzes text and returns structured results",
    Instruction: "Analyze the provided text and return a structured analysis.",
    OutputSchema: &genai.Schema{
        Type: genai.TypeObject,
        Properties: map[string]*genai.Schema{
            "summary":    {Type: genai.TypeString},
            "key_points": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
            "sentiment":  {Type: genai.TypeString, Enum: []string{"positive", "negative", "neutral"}},
            "confidence": {Type: genai.TypeNumber},
        },
        Required: []string{"summary", "key_points", "sentiment", "confidence"},
    },
    // Note: OutputSchema disables tools
})
```

### Context-Aware Agent with State

```go
agent, err := llmagent.New(llmagent.Config{
    Name:        "personalized_assistant",
    Model:       model,
    Description: "Assistant that remembers user preferences",
    Instruction: `You are helping {user:name}.
Their preferences: {user:preferences?}
Their timezone: {user:timezone?}

Be helpful and remember their context across interactions.`,
    OutputKey: "last_response", // Save response to state
})
```

## Multi-Agent Patterns

### Hierarchical Delegation

Root agent delegates to specialized sub-agents:

```go
// Create specialized agents
researchAgent, _ := llmagent.New(llmagent.Config{
    Name:        "researcher",
    Description: "Performs web research and fact-finding",
    Tools:       []tool.Tool{geminitool.GoogleSearch{}},
    // ...
})

writerAgent, _ := llmagent.New(llmagent.Config{
    Name:        "writer",
    Description: "Writes and formats content",
    // ...
})

editorAgent, _ := llmagent.New(llmagent.Config{
    Name:        "editor",
    Description: "Reviews and improves writing",
    // ...
})

// Root agent orchestrates
rootAgent, _ := llmagent.New(llmagent.Config{
    Name:        "coordinator",
    Description: "Coordinates research and writing tasks",
    Instruction: `You coordinate a team of specialists:
- researcher: for finding information
- writer: for creating content
- editor: for reviewing and improving

Delegate appropriately and combine their outputs.`,
    Tools: []tool.Tool{
        agenttool.New(researchAgent, nil),
        agenttool.New(writerAgent, nil),
        agenttool.New(editorAgent, nil),
    },
})
```

### Pipeline Processing (Sequential)

Process through stages in order:

```go
// Stage 1: Extract information
extractAgent, _ := llmagent.New(llmagent.Config{
    Name:        "extractor",
    Description: "Extracts key information from input",
    Instruction: "Extract all relevant facts, dates, and entities from the input.",
    OutputKey:   "extracted_data",
})

// Stage 2: Analyze extracted data
analyzeAgent, _ := llmagent.New(llmagent.Config{
    Name:        "analyzer",
    Description: "Analyzes extracted information",
    Instruction: "Analyze the data in {extracted_data} and identify patterns and insights.",
    OutputKey:   "analysis",
})

// Stage 3: Generate report
reportAgent, _ := llmagent.New(llmagent.Config{
    Name:        "reporter",
    Description: "Generates final report",
    Instruction: "Create a comprehensive report using:\nData: {extracted_data}\nAnalysis: {analysis}",
})

// Combine in sequence
pipeline, _ := sequentialagent.New(sequentialagent.Config{
    AgentConfig: agent.Config{
        Name:        "processing_pipeline",
        Description: "Processes documents through extraction, analysis, and reporting",
        SubAgents:   []agent.Agent{extractAgent, analyzeAgent, reportAgent},
    },
})
```

### Parallel Processing

Run multiple agents concurrently:

```go
// Multiple perspectives on same input
optimistAgent, _ := llmagent.New(llmagent.Config{
    Name:        "optimist",
    Description: "Analyzes from optimistic perspective",
    Instruction: "Analyze the input focusing on opportunities and positive aspects.",
    OutputKey:   "optimist_view",
})

pessimistAgent, _ := llmagent.New(llmagent.Config{
    Name:        "pessimist",
    Description: "Analyzes from cautious perspective",
    Instruction: "Analyze the input focusing on risks and potential problems.",
    OutputKey:   "pessimist_view",
})

realistAgent, _ := llmagent.New(llmagent.Config{
    Name:        "realist",
    Description: "Analyzes from balanced perspective",
    Instruction: "Analyze the input with balanced, pragmatic view.",
    OutputKey:   "realist_view",
})

// Run in parallel
parallelAnalysis, _ := parallelagent.New(parallelagent.Config{
    AgentConfig: agent.Config{
        Name:        "multi_perspective",
        Description: "Analyzes from multiple perspectives simultaneously",
        SubAgents:   []agent.Agent{optimistAgent, pessimistAgent, realistAgent},
    },
})
```

### Iterative Refinement (Loop)

Refine output through iterations:

```go
// Draft creator
draftAgent, _ := llmagent.New(llmagent.Config{
    Name:        "drafter",
    Description: "Creates initial draft or revision",
    Instruction: `Create or revise content based on:
Current draft: {temp:current_draft?}
Feedback: {temp:feedback?}`,
    OutputKey: "temp:current_draft",
})

// Critic provides feedback
criticAgent, _ := llmagent.New(llmagent.Config{
    Name:        "critic",
    Description: "Reviews draft and provides feedback",
    Instruction: `Review the draft: {temp:current_draft}
Provide specific improvement suggestions.
If the draft meets quality standards, respond with "APPROVED".`,
    OutputKey: "temp:feedback",
    Tools: []tool.Tool{
        exitlooptool.New(), // Can exit loop when satisfied
    },
})

// Iterate until approved or max iterations
refiner, _ := loopagent.New(loopagent.Config{
    AgentConfig: agent.Config{
        Name:        "refiner",
        Description: "Iteratively improves content",
        SubAgents:   []agent.Agent{draftAgent, criticAgent},
    },
    MaxIterations: 5,
})
```

### Critic-Reviser Pattern

Common pattern for quality assurance:

```go
// First agent generates content
generatorAgent, _ := llmagent.New(llmagent.Config{
    Name:        "generator",
    Description: "Generates initial content",
    OutputKey:   "generated_content",
})

// Critic evaluates and identifies issues
criticAgent, _ := llmagent.New(llmagent.Config{
    Name:        "critic",
    Description: "Evaluates content for accuracy and quality",
    Instruction: `Review: {generated_content}
Identify:
- Factual errors
- Unclear sections
- Missing information
- Style issues`,
    OutputKey: "critique",
    Tools:     []tool.Tool{geminitool.GoogleSearch{}}, // Verify facts
})

// Reviser fixes identified issues
reviserAgent, _ := llmagent.New(llmagent.Config{
    Name:        "reviser",
    Description: "Revises content based on critique",
    Instruction: `Original: {generated_content}
Critique: {critique}
Revise the content addressing all identified issues.`,
})

// Run in sequence
qualityPipeline, _ := sequentialagent.New(sequentialagent.Config{
    AgentConfig: agent.Config{
        Name:      "quality_pipeline",
        SubAgents: []agent.Agent{generatorAgent, criticAgent, reviserAgent},
    },
})
```

## Tool Patterns

### API Integration Tool

```go
type APIInput struct {
    Endpoint string            `json:"endpoint"`
    Method   string            `json:"method"`
    Body     map[string]any    `json:"body,omitempty"`
    Headers  map[string]string `json:"headers,omitempty"`
}

type APIOutput struct {
    StatusCode int            `json:"status_code"`
    Body       map[string]any `json:"body"`
    Error      string         `json:"error,omitempty"`
}

func apiCall(ctx tool.Context, input APIInput) (APIOutput, error) {
    // Build request
    var bodyReader io.Reader
    if input.Body != nil {
        bodyBytes, _ := json.Marshal(input.Body)
        bodyReader = bytes.NewReader(bodyBytes)
    }

    req, err := http.NewRequestWithContext(ctx, input.Method, input.Endpoint, bodyReader)
    if err != nil {
        return APIOutput{Error: err.Error()}, nil
    }

    for k, v := range input.Headers {
        req.Header.Set(k, v)
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return APIOutput{Error: err.Error()}, nil
    }
    defer resp.Body.Close()

    var result map[string]any
    json.NewDecoder(resp.Body).Decode(&result)

    return APIOutput{
        StatusCode: resp.StatusCode,
        Body:       result,
    }, nil
}

apiTool, _ := functiontool.New(functiontool.Config{
    Name:        "api_call",
    Description: "Make HTTP API calls",
}, apiCall)
```

### Database Query Tool

```go
type QueryInput struct {
    SQL    string   `json:"sql"`
    Params []any    `json:"params,omitempty"`
}

type QueryOutput struct {
    Rows    []map[string]any `json:"rows"`
    Count   int              `json:"count"`
    Error   string           `json:"error,omitempty"`
}

func queryDB(ctx tool.Context, input QueryInput) (QueryOutput, error) {
    // Validate SQL (prevent injection)
    if !isReadOnly(input.SQL) {
        return QueryOutput{Error: "Only SELECT queries allowed"}, nil
    }

    rows, err := db.QueryContext(ctx, input.SQL, input.Params...)
    if err != nil {
        return QueryOutput{Error: err.Error()}, nil
    }
    defer rows.Close()

    var results []map[string]any
    // ... scan rows

    return QueryOutput{Rows: results, Count: len(results)}, nil
}

dbTool, _ := functiontool.New(functiontool.Config{
    Name:        "query_database",
    Description: "Query the database (read-only)",
}, queryDB)
```

### File Operation Tool with Confirmation

```go
type FileOp struct {
    Operation string `json:"operation"` // read, write, delete
    Path      string `json:"path"`
    Content   string `json:"content,omitempty"`
}

type FileResult struct {
    Success bool   `json:"success"`
    Content string `json:"content,omitempty"`
    Error   string `json:"error,omitempty"`
}

func fileOperation(ctx tool.Context, input FileOp) (FileResult, error) {
    switch input.Operation {
    case "read":
        content, err := os.ReadFile(input.Path)
        if err != nil {
            return FileResult{Error: err.Error()}, nil
        }
        return FileResult{Success: true, Content: string(content)}, nil

    case "write", "delete":
        // These require confirmation
        if ctx.ToolConfirmation() == nil {
            ctx.RequestConfirmation(
                fmt.Sprintf("Confirm %s on %s", input.Operation, input.Path),
                input,
            )
            return FileResult{Success: false, Error: "Awaiting confirmation"}, nil
        }
        if !ctx.ToolConfirmation().Confirmed {
            return FileResult{Error: "Operation rejected"}, nil
        }

        if input.Operation == "write" {
            err := os.WriteFile(input.Path, []byte(input.Content), 0644)
            return FileResult{Success: err == nil}, nil
        }
        err := os.Remove(input.Path)
        return FileResult{Success: err == nil}, nil
    }

    return FileResult{Error: "Unknown operation"}, nil
}

fileTool, _ := functiontool.New(functiontool.Config{
    Name:        "file_operation",
    Description: "Read, write, or delete files",
    RequireConfirmationProvider: func(input FileOp) bool {
        return input.Operation == "write" || input.Operation == "delete"
    },
}, fileOperation)
```

## State Management Patterns

### User Preferences

```go
// Store user preferences
func storePreference(ctx tool.Context, input PreferenceInput) (Result, error) {
    key := fmt.Sprintf("user:pref:%s", input.Name)
    ctx.State().Set(key, input.Value)
    return Result{Success: true}, nil
}

// Use in instruction
llmagent.Config{
    Instruction: `User preferences:
- Language: {user:pref:language?}
- Tone: {user:pref:tone?}
- Format: {user:pref:format?}

Apply these preferences to all responses.`,
}
```

### Conversation Context

```go
// Track conversation state
agent, _ := llmagent.New(llmagent.Config{
    Name: "contextual_agent",
    BeforeAgentCallbacks: []agent.BeforeAgentCallback{
        func(ctx agent.CallbackContext) (*genai.Content, error) {
            // Increment turn counter
            turn, _ := ctx.ReadonlyState().Get("temp:turn")
            turnNum, _ := turn.(int)
            ctx.State().Set("temp:turn", turnNum+1)
            return nil, nil
        },
    },
    AfterAgentCallbacks: []agent.AfterAgentCallback{
        func(ctx agent.CallbackContext) (*genai.Content, error) {
            // Store summary of interaction
            // ...
            return nil, nil
        },
    },
})
```

### Cross-Agent Communication via State

```go
// Agent 1 writes to state
agent1, _ := llmagent.New(llmagent.Config{
    Name:      "producer",
    OutputKey: "temp:agent1_output",
})

// Agent 2 reads from state
agent2, _ := llmagent.New(llmagent.Config{
    Name:        "consumer",
    Instruction: "Process the data: {temp:agent1_output}",
})
```

## Error Handling Patterns

### Graceful Tool Errors

```go
func robustTool(ctx tool.Context, input Input) (Output, error) {
    result, err := riskyOperation(input)
    if err != nil {
        // Return structured error, don't fail the tool
        return Output{
            Success: false,
            Error:   err.Error(),
            Suggestion: "Try with different parameters",
        }, nil
    }
    return Output{Success: true, Data: result}, nil
}
```

### Retry with OnModelErrorCallback

```go
retryCount := 0
maxRetries := 3

agent, _ := llmagent.New(llmagent.Config{
    OnModelErrorCallbacks: []llmagent.OnModelErrorCallback{
        func(ctx agent.CallbackContext, req *model.LLMRequest, err error) (*model.LLMResponse, error) {
            if retryCount < maxRetries && isRetryable(err) {
                retryCount++
                time.Sleep(time.Duration(retryCount) * time.Second)
                return nil, nil // Retry
            }
            return nil, err // Fail
        },
    },
})
```

## Testing Patterns

### Mock Model Responses

```go
func TestAgent(t *testing.T) {
    mockResponses := []string{
        "First response",
        "Second response",
    }
    responseIdx := 0

    agent, _ := llmagent.New(llmagent.Config{
        Name:  "test_agent",
        Model: realModel,
        BeforeModelCallbacks: []llmagent.BeforeModelCallback{
            func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
                resp := &model.LLMResponse{
                    Content: genai.NewContentFromText(mockResponses[responseIdx], genai.RoleModel),
                }
                responseIdx++
                return resp, nil
            },
        },
    })

    // Test agent behavior with controlled responses
}
```

### Test Tool in Isolation

```go
func TestWeatherTool(t *testing.T) {
    tool, _ := functiontool.New(functiontool.Config{
        Name: "weather",
    }, getWeather)

    // Create minimal context
    ctx := &mockToolContext{}

    result, err := tool.Run(ctx, map[string]any{"city": "Seattle"})

    assert.NoError(t, err)
    assert.Contains(t, result, "temperature")
}
```

## Production Patterns

### Structured Logging Callback

```go
agent, _ := llmagent.New(llmagent.Config{
    BeforeModelCallbacks: []llmagent.BeforeModelCallback{
        func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
            slog.Info("model_request",
                "agent", ctx.AgentName(),
                "invocation_id", ctx.InvocationID(),
                "message_count", len(req.Contents),
            )
            return nil, nil
        },
    },
    AfterModelCallbacks: []llmagent.AfterModelCallback{
        func(ctx agent.CallbackContext, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
            slog.Info("model_response",
                "agent", ctx.AgentName(),
                "invocation_id", ctx.InvocationID(),
                "token_count", resp.UsageMetadata.TotalTokenCount,
                "finish_reason", resp.FinishReason,
                "error", err,
            )
            return resp, nil
        },
    },
})
```

### Response Caching

```go
cache := make(map[string]*model.LLMResponse)
cacheMu := sync.RWMutex{}

agent, _ := llmagent.New(llmagent.Config{
    BeforeModelCallbacks: []llmagent.BeforeModelCallback{
        func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
            key := hashRequest(req)
            cacheMu.RLock()
            if cached, ok := cache[key]; ok {
                cacheMu.RUnlock()
                return cached, nil
            }
            cacheMu.RUnlock()
            return nil, nil
        },
    },
    AfterModelCallbacks: []llmagent.AfterModelCallback{
        func(ctx agent.CallbackContext, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
            if err == nil && resp != nil {
                key := hashRequest(lastReq)
                cacheMu.Lock()
                cache[key] = resp
                cacheMu.Unlock()
            }
            return resp, nil
        },
    },
})
```

### Rate Limiting

```go
limiter := rate.NewLimiter(rate.Limit(10), 1) // 10 requests/sec

agent, _ := llmagent.New(llmagent.Config{
    BeforeModelCallbacks: []llmagent.BeforeModelCallback{
        func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
            if err := limiter.Wait(ctx); err != nil {
                return nil, fmt.Errorf("rate limit: %w", err)
            }
            return nil, nil
        },
    },
})
```

### Health Check Middleware

```go
func healthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/health" {
            w.WriteHeader(http.StatusOK)
            json.NewEncoder(w).Encode(map[string]string{
                "status": "healthy",
                "version": version,
            })
            return
        }
        next.ServeHTTP(w, r)
    })
}
```
