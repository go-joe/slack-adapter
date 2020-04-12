package slack

import (
	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

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

// WithMessageParams overrides the default parameters that are used when sending
// any message to slack.
func WithMessageParams(params slack.PostMessageParameters) Option {
	return func(conf *Config) error {
		conf.SendMsgParams = params
		return nil
	}
}

// WithLogUnknownMessageTypes makes the adapter log unknown message types as
// error message for debugging. This option is disabled by default.
func WithLogUnknownMessageTypes() Option {
	return func(conf *Config) error {
		conf.LogUnknownMessageTypes = true
		return nil
	}
}

// WithListenPassive makes the adapter listen and respond to all messages not
// just those directed at it
func WithListenPassive() Option {
	return func(conf *Config) error {
		conf.ListenPassive = true
		return nil
	}
}
