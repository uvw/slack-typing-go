package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"golang.org/x/sync/errgroup"
)

func main() {
	const debug = false
	_ = godotenv.Load()
	logger := log.New(os.Stdout, "typing: ", 0)
	tokens := strings.Fields(os.Getenv("SLACK_API_TOKENS"))
	if len(tokens) == 0 {
		logger.Fatal("SLACK_API_TOKENS is missing")
	}

	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	defer cancel()

	g.Go(func() error {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
		return errors.New((<-signals).String())
	})

	for _, t := range tokens {
		token := t
		g.Go(func() error {
			logger.Println("Starting", token[0:12], "...")

			api := slack.New(token,
				slack.OptionLog(log.New(os.Stdout, "slack: ", 0)),
				slack.OptionDebug(debug)).NewRTM()

			if r, err := api.AuthTest(); err != nil {
				return err
			} else {
				logger.Printf("Running as %s (%s, %s)\n", r.User, r.Team, r.URL)
			}

			defer api.Disconnect()
			go api.ManageConnection()

			for {
				select {
				case e, ok := <-api.IncomingEvents:
					if !ok {
						return nil
					}
					switch data := e.Data.(type) {
					case *slack.UserTypingEvent:
						logger.Println("Responding to", data.Channel, data.User)
						api.SendMessage(api.NewTypingMessage(data.Channel))
						//api.SendMessage(api.NewOutgoingMessage(fmt.Sprintf("What are you typing <@%s>?", data.User), data.Channel))
					}
				case <-ctx.Done():
					return nil
				}
			}
		})
	}

	logger.Println("Exit:", g.Wait())
}
