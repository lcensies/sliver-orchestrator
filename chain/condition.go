package chain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ApplyFilter extracts a regex capture group from s.
// group is 1-based (matches standard regex group numbering).
// If group is 0 or unset it defaults to 1.
// Returns the original string unchanged if the pattern does not match.
func ApplyFilter(s string, f *OutputFilter) (string, bool) {
	if f == nil || f.Regex == "" {
		return s, false
	}
	re, err := regexp.Compile(f.Regex)
	if err != nil {
		return s, false
	}
	m := re.FindStringSubmatch(s)
	if m == nil {
		return s, false
	}
	g := f.Group
	if g <= 0 {
		g = 1
	}
	if g >= len(m) {
		return s, false
	}
	return m[g], true
}

// ExtractCaptures applies each OutputCapture against stdout and returns a map of
// variable name → extracted value.  Entries that don't match are omitted.
func ExtractCaptures(stdout string, caps []OutputCapture) map[string]string {
	out := make(map[string]string, len(caps))
	for _, c := range caps {
		if c.Var == "" || c.Regex == "" {
			continue
		}
		val, ok := ApplyFilter(stdout, &OutputFilter{Regex: c.Regex, Group: c.Group})
		if ok {
			out[c.Var] = strings.TrimRight(val, "\r\n")
		}
	}
	return out
}

// VarMap stores named output variables captured from step execution.
type VarMap map[string]string

// Substitute replaces every {{Name}} placeholder in s with the corresponding value from vars.
// Unknown placeholders are left unchanged.
func Substitute(s string, vars VarMap) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}

// SubstituteAction returns a shallow copy of action with all string fields substituted.
func SubstituteAction(a Action, vars VarMap) Action {
	if a.Command != nil {
		c := *a.Command
		c.Cmd = Substitute(c.Cmd, vars)
		a.Command = &c
	}
	if a.AtomicRef != nil {
		ref := *a.AtomicRef
		newArgs := make(map[string]string, len(ref.Args))
		for k, v := range ref.Args {
			newArgs[k] = Substitute(v, vars)
		}
		ref.Args = newArgs
		a.AtomicRef = &ref
	}
	if a.Upload != nil {
		up := *a.Upload
		up.LocalPath = Substitute(up.LocalPath, vars)
		up.RemotePath = Substitute(up.RemotePath, vars)
		a.Upload = &up
	}
	if a.Binary != nil {
		bin := *a.Binary
		bin.URL = Substitute(bin.URL, vars)
		bin.RemotePath = Substitute(bin.RemotePath, vars)
		bin.Args = Substitute(bin.Args, vars)
		a.Binary = &bin
	}
	if a.Probe != nil {
		p := *a.Probe
		p.Software = Substitute(p.Software, vars)
		p.Match = Substitute(p.Match, vars)
		a.Probe = &p
	}
	if a.Python != nil {
		py := *a.Python
		py.Script = Substitute(py.Script, vars)
		newArgs := make([]string, len(py.Args))
		for i, arg := range py.Args {
			newArgs[i] = Substitute(arg, vars)
		}
		py.Args = newArgs
		newEnv := make(map[string]string, len(py.Env))
		for k, v := range py.Env {
			newEnv[k] = Substitute(v, vars)
		}
		py.Env = newEnv
		a.Python = &py
	}
	if a.RPCAction != nil {
		rpc := *a.RPCAction
		newParams := make(map[string]string, len(rpc.Params))
		for k, v := range rpc.Params {
			newParams[k] = Substitute(v, vars)
		}
		rpc.Params = newParams
		a.RPCAction = &rpc
	}
	return a
}

// EvalCondition evaluates a single condition against vars and the last step's exit code.
func EvalCondition(c Condition, vars VarMap, exitCode int) (bool, error) {
	var val string
	if c.Var == "exit_code" {
		val = strconv.Itoa(exitCode)
	} else {
		v, ok := vars[c.Var]
		if !ok {
			return false, fmt.Errorf("variable %q not found in scope", c.Var)
		}
		val = v
	}

	var result bool
	switch c.Op {
	case "eq":
		result = val == c.Value
	case "neq":
		result = val != c.Value
	case "contains":
		result = strings.Contains(val, c.Value)
	case "matches":
		re, err := regexp.Compile(c.Value)
		if err != nil {
			return false, fmt.Errorf("invalid regex %q in condition: %w", c.Value, err)
		}
		result = re.MatchString(val)
	case "gt":
		n, err := strconv.Atoi(val)
		if err != nil {
			return false, fmt.Errorf("cannot compare non-numeric %q with gt", val)
		}
		threshold, err := strconv.Atoi(c.Value)
		if err != nil {
			return false, fmt.Errorf("cannot parse threshold %q for gt", c.Value)
		}
		result = n > threshold
	case "lt":
		n, err := strconv.Atoi(val)
		if err != nil {
			return false, fmt.Errorf("cannot compare non-numeric %q with lt", val)
		}
		threshold, err := strconv.Atoi(c.Value)
		if err != nil {
			return false, fmt.Errorf("cannot parse threshold %q for lt", c.Value)
		}
		result = n < threshold
	default:
		return false, fmt.Errorf("unknown condition op %q (valid: eq, neq, contains, matches, gt, lt)", c.Op)
	}

	if c.Negate {
		return !result, nil
	}
	return result, nil
}

// EvalConditions returns true only when all conditions pass.
// Returns the first error encountered in condition evaluation.
func EvalConditions(conds []Condition, vars VarMap, exitCode int) (bool, error) {
	for _, c := range conds {
		ok, err := EvalCondition(c, vars, exitCode)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}
