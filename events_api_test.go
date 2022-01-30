package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-joe/joe"
	"github.com/go-joe/joe/joetest"
	"github.com/go-joe/joe/reactions"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

func newTestEventsAPIServer(t *testing.T, optionalConf ...Config) (_ *EventsAPIServer, finish func() (events []interface{})) {
	var conf Config
	if len(optionalConf) > 0 {
		conf = optionalConf[0]
	}

	ctx := context.Background()
	conf.Debug = true
	if conf.Logger == nil {
		conf.Logger = zaptest.NewLogger(t)
	}

	slackAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth.test" {
			_ = json.NewEncoder(w).Encode(slack.AuthTestResponse{
				UserID: "test-userID",
			})
		}
	}))
	t.Cleanup(slackAPI.Close)
	conf.SlackAPIURL = slackAPI.URL

	s, err := NewEventsAPIServer(ctx, "127.0.0.1:0", conf)
	require.NoError(t, err)

	brain := joetest.NewBrain(t)
	done := make(chan bool)
	go func() {
		s.handleSlackEvents(brain.Brain)
		done <- true
	}()

	finish = func() []interface{} {
		assert.NoError(t, s.Close())
		<-done // wait until event processing loop has stopped
		brain.Finish()
		return brain.RecordedEvents()
	}

	return s, finish
}

func TestEventsAPIServer_HandleMessageEvent(t *testing.T) {
	s, recordedEvents := newTestEventsAPIServer(t)

	req := httptest.NewRequest("POST", "/foo", toJSON(slackevents.EventsAPICallbackEvent{
		Type: slackevents.CallbackEvent,
		InnerEvent: rawJSON(slackevents.MessageEvent{
			Type:            slackevents.Message,
			Channel:         "D023BB3L2",
			User:            "U1234",
			Username:        "fgrosse",
			Text:            "Hello World!",
			TimeStamp:       "1595070350",
			ThreadTimeStamp: "1595070351",
			EventTimeStamp:  "1595070352",
		}),
	}))

	resp := httptest.NewRecorder()
	s.httpHandler(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	events := recordedEvents()
	require.NotEmpty(t, events)

	actual, ok := events[0].(joe.ReceiveMessageEvent)
	require.True(t, ok)

	actualRawData := actual.Data
	actual.Data = nil // validated separately
	assert.IsType(t, new(slack.MessageEvent), actualRawData)
	assert.Equal(t, "Hello World!", actualRawData.(*slack.MessageEvent).Text)
}

func TestEventsAPIServer_HandleAppMentionEvent(t *testing.T) {
	s, recordedEvents := newTestEventsAPIServer(t)

	req := httptest.NewRequest("POST", "/foo", toJSON(slackevents.EventsAPICallbackEvent{
		Type: slackevents.CallbackEvent,
		InnerEvent: rawJSON(slackevents.AppMentionEvent{
			Type:            slackevents.AppMention,
			Channel:         "D023BB3L2",
			User:            "U1234",
			Text:            "Hey @joe!",
			TimeStamp:       "1595070350",
			ThreadTimeStamp: "1595070351",
			EventTimeStamp:  "1595070352",
		}),
	}))

	resp := httptest.NewRecorder()
	s.httpHandler(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	events := recordedEvents()
	require.NotEmpty(t, events)

	actual, ok := events[0].(joe.ReceiveMessageEvent)
	require.True(t, ok)

	actualRawData := actual.Data
	actual.Data = nil // validated separately
	assert.IsType(t, new(slack.MessageEvent), actualRawData)
	assert.Equal(t, "Hey @joe!", actualRawData.(*slack.MessageEvent).Text)
}

func TestEventsAPIServer_HandleReactionAddedEvent(t *testing.T) {
	s, recordedEvents := newTestEventsAPIServer(t)

	req := httptest.NewRequest("POST", "/foo", toJSON(slackevents.EventsAPICallbackEvent{
		Type: slackevents.CallbackEvent,
		InnerEvent: rawJSON(slackevents.ReactionAddedEvent{
			Type:     slackevents.ReactionAdded,
			User:     "U1234",
			Reaction: "+1",
			Item: slackevents.Item{
				Type:      "message",
				Channel:   "D023BB3L2",
				Timestamp: "1595070350",
			},
		}),
	}))

	resp := httptest.NewRecorder()
	s.httpHandler(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	events := recordedEvents()
	require.NotEmpty(t, events)
	require.IsType(t, reactions.Event{}, events[0])

	actual := events[0].(reactions.Event)
	assert.Equal(t, "D023BB3L2", actual.Channel)
	assert.Equal(t, "1595070350", actual.MessageID)
	assert.Equal(t, "U1234", actual.AuthorID)
	assert.Equal(t, "+1", actual.Reaction.Shortcode)
}

func TestEventsAPIServer_HTTPHandlerMiddleware(t *testing.T) {
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path == "/health" {
				_, _ = fmt.Fprint(w, "Everything is fine!")
				return
			}

			next.ServeHTTP(w, req)
		})
	}

	obs, errorLogs := observer.New(zap.ErrorLevel)
	s, _ := newTestEventsAPIServer(t, Config{
		Logger: zap.New(obs),
		EventsAPI: EventsAPIConfig{
			Middleware: middleware,
		},
	})

	req := httptest.NewRequest("GET", "/health", strings.NewReader("Are you okay?"))
	resp := httptest.NewRecorder()

	s.http.Handler.ServeHTTP(resp, req)

	assert.Equal(t, "Everything is fine!", resp.Body.String())
	assert.Empty(t, errorLogs.All())
}

func toJSON(req interface{}) io.Reader {
	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(req)
	if err != nil {
		panic(err)
	}

	return b
}

func rawJSON(v interface{}) *json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	msg := json.RawMessage(raw)
	return &msg
}
