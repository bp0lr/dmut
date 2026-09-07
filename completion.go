package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/pflag"
)

const helpExamples = `Examples:
  Preview a few names and see which rules produced them (offline):
    dmut -u test.example.com -d words.txt --preview --explain

  Save all generated names to a file (offline):
    dmut -u test.example.com -d words.txt --save-gen --save-to generated.txt

  Read domains from stdin (offline):
    cat domains.txt | dmut -d words.txt --save-gen
    Get-Content domains.txt | dmut -d words.txt --save-gen

  Print shell completion for Bash, Zsh, Fish or PowerShell:
    dmut --completion powershell

Results go to stdout; progress, summaries and errors go to stderr.
`

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func writeCompletion(out io.Writer, shell string, flags *pflag.FlagSet) error {
	var options []string
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		options = append(options, "--"+flag.Name)
		if flag.Shorthand != "" {
			options = append(options, "-"+flag.Shorthand)
		}
	})
	var script strings.Builder
	switch shell {
	case "bash":
		fmt.Fprintf(&script, `# Load with: source <(dmut --completion bash)
_dmut_complete() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    COMPREPLY=( $(compgen -W %s -- "$cur") )
}
complete -o default -o bashdefault -F _dmut_complete dmut dmut.exe
`, shellQuote(strings.Join(options, " ")))
	case "zsh":
		script.WriteString("#compdef dmut dmut.exe\n# Save as _dmut in a directory on your fpath, then run compinit.\n_arguments \\\n")
		flags.VisitAll(func(flag *pflag.Flag) {
			if flag.Hidden {
				return
			}
			description := strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]", ":", "\\:").Replace(flag.Usage)
			names := []string{"--" + flag.Name}
			if flag.Shorthand != "" {
				names = append(names, "-"+flag.Shorthand)
			}
			for _, name := range names {
				spec := name + "[" + description + "]"
				if flag.NoOptDefVal == "" {
					spec += ":value:"
					if flag.Name == "completion" {
						spec += "(bash zsh fish powershell)"
					} else if isFileFlag(flag.Name) {
						spec += "_files"
					}
				}
				fmt.Fprintf(&script, "  %s \\\n", shellQuote(spec))
			}
		})
		script.WriteString("  '*:file:_files'\n")
	case "fish":
		script.WriteString("# Load with: dmut --completion fish | source\n")
		flags.VisitAll(func(flag *pflag.Flag) {
			if flag.Hidden {
				return
			}
			fmt.Fprintf(&script, "complete -c dmut -l %s", flag.Name)
			if flag.Shorthand != "" {
				fmt.Fprintf(&script, " -s %s", flag.Shorthand)
			}
			if flag.NoOptDefVal == "" {
				script.WriteString(" -r")
			}
			if !isFileFlag(flag.Name) {
				script.WriteString(" -f")
			}
			if flag.Name == "completion" {
				script.WriteString(" -a 'bash zsh fish powershell'")
			}
			fmt.Fprintf(&script, " -d %s\n", shellQuote(flag.Usage))
		})
	case "powershell":
		script.WriteString("# Load with: dmut --completion powershell | Out-String | Invoke-Expression\nRegister-ArgumentCompleter -Native -CommandName dmut,dmut.exe -ScriptBlock {\n    param($wordToComplete, $commandAst, $cursorPosition)\n    @(\n")
		for _, option := range options {
			fmt.Fprintf(&script, "        '%s'\n", strings.ReplaceAll(option, "'", "''"))
		}
		script.WriteString("    ) | Where-Object { $_.StartsWith($wordToComplete, [System.StringComparison]::OrdinalIgnoreCase) } | ForEach-Object {\n        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterName', $_)\n    }\n}\n")
	default:
		return &usageError{fmt.Errorf("unsupported completion shell %q; choose bash, zsh, fish or powershell", shell)}
	}
	_, err := io.WriteString(out, script.String())
	return err
}

func isFileFlag(name string) bool {
	switch name {
	case "dictionary", "dns-file", "output", "save-to":
		return true
	}
	return false
}
