package slack

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/go-joe/joe"
	"github.com/go-joe/joe/joetest"
	"github.com/go-joe/joe/reactions"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
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
	events := make(chan slackEvent)
	a, err := newAdapter(ctx, client, client, events, conf)
	require.NoError(t, err)

	return a, client
}

func TestNewAdapter_Name(t *testing.T) {
	ctx := context.Background()
	client := new(mockSlack)

	authTestResp := &slack.AuthTestResponse{UserID: "42"}
	client.On("AuthTestContext", ctx).Return(authTestResp, nil)

	conf := Config{Name: "Test"}
	a, err := newAdapter(ctx, client, client, nil, conf)
	require.NoError(t, err)
	assert.Equal(t, "Test", a.name)
	assert.NotNil(t, a.logger)
}

func TestNewAdapter_ErrorWrap(t *testing.T) {
	ctx := context.Background()
	client := new(mockSlack)

	authErr := errors.New("this did not work")
	client.On("AuthTestContext", ctx).Return(nil, authErr)

	conf := Config{Name: "Test"}
	a, err := newAdapter(ctx, client, client, nil, conf)
	assert.Nil(t, a)
	assert.EqualError(t, err, "slack auth test failed: this did not work")
	assert.True(t, errors.Is(err, authErr))
}

func TestAdapter_IgnoreNormalMessages(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	a.events <- slackEvent{Data: &slack.MessageEvent{
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

	evt := &slack.MessageEvent{
		Msg: slack.Msg{
			Text:      "Hello world",
			Timestamp: "1360782400.498405",
			Channel:   "D023BB3L2",
		},
	}

	a.events <- slackEvent{Data: evt}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)
	expectedEvt := joe.ReceiveMessageEvent{Text: "Hello world", Channel: "D023BB3L2", ID: "1360782400.498405", Data: evt}
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

	evt := &slack.MessageEvent{
		Msg: slack.Msg{
			Text:      fmt.Sprintf("Hey %s!", a.userLink(a.userID)),
			Timestamp: "1360782400.498405",
			Channel:   "D023BB3L2",
			User:      "test",
		},
	}

	a.events <- slackEvent{Data: evt}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)
	expectedEvt := joe.ReceiveMessageEvent{Text: evt.Text, Channel: evt.Channel, ID: evt.Timestamp, AuthorID: evt.User, Data: evt}
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

	a.events <- slackEvent{Data: evt}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)
	expectedEvt := joe.ReceiveMessageEvent{Text: "PING", Data: evt}
	assert.Equal(t, expectedEvt, events[0])
}

func TestAdapter_PassiveMessage(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)
	a.listenPassive = true

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	evt := &slack.MessageEvent{
		Msg: slack.Msg{
			Text:    "Hello world",
			Channel: "C1H9RESGL",
		},
	}
	a.events <- slackEvent{Data: evt}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.NotEmpty(t, events)
	expectedEvt := joe.ReceiveMessageEvent{Text: evt.Text, Channel: evt.Channel, ID: evt.Timestamp, AuthorID: evt.User, Data: evt}
	assert.Equal(t, expectedEvt, events[0])
}

