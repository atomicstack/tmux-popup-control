#!/usr/bin/env zsh

# Runs inside the test tmux session. Prints instructions, then drops to a shell.

echo "plugin test environment"
echo ""
echo "keybinding:  prefix + P  (open plugin menu)"
echo ""
echo "suggested test steps:"
echo "  1. prefix+P → plugins → install    (clones tmux-sensible + tmux-yank)"
echo "  2. prefix+P → plugins → list       (verify installed)"
echo "  3. prefix+P → plugins → update     (select all or individual)"
echo "  4. prefix+P → plugins → uninstall  (remove one)"
echo "  5. prefix+P → plugins → tidy       (remove undeclared)"
echo ""
echo "plugin dir:  $TMUX_PLUGIN_MANAGER_PATH"
echo ""
exec zsh
