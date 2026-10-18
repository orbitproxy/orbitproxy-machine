// Package cliout 统一 WorkBuddy 等非交互调用方读取的输出结构与退出码。
package cliout

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/orbitproxy/orbitproxy-machine/buildmeta"
	"github.com/orbitproxy/orbitproxy-machine/machineapi"
)

const (
	ExitOK            = 0
	ExitError         = 1
	ExitUsage         = 2
	ExitNotLoggedIn   = 3
	ExitQuota         = 4
	ExitUnreachable   = 5
	ExitClientOffline = 6
)

// exitByCode 集中维护控制面 / CLI 错误码到退出码的映射，未列出的都是 1。
var exitByCode = map[string]int{
	"invalid_port":            ExitUsage,
	"invalid_host":            ExitUsage,
	"invalid_subdomain":       ExitUsage,
	"subdomain_required":      ExitUsage,
	"invalid_local_addr":      ExitUsage,
	"machine_ambiguous":       ExitUsage,
	"ambiguous_port":          ExitUsage,
	"invalid_request":         ExitUsage,
	"invalid_tunnel_protocol": ExitUsage,
	"unsupported_proxy_type":  ExitUsage,

	"not_logged_in":          ExitNotLoggedIn,
	"invalid_machine":        ExitNotLoggedIn,
	"invalid_signature":      ExitNotLoggedIn,
	"machine_owner_inactive": ExitNotLoggedIn,
	"login_expired":          ExitNotLoggedIn,
	"login_denied":           ExitNotLoggedIn,

	"no_active_subscription":  ExitQuota,
	"proxy_quota_exceeded":    ExitQuota,
	"domain_quota_exceeded":   ExitQuota,
	"port_quota_exceeded":     ExitQuota,
	"bandwidth_exceeded":      ExitQuota,
	"proxy_type_not_entitled": ExitQuota,
	"rootdomain_incompatible": ExitQuota,
	"no_rootdomain_available": ExitQuota,

	"cp_unreachable": ExitUnreachable,

	"client_offline": ExitClientOffline,
}

func ExitCodeFor(code string) int {
	if exit, ok := exitByCode[code]; ok {
		return exit
	}
	return ExitError
}

// Error 是 CLI 对外的错误；Data 用于附带候选项等结构化信息。
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func (err *Error) Error() string { return err.Message }

func (err *Error) ExitCode() int { return ExitCodeFor(err.Code) }

func New(code, message string) *Error {
	return &Error{Code: code, Message: message, Hint: HintFor(code)}
}

// FromError 把控制面错误、网络错误与其他错误统一成 *Error。
func FromError(err error) *Error {
	var cliErr *Error
	if errors.As(err, &cliErr) {
		return cliErr
	}
	var apiErr *machineapi.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.Code
		if code == "" {
			code = "control_plane_error"
		}
		return &Error{Code: code, Message: apiErr.Error(), Hint: HintFor(code)}
	}
	var netErr *machineapi.NetworkError
	if errors.As(err, &netErr) {
		return &Error{Code: "cp_unreachable", Message: netErr.Error(), Hint: HintFor("cp_unreachable")}
	}
	return &Error{Code: "internal_error", Message: err.Error()}
}

func HintFor(code string) string {
	switch ExitCodeFor(code) {
	case ExitNotLoggedIn:
		return "在 WorkBuddy 连接器里重新连接 OrbitProxy，或在终端执行 orbitproxy login"
	case ExitQuota:
		return fmt.Sprintf("关闭一个分享，或前往 %s/billing 升级套餐", buildmeta.ConsoleURL)
	case ExitUnreachable:
		return "检查网络连接后重试"
	case ExitClientOffline:
		return "本机客户端无法连上 OrbitProxy，检查网络后重试"
	}
	return ""
}

type successBody struct {
	OK   bool `json:"ok"`
	Data any  `json:"data"`
}

type failureBody struct {
	OK    bool   `json:"ok"`
	Error *Error `json:"error"`
}

// Printer 按 --json 决定输出 JSON 还是人读文本。
type Printer struct {
	JSON bool
	Out  io.Writer
	Err  io.Writer
}

func (printer Printer) Success(data any, human func(io.Writer)) error {
	if printer.JSON {
		return writeJSON(printer.Out, successBody{OK: true, Data: data})
	}
	if human != nil {
		human(printer.Out)
	}
	return nil
}

// Fail 输出错误并返回退出码。
func (printer Printer) Fail(err error) int {
	cliErr := FromError(err)
	if printer.JSON {
		_ = writeJSON(printer.Out, failureBody{OK: false, Error: cliErr})
		return cliErr.ExitCode()
	}
	fmt.Fprintf(printer.Err, "error: %s (%s)\n", cliErr.Message, cliErr.Code)
	if cliErr.Hint != "" {
		fmt.Fprintf(printer.Err, "hint: %s\n", cliErr.Hint)
	}
	return cliErr.ExitCode()
}

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