func TestAdapter_Send(t *testing.T) {
	a, slackAPI := newTestAdapter(t)

	slackAPI.On("PostMessageContext", a.context, "C1H9RESGL",
		mock.AnythingOfType("slack.MsgOption"), // slack.MsgOptionText
		mock.AnythingOfType("slack.MsgOption"), // slack.MsgOptionPostMessageParameters
		mock.AnythingOfType("slack.MsgOption"), // slack.MsgOptionUser
		mock.AnythingOfType("slack.MsgOption"), // slack.MsgOptionUsername
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

	a.events <- slackEvent{Data: &slack.UserTypingEvent{
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

	evt := slackEvent{Data: &slack.UserTypingEvent{
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

	a.events <- slackEvent{Data: &slack.UserTypingEvent{
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

func TestAdapter_IgnoreOwnMessages(t *testing.T) {
	cases := map[string]struct{ Channel string }{
		"channel message": {"C1H9RESGL"}, // map test case name to channel ID
		"direct message":  {"D023BB3L2"}, // direct slack channels start with a "D"
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
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
					Channel: c.Channel,
					User:    a.userID,
				},
			}

			a.events <- slackEvent{Data: evt}

			close(a.events)
			<-done
			brain.Finish()

			assert.Empty(t, brain.RecordedEvents())
		})
	}
}

func TestAdapter_ReactionAddedEvent(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	evt := &slack.ReactionAddedEvent{
		Type:           "reaction_added",
		User:           "U024BE7LH",
		ItemUser:       "U0G9QF9C6",
		Reaction:       "thumbsup",
		EventTimestamp: "1360782804.083113",
	}

	evt.Item.Type = "message"
	evt.Item.Channel = "C0G9QF9GZ"
	evt.Item.Timestamp = "1360782400.498405"

	a.events <- slackEvent{Data: evt}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.Len(t, events, 1)
	assert.Equal(t, reactions.Event{
		Reaction:  reactions.Reaction{Shortcode: "thumbsup"},
		MessageID: "1360782400.498405",
		Channel:   "C0G9QF9GZ",
		AuthorID:  "U024BE7LH",
	}, events[0])
}

func TestAdapter_ReactionAddedEvent_IgnoredItemTypes(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	evt := &slack.ReactionAddedEvent{
		Type:           "reaction_added",
		User:           "U024BE7LH",
		ItemUser:       "U0G9QF9C6",
		Reaction:       "thumbsup",
		EventTimestamp: "1360782804.083113",
	}

	evt.Item.Type = "file"
	evt.Item.File = "F0HS27V1Z"

	a.events <- slackEvent{Data: evt}

	close(a.events)
	<-done
	brain.Finish()

	assert.Empty(t, brain.RecordedEvents())
}

func TestAdapter_ReactionAddedEvent_IgnoreOwnReactions(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	evt := &slack.ReactionAddedEvent{
		Type:           "reaction_added",
		User:           a.userID,
		ItemUser:       "U0G9QF9C6",
		Reaction:       "thumbsup",
		EventTimestamp: "1360782804.083113",
	}

	evt.Item.Type = "message"
	evt.Item.Channel = "C0G9QF9GZ"
	evt.Item.Timestamp = "1360782400.498405"

	a.events <- slackEvent{Data: evt}

	close(a.events)
	<-done
	brain.Finish()

	assert.Empty(t, brain.RecordedEvents())
}

func TestAdapter_React(t *testing.T) {
	a, slackAPI := newTestAdapter(t)

	msg := joe.Message{
		Channel: "C0G9QF9GZ",
		ID:      "1360782400.498405",
	}

	ref := slack.NewRefToMessage(msg.Channel, msg.ID)
	slackAPI.On("AddReactionContext", a.context, "thumbsup", ref).Return(nil)

	err := a.React(reactions.Thumbsup, msg)
	require.NoError(t, err)
	slackAPI.AssertExpectations(t)
}

func TestAdapter_IgnoreUnknownEventTypes(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)

	core, logs := observer.New(zap.DebugLevel)
	a.logger = zap.New(core)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	type unknownEvent struct{ Text string }
	a.events <- slackEvent{Data: unknownEvent{"test"}}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.Len(t, events, 0)

	assert.Empty(t, logs.TakeAll())
}

func TestAdapter_LogUnknownEventTypes(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)

	core, logs := observer.New(zap.DebugLevel)
	a.logger = zap.New(core)
	a.logUnknownMessageTypes = true

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	type unknownEvent struct{ Text string }
	evt := slackEvent{Type: "test_type", Data: unknownEvent{"test"}}
	a.events <- evt

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.Empty(t, events)

	messages := logs.TakeAll()
	require.Len(t, messages, 1)
	assert.Equal(t, "Received unknown type from Real Time Messaging (RTM) system", messages[0].Message)

	fields := messages[0].ContextMap()
	assert.Equal(t, "test_type", fields["type"])
	assert.Equal(t, "slack.unknownEvent", fields["go_type"])
	assert.Equal(t, evt.Data, fields["data"])
}

func TestAdapter_RTMError(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)

	core, logs := observer.New(zap.DebugLevel)
	a.logger = zap.New(core)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	a.events <- slackEvent{Data: &slack.RTMError{
		Code: 42,
		Msg:  "this did not work",
	}}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.Empty(t, events)

	messages := logs.TakeAll()
	require.Len(t, messages, 1)
	assert.Equal(t, "Slack Real Time Messaging (RTM) error", messages[0].Message)

	fields := messages[0].ContextMap()
	assert.EqualValues(t, 42, fields["code"])
	assert.Equal(t, "this did not work", fields["msg"])
}

func TestAdapter_UnmarshallingErrorEvent(t *testing.T) {
	brain := joetest.NewBrain(t)
	a, _ := newTestAdapter(t)

	core, logs := observer.New(zap.DebugLevel)
	a.logger = zap.New(core)

	done := make(chan bool)
	go func() {
		a.handleSlackEvents(brain.Brain)
		done <- true
	}()

	a.events <- slackEvent{Data: &slack.UnmarshallingErrorEvent{
		ErrorObj: errors.New("failure"),
	}}

	close(a.events)
	<-done
	brain.Finish()

	events := brain.RecordedEvents()
	require.Empty(t, events)

	messages := logs.TakeAll()
	require.Len(t, messages, 1)
	assert.Equal(t, "Slack unmarshalling error", messages[0].Message)

	fields := messages[0].ContextMap()
	assert.Equal(t, "failure", fields["error"])
}

type mockSlack struct {
	mock.Mock
}

var _ slackAPI = new(mockSlack)

func (m *mockSlack) AuthTestContext(ctx context.Context) (resp *slack.AuthTestResponse, err error) {
	args := m.Called(ctx)
	if x := args.Get(0); x != nil {
		resp = x.(*slack.AuthTestResponse)
	}
	return resp, args.Error(1)
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

func (m *mockSlack) AddReactionContext(ctx context.Context, name string, item slack.ItemRef) error {
	args := m.Called(ctx, name, item)
	return args.Error(0)
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
