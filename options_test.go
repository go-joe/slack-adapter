package slack

import (
	"net/http"
	"testing"

	"github.com/go-joe/joe"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func joeConf(t *testing.T) *joe.Config {
	joeConf := new(joe.Config)
	require.NoError(t, joe.WithLogger(zaptest.NewLogger(t)).Apply(joeConf))
	return joeConf
}

func TestDefaultConfig(t *testing.T) {
	conf, err := newConf("my-secret-token", joeConf(t), nil)
	require.NoError(t, err)
	assert.NotNil(t, conf.Logger)
	assert.Equal(t, "full", conf.SendMsgParams.Parse)
	assert.Equal(t, 1, conf.SendMsgParams.LinkNames)
}

func TestWithLogger(t *testing.T) {
	logger := zaptest.NewLogger(t)
	conf, err := newConf("my-secret-token", joeConf(t), []Option{
		WithLogger(logger),
	})

	require.NoError(t, err)
	assert.Equal(t, logger, conf.Logger)
}

func TestWithDebug(t *testing.T) {
	conf, err := newConf("my-secret-token", joeConf(t), []Option{
		WithDebug(true),
	})

	require.NoError(t, err)
	assert.Equal(t, true, conf.Debug)

	conf, err = newConf("my-secret-token", joeConf(t), []Option{
		WithDebug(false),
	})

	require.NoError(t, err)
	assert.Equal(t, false, conf.Debug)
}

func TestWithMessageParams(t *testing.T) {
	conf, err := newConf("my-secret-token", joeConf(t), []Option{
		WithMessageParams(slack.PostMessageParameters{
			Parse:     "none",
			LinkNames: 0,
		}),
	})

	require.NoError(t, err)
	assert.NotNil(t, conf.Logger)
	assert.Equal(t, "none", conf.SendMsgParams.Parse)
	assert.Equal(t, 0, conf.SendMsgParams.LinkNames)
}

func TestWithLogUnknownMessageTypes(t *testing.T) {
	conf, err := newConf("my-secret-token", joeConf(t), nil)
	require.NoError(t, err)

	assert.Equal(t, false, conf.LogUnknownMessageTypes, "LogUnknownMessageTypes should be disabled by default")

	conf, err = newConf("my-secret-token", joeConf(t), []Option{
		WithLogUnknownMessageTypes(),
	})
	require.NoError(t, err)
	assert.Equal(t, true, conf.LogUnknownMessageTypes)
}

func TestWithListenPassive(t *testing.T) {
	conf, err := newConf("my-secret-token", joeConf(t), []Option{
		WithListenPassive(),
	})

	require.NoError(t, err)
	assert.Equal(t, true, conf.ListenPassive)
}

func TestWithMiddleware(t *testing.T) {
	var ok bool
	middleware := func(next http.Handler) http.Handler {
		ok = true // proof this function executed
		return next
	}

	conf, err := newConf("my-secret-token", joeConf(t), []Option{
		WithMiddleware(middleware),
	})

	require.NoError(t, err)
	require.NotNil(t, conf.EventsAPI.Middleware)

	conf.EventsAPI.Middleware(nil)
	assert.True(t, ok)
}
