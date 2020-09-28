package lumberjack

import "context"

// LoggerOption ...
type LoggerOption interface {
	apply(*loggerOption)
}

type funcLoggerOption struct {
	f func(*loggerOption)
}

func (fdo *funcLoggerOption) apply(do *loggerOption) {
	fdo.f(do)
}

func newFuncLoggerOption(f func(*loggerOption)) *funcLoggerOption {
	return &funcLoggerOption{
		f: f,
	}
}

func defaultOptions() *loggerOption {
	return &loggerOption{
		bufSize: 1,
		millCh:  make(chan bool, millChSize),
	}
}

// New ...
func New(opts ...LoggerOption) (Writer, error) {
	fo := defaultOptions()
	ctx, cancel := context.WithCancel(context.Background())
	fo.cancel = cancel

	for _, opt := range opts {
		opt.apply(fo)
	}

	if err := fo.openExistingOrNew(); err != nil {
		return nil, err
	}

	go fo.millRun(ctx)

	return fo, nil
}

// WithName ...
func WithName(name string) LoggerOption {
	return newFuncLoggerOption(func(l *loggerOption) {
		l.filename = name
	})
}

// WithMaxBytes ...
func WithMaxBytes(bytes int64) LoggerOption {
	return newFuncLoggerOption(func(l *loggerOption) {
		l.maxBytes = bytes
	})
}

// WithMaxBackups ...
func WithMaxBackups(size int) LoggerOption {
	return newFuncLoggerOption(func(l *loggerOption) {
		l.maxBackups = size
	})
}

// WithMaxDays ...
func WithMaxDays(days int) LoggerOption {
	return newFuncLoggerOption(func(l *loggerOption) {
		l.maxDays = days
	})
}

// WithCompress ...
func WithCompress() LoggerOption {
	return newFuncLoggerOption(func(l *loggerOption) {
		l.compress = true
	})
}

// WithLocalTime ...
func WithLocalTime() LoggerOption {
	return newFuncLoggerOption(func(l *loggerOption) {
		l.localTime = true
	})
}

// WithBufferSize ...
func WithBufferSize(size int) LoggerOption {
	return newFuncLoggerOption(func(l *loggerOption) {
		l.bufSize = size
	})
}

// WithReWrite ...
func WithReWrite() LoggerOption {
	return newFuncLoggerOption(func(l *loggerOption) {
		l.rewrite = true
	})
}
