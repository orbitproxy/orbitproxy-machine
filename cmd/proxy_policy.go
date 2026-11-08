package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"

	orbitproxy "github.com/orbitproxy/orbitproxy-go"
	"github.com/orbitproxy/orbitproxy-machine/cliout"
	"github.com/spf13/cobra"
)

func init() {
	proxyCmd.AddCommand(proxyPolicyCmd, proxyMaintenanceCmd, proxyLogsCmd)
	proxyPolicyCmd.AddCommand(proxyPolicyApplyCmd, proxyPolicyListCmd, proxyPolicyDeleteCmd, proxyPolicyPasswordCmd, proxyPolicyKeyCmd)
	proxyPolicyApplyCmd.Flags().StringVar(&policyProxyID, "proxy-id", "", "proxy id")
	proxyPolicyApplyCmd.Flags().StringVar(&policyFile, "file", "", "policy json file")
	proxyPolicyApplyCmd.Flags().BoolVar(&policyJSON, "json", false, "print JSON")
	proxyPolicyListCmd.Flags().StringVar(&policyProxyID, "proxy-id", "", "proxy id")
	proxyPolicyListCmd.Flags().BoolVar(&policyJSON, "json", false, "print JSON")
	proxyPolicyDeleteCmd.Flags().StringVar(&policyProxyID, "proxy-id", "", "proxy id")
	proxyPolicyDeleteCmd.Flags().StringVar(&policyTypeFlag, "type", "", "policy type")
	proxyPolicyDeleteCmd.Flags().StringVar(&policyRuleID, "rule-group-id", "", "rule group id")
	proxyPolicyDeleteCmd.Flags().BoolVar(&policyJSON, "json", false, "print JSON")
	proxyPolicyPasswordCmd.Flags().StringVar(&policyFile, "file", "", "policy json file")
	proxyPolicyPasswordCmd.Flags().StringVar(&policyRuleName, "name", "", "basic auth rule name")
	proxyPolicyPasswordCmd.Flags().StringVar(&policyUsername, "username", "", "basic auth username")
	proxyPolicyKeyCmd.Flags().StringVar(&policyFile, "file", "", "policy json file")
	proxyPolicyKeyCmd.Flags().StringVar(&policyRuleName, "name", "", "api key rule name")
	proxyPolicyKeyCmd.Flags().StringVar(&policyClientName, "client", "", "api key client name")
	proxyMaintenanceCmd.Flags().StringVar(&policyProxyID, "proxy-id", "", "proxy id")
	proxyMaintenanceCmd.Flags().BoolVar(&maintenanceEnable, "enable", false, "enable maintenance")
	proxyMaintenanceCmd.Flags().BoolVar(&maintenanceDisable, "disable", false, "disable maintenance")
	proxyMaintenanceCmd.Flags().BoolVar(&policyJSON, "json", false, "print JSON")
	proxyLogsCmd.Flags().StringVar(&policyProxyID, "proxy-id", "", "proxy id")
	proxyLogsCmd.Flags().BoolVar(&policyJSON, "json", false, "print JSON")
}

var (
	policyProxyID      string
	policyFile         string
	policyJSON         bool
	policyTypeFlag     string
	policyRuleID       string
	policyRuleName     string
	policyUsername     string
	policyClientName   string
	maintenanceEnable  bool
	maintenanceDisable bool
)

