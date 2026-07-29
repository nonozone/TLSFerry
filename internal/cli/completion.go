package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func runCompletion(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "completion: shell is required; supported shells: zsh, bash, fish")
		return 2
	}
	if args[0] == "install" {
		flags := flag.NewFlagSet("completion install", flag.ContinueOnError)
		flags.SetOutput(stderr)
		shell := flags.String("shell", detectShell(), "shell to install completion for")
		if err := flags.Parse(args[1:]); err != nil {
			return 2
		}
		return installCompletion(*shell, stdout, stderr)
	}
	script, ok := completionScript(args[0])
	if !ok {
		fmt.Fprintln(stderr, "completion: supported shells: zsh, bash, fish")
		return 2
	}
	fmt.Fprint(stdout, script)
	return 0
}

func installCompletion(shell string, stdout, stderr io.Writer) int {
	shell = strings.ToLower(strings.TrimSpace(shell))
	script, ok := completionScript(shell)
	if !ok {
		fmt.Fprintln(stderr, "completion install: supported shells: zsh, bash, fish")
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "completion install: %v\n", err)
		return 1
	}
	paths := map[string]string{
		"zsh":  filepath.Join(home, ".zfunc", "_tlsferry"),
		"bash": filepath.Join(home, ".local", "share", "bash-completion", "completions", "tlsferry"),
		"fish": filepath.Join(home, ".config", "fish", "completions", "tlsferry.fish"),
	}
	path := paths[shell]
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(stderr, "completion install: %v\n", err)
		return 1
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".tlsferry-completion-*")
	if err != nil {
		fmt.Fprintf(stderr, "completion install: %v\n", err)
		return 1
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.WriteString(script); err != nil {
		temporary.Close()
		fmt.Fprintf(stderr, "completion install: %v\n", err)
		return 1
	}
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		fmt.Fprintf(stderr, "completion install: %v\n", err)
		return 1
	}
	if err := temporary.Close(); err != nil {
		fmt.Fprintf(stderr, "completion install: %v\n", err)
		return 1
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		fmt.Fprintf(stderr, "completion install: %v\n", err)
		return 1
	}
	activationPath := ""
	switch shell {
	case "zsh":
		activationPath = filepath.Join(home, ".zshrc")
		err = appendCompletionActivation(activationPath, `fpath=("$HOME/.zfunc" $fpath)
autoload -Uz compinit
compinit`)
	case "bash":
		activationPath = filepath.Join(home, ".bashrc")
		err = appendCompletionActivation(activationPath, `if [ -f "$HOME/.local/share/bash-completion/completions/tlsferry" ]; then
  source "$HOME/.local/share/bash-completion/completions/tlsferry"
fi`)
	}
	if err != nil {
		fmt.Fprintf(stderr, "completion install: activate completion: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s completion installed: %s\n", shell, path)
	if activationPath != "" {
		fmt.Fprintf(stdout, "shell activation configured: %s\n", activationPath)
	}
	fmt.Fprintln(stdout, "Start a new shell to activate completion.")
	return 0
}

func appendCompletionActivation(path, content string) error {
	const begin = "# >>> TLSFerry completion >>>"
	const end = "# <<< TLSFerry completion <<<"
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(existing), begin) {
		return nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		if _, err := file.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(file, "%s\n%s\n%s\n", begin, content, end)
	return err
}

func detectShell() string {
	shell := filepath.Base(os.Getenv("SHELL"))
	switch shell {
	case "zsh", "bash", "fish":
		return shell
	default:
		return "zsh"
	}
}

func completionScript(shell string) (string, bool) {
	switch strings.ToLower(shell) {
	case "zsh":
		return zshCompletion, true
	case "bash":
		return bashCompletion, true
	case "fish":
		return fishCompletion, true
	default:
		return "", false
	}
}

