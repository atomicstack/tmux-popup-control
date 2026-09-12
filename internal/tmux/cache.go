package tmux

import "sync"

// optionCache memoizes global option reads between user commands. A generation
// prevents reads started before invalidation from repopulating stale entries.
var (
	optionCacheMu         sync.RWMutex
	optionCache           = map[string]string{}
	optionCacheGeneration uint64
)

func optionCacheKey(socketPath, option string) string {
	return socketPath + "\x00" + option
}

// resetCaches clears per-process option reads on shutdown, after commands,
// and when tests swap socket paths or fake clients.
func resetCaches() {
	optionCacheMu.Lock()
	optionCacheGeneration++
	optionCache = map[string]string{}
	optionCacheMu.Unlock()
	resetThemeColourCache()
}

// InvalidateOptionCache refreshes options after a user command may change them.
// Commands can partially succeed, so callers invalidate even after an error.
func InvalidateOptionCache() { resetCaches() }
