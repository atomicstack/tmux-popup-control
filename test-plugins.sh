#!/usr/bin/env zsh
set -euo pipefail

# Bootstrap a temporary tmux environment to test plugin management.
# Usage: ./test-plugins.sh

REPO_DIR="${0:A:h}"
BINARY="$REPO_DIR/tmux-popup-control"

# Build if needed
if [[ ! -x "$BINARY" ]] || [[ "$REPO_DIR/main.go" -nt "$BINARY" ]]; then
  echo "building tmux-popup-control..."
  make -C "$REPO_DIR" build
fi

TEST_DIR=$(mktemp -d -t tmux-plugins-test)
PLUGINS="$TEST_DIR/plugins"
SOCKET="$TEST_DIR/tmux.sock"
CONF="$TEST_DIR/tmux.conf"
mkdir -p "$PLUGINS"
ln -s "$REPO_DIR" "$PLUGINS/tmux-popup-control"

echo "TEST_DIR=$TEST_DIR, PLUGINS=$PLUGINS, SOCKET=$SOCKET, CONF=$CONF"

trap 'tmux -S "$SOCKET" kill-server 2>/dev/null; rm -rf "$TEST_DIR"' EXIT INT TERM

cp "$REPO_DIR/test-plugins-inner.sh" "$TEST_DIR/inner.sh"
chmod +x "$TEST_DIR/inner.sh"

cat > "$CONF" <<EOF
# --- prefix ---
unbind-key -q C-b
set -g prefix ^A
bind-key a send-prefix

# --- windows ---
unbind-key -q ^C
bind-key ^C new-window
unbind-key -q c
bind-key c new-window
unbind-key -q C
bind-key C new-window -a

unbind-key -q ^@
bind-key ^@ next-window
unbind-key -q ^N
bind-key ^N next-window
unbind-key -q " "
bind-key " " next-window
unbind-key -q n
bind-key n next-window

unbind-key -q ^P
bind-key ^P previous-window
unbind-key -q p
bind-key p previous-window

unbind-key -q ^A
bind-key ^A last-window

unbind-key -q w
bind-key w list-windows
unbind-key -q '"'
bind-key '"' choose-tree -Z
bind-key "'" command-prompt       { select-window -t ':%%' }
bind-key , command-prompt -I '#W' { rename-window -- '%%'  }
bind-key . command-prompt -I '#I' { move-window -t '%%'    }
bind-key / command-prompt -k      { list-keys -1 "%%"      }
bind-key '\$' command-prompt -I '#S' { rename-session -- '%%' }

unbind-key -q '<'
unbind-key -q '>'
bind-key '<' swap-window -d -t -1
bind-key '>' swap-window -d -t +1

# --- sessions ---
unbind-key -q l
bind-key l switch-client -l
unbind-key -q L
bind-key L switch-client -l

unbind-key -q s
bind-key s choose-tree -sZ

unbind-key -q '{'
unbind-key -q '}'
bind-key '{' switch-client -p
bind-key '}' switch-client -n

# --- panes ---
unbind-key -q |
bind-key | split-window -h
unbind-key -q %
bind-key % split-window

unbind-key -q '('
unbind-key -q ')'
bind-key '(' swap-pane -U
bind-key ')' swap-pane -D

unbind-key -q '!'
bind-key '!' break-pane

bind-key k confirm-before -p "#[fg=yellow]kill-pane#[default] #{session_name}:#{window_index}.#{pane_index} (title=#{pane_title}, pane_id=#{pane_id})? #[fg=yellow](y/n)#[default]" kill-pane
bind-key K confirm-before -p "#[fg=orange]kill-window#[default] #{session_name}:#{window_index} (name=#{window_name}, window_id=#{window_id})? #[fg=orange](y/n)#[default]" kill-window
bind-key ^K confirm-before -p "#[fg=red,bold]kill-server#[default] #{pid}? #[fg=red,bold](y/n)#[default]" kill-server

# --- misc ---
unbind-key -q ^D
bind-key ^D detach
unbind-key -q *
bind-key * list-clients
unbind-key -q ^L
bind-key ^L refresh-client
unbind-key -q o
bind-key o customize-mode

unbind-key -q ^M
bind-key ^M set -g mouse\; display 'mouse mode: #{?#{mouse},#[fg=colour46]on,#[fg=colour250]off}#[default]'

unbind-key -q &
bind-key & set-window-option synchronize-panes\; display-message "set synchronize-panes to: #{?pane_synchronized,#[fg=colour46]on#[default],#[fg=colour250]off#[default]}"

bind-key f command-prompt -p '(find-window-by-name)' "find-window -N '%%'"
bind-key H command-prompt -p 'capture-pane-to-file:' -I "~/tmux-#D.%F-%H-%M-%S.log" 'capture-pane -S - ; save-buffer %1 ; delete-buffer'

# --- appearance ---
set-option -g cursor-colour colour33
set-option -g prompt-cursor-colour colour33
set-option -g message-style fg=colour33
set-window-option -g menu-border-lines rounded
set-window-option -g menu-selected-style fg=colour255,bg=colour33
set-window-option -g menu-style fg=colour251
set-option -g status-left ''
set-option -g status-right "#[fg=colour39]shells #[fg=colour245]| #[fg=colour33]#S"
set-option -g status-keys emacs
set-option -g pane-border-style fg=colour236
set-option -g pane-active-border-style fg=colour33
set-option -g pane-border-lines heavy
set-option -g pane-border-status top
set-option -g pane-border-format '#[fg=yellow]#{pane_id}#[default] #[fg=white]#{session_name}:#{window_index}.#{pane_index}#[default] title=#[fg=white]#{pane_title}#[default], cmd=#[fg=white]#{pane_current_command}#[default]'
set-option -g popup-border-style fg=colour239
set-option -g popup-border-lines rounded
set-window-option -g status-style bg=colour236,fg=white
set-window-option -g window-status-current-style bg=white,fg=colour33
set-window-option -g window-status-current-format "#I:#W#{s/\\\\*//:?window_flags,#{window_flags}, }"
set-option -g mode-style fg=colour255,bg=colour69
set-option -g history-limit 50000
set-option -g display-time 4000
set-option -g visual-activity on
set-option -g visual-bell on

# --- plugins ---
set -g @plugin 'atomicstack/tmux-popup-control'
set -g @plugin 'tmux-plugins/tmux-sensible'
set -g @plugin 'tmux-plugins/tmux-yank'

set-environment -g TMUX_PLUGIN_MANAGER_PATH "$PLUGINS"

bind P display-popup -E "$BINARY -socket $SOCKET"

run '$BINARY install-and-init-plugins'
EOF

export TMUX_SOCKET="$SOCKET"

tmux -S "$SOCKET" -f "$CONF" new-session -s test "$TEST_DIR/inner.sh"

echo "$0 exiting" >&2
