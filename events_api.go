package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-joe/joe"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"go.uber.org/zap"
)

// EventsAPIServer is an adapter that receives messages from Slack using the events API.
// In contrast to the classical adapter, this server receives messages as HTTP
// requests instead of via a websocket.
//
// See https://api.slack.com/events-api
type EventsAPIServer struct {
	*BotAdapter
	http *http.Server
	conf EventsAPIConfig
	opts []slackevents.Option
}

// EventsAPIAdapter returns a new EventsAPIServer as joe.Module.
// If you want to use the slack RTM API instead (i.e. using web sockets), you
// should use the slack.Adapter(…) function instead.
func EventsAPIAdapter(listenAddr, token, verificationToken string, opts ...Option) joe.Module {
	return joe.ModuleFunc(func(joeConf *joe.Config) error {
		conf, err := newConf(token, joeConf, opts)
		if err != nil {
			return err
		}
		conf.VerificationToken = verificationToken

		a, err := NewEventsAPIServer(joeConf.Context, listenAddr, conf)
		if err != nil {
			return err
		}

		joeConf.SetAdapter(a)
		return nil
	})
}

// NewEventsAPIServer creates a new *EventsAPIServer that connects to Slack
// using the events API. Note that you will usually configure this type of slack
// adapter as joe.Module (i.e. using the EventsAPIAdapter function of this package).
func NewEventsAPIServer(ctx context.Context, listenAddr string, conf Config) (*EventsAPIServer, error) {
	events := make(chan slackEvent)
	client := slack.New(conf.Token, slack.OptionDebug(conf.Debug))
	adapter, err := newAdapter(ctx, client, nil, events, conf)
	if err != nil {
		return nil, err
	}

	a := &EventsAPIServer{
		BotAdapter: adapter,
		conf:       conf.EventsAPI,
	}

	a.opts = append(a.opts, slackevents.OptionVerifyToken(
		&slackevents.TokenComparator{
			VerificationToken: conf.VerificationToken,
		},
	))

	a.http = &http.Server{
		Addr:         listenAddr,
		Handler:      http.HandlerFunc(a.httpHandler),
		ErrorLog:     zap.NewStdLog(conf.Logger),
		TLSConfig:    conf.EventsAPI.TLSConf,
		ReadTimeout:  conf.EventsAPI.ReadTimeout,
		WriteTimeout: conf.EventsAPI.WriteTimeout,
	}

	return a, nil
}

// RegisterAt implements the joe.Adapter interface by emitting the slack API
// events to the given brain.
func (a *EventsAPIServer) RegisterAt(brain *joe.Brain) {
	// Start the HTTP server. The goroutine will stop when the adapter is closed.
	go a.startHTTPServer()
	a.BotAdapter.RegisterAt(brain)
}

func (a *EventsAPIServer) startHTTPServer() {
	var err error
	if a.conf.CertFile == "" {
		err = a.http.ListenAndServe()
	} else {
		err = a.http.ListenAndServeTLS(a.conf.CertFile, a.conf.KeyFile)
	}

	if err != nil && err != http.ErrServerClosed {
		a.logger.Error("HTTP server failure", zap.Error(err))
	}
}

func (a *EventsAPIServer) httpHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		a.logger.Error("Failed to read request body", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	eventsAPIEvent, err := slackevents.ParseEvent(body, a.opts...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch eventsAPIEvent.Type {
	case slackevents.URLVerification:
		a.handleURLVerification(body, w)

	case slackevents.CallbackEvent:
		a.handleEvent(eventsAPIEvent.InnerEvent)

	default:
		a.logger.Error("Received unknown top level event type",
			zap.String("type", eventsAPIEvent.Type),
		)
	}
}

func (a *EventsAPIServer) handleURLVerification(req []byte, resp http.ResponseWriter) {
	a.logger.Info("Received URL verification challenge request")

	var r slackevents.ChallengeResponse
	err := json.Unmarshal(req, &r)
	if err != nil {
		a.logger.Error("Failed to unmarshal challenge as JSON", zap.Error(err))
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp.Header().Set("Content-Type", "text")
	_, err = fmt.Fprint(resp, r.Challenge)
	if err != nil {
		a.logger.Error("Failed to write challenge response", zap.Error(err))
	}

	resp.WriteHeader(http.StatusOK)
}

func (a *EventsAPIServer) handleEvent(innerEvent slackevents.EventsAPIInnerEvent) {
	switch ev := innerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		a.handleMessageEvent(ev)

	case *slackevents.AppMentionEvent:
		a.handleAppMentionEvent(ev)

	case *slackevents.ReactionAddedEvent:
		a.handleReactionAddedEvent(ev)

	default:
		if a.logUnknownMessageTypes {
			a.logger.Error("Received unknown event type",
				zap.String("type", innerEvent.Type),
				zap.Any("data", innerEvent.Data),
				zap.String("go_type", fmt.Sprintf("%T", innerEvent.Data)),
			)
		}
	}
}

func (a *EventsAPIServer) handleMessageEvent(ev *slackevents.MessageEvent) {
	var edited *slack.Edited
	if ev.Edited != nil {
		edited = &slack.Edited{
			User:      ev.Edited.User,
			Timestamp: ev.Edited.TimeStamp,
		}
	}

	icons := &slack.Icon{}
	if ev.Icons != nil {
		icons = &slack.Icon{
			IconURL:   ev.Icons.IconURL,
			IconEmoji: ev.Icons.IconEmoji,
		}
	}

	a.events <- slackEvent{
		Type: ev.Type,
		Data: &slack.MessageEvent{
			Msg: slack.Msg{
				Type:            ev.Type,
				Channel:         ev.Channel,
				User:            ev.User,
				Text:            ev.Text,
				Timestamp:       ev.TimeStamp,
				ThreadTimestamp: ev.ThreadTimeStamp,
				Edited:          edited,
				SubType:         ev.SubType,
				EventTimestamp:  ev.EventTimeStamp.String(),
				BotID:           ev.BotID,
				Username:        ev.Username,
				Icons:           icons,
			},
		},
	}
}

func (a *EventsAPIServer) handleAppMentionEvent(ev *slackevents.AppMentionEvent) {
	a.events <- slackEvent{
		Type: ev.Type,
		Data: &slack.MessageEvent{
			Msg: slack.Msg{
				Type:            ev.Type,
				User:            ev.User,
				Text:            ev.Text,
				Timestamp:       ev.TimeStamp,
				ThreadTimestamp: ev.ThreadTimeStamp,
				Channel:         ev.Channel,
				EventTimestamp:  ev.EventTimeStamp.String(),
				BotID:           ev.BotID,
			},
		},
	}
}

func (a *EventsAPIServer) handleReactionAddedEvent(ev *slackevents.ReactionAddedEvent) {
	evt := &slack.ReactionAddedEvent{
		Type:           ev.Type,
		User:           ev.User,
		ItemUser:       ev.ItemUser,
		Reaction:       ev.Reaction,
		EventTimestamp: ev.EventTimestamp,
	}

	evt.Item.Type = ev.Item.Type
	evt.Item.Channel = ev.Item.Channel
	evt.Item.Timestamp = ev.Item.Timestamp

	a.events <- slackEvent{
		Type: ev.Type,
		Data: evt,
	}
}

// Close shuts down the disconnects the adapter from the slack API.
func (a *EventsAPIServer) Close() error {
	ctx := context.Background()
	if a.conf.ShutdownTimeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, a.conf.ShutdownTimeout)
		defer cancel()
	}

	return a.http.Shutdown(ctx)
}
