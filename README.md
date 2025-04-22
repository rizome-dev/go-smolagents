# smolagentsgo

[![GoDoc](https://pkg.go.dev/badge/github.com/rizome-dev/smolagentsgo)](https://pkg.go.dev/github.com/rizome-dev/smolagentsgo)
[![Go Report Card](https://goreportcard.com/badge/github.com/rizome-dev/smolagentsgo)](https://goreportcard.com/report/github.com/rizome-dev/smolagentsgo)

```shell
go get github.com/rizome-dev/smolagentsgo
```

built by: [rizome labs](https://rizome.dev)

contact us: [hi (at) rizome.dev](mailto:hi@rizome.dev)

## Example

```go
package main

import (
    "fmt"
    "github.com/rizome-dev/smolagentsgo"
)

func main() {
    calculator := smolagentsgo.NewBaseTool(
        "calculator",
        "A simple calculator that can add, subtract, multiply, and divide",
        map[string]smolagentsgo.InputProperty{
            "expression": {
                Type:        "string",
                Description: "The mathematical expression to evaluate",
            },
        },
        "string",
        func(args map[string]interface{}) (interface{}, error) {
            return "42", nil
        },
    )

    agent, err := smolagentsgo.NewToolCallingAgent(
        []smolagentsgo.Tool{calculator},
        nil,
        smolagentsgo.EmptyPromptTemplates(),
        0,
        20,
        nil,
        nil,
        nil,
        "math_agent",
        "An agent that can solve math problems",
        false,
        nil,
    )

    if err != nil {
        fmt.Printf("Error creating agent: %v\n", err)
        return
    }

    result, err := agent.Run("Calculate 2 + 2", false, true, nil, nil, 0)
}
```
 