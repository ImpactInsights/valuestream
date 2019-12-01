package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources"
	httpsource "github.com/ImpactInsights/valuestream/eventsources/http"
	"github.com/ImpactInsights/valuestream/eventsources/types"
	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"strings"
)

func parseTags(cliTags []string) (map[string]interface{}, error) {
	if len(cliTags) == 0 {
		return nil, nil
	}
	tags := make(map[string]interface{})
	for _, t := range cliTags {
		parts := strings.Split(t, "|")
		if len(parts) != 2 {
			return nil, fmt.Errorf("expected tag to be of form %q, received: %q",
				"k|v",
				t,
			)
		}

		tags[parts[0]] = parts[1]
	}

	return tags, nil
}

func addDefaultTags(tags map[string]interface{}) map[string]interface{} {
	user, err := user.Current()

	if err == nil {
		tags["user.name"] = user.Name
		tags["user.username"] = user.Username
	}

	return tags
}

func main() {
	app := cli.NewApp()

	app.Commands = []*cli.Command{
		{
			Name:    "event",
			Aliases: []string{"ev"},
			Usage:   "monitor custom events",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "event-source-url",
					Value:   "http://localhost:5001/customhttp",
					Usage:   "URL and path for custom HTTP event source",
					EnvVars: []string{"VS_CUSTOM_HTTP_EVENT_SOURCE_URL"},
				},
				&cli.StringFlag{
					Name:    "secret-key",
					Value:   "",
					Usage:   "Secret key for signing custom http events",
					EnvVars: []string{"VS_CUSTOM_HTTP_EVENT_SOURCE_SECRET"},
				},
				&cli.StringSliceFlag{
					Name:  "tag",
					Value: nil,
					Usage: "provide flags for each metadata key value.  Flag is of for -tags=key|value",
				},
				// action
				// namespace
				// type
				&cli.StringFlag{
					Name:    "type",
					Value:   types.DeployEventType,
					Usage:   "Types: issue|pull_request|build|deploy|sprint|pipeline",
					EnvVars: []string{"VS_CUSTOM_HTTP_EVENT_SOURCE_SECRET"},
				},
			},
			Subcommands: []*cli.Command{
				{
					Name:  "start",
					Usage: "start a new event",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:  "parent-event-id",
							Value: "",
							Usage: "parent event id to reference",
						},
					},
					Action: func(c *cli.Context) error {
						tags, err := parseTags(c.StringSlice("tag"))
						if err != nil {
							return err
						}

						if tags != nil {
							tags = addDefaultTags(tags)
						}

						var parentID *string
						if c.String("parent-event-id") != "" {
							id := c.String("parent-event-id")
							parentID = &id
						}

						id := xid.New()

						event := httpsource.Event{
							Identifier: id.String(),
							Action:     string(eventsources.StartState),
							ParentID:   parentID,
							Error:      false,
							Namespace:  "default",
							Type:       c.String("type"),
							Metadata:   tags,
						}

						bs, err := json.Marshal(&event)
						if err != nil {
							return err
						}

						resp, err := http.Post(
							c.String("event-source-url"),
							"application/javascript",
							bytes.NewReader(bs),
						)
						if err != nil {
							return err
						}
						defer resp.Body.Close()

						body, err := ioutil.ReadAll(resp.Body)
						if err != nil {
							return err
						}

						if resp.StatusCode != http.StatusOK {
							return fmt.Errorf("error creating event, received %q", string(body))
						}

						fmt.Println(id.String())
						return nil
					},
				},
				{
					Name:  "end",
					Usage: "ends an event requires event reference",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "event-id",
							Value:   "",
							Usage:   "event id to end",
							EnvVars: []string{"VS_CUSTOM_HTTP_EVENT_SOURCE_SECRET"},
						},
					},
					Action: func(c *cli.Context) error {
						eventID := c.String("event-id")
						if eventID == "" {
							return fmt.Errorf("must provide an event id")
						}

						event := httpsource.Event{
							Identifier: eventID,
							Action:     string(eventsources.EndState),
							Error:      false,
							Namespace:  "default",
							Type:       c.String("type"),
							Metadata:   nil,
						}

						bs, err := json.Marshal(&event)
						if err != nil {
							return err
						}

						resp, err := http.Post(
							c.String("event-source-url"),
							"application/javascript",
							bytes.NewReader(bs),
						)
						if err != nil {
							return err
						}
						defer resp.Body.Close()

						body, err := ioutil.ReadAll(resp.Body)
						if err != nil {
							return err
						}

						if resp.StatusCode != http.StatusOK {
							return fmt.Errorf("error creating event, statusCode(%d) received %q",
								resp.StatusCode,
								string(body),
							)
						}

						return nil
					},
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
