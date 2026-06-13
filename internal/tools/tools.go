// Package tools 提供 Agent 可调用的可复用工具。
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// CalculatorInput 计算器工具参数。
type CalculatorInput struct {
	Expression string `json:"expression" jsonschema:"required" jsonschema_description:"A simple arithmetic expression, e.g. 2 + 2 * 3"`
}

// CurrentTimeInput 当前时间工具参数。
type CurrentTimeInput struct {
	Timezone string `json:"timezone" jsonschema_description:"Optional IANA timezone, defaults to UTC"`
}

// WeatherInput 天气模拟工具参数。
type WeatherInput struct {
	City string `json:"city" jsonschema:"required" jsonschema_description:"City name, e.g. Beijing"`
}

// SearchInput 网页搜索模拟工具参数。
type SearchInput struct {
	Query string `json:"query" jsonschema:"required" jsonschema_description:"Search query"`
}

// NewCalculator 创建计算器工具。
func NewCalculator() (tool.InvokableTool, error) {
	return utils.InferTool("calculator", "Evaluate a simple arithmetic expression containing +, -, *, / and parentheses.",
		func(ctx context.Context, input *CalculatorInput) (string, error) {
			result, err := evalExpression(input.Expression)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%.4f", result), nil
		})
}

// NewCurrentTime 创建当前时间工具。
func NewCurrentTime() (tool.InvokableTool, error) {
	return utils.InferTool("current_time", "Get the current date and time in the requested timezone.",
		func(ctx context.Context, input *CurrentTimeInput) (string, error) {
			tz := input.Timezone
			if tz == "" {
				tz = "UTC"
			}
			loc, err := time.LoadLocation(tz)
			if err != nil {
				return "", fmt.Errorf("invalid timezone %s: %w", tz, err)
			}
			return time.Now().In(loc).Format(time.RFC3339), nil
		})
}

// NewWeather 创建模拟天气查询工具。
func NewWeather() (tool.InvokableTool, error) {
	return utils.InferTool("weather", "Look up the current weather for a city (mock data for demo purposes).",
		func(ctx context.Context, input *WeatherInput) (string, error) {
			mock := map[string]any{
				"city":        input.City,
				"temperature": 22,
				"condition":   "sunny",
				"updated_at":  time.Now().UTC().Format(time.RFC3339),
			}
			b, _ := json.Marshal(mock)
			return string(b), nil
		})
}

// NewWebSearch 创建模拟网页搜索工具。
func NewWebSearch() (tool.InvokableTool, error) {
	return utils.InferTool("web_search", "Search the web for a query and return a short summary (mock data for demo purposes).",
		func(ctx context.Context, input *SearchInput) (string, error) {
			mock := map[string]any{
				"query": input.Query,
				"results": []string{
					fmt.Sprintf("Top result for '%s' from example.com", input.Query),
					fmt.Sprintf("Related article about %s", input.Query),
				},
			}
			b, _ := json.Marshal(mock)
			return string(b), nil
		})
}

// DefaultTools 返回默认工具集合。
func DefaultTools() ([]tool.InvokableTool, error) {
	calc, err := NewCalculator()
	if err != nil {
		return nil, err
	}
	timeTool, err := NewCurrentTime()
	if err != nil {
		return nil, err
	}
	weather, err := NewWeather()
	if err != nil {
		return nil, err
	}
	search, err := NewWebSearch()
	if err != nil {
		return nil, err
	}
	return []tool.InvokableTool{calc, timeTool, weather, search}, nil
}

// Registry 按名称管理工具。
type Registry struct {
	tools map[string]tool.InvokableTool
}

// NewRegistry 创建空工具注册表。
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]tool.InvokableTool)}
}

// Register 将工具加入注册表。
func (r *Registry) Register(t tool.InvokableTool) error {
	info, err := t.Info(context.Background())
	if err != nil {
		return err
	}
	r.tools[info.Name] = t
	return nil
}

// Get 按名称获取工具。
func (r *Registry) Get(name string) (tool.InvokableTool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List 返回所有已注册工具，供 Eino 编排使用。
func (r *Registry) List() []tool.BaseTool {
	out := make([]tool.BaseTool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

// Names 返回所有已注册工具名称。
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.tools))
	for name := range r.tools {
		out = append(out, name)
	}
	return out
}

// DefaultRegistry 创建并填充默认工具的注册表。
func DefaultRegistry() (*Registry, error) {
	reg := NewRegistry()
	tools, err := DefaultTools()
	if err != nil {
		return nil, err
	}
	for _, t := range tools {
		if err := reg.Register(t); err != nil {
			return nil, err
		}
	}
	return reg, nil
}

// evalExpression 计算简单算术表达式（支持 + - * / 和括号）。
func evalExpression(expr string) (float64, error) {
	expr = strings.ReplaceAll(expr, " ", "")
	if expr == "" {
		return 0, fmt.Errorf("empty expression")
	}
	result, err := parseAndEval(expr)
	if err != nil {
		return 0, err
	}
	return result, nil
}

func parseAndEval(expr string) (float64, error) {
	// Simple recursive descent parser for +, -, *, / and parentheses.
	pos := 0
	var parseExpression func() (float64, error)
	var parseTerm func() (float64, error)
	var parseFactor func() (float64, error)

	parseFactor = func() (float64, error) {
		if pos >= len(expr) {
			return 0, fmt.Errorf("unexpected end of expression")
		}
		if expr[pos] == '(' {
			pos++
			val, err := parseExpression()
			if err != nil {
				return 0, err
			}
			if pos >= len(expr) || expr[pos] != ')' {
				return 0, fmt.Errorf("missing closing parenthesis")
			}
			pos++
			return val, nil
		}
		start := pos
		for pos < len(expr) && (isDigit(expr[pos]) || expr[pos] == '.') {
			pos++
		}
		if start == pos {
			return 0, fmt.Errorf("expected number at position %d", pos)
		}
		return strconv.ParseFloat(expr[start:pos], 64)
	}

	parseTerm = func() (float64, error) {
		left, err := parseFactor()
		if err != nil {
			return 0, err
		}
		for pos < len(expr) && (expr[pos] == '*' || expr[pos] == '/') {
			op := expr[pos]
			pos++
			right, err := parseFactor()
			if err != nil {
				return 0, err
			}
			if op == '*' {
				left *= right
			} else {
				if right == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				left /= right
			}
		}
		return left, nil
	}

	parseExpression = func() (float64, error) {
		left, err := parseTerm()
		if err != nil {
			return 0, err
		}
		for pos < len(expr) && (expr[pos] == '+' || expr[pos] == '-') {
			op := expr[pos]
			pos++
			right, err := parseTerm()
			if err != nil {
				return 0, err
			}
			if op == '+' {
				left += right
			} else {
				left -= right
			}
		}
		return left, nil
	}

	result, err := parseExpression()
	if err != nil {
		return 0, err
	}
	if pos != len(expr) {
		return 0, fmt.Errorf("unexpected character at position %d", pos)
	}
	return result, nil
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
