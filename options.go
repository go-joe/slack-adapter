package slack

import (
	"crypto/tls"
	"errors"
	"time"

	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

// An Option is used to configure the slack adapter.
type Option func(*Config) error

// Config contains the configuration of a BotAdapter.
type Config struct {
	Token             string
	VerificationToken string
	Name              string
	Debug             bool
	Logger            *zap.Logger
	SlackAPIURL       string // defaults to github.com/slack-go/slack.APIURL but can be changed for unit tests

	// SendMsgParams contains settings that are applied to all messages sent
	// by the BotAdapter.
	SendMsgParams slack.PostMessageParameters

	// Log unknown message types as error message for debugging. This option is
	// disabled by default.
	LogUnknownMessageTypes bool

	// Listen and respond to all messages not just those directed at the Bot User.
	ListenPassive bool

	// Options if you want to use the Slack Events API. Ignored on the normal RTM adapter.
	EventsAPI EventsAPIConfig
}

// EventsAPIConfig contains the configuration of an EventsAPIServer.
type EventsAPIConfig struct {
	ShutdownTimeout   time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	TLSConf           *tls.Config
	CertFile, KeyFile string
}

func (conf Config) slackOptions() []slack.Option {
	if conf.Logger == nil {
		conf.Logger = zap.NewNop()
	}
	if conf.SlackAPIURL == "" {
		conf.SlackAPIURL = slack.APIURL
	}
	if conf.SlackAPIURL[len(conf.SlackAPIURL)-1] != '/' {
		conf.SlackAPIURL += "/"
	}

	opts := []slack.Option{
		slack.OptionAPIURL(conf.SlackAPIURL),
	}

	if conf.Debug {
		opts = append(opts,
			slack.OptionDebug(conf.Debug),
			slack.OptionLog(zap.NewStdLog(conf.Logger)),
		)
	}

	return opts
}

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

// WithTLS is an option for the EventsAPIServer that enables serving HTTP
// requests via TLS.
func WithTLS(certFile, keyFile string) Option {
	return func(conf *Config) error {
		if certFile == "" {
			return errors.New("path to certificate file cannot be empty")
		}
		if keyFile == "" {
			return errors.New("path to private key file cannot be empty")
		}

		conf.EventsAPI.CertFile = certFile
		conf.EventsAPI.KeyFile = keyFile

		return nil
	}
}

// WithTLSConfig is an option for the EventsAPIServer that can be used in
// combination with the WithTLS(…) option to configure the HTTPS server.
func WithTLSConfig(tlsConf *tls.Config) Option {
	return func(conf *Config) error {
		conf.EventsAPI.TLSConf = tlsConf
		return nil
	}
}

// WithTimeouts is an option for the EventsAPIServer that sets both the read
// and write timeout of the HTTP server to the same given value.
func WithTimeouts(d time.Duration) Option {
	return func(conf *Config) error {
		conf.EventsAPI.ReadTimeout = d
		conf.EventsAPI.WriteTimeout = d
		return nil
	}
}

// WithReadTimeout is an option for the EventsAPIServer that sets the servers
// maximum duration for reading the entire HTTP request, including the body.
func WithReadTimeout(d time.Duration) Option {
	return func(conf *Config) error {
		conf.EventsAPI.ReadTimeout = d
		return nil
	}
}

// WithWriteTimeout is an option for the EventsAPIServer that sets the
// servers maximum duration before timing out writes of the HTTP response.
func WithWriteTimeout(d time.Duration) Option {
	return func(conf *Config) error {
		conf.EventsAPI.WriteTimeout = d
		return nil
	}
}
