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

func newTestAdapter(t *testing.T) (*BotAdapter, *mockSlack) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)
	client := new(mockSlack)

	authTestResp := &slack.AuthTestResponse{User: "test-bot", UserID: "42"}
	client.On("AuthTestContext", ctx).Return(authTestResp, nil)

	conf := Config{Logger: logger}
	events := make(chan slack.RTMEvent)
	a, err := newAdapter(ctx, client, events, conf)
	require.NoError(t, err)

	return a, client
}

func TestAdapter_IgnoreNormalMessages(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	a.events <- slack.RTMEvent{Data: &slack.MessageEvent{
		Msg: slack.Msg{
			Text:    "Hello world",
			Channel: "C1H9RESGL",
		},
	}}

	close(a.events)
	<-done
	brain.Finish()

	assert.Empty(t, brain.RecordedEvents())
}

func TestAdapter_DirectMessages(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	a.events <- slack.RTMEvent{Data: &slack.MessageEvent{
		Msg: slack.Msg{
			Text:    "Hello world",
			Channel: "D023BB3L2",
		},
	}}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)
	expectedEvt := joe.ReceiveMessageEvent{Text: "Hello world", Channel: "D023BB3L2"}
	assert.Equal(t, expectedEvt, events[0])
}

func TestAdapter_MentionBot(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	msg := fmt.Sprintf("Hey %s!", a.userLink(a.userID))
	channel := "D023BB3L2"
	a.events <- slack.RTMEvent{Data: &slack.MessageEvent{
		Msg: slack.Msg{
			Text:    msg,
			Channel: channel,
		},
	}}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)
	expectedEvt := joe.ReceiveMessageEvent{Text: msg, Channel: channel}
	assert.Equal(t, expectedEvt, events[0])
}

func TestAdapter_MentionBotPrefix(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	a.events <- slack.RTMEvent{Data: &slack.MessageEvent{
		Msg: slack.Msg{
			Text: fmt.Sprintf("%s PING", a.userLink(a.userID)),
		},
	}}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)
	expectedEvt := joe.ReceiveMessageEvent{Text: "PING"}
	assert.Equal(t, expectedEvt, events[0])
}

func TestAdapter_Send(t *testing.T) {
	a, slackAPI := newTestAdapter(t)
	slackAPI.On("PostMessageContext", a.context, "C1H9RESGL",
		mock.AnythingOfType("slack.MsgOption"), // the text
		mock.AnythingOfType("slack.MsgOption"), // enable parsing
	).Return("", "", nil)

	err := a.Send("Hello World", "C1H9RESGL")
	require.NoError(t, err)
	slackAPI.AssertExpectations(t)
}

type mockSlack struct {
	mock.Mock
}

var _ slackAPI = new(mockSlack)

func (m *mockSlack) AuthTestContext(ctx context.Context) (*slack.AuthTestResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(*slack.AuthTestResponse), args.Error(1)
}

func (m *mockSlack) PostMessageContext(ctx context.Context, channelID string, opts ...slack.MsgOption) (respChannel, respTimestamp string, err error) {
	callArgs := []interface{}{ctx, channelID}
	for _, o := range opts {
		callArgs = append(callArgs, o)
	}
	args := m.Called(callArgs...)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *mockSlack) GetUserInfo(user string) (*slack.User, error) {
	args := m.Called(user)
	return args.Get(0).(*slack.User), args.Error(1)
}

func (m *mockSlack) Disconnect() error {
	args := m.Called()
	return args.Error(0)
}
