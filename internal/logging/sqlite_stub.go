//go:build !sqlitedebug

package logging

import "errors"

// ErrSQLiteDebugUnavailable is returned by EnableSQLiteDebug in builds made
// without the sqlitedebug tag, which leave the modernc.org/sqlite driver out.
var ErrSQLiteDebugUnavailable = errors.New("sqlite debug logging not compiled in (rebuild with -tags sqlitedebug)")

// sqliteSink is never instantiated in this build; it exists so the rest of
// the package compiles unchanged. sqliteDebug therefore always stays nil.
type sqliteSink struct{}

func openSQLiteSink(SQLiteRunInfo) (*sqliteSink, error) {
	return nil, ErrSQLiteDebugUnavailable
}

func (*sqliteSink) nextSeq() int64           { return 0 }
func (*sqliteSink) enqueueEvent(eventRecord) {}
func (*sqliteSink) enqueueSpan(spanRecord)   {}
func (*sqliteSink) Close(RunResult)          {}
