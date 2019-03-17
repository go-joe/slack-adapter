package slack

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-joe/joe"
	"github.com/go-joe/joe/joetest"
	"github.com/nlopes/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// compile time test to check if we are implementing the interface.
var _ joe.Adapter = new(BotAdapter)

func newTestAdapter(t *testing.T) (*BotAdapter, *slack.RTM) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)
	client := new(mockSlack)

	authTestResp := &slack.AuthTestResponse{User: "test-bot", UserID: "42"}
	client.On("AuthTestContext", ctx).Return(authTestResp, nil)

	rtm := &slack.RTM{IncomingEvents: make(chan slack.RTMEvent)}
	client.On("NewRTM").Return(rtm)

	conf := Config{Logger: logger}
	a, err := newAdapter(client, ctx, conf)
	require.NoError(t, err)

	return a, rtm
}

func TestAdapter_IgnoreNormalMessages(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, rtm := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	rtm.IncomingEvents <- slack.RTMEvent{Data: &slack.MessageEvent{
		Msg: slack.Msg{
			Text:    "Hello world",
			Channel: "C1H9RESGL",
		},
	}}

	close(rtm.IncomingEvents)
	<-done
	brain.Finish()

	assert.Empty(t, brain.RecordedEvents())
}

func TestAdapter_DirectMessages(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, rtm := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	rtm.IncomingEvents <- slack.RTMEvent{Data: &slack.MessageEvent{
		Msg: slack.Msg{
			Text:    "Hello world",
			Channel: "D023BB3L2",
		},
	}}

	close(rtm.IncomingEvents)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)
	expectedEvt := joe.ReceiveMessageEvent{Text: "Hello world", Channel: "D023BB3L2"}
	assert.Equal(t, expectedEvt, events[0])
}

func TestAdapter_MentionBot(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, rtm := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	msg := fmt.Sprintf("Hey %s!", a.userLink(a.userID))
	channel := "D023BB3L2"
	rtm.IncomingEvents <- slack.RTMEvent{Data: &slack.MessageEvent{
		Msg: slack.Msg{
			Text:    msg,
			Channel: channel,
		},
	}}

	close(rtm.IncomingEvents)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)
	expectedEvt := joe.ReceiveMessageEvent{Text: msg, Channel: channel}
	assert.Equal(t, expectedEvt, events[0])
}

func TestAdapter_MentionBotPrefix(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, rtm := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	rtm.IncomingEvents <- slack.RTMEvent{Data: &slack.MessageEvent{
		Msg: slack.Msg{
			Text: fmt.Sprintf("%s PING", a.userLink(a.userID)),
		},
	}}

	close(rtm.IncomingEvents)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)
	expectedEvt := joe.ReceiveMessageEvent{Text: "PING"}
	assert.Equal(t, expectedEvt, events[0])
}

type mockSlack struct {
	mock.Mock
}

func (a *mockSlack) AuthTestContext(ctx context.Context) (*slack.AuthTestResponse, error) {
	args := a.Called(ctx)
	return args.Get(0).(*slack.AuthTestResponse), args.Error(1)
}

func (a *mockSlack) NewRTM(opts ...slack.RTMOption) *slack.RTM {
	callArgs := make([]interface{}, len(opts))
	for i, opt := range opts {
		callArgs[i] = opt
	}
	args := a.Called(callArgs...)
	return args.Get(0).(*slack.RTM)
}

func (a *mockSlack) GetUserInfo(user string) (*slack.User, error) {
	args := a.Called(user)
	return args.Get(0).(*slack.User), args.Error(1)
}