var proxyPolicyCmd = &cobra.Command{Use: "policy", Short: "Manage traffic policy rules for a proxy on this computer"}
var proxyPolicyApplyCmd = &cobra.Command{
	Use: "apply", Short: "Create or update policy rules from a JSON file", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		env, err := newCLIEnv(cmd)
		if err != nil {
			return err
		}
		printer := env.printer(policyJSON)
		return runCLI(printer, func() error { return runPolicyApply(cmd, env, printer) })
	},
}
var proxyPolicyListCmd = &cobra.Command{
	Use: "list", Short: "List policy rules", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		env, err := newCLIEnv(cmd)
		if err != nil {
			return err
		}
		printer := env.printer(policyJSON)
		return runCLI(printer, func() error { return runPolicyList(cmd, env, printer) })
	},
}
var proxyPolicyDeleteCmd = &cobra.Command{
	Use: "delete", Short: "Delete one policy rule", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		env, err := newCLIEnv(cmd)
		if err != nil {
			return err
		}
		printer := env.printer(policyJSON)
		return runCLI(printer, func() error { return runPolicyDelete(cmd, env, printer) })
	},
}
var proxyPolicyPasswordCmd = &cobra.Command{
	Use: "generate-password", Short: "Write a new basic auth password into a policy file", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		env, err := newCLIEnv(cmd)
		if err != nil {
			return err
		}
		return runCLI(env.printer(false), func() error {
			return generateSecret(policyFile, policyRuleName, "basic_auth", "users", "username", policyUsername, "password")
		})
	},
}
var proxyPolicyKeyCmd = &cobra.Command{
	Use: "generate-key", Short: "Write a new API key into a policy file", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		env, err := newCLIEnv(cmd)
		if err != nil {
			return err
		}
		return runCLI(env.printer(false), func() error {
			return generateSecret(policyFile, policyRuleName, "api_key_auth", "credentials", "client", policyClientName, "key")
		})
	},
}
var proxyMaintenanceCmd = &cobra.Command{
	Use: "maintenance", Short: "Turn maintenance mode on or off", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		env, err := newCLIEnv(cmd)
		if err != nil {
			return err
		}
		printer := env.printer(policyJSON)
		return runCLI(printer, func() error { return runMaintenance(cmd, env, printer) })
	},
}
var proxyLogsCmd = &cobra.Command{
	Use: "logs", Short: "Show recent access log summaries", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		env, err := newCLIEnv(cmd)
		if err != nil {
			return err
		}
		printer := env.printer(policyJSON)
		return runCLI(printer, func() error { return runProxyLogs(cmd, env, printer) })
	},
}

func runPolicyApply(cmd *cobra.Command, env *cliEnv, printer cliout.Printer) error {
	if policyProxyID == "" || policyFile == "" {
		return cliout.New("invalid_request", "必须指定 --proxy-id 和 --file")
	}
	current, err := env.currentSession("")
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(policyFile)
	if err != nil {
		return err
	}
	var file struct {
		Rules []map[string]any `json:"rules"`
	}
	if err := json.Unmarshal(raw, &file); err != nil || file.Rules == nil {
		return cliout.New("invalid_policy_file", "策略文件不是合法 JSON")
	}
	for _, rule := range file.Rules {
		if _, ok := rule["proxyId"]; ok {
			return cliout.New("invalid_policy_file", "策略文件不能包含 proxyId")
		}
		enabled, ok := rule["enabled"].(bool)
		if !ok || (enabled != true && enabled != false) {
			return cliout.New("enabled_required", "每条规则都必须有布尔 enabled")
		}
	}
	client, err := newPolicyClient(env, current)
	if err != nil {
		return err
	}
	tunnels, err := current.client.ListTunnels(cmd.Context())
	if err != nil {
		return err
	}
	protocol := ""
	for _, tunnel := range tunnels {
		if tunnel.ProxyID == policyProxyID {
			protocol = tunnel.Protocol
		}
	}
	if protocol == "tcp" {
		for _, rule := range file.Rules {
			if rule["type"] != "ip_access_control" {
				return cliout.New("policy_type_not_allowed", "tcp 分享只能使用 ip_access_control")
			}
		}
	}
	summary := policyApplySummary{ProxyID: policyProxyID, Applied: []policyApplied{}}
	for _, rule := range file.Rules {
		data, err := client.ApplyPolicy(cmd.Context(), policyProxyID, rule)
		if err != nil {
			summary.Failed = policyFailure(rule, err)
			return finishPolicyApply(printer, summary, err)
		}
		item, err := projectApplied(data)
		if err != nil {
			return err
		}
		summary.Applied = append(summary.Applied, item)
	}
	return finishPolicyApply(printer, summary, nil)
}

