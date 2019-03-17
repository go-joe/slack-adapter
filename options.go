package slack

import "go.uber.org/zap"

// An Option is used to configure the slack adapter.
type Option func(*Config) error

// WithLogger can be used to inject a different logger for the slack adapater.
func WithLogger(logger *zap.Logger) Option {
	return func(conf *Config) error {
		conf.Logger = logger
		return nil
	}
}

// WithDebug enables debug messages of the slack client.
func WithDebug(debug bool) Option {
	return func(conf *Config) error {
		conf.Debug = debug
		return nil
	}
}
