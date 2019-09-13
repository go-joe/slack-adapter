package slack

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-joe/joe"
	"github.com/go-joe/joe/joetest"
	"github.com/nlopes/slack"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var (
	// compile time test to check if we are implementing the interface.
	_ joe.Adapter = new(BotAdapter)

	botUser   = "test-bot"
	botUserID = "42"
)

func newTestAdapter(t *testing.T) (*BotAdapter, *mockSlack) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)
	client := new(mockSlack)

	authTestResp := &slack.AuthTestResponse{User: botUser, UserID: botUserID}
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

func TestAdapter_IgnoreChannelOwnMessages(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	evt := &slack.MessageEvent{
		Msg: slack.Msg{
			Text:    "Hello world",
			Channel: "C1H9RESGL",
			User:    botUserID,
		},
	}

	a.events <- slack.RTMEvent{Data: evt}

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

	evt := &slack.MessageEvent{
		Msg: slack.Msg{
			Text:    "Hello world",
			Channel: "D023BB3L2",
		},
	}

	a.events <- slack.RTMEvent{Data: evt}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)
	expectedEvt := joe.ReceiveMessageEvent{Text: "Hello world", Channel: "D023BB3L2", Data: evt}
	assert.Equal(t, expectedEvt, events[0])
}

func TestAdapter_IgnoreDirectOwnMessages(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	evt := &slack.MessageEvent{
		Msg: slack.Msg{
			Text:    "Hello world",
			Channel: "D023BB3L2",
			User:    botUserID,
		},
	}

	a.events <- slack.RTMEvent{Data: evt}

	close(a.events)
	<-done
	brain.Finish()

	assert.Empty(t, brain.RecordedEvents())
}

func TestAdapter_MentionBot(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	evt := &slack.MessageEvent{
		Msg: slack.Msg{
			Text:    fmt.Sprintf("Hey %s!", a.userLink(a.userID)),
			Channel: "D023BB3L2",
			User:    "test",
		},
	}

	a.events <- slack.RTMEvent{Data: evt}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)
	expectedEvt := joe.ReceiveMessageEvent{Text: evt.Text, Channel: evt.Channel, AuthorID: evt.User, Data: evt}
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

	evt := &slack.MessageEvent{
		Msg: slack.Msg{
			Text: fmt.Sprintf("%s PING", a.userLink(a.userID)),
		},
	}

	a.events <- slack.RTMEvent{Data: evt}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)
	expectedEvt := joe.ReceiveMessageEvent{Text: "PING", Data: evt}
	assert.Equal(t, expectedEvt, events[0])
}

func TestAdapter_Send(t *testing.T) {
	a, slackAPI := newTestAdapter(t)
	slackAPI.On("PostMessageContext", a.context, "C1H9RESGL",
		mock.AnythingOfType("slack.MsgOption"), // the text
		mock.AnythingOfType("slack.MsgOption"), // enable parsing
		mock.AnythingOfType("slack.MsgOption"), // user ID
		mock.AnythingOfType("slack.MsgOption"), // user name
	).Return("", "", nil)

	err := a.Send("Hello World", "C1H9RESGL")
	require.NoError(t, err)
	slackAPI.AssertExpectations(t)
}

func TestAdapter_Close(t *testing.T) {
	a, slackAPI := newTestAdapter(t)
	slackAPI.On("Disconnect").Return(nil)

	err := a.Close()
	require.NoError(t, err)
	slackAPI.AssertExpectations(t)
}

func TestAdapter_UserTypingEvent(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, slackAPI := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	slackAPI.On("GetUserInfo", "UG96B2SGJ").Return(&slack.User{
		ID:       "UG96B2SGJ",
		Name:     "JD",
		RealName: "John Doe",
	}, nil)

	a.events <- slack.RTMEvent{Data: &slack.UserTypingEvent{
		User:    "UG96B2SGJ",
		Channel: "C1H9RESGL",
	}}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)

	expectedUser := joe.User{ID: "UG96B2SGJ", Name: "JD", RealName: "John Doe"}
	expectedEvt := joe.UserTypingEvent{User: expectedUser, Channel: "C1H9RESGL"}
	assert.Equal(t, expectedEvt, events[0])
}

func TestAdapter_UserTypingCache(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, slackAPI := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	slackAPI.On("GetUserInfo", "UG96B2SGJ").Return(&slack.User{
		ID:       "UG96B2SGJ",
		Name:     "JD",
		RealName: "John Doe",
	}, nil).Once()

	evt := slack.RTMEvent{Data: &slack.UserTypingEvent{
		User:    "UG96B2SGJ",
		Channel: "C1H9RESGL",
	}}

	a.events <- evt
	a.events <- evt
	a.events <- evt

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)

	expectedUser := joe.User{ID: "UG96B2SGJ", Name: "JD", RealName: "John Doe"}
	expectedEvt := joe.UserTypingEvent{User: expectedUser, Channel: "C1H9RESGL"}
	assert.Equal(t, expectedEvt, events[0])
}

func TestAdapter_UserTypingEventError(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, slackAPI := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	slackAPI.On("GetUserInfo", "UG96B2SGJ").Return(nil, errors.New("something went wrong"))

	a.events <- slack.RTMEvent{Data: &slack.UserTypingEvent{
		User:    "UG96B2SGJ",
		Channel: "C1H9RESGL",
	}}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)

	expectedUser := joe.User{ID: "UG96B2SGJ"}
	expectedEvt := joe.UserTypingEvent{User: expectedUser, Channel: "C1H9RESGL"}
	assert.Equal(t, expectedEvt, events[0])
}

type mockSlack struct {
	mock.Mock
}

var _ slackAPI = new(mockSlack)

func (m *mockSlack) AuthTestContext(ctx context.Context) (*slack.AuthTestResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(*slack.AuthTestResponse), args.Error(1)
}

func (m *mockSlack) PostMessageContext(ctx context.Context, channelID string,
	opts ...slack.MsgOption) (respChannel, respTimestamp string, err error) {
	callArgs := []interface{}{ctx, channelID}
	for _, o := range opts {
		callArgs = append(callArgs, o)
	}
	args := m.Called(callArgs...)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *mockSlack) GetUserInfo(user string) (usr *slack.User, err error) {
	args := m.Called(user)
	if x := args.Get(0); x != nil {
		usr = x.(*slack.User)
	}

	return usr, args.Error(1)
}

func (m *mockSlack) Disconnect() error {
	args := m.Called()
	return args.Error(0)
}
