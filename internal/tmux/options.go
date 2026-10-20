package tmux

import (
	"os"
	"strings"
	"sync"
)

// pluginOptionPrefix names every option this plugin reads from tmux.
const pluginOptionPrefix = "@tmux-popup-control-"

// pluginOptionsFormat prints each global option raw, unlike plain
// show-options, which quotes and escapes values containing spaces.
const pluginOptionsFormat = "#{option_name}=#{option_value}"

// pluginOptionCache holds one bulk read of every @tmux-popup-control-* option
// per socket. It shares the option cache's generation so a user command that
// may have changed an option invalidates both.
var (
	pluginOptionMu    sync.Mutex
	pluginOptionCache = map[string]map[string]string{}
)

// PluginOptions returns every global @tmux-popup-control-* option, read with
// a single control-mode show-options and memoised until the option cache is
// invalidated. A failed read returns an empty map and is not cached.
func PluginOptions(socketPath string) map[string]string {
	pluginOptionMu.Lock()
	defer pluginOptionMu.Unlock()
	if opts, ok := pluginOptionCache[socketPath]; ok {
		return opts
	}
	optionCacheMu.RLock()
	generation := optionCacheGeneration
	optionCacheMu.RUnlock()

	client, err := newTmux(socketPath)
	if err != nil {
		return map[string]string{}
	}
	output, err := client.Command("show-options", "-g", "-F", pluginOptionsFormat)
	if err != nil {
		return map[string]string{}
	}
	opts := parsePluginOptions(output)

	optionCacheMu.RLock()
	current := generation == optionCacheGeneration
	optionCacheMu.RUnlock()
	if current {
		pluginOptionCache[socketPath] = opts
	}
	return opts
}

func parsePluginOptions(output string) map[string]string {
	opts := map[string]string{}
	for line := range strings.SplitSeq(output, "\n") {
		name, value, ok := strings.Cut(line, "=")
		if ok && strings.HasPrefix(name, pluginOptionPrefix) {
			opts[name] = value
		}
	}
	return opts
}

func resetPluginOptionCache() {
	pluginOptionMu.Lock()
	pluginOptionCache = map[string]map[string]string{}
	pluginOptionMu.Unlock()
}

// Option resolves a plugin setting: a non-empty environment variable wins,
// otherwise the @tmux-popup-control-* option from the bulk read.
func Option(socketPath, envKey, optionName string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return ShowOption(socketPath, optionName)
}

// ShowOption returns a global option's value, or "" when it is unset or tmux
// cannot be reached. @tmux-popup-control-* options come from the one bulk read
// in PluginOptions; anything else is a single control-mode show-options.
// Results are memoised until a user command or shutdown invalidates the cache.
func ShowOption(socketPath, option string) string {
	if strings.HasPrefix(option, pluginOptionPrefix) {
		return PluginOptions(socketPath)[option]
	}
	key := optionCacheKey(socketPath, option)
	optionCacheMu.RLock()
	if v, ok := optionCache[key]; ok {
		optionCacheMu.RUnlock()
		return v
	}
	generation := optionCacheGeneration
	optionCacheMu.RUnlock()

	client, err := newTmux(socketPath)
	if err != nil {
		return ""
	}
	value, err := client.GlobalOption(option)
	if err != nil {
		return ""
	}
	value = strings.TrimSpace(value)
	optionCacheMu.Lock()
	if generation == optionCacheGeneration {
		optionCache[key] = value
	}
	optionCacheMu.Unlock()
	return value
}
