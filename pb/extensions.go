package pb

import "strings"

func (f Submission_Constraints_Flag) In(flags uint32) bool {
	return flags&uint32(f) != 0
}

func (rt *RuntimeDefinition) BuildCompileCommand(inp, output string) (string, []string) {
	r := strings.NewReplacer("{{input}}", inp, "{{output}}", output)
	return rt.CompileCmd, strings.Split(r.Replace(rt.CompileArgs), " ")
}

func (rt *RuntimeDefinition) BuildExecCommand(prog string) (string, []string) {
	if rt.ExecCmd == "" {
		return prog, []string{}
	}
	// Wrap the command within `sh` for piping and other things to work.
	return "sh", []string{"-c", strings.ReplaceAll(rt.ExecCmd, "{{program}}", prog)}
}