func runPolicyList(cmd *cobra.Command, env *cliEnv, printer cliout.Printer) error {
	_, client, err := policySession(cmd, env)
	if err != nil {
		return err
	}
	data, err := client.ListPolicies(cmd.Context(), policyProxyID)
	if err != nil {
		return err
	}
	var body struct {
		Rules json.RawMessage `json:"rules"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return err
	}
	rules := rawOrEmpty(body.Rules)
	var rows []map[string]any
	if err := json.Unmarshal(rules, &rows); err != nil {
		return err
	}
	return printer.Success(rules, func(out io.Writer) {
		for _, rule := range rows {
			fmt.Fprintf(out, "%s  %s  %s  %s\n", jsonString(rule["rule_group_id"]), jsonString(rule["type"]), jsonString(rule["name"]), jsonBool(rule["enabled"]))
		}
	})
}

func runPolicyDelete(cmd *cobra.Command, env *cliEnv, printer cliout.Printer) error {
	_, client, err := policySession(cmd, env)
	if err != nil {
		return err
	}
	data, err := client.DeletePolicy(cmd.Context(), policyProxyID, policyTypeFlag, policyRuleID)
	if err != nil {
		return err
	}
	var body struct {
		RuleGroupID string `json:"rule_group_id"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return err
	}
	return printer.Success(json.RawMessage(data), func(out io.Writer) {
		fmt.Fprintf(out, "deleted %s\n", body.RuleGroupID)
	})
}