const zshCompletion = `#compdef tlsferry

_tlsferry() {
  local -a commands auth_providers cloud_providers
  commands=(auth completion deploy discover help issue plan preflight renew service validate version)
  auth_providers=(cloudflare tencent aliyun qiniu)
  cloud_providers=(tencent aliyun qiniu)
  if (( CURRENT == 2 )); then
    _describe 'command' commands
    return
  fi
  case $words[2] in
    auth)
      if (( CURRENT == 3 )); then _values 'action' login status logout; return; fi
      if [[ $words[3] == login && CURRENT == 4 ]]; then _describe 'provider' auth_providers; return; fi
      _arguments '--profile[keychain profile]:profile' '--no-browser[do not open the cloud console]'
      ;;
    discover)
      if (( CURRENT == 3 )); then _values 'source' cloud; return; fi
      _arguments '--provider[cloud provider]:provider:(tencent aliyun qiniu)' '--credential[credential reference]:reference' '--format[output format]:format:(table json)'
      ;;
    completion)
      if [[ $words[3] == install ]]; then _arguments '--shell[target shell]:shell:(zsh bash fish)'; return; fi
      _values 'shell or action' zsh bash fish install
      ;;
    service)
      if (( CURRENT == 3 )); then _values 'action' install status run-now logs uninstall; return; fi
      _arguments '--config[configuration file]:file:_files' '--hour[daily hour]:hour' '--minute[daily minute]:minute' '--accept-tos[accept ACME terms]' '--execute[allow external operations]'
      ;;
    issue|deploy|renew|validate|plan|preflight)
      _arguments '--config[configuration file]:file:_files' '--certificate[certificate name]:name' '--provider[deployment provider]:provider:(tencent-cdn tencent-cos aliyun-cdn qiniu-cdn)' '--state-dir[state directory]:directory:_directories' '--output-dir[certificate output directory]:directory:_directories' '--input-dir[certificate input directory]:directory:_directories' '--accept-tos[accept ACME terms]' '--execute[allow external operations]' '--force[force renewal]'
      ;;
  esac
}

compdef _tlsferry tlsferry
`

const bashCompletion = `_tlsferry_completion() {
  local cur prev command action
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  command="${COMP_WORDS[1]}"
  action="${COMP_WORDS[2]}"
  local commands="auth completion deploy discover help issue plan preflight renew service validate version"
  local auth_providers="cloudflare tencent aliyun qiniu"
  local cloud_providers="tencent aliyun qiniu"
  if [[ $COMP_CWORD -eq 1 ]]; then COMPREPLY=( $(compgen -W "$commands" -- "$cur") ); return; fi
  case "$prev" in
    --provider) COMPREPLY=( $(compgen -W "$cloud_providers tencent-cdn tencent-cos aliyun-cdn qiniu-cdn" -- "$cur") ); return ;;
    --format) COMPREPLY=( $(compgen -W "table json" -- "$cur") ); return ;;
	--shell) COMPREPLY=( $(compgen -W "zsh bash fish" -- "$cur") ); return ;;
    --config|--state-dir|--output-dir|--input-dir) COMPREPLY=( $(compgen -f -- "$cur") ); return ;;
  esac
  case "$command" in
    auth)
      if [[ $COMP_CWORD -eq 2 ]]; then COMPREPLY=( $(compgen -W "login status logout" -- "$cur") );
      elif [[ "$action" == login && $COMP_CWORD -eq 3 ]]; then COMPREPLY=( $(compgen -W "$auth_providers" -- "$cur") );
      else COMPREPLY=( $(compgen -W "--profile --no-browser" -- "$cur") ); fi ;;
    discover) COMPREPLY=( $(compgen -W "cloud --provider --credential --format" -- "$cur") ) ;;
    completion) COMPREPLY=( $(compgen -W "zsh bash fish install --shell" -- "$cur") ) ;;
    service) COMPREPLY=( $(compgen -W "install status run-now logs uninstall --config --hour --minute --accept-tos --execute" -- "$cur") ) ;;
    issue|deploy|renew|validate|plan|preflight) COMPREPLY=( $(compgen -W "--config --certificate --provider --state-dir --output-dir --input-dir --accept-tos --execute --force" -- "$cur") ) ;;
  esac
}
complete -F _tlsferry_completion tlsferry
`

const fishCompletion = `complete -c tlsferry -f
complete -c tlsferry -n '__fish_use_subcommand' -a 'auth completion deploy discover help issue plan preflight renew service validate version'
complete -c tlsferry -n '__fish_seen_subcommand_from auth' -a 'login status logout'
complete -c tlsferry -n '__fish_seen_subcommand_from login' -a 'cloudflare tencent aliyun qiniu'
complete -c tlsferry -n '__fish_seen_subcommand_from discover' -a 'cloud'
complete -c tlsferry -n '__fish_seen_subcommand_from discover' -l provider -a 'tencent aliyun qiniu'
complete -c tlsferry -n '__fish_seen_subcommand_from discover' -l credential
complete -c tlsferry -n '__fish_seen_subcommand_from discover' -l format -a 'table json'
complete -c tlsferry -n '__fish_seen_subcommand_from completion' -a 'zsh bash fish install'
complete -c tlsferry -n '__fish_seen_subcommand_from completion' -l shell -a 'zsh bash fish'
complete -c tlsferry -n '__fish_seen_subcommand_from service' -a 'install status run-now logs uninstall'
complete -c tlsferry -l config -r
complete -c tlsferry -l certificate -r
complete -c tlsferry -l profile -r
complete -c tlsferry -l accept-tos
complete -c tlsferry -l execute
complete -c tlsferry -l force
`
