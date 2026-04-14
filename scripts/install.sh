#!/usr/bin/env bash
set -euo pipefail

repo="github.com/cloudboy-jh/glib/cmd/glib"

ensure_path_line() {
  local shell_file="$1"
  local path_line="$2"

  if [[ -f "$shell_file" ]] && grep -Fq "$path_line" "$shell_file"; then
    return 1
  fi

  if [[ ! -f "$shell_file" ]]; then
    touch "$shell_file"
  fi

  {
    printf "\n"
    printf "# glib: add Go bin to PATH\n"
    printf "%s\n" "$path_line"
  } >> "$shell_file"

  return 0
}

resolve_go_bin() {
  local gobin
  local gopath

  gobin="$(go env GOBIN)"
  if [[ -n "$gobin" ]]; then
    printf "%s\n" "$gobin"
    return
  fi

  gopath="$(go env GOPATH)"
  if [[ -n "$gopath" ]]; then
    printf "%s/bin\n" "${gopath%%:*}"
    return
  fi

  printf "%s/go/bin\n" "$HOME"
}

go_bin="$(resolve_go_bin)"
go_bin_export="$go_bin"

if [[ "$go_bin" == "$HOME"/* ]]; then
  go_bin_export="\$HOME/${go_bin#"$HOME"/}"
fi

path_line="export PATH=\"$go_bin_export:\$PATH\""

echo "Installing glib from $repo@latest..."
go install "$repo@latest"

updated_files=()
for shell_file in "$HOME/.zprofile" "$HOME/.zshrc"; do
  if ensure_path_line "$shell_file" "$path_line"; then
    updated_files+=("$shell_file")
  fi
done

if [[ ":$PATH:" != *":$go_bin:"* ]]; then
  export PATH="$go_bin:$PATH"
fi

echo ""
echo "glib installed to: $go_bin/glib"
if [[ ${#updated_files[@]} -gt 0 ]]; then
  echo "Updated PATH in:"
  for file in "${updated_files[@]}"; do
    echo "- $file"
  done
  echo "Open a new shell, or run: source ~/.zprofile && source ~/.zshrc"
else
  echo "PATH already configured in zsh profile files."
fi
echo "Verify: command -v glib"