func runMaintenance(cmd *cobra.Command, env *cliEnv, printer cliout.Printer) error {
	if maintenanceEnable == maintenanceDisable {
		return cliout.New("invalid_request", "必须指定 --enable 或 --disable 其中一个")
	}
	_, client, err := policySession(cmd, env)
	if err != nil {
		return err
	}
	data, err := client.SetMaintenance(cmd.Context(), policyProxyID, maintenanceEnable)
	if err != nil {
		return err
	}
	var body struct {
		PublicURL   string `json:"public_url"`
		Maintenance bool   `json:"maintenance"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return err
	}
	state := "disabled"
	if body.Maintenance {
		state = "enabled"
	}
	return printer.Success(json.RawMessage(data), func(out io.Writer) {
		fmt.Fprintf(out, "maintenance %s\n%s\n", state, body.PublicURL)
	})
}

func runProxyLogs(cmd *cobra.Command, env *cliEnv, printer cliout.Printer) error {
	_, client, err := policySession(cmd, env)
	if err != nil {
		return err
	}
	data, err := client.ListAccessLogs(cmd.Context(), policyProxyID)
	if err != nil {
		return err
	}
	var body struct {
		Items json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return err
	}
	items := rawOrEmpty(body.Items)
	var rows []struct {
		EventTime  string `json:"event_time"`
		Method     string `json:"method"`
		Path       string `json:"path"`
		StatusCode int    `json:"status_code"`
		ErrorCode  string `json:"error_code"`
	}
	if err := json.Unmarshal(items, &rows); err != nil {
		return err
	}
	return printer.Success(items, func(out io.Writer) {
		if len(rows) == 0 {
			fmt.Fprintln(out, "no access logs")
			return
		}
		for _, row := range rows {
			fmt.Fprintf(out, "%s  %s  %s  %d  %s\n", row.EventTime, row.Method, row.Path, row.StatusCode, row.ErrorCode)
		}
	})
}

type policyApplySummary struct {
	ProxyID string           `json:"proxy_id"`
	Applied []policyApplied  `json:"applied"`
	Failed  *policyApplyFail `json:"failed"`
}

type policyApplied struct {
	Result       string `json:"result"`
	RuleGroupID  string `json:"rule_group_id"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	ReviewStatus string `json:"reviewStatus,omitempty"`
}

type policyApplyFail struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func finishPolicyApply(printer cliout.Printer, summary policyApplySummary, cause error) error {
	if cause == nil {
		return printer.Success(summary, func(out io.Writer) { writePolicyApplyHuman(out, summary) })
	}
	if printer.JSON {
		cliErr := cliout.FromError(cause)
		cliErr.Data = summary
		return cliErr
	}
	writePolicyApplyHuman(printer.Out, summary)
	return cause
}

func writePolicyApplyHuman(out io.Writer, summary policyApplySummary) {
	for _, item := range summary.Applied {
		if item.Type == "custom_response" {
			fmt.Fprintf(out, "%s  %s  %s  %s  %s\n", item.Result, item.Type, item.Name, item.RuleGroupID, item.ReviewStatus)
			continue
		}
		fmt.Fprintf(out, "%s  %s  %s  %s\n", item.Result, item.Type, item.Name, item.RuleGroupID)
	}
	if summary.Failed != nil {
		fmt.Fprintf(out, "failed  %s  %s  %s  %s\n", summary.Failed.Type, summary.Failed.Name, summary.Failed.Code, summary.Failed.Message)
	}
}

func projectApplied(data json.RawMessage) (policyApplied, error) {
	var body struct {
		Result string `json:"result"`
		Rule   struct {
			RuleGroupID  string `json:"rule_group_id"`
			Type         string `json:"type"`
			Name         string `json:"name"`
			ReviewStatus string `json:"reviewStatus"`
		} `json:"rule"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return policyApplied{}, err
	}
	item := policyApplied{Result: body.Result, RuleGroupID: body.Rule.RuleGroupID, Type: body.Rule.Type, Name: body.Rule.Name}
	// 只有自定义页需要把审核状态交给调用方，其它类型没有这个字段。
	if body.Rule.Type == "custom_response" {
		item.ReviewStatus = body.Rule.ReviewStatus
	}
	return item, nil
}

func policyFailure(rule map[string]any, cause error) *policyApplyFail {
	cliErr := cliout.FromError(cause)
	return &policyApplyFail{Type: jsonString(rule["type"]), Name: jsonString(rule["name"]), Code: cliErr.Code, Message: cliErr.Message}
}

func rawOrEmpty(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("[]")
	}
	return raw
}

func jsonString(value any) string {
	text, _ := value.(string)
	return text
}

func jsonBool(value any) string {
	flag, ok := value.(bool)
	if !ok {
		return ""
	}
	if flag {
		return "true"
	}
	return "false"
}

func policySession(cmd *cobra.Command, env *cliEnv) (*session, *orbitproxy.MachineClient, error) {
	if policyProxyID == "" {
		return nil, nil, cliout.New("invalid_request", "必须指定 --proxy-id")
	}
	current, err := env.currentSession("")
	if err != nil {
		return nil, nil, err
	}
	client, err := newPolicyClient(env, current)
	if err != nil {
		return nil, nil, err
	}
	return current, client, nil
}

func newPolicyClient(env *cliEnv, current *session) (*orbitproxy.MachineClient, error) {
	return orbitproxy.NewMachineClient(orbitproxy.MachineClientOptions{
		APIURL:        env.baseURL,
		MachineKey:    current.cfg.MachineKey,
		PrivateKeyPEM: current.cfg.PrivateKeyPEM,
		HTTPClient:    env.http,
	})
}

func generateSecret(path, ruleName, ruleType, listKey, matchKey, matchValue, secretKey string) error {
	if path == "" || ruleName == "" || matchValue == "" {
		return cliout.New("invalid_request", "必须指定 --file、--name，以及 --username 或 --client")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var file struct {
		Rules []map[string]any `json:"rules"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return cliout.New("invalid_policy_file", "策略文件不是合法 JSON")
	}
	secret, err := randomSecret()
	if err != nil {
		return err
	}
	found := false
	for _, rule := range file.Rules {
		if rule["type"] != ruleType || rule["name"] != ruleName {
			continue
		}
		items, _ := rule[listKey].([]any)
		for _, item := range items {
			fields, _ := item.(map[string]any)
			if fields[matchKey] != matchValue {
				continue
			}
			fields[secretKey] = secret
			found = true
		}
	}
	if !found {
		return cliout.New("invalid_policy_file", "策略文件里没有这条规则")
	}
	encoded, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	fmt.Printf("wrote %s %s %s\n", path, ruleName, matchValue)
	return nil
}

func randomSecret() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	out := make([]byte, len(buf))
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out), nil
}
