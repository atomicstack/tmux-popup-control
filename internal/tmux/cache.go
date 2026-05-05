package tmux

import "sync"

// optionCache memoizes per-(socket,option) global tmux option lookups for the
// life of the process. tmux server options the binary cares about
// (`@tmux-popup-control-*`, plus a few session-storage settings) are
// configured at tmux startup and do not change while the popup runs, so a
// permanent cache is safe and avoids issuing redundant `show-options`
// commands on every backend poll cycle. Each backend tick previously fired
// 3+ duplicate ShowOption calls for the same option; under heavy parallel
// test load each control-mode round-trip costs hundreds of milliseconds and
// these dominated startup latency.
var (
	optionCacheMu sync.RWMutex
	optionCache   = map[string]string{}
)

func optionCacheKey(socketPath, option string) string {
	return socketPath + "\x00" + option
}

// resetCaches clears the per-process tmux caches. Test-only helper used by
// table-driven cases that swap socket paths or fake clients between runs.
func resetCaches() {
	optionCacheMu.Lock()
	optionCache = map[string]string{}
	optionCacheMu.Unlock()
}
